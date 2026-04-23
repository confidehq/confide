# Forms Subdomain & Custom Domain Hosting

## Goal

Serve public-facing forms from `forms.example.com` (and workspace custom domains) while
keeping the admin dashboard on `app.example.com`. A single Go binary handles all traffic;
Traefik routes by hostname and terminates TLS.

## Architecture

```
Internet
    │
Traefik (80/443, TLS termination, Let's Encrypt)
    │
    ├─ app.example.com ──────────────────┐
    ├─ forms.example.com ────────────────┤──► confide (single container :8080)
    └─ custom.domain.com (dynamic) ──────┘
```

All three domain types hit the same Go binary. Traefik handles certificate issuance per domain.
The Go backend inspects the `Host` header and enforces what each domain can access.

## Routing Rules

### Forms domain / custom domains — allowed paths

| Path | Purpose |
|---|---|
| `/f/*` | Public form page (SvelteKit SPA) |
| `/api/f/*` | Public form schema API |
| `/relay/submit` | Encrypted response submission |
| `/api/health` | Health check (monitoring) |
| `/api/config` | Runtime client config |
| `/_app/*` | SvelteKit asset chunks |
| `/*.js`, `/*.css`, `/*.svg`, etc. | Static assets |

All other paths → 302 redirect to the app domain at the same path.

### App domain — full access

No restrictions. Auth, dashboard, settings, workspace management, billing.

## New Config Variables

| Variable | Description |
|---|---|
| `CONFIDE_FORMS_DOMAIN` | Hostname for public form hosting (e.g. `forms.example.com`) |
| `CONFIDE_APP_DOMAIN` | Admin dashboard hostname (e.g. `app.example.com`) — already existed |

## Files Changed

### Backend

- `internal/config/config.go` — adds `FormsDomain` field
- `internal/middleware/formsdomain.go` — new `FormsDomainGate` middleware
- `internal/server/server.go` — wires the middleware; adds forms domain to CORS; adds `GET /api/config`

### Frontend

- `web/src/lib/config.ts` — fetches `/api/config` once and caches it
- `web/src/routes/(app)/forms/[id]/+page.svelte` — uses forms domain when generating share URL
- `web/src/routes/(app)/forms/[id]/edit/+page.svelte` — uses forms domain as URL fallback when no custom domain is set

### Deployment

- `deploy/traefik.yml` — Traefik static config (entrypoints, Let's Encrypt, file provider)
- `deploy/dynamic/confide.yml` — static Traefik routes for `app.*` and `forms.*` subdomains
- `deploy/docker-compose.yml` — production Compose (Traefik + Postgres + app)

## Traefik Dynamic Config Layout

```
deploy/dynamic/
├── confide.yml                  # edited by operator: routes for app + forms subdomains
└── confide-custom-domains.yml   # written by confide at runtime: per-workspace custom domains
```

Traefik watches the directory with `watch: true`. Custom domain routes appear within seconds
of a workspace verifying their domain — no Traefik restart required.

## TLS

Individual Let's Encrypt certificates are obtained per hostname via HTTP-01 challenge:
- `app.example.com` — issued on first request
- `forms.example.com` — issued on first request
- `custom.domain.com` — issued automatically by the Traefik writer after DNS verification

## Share URL Generation

Before: share URLs used `window.location.origin` (the admin app domain).

After: the frontend fetches `GET /api/config` on first publish to get `formsDomain`, then
constructs share URLs as `https://forms.example.com/f/{id}#rk={base64urlKey}`.

Custom-domain forms continue to use the workspace's verified custom domain.

## Deployment Quick Start

```bash
# 1. Edit deploy/dynamic/confide.yml — replace example.com domains with your own.

# 2. Edit deploy/traefik.yml — set your ACME email.

# 3. Set env vars in .env:
CONFIDE_APP_DOMAIN=https://app.example.com
CONFIDE_FORMS_DOMAIN=forms.example.com
CONFIDE_RP_ID=example.com
CONFIDE_RP_ORIGIN=https://app.example.com
CONFIDE_CORS_ORIGIN=https://app.example.com

# 4. Start:
docker compose -f deploy/docker-compose.yml up -d
```

## WebAuthn Note

`CONFIDE_RP_ID` must be set to the effective domain shared by all subdomains (e.g. `example.com`),
not a specific subdomain. This lets passkeys registered on `app.example.com` continue to work
after moving to a subdomain. Changing `RP_ID` after users have registered invalidates all existing
passkeys.
