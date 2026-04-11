package forms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/phantompunk/confide/internal/botguard"
	mw "github.com/phantompunk/confide/internal/middleware"
)

// workspaceSvc is the minimal workspace interface the handler needs.
type workspaceSvc interface {
	GetPersonalWorkspaceID(ctx context.Context, accountID string) (string, error)
	ValidateMember(ctx context.Context, workspaceID, accountID string) error
}

// Handler builds the authenticated /api/forms sub-router.
func Handler(svc *Service, wsSvc workspaceSvc) http.Handler {
	r := chi.NewRouter()
	r.Post("/", createForm(svc, wsSvc))
	r.Get("/", listForms(svc, wsSvc))
	r.Get("/{id}", getForm(svc, wsSvc))
	r.Put("/{id}", updateFormSchema(svc, wsSvc))
	r.Put("/{id}/status", updateFormStatus(svc, wsSvc))
	r.Put("/{id}/expiration", updateFormExpiration(svc, wsSvc))
	r.Delete("/{id}", deleteForm(svc, wsSvc))
	r.Get("/{id}/schema-versions/{version}", getSchemaVersion(svc, wsSvc))
	return r
}

// resolveFormWorkspace returns the workspace ID that owns formID, after verifying
// that accountID is a member. Returns (workspaceID, true) on success, or writes
// an error response and returns ("", false) on failure.
func resolveFormWorkspace(w http.ResponseWriter, r *http.Request, svc *Service, wsSvc workspaceSvc, accountID, formID string) (string, bool) {
	workspaceID, err := svc.GetFormWorkspace(r.Context(), formID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
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

// PublicSchemaHandler handles GET /api/f/{id}/schema — no authentication.
func PublicSchemaHandler(svc *Service, guard *botguard.Guard) http.HandlerFunc {
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
			"status":                effectiveStatus(rec.Status, rec.ResponseCount, rec.ExpiresAt, rec.ResponseLimit),
			"honeypotFields":        guard.HoneypotNames(formID),
			"loadToken":             guard.IssueToken(formID),
		})
	}
}

// ─── Authenticated handlers ────────────────────────────────────────────────────

func createForm(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var req struct {
			WorkspaceID           string  `json:"workspaceId"`
			FormID                string  `json:"formId"`
			EncryptedSchema       string  `json:"encryptedSchema"`
			RenderEncryptedSchema string  `json:"renderEncryptedSchema"`
			PublicFormKey         string  `json:"publicFormKey"`
			RenderKeySalt         string  `json:"renderKeySalt"`
			ExpiresAt             *string `json:"expiresAt"`
			ResponseLimit         *int32  `json:"responseLimit"`
			ResponseTtlDays       *int32  `json:"responseTtlDays"`
			BurnAfterReading      bool    `json:"burnAfterReading"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		var workspaceID string
		var err error
		if req.WorkspaceID != "" {
			if err = wsSvc.ValidateMember(r.Context(), req.WorkspaceID, accountID); err != nil {
				writeError(w, http.StatusForbidden, "forbidden", "not a member of this workspace")
				return
			}
			workspaceID = req.WorkspaceID
		} else {
			workspaceID, err = wsSvc.GetPersonalWorkspaceID(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "failed to resolve workspace")
				return
			}
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
		var renderKeySalt []byte
		if req.RenderKeySalt != "" {
			renderKeySalt, err = base64.StdEncoding.DecodeString(req.RenderKeySalt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "renderKeySalt must be base64")
				return
			}
		}

		expiresAt, responseLimit, responseTtlDays, burnAfterReading, parseErr := parseExpirationFields(req.ExpiresAt, req.ResponseLimit, req.ResponseTtlDays, req.BurnAfterReading)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", parseErr.Error())
			return
		}

		formID, err := svc.CreateForm(r.Context(), workspaceID, accountID, req.FormID, encSchema, renderSchema, pubKey, renderKeySalt, expiresAt, responseLimit, responseTtlDays, burnAfterReading)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to create form")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"formId": formID})
	}
}

func listForms(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var workspaceID string
		var err error
		if wsID := r.URL.Query().Get("workspaceId"); wsID != "" {
			if err = wsSvc.ValidateMember(r.Context(), wsID, accountID); err != nil {
				writeError(w, http.StatusForbidden, "forbidden", "not a member of this workspace")
				return
			}
			workspaceID = wsID
		} else {
			workspaceID, err = wsSvc.GetPersonalWorkspaceID(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "failed to resolve workspace")
				return
			}
		}

		forms, err := svc.ListForms(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list forms")
			return
		}

		type formJSON struct {
			ID               string  `json:"formId"`
			Status           string  `json:"status"`
			SchemaVersion    int32   `json:"schemaVersion"`
			ResponseCount    int32   `json:"responseCount"`
			CreatedAt        string  `json:"createdAt"`
			UpdatedAt        string  `json:"updatedAt"`
			ExpiresAt        *string `json:"expiresAt,omitempty"`
			ResponseLimit    *int32  `json:"responseLimit,omitempty"`
			ResponseTtlDays  *int32  `json:"responseTtlDays,omitempty"`
			BurnAfterReading bool    `json:"burnAfterReading"`
		}
		out := make([]formJSON, len(forms))
		for i, f := range forms {
			out[i] = formJSON{
				ID:               f.ID,
				Status:           effectiveStatus(f.Status, f.ResponseCount, f.ExpiresAt, f.ResponseLimit),
				SchemaVersion:    f.SchemaVersion,
				ResponseCount:    f.ResponseCount,
				CreatedAt:        f.CreatedAt.Time.Format("2006-01-02"),
				UpdatedAt:        f.UpdatedAt.Time.Format("2006-01-02"),
				ExpiresAt:        nullableDateString(f.ExpiresAt),
				ResponseLimit:    nullableInt32(f.ResponseLimit),
				ResponseTtlDays:  nullableInt32(f.ResponseTtlDays),
				BurnAfterReading: f.BurnAfterReading,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"forms": out})
	}
}

func getForm(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")
		workspaceID, ok := resolveFormWorkspace(w, r, svc, wsSvc, accountID, formID)
		if !ok {
			return
		}

		form, err := svc.GetForm(r.Context(), workspaceID, formID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "form not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get form")
			return
		}

		resp := map[string]any{
			"formId":                form.ID,
			"status":                effectiveStatus(form.Status, form.ResponseCount, form.ExpiresAt, form.ResponseLimit),
			"schemaVersion":         form.SchemaVersion,
			"responseCount":         form.ResponseCount,
			"createdAt":             form.CreatedAt.Time.Format("2006-01-02"),
			"updatedAt":             form.UpdatedAt.Time.Format("2006-01-02"),
			"encryptedSchema":       base64.StdEncoding.EncodeToString(form.EncryptedSchema),
			"renderEncryptedSchema": base64.StdEncoding.EncodeToString(form.RenderEncryptedSchema),
			"publicFormKey":         base64.StdEncoding.EncodeToString(form.PublicFormKey),
			"burnAfterReading":      form.BurnAfterReading,
		}
		if len(form.RenderKeySalt) > 0 {
			resp["renderKeySalt"] = base64.StdEncoding.EncodeToString(form.RenderKeySalt)
		}
		if d := nullableDateString(form.ExpiresAt); d != nil {
			resp["expiresAt"] = *d
		}
		if n := nullableInt32(form.ResponseLimit); n != nil {
			resp["responseLimit"] = *n
		}
		if n := nullableInt32(form.ResponseTtlDays); n != nil {
			resp["responseTtlDays"] = *n
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func updateFormSchema(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")
		workspaceID, ok := resolveFormWorkspace(w, r, svc, wsSvc, accountID, formID)
		if !ok {
			return
		}

		var req struct {
			EncryptedSchema       string `json:"encryptedSchema"`
			RenderEncryptedSchema string `json:"renderEncryptedSchema"`
			RenderKeySalt         string `json:"renderKeySalt"`
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
		var renderKeySalt []byte
		if req.RenderKeySalt != "" {
			renderKeySalt, err = base64.StdEncoding.DecodeString(req.RenderKeySalt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "renderKeySalt must be base64")
				return
			}
		}

		version, err := svc.UpdateFormSchema(r.Context(), workspaceID, formID, encSchema, renderSchema, renderKeySalt)
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

func updateFormStatus(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")
		workspaceID, ok := resolveFormWorkspace(w, r, svc, wsSvc, accountID, formID)
		if !ok {
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		if err := svc.UpdateFormStatus(r.Context(), workspaceID, formID, req.Status); err != nil {
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

func deleteForm(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")
		workspaceID, ok := resolveFormWorkspace(w, r, svc, wsSvc, accountID, formID)
		if !ok {
			return
		}

		if err := svc.DeleteForm(r.Context(), workspaceID, formID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to delete form")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func getSchemaVersion(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")
		workspaceID, ok := resolveFormWorkspace(w, r, svc, wsSvc, accountID, formID)
		if !ok {
			return
		}
		versionStr := chi.URLParam(r, "version")

		version64, err := strconv.ParseInt(versionStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_param", "version must be an integer")
			return
		}

		blob, err := svc.GetSchemaVersion(r.Context(), workspaceID, formID, int32(version64))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "form or version not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get schema version")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"encryptedSchema": base64.StdEncoding.EncodeToString(blob),
			"version":         int32(version64),
		})
	}
}

func updateFormExpiration(svc *Service, wsSvc workspaceSvc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		formID := chi.URLParam(r, "id")
		workspaceID, ok := resolveFormWorkspace(w, r, svc, wsSvc, accountID, formID)
		if !ok {
			return
		}

		var req struct {
			ExpiresAt        *string `json:"expiresAt"`
			ResponseLimit    *int32  `json:"responseLimit"`
			ResponseTtlDays  *int32  `json:"responseTtlDays"`
			BurnAfterReading bool    `json:"burnAfterReading"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		expiresAt, responseLimit, responseTtlDays, burnAfterReading, parseErr := parseExpirationFields(req.ExpiresAt, req.ResponseLimit, req.ResponseTtlDays, req.BurnAfterReading)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", parseErr.Error())
			return
		}

		if err := svc.UpdateExpiration(r.Context(), workspaceID, formID, expiresAt, responseLimit, responseTtlDays, burnAfterReading); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to update expiration")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// effectiveStatus computes the observable status of a form. A form is closed if
// manually set to "closed", past its sunset date, or at its response cap.
// Expiration rules always win over a manual "open" setting.
func effectiveStatus(status string, responseCount int32, expiresAt pgtype.Date, responseLimit pgtype.Int4) string {
	if status == "closed" {
		return "closed"
	}
	if expiresAt.Valid && !time.Now().Before(expiresAt.Time) {
		return "closed"
	}
	if responseLimit.Valid && responseCount >= responseLimit.Int32 {
		return "closed"
	}
	return "open"
}

// parseExpirationFields converts optional JSON expiration inputs to pgtype values.
func parseExpirationFields(expiresAtStr *string, responseLimit *int32, responseTtlDaysRaw *int32, burnAfterReading bool) (pgtype.Date, pgtype.Int4, pgtype.Int4, bool, error) {
	var expiresAt pgtype.Date
	if expiresAtStr != nil {
		t, err := time.Parse("2006-01-02", *expiresAtStr)
		if err != nil {
			return pgtype.Date{}, pgtype.Int4{}, pgtype.Int4{}, false, errors.New("expiresAt must be a date in YYYY-MM-DD format")
		}
		expiresAt = pgtype.Date{Time: t, Valid: true}
	}

	var limit pgtype.Int4
	if responseLimit != nil {
		if *responseLimit < 1 {
			return pgtype.Date{}, pgtype.Int4{}, pgtype.Int4{}, false, errors.New("responseLimit must be a positive integer")
		}
		limit = pgtype.Int4{Int32: *responseLimit, Valid: true}
	}

	var ttlDays pgtype.Int4
	if responseTtlDaysRaw != nil {
		if *responseTtlDaysRaw < 1 {
			return pgtype.Date{}, pgtype.Int4{}, pgtype.Int4{}, false, errors.New("responseTtlDays must be a positive integer")
		}
		ttlDays = pgtype.Int4{Int32: *responseTtlDaysRaw, Valid: true}
	}

	return expiresAt, limit, ttlDays, burnAfterReading, nil
}

func nullableDateString(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func nullableInt32(n pgtype.Int4) *int32 {
	if !n.Valid {
		return nil
	}
	return &n.Int32
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
