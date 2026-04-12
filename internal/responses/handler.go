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
	ValidateMember(ctx context.Context, workspaceID, accountID string) error
}

// formsSvc is the minimal forms interface the handler needs.
type formsSvc interface {
	GetFormWorkspace(ctx context.Context, formID string) (string, error)
}

var errFormNotFound = errors.New("form not found")

// resolveFormWorkspace returns the workspace ID that owns formID, after verifying
// that accountID is a member. Returns (workspaceID, true) on success, or writes
// an error response and returns ("", false) on failure.
func resolveFormWorkspace(w http.ResponseWriter, r *http.Request, fSvc formsSvc, wsSvc workspaceSvc, accountID, formID string) (string, bool) {
	workspaceID, err := fSvc.GetFormWorkspace(r.Context(), formID)
	if err != nil {
		if errors.Is(err, errFormNotFound) || err.Error() == "form not found" {
			writeError(w, http.StatusNotFound, "not_found", "form not found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal", "failed to resolve form workspace")
		}
		return "", false
	}
	if err := wsSvc.ValidateMember(r.Context(), workspaceID, accountID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return "", false
	}
	return workspaceID, true
}

// Handler builds the responses sub-router, mounted under /api/forms/{formId}/responses.
func Handler(svc *Service, fSvc formsSvc, wsSvc workspaceSvc) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listResponses(svc, fSvc, wsSvc))
	r.Get("/{rid}", getResponse(svc, fSvc, wsSvc))
	r.Delete("/{rid}", deleteResponse(svc, fSvc, wsSvc))
	return r
}

func listResponses(svc *Service, fSvc formsSvc, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "formId")
		workspaceID, ok := resolveFormWorkspace(w, r, fSvc, wsSvc, accountID, formID)
		if !ok {
			return
		}

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

func getResponse(svc *Service, fSvc formsSvc, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "formId")
		responseID := chi.URLParam(r, "rid")
		workspaceID, ok := resolveFormWorkspace(w, r, fSvc, wsSvc, accountID, formID)
		if !ok {
			return
		}

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

func deleteResponse(svc *Service, fSvc formsSvc, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "formId")
		responseID := chi.URLParam(r, "rid")
		workspaceID, ok := resolveFormWorkspace(w, r, fSvc, wsSvc, accountID, formID)
		if !ok {
			return
		}

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
