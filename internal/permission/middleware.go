package permission

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/phantompunk/confide/internal/db/queries"
	mw "github.com/phantompunk/confide/internal/middleware"
)

// MemberResolver is the narrow interface used by ResolveWorkspaceRole for
// cache-miss DB lookups.
type MemberResolver interface {
	GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error)
}

// ResolveWorkspaceRole fetches (or cache-hits) the caller's role for the
// workspace identified by the named Chi URL param, then injects it into context.
// Returns 403 if the caller is not a workspace member.
func ResolveWorkspaceRole(db MemberResolver, cache *RoleCache, workspaceIDParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accountID := mw.AccountID(r.Context())
			workspaceID := chi.URLParam(r, workspaceIDParam)

			role, ok := cache.Get(workspaceID, accountID)
			if !ok {
				member, err := db.GetWorkspaceMember(r.Context(), queries.GetWorkspaceMemberParams{
					WorkspaceID: workspaceID,
					AccountID:   accountID,
				})
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						writeJSON(w, http.StatusForbidden, map[string]string{
							"code":    "forbidden",
							"message": "not a member of this workspace",
						})
						return
					}
					writeJSON(w, http.StatusInternalServerError, map[string]string{
						"code":    "internal",
						"message": "failed to resolve workspace membership",
					})
					return
				}
				role = member.Role
				cache.Set(workspaceID, accountID, role)
			}

			ctx := context.WithValue(r.Context(), workspaceRoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAction reads the workspace role already in context and returns 403 if
// it is insufficient for the given action. Must run after ResolveWorkspaceRole.
func RequireAction(action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := WorkspaceRole(r.Context())
			if !Can(role, action) {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"code":    "forbidden",
					"message": "insufficient role",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WorkspaceRole reads the resolved workspace role from the context.
func WorkspaceRole(ctx context.Context) string {
	v, _ := ctx.Value(workspaceRoleKey).(string)
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
