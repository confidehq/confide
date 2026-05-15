# Plan: Workspace Plan Limits & Usage Tracking

## Context

The workspace billing model (free/pro/org) currently only enforces member count limits. Forms created and monthly responses collected have no enforcement — the DB count queries exist in `billing.sql` but are never used to gate operations. This change adds hard-block enforcement for all three resource types and exposes a usage API so the frontend can display quota status and prompt upgrades.

**Limits to enforce:**

| Plan | Members | Responses/month | Responses stored | Emails/month | File storage |
|------|---------|-----------------|------------------|--------------|--------------|
| free | 2       | 250             | 2,000            | 50           | 100 MB       |
| pro  | 10      | 10,000          | 100,000          | 500          | 5 GB         |
| org  | -1 (∞)  | 100,000         | -1 (∞)           | 5,000        | 50 GB        |

Forms are unlimited on all plans and are not enforced — only counted for display.

**Behavior:** Hard block — return HTTP 402 when a workspace is at its limit.

---

## Phase 1 — Billing Service

### Limit functions (`internal/billing/service.go`)

Add two package-level functions alongside the existing `PlanMemberLimit` (line 491):

```go
// responseOverageFactor allows free and pro plans to exceed their advertised
// limit by 10% before the hard block fires. Org and default (-1) are unaffected.
const responseOverageFactor = 1.10

func PlanMonthlyResponseLimit(plan string) int64 {
    switch plan {
    case "free": return 250
    case "pro":  return 10_000
    case "org":  return 100_000
    default:     return -1
    }
}

func PlanStoredResponseLimit(plan string) int64 {
    switch plan {
    case "free": return 2_000
    case "pro":  return 100_000
    default:     return -1
    }
}

// hardResponseLimit returns the actual enforcement threshold — 110% of the plan
// limit for free/pro, or the limit unchanged for org/unlimited plans.
func hardResponseLimit(limit int64) int64 {
    if limit == -1 {
        return -1
    }
    return int64(float64(limit) * responseOverageFactor)
}

func PlanMonthlyEmailLimit(plan string) int64 {
    switch plan {
    case "free": return 50
    case "pro":  return 500
    case "org":  return 5_000
    default:     return -1
    }
}

// PlanFileStorageLimit returns the max total file upload bytes for the plan.
func PlanFileStorageLimit(plan string) int64 {
    switch plan {
    case "free": return 100 * 1024 * 1024        // 100 MB
    case "pro":  return 5 * 1024 * 1024 * 1024   // 5 GB
    case "org":  return 50 * 1024 * 1024 * 1024  // 50 GB
    default:     return -1
    }
}
```

### Sentinel errors (`internal/billing/service.go`)

```go
var ErrResponseLimitReached       = errors.New("monthly response limit reached for current plan")
var ErrStoredResponseLimitReached = errors.New("stored response limit reached for current plan")
var ErrEmailLimitReached          = errors.New("monthly email limit reached for current plan")
var ErrFileStorageLimitReached    = errors.New("file storage limit reached for current plan")
```

### Limit check methods on `*Service`

**`CheckMonthlyResponseLimit(ctx context.Context, workspaceID string) error`**
1. `GetWorkspaceForBilling(ctx, workspaceID)` → `.Plan`
2. `CountMonthlyResponses(ctx, workspaceID)` → current count
3. `hard := hardResponseLimit(PlanMonthlyResponseLimit(plan))`
4. If `hard != -1 && count >= hard` → return `ErrResponseLimitReached`

**`CheckStoredResponseLimit(ctx context.Context, workspaceID string) error`**
1. `GetWorkspaceForBilling(ctx, workspaceID)` → `.Plan`
2. `CountTotalResponses(ctx, workspaceID)` → current count (add this query if not present)
3. `hard := hardResponseLimit(PlanStoredResponseLimit(plan))`
4. If `hard != -1 && count >= hard` → return `ErrStoredResponseLimitReached`

**`CheckMonthlyEmailLimit(ctx context.Context, workspaceID string) error`**
1. `GetWorkspaceForBilling(ctx, workspaceID)` → `.Plan`
2. `CountMonthlyEmails(ctx, workspaceID)` → current count (add this query if not present)
3. If `PlanMonthlyEmailLimit(plan) != -1 && count >= limit` → return `ErrEmailLimitReached`

**`CheckFileStorageLimit(ctx context.Context, workspaceID string, incomingBytes int64) error`**
1. `GetWorkspaceForBilling(ctx, workspaceID)` → `.Plan`
2. `SumFileStorageByWorkspace(ctx, workspaceID)` → total bytes currently stored (add this query)
3. If `PlanFileStorageLimit(plan) != -1 && used+incomingBytes > limit` → return `ErrFileStorageLimitReached`

Takes `incomingBytes` so the check is pre-upload — reject before storing rather than after.

`CountMonthlyResponses` and `CountFormsByWorkspace` are already generated in `internal/db/queries/billing.sql`. Add `CountTotalResponses`, `CountMonthlyEmails`, and `SumFileStorageByWorkspace` queries alongside them.

---

## Phase 2 — Usage Endpoint

### New queries (`internal/db/queries/billing.sql`)

Check `internal/db/queries/workspaces.sql` first — if `CountWorkspaceMembers` already exists there, reuse it. Otherwise add:

```sql
-- name: CountActiveWorkspaceMembers :one
SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1;

-- name: CountTotalResponses :one
SELECT COUNT(*) FROM responses WHERE workspace_id = $1;

-- name: CountMonthlyEmails :one
SELECT COUNT(*) FROM email_notifications
WHERE workspace_id = $1
  AND sent_at >= date_trunc('month', now());

-- name: SumFileStorageByWorkspace :one
SELECT COALESCE(SUM(size_bytes), 0) FROM uploaded_files
WHERE workspace_id = $1 AND deleted_at IS NULL;
```

Run `sqlc generate` after adding.

### Usage types and method (`internal/billing/service.go`)

```go
type UsageInfo struct {
    Current int64 `json:"current"`
    Limit   int64 `json:"limit"` // -1 = unlimited
}

type WorkspaceUsage struct {
    Members          UsageInfo `json:"members"`
    Forms            UsageInfo `json:"forms"`
    MonthlyResponses UsageInfo `json:"monthly_responses"`
    StoredResponses  UsageInfo `json:"stored_responses"`
    MonthlyEmails    UsageInfo `json:"monthly_emails"`
    FileStorageBytes UsageInfo `json:"file_storage_bytes"` // current/limit in bytes; -1 = unlimited
}
```

**`GetUsage(ctx context.Context, workspaceID string) (*WorkspaceUsage, error)`**
- Fetch plan via `GetWorkspaceForBilling`
- Run member, form, monthly response, total response, monthly email, and file storage queries (parallel with errgroup)
- Return `WorkspaceUsage` with current counts and plan limits

### New HTTP endpoint (`internal/billing/handler.go`)

**`GET /api/workspaces/{workspaceID}/usage`**
- Auth: viewer+ — call `wsSvc.ValidateMember(ctx, workspaceID, accountID)`
- Call `svc.GetUsage(ctx, workspaceID)`
- Return 200 JSON `WorkspaceUsage`

Register in `billing/handler.go`'s `Handler()` and mount in `server.go` alongside existing billing routes.

---

## Phase 3 — Enforcement at Write Paths

### Response submission (`internal/relay/relay.go`)

Add billing interface to relay handler and update `SubmitHandler` signature to accept it.

Before enqueuing/processing each submission, resolve the form's `workspace_id` (it may already be accessible via the form lookup in the relay path — if not, add `GetFormWorkspace(ctx, formID) (string, error)` to the relay's form dependency interface).

Check both monthly and stored response limits before accepting a submission:

```go
if err := billingSvc.CheckMonthlyResponseLimit(r.Context(), workspaceID); err != nil {
    if errors.Is(err, billing.ErrResponseLimitReached) {
        writeError(w, http.StatusPaymentRequired, "monthly response limit reached for this workspace")
        return
    }
    writeError(w, http.StatusInternalServerError, "")
    return
}
if err := billingSvc.CheckStoredResponseLimit(r.Context(), workspaceID); err != nil {
    if errors.Is(err, billing.ErrStoredResponseLimitReached) {
        writeError(w, http.StatusPaymentRequired, "stored response limit reached for this workspace")
        return
    }
    writeError(w, http.StatusInternalServerError, "")
    return
}
```

### Email sending (notification path)

Wherever outbound notification emails are dispatched, add a pre-send check:

```go
if err := billingSvc.CheckMonthlyEmailLimit(r.Context(), workspaceID); err != nil {
    if errors.Is(err, billing.ErrEmailLimitReached) {
        // skip sending silently or log — do not return 402 to the submitter
        return nil
    }
    return err
}
```

Silently skip (rather than hard-blocking the submission) so the form submitter is not penalized for the workspace owner's quota.

---

## Phase 4 — Wiring (`internal/server/server.go`)

Two handler call-site updates:

```go
forms.Handler(svc.Forms, svc.Workspace) // no billing dependency — forms are unlimited
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

1. **Form count** — `GET /api/workspaces/{id}/usage` returns current form count with `limit: -1`; no 402 is ever returned on form creation
2. **Monthly response limit** — free workspace: submit 275 responses (250 × 1.10) → 276th returns 402
3. **Stored response limit** — free workspace: accumulate 2,200 responses (2,000 × 1.10) → 2,201st returns 402
4. **Monthly email limit** — free workspace: send 50 notification emails → 51st is silently skipped
5. **Member limit** — existing behavior unchanged
6. **File storage limit** — free workspace: upload files totalling 100 MB → next upload that would exceed 100 MB returns 402
7. **Usage endpoint** — `GET /api/workspaces/{id}/usage` returns correct counts + limits for all six resource types
8. **Org plan** — monthly/stored response and email limits enforced per table; member, form, and file storage limits are -1
9. **Plan upgrade** — upgrade free → pro → new limits apply immediately on next request
