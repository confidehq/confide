package workspace

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
	domainpkg "github.com/phantompunk/confide/internal/domain"
	"github.com/phantompunk/confide/internal/permission"
)

var (
	ErrNotFound      = errors.New("workspace not found")
	ErrForbidden     = errors.New("insufficient role")
	ErrPlanLimit     = errors.New("workspace limit reached for free plan")
	ErrLastOwner     = errors.New("cannot remove or demote the sole owner")
	ErrHasMembers    = errors.New("workspace still has non-owner members")
	ErrInvalidDomain = errors.New("invalid domain: must be a plain hostname with no scheme or path")
)

// SubscriptionCanceler cancels a Stripe subscription. Implemented by billing.Service.
type SubscriptionCanceler interface {
	CancelSubscription(ctx context.Context, subscriptionID string) error
}

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
	GetWorkspaceForBilling(ctx context.Context, id string) (queries.GetWorkspaceForBillingRow, error)

	InsertCustomDomain(ctx context.Context, arg queries.InsertCustomDomainParams) (queries.CustomDomain, error)
	GetCustomDomainByWorkspace(ctx context.Context, workspaceID string) (queries.CustomDomain, error)
	GetCustomDomainByHost(ctx context.Context, domain string) (queries.CustomDomain, error)
	DeleteCustomDomain(ctx context.Context, workspaceID string) error
	ListAllEnabledDomains(ctx context.Context) ([]string, error)
}

type Service struct {
	log              zerolog.Logger
	db               DB
	pool             *pgxpool.Pool
	cache            *permission.RoleCache
	registry         domainRegistry
	checker          domainChecker
	canceler         SubscriptionCanceler
	traefikConfigDir string
}

type domainRegistry interface {
	IsEnabled(domain string) bool
	Enable(domain string)
	Disable(domain string)
}

// domainChecker performs an on-demand DNS verification pass.
type domainChecker interface {
	CheckNow(ctx context.Context, cd queries.CustomDomain) (cnameOK, txtOK bool)
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		log:   log.With().Str("module", "workspace").Logger(),
		db:    queries.New(pool),
		pool:  pool,
		cache: permission.NewRoleCache(),
	}
}

// NewServiceWithDB is for test injection.
func NewServiceWithDB(db DB) *Service {
	return &Service{db: db}
}

// WithRegistry attaches the domain registry to the service so domain changes
// are reflected in the in-memory routing table immediately.
func (s *Service) WithRegistry(r domainRegistry) {
	s.registry = r
}

// WithDomainChecker attaches the worker so the /verify endpoint can trigger
// an on-demand DNS check.
func (s *Service) WithDomainChecker(c domainChecker) {
	s.checker = c
}

// WithSubscriptionCanceler attaches a billing canceler used during workspace deletion.
func (s *Service) WithSubscriptionCanceler(c SubscriptionCanceler) {
	s.canceler = c
}

// WithTraefikConfigDir enables deleting per-domain Traefik config files when a
// custom domain is replaced or removed.
func (s *Service) WithTraefikConfigDir(dir string) {
	s.traefikConfigDir = dir
}

// Cache returns the shared role cache. Used by server to wire middleware.
func (s *Service) Cache() *permission.RoleCache {
	return s.cache
}

// GetWorkspaceMember satisfies permission.MemberResolver for use in middleware.
func (s *Service) GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error) {
	return s.db.GetWorkspaceMember(ctx, arg)
}

// Workspace is the service-layer representation of a workspace with the caller's role.
type Workspace struct {
	ID         string
	Name       string
	Slug       string
	Plan       string
	PlanStatus string
	Role       string
	Status     string // "active" | "pending"
}

// Member is a workspace member as seen by other members.
type Member struct {
	AccountID string
	Username  string
	Role      string
	JoinedAt  string
	Status    string // "active" | "pending"
	LastSeen  string // ISO date, empty if never logged in
}

// invalidate removes a role cache entry if the cache is available.
func (s *Service) invalidate(workspaceID, accountID string) {
	if s.cache != nil {
		s.cache.Invalidate(workspaceID, accountID)
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
		Name: "Workspace",
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

// ValidateMember returns nil if accountID is a member of workspaceID, ErrForbidden if not.
// Used by forms routes which cannot use the workspace role middleware.
func (s *Service) ValidateMember(ctx context.Context, workspaceID, accountID string) error {
	_, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	return nil
}

// ValidateActiveMember returns nil if accountID is an active (key-granted) member of
// workspaceID. Returns ErrForbidden for non-members and pending members awaiting key grant.
func (s *Service) ValidateActiveMember(ctx context.Context, workspaceID, accountID string) error {
	if err := s.ValidateMember(ctx, workspaceID, accountID); err != nil {
		return err
	}
	_, err := s.db.GetWorkspaceMemberKey(ctx, queries.GetWorkspaceMemberKeyParams{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	return nil
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
func (s *Service) Create(ctx context.Context, accountID, name string) (Workspace, error) {
	count, err := s.db.CountOwnerWorkspaces(ctx, accountID)
	if err != nil {
		return Workspace{}, err
	}
	if count >= 5 {
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
			Status:     r.Status,
		}
	}
	return out, nil
}

// Get returns a workspace. callerRole is pre-resolved by middleware and used
// to populate the role field in the response.
func (s *Service) Get(ctx context.Context, workspaceID, callerRole string) (Workspace, error) {
	ws, err := s.db.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	return Workspace{
		ID:         ws.ID,
		Name:       ws.Name,
		Slug:       ws.Slug,
		Plan:       ws.Plan,
		PlanStatus: ws.PlanStatus,
		Role:       callerRole,
	}, nil
}

// Rename updates the workspace name. Caller role is enforced by middleware.
func (s *Service) Rename(ctx context.Context, workspaceID, name string) error {
	return s.db.RenameWorkspace(ctx, queries.RenameWorkspaceParams{
		ID:   workspaceID,
		Name: name,
	})
}

// Delete deletes a workspace. Caller role is enforced by middleware.
// Fails if any non-owner members remain. Cancels any active Stripe subscription first.
func (s *Service) Delete(ctx context.Context, workspaceID string) error {
	count, err := s.db.CountNonOwnerMembers(ctx, workspaceID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrHasMembers
	}
	if s.canceler != nil {
		ws, err := s.db.GetWorkspaceForBilling(ctx, workspaceID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && ws.StripeSubscriptionID.Valid {
			if err := s.canceler.CancelSubscription(ctx, ws.StripeSubscriptionID.String); err != nil {
				return err
			}
		}
	}
	return s.db.DeleteWorkspace(ctx, workspaceID)
}

// ListMembers returns all members of a workspace. Membership is guaranteed by middleware.
func (s *Service) ListMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	rows, err := s.db.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		lastSeen := ""
		switch ts := r.LastSeen.(type) {
		case pgtype.Timestamptz:
			if ts.Valid {
				lastSeen = ts.Time.UTC().Format(time.RFC3339)
			}
		case time.Time:
			if !ts.IsZero() {
				lastSeen = ts.UTC().Format(time.RFC3339)
			}
		}
		out[i] = Member{
			AccountID: r.AccountID,
			Username:  r.Username.String,
			Role:      r.Role,
			JoinedAt:  r.JoinedAt.Time.UTC().Format(time.RFC3339),
			Status:    r.Status,
			LastSeen:  lastSeen,
		}
	}
	return out, nil
}

// UpdateMemberRole changes a member's role. callerRole is pre-resolved by middleware.
// Cannot promote to a role higher than the caller's own. Cannot demote the sole owner.
func (s *Service) UpdateMemberRole(ctx context.Context, workspaceID, callerRole, targetAccountID, newRole string) error {
	if !permission.Can(callerRole, permission.ActionChangeRoles) {
		return ErrForbidden
	}
	if permission.RoleRank(newRole) > permission.RoleRank(callerRole) {
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
	if target.Role == "owner" && newRole != "owner" {
		ownerCount, err := s.db.CountWorkspaceOwners(ctx, workspaceID)
		if err != nil {
			return err
		}
		if ownerCount <= 1 {
			return ErrLastOwner
		}
	}
	if err := s.db.UpdateWorkspaceMemberRole(ctx, queries.UpdateWorkspaceMemberRoleParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
		Role:        newRole,
	}); err != nil {
		return err
	}
	s.invalidate(workspaceID, targetAccountID)
	return nil
}

// RemoveMember removes a member from a workspace and deletes their workspace key.
// Caller role is enforced by middleware. Cannot remove the sole owner.
func (s *Service) RemoveMember(ctx context.Context, workspaceID, targetAccountID string) error {
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
	if err := s.db.DeleteWorkspaceMember(ctx, queries.DeleteWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		AccountID:   targetAccountID,
	}); err != nil {
		return err
	}
	s.invalidate(workspaceID, targetAccountID)
	return nil
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

// ListMemberIdentityKeys returns the identity public key for every workspace member.
// Membership is guaranteed by middleware.
func (s *Service) ListMemberIdentityKeys(ctx context.Context, workspaceID string) ([]MemberIdentityKey, error) {
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
// Caller role is enforced by middleware.
func (s *Service) GrantMemberKey(ctx context.Context, workspaceID, callerAccountID, targetAccountID string, wrappedKey, ephemeralPub []byte) error {
	_, err := s.db.GetWorkspaceMember(ctx, queries.GetWorkspaceMemberParams{
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
		GrantedByAccountID:  pgtype.Text{String: callerAccountID, Valid: true},
	})
}

// PendingKeyGrants returns members who have no workspace_member_keys entry.
// Caller role is enforced by middleware.
func (s *Service) PendingKeyGrants(ctx context.Context, workspaceID string) ([]PendingGrant, error) {
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

// ─── Custom Domains ───────────────────────────────────────────────────────────

// CustomDomainInfo holds the workspace's current custom domain configuration.
type CustomDomainInfo struct {
	ID       string
	Domain   string // empty string when no domain is configured
	TxtToken string
	CnameOK  bool
	TxtOK    bool
	Enabled  bool
}

// SetCustomDomain stores a new custom domain for the workspace, replacing any
// existing one. Generates a fresh TXT verification token.
// Returns UpgradeRequiredError if the workspace is on the free plan.
// Returns ErrInvalidDomain if the value is not a plain hostname.
func (s *Service) SetCustomDomain(ctx context.Context, workspaceID, plan, domain string) (CustomDomainInfo, error) {
	if !permission.PlanAllows(plan, permission.FeatureCustomDomains) {
		return CustomDomainInfo{}, &permission.UpgradeRequiredError{Feature: permission.FeatureCustomDomains}
	}
	if !isValidHostname(domain) {
		return CustomDomainInfo{}, ErrInvalidDomain
	}

	// Disable previous domain in registry and remove its Traefik config if any.
	if prev, err := s.db.GetCustomDomainByWorkspace(ctx, workspaceID); err == nil {
		if s.registry != nil {
			s.registry.Disable(prev.Domain)
		}
		if s.traefikConfigDir != "" {
			if err := domainpkg.DeleteTraefikConfig(s.traefikConfigDir, prev.Domain); err != nil {
				log.Error().Err(err).Str("domain", prev.Domain).Msg("workspace: delete traefik config on domain change")
			}
		}
	}

	token, err := randomToken()
	if err != nil {
		return CustomDomainInfo{}, err
	}
	cd, err := s.db.InsertCustomDomain(ctx, queries.InsertCustomDomainParams{
		WorkspaceID: workspaceID,
		Domain:      domain,
		TxtToken:    token,
	})
	if err != nil {
		return CustomDomainInfo{}, err
	}
	return domainInfoFrom(cd), nil
}

// ClearCustomDomain removes the custom domain from the workspace.
func (s *Service) ClearCustomDomain(ctx context.Context, workspaceID string) error {
	if prev, err := s.db.GetCustomDomainByWorkspace(ctx, workspaceID); err == nil {
		if s.registry != nil {
			s.registry.Disable(prev.Domain)
		}
		if s.traefikConfigDir != "" {
			if err := domainpkg.DeleteTraefikConfig(s.traefikConfigDir, prev.Domain); err != nil {
				log.Error().Err(err).Str("domain", prev.Domain).Msg("workspace: delete traefik config on domain clear")
			}
		}
	}
	return s.db.DeleteCustomDomain(ctx, workspaceID)
}

// GetCustomDomain returns the current custom domain configuration for a workspace.
// Returns an empty CustomDomainInfo (Domain == "") when none is set.
func (s *Service) GetCustomDomain(ctx context.Context, workspaceID string) (CustomDomainInfo, error) {
	cd, err := s.db.GetCustomDomainByWorkspace(ctx, workspaceID)
	if err != nil {
		if isNotFound(err) {
			return CustomDomainInfo{}, nil
		}
		return CustomDomainInfo{}, err
	}
	return domainInfoFrom(cd), nil
}

// CheckCustomDomain triggers an immediate DNS verification pass for the
// workspace's custom domain and returns the refreshed status.
func (s *Service) CheckCustomDomain(ctx context.Context, workspaceID string) (CustomDomainInfo, error) {
	cd, err := s.db.GetCustomDomainByWorkspace(ctx, workspaceID)
	if err != nil {
		if isNotFound(err) {
			return CustomDomainInfo{}, ErrNotFound
		}
		return CustomDomainInfo{}, err
	}
	if s.checker != nil && !cd.Enabled {
		cnameOK, txtOK := s.checker.CheckNow(ctx, cd)
		cd.CnameOk = cnameOK
		cd.TxtOk = txtOK
		cd.Enabled = cnameOK && txtOK
	}
	return domainInfoFrom(cd), nil
}

// GetWorkspaceIDByCustomDomain returns the workspace ID that owns the given
// enabled custom domain, or an error if not found.
func (s *Service) GetWorkspaceIDByCustomDomain(ctx context.Context, domain string) (string, error) {
	cd, err := s.db.GetCustomDomainByHost(ctx, domain)
	if err != nil {
		return "", err
	}
	if !cd.Enabled {
		return "", ErrNotFound
	}
	return cd.WorkspaceID, nil
}

// IsEnabledCustomDomain reports whether hostname is an enabled custom domain.
// Uses the in-memory registry (zero DB hits).
func (s *Service) IsEnabledCustomDomain(hostname string) bool {
	if s.registry != nil {
		return s.registry.IsEnabled(hostname)
	}
	return false
}

func domainInfoFrom(cd queries.CustomDomain) CustomDomainInfo {
	return CustomDomainInfo{
		ID:       cd.ID,
		Domain:   cd.Domain,
		TxtToken: cd.TxtToken,
		CnameOK:  cd.CnameOk,
		TxtOK:    cd.TxtOk,
		Enabled:  cd.Enabled,
	}
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isValidHostname returns true if s is a plain DNS hostname with no scheme, path, or port.
func isValidHostname(s string) bool {
	if s == "" || strings.ContainsAny(s, "/:? ") {
		return false
	}
	// net.LookupHost accepts IPs too; use ParseIP to reject bare IPs if desired,
	// but mostly we just need a syntactically valid label string.
	host, port, err := net.SplitHostPort(s)
	if err == nil {
		// s contained a port — reject
		_ = host
		_ = port
		return false
	}
	// Validate each label.
	for _, label := range strings.Split(s, ".") {
		if label == "" {
			return false
		}
	}
	return true
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
