# Custom Domains v2: Domain-Aware Routing with Explicit DNS Verification

Replaces the Dynamic Traefik Routing model (v1) with Domain-Aware Routing backed by explicit CNAME + TXT record verification before enabling routing. Removes per-form domain toggle in favour of workspace-level domain enablement.

## Problems with v1

- Traefik dynamic config written per domain — tightly coupled to Traefik file I/O, no-ops silently in local dev
- Verification fires on the first inbound request — no DNS proof before routing traffic
- Let's Encrypt per domain via Traefik certResolver — incompatible with Cloudflare-proxied forms subdomain
- `VerifyCustomDomain` middleware performs a DB write inside request handling (side-effect in hot path)
- Per-form `use_custom_domain` toggle creates partial adoption states within a workspace

## Architecture

```
User browser (HTTPS)
        │
        ▼
Cloudflare Edge  (TLS for our own domains: app, forms subdomain)
        │  Host: forms.customer.com
        ▼
Traefik (static config — no per-domain changes ever)
   ├── Host(app.confide.com)         → app service   (priority: high)
   ├── Host(forms.confide.com)       → forms service (priority: medium)
   └── HostRegexp({host:.+})         → forms service (priority: 1 — catch-all)
        │
        ▼
FormsDomainGate middleware
   ├── forms.confide.com             → allow form paths
   ├── domain in Registry            → allow form paths
   └── unknown host                  → 421 Misdirected Request
```

Custom domain traffic path:
1. User CNAMEs `forms.customer.com → forms.confide.com`
2. DNS resolves to our server (forms subdomain must be DNS-only / grey cloud for Let's Encrypt HTTP-01 to work on custom domains)
3. Traefik catch-all receives the connection, issues a Let's Encrypt cert for `forms.customer.com`
4. Request reaches `FormsDomainGate`, which checks the in-memory `Registry`
5. Verified domains pass through; unknown domains get 421

Self-hosters front their own TLS-terminating proxy and configure it as needed.

## DNS Records Required from Users

```
CNAME  forms.example.com                   →  forms.confide.com
TXT    _confide-verify.forms.example.com   →  confide-verification=<token>
```

The `_confide-verify.` prefix keeps the verification record on a dedicated subdomain (same convention used by Google, Stripe, etc.) and avoids clobbering any existing TXT record on the domain root.

## Verification State Machine

```
stored
  cname_ok=F, txt_ok=F, enabled=F
      │
      │  worker: CheckCNAME + CheckTXT every 2 min
      ↓
dns verified
  cname_ok=T, txt_ok=T, enabled=F
      │
      │  worker: EnableCustomDomain + registry.Enable
      ↓
enabled
  enabled=T, verified_at=<timestamp>
```

Partial DNS states (one record verified, one pending) are stored so the UI can show per-record status. The worker stops polling once a domain is enabled. A recheck is triggered if the user removes and re-adds the domain.

## Implementation Plan

### Step 1 — Database migration

New `custom_domains` table. Drops `workspaces.custom_domain`, `workspaces.custom_domain_verified`, and `forms.use_custom_domain`.

```sql
-- migrations/0019_custom_domains_v2.up.sql

CREATE TABLE custom_domains (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain       TEXT NOT NULL UNIQUE,
    txt_token    TEXT NOT NULL,
    cname_ok     BOOLEAN NOT NULL DEFAULT FALSE,
    txt_ok       BOOLEAN NOT NULL DEFAULT FALSE,
    enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at  TIMESTAMPTZ
);

ALTER TABLE workspaces
    DROP COLUMN IF EXISTS custom_domain,
    DROP COLUMN IF EXISTS custom_domain_verified;

ALTER TABLE forms
    DROP COLUMN IF EXISTS use_custom_domain;
```

New SQL queries (`internal/db/queries/workspaces.sql`):

| Query | Purpose |
|---|---|
| `InsertCustomDomain` | Store new domain + generated TXT token |
| `GetCustomDomainByWorkspace` | Fetch current domain config for a workspace |
| `GetCustomDomainByHost` | Look up workspace by incoming hostname |
| `UpdateDNSStatus` | Write cname_ok + txt_ok from worker |
| `EnableCustomDomain` | Set enabled=T, verified_at=now |
| `DeleteCustomDomain` | Remove by workspace ID |
| `ListAllEnabledDomains` | Startup load for the registry |

### Step 2 — Domain Registry

`internal/domain/registry.go` — in-memory set of enabled custom domains. Direct replacement for `traefik.Writer` with no file I/O.

```go
type Registry struct {
    mu      sync.RWMutex
    domains map[string]struct{}
}

func NewRegistry(initial []string) *Registry
func (r *Registry) Enable(domain string)
func (r *Registry) Disable(domain string)
func (r *Registry) IsEnabled(domain string) bool
```

Populated at startup from `ListAllEnabledDomains()`. Updated only by the verification worker.

### Step 3 — DNS Verifier

`internal/domain/verifier.go` — pure DNS lookups, no external dependencies.

```go
type Verifier struct {
    cnameTarget string  // e.g. "forms.confide.com"
}

// CheckCNAME resolves domain's CNAME chain and checks it reaches cnameTarget.
func (v *Verifier) CheckCNAME(ctx context.Context, domain string) (bool, error)

// CheckTXT looks for "confide-verification=<token>" in TXT records on _confide-verify.<domain>.
func (v *Verifier) CheckTXT(ctx context.Context, domain, token string) (bool, error)
```

Uses `net.DefaultResolver` — no third-party DNS library needed.

### Step 4 — Verification Worker

`internal/domain/worker.go` — background goroutine, polls every 2 minutes (configurable via `CONFIDE_DOMAIN_VERIFY_INTERVAL`). Only processes domains where `enabled = FALSE`.

```go
type Worker struct {
    db       db.Querier
    verifier *Verifier
    registry *Registry
    interval time.Duration
}

func (w *Worker) Run(ctx context.Context)
```

On each tick: load all unverified domains → check CNAME + TXT → update DNS status → if both OK, enable domain and update registry.

### Step 5 — Middleware

**Delete** `internal/middleware/customdomain.go` entirely.

**Update** `internal/middleware/formsdomain.go`:

- Change `isVerifiedCustomDomain func(context.Context, string) bool` to `isEnabled func(string) bool` — context not needed for a pure in-memory lookup
- Return `421 Misdirected Request` for unknown hosts instead of redirecting (clearer for API clients; browsers also benefit from the explicit signal)

### Step 6 — API

**`GET /api/workspaces/{id}/custom-domain`** — expanded response:

```json
{
  "domain": "forms.example.com",
  "cnameRecord": {
    "type": "CNAME",
    "name": "forms.example.com",
    "value": "forms.confide.com"
  },
  "txtRecord": {
    "type": "TXT",
    "name": "_confide-verify.forms.example.com",
    "value": "confide-verification=abc123"
  },
  "cnameOK": false,
  "txtOK": false,
  "enabled": false
}
```

**`PUT /api/workspaces/{id}/custom-domain`** — generates a random TXT token and stores it alongside the domain. Returns the full record set immediately so the UI can display instructions without a second request.

**`POST /api/workspaces/{id}/custom-domain/verify`** — triggers one immediate DNS check outside the polling cycle. Returns current status. Backs the "Check now" button.

**`DELETE /api/workspaces/{id}/custom-domain`** — removes from DB and calls `registry.Disable`.

**Remove** `PUT /api/forms/{id}/custom-domain` — per-form toggle is gone. All forms in a workspace automatically use the workspace's enabled custom domain.

### Step 7 — Traefik static config

Add to `confide.yml` (or the static config file):

```yaml
http:
  routers:
    confide-custom-domain-catchall:
      rule: "HostRegexp(`{host:.+}`)"
      entryPoints: [websecure]
      service: confide
      tls:
        certResolver: letsencrypt
      priority: 1
```

`confide-custom-domains.yml` (the v1 dynamic file) can be deleted. `CONFIDE_TRAEFIK_DYNAMIC_DIR` is removed from config.

**Note:** for Let's Encrypt HTTP-01 challenges to succeed on custom domains, `forms.confide.com` must be DNS-only (grey cloud) in Cloudflare. The app and API subdomains can remain proxied.

### Step 8 — Config changes

Remove: `TraefikDynamicDir`

Add:
```
CONFIDE_DOMAIN_VERIFY_INTERVAL   # optional, default "2m"
```

`CONFIDE_CUSTOM_DOMAIN_TARGET` stays — the CNAME target shown to users.

### Step 9 — Frontend

**Settings page** — replace the single CNAME instruction with a two-record verification UI:

```
┌─ Custom Domain ──────────────────────────────────────┐
│ forms.example.com                          [Remove]  │
│                                                      │
│ Add these DNS records, then click Check:             │
│                                                      │
│  CNAME  forms.example.com                            │
│         → forms.confide.com              ✓ verified  │
│                                                      │
│  TXT    _confide-verify.forms.example.com            │
│         → confide-verification=abc123    ✗ pending   │
│                                                      │
│                              [Check now]             │
└──────────────────────────────────────────────────────┘
```

Once `enabled = true`, show the domain as an active link.

**Form edit page** — remove the "Use custom domain" toggle. The share URL automatically uses the workspace's enabled custom domain when present, same as the current verified-domain behaviour.

## Files Changed

### Deleted
- `internal/traefik/writer.go` (entire package)
- `internal/middleware/customdomain.go`

### Created
- `internal/domain/registry.go`
- `internal/domain/verifier.go`
- `internal/domain/worker.go`
- `migrations/0019_custom_domains_v2.up.sql`
- `migrations/0019_custom_domains_v2.down.sql`

### Updated
- `internal/config/config.go` — remove `TraefikDynamicDir`, add `DomainVerifyInterval`
- `internal/middleware/formsdomain.go` — simplified signature, 421 response
- `internal/server/server.go` — wire registry + worker; remove traefik writer and `VerifyCustomDomain`
- `internal/workspace/service.go` — rewrite custom domain methods against new schema
- `internal/workspace/handler.go` — add `/verify` endpoint, remove form toggle handler
- `internal/db/queries/workspaces.sql` + generated `workspaces.sql.go`
- `internal/forms/handler.go` — remove `setFormCustomDomain`
- `internal/forms/service.go` — remove `UseCustomDomain` from form records
- `web/src/lib/workspaces.ts` — updated types, add verify call
- `web/src/routes/(app)/settings/+page.svelte` — two-record UI + per-record status
- `web/src/routes/(app)/forms/[id]/edit/+page.svelte` — remove custom domain toggle
- `docs/016_forms_subdomain.md` — update to reflect new architecture

## Suggested Implementation Order

1. Migration + SQL queries
2. Registry
3. Verifier + Worker
4. Middleware updates
5. API + service layer
6. Server wiring
7. Frontend
