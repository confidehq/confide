# Permissions — Roles & Tiers

**Status:** Planning  
**Scope:** Centralise role enforcement and add Pro-tier feature gates across workspace, billing, and invitation routes.

---

## Context

Workspace membership already stores a `role` column (`'owner'|'admin'|'member'|'viewer'`) but enforcement is scattered — every service method does its own `GetWorkspaceMember` DB query and inline `roleRank` check. There is no plan-tier enforcement at all. This design introduces a single `permission` package as the source of truth for both role-based and plan-based access control, moves role resolution to middleware (with request-level caching), and lays the structural groundwork for Pro-tier feature gates.

---

## Architecture

### Role enforcement: two-layer middleware

```
request
  └── mw.Authenticator          (injects accountID into context)
        └── ResolveWorkspaceRole  (fetches/caches role, injects into context)
              └── RequireAction   (per-route minimum check, reads role from context)
                    └── handler → service.Method(ctx, callerRole, ...)
```

`ResolveWorkspaceRole` runs once per request, caches the `(accountID, workspaceID) → role` lookup for 60 seconds, and returns 403 for non-members. `RequireAction` is a lightweight, zero-DB middleware applied per route. Service methods receive `callerRole string` from handlers and call `permission.Can()` for any nuanced checks (e.g. "cannot promote above own role").

### Plan enforcement: inline service checks

Pro-feature checks are inline in service methods (`permission.PlanAllows(plan, feature)`). No middleware for plan gates — each Pro feature has exactly one enforcement point when it is built.

---

## Permission Matrix

| Action | Owner | Admin | Member | Viewer |
|---|:---:|:---:|:---:|:---:|
| Delete workspace | ✓ | | | |
| Manage billing | ✓ | | | |
| Invite / remove members | ✓ | ✓ | | |
| Change roles (up to own level) | ✓ | ✓ | | |
| Rename workspace | ✓ | ✓ | | |
| Distribute form keys to new members | ✓ | ✓ | | |
| Create / edit / delete forms | ✓ | ✓ | ✓ | |
| View forms & decrypt responses | ✓ | ✓ | ✓ | ✓ |

## Pro-Tier Feature Gates

Features gated to Pro plan (enforce via `permission.PlanAllows`):

| Feature constant | Description |
|---|---|
| `FeatureCustomStyles` | Custom form styles |
| `FeatureWhitelabel` | Whitelabel forms (remove Confide branding) |
| `FeatureCustomDomains` | Custom domains |
| `FeatureAdvancedAnalytics` | Advanced analytics dashboard |
| `FeaturePartialSubmissions` | Save and resume partial form submissions |
| `FeatureVersionHistory` | Form version history |
| `FeatureExtendedEmailFwd` | Additional email forwarding destinations |

Pro feature gates are defined now; actual feature functionality is built separately per feature.

---

## New Package: `/internal/permission/`

### `permission.go` — types and matrices

```go
type Action string
const (
    ActionViewForms        Action = "view_forms"         // viewer+
    ActionManageForms      Action = "manage_forms"        // member+
    ActionDistributeKeys   Action = "distribute_keys"     // admin+
    ActionInviteMembers    Action = "invite_members"      // admin+
    ActionChangeRoles      Action = "change_roles"        // admin+
    ActionRenameWorkspace  Action = "rename_workspace"    // admin+
    ActionManageBilling    Action = "manage_billing"      // owner only
    ActionDeleteWorkspace  Action = "delete_workspace"    // owner only
)

type Feature string
const (
    FeatureCustomStyles       Feature = "custom_styles"
    FeatureWhitelabel         Feature = "whitelabel"
    FeatureCustomDomains      Feature = "custom_domains"
    FeatureAdvancedAnalytics  Feature = "advanced_analytics"
    FeaturePartialSubmissions Feature = "partial_submissions"
    FeatureVersionHistory     Feature = "version_history"
    FeatureExtendedEmailFwd   Feature = "extended_email_forwarding"
)

func RoleRank(role string) int             // exported; owner=4, admin=3, member=2, viewer=1
func Can(role string, action Action) bool  // RoleRank(role) >= matrix[action]
func PlanAllows(plan string, f Feature) bool

// UpgradeRequiredError is returned when a Pro feature is accessed on free plan.
// Handlers translate it to HTTP 402.
type UpgradeRequiredError struct{ Feature Feature }
```

### `cache.go` — role cache

```go
type RoleCache struct {
    mu      sync.RWMutex
    entries map[cacheKey]cacheEntry  // cacheKey = { WorkspaceID, AccountID string }
    done    chan struct{}
}

func NewRoleCache() *RoleCache               // starts background purge goroutine (5-min interval)
func (c *RoleCache) Get(wsID, acctID string) (role string, ok bool)
func (c *RoleCache) Set(wsID, acctID, role string)   // TTL = 60 s
func (c *RoleCache) Invalidate(wsID, acctID string)  // call after UpdateMemberRole / RemoveMember
func (c *RoleCache) Stop()                           // test cleanup
```

Instantiated **once** in `server.New()` and threaded into all workspace-scoped middleware.

**Trade-off:** A role change takes up to 60 s to propagate unless `Invalidate` is called. `UpdateMemberRole` and `RemoveMember` both call `cache.Invalidate()` after a successful DB write.

### `middleware.go` — Chi middleware

```go
// MemberResolver is the narrow interface used for cache-miss DB lookups.
type MemberResolver interface {
    GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error)
}

// ResolveWorkspaceRole fetches (or cache-hits) the caller's role for the workspace
// identified by the named Chi URL param. Returns 403 if caller is not a member.
func ResolveWorkspaceRole(db MemberResolver, cache *RoleCache, workspaceIDParam string) func(http.Handler) http.Handler

// RequireAction reads the role already in context and returns 403 if insufficient.
// Must run after ResolveWorkspaceRole.
func RequireAction(action Action) func(http.Handler) http.Handler

func WorkspaceRole(ctx context.Context) string
```

---

## Files to Modify

### `/internal/workspace/service.go`

- Delete private `roleRank()` → replaced by `permission.RoleRank()`
- Add `cache *permission.RoleCache` field; set in constructor
- Add forwarding method to satisfy `permission.MemberResolver`:
  ```go
  func (s *Service) GetWorkspaceMember(ctx context.Context, arg queries.GetWorkspaceMemberParams) (queries.WorkspaceMember, error) {
      return s.db.GetWorkspaceMember(ctx, arg)
  }
  ```
- Refactor all methods that start with inline `GetWorkspaceMember` + role check:
  - Add `callerRole string` parameter
  - Replace inline DB lookup + `roleRank` with `permission.Can(callerRole, action)`
  - Methods that only proved membership (no role logic) — `ListMembers`, `ListMemberIdentityKeys`, `GetMyKey` — drop `GetWorkspaceMember` entirely
  - `UpdateMemberRole` and `RemoveMember`: call `s.cache.Invalidate(workspaceID, targetAccountID)` after successful DB write

**Before/After (`Rename`):**
```go
// Before
func (s *Service) Rename(ctx context.Context, workspaceID, accountID, name string) error {
    member, err := s.db.GetWorkspaceMember(ctx, ...)
    if err != nil { ... return ErrForbidden }
    if roleRank(member.Role) < roleRank("admin") { return ErrForbidden }
    return s.db.RenameWorkspace(ctx, ...)
}

// After
func (s *Service) Rename(ctx context.Context, workspaceID, callerRole, name string) error {
    if !permission.Can(callerRole, permission.ActionRenameWorkspace) { return ErrForbidden }
    return s.db.RenameWorkspace(ctx, ...)
}
```

### `/internal/workspace/handler.go`

Restructure to a `/{id}` sub-router:

```go
func Handler(svc *Service, cache *permission.RoleCache) http.Handler {
    r := chi.NewRouter()
    r.Post("/", createWorkspace(svc))
    r.Get("/",  listWorkspaces(svc))

    r.Route("/{id}", func(r chi.Router) {
        r.Use(permission.ResolveWorkspaceRole(svc, cache, "id"))

        // viewer+ (membership proof is sufficient)
        r.Get("/",                      getWorkspace(svc))
        r.Get("/members",               listMembers(svc))
        r.Get("/members/identity-keys", listMemberIdentityKeys(svc))
        r.Get("/member-key",            getMyKey(svc))

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
```

Handlers pass `callerRole` to service methods:
```go
callerRole := permission.WorkspaceRole(r.Context())
svc.Rename(r.Context(), workspaceID, callerRole, req.Name)
```

### `/internal/server/server.go`

```go
roleCache := permission.NewRoleCache()

// Invitation routes — admin+
r.Route("/workspaces/{workspaceId}/invitations", func(r chi.Router) {
    r.Use(permission.ResolveWorkspaceRole(svc.Workspace, roleCache, "workspaceId"))
    r.Use(permission.RequireAction(permission.ActionInviteMembers))
    r.Mount("/", invitation.WorkspaceHandler(svc.Invitation))
})

// Billing routes — owner only
r.Route("/workspaces/{workspaceId}/billing", func(r chi.Router) {
    r.Use(permission.ResolveWorkspaceRole(svc.Workspace, roleCache, "workspaceId"))
    r.Use(permission.RequireAction(permission.ActionManageBilling))
    r.Mount("/", billing.Handler(svc.Billing))
})

r.Mount("/workspaces", workspace.Handler(svc.Workspace, roleCache))
```

### `/internal/billing/service.go` and `/internal/invitation/service.go`

Same refactor as workspace service: replace `GetWorkspaceMember` + `roleRank` with `callerRole string` parameter + `permission.Can()`. Middleware already enforces the minimum, so these checks simplify significantly.

### `/internal/forms/handler.go`

No changes. Form routes are not nested under a workspace URL segment, so `ResolveWorkspaceRole` cannot apply at the router level. The existing `workspaceSvc.ValidateMember()` in `resolveFormWorkspace` remains. Form-level role differentiation (member vs. viewer) is deferred.

---

## 402 Response Contract (Pro Gates)

Service methods return `&permission.UpgradeRequiredError{Feature: feature}` when a Pro feature is accessed on the free plan.

Handler pattern:
```go
var upgradeErr *permission.UpgradeRequiredError
if errors.As(err, &upgradeErr) {
    writeJSON(w, http.StatusPaymentRequired, map[string]string{
        "code":    "upgrade_required",
        "feature": string(upgradeErr.Feature),
        "message": "This feature requires a Pro plan.",
    })
    return
}
```

Wire format (HTTP 402):
```json
{
    "code": "upgrade_required",
    "feature": "version_history",
    "message": "This feature requires a Pro plan."
}
```

The frontend keys off `feature` to display feature-specific upgrade CTAs. All gated features surface a lock state in the UI and link to the billing page.

---

## What Is Explicitly Deferred

- Pro feature functionality — gates are defined but no service method calls them until each feature is built
- Distributed cache (Redis) — the in-process `RoleCache` is correct for a single-instance deployment
- Cache invalidation on plan change — not needed; plan checks read the workspace row directly, not the role cache
- Audit log for denied requests
- Form-level role differentiation (member vs. viewer)
- Transferring workspace ownership via UI

---

## Verification

1. **Compile:** `go build ./...` after creating `permission/` (before modifying services).
2. **Unit tests — permission matrix** (`permission/permission_test.go`):
   - `Can("owner", ActionDeleteWorkspace)` → true
   - `Can("admin", ActionDeleteWorkspace)` → false
   - `Can("viewer", ActionViewForms)` → true; `Can("viewer", ActionManageForms)` → false
   - `PlanAllows("pro", FeatureVersionHistory)` → true; `PlanAllows("free", FeatureVersionHistory)` → false
3. **Unit tests — cache:** TTL expiry, `Invalidate`, purge goroutine, concurrent reads.
4. **Unit tests — middleware** (`httptest` + mock `MemberResolver`):
   - Valid member → role in context, handler called
   - Non-member (`pgx.ErrNoRows`) → 403, handler not called
   - Second request same workspace+account → resolver not called (cache hit)
   - Role below minimum for `RequireAction` → 403
5. **Service refactor tests:** DB mock's `GetWorkspaceMember` is not called from refactored service methods.
6. **Smoke tests (server running):**
   - `DELETE /workspaces/{id}` as viewer → 403 at middleware
   - `DELETE /workspaces/{id}` as owner → 200/204
   - `POST /workspaces/{workspaceId}/invitations` as member → 403 at middleware
   - Any workspace route as non-member → 403 from `ResolveWorkspaceRole`
7. **Cleanup check:** `grep -r "roleRank" ./internal` → zero results
