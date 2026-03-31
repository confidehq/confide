package responses

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
	"github.com/phantompunk/confide/internal/relay"
)

var ErrNotFound = errors.New("response not found")

// DB is the subset of queries.Queries used by responses.Service.
type DB interface {
	GetFormByOwner(ctx context.Context, arg queries.GetFormByOwnerParams) (queries.Form, error)
	ListResponsesFirst(ctx context.Context, arg queries.ListResponsesFirstParams) ([]queries.Response, error)
	ListResponsesAfter(ctx context.Context, arg queries.ListResponsesAfterParams) ([]queries.Response, error)
	GetResponse(ctx context.Context, arg queries.GetResponseParams) (queries.Response, error)
	DeleteResponse(ctx context.Context, arg queries.DeleteResponseParams) error
	CreateResponse(ctx context.Context, arg queries.CreateResponseParams) error
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
	ReceivedAt         pgtype.Date
	SchemaVersion      int32
	EncryptedData      []byte
	EphemeralPublicKey []byte
}

// Cursor encodes a pagination position.
type Cursor struct {
	Date string `json:"d"` // "2006-01-02"
	ID   string `json:"i"`
}

// ListResult is a page of responses with an optional next cursor.
type ListResult struct {
	Responses  []ResponseRecord
	NextCursor *string // nil on last page
}

const defaultPageSize = 50

// ListResponses returns a page of responses for a form owned by accountID.
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) ListResponses(ctx context.Context, accountID, formID string, after *string, limit int) (ListResult, error) {
	if _, err := s.db.GetFormByOwner(ctx, queries.GetFormByOwnerParams{
		ID:        formID,
		AccountID: accountID,
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
		var date pgtype.Date
		if scanErr := date.Scan(c.Date); scanErr != nil {
			return ListResult{}, errors.New("invalid cursor date")
		}
		rows, err = s.db.ListResponsesAfter(ctx, queries.ListResponsesAfterParams{
			FormID:     formID,
			ReceivedAt: date,
			ID:         c.ID,
			Limit:      int32(limit),
		})
	}
	if err != nil {
		return ListResult{}, err
	}

	out := make([]ResponseRecord, len(rows))
	for i, r := range rows {
		out[i] = responseRecordFromDB(r)
	}

	var nextCursor *string
	if len(rows) == limit {
		last := rows[len(rows)-1]
		c := encodeCursor(Cursor{
			Date: last.ReceivedAt.Time.Format("2006-01-02"),
			ID:   last.ID,
		})
		nextCursor = &c
	}

	return ListResult{Responses: out, NextCursor: nextCursor}, nil
}

// GetResponse returns a single response for the given form and response ID.
// Returns ErrNotFound if the form or response doesn't exist or isn't owned by accountID.
func (s *Service) GetResponse(ctx context.Context, accountID, formID, responseID string) (ResponseRecord, error) {
	if _, err := s.db.GetFormByOwner(ctx, queries.GetFormByOwnerParams{
		ID:        formID,
		AccountID: accountID,
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
// Returns ErrNotFound if the form doesn't exist or isn't owned by accountID.
func (s *Service) DeleteResponse(ctx context.Context, accountID, formID, responseID string) error {
	if _, err := s.db.GetFormByOwner(ctx, queries.GetFormByOwnerParams{
		ID:        formID,
		AccountID: accountID,
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

		insertErr := q.CreateResponse(ctx, queries.CreateResponseParams{
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
