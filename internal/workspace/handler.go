package workspace

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/confide/internal/middleware"
	"github.com/phantompunk/confide/internal/permission"
)

// Handler builds the authenticated /api/workspaces sub-router.
func Handler(svc *Service, cache *permission.RoleCache) http.Handler {
	r := chi.NewRouter()
	r.Post("/", createWorkspace(svc))
	r.Get("/", listWorkspaces(svc))

	r.Route("/{id}", func(r chi.Router) {
		r.Use(permission.ResolveWorkspaceRole(svc, cache, "id"))

		// viewer+ (membership proof is sufficient)
		r.Get("/", getWorkspace(svc))
		r.Get("/members", listMembers(svc))
		r.Get("/members/identity-keys", listMemberIdentityKeys(svc))
		r.Get("/member-key", getMyKey(svc))

		// admin+
		r.With(permission.RequireAction(permission.ActionRenameWorkspace)).
			Patch("/", renameWorkspace(svc))
		r.With(permission.RequireAction(permission.ActionChangeRoles)).
			Patch("/members/{accountId}", updateMemberRole(svc))
		r.With(permission.RequireAction(permission.ActionInviteMembers)).
			Delete("/members/{accountId}", removeMember(svc))
		r.With(permission.RequireAction(permission.ActionDistributeKeys)).
			Post("/member-key", grantMemberKey(svc))
		r.With(permission.RequireAction(permission.ActionDistributeKeys)).
			Get("/pending-key-grants", pendingKeyGrants(svc))

		// owner only
		r.With(permission.RequireAction(permission.ActionDeleteWorkspace)).
			Delete("/", deleteWorkspace(svc))
	})
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
		workspaceID := chi.URLParam(r, "id")
		callerRole := permission.WorkspaceRole(r.Context())

		ws, err := svc.Get(r.Context(), workspaceID, callerRole)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
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

		if err := svc.Rename(r.Context(), workspaceID, req.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rename workspace")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteWorkspace(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "id")

		if err := svc.Delete(r.Context(), workspaceID); err != nil {
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
		workspaceID := chi.URLParam(r, "id")

		members, err := svc.ListMembers(r.Context(), workspaceID)
		if err != nil {
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
				"status":    m.Status,
				"lastSeen":  m.LastSeen,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": out})
	}
}

func updateMemberRole(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "id")
		targetID := chi.URLParam(r, "accountId")
		callerRole := permission.WorkspaceRole(r.Context())

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

		if err := svc.UpdateMemberRole(r.Context(), workspaceID, callerRole, targetID, req.Role); err != nil {
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
		workspaceID := chi.URLParam(r, "id")
		targetID := chi.URLParam(r, "accountId")

		if err := svc.RemoveMember(r.Context(), workspaceID, targetID); err != nil {
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

// ─── Phase 5: Collaborative Key Distribution ──────────────────────────────────

func listMemberIdentityKeys(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "id")

		keys, err := svc.ListMemberIdentityKeys(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list identity keys")
			return
		}

		out := make([]map[string]any, len(keys))
		for i, k := range keys {
			out[i] = map[string]any{
				"accountId":         k.AccountID,
				"identityPublicKey": base64.StdEncoding.EncodeToString(k.IdentityPublicKey),
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": out})
	}
}

func getMyKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")

		key, err := svc.GetMyKey(r.Context(), workspaceID, accountID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace key not yet granted")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get workspace key")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"wrappedWorkspaceKey": base64.StdEncoding.EncodeToString(key.WrappedWorkspaceKey),
			"ephemeralPublicKey":  base64.StdEncoding.EncodeToString(key.EphemeralPublicKey),
		})
	}
}

func grantMemberKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID := mw.AccountID(r.Context())
		workspaceID := chi.URLParam(r, "id")

		var req struct {
			AccountID           string `json:"accountId"`
			WrappedWorkspaceKey string `json:"wrappedWorkspaceKey"`
			EphemeralPublicKey  string `json:"ephemeralPublicKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.AccountID == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "accountId is required")
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

		if err := svc.GrantMemberKey(r.Context(), workspaceID, callerID, req.AccountID, wrappedKey, ephemeralPub); err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "member not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to grant workspace key")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func pendingKeyGrants(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "id")

		grants, err := svc.PendingKeyGrants(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list pending key grants")
			return
		}

		out := make([]map[string]any, len(grants))
		for i, g := range grants {
			out[i] = map[string]any{
				"accountId": g.AccountID,
				"username":  g.Username,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"pending": out})
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
