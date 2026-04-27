package forms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/internal/db/queries"
)

var ErrNotFound = errors.New("form not found")

// DB is the subset of queries.Queries used by forms.Service.
type DB interface {
	CreateForm(ctx context.Context, arg queries.CreateFormParams) (queries.Form, error)
	GetFormWorkspaceID(ctx context.Context, id string) (string, error)
	GetFormByWorkspace(ctx context.Context, arg queries.GetFormByWorkspaceParams) (queries.Form, error)
	GetFormPublic(ctx context.Context, id string) (queries.GetFormPublicRow, error)
	ListFormsByWorkspace(ctx context.Context, workspaceID string) ([]queries.ListFormsByWorkspaceRow, error)
	UpdateFormSchema(ctx context.Context, arg queries.UpdateFormSchemaParams) (int32, error)
	PublishForm(ctx context.Context, arg queries.PublishFormParams) error
	UpdateFormStatus(ctx context.Context, arg queries.UpdateFormStatusParams) error
	UpdateFormExpiration(ctx context.Context, arg queries.UpdateFormExpirationParams) error
	DeleteForm(ctx context.Context, arg queries.DeleteFormParams) error
	InsertSchemaVersion(ctx context.Context, arg queries.InsertSchemaVersionParams) error
	GetSchemaVersion(ctx context.Context, arg queries.GetSchemaVersionParams) ([]byte, error)
	SetWorkspaceFormKey(ctx context.Context, arg queries.SetWorkspaceFormKeyParams) error
	UpdateFormPGPNotification(ctx context.Context, arg queries.UpdateFormPGPNotificationParams) error
}

// Service handles form CRUD operations.
type Service struct {
	log  zerolog.Logger
	db   DB
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		log:  log.With().Str("module", "forms").Logger(),
		db:   queries.New(pool),
		pool: pool,
	}
}

// FormRecord is the full form including encrypted blobs, returned to the owner.
type FormRecord struct {
	ID                      string
	Status                  string
	SchemaVersion           int32
	ResponseCount           int32
	CreatedAt               pgtype.Timestamptz
	UpdatedAt               pgtype.Timestamptz
	EncryptedSchema         []byte
	RenderEncryptedSchema   []byte
	PublicFormKey           []byte
	RenderKeySalt           []byte
	ExpiresAt               pgtype.Date
	ResponseLimit           pgtype.Int4
	ResponseTtlDays         pgtype.Int4
	BurnAfterReading        bool
	WorkspaceWrappedFormKey []byte // nil if not yet set
	HasUnpublishedChanges   bool
	NotificationEmail       string // empty if not configured
	PGPPublicKey            string // ASCII-armored PGP public key; empty if not configured
	NotificationFrom        string // optional from address override
	NotificationSubject     string // optional subject override
}

// FormSummary is a list-view row — no schema blobs.
type FormSummary struct {
	ID                    string
	Status                string
	SchemaVersion         int32
	ResponseCount         int32
	CreatedAt             pgtype.Timestamptz
	UpdatedAt             pgtype.Timestamptz
	ExpiresAt             pgtype.Date
	ResponseLimit         pgtype.Int4
	ResponseTtlDays       pgtype.Int4
	BurnAfterReading      bool
	HasUnpublishedChanges bool
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
	WorkspaceID           string
	PGPPublicKey          string // ASCII-armored PGP public key; empty if not configured
}

// CreateForm stores a new form and returns its ID.
// If clientID is non-empty it is used as the form ID; otherwise a random ID is generated.
// Both the form row and version 1 snapshot are inserted in a single transaction.
func (s *Service) CreateForm(ctx context.Context, workspaceID, createdByAccountID, clientID string, encryptedSchema, renderEncryptedSchema, publicFormKey, renderKeySalt, workspaceWrappedFormKey []byte, expiresAt pgtype.Date, responseLimit pgtype.Int4, responseTtlDays pgtype.Int4, burnAfterReading bool) (string, error) {
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
		ID:                      id,
		WorkspaceID:             workspaceID,
		CreatedByAccountID:      pgtype.Text{String: createdByAccountID, Valid: true},
		EncryptedSchema:         encryptedSchema,
		RenderEncryptedSchema:   renderEncryptedSchema,
		PublicFormKey:           publicFormKey,
		RenderKeySalt:           renderKeySalt,
		ExpiresAt:               expiresAt,
		ResponseLimit:           responseLimit,
		ResponseTtlDays:         responseTtlDays,
		BurnAfterReading:        burnAfterReading,
		WorkspaceWrappedFormKey: workspaceWrappedFormKey,
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

// UpdateExpiration sets the sunset date, response cap, and/or response TTL policy for a form.
func (s *Service) UpdateExpiration(ctx context.Context, workspaceID, formID string, expiresAt pgtype.Date, responseLimit pgtype.Int4, responseTtlDays pgtype.Int4, burnAfterReading bool) error {
	return s.db.UpdateFormExpiration(ctx, queries.UpdateFormExpirationParams{
		ID:               formID,
		WorkspaceID:      workspaceID,
		ExpiresAt:        expiresAt,
		ResponseLimit:    responseLimit,
		ResponseTtlDays:  responseTtlDays,
		BurnAfterReading: burnAfterReading,
	})
}

// GetFormWorkspace returns the workspace ID that owns the given form, or ErrNotFound.
func (s *Service) GetFormWorkspace(ctx context.Context, formID string) (string, error) {
	wsID, err := s.db.GetFormWorkspaceID(ctx, formID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return wsID, nil
}

// GetForm returns the full form record for the workspace. Returns ErrNotFound if
// the form does not exist or belongs to a different workspace.
func (s *Service) GetForm(ctx context.Context, workspaceID, formID string) (FormRecord, error) {
	row, err := s.db.GetFormByWorkspace(ctx, queries.GetFormByWorkspaceParams{
		ID:          formID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FormRecord{}, ErrNotFound
		}
		return FormRecord{}, err
	}
	return formRecordFromDB(row), nil
}

// ListForms returns summary rows for all forms in the workspace.
func (s *Service) ListForms(ctx context.Context, workspaceID string) ([]FormSummary, error) {
	rows, err := s.db.ListFormsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]FormSummary, len(rows))
	for i, r := range rows {
		out[i] = FormSummary{
			ID:                    r.ID,
			Status:                r.Status,
			SchemaVersion:         r.SchemaVersion,
			ResponseCount:         r.ResponseCount,
			CreatedAt:             r.CreatedAt,
			UpdatedAt:             r.UpdatedAt,
			ExpiresAt:             r.ExpiresAt,
			ResponseLimit:         r.ResponseLimit,
			ResponseTtlDays:       r.ResponseTtlDays,
			BurnAfterReading:      r.BurnAfterReading,
			HasUnpublishedChanges: r.HasUnpublishedChanges,
		}
	}
	return out, nil
}

// UpdateFormSchema replaces the owner-encrypted schema and bumps schema_version.
// This is called on every auto-save and marks the form as having unpublished changes.
// The render-encrypted schema is NOT updated here — that only happens via PublishForm.
func (s *Service) UpdateFormSchema(ctx context.Context, workspaceID, formID string, encryptedSchema []byte) (int32, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := queries.New(tx)

	newVersion, err := qtx.UpdateFormSchema(ctx, queries.UpdateFormSchemaParams{
		ID:              formID,
		WorkspaceID:     workspaceID,
		EncryptedSchema: encryptedSchema,
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

// PublishForm updates the render-encrypted schema, sets status to 'open',
// and clears the has_unpublished_changes flag atomically.
func (s *Service) PublishForm(ctx context.Context, workspaceID, formID string, renderEncryptedSchema, renderKeySalt []byte) error {
	return s.db.PublishForm(ctx, queries.PublishFormParams{
		ID:                    formID,
		WorkspaceID:           workspaceID,
		RenderEncryptedSchema: renderEncryptedSchema,
		RenderKeySalt:         renderKeySalt,
	})
}

// UpdateFormStatus sets status to "open" or "closed".
// Draft forms cannot be set to 'open' via this method — use PublishForm instead.
func (s *Service) UpdateFormStatus(ctx context.Context, workspaceID, formID, status string) error {
	if status != "open" && status != "closed" {
		return errors.New("status must be 'open' or 'closed'")
	}
	// Prevent bypassing the publish flow for draft forms.
	if status == "open" {
		form, err := s.db.GetFormByWorkspace(ctx, queries.GetFormByWorkspaceParams{
			ID: formID, WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if form.Status == "draft" {
			return errors.New("draft forms must be published before they can be opened")
		}
	}
	return s.db.UpdateFormStatus(ctx, queries.UpdateFormStatusParams{
		ID:          formID,
		WorkspaceID: workspaceID,
		Status:      status,
	})
}

// DeleteForm hard-deletes the form and all its responses (via CASCADE).
func (s *Service) DeleteForm(ctx context.Context, workspaceID, formID string) error {
	return s.db.DeleteForm(ctx, queries.DeleteFormParams{
		ID:          formID,
		WorkspaceID: workspaceID,
	})
}

// GetPublicSchema returns render-encrypted schema for unauthenticated respondents.
func (s *Service) GetPublicSchema(ctx context.Context, formID string) (PublicFormRecord, error) {
	row, err := s.db.GetFormPublic(ctx, formID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicFormRecord{}, ErrNotFound
		}
		return PublicFormRecord{}, err
	}
	rec := PublicFormRecord{
		ID:                    row.ID,
		Status:                row.Status,
		SchemaVersion:         row.SchemaVersion,
		ResponseCount:         row.ResponseCount,
		RenderEncryptedSchema: row.RenderEncryptedSchema,
		PublicFormKey:         row.PublicFormKey,
		ExpiresAt:             row.ExpiresAt,
		ResponseLimit:         row.ResponseLimit,
		WorkspaceID:           row.WorkspaceID,
	}
	if row.PgpPublicKey.Valid {
		rec.PGPPublicKey = row.PgpPublicKey.String
	}
	return rec, nil
}

// SetWorkspaceFormKey stores the workspace-encrypted form key for a form.
func (s *Service) SetWorkspaceFormKey(ctx context.Context, workspaceID, formID string, wrappedKey []byte) error {
	return s.db.SetWorkspaceFormKey(ctx, queries.SetWorkspaceFormKeyParams{
		ID:                      formID,
		WorkspaceID:             workspaceID,
		WorkspaceWrappedFormKey: wrappedKey,
	})
}

// UpdatePGPNotification sets or clears the email notification destination, PGP public key,
// and optional from/subject overrides. Pass empty strings to clear optional fields.
func (s *Service) UpdatePGPNotification(ctx context.Context, workspaceID, formID, notificationEmail, pgpPublicKey, notificationFrom, notificationSubject string) error {
	var email pgtype.Text
	if notificationEmail != "" {
		email = pgtype.Text{String: notificationEmail, Valid: true}
	}
	var pgpKey pgtype.Text
	if pgpPublicKey != "" {
		pgpKey = pgtype.Text{String: pgpPublicKey, Valid: true}
	}
	var from pgtype.Text
	if notificationFrom != "" {
		from = pgtype.Text{String: notificationFrom, Valid: true}
	}
	var subject pgtype.Text
	if notificationSubject != "" {
		subject = pgtype.Text{String: notificationSubject, Valid: true}
	}
	return s.db.UpdateFormPGPNotification(ctx, queries.UpdateFormPGPNotificationParams{
		ID:                  formID,
		WorkspaceID:         workspaceID,
		NotificationEmail:   email,
		PgpPublicKey:        pgpKey,
		NotificationFrom:    from,
		NotificationSubject: subject,
	})
}

// GetSchemaVersion returns the owner-encrypted schema for a specific version snapshot.
func (s *Service) GetSchemaVersion(ctx context.Context, workspaceID, formID string, version int32) ([]byte, error) {
	_, err := s.db.GetFormByWorkspace(ctx, queries.GetFormByWorkspaceParams{
		ID:          formID,
		WorkspaceID: workspaceID,
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
	rec := FormRecord{
		ID:                      f.ID,
		Status:                  f.Status,
		SchemaVersion:           f.SchemaVersion,
		ResponseCount:           f.ResponseCount,
		CreatedAt:               f.CreatedAt,
		UpdatedAt:               f.UpdatedAt,
		EncryptedSchema:         f.EncryptedSchema,
		RenderEncryptedSchema:   f.RenderEncryptedSchema,
		PublicFormKey:           f.PublicFormKey,
		RenderKeySalt:           f.RenderKeySalt,
		ExpiresAt:               f.ExpiresAt,
		ResponseLimit:           f.ResponseLimit,
		ResponseTtlDays:         f.ResponseTtlDays,
		BurnAfterReading:        f.BurnAfterReading,
		WorkspaceWrappedFormKey: f.WorkspaceWrappedFormKey,
		HasUnpublishedChanges:   f.HasUnpublishedChanges,
	}
	if f.NotificationEmail.Valid {
		rec.NotificationEmail = f.NotificationEmail.String
	}
	if f.PgpPublicKey.Valid {
		rec.PGPPublicKey = f.PgpPublicKey.String
	}
	if f.NotificationFrom.Valid {
		rec.NotificationFrom = f.NotificationFrom.String
	}
	if f.NotificationSubject.Valid {
		rec.NotificationSubject = f.NotificationSubject.String
	}
	return rec
}
