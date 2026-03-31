package relay

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/phantompunk/confide/internal/botguard"
)

const maxSubmitBody = 64 * 1024 // 64KB

// SubmitHandler handles POST /relay/submit.
// It accepts an anonymous form submission, validates the shape, and enqueues it.
// No authentication. No cookies. No response logging. No submission ID returned.
// Bot submissions (honeypot triggered or velocity too fast) are silently discarded
// with the same 202 response so bots receive no distinguishing signal.
func SubmitHandler(q *Queue, guard *botguard.Guard) http.HandlerFunc {
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

		q.Enqueue(SubmissionItem{
			FormID:             req.FormID,
			EncryptedData:      encData,
			EphemeralPublicKey: ephKey,
			SchemaVersion:      req.SchemaVersion,
		})

		w.WriteHeader(http.StatusAccepted)
	}
}
