package responses

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/confide/internal/middleware"
)

// workspaceSvc is the minimal workspace interface the handler needs.
type workspaceSvc interface {
	GetPersonalWorkspaceID(ctx context.Context, accountID string) (string, error)
}

// Handler builds the responses sub-router, mounted under /api/forms/{formId}/responses.
func Handler(svc *Service, wsSvc workspaceSvc) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listResponses(svc, wsSvc))
	r.Get("/{rid}", getResponse(svc, wsSvc))
	r.Delete("/{rid}", deleteResponse(svc, wsSvc))
	return r
}

func listResponses(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID, err := wsSvc.GetPersonalWorkspaceID(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to resolve workspace")
			return
		}
		formID := chi.URLParam(r, "formId")

		var after *string
		if v := r.URL.Query().Get("after"); v != "" {
			after = &v
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		result, err := svc.ListResponses(r.Context(), workspaceID, formID, after, limit)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "form not found")
				return
			}
			if err.Error() == "invalid cursor" {
				writeError(w, http.StatusBadRequest, "invalid_cursor", "invalid cursor")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to list responses")
			return
		}

		type respJSON struct {
			ID                 string `json:"id"`
			ReceivedAt         string `json:"receivedAt"`
			SchemaVersion      int32  `json:"schemaVersion"`
			EncryptedData      string `json:"encryptedData"`
			EphemeralPublicKey string `json:"ephemeralPublicKey"`
		}
		out := make([]respJSON, len(result.Responses))
		for i, resp := range result.Responses {
			out[i] = respJSON{
				ID:                 resp.ID,
				ReceivedAt:         resp.ReceivedAt.Time.Format("2006-01-02"),
				SchemaVersion:      resp.SchemaVersion,
				EncryptedData:      base64.StdEncoding.EncodeToString(resp.EncryptedData),
				EphemeralPublicKey: base64.StdEncoding.EncodeToString(resp.EphemeralPublicKey),
			}
		}

		body := map[string]any{"responses": out}
		if result.NextCursor != nil {
			body["nextCursor"] = *result.NextCursor
		}
		writeJSON(w, http.StatusOK, body)
	}
}

func getResponse(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID, err := wsSvc.GetPersonalWorkspaceID(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to resolve workspace")
			return
		}
		formID := chi.URLParam(r, "formId")
		responseID := chi.URLParam(r, "rid")

		resp, err := svc.GetResponse(r.Context(), workspaceID, formID, responseID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "response not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get response")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":                 resp.ID,
			"formId":             resp.FormID,
			"receivedAt":         resp.ReceivedAt.Time.Format("2006-01-02"),
			"schemaVersion":      resp.SchemaVersion,
			"encryptedData":      base64.StdEncoding.EncodeToString(resp.EncryptedData),
			"ephemeralPublicKey": base64.StdEncoding.EncodeToString(resp.EphemeralPublicKey),
		})
	}
}

func deleteResponse(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID, err := wsSvc.GetPersonalWorkspaceID(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to resolve workspace")
			return
		}
		formID := chi.URLParam(r, "formId")
		responseID := chi.URLParam(r, "rid")

		if err := svc.DeleteResponse(r.Context(), workspaceID, formID, responseID); err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "response not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to delete response")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
