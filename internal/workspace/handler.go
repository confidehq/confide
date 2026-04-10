package workspace

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/confide/internal/middleware"
)

// Handler builds the authenticated /api/workspaces sub-router.
func Handler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Post("/", createWorkspace(svc))
	r.Get("/", listWorkspaces(svc))
	r.Get("/{id}", getWorkspace(svc))
	r.Patch("/{id}", renameWorkspace(svc))
	r.Delete("/{id}", deleteWorkspace(svc))
	r.Get("/{id}/members", listMembers(svc))
	r.Patch("/{id}/members/{accountId}", updateMemberRole(svc))
	r.Delete("/{id}/members/{accountId}", removeMember(svc))
	return r
}

func createWorkspace(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var req struct {
			Name                string `json:"name"`
			WrappedWorkspaceKey string `json:"wrappedWorkspaceKey"`
			EphemeralPublicKey  string `json:"ephemeralPublicKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "name is required")
			return
		}

		wrappedKey, err := base64.StdEncoding.DecodeString(req.WrappedWorkspaceKey)
		if err != nil || len(wrappedKey) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedWorkspaceKey must be non-empty base64")
			return
		}
		ephemeralPub, err := base64.StdEncoding.DecodeString(req.EphemeralPublicKey)
		if err != nil || len(ephemeralPub) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_field", "ephemeralPublicKey must be non-empty base64")
			return
		}

		ws, err := svc.Create(r.Context(), accountID, req.Name, wrappedKey, ephemeralPub)
		if err != nil {
			if errors.Is(err, ErrPlanLimit) {
				writeError(w, http.StatusPaymentRequired, "plan_limit", "free plan allows only one workspace")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to create workspace")
			return
		}

		writeJSON(w, http.StatusCreated, workspaceJSON(ws))
	}
}

func listWorkspaces(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaces, err := svc.List(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list workspaces")
			return
		}
		out := make([]map[string]any, len(workspaces))
		for i, ws := range workspaces {
			out[i] = workspaceJSON(ws)
		}
		writeJSON(w, http.StatusOK, map[string]any{"workspaces": out})
	}
}

func getWorkspace(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")

		ws, err := svc.Get(r.Context(), workspaceID, accountID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
				return
			}
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "not a member of this workspace")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get workspace")
			return
		}

		writeJSON(w, http.StatusOK, workspaceJSON(ws))
	}
}

func renameWorkspace(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "name is required")
			return
		}

		if err := svc.Rename(r.Context(), workspaceID, accountID, req.Name); err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "owner or admin role required")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to rename workspace")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteWorkspace(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")

		if err := svc.Delete(r.Context(), workspaceID, accountID); err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "owner role required")
				return
			}
			if errors.Is(err, ErrHasMembers) {
				writeError(w, http.StatusConflict, "has_members", "remove all non-owner members before deleting")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to delete workspace")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func listMembers(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")

		members, err := svc.ListMembers(r.Context(), workspaceID, accountID)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "not a member of this workspace")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to list members")
			return
		}

		out := make([]map[string]any, len(members))
		for i, m := range members {
			out[i] = map[string]any{
				"accountId": m.AccountID,
				"username":  m.Username,
				"role":      m.Role,
				"joinedAt":  m.JoinedAt,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": out})
	}
}

func updateMemberRole(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")
		targetID := chi.URLParam(r, "accountId")

		var req struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.Role == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "role is required")
			return
		}

		if err := svc.UpdateMemberRole(r.Context(), workspaceID, callerID, targetID, req.Role); err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "insufficient role")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "member not found")
				return
			}
			if errors.Is(err, ErrLastOwner) {
				writeError(w, http.StatusConflict, "last_owner", "cannot demote the sole owner")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to update member role")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func removeMember(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")
		targetID := chi.URLParam(r, "accountId")

		if err := svc.RemoveMember(r.Context(), workspaceID, callerID, targetID); err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "insufficient role")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "member not found")
				return
			}
			if errors.Is(err, ErrLastOwner) {
				writeError(w, http.StatusConflict, "last_owner", "cannot remove the sole owner")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to remove member")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func workspaceJSON(ws Workspace) map[string]any {
	return map[string]any{
		"id":         ws.ID,
		"name":       ws.Name,
		"slug":       ws.Slug,
		"plan":       ws.Plan,
		"planStatus": ws.PlanStatus,
		"role":       ws.Role,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
