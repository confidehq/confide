package workspace

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
)

var (
	ErrNotFound  = errors.New("workspace not found")
	ErrForbidden = errors.New("insufficient role")
	ErrPlanLimit = errors.New("workspace limit reached for free plan")
	ErrLastOwner = errors.New("cannot remove or demote the sole owner")
	ErrHasMembers = errors.New("workspace still has non-owner members")
)

// DB is the subset of queries.Queries used by workspace.Service.
type DB interface {
	CreateWorkspace(ctx context.Context, arg queries.CreateWorkspaceParams) (queries.Workspace, error)
	CreateWorkspaceMember(ctx context.Context, arg queries.CreateWorkspaceMemberParams) error
	GetPersonalWorkspace(ctx context.Context, accountID string) (queries.GetPersonalWorkspaceRow, error)
	GetWorkspaceByID(ctx context.Context, id string) (queries.GetWorkspaceByIDRow, error)
	GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error)
	CountOwnerWorkspaces(ctx context.Context, accountID string) (int64, error)
	UpsertWorkspaceMemberKey(ctx context.Context, arg queries.UpsertWorkspaceMemberKeyParams) error
	GetWorkspaceMemberKey(ctx context.Context, arg queries.GetWorkspaceMemberKeyParams) (queries.GetWorkspaceMemberKeyRow, error)
	ListMemberIdentityKeys(ctx context.Context, workspaceID string) ([]queries.ListMemberIdentityKeysRow, error)
	GetMembersWithoutWorkspaceKeyWithUsername(ctx context.Context, workspaceID string) ([]queries.GetMembersWithoutWorkspaceKeyWithUsernameRow, error)

	ListWorkspacesByAccount(ctx context.Context, accountID string) ([]queries.ListWorkspacesByAccountRow, error)
	RenameWorkspace(ctx context.Context, arg queries.RenameWorkspaceParams) error
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]queries.ListWorkspaceMembersRow, error)
	UpdateWorkspaceMemberRole(ctx context.Context, arg queries.UpdateWorkspaceMemberRoleParams) error
	DeleteWorkspaceMember(ctx context.Context, arg queries.DeleteWorkspaceMemberParams) error
	DeleteWorkspaceMemberKey(ctx context.Context, arg queries.DeleteWorkspaceMemberKeyParams) error
	CountWorkspaceOwners(ctx context.Context, workspaceID string) (int64, error)
	CountNonOwnerMembers(ctx context.Context, workspaceID string) (int64, error)
}

type Service struct {
	db   DB
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: queries.New(pool), pool: pool}
}

// NewServiceWithDB is for test injection.
func NewServiceWithDB(db DB) *Service {
	return &Service{db: db}
}

// Workspace is the service-layer representation of a workspace with the caller's role.
type Workspace struct {
	ID         string
	Name       string
	Slug       string
	Plan       string
	PlanStatus string
	Role       string
}

// Member is a workspace member as seen by other members.
type Member struct {
	AccountID string
	Username  string
	Role      string
	JoinedAt  string
}

// roleRank returns a numeric rank for comparison (higher = more privileged).
func roleRank(r string) int {
	switch r {
	case "owner":
		return 4
	case "admin":
		return 3
	case "member":
		return 2
	case "viewer":
		return 1
	default:
		return 0
	}
}

// CreatePersonalWorkspace creates a new workspace and assigns the account as owner.
// Used during registration to provision the personal workspace.
func CreatePersonalWorkspace(ctx context.Context, q *queries.Queries, accountID string) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	ws, err := q.CreateWorkspace(ctx, queries.CreateWorkspaceParams{
		ID:   id,
		Name: "Private",
		Slug: id,
	})
	if err != nil {
		return "", err
	}
	if err := q.CreateWorkspaceMember(ctx, queries.CreateWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		AccountID:   accountID,
		Role:        "owner",
	}); err != nil {
		return "", err
	}
	return ws.ID, nil
}

// GetPersonalWorkspaceID returns the workspace ID for the account's first owned workspace.
func (s *Service) GetPersonalWorkspaceID(ctx context.Context, accountID string) (string, error) {
	row, err := s.db.GetPersonalWorkspace(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return row.ID, nil
}

// Create creates a new workspace for the account, enforcing the free-plan 1-workspace limit.
// wrappedWorkspaceKey and ephemeralPublicKey are stored in workspace_member_keys for the owner.
func (s *Service) Create(ctx context.Context, accountID, name string, wrappedWorkspaceKey, ephemeralPublicKey []byte) (Workspace, error) {
	count, err := s.db.CountOwnerWorkspaces(ctx, accountID)
	if err != nil {
		return Workspace{}, err
	}
	if count >= 1 {
		return Workspace{}, ErrPlanLimit
	}

	id, err := randomID()
	if err != nil {
		return Workspace{}, err
	}
	ws, err := s.db.CreateWorkspace(ctx, queries.CreateWorkspaceParams{
		ID:   id,
		Name: name,
		Slug: id,
	})
	if err != nil {
		return Workspace{}, err
	}
	if err := s.db.CreateWorkspaceMember(ctx, queries.CreateWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		AccountID:   accountID,
		Role:        "owner",
	}); err != nil {
		return Workspace{}, err
	}
	if err := s.db.UpsertWorkspaceMemberKey(ctx, queries.UpsertWorkspaceMemberKeyParams{
		WorkspaceID:         ws.ID,
		AccountID:           accountID,
		WrappedWorkspaceKey: wrappedWorkspaceKey,
		EphemeralPublicKey:  ephemeralPublicKey,
		GrantedByAccountID:  accountID,
	}); err != nil {
		return Workspace{}, err
	}
	return Workspace{
		ID:         ws.ID,
		Name:       ws.Name,
		Slug:       ws.Slug,
		Plan:       ws.Plan,
		PlanStatus: ws.PlanStatus,
		Role:       "owner",
	}, nil
}

// List returns all workspaces the account belongs to, with their role in each.
func (s *Service) List(ctx context.Context, accountID string) ([]Workspace, error) {
	rows, err := s.db.ListWorkspacesByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, len(rows))
	for i, r := range rows {
		out[i] = Workspace{
			ID:         r.ID,
			Name:       r.Name,
			Slug:       r.Slug,
			Plan:       r.Plan,
			PlanStatus: r.PlanStatus,
			Role:       r.Role,
		}
	}
	return out, nil
}

// Get returns a workspace and the caller's role. Returns ErrForbidden if not a member.
func (s *Service) Get(ctx context.Context, workspaceID, accountID string) (Workspace, error) {
	ws, err := s.db.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	member, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrForbidden
		}
		return Workspace{}, err
	}
	return Workspace{
		ID:         ws.ID,
		Name:       ws.Name,
		Slug:       ws.Slug,
		Plan:       ws.Plan,
		PlanStatus: ws.PlanStatus,
		Role:       member.Role,
	}, nil
}

// Rename updates the workspace name. Requires owner or admin role.
func (s *Service) Rename(ctx context.Context, workspaceID, accountID, name string) error {
	member, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if roleRank(member.Role) < roleRank("admin") {
		return ErrForbidden
	}
	return s.db.RenameWorkspace(ctx, queries.RenameWorkspaceParams{
		ID:   workspaceID,
		Name: name,
	})
}

// Delete deletes a workspace. Requires owner role and no remaining non-owner members.
func (s *Service) Delete(ctx context.Context, workspaceID, accountID string) error {
	member, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if member.Role != "owner" {
		return ErrForbidden
	}
	count, err := s.db.CountNonOwnerMembers(ctx, workspaceID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrHasMembers
	}
	return s.db.DeleteWorkspace(ctx, workspaceID)
}

// ListMembers returns all members of a workspace. Requires membership.
func (s *Service) ListMembers(ctx context.Context, workspaceID, accountID string) ([]Member, error) {
	_, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	rows, err := s.db.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		out[i] = Member{
			AccountID: r.AccountID,
			Username:  r.Username.String,
			Role:      r.Role,
			JoinedAt:  r.JoinedAt.Time.Format("2006-01-02"),
		}
	}
	return out, nil
}

// UpdateMemberRole changes a member's role. Requires owner or admin. Cannot promote
// above caller's own role. Cannot demote the sole owner.
func (s *Service) UpdateMemberRole(ctx context.Context, workspaceID, callerAccountID, targetAccountID, newRole string) error {
	caller, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   callerAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if roleRank(caller.Role) < roleRank("admin") {
		return ErrForbidden
	}
	// Cannot promote to a role higher than the caller's own.
	if roleRank(newRole) > roleRank(caller.Role) {
		return ErrForbidden
	}

	target, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	// Protect the sole owner.
	if target.Role == "owner" && newRole != "owner" {
		ownerCount, err := s.db.CountWorkspaceOwners(ctx, workspaceID)
		if err != nil {
			return err
		}
		if ownerCount <= 1 {
			return ErrLastOwner
		}
	}
	return s.db.UpdateWorkspaceMemberRole(ctx, queries.UpdateWorkspaceMemberRoleParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
		Role:        newRole,
	})
}

// RemoveMember removes a member from a workspace and deletes their workspace key.
// Requires owner or admin. Cannot remove the sole owner.
func (s *Service) RemoveMember(ctx context.Context, workspaceID, callerAccountID, targetAccountID string) error {
	caller, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   callerAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if roleRank(caller.Role) < roleRank("admin") {
		return ErrForbidden
	}

	target, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if target.Role == "owner" {
		ownerCount, err := s.db.CountWorkspaceOwners(ctx, workspaceID)
		if err != nil {
			return err
		}
		if ownerCount <= 1 {
			return ErrLastOwner
		}
	}

	if err := s.db.DeleteWorkspaceMemberKey(ctx, queries.DeleteWorkspaceMemberKeyParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
	}); err != nil {
		return err
	}
	return s.db.DeleteWorkspaceMember(ctx, queries.DeleteWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
	})
}

// ─── Phase 5: Collaborative Key Distribution ──────────────────────────────────

// MemberIdentityKey holds a member's account ID and raw identity public key.
type MemberIdentityKey struct {
	AccountID         string
	IdentityPublicKey []byte
}

// PendingGrant is a member who has no workspace key entry yet.
type PendingGrant struct {
	AccountID string
	Username  string
}

// MemberKey holds the caller's wrapped workspace key material.
type MemberKey struct {
	WrappedWorkspaceKey []byte
	EphemeralPublicKey  []byte
}

// ListMemberIdentityKeys returns the identity public key for every member of the
// workspace. Requires membership (viewer or above).
func (s *Service) ListMemberIdentityKeys(ctx context.Context, workspaceID, accountID string) ([]MemberIdentityKey, error) {
	_, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	rows, err := s.db.ListMemberIdentityKeys(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberIdentityKey, len(rows))
	for i, r := range rows {
		out[i] = MemberIdentityKey{
			AccountID:         r.AccountID,
			IdentityPublicKey: r.IdentityPublicKey,
		}
	}
	return out, nil
}

// GetMyKey returns the caller's wrapped workspace key entry.
// Returns ErrNotFound if the key has not been granted yet.
func (s *Service) GetMyKey(ctx context.Context, workspaceID, accountID string) (MemberKey, error) {
	_, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberKey{}, ErrForbidden
		}
		return MemberKey{}, err
	}
	row, err := s.db.GetWorkspaceMemberKey(ctx, queries.GetWorkspaceMemberKeyParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberKey{}, ErrNotFound
		}
		return MemberKey{}, err
	}
	return MemberKey{
		WrappedWorkspaceKey: row.WrappedWorkspaceKey,
		EphemeralPublicKey:  row.EphemeralPublicKey,
	}, nil
}

// GrantMemberKey upserts a wrapped workspace key for a target member.
// Requires owner or admin role. The caller must be a member of the workspace.
func (s *Service) GrantMemberKey(ctx context.Context, workspaceID, callerAccountID, targetAccountID string, wrappedKey, ephemeralPub []byte) error {
	caller, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   callerAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if roleRank(caller.Role) < roleRank("admin") {
		return ErrForbidden
	}
	// Ensure the target is actually a member.
	_, err = s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return s.db.UpsertWorkspaceMemberKey(ctx, queries.UpsertWorkspaceMemberKeyParams{
		WorkspaceID:         workspaceID,
		AccountID:           targetAccountID,
		WrappedWorkspaceKey: wrappedKey,
		EphemeralPublicKey:  ephemeralPub,
		GrantedByAccountID:  callerAccountID,
	})
}

// PendingKeyGrants returns members who have no workspace_member_keys entry.
// Requires owner or admin role (only they need to act on the result).
func (s *Service) PendingKeyGrants(ctx context.Context, workspaceID, accountID string) ([]PendingGrant, error) {
	caller, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	if roleRank(caller.Role) < roleRank("admin") {
		return nil, ErrForbidden
	}
	rows, err := s.db.GetMembersWithoutWorkspaceKeyWithUsername(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]PendingGrant, len(rows))
	for i, r := range rows {
		out[i] = PendingGrant{
			AccountID: r.AccountID,
			Username:  r.Username.String,
		}
	}
	return out, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
