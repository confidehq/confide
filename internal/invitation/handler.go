package invitation

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/confide/internal/middleware"
	"github.com/phantompunk/confide/internal/permission"
)

// WorkspaceHandler returns routes mounted at /api/workspaces/{workspaceId}/invitations.
// All routes require authentication (caller must be at minimum admin).
func WorkspaceHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Post("/", createInvitation(svc))
	r.Get("/", listInvitations(svc))
	r.Delete("/{inviteId}", revokeInvitation(svc))
	return r
}

// PublicHandler returns routes mounted at /api/invitations.
// Resolve is unauthenticated; Accept requires auth and must be placed inside an auth group.
func PublicHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/{token}", resolveInvitation(svc))
	return r
}

// AcceptHandler returns the accept route, intended for mounting inside an auth group.
func AcceptHandler(svc *Service) http.HandlerFunc {
	return acceptInvitation(svc)
}

// ─── Workspace-scoped handlers (auth required) ────────────────────────────────

func createInvitation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID := mw.AccountID(r.Context())
		callerRole := permission.WorkspaceRole(r.Context())
		workspaceID := chi.URLParam(r, "workspaceId")

		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "email is required")
			return
		}
		if req.Role == "" {
			writeError(w, http.StatusBadRequest, "invalid_field", "role is required")
			return
		}

		inv, err := svc.Create(r.Context(), workspaceID, callerRole, callerID, req.Email, req.Role)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "cannot invite to a higher role than your own")
				return
			}
			if errors.Is(err, ErrPlanLimit) {
				writeError(w, http.StatusPaymentRequired, "plan_limit", "free plan allows only one collaborator")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to create invitation")
			return
		}

		writeJSON(w, http.StatusCreated, invitationJSON(inv))
	}
}

func listInvitations(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceId")

		invitations, err := svc.List(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list invitations")
			return
		}

		out := make([]map[string]any, len(invitations))
		for i, inv := range invitations {
			out[i] = invitationJSON(inv)
		}
		writeJSON(w, http.StatusOK, map[string]any{"invitations": out})
	}
}

func revokeInvitation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceId")
		inviteID := chi.URLParam(r, "inviteId")

		if err := svc.Revoke(r.Context(), workspaceID, inviteID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to revoke invitation")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Public / accept handlers ─────────────────────────────────────────────────

func resolveInvitation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")

		preview, err := svc.Resolve(r.Context(), token)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "invitation not found")
				return
			}
			if errors.Is(err, ErrExpired) {
				writeError(w, http.StatusGone, "expired", "invitation expired or already accepted")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to resolve invitation")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":              preview.ID,
			"workspaceName":   preview.WorkspaceName,
			"inviterUsername": preview.InviterUsername,
			"role":            preview.Role,
			"expiresAt":       preview.ExpiresAt,
		})
	}
}

func acceptInvitation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		token := chi.URLParam(r, "token")

		if err := svc.Accept(r.Context(), token, accountID); err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "invitation not found")
				return
			}
			if errors.Is(err, ErrExpired) {
				writeError(w, http.StatusGone, "expired", "invitation expired or already accepted")
				return
			}
			if errors.Is(err, ErrConflict) {
				writeError(w, http.StatusConflict, "already_member", "already a member of this workspace")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to accept invitation")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func invitationJSON(inv Invitation) map[string]any {
	return map[string]any{
		"id":          inv.ID,
		"workspaceId": inv.WorkspaceID,
		"email":       inv.Email,
		"role":        inv.Role,
		"expiresAt":   inv.ExpiresAt,
		"createdAt":   inv.CreatedAt,
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
