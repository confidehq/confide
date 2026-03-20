package forms

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/wisp/internal/middleware"
)

// Handler builds the authenticated /api/forms sub-router.
func Handler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Post("/", createForm(svc))
	r.Get("/", listForms(svc))
	r.Get("/{id}", getForm(svc))
	r.Put("/{id}", updateFormSchema(svc))
	r.Put("/{id}/status", updateFormStatus(svc))
	r.Delete("/{id}", deleteForm(svc))
	return r
}

// PublicSchemaHandler handles GET /api/f/{id}/schema — no authentication.
func PublicSchemaHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		formID := chi.URLParam(r, "id")
		rec, err := svc.GetPublicSchema(r.Context(), formID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "form not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}

		w.Header().Set("Cache-Control", "no-store, no-cache")
		writeJSON(w, http.StatusOK, map[string]any{
			"renderEncryptedSchema": base64.StdEncoding.EncodeToString(rec.RenderEncryptedSchema),
			"publicFormKey":         base64.StdEncoding.EncodeToString(rec.PublicFormKey),
			"schemaVersion":         rec.SchemaVersion,
			"status":                rec.Status,
		})
	}
}

// ─── Authenticated handlers ────────────────────────────────────────────────────

func createForm(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var req struct {
			FormID                string `json:"formId"`
			EncryptedSchema       string `json:"encryptedSchema"`
			RenderEncryptedSchema string `json:"renderEncryptedSchema"`
			PublicFormKey         string `json:"publicFormKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		encSchema, err := base64.StdEncoding.DecodeString(req.EncryptedSchema)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "encryptedSchema must be base64")
			return
		}
		renderSchema, err := base64.StdEncoding.DecodeString(req.RenderEncryptedSchema)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "renderEncryptedSchema must be base64")
			return
		}
		pubKey, err := base64.StdEncoding.DecodeString(req.PublicFormKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "publicFormKey must be base64")
			return
		}

		formID, err := svc.CreateForm(r.Context(), accountID, req.FormID, encSchema, renderSchema, pubKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to create form")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"formId": formID})
	}
}

func listForms(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		forms, err := svc.ListForms(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list forms")
			return
		}

		type formJSON struct {
			ID            string `json:"formId"`
			Status        string `json:"status"`
			SchemaVersion int32  `json:"schemaVersion"`
			ResponseCount int32  `json:"responseCount"`
			CreatedAt     string `json:"createdAt"`
			UpdatedAt     string `json:"updatedAt"`
		}
		out := make([]formJSON, len(forms))
		for i, f := range forms {
			out[i] = formJSON{
				ID:            f.ID,
				Status:        f.Status,
				SchemaVersion: f.SchemaVersion,
				ResponseCount: f.ResponseCount,
				CreatedAt:     f.CreatedAt.Time.Format("2006-01-02"),
				UpdatedAt:     f.UpdatedAt.Time.Format("2006-01-02"),
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"forms": out})
	}
}

func getForm(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")

		form, err := svc.GetForm(r.Context(), accountID, formID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "form not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get form")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"formId":                form.ID,
			"status":                form.Status,
			"schemaVersion":         form.SchemaVersion,
			"responseCount":         form.ResponseCount,
			"createdAt":             form.CreatedAt.Time.Format("2006-01-02"),
			"updatedAt":             form.UpdatedAt.Time.Format("2006-01-02"),
			"encryptedSchema":       base64.StdEncoding.EncodeToString(form.EncryptedSchema),
			"renderEncryptedSchema": base64.StdEncoding.EncodeToString(form.RenderEncryptedSchema),
			"publicFormKey":         base64.StdEncoding.EncodeToString(form.PublicFormKey),
		})
	}
}

func updateFormSchema(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")

		var req struct {
			EncryptedSchema       string `json:"encryptedSchema"`
			RenderEncryptedSchema string `json:"renderEncryptedSchema"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		encSchema, err := base64.StdEncoding.DecodeString(req.EncryptedSchema)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "encryptedSchema must be base64")
			return
		}
		renderSchema, err := base64.StdEncoding.DecodeString(req.RenderEncryptedSchema)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "renderEncryptedSchema must be base64")
			return
		}

		version, err := svc.UpdateFormSchema(r.Context(), accountID, formID, encSchema, renderSchema)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "form not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to update form")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"schemaVersion": version})
	}
}

func updateFormStatus(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		if err := svc.UpdateFormStatus(r.Context(), accountID, formID, req.Status); err != nil {
			if err.Error() == "status must be 'open' or 'closed'" {
				writeError(w, http.StatusBadRequest, "invalid_field", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to update status")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteForm(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")

		if err := svc.DeleteForm(r.Context(), accountID, formID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to delete form")
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
