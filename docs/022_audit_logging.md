# Plan: Audit Logging (Stripe-style)

## Context

Confide needs a Stripe-style audit trail so workspace owners and admins can see who accessed forms, who read responses, who sent invites, etc. The project is privacy-first (no IP logging, anonymous submissions stay anonymous), so the audit log tracks workspace member actions only — not submitter identity.

**Scope confirmed by user:**
- Events: auth, form lifecycle, response access, team & invites
- Visibility: owners see all; admins see all except other owners' auth events; members see their own actions only
- No export — in-app UI only
- Submissions stay anonymous (only log when a member *reads* or *deletes* a response)

---

## Phase 1 — Database

### Migration 0026: `audit_logs` table

```sql
CREATE TABLE audit_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID       NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    account_id  UUID        REFERENCES accounts(id) ON DELETE SET NULL,
    session_id  UUID        REFERENCES sessions(id) ON DELETE SET NULL,
    action      TEXT        NOT NULL,  -- dot-namespaced: 'form.created', 'response.viewed'
    resource_type TEXT      NOT NULL,  -- 'form' | 'response' | 'member' | 'workspace' | 'credential' | 'invitation'
    resource_id TEXT,                  -- UUID of affected resource
    resource_name TEXT,                -- denormalized human name at time of event
    metadata    JSONB,                 -- extra context (role assigned, new status, etc.)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX audit_logs_workspace_created ON audit_logs(workspace_id, created_at DESC);
CREATE INDEX audit_logs_account ON audit_logs(account_id);
```

**Action constants** (full list):
- `auth.login`, `auth.logout`, `auth.credential_added`, `auth.credential_removed`, `auth.recovery_used`
- `form.created`, `form.published`, `form.status_changed`, `form.deleted`, `form.expiration_set`, `form.updated`
- `response.viewed`, `response.deleted`
- `workspace.updated`
- `member.role_changed`, `member.removed`
- `invitation.sent`, `invitation.accepted`, `invitation.cancelled`

---

## Phase 2 — Backend

### New package: `internal/audit/`

**`internal/audit/event.go`** — typed event struct and action constants:
```go
type Event struct {
    WorkspaceID  uuid.UUID
    AccountID    uuid.UUID  // actor
    SessionID    uuid.UUID
    Action       string
    ResourceType string
    ResourceID   string
    ResourceName string
    Metadata     map[string]any
}
```

**`internal/audit/service.go`** — thin service wrapping DB write:
```go
type Service struct { db *pgxpool.Pool }

func (s *Service) Log(ctx context.Context, e Event) {
    // fire-and-forget: spawn goroutine, log zerolog error on failure, never block caller
    go func() {
        // INSERT INTO audit_logs ...
    }()
}
```

Fire-and-forget is critical — a failed audit write must never fail the actual API request.

**`internal/audit/handler.go`** — paginated list endpoint:
```go
// GET /api/workspaces/{id}/audit-logs?cursor=<created_at,id>&limit=50&action=<filter>
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request)
```

Role-based filtering applied at query level:
- `owner` → no additional WHERE clause
- `admin` → exclude rows where `action LIKE 'auth.%'` AND `account_id != self`
- `member` → WHERE `account_id = self`

**`internal/db/queries/audit.sql`** — sqlc query:
```sql
-- name: ListAuditLogs :many
SELECT al.*, a.username
FROM audit_logs al
LEFT JOIN accounts a ON a.id = al.account_id
WHERE al.workspace_id = @workspace_id
  AND (@action_filter::text IS NULL OR al.action = @action_filter)
  AND (al.created_at, al.id) < (@cursor_time, @cursor_id)  -- cursor pagination
ORDER BY al.created_at DESC, al.id DESC
LIMIT @limit;
```

### Wiring: where to call `audit.Log`

| File | Handler | Event |
|------|---------|-------|
| `internal/auth/handler.go` | `LoginFinish` | `auth.login` |
| `internal/auth/handler.go` | `Logout` | `auth.logout` |
| `internal/auth/handler.go` | `AddCredentialFinish` | `auth.credential_added` |
| `internal/auth/handler.go` | `DeleteCredential` | `auth.credential_removed` |
| `internal/auth/handler.go` | `UseRecoveryCode` | `auth.recovery_used` |
| `internal/forms/handler.go` | `CreateForm` | `form.created` |
| `internal/forms/handler.go` | `PublishForm` | `form.published` |
| `internal/forms/handler.go` | `UpdateFormStatus` | `form.status_changed` |
| `internal/forms/handler.go` | `DeleteForm` | `form.deleted` |
| `internal/forms/handler.go` | `UpdateFormExpiration` | `form.expiration_set` |
| `internal/responses/handler.go` | `GetResponse` | `response.viewed` |
| `internal/responses/handler.go` | `DeleteResponse` | `response.deleted` |
| `internal/workspace/handler.go` | `UpdateWorkspace` | `workspace.updated` |
| `internal/workspace/handler.go` | `UpdateMemberRole` | `member.role_changed` |
| `internal/workspace/handler.go` | `RemoveMember` | `member.removed` |
| `internal/invitation/handler.go` | `CreateInvitation` | `invitation.sent` |
| `internal/invitation/handler.go` | `AcceptInvitation` | `invitation.accepted` |
| `internal/invitation/handler.go` | `DeleteInvitation` | `invitation.cancelled` |

### Route registration in `internal/server/server.go`

```go
r.Get("/api/workspaces/{id}/audit-logs", auditHandler.ListAuditLogs)
```

Permission check: require `owner`, `admin`, or `member` (member gets filtered result).

### Inject audit service

Wire `audit.Service` in `cmd/api/app/app.go` alongside existing services; pass to handlers that need to emit events.

---

## Phase 3 — Frontend

### New route: `web/src/routes/(app)/audit/+page.svelte`

Add "Audit Log" to the sidebar nav (under Settings section), visible to owners and admins only.

### API client: `web/src/lib/audit.ts`

```ts
export interface AuditLogEntry {
  id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  resource_name: string;
  metadata: Record<string, unknown>;
  account_id: string;
  username: string;
  created_at: string;
}

export async function listAuditLogs(workspaceId: string, params?: {
  cursor?: string;
  action?: string;
  limit?: number;
}): Promise<{ entries: AuditLogEntry[]; next_cursor: string | null }>
```

### UI layout (Stripe-inspired)

```
┌─────────────────────────────────────────────────────┐
│  Audit Log                          [Filter: All ▼] │
├─────────────────────────────────────────────────────┤
│ [badge] form.published   "Contact Form"             │
│         Rodrigo Moran · 2 minutes ago               │
├─────────────────────────────────────────────────────┤
│ [badge] response.viewed  "Contact Form" / resp #42  │
│         Rodrigo Moran · 1 hour ago                  │
├─────────────────────────────────────────────────────┤
│ [badge] invitation.sent  alice@example.com (admin)  │
│         Rodrigo Moran · 3 hours ago                 │
├─────────────────────────────────────────────────────┤
│                    [ Load more ]                     │
└─────────────────────────────────────────────────────┘
```

**Badge colors:**
- Green: `form.created`, `form.published`, `invitation.accepted`, `member.*` (add)
- Red: `form.deleted`, `response.deleted`, `member.removed`, `invitation.cancelled`
- Blue: `response.viewed`, `form.status_changed`
- Gray: `auth.*`, `workspace.updated`
- Amber: `invitation.sent`, `form.expiration_set`

### Components

| File | Purpose |
|------|---------|
| `routes/(app)/audit/+page.svelte` | Main page, pagination, filter |
| `routes/(app)/audit/+page.ts` | Load initial entries server-side |
| `lib/components/AuditEventBadge.svelte` | Colored action chip |
| `lib/components/AuditLogEntry.svelte` | Single row (badge + resource + actor + time) |
| `lib/audit.ts` | API client |

**Relative time**: use a simple `formatRelative` helper (e.g., "2 minutes ago", "3 hours ago", "May 5") with exact ISO tooltip on hover.

**"You" indicator**: compare `entry.account_id` to current session's account ID, display "You" instead of username.

**Metadata rendering**: for events with useful metadata (e.g., `member.role_changed` → "changed role to admin"), render a subtitle line below the resource name.

### Sidebar nav update

Add entry in the sidebar navigation component (find exact file by grepping for "Settings" nav links) with an `owner | admin` visibility guard.

---

## Files to create

| Path | Description |
|------|-------------|
| `migrations/0026_audit_logs.up.sql` | New table + indexes |
| `migrations/0026_audit_logs.down.sql` | DROP TABLE |
| `internal/audit/event.go` | Event struct + action constants |
| `internal/audit/service.go` | Log() fire-and-forget method + DB query |
| `internal/audit/handler.go` | ListAuditLogs HTTP handler |
| `internal/db/queries/audit.sql` | sqlc queries (ListAuditLogs, InsertAuditLog) |
| `web/src/lib/audit.ts` | Frontend API client |
| `web/src/lib/components/AuditEventBadge.svelte` | Event badge |
| `web/src/lib/components/AuditLogEntry.svelte` | Row component |
| `web/src/routes/(app)/audit/+page.svelte` | Audit log page |
| `web/src/routes/(app)/audit/+page.ts` | Server load |

## Files to modify

| Path | Change |
|------|--------|
| `migrations/0025_*.up.sql` | No change — new migration only |
| `internal/auth/handler.go` | Add audit.Log calls at success paths |
| `internal/forms/handler.go` | Add audit.Log calls |
| `internal/responses/handler.go` | Add audit.Log calls |
| `internal/workspace/handler.go` | Add audit.Log calls |
| `internal/invitation/handler.go` | Add audit.Log calls |
| `internal/server/server.go` | Register new route |
| `cmd/api/app/app.go` | Wire audit.Service |
| `sqlc.yaml` | Add audit.sql to queries |
| Sidebar nav component | Add Audit Log link |

---

## Verification

1. Run migration: `migrate up` → verify `audit_logs` table exists
2. Run `sqlc generate` → verify generated models include audit types
3. Login → check `audit_logs` for `auth.login` row with correct workspace/account IDs
4. Create + publish a form → check for `form.created` and `form.published` rows
5. View a response → check `response.viewed` row
6. Send an invite → check `invitation.sent` row
7. Visit `/audit` as owner → see all events
8. Visit `/audit` as member → see only own events
9. Test "Load more" / cursor pagination
10. Test action filter dropdown
