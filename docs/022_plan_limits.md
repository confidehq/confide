# Plan: Workspace Plan Limits & Usage Tracking

## Context

The workspace billing model (free/pro/org) currently only enforces member count limits. Forms created and monthly responses collected have no enforcement — the DB count queries exist in `billing.sql` but are never used to gate operations. This change adds hard-block enforcement for all three resource types and exposes a usage API so the frontend can display quota status and prompt upgrades.

**Limits to enforce:**

| Plan | Members | Forms | Responses/month |
|------|---------|-------|-----------------|
| free | 2       | 3     | 100             |
| pro  | 10      | 20    | 1,000           |
| org  | -1 (∞)  | -1 (∞) | -1 (∞)         |

**Behavior:** Hard block — return HTTP 402 when a workspace is at its limit.

---

## Phase 1 — Billing Service

### Limit functions (`internal/billing/service.go`)

Add two package-level functions alongside the existing `PlanMemberLimit` (line 491):

```go
func PlanFormLimit(plan string) int64 {
    switch plan {
    case "free": return 3
    case "pro":  return 20
    default:     return -1
    }
}

func PlanMonthlyResponseLimit(plan string) int64 {
    switch plan {
    case "free": return 100
    case "pro":  return 1000
    default:     return -1
    }
}
```

### Sentinel errors (`internal/billing/service.go`)

```go
var ErrFormLimitReached     = errors.New("form limit reached for current plan")
var ErrResponseLimitReached = errors.New("monthly response limit reached for current plan")
```

### Limit check methods on `*Service`

**`CheckFormLimit(ctx context.Context, workspaceID string) error`**
1. `GetWorkspaceForBilling(ctx, workspaceID)` → `.Plan`
2. `CountFormsByWorkspace(ctx, workspaceID)` → current count
3. If `PlanFormLimit(plan) != -1 && count >= limit` → return `ErrFormLimitReached`

**`CheckMonthlyResponseLimit(ctx context.Context, workspaceID string) error`**
1. `GetWorkspaceForBilling(ctx, workspaceID)` → `.Plan`
2. `CountMonthlyResponses(ctx, workspaceID)` → current count
3. If `PlanMonthlyResponseLimit(plan) != -1 && count >= limit` → return `ErrResponseLimitReached`

Both queries are already generated in `internal/db/queries/billing.sql`.

---

## Phase 2 — Usage Endpoint

### New query (`internal/db/queries/billing.sql`)

Check `internal/db/queries/workspaces.sql` first — if `CountWorkspaceMembers` already exists there, reuse it. Otherwise add:

```sql
-- name: CountActiveWorkspaceMembers :one
SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1;
```

Run `sqlc generate` after adding.

### Usage types and method (`internal/billing/service.go`)

```go
type UsageInfo struct {
    Current int64 `json:"current"`
    Limit   int64 `json:"limit"` // -1 = unlimited
}

type WorkspaceUsage struct {
    Members   UsageInfo `json:"members"`
    Forms     UsageInfo `json:"forms"`
    Responses UsageInfo `json:"responses"`
}
```

**`GetUsage(ctx context.Context, workspaceID string) (*WorkspaceUsage, error)`**
- Fetch plan via `GetWorkspaceForBilling`
- Run member, form, and response count queries (parallel with errgroup)
- Return `WorkspaceUsage` with current counts and plan limits

### New HTTP endpoint (`internal/billing/handler.go`)

**`GET /api/workspaces/{workspaceID}/usage`**
- Auth: viewer+ — call `wsSvc.ValidateMember(ctx, workspaceID, accountID)`
- Call `svc.GetUsage(ctx, workspaceID)`
- Return 200 JSON `WorkspaceUsage`

Register in `billing/handler.go`'s `Handler()` and mount in `server.go` alongside existing billing routes.

---

## Phase 3 — Enforcement at Write Paths

### Form creation (`internal/forms/handler.go`)

Add billing interface to handler:

```go
type billingSvc interface {
    CheckFormLimit(ctx context.Context, workspaceID string) error
}
```

Update `Handler(svc *Service, wsSvc workspaceSvc, billingSvc billingSvc)`.

In `createForm` (line ~201), after resolving `workspaceID`, before calling `svc.CreateForm`:

```go
if err := billingSvc.CheckFormLimit(r.Context(), workspaceID); err != nil {
    if errors.Is(err, billing.ErrFormLimitReached) {
        writeError(w, http.StatusPaymentRequired, "form limit reached for your plan")
        return
    }
    writeError(w, http.StatusInternalServerError, "")
    return
}
```

### Response submission (`internal/relay/relay.go`)

Add billing interface to relay handler and update `SubmitHandler` signature to accept it.

Before enqueuing/processing each submission, resolve the form's `workspace_id` (it may already be accessible via the form lookup in the relay path — if not, add `GetFormWorkspace(ctx, formID) (string, error)` to the relay's form dependency interface).

```go
if err := billingSvc.CheckMonthlyResponseLimit(r.Context(), workspaceID); err != nil {
    if errors.Is(err, billing.ErrResponseLimitReached) {
        writeError(w, http.StatusPaymentRequired, "response limit reached for this workspace")
        return
    }
    // ...
}
```

---

## Phase 4 — Wiring (`internal/server/server.go`)

Two handler call-site updates:

```go
forms.Handler(svc.Forms, svc.Workspace, svc.Billing)
relay.SubmitHandler(svc.RelayQ, svc.Responses, svc.Billing, guard)
```

---

## Reused Patterns

| Pattern | Location |
|---------|----------|
| `PlanMemberLimit(plan string) int64` | `billing/service.go:491` |
| `GetWorkspaceForBilling` sqlc query | `billing.sql` |
| `CountFormsByWorkspace` sqlc query | `billing.sql` |
| `CountMonthlyResponses` sqlc query | `billing.sql` |
| `errors.Is(err, ErrFoo)` dispatch | existing handlers |
| `writeError(w, code, msg)` helper | existing form/workspace handlers |

---

## Verification

1. **Form limit** — free workspace: create 3 forms → 4th returns 402
2. **Response limit** — free workspace: submit 100 responses → 101st returns 402
3. **Member limit** — existing behavior unchanged
4. **Usage endpoint** — `GET /api/workspaces/{id}/usage` returns correct counts + limits
5. **Org plan** — all three limits bypassed (plan limit = -1)
6. **Plan upgrade** — upgrade free → pro → new limits apply immediately on next request
