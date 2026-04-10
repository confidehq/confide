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

var ErrNotFound = errors.New("workspace not found")

// DB is the subset of queries.Queries used by workspace.Service.
type DB interface {
	CreateWorkspace(ctx context.Context, arg queries.CreateWorkspaceParams) (queries.Workspace, error)
	CreateWorkspaceMember(ctx context.Context, arg queries.CreateWorkspaceMemberParams) error
	GetPersonalWorkspace(ctx context.Context, accountID string) (queries.GetPersonalWorkspaceRow, error)
	GetWorkspaceByID(ctx context.Context, id string) (queries.GetWorkspaceByIDRow, error)
	GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error)
	CountOwnerWorkspaces(ctx context.Context, accountID string) (int64, error)
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

// Workspace is the service-layer representation of a workspace.
type Workspace struct {
	ID   string
	Name string
	Slug string
	Plan string
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

// GetPersonalWorkspaceID returns the workspace ID for the account's personal workspace.
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

// ─── Helpers ──────────────────────────────────────────────────────────────────

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
