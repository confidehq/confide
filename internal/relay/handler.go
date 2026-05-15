package relay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/phantompunk/confide/internal/billing"
	"github.com/phantompunk/confide/internal/botguard"
)

const maxSubmitBody = 64 * 1024 // 64KB

// BillingChecker gates submissions against workspace plan limits.
type BillingChecker interface {
	CheckMonthlyResponseLimit(ctx context.Context, workspaceID string) error
	CheckStoredResponseLimit(ctx context.Context, workspaceID string) error
}

// SubmitHandler handles POST /relay/submit.
// It accepts an anonymous form submission, validates the shape, enqueues it,
// and flushes immediately to the database.
// No authentication. No cookies. No response logging. No submission ID returned.
// Bot submissions (honeypot triggered or velocity too fast) are silently discarded
// with the same 202 response so bots receive no distinguishing signal.
func SubmitHandler(q *Queue, storer BatchStorer, checker BillingChecker, guard *botguard.Guard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxSubmitBody))
		if err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		var req struct {
			FormID             string            `json:"formId"`
			EncryptedData      string            `json:"encryptedData"`
			EphemeralPublicKey string            `json:"ephemeralPublicKey"`
			SchemaVersion      int32             `json:"schemaVersion"`
			LoadToken          string            `json:"loadToken"`
			HoneypotFields     map[string]string `json:"honeypotFields"`
			PGPEncryptedData   string            `json:"pgpEncryptedData"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		if req.FormID == "" || req.EncryptedData == "" || req.EphemeralPublicKey == "" {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		encData, err := base64.StdEncoding.DecodeString(req.EncryptedData)
		if err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		ephKey, err := base64.StdEncoding.DecodeString(req.EphemeralPublicKey)
		if err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if guard.IsHoneypotTriggered(req.FormID, req.HoneypotFields) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if guard.VelocityTooFast(req.FormID, req.LoadToken) {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		workspaceID, wsErr := storer.GetFormWorkspace(r.Context(), req.FormID)
		if wsErr != nil && !errors.Is(wsErr, pgx.ErrNoRows) {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		if workspaceID != "" {
			if err := checker.CheckMonthlyResponseLimit(r.Context(), workspaceID); err != nil {
				if errors.Is(err, billing.ErrResponseLimitReached) {
					http.Error(w, `{"code":"payment_required","message":"monthly response limit reached for this workspace"}`, http.StatusPaymentRequired)
					return
				}
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			if err := checker.CheckStoredResponseLimit(r.Context(), workspaceID); err != nil {
				if errors.Is(err, billing.ErrStoredResponseLimitReached) {
					http.Error(w, `{"code":"payment_required","message":"stored response limit reached for this workspace"}`, http.StatusPaymentRequired)
					return
				}
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		}

		q.Enqueue(SubmissionItem{
			FormID:             req.FormID,
			EncryptedData:      encData,
			EphemeralPublicKey: ephKey,
			SchemaVersion:      req.SchemaVersion,
			PGPEncryptedData:   req.PGPEncryptedData,
		})

		if items := q.Drain(); len(items) > 0 {
			if err := storer.CreateBatch(r.Context(), items); err != nil {
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
