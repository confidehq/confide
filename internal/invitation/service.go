package invitation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/internal/billing"
	"github.com/phantompunk/confide/internal/db/queries"
	"github.com/phantompunk/confide/internal/mailer"
	"github.com/phantompunk/confide/internal/permission"
)

var (
	ErrNotFound  = errors.New("invitation not found")
	ErrExpired   = errors.New("invitation expired or already accepted")
	ErrForbidden = errors.New("insufficient role")
	ErrPlanLimit = errors.New("member limit reached for current plan")
	ErrConflict  = errors.New("already a member of this workspace")
)

// DB is the subset of queries used by the invitation service.
type DB interface {
	CreateInvitation(ctx context.Context, arg queries.CreateInvitationParams) (queries.CreateInvitationRow, error)
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (queries.GetInvitationByTokenHashRow, error)
	ListPendingInvitations(ctx context.Context, workspaceID string) ([]queries.ListPendingInvitationsRow, error)
	DeleteInvitation(ctx context.Context, arg queries.DeleteInvitationParams) error
	DeleteAllExpiredInvitations(ctx context.Context) error

	GetWorkspaceByID(ctx context.Context, id string) (queries.GetWorkspaceByIDRow, error)
	GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error)
	CountWorkspaceMembers(ctx context.Context, workspaceID string) (int64, error)
	CreateWorkspaceMember(ctx context.Context, arg queries.CreateWorkspaceMemberParams) error
}

// Mailer is the email sending dependency.
type Mailer interface {
	SendInvitation(to, workspaceName, inviterUsername, role, link string)
}

type Service struct {
	log       zerolog.Logger
	db        DB
	mailer    Mailer
	appDomain string
}

func NewService(pool *pgxpool.Pool, m *mailer.Mailer, appDomain string) *Service {
	return &Service{
		log:       log.With().Str("module", "invitation").Logger(),
		db:        queries.New(pool),
		mailer:    m,
		appDomain: appDomain,
	}
}

// Invitation is the service-layer type returned by Create and List.
type Invitation struct {
	ID          string
	WorkspaceID string
	Email       string
	Role        string
	ExpiresAt   string
	CreatedAt   string
	// Link is the invite URL, populated only when Create is called without an email.
	Link string
}

// InvitationPreview is the public view returned by Resolve (no email).
type InvitationPreview struct {
	ID              string
	WorkspaceName   string
	InviterUsername string
	Role            string
	ExpiresAt       string
}

// Create generates and stores a new invitation, then sends the invite email.
// callerRole is pre-resolved by middleware. Enforces the free-plan 1-collaborator limit.
func (s *Service) Create(ctx context.Context, workspaceID, callerRole, callerAccountID, email, role string) (Invitation, error) {
	// Cannot invite someone to a role higher than the caller's own.
	if permission.RoleRank(role) > permission.RoleRank(callerRole) {
		return Invitation{}, ErrForbidden
	}

	ws, err := s.db.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return Invitation{}, err
	}
	limit := billing.PlanMemberLimit(ws.Plan)
	if limit >= 0 {
		count, err := s.db.CountWorkspaceMembers(ctx, workspaceID)
		if err != nil {
			return Invitation{}, err
		}
		if count >= limit {
			return Invitation{}, ErrPlanLimit
		}
	}

	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return Invitation{}, fmt.Errorf("generate token: %w", err)
	}

	id, err := randomID()
	if err != nil {
		return Invitation{}, err
	}

	expiresAt := pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true}

	inv, err := s.db.CreateInvitation(ctx, queries.CreateInvitationParams{
		ID:                 id,
		WorkspaceID:        workspaceID,
		InvitedByAccountID: callerAccountID,
		Email:              email,
		Role:               role,
		TokenHash:          tokenHash,
		ExpiresAt:          expiresAt,
	})
	if err != nil {
		return Invitation{}, err
	}

	link := fmt.Sprintf("%s/invite/%s", s.appDomain, rawToken)

	out := Invitation{
		ID:          inv.ID,
		WorkspaceID: inv.WorkspaceID,
		Email:       inv.Email,
		Role:        inv.Role,
		ExpiresAt:   inv.ExpiresAt.Time.Format(time.RFC3339),
		CreatedAt:   inv.CreatedAt.Time.Format(time.RFC3339),
	}
	if email != "" {
		go s.mailer.SendInvitation(email, ws.Name, callerRole, role, link)
	} else {
		out.Link = link
	}
	return out, nil
}

// List returns pending (unexpired) invitations for a workspace.
// Caller role is enforced by middleware.
func (s *Service) List(ctx context.Context, workspaceID string) ([]Invitation, error) {
	rows, err := s.db.ListPendingInvitations(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Invitation, len(rows))
	for i, r := range rows {
		out[i] = Invitation{
			ID:          r.ID,
			WorkspaceID: workspaceID,
			Email:       r.Email,
			Role:        r.Role,
			ExpiresAt:   r.ExpiresAt.Time.Format(time.RFC3339),
			CreatedAt:   r.CreatedAt.Time.Format(time.RFC3339),
		}
	}
	return out, nil
}

// DeleteExpiredInvitations hard-deletes all expired invitation rows. Called by the reaper.
func (s *Service) DeleteExpiredInvitations(ctx context.Context) error {
	return s.db.DeleteAllExpiredInvitations(ctx)
}

// Revoke deletes a pending invitation. Caller role is enforced by middleware.
func (s *Service) Revoke(ctx context.Context, workspaceID, inviteID string) error {
	return s.db.DeleteInvitation(ctx, queries.DeleteInvitationParams{
		ID:          inviteID,
		WorkspaceID: workspaceID,
	})
}

// Resolve looks up an invitation by raw token for the public preview page.
// Returns ErrNotFound if invalid, ErrExpired if expired or already accepted.
func (s *Service) Resolve(ctx context.Context, rawToken string) (InvitationPreview, error) {
	tokenHash, err := hashToken(rawToken)
	if err != nil {
		return InvitationPreview{}, ErrNotFound
	}
	row, err := s.db.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InvitationPreview{}, ErrNotFound
		}
		return InvitationPreview{}, err
	}
	if time.Now().After(row.ExpiresAt.Time) {
		return InvitationPreview{}, ErrExpired
	}
	return InvitationPreview{
		ID:              row.ID,
		WorkspaceName:   row.WorkspaceName,
		InviterUsername: row.InviterUsername.String,
		Role:            row.Role,
		ExpiresAt:       row.ExpiresAt.Time.Format(time.RFC3339),
	}, nil
}

// Accept marks the invitation as accepted and adds the account as a workspace member.
// Returns ErrConflict if the account is already a member.
func (s *Service) Accept(ctx context.Context, rawToken, accountID string) error {
	tokenHash, err := hashToken(rawToken)
	if err != nil {
		return ErrNotFound
	}
	row, err := s.db.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if time.Now().After(row.ExpiresAt.Time) {
		return ErrExpired
	}

	// Check if already a member.
	_, err = s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: row.WorkspaceID,
		AccountID:   accountID,
	})
	if err == nil {
		return ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if err := s.db.CreateWorkspaceMember(ctx, queries.CreateWorkspaceMemberParams{
		WorkspaceID: row.WorkspaceID,
		AccountID:   accountID,
		Role:        row.Role,
	}); err != nil {
		return err
	}
	return s.db.DeleteInvitation(ctx, queries.DeleteInvitationParams{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// generateToken returns a raw base64url token and its hex-encoded SHA-256 hash.
func generateToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	rawToken = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256(b)
	tokenHash = hex.EncodeToString(h[:])
	return
}

// hashToken hashes a raw base64url token for DB lookup.
func hashToken(rawToken string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
