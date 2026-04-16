package responses

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
	"github.com/phantompunk/confide/internal/relay"
)

var ErrNotFound = errors.New("response not found")

// DB is the subset of queries.Queries used by responses.Service.
type DB interface {
	GetFormByWorkspace(ctx context.Context, arg queries.GetFormByWorkspaceParams) (queries.Form, error)
	ListResponsesFirst(ctx context.Context, arg queries.ListResponsesFirstParams) ([]queries.Response, error)
	ListResponsesAfter(ctx context.Context, arg queries.ListResponsesAfterParams) ([]queries.Response, error)
	GetResponse(ctx context.Context, arg queries.GetResponseParams) (queries.Response, error)
	DeleteResponse(ctx context.Context, arg queries.DeleteResponseParams) error
	InsertResponseWithTTL(ctx context.Context, arg queries.InsertResponseWithTTLParams) error
	MarkResponsesRead(ctx context.Context, arg queries.MarkResponsesReadParams) error
	DeleteExpiredResponses(ctx context.Context) error
	IncrementResponseCount(ctx context.Context, id string) (int32, error)
}

// Service handles response storage and retrieval.
type Service struct {
	db   DB
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: queries.New(pool), pool: pool}
}

// ResponseRecord is a single response as returned to the creator.
type ResponseRecord struct {
	ID                 string
	FormID             string
	ReceivedAt         pgtype.Timestamptz
	SchemaVersion      int32
	EncryptedData      []byte
	EphemeralPublicKey []byte
}

// Cursor encodes a pagination position.
type Cursor struct {
	Date string `json:"d"` // RFC3339
	ID   string `json:"i"`
}

// ListResult is a page of responses with an optional next cursor.
type ListResult struct {
	Responses  []ResponseRecord
	NextCursor *string // nil on last page
}

const defaultPageSize = 50

// ListResponses returns a page of responses for a form in the given workspace.
// Returns ErrNotFound if the form doesn't exist or isn't in the workspace.
func (s *Service) ListResponses(ctx context.Context, workspaceID, formID string, after *string, limit int) (ListResult, error) {
	if _, err := s.db.GetFormByWorkspace(ctx, queries.GetFormByWorkspaceParams{
		ID:          formID,
		WorkspaceID: workspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ListResult{}, ErrNotFound
		}
		return ListResult{}, err
	}

	if limit <= 0 || limit > 200 {
		limit = defaultPageSize
	}

	var rows []queries.Response
	var err error

	if after == nil {
		rows, err = s.db.ListResponsesFirst(ctx, queries.ListResponsesFirstParams{
			FormID: formID,
			Limit:  int32(limit),
		})
	} else {
		c, parseErr := decodeCursor(*after)
		if parseErr != nil {
			return ListResult{}, errors.New("invalid cursor")
		}
		t, parseErr := time.Parse(time.RFC3339, c.Date)
		if parseErr != nil {
			return ListResult{}, errors.New("invalid cursor date")
		}
		rows, err = s.db.ListResponsesAfter(ctx, queries.ListResponsesAfterParams{
			FormID:     formID,
			ReceivedAt: pgtype.Timestamptz{Time: t, Valid: true},
			ID:         c.ID,
			Limit:      int32(limit),
		})
	}
	if err != nil {
		return ListResult{}, err
	}

	out := make([]ResponseRecord, len(rows))
	ids := make([]string, len(rows))
	for i, r := range rows {
		out[i] = responseRecordFromDB(r)
		ids[i] = r.ID
	}

	// Mark responses as read for burn-after-reading forms. The reaper will delete
	// them on the next pass. Failure here is non-fatal — responses remain visible
	// until the next list call or the reaper runs.
	if len(ids) > 0 {
		_ = s.db.MarkResponsesRead(ctx, queries.MarkResponsesReadParams{
			FormID:  formID,
			Column2: ids,
		})
	}

	var nextCursor *string
	if len(rows) == limit {
		last := rows[len(rows)-1]
		c := encodeCursor(Cursor{
			Date: last.ReceivedAt.Time.UTC().Format(time.RFC3339),
			ID:   last.ID,
		})
		nextCursor = &c
	}

	return ListResult{Responses: out, NextCursor: nextCursor}, nil
}

// GetResponse returns a single response for the given form and response ID.
// Returns ErrNotFound if the form or response doesn't exist or isn't in the workspace.
func (s *Service) GetResponse(ctx context.Context, workspaceID, formID, responseID string) (ResponseRecord, error) {
	if _, err := s.db.GetFormByWorkspace(ctx, queries.GetFormByWorkspaceParams{
		ID:          formID,
		WorkspaceID: workspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResponseRecord{}, ErrNotFound
		}
		return ResponseRecord{}, err
	}

	row, err := s.db.GetResponse(ctx, queries.GetResponseParams{
		ID:     responseID,
		FormID: formID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResponseRecord{}, ErrNotFound
		}
		return ResponseRecord{}, err
	}
	return responseRecordFromDB(row), nil
}

// DeleteResponse hard-deletes a single response.
// Returns ErrNotFound if the form doesn't exist or isn't in the workspace.
func (s *Service) DeleteResponse(ctx context.Context, workspaceID, formID, responseID string) error {
	if _, err := s.db.GetFormByWorkspace(ctx, queries.GetFormByWorkspaceParams{
		ID:          formID,
		WorkspaceID: workspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return s.db.DeleteResponse(ctx, queries.DeleteResponseParams{
		ID:     responseID,
		FormID: formID,
	})
}

// CreateBatch inserts relay submissions in a single transaction.
// Submissions referencing non-existent forms are silently dropped (FK violation).
// Submissions that would exceed the form's response_limit, are past expires_at,
// or target a closed form are also silently dropped.
// Implements relay.BatchStorer.
func (s *Service) CreateBatch(ctx context.Context, items []relay.SubmissionItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := queries.New(tx)

	for i, item := range items {
		id, err := randomID()
		if err != nil {
			return err
		}

		// Each item gets its own savepoint so failures don't abort the transaction.
		sp := fmt.Sprintf("sp_%d", i)
		if _, err := tx.Exec(ctx, "SAVEPOINT "+sp); err != nil {
			return err
		}

		insertErr := q.InsertResponseWithTTL(ctx, queries.InsertResponseWithTTLParams{
			ID:                 id,
			FormID:             item.FormID,
			SchemaVersion:      item.SchemaVersion,
			EncryptedData:      item.EncryptedData,
			EphemeralPublicKey: item.EphemeralPublicKey,
		})
		if insertErr != nil {
			// FK violation or other insert error — roll back and skip.
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				return rbErr
			}
			continue
		}

		// Conditionally increment response_count. Returns pgx.ErrNoRows if the
		// form is closed, expired, or has hit its response cap.
		_, incrErr := q.IncrementResponseCount(ctx, item.FormID)
		if incrErr != nil {
			// Cap/expiry hit or form closed — roll back the insert too.
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				return rbErr
			}
			continue
		}

		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// DeleteExpiredResponses hard-deletes all responses that have passed their TTL
// or have been read on a burn-after-reading form. Called by the reaper goroutine.
func (s *Service) DeleteExpiredResponses(ctx context.Context) error {
	return s.db.DeleteExpiredResponses(ctx)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func responseRecordFromDB(r queries.Response) ResponseRecord {
	return ResponseRecord{
		ID:                 r.ID,
		FormID:             r.FormID,
		ReceivedAt:         r.ReceivedAt,
		SchemaVersion:      r.SchemaVersion,
		EncryptedData:      r.EncryptedData,
		EphemeralPublicKey: r.EphemeralPublicKey,
	}
}

func encodeCursor(c Cursor) string {
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (Cursor, error) {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, err
	}
	if c.Date == "" || c.ID == "" {
		return Cursor{}, errors.New("incomplete cursor")
	}
	return c, nil
}
