package forms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
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
	UpdateFormExpiration(ctx context.Context, arg queries.UpdateFormExpirationParams) error
	DeleteForm(ctx context.Context, arg queries.DeleteFormParams) error
	InsertSchemaVersion(ctx context.Context, arg queries.InsertSchemaVersionParams) error
	GetSchemaVersion(ctx context.Context, arg queries.GetSchemaVersionParams) ([]byte, error)
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
	RenderKeySalt         []byte // nil if never published
	ExpiresAt             pgtype.Date
	ResponseLimit         pgtype.Int4
}

// FormSummary is a list-view row — no schema blobs.
type FormSummary struct {
	ID            string
	Status        string
	SchemaVersion int32
	ResponseCount int32
	CreatedAt     pgtype.Date
	UpdatedAt     pgtype.Date
	ExpiresAt     pgtype.Date
	ResponseLimit pgtype.Int4
}

// PublicFormRecord is returned to unauthenticated respondents.
type PublicFormRecord struct {
	ID                    string
	Status                string
	SchemaVersion         int32
	ResponseCount         int32
	RenderEncryptedSchema []byte
	PublicFormKey         []byte
	ExpiresAt             pgtype.Date
	ResponseLimit         pgtype.Int4
}

// CreateForm stores a new form and returns its ID.
// If clientID is non-empty it is used as the form ID (client-proposed, for key derivation);
// otherwise a random ID is generated server-side.
// Both the form row and version 1 snapshot are inserted in a single transaction.
func (s *Service) CreateForm(ctx context.Context, accountID, clientID string, encryptedSchema, renderEncryptedSchema, publicFormKey, renderKeySalt []byte, expiresAt pgtype.Date, responseLimit pgtype.Int4) (string, error) {
	id := clientID
	if id == "" {
		var err error
		id, err = randomID()
		if err != nil {
			return "", err
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := queries.New(tx)

	_, err = qtx.CreateForm(ctx, queries.CreateFormParams{
		ID:                    id,
		AccountID:             accountID,
		EncryptedSchema:       encryptedSchema,
		RenderEncryptedSchema: renderEncryptedSchema,
		PublicFormKey:         publicFormKey,
		RenderKeySalt:         renderKeySalt,
		ExpiresAt:             expiresAt,
		ResponseLimit:         responseLimit,
	})
	if err != nil {
		return "", err
	}

	if err := qtx.InsertSchemaVersion(ctx, queries.InsertSchemaVersionParams{
		FormID:          id,
		Version:         1,
		EncryptedSchema: encryptedSchema,
	}); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// UpdateExpiration sets the sunset date and/or response cap for a form.
// Passing zero-value pgtype.Date/pgtype.Int4 (Valid=false) clears the respective limit.
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) UpdateExpiration(ctx context.Context, accountID, formID string, expiresAt pgtype.Date, responseLimit pgtype.Int4) error {
	return s.db.UpdateFormExpiration(ctx, queries.UpdateFormExpirationParams{
		ID:            formID,
		AccountID:     accountID,
		ExpiresAt:     expiresAt,
		ResponseLimit: responseLimit,
	})
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
			ExpiresAt:     r.ExpiresAt,
			ResponseLimit: r.ResponseLimit,
		}
	}
	return out, nil
}

// UpdateFormSchema replaces the encrypted schema blobs and bumps schema_version.
// renderKeySalt may be unchanged (regular edit) or new (key rotation) — always stored.
// Both the forms row update and the new schema version snapshot are done in a transaction.
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) UpdateFormSchema(ctx context.Context, accountID, formID string, encryptedSchema, renderEncryptedSchema, renderKeySalt []byte) (int32, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := queries.New(tx)

	newVersion, err := qtx.UpdateFormSchema(ctx, queries.UpdateFormSchemaParams{
		ID:                    formID,
		AccountID:             accountID,
		EncryptedSchema:       encryptedSchema,
		RenderEncryptedSchema: renderEncryptedSchema,
		RenderKeySalt:         renderKeySalt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}

	if err := qtx.InsertSchemaVersion(ctx, queries.InsertSchemaVersionParams{
		FormID:          formID,
		Version:         newVersion,
		EncryptedSchema: encryptedSchema,
	}); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newVersion, nil
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
		ResponseCount:         row.ResponseCount,
		RenderEncryptedSchema: row.RenderEncryptedSchema,
		PublicFormKey:         row.PublicFormKey,
		ExpiresAt:             row.ExpiresAt,
		ResponseLimit:         row.ResponseLimit,
	}, nil
}

// GetSchemaVersion returns the owner-encrypted schema for a specific version snapshot.
// Verifies ownership via GetFormByOwner before fetching the snapshot.
// Returns ErrNotFound if the form or version doesn't exist or isn't owned by accountID.
func (s *Service) GetSchemaVersion(ctx context.Context, accountID, formID string, version int32) ([]byte, error) {
	// Verify ownership first.
	_, err := s.db.GetFormByOwner(ctx, queries.GetFormByOwnerParams{
		ID:        formID,
		AccountID: accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	blob, err := s.db.GetSchemaVersion(ctx, queries.GetSchemaVersionParams{
		FormID:  formID,
		Version: version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return blob, nil
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
		RenderKeySalt:         f.RenderKeySalt,
		ExpiresAt:             f.ExpiresAt,
		ResponseLimit:         f.ResponseLimit,
	}
}
