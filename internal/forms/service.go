package forms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/wisp/internal/db/queries"
)

var ErrNotFound = errors.New("form not found")

// DB is the subset of queries.Queries used by forms.Service.
type DB interface {
	CreateForm(ctx context.Context, arg queries.CreateFormParams) (queries.Form, error)
	GetFormByOwner(ctx context.Context, arg queries.GetFormByOwnerParams) (queries.Form, error)
	GetFormPublic(ctx context.Context, id string) (queries.GetFormPublicRow, error)
	ListFormsByAccount(ctx context.Context, accountID string) ([]queries.ListFormsByAccountRow, error)
	UpdateFormSchema(ctx context.Context, arg queries.UpdateFormSchemaParams) (int32, error)
	UpdateFormStatus(ctx context.Context, arg queries.UpdateFormStatusParams) error
	DeleteForm(ctx context.Context, arg queries.DeleteFormParams) error
}

// Service handles form CRUD operations.
type Service struct {
	db   DB
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: queries.New(pool), pool: pool}
}

// FormRecord is the full form including encrypted blobs, returned to the owner.
type FormRecord struct {
	ID                    string
	Status                string
	SchemaVersion         int32
	ResponseCount         int32
	CreatedAt             pgtype.Date
	UpdatedAt             pgtype.Date
	EncryptedSchema       []byte
	RenderEncryptedSchema []byte
	PublicFormKey         []byte
}

// FormSummary is a list-view row — no schema blobs.
type FormSummary struct {
	ID            string
	Status        string
	SchemaVersion int32
	ResponseCount int32
	CreatedAt     pgtype.Date
	UpdatedAt     pgtype.Date
}

// PublicFormRecord is returned to unauthenticated respondents.
type PublicFormRecord struct {
	ID                    string
	Status                string
	SchemaVersion         int32
	RenderEncryptedSchema []byte
	PublicFormKey         []byte
}

// CreateForm stores a new form and returns its ID.
// If clientID is non-empty it is used as the form ID (client-proposed, for key derivation);
// otherwise a random ID is generated server-side.
func (s *Service) CreateForm(ctx context.Context, accountID, clientID string, encryptedSchema, renderEncryptedSchema, publicFormKey []byte) (string, error) {
	id := clientID
	if id == "" {
		var err error
		id, err = randomID()
		if err != nil {
			return "", err
		}
	}
	_, err := s.db.CreateForm(ctx, queries.CreateFormParams{
		ID:                    id,
		AccountID:             accountID,
		EncryptedSchema:       encryptedSchema,
		RenderEncryptedSchema: renderEncryptedSchema,
		PublicFormKey:         publicFormKey,
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetForm returns the full form record for the owner. Returns ErrNotFound if
// the form does not exist or belongs to a different account.
func (s *Service) GetForm(ctx context.Context, accountID, formID string) (FormRecord, error) {
	row, err := s.db.GetFormByOwner(ctx, queries.GetFormByOwnerParams{
		ID:        formID,
		AccountID: accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FormRecord{}, ErrNotFound
		}
		return FormRecord{}, err
	}
	return formRecordFromDB(row), nil
}

// ListForms returns summary rows for all forms owned by accountID.
func (s *Service) ListForms(ctx context.Context, accountID string) ([]FormSummary, error) {
	rows, err := s.db.ListFormsByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]FormSummary, len(rows))
	for i, r := range rows {
		out[i] = FormSummary{
			ID:            r.ID,
			Status:        r.Status,
			SchemaVersion: r.SchemaVersion,
			ResponseCount: r.ResponseCount,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		}
	}
	return out, nil
}

// UpdateFormSchema replaces the encrypted schema blobs and bumps schema_version.
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) UpdateFormSchema(ctx context.Context, accountID, formID string, encryptedSchema, renderEncryptedSchema []byte) (int32, error) {
	version, err := s.db.UpdateFormSchema(ctx, queries.UpdateFormSchemaParams{
		ID:                    formID,
		AccountID:             accountID,
		EncryptedSchema:       encryptedSchema,
		RenderEncryptedSchema: renderEncryptedSchema,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return version, nil
}

// UpdateFormStatus sets status to "open" or "closed".
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) UpdateFormStatus(ctx context.Context, accountID, formID, status string) error {
	if status != "open" && status != "closed" {
		return errors.New("status must be 'open' or 'closed'")
	}
	return s.db.UpdateFormStatus(ctx, queries.UpdateFormStatusParams{
		ID:        formID,
		AccountID: accountID,
		Status:    status,
	})
}

// DeleteForm hard-deletes the form and all its responses (via CASCADE).
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) DeleteForm(ctx context.Context, accountID, formID string) error {
	return s.db.DeleteForm(ctx, queries.DeleteFormParams{
		ID:        formID,
		AccountID: accountID,
	})
}

// GetPublicSchema returns render-encrypted schema for unauthenticated respondents.
// Returns ErrNotFound if the form does not exist.
func (s *Service) GetPublicSchema(ctx context.Context, formID string) (PublicFormRecord, error) {
	row, err := s.db.GetFormPublic(ctx, formID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicFormRecord{}, ErrNotFound
		}
		return PublicFormRecord{}, err
	}
	return PublicFormRecord{
		ID:                    row.ID,
		Status:                row.Status,
		SchemaVersion:         row.SchemaVersion,
		RenderEncryptedSchema: row.RenderEncryptedSchema,
		PublicFormKey:         row.PublicFormKey,
	}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func formRecordFromDB(f queries.Form) FormRecord {
	return FormRecord{
		ID:                    f.ID,
		Status:                f.Status,
		SchemaVersion:         f.SchemaVersion,
		ResponseCount:         f.ResponseCount,
		CreatedAt:             f.CreatedAt,
		UpdatedAt:             f.UpdatedAt,
		EncryptedSchema:       f.EncryptedSchema,
		RenderEncryptedSchema: f.RenderEncryptedSchema,
		PublicFormKey:         f.PublicFormKey,
	}
}
