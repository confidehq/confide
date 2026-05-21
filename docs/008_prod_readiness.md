# Phase 8 — Production Readiness: Branding, Deployment, and Open Source

**Status:** Planning
**Exit criterion:** Confide can be self-hosted by cloning the repo, editing a single `.env` file, and running one `docker compose up` command. CORS and CSP are properly configured. Environment variables are consistently namespaced under `CONFIDE_*`. The root URL shows a real landing page. The project has a LICENSE, README, and responsible disclosure policy.

---

## Problem

The project currently has a combination of rough edges that block a production launch and public release:

1. **Namespace pollution** — all env vars are prefixed `CONFIDE_*`, leaking the old working name. Self-hosters will set `CONFIDE_DATABASE_URL` and have a confusing experience.
2. **No registration gate** — a self-hoster cannot lock their instance after creating their first account. Anyone who can reach the URL can register.
3. **CORS is loaded but never applied** — `cfg.CORSOrigin` is read in `config.go` but the middleware is never attached in `server.go`. Cross-origin requests from the web frontend fail in any environment where the API and frontend are on different ports or subpaths.
4. **No CSP header** — `SecurityHeaders` sets `X-Frame-Options` and friends but not `Content-Security-Policy`. The public form page (`/f/*`) especially needs a strict CSP.
5. **Placeholder frontend services** — `docker-compose.yml` still has busybox placeholders for `relay` and `web`. There is no `Dockerfile.web`.
6. **No reverse proxy** — the compose stack exposes the Go binary directly, which means no TLS. WebAuthn requires HTTPS in any non-localhost context.
7. **No public landing page** — the root route immediately redirects to `/login`, giving a blank flash for unauthenticated visitors and making the project look unfinished.
8. **No self-hosting documentation** — there is nothing telling a new operator how to configure and run Confide.
9. **No LICENSE, README, or SECURITY.md** — the project cannot be meaningfully open-sourced without these.

---

## Scope

- Backend: env var rename, registration control, CORS middleware, CSP headers.
- Frontend: landing page, `Dockerfile.web`.
- Infra: complete `docker-compose.yml`, new `Caddyfile`.
- Docs: `docs/self-hosting.md`.
- Open source: `LICENSE`, `README.md`, `SECURITY.md`.

**Deferred:** CI/CD pipelines, email notifications, multi-user support, CSV export, Kubernetes, Tor improvements.

---

## Step 1 — Rename `CONFIDE_*` env vars to `CONFIDE_*`

**File:** `internal/config/config.go`

Replace every `os.Getenv("CONFIDE_...")` and `getEnv("CONFIDE_...", ...)` call with the `CONFIDE_*` equivalent. Also add the new `RegistrationOpen` field (covered in Step 2 below):

Updated `Config` struct:

```go
type Config struct {
    DatabaseURL        string
    BindAddr           string
    CORSOrigin         string
    HMACKey            []byte
    RPID               string
    RPOrigin           string
    RPDisplayName      string
    Env                string
    RelayFlushInterval time.Duration
    RegistrationOpen   bool
}
```

Updated `Load()` function — full rename map:

| Old env var | New env var | Default |
|---|---|---|
| `CONFIDE_DATABASE_URL` | `CONFIDE_DATABASE_URL` | (required) |
| `CONFIDE_SECRET_KEY` | `CONFIDE_SECRET_KEY` | (required) |
| `PORT` | `PORT` | `:8080` |
| `CONFIDE_CORS_ORIGIN` | `CONFIDE_CORS_ORIGIN` | `http://localhost:3000` |
| `CONFIDE_RELAY_FLUSH_INTERVAL` | `CONFIDE_RELAY_FLUSH_INTERVAL` | `60s` |
| `CONFIDE_RP_ID` | `CONFIDE_RP_ID` | `localhost` |
| `CONFIDE_RP_ORIGIN` | `CONFIDE_RP_ORIGIN` | `http://localhost:3000` |
| `CONFIDE_RP_DISPLAY_NAME` | `CONFIDE_RP_DISPLAY_NAME` | `Confide` |
| `CONFIDE_ENV` | `CONFIDE_ENV` | `development` |
| (new) | `CONFIDE_REGISTRATION_OPEN` | `true` |

The `flushInterval` block near the top of `Load()` currently reads `CONFIDE_RELAY_FLUSH_INTERVAL` — update that first, before the struct literal:

```go
if v := os.Getenv("CONFIDE_RELAY_FLUSH_INTERVAL"); v != "" {
    if d, err := time.ParseDuration(v); err == nil {
        flushInterval = d
    }
}
```

Error messages in the required-field validation blocks must also be updated to reference the new names:

```go
errs = append(errs, errors.New("CONFIDE_DATABASE_URL is required"))
// ...
errs = append(errs, fmt.Errorf("CONFIDE_SECRET_KEY must be base64url-encoded 32 bytes"))
```

**File:** `.env.example`

Replace the entire file content. Retain the same structure (required section, optional section, docker section) but rename all keys and update the display name default:

```
# Required
CONFIDE_DATABASE_URL=postgresql://wisp:changeme@localhost:5432/wisp?sslmode=disable
# Generate with: openssl rand -base64 32 | tr '+/' '-_'
CONFIDE_SECRET_KEY=

# Optional (defaults shown)
PORT=8080
CONFIDE_RELAY_FLUSH_INTERVAL=60s
CONFIDE_CORS_ORIGIN=http://localhost:3000
CONFIDE_RP_ID=localhost
CONFIDE_RP_ORIGIN=http://localhost:3000
CONFIDE_RP_DISPLAY_NAME=Wisp
CONFIDE_ENV=development
CONFIDE_REGISTRATION_OPEN=true

# Self-hosting (used by docker-compose.yml)
CONFIDE_DOMAIN=wisp.example.com
DB_PASSWORD=changeme
```

Note: `CONFIDE_SECRET_KEY` in `.env.example` is for local dev where `godotenv.Load()` picks it up directly. In the compose stack, `api` uses `CONFIDE_SECRET_KEY: ${CONFIDE_SECRET_KEY}` — set the same key in `.env` and it flows through.

---

## Step 2 — Registration control (`CONFIDE_REGISTRATION_OPEN`)

**File:** `internal/config/config.go`

Parse `CONFIDE_REGISTRATION_OPEN` in `Load()`. Use a helper to read boolean strings with a safe default of `true`:

```go
cfg.RegistrationOpen = parseBool(os.Getenv("CONFIDE_REGISTRATION_OPEN"), true)
```

Add the private helper (at the bottom of `config.go`, alongside `getEnv`):

```go
// parseBool parses "true"/"1"/"yes" as true, "false"/"0"/"no" as false.
// If the value is empty or unrecognized, defaultVal is returned.
func parseBool(s string, defaultVal bool) bool {
    switch strings.ToLower(strings.TrimSpace(s)) {
    case "true", "1", "yes":
        return true
    case "false", "0", "no":
        return false
    default:
        return defaultVal
    }
}
```

Add `"strings"` to the import block.

**File:** `internal/auth/handler.go`

The `Handler` function currently has signature:

```go
func Handler(svc *Service, recoveryHMACKey []byte, dev bool) http.Handler
```

Add a `registrationOpen bool` parameter:

```go
func Handler(svc *Service, recoveryHMACKey []byte, dev bool, registrationOpen bool) http.Handler
```

Pass `registrationOpen` down to `registerBegin`:

```go
r.Post("/register/begin", registerBegin(svc, registrationOpen))
```

Update `registerBegin` to accept and enforce the flag:

```go
func registerBegin(svc *Service, registrationOpen bool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !registrationOpen {
            writeError(w, http.StatusForbidden, "registration_closed", "registration is closed")
            return
        }
        res, err := svc.RegisterBegin(r.Context())
        // ... rest unchanged
    }
}
```

**File:** `internal/server/server.go`

Update the auth handler mount to pass `cfg.RegistrationOpen`:

```go
r.Mount("/", auth.Handler(svc.Auth, cfg.HMACKey, cfg.Env == "development", cfg.RegistrationOpen))
```

---

## Step 3 — Wire CORS middleware

**File:** `internal/server/server.go`

The current code loads `cfg.CORSOrigin` but never applies it. Add a proper CORS handler using `github.com/go-chi/cors`.

First check if it is already in `go.mod`:

```bash
grep cors go.mod
```

If not present, add the dependency:

```bash
go get github.com/go-chi/cors
```

In `server.go`, import `"github.com/go-chi/cors"`.

Apply CORS to all `/api/*` routes via a route group. The relay endpoint requires permissive CORS (respondents arrive from arbitrary origins) — handle it separately.

Updated route structure in `New()`:

```go
// CORS for API routes — restricted to the configured app origin.
r.Route("/api", func(r chi.Router) {
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{cfg.CORSOrigin},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Content-Type", "Authorization"},
        AllowCredentials: false,
        MaxAge:           300,
    }))
    r.Use(mw.AppCSP)

    // Auth routes — general rate limit.
    r.Route("/auth", func(r chi.Router) {
        r.Use(mw.RateLimit(cfg.HMACKey))
        r.Mount("/", auth.Handler(svc.Auth, cfg.HMACKey, cfg.Env == "development", cfg.RegistrationOpen))
    })

    // Authenticated form + response routes.
    r.Group(func(r chi.Router) {
        r.Use(mw.Authenticator(svc.Auth))
        r.Mount("/forms", forms.Handler(svc.Forms))
        r.Route("/forms/{formId}/responses", func(r chi.Router) {
            r.Mount("/", responses.Handler(svc.Responses))
        })
    })

    // Public unauthenticated schema endpoint — stricter CSP.
    r.With(mw.FormPageCSP, mw.PublicSchemaRateLimit(cfg.HMACKey)).
        Get("/f/{id}/schema", forms.PublicSchemaHandler(svc.Forms))
})

// Relay submit — open CORS (respondents arrive from arbitrary origins).
r.With(
    cors.Handler(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"POST", "OPTIONS"},
        AllowedHeaders:   []string{"Content-Type"},
        AllowCredentials: false,
        MaxAge:           300,
    }),
    mw.RelayRateLimit(cfg.HMACKey),
).Post("/relay/submit", relay.SubmitHandler(svc.RelayQ))
```

The absolute URL paths (`/api/auth/...`, `/api/forms/...`, `/api/f/...`) do not change — only the grouping in the Go router changes. The frontend makes no API call URL changes.

---

## Step 4 — CSP headers

**File:** `internal/middleware/security_headers.go`

Add two new middleware functions alongside the existing `SecurityHeaders`:

```go
// AppCSP adds a general Content-Security-Policy for API and app routes.
func AppCSP(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        next.ServeHTTP(w, r)
    })
}

// FormPageCSP adds a strict Content-Security-Policy for the public form
// schema endpoint. This is the privacy-critical surface — no external
// resources of any kind are permitted.
func FormPageCSP(next http.Handler) http.Handler {
    const csp = "default-src 'self'; " +
        "script-src 'self'; " +
        "style-src 'self'; " +
        "img-src 'self' data:; " +
        "connect-src 'self'; " +
        "frame-ancestors 'none'"
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy", csp)
        next.ServeHTTP(w, r)
    })
}
```

`AppCSP` is applied at the `/api` route group level. `FormPageCSP` is applied via `r.With(mw.FormPageCSP, ...)` on the public schema route — it runs after `AppCSP` in the middleware chain and its `Set` overwrites the earlier value. This is the correct behavior: the schema endpoint gets the stricter policy.

---

## Step 5 — Landing page (`web/src/routes/+page.svelte`)

Replace the current file with a real public landing page. Authenticated users still redirect to `/dashboard`; unauthenticated visitors see the landing page directly.

No external resources — no CDN fonts, no remote images, no third-party scripts. Use existing CSS variables from `app.css`.

```svelte
<script lang="ts">
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth.svelte';

  // Logged-in users go straight to the dashboard.
  if (typeof window !== 'undefined' && auth.accountId) {
    goto('/dashboard');
  }
</script>

<svelte:head>
  <title>Confide — Private, end-to-end encrypted forms</title>
  <meta name="description" content="Create private forms. Only you can read the responses." />
</svelte:head>

<main class="landing">
  <section class="hero">
    <h1>Confide</h1>
    <p class="tagline">Private forms. End-to-end encrypted.</p>
    <div class="cta-group">
      <a href="/register" class="btn btn-primary">Create an account</a>
      <a href="/login" class="btn btn-ghost">Sign in</a>
    </div>
  </section>

  <section class="features">
    <div class="feature">
      <span class="icon" aria-hidden="true">🚫</span>
      <h2>Zero tracking</h2>
      <p>No IP logs. No cookies for respondents. No fingerprinting. Nothing.</p>
    </div>
    <div class="feature">
      <span class="icon" aria-hidden="true">🔒</span>
      <h2>End-to-end encrypted</h2>
      <p>Responses are encrypted in your browser before they leave the page. The server never sees plaintext.</p>
    </div>
    <div class="feature">
      <span class="icon" aria-hidden="true">🖥️</span>
      <h2>Self-hostable</h2>
      <p>Open source. One <code>docker compose up</code>. Your data, your server, your rules.</p>
    </div>
  </section>

  <footer class="landing-footer">
    <a href="https://github.com/phantompunk/wisp" rel="noopener">GitHub</a>
    <span aria-hidden="true">·</span>
    <a href="/docs/self-hosting">Self-hosting guide</a>
  </footer>
</main>
```

Use scoped `<style>` for layout (`.landing`, `.hero`, `.features`, `.feature`, `.landing-footer`). Reference only existing CSS variables — do not add new global variables. Keep styles minimal: centered column, readable whitespace, consistent with the rest of the app.

Confirm that the root `+layout.svelte` does not contain an auth guard that would redirect unauthenticated users before the landing page can render. If it does, move that guard into the `(app)` group layout only.

---

## Step 6 — `deploy/Dockerfile.web`

New file at `deploy/Dockerfile.web`. Build context is the repo root (same as `Dockerfile.api`).

```dockerfile
# ── Build stage ────────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /build

# Enable pnpm via corepack (ships with Node 22).
RUN corepack enable && corepack prepare pnpm@latest --activate

# Install dependencies first (layer-cached separately from source).
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# Copy source and build.
COPY web/ .
RUN pnpm build

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM node:22-alpine

WORKDIR /app

ENV NODE_ENV=production

COPY --from=builder /build/build ./build
COPY --from=builder /build/package.json .

EXPOSE 3000

ENTRYPOINT ["node", "build"]
```

Notes:
- `pnpm install --frozen-lockfile` fails loudly if `pnpm-lock.yaml` is stale.
- `COPY web/ .` after the dependency layer means source changes don't bust the install cache.
- SvelteKit adapter-node outputs a Node.js server. `node build` resolves via the `main` field in `package.json`. If `main` is not set, use `node build/index.js` explicitly.
- The runtime container has no build tools — only the `build/` output and `package.json`.

---

## Step 7 — Complete `deploy/docker-compose.yml`

Replace the existing file entirely:

```yaml
services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: wisp
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: wisp
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wisp"]
      interval: 5s
      timeout: 3s
      retries: 10
    networks:
      - internal

  api:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.api
    restart: unless-stopped
    depends_on:
      db:
        condition: service_healthy
    environment:
      CONFIDE_DATABASE_URL: postgresql://wisp:${DB_PASSWORD}@db:5432/wisp?sslmode=disable
      PORT: 8080
      CONFIDE_SECRET_KEY: ${CONFIDE_SECRET_KEY}
      CONFIDE_CORS_ORIGIN: https://${CONFIDE_DOMAIN}
      CONFIDE_RP_ID: ${CONFIDE_DOMAIN}
      CONFIDE_RP_ORIGIN: https://${CONFIDE_DOMAIN}
      CONFIDE_RP_DISPLAY_NAME: ${CONFIDE_RP_DISPLAY_NAME:-Confide}
      CONFIDE_ENV: production
      CONFIDE_REGISTRATION_OPEN: ${CONFIDE_REGISTRATION_OPEN:-true}
    networks:
      - internal
      - external

  web:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.web
    restart: unless-stopped
    environment:
      ORIGIN: https://${CONFIDE_DOMAIN}
      PUBLIC_API_URL: ""
      BODY_SIZE_LIMIT: 1M
    networks:
      - external

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    environment:
      CONFIDE_DOMAIN: ${CONFIDE_DOMAIN}
    depends_on:
      - api
      - web
    networks:
      - external

volumes:
  pgdata:
  caddy_data:
  caddy_config:

networks:
  internal:
    internal: true
  external:
```

Key decisions:
- The `relay` service is removed — it is built into the `api` binary.
- `PUBLIC_API_URL: ""` means the browser uses relative paths (`/api/...`), which works when Caddy routes everything through a single domain. Operators who want a separate API domain can override this.
- `api` is on both networks: `internal` to reach `db`, `external` so Caddy can proxy it. `db` is `internal` only — not reachable from outside.
- `caddy_data` and `caddy_config` volumes persist TLS certificates across restarts. Without these, Caddy re-requests certs from Let's Encrypt on every `up` and will hit rate limits.
- The `api` service uses explicit `environment:` entries — no `env_file`. Local dev still uses `godotenv.Load()` which reads `.env` directly.

---

## Step 8 — `deploy/Caddyfile`

New file at `deploy/Caddyfile`:

```
{$CONFIDE_DOMAIN} {
    reverse_proxy /api/* api:8080
    reverse_proxy /relay/* api:8080
    reverse_proxy /* web:3000
}
```

`{$CONFIDE_DOMAIN}` is Caddy's environment variable interpolation syntax. Caddy reads it from the `caddy` service's `environment:` block in the compose file.

Caddy automatically provisions a TLS certificate from Let's Encrypt. Ports 80 and 443 must be reachable from the internet for the ACME HTTP-01 challenge to succeed.

Routes are evaluated top-to-bottom: `/api/*` and `/relay/*` hit the API first; everything else goes to the SvelteKit frontend.

---

## Step 9 — `docs/self-hosting.md`

New file — see content below. The guide must be self-contained: a new operator should be able to follow it without reading anything else.

### Prerequisites

- Docker 24+ and Docker Compose v2 (`docker compose`, not `docker-compose`)
- A domain with an A/AAAA record pointing to your server's public IP
- Ports 80 and 443 open inbound

### Why HTTPS is required

WebAuthn (passkeys) require a Secure Context. The browser enforces this — only HTTPS (or localhost) allows passkey operations. Caddy provisions TLS certificates automatically via Let's Encrypt.

### Setup steps

1. Clone the repo and copy `.env.example` to `.env`
2. Generate HMAC key: `openssl rand -base64 32 | tr '+/' '-_'`
3. Set `CONFIDE_DOMAIN`, `CONFIDE_SECRET_KEY`, `DB_PASSWORD`, `CONFIDE_RP_ID`, `CONFIDE_RP_ORIGIN` in `.env`
4. `docker compose -f deploy/docker-compose.yml up -d`
5. Open `https://your-domain.com/register` and create your account
6. Set `CONFIDE_REGISTRATION_OPEN=false` in `.env`, then `docker compose -f deploy/docker-compose.yml restart api`

### Upgrading

```bash
git pull
docker compose -f deploy/docker-compose.yml up --build -d
```

Migrations run automatically on API startup.

### Backup

```bash
docker compose -f deploy/docker-compose.yml exec db \
  pg_dump -U wisp wisp | gzip > wisp-backup-$(date +%Y%m%d).sql.gz
```

### Environment variable reference

| Variable | Type | Default | Description |
|---|---|---|---|
| `CONFIDE_DATABASE_URL` | string | required | PostgreSQL connection string |
| `CONFIDE_SECRET_KEY` | base64url | required | 32-byte HMAC key for session tokens |
| `CONFIDE_DOMAIN` | string | — | Public domain; used by compose to configure Caddy and RP vars |
| `PORT` | string | `:8080` | API listen address inside container |
| `CONFIDE_CORS_ORIGIN` | string | `http://localhost:3000` | Allowed CORS origin for `/api/*` |
| `CONFIDE_RP_ID` | string | `localhost` | WebAuthn RP ID — must match your domain |
| `CONFIDE_RP_ORIGIN` | string | `http://localhost:3000` | WebAuthn RP origin — must be `https://` + domain |
| `CONFIDE_RP_DISPLAY_NAME` | string | `Confide` | Display name in passkey prompts |
| `CONFIDE_ENV` | string | `development` | Set to `production` for Secure cookies |
| `CONFIDE_RELAY_FLUSH_INTERVAL` | duration | `60s` | How often the relay queue flushes to Postgres |
| `CONFIDE_REGISTRATION_OPEN` | bool | `true` | Set to `false` to close new account registration |
| `DB_PASSWORD` | string | `changeme` | Postgres password (used by compose) |

---

## Step 10 — Open source hygiene

### `LICENSE`

MIT License. Year: 2026. Author: phantompunk.

### `README.md`

At repo root:

```markdown
# Confide

**Private forms. End-to-end encrypted.**

Confide is an open-source form builder where responses are encrypted in the respondent's browser before they reach the server. Only the form creator can read them.

## Features

- Passkey (WebAuthn) authentication — no passwords, no email
- End-to-end encrypted responses using WebCrypto AES-GCM
- No IP logging, no respondent cookies, no tracking
- Multi-language form support
- Self-hosted with a single `docker compose up`

## Quick start

```bash
git clone https://github.com/phantompunk/wisp.git
cd wisp
cp .env.example .env
# Edit .env: set CONFIDE_DOMAIN, CONFIDE_SECRET_KEY, DB_PASSWORD
docker compose -f deploy/docker-compose.yml up -d
```

Open `https://your-domain.com/register` to create your account.

See the [self-hosting guide](docs/self-hosting.md) for full setup instructions.

## Architecture

The Go API handles authentication, form storage, and response collection. Responses are queued by a relay endpoint and flushed to Postgres in encrypted form — the API never decrypts them. The SvelteKit frontend handles all encryption and decryption in the browser using the WebCrypto API. Caddy provides automatic TLS termination.

## Security

To report a vulnerability, use [GitHub Security Advisories](https://github.com/phantompunk/wisp/security/advisories/new). See [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
```

### `SECURITY.md`

At repo root:

```markdown
# Security Policy

## Reporting a vulnerability

Do **not** open a public GitHub issue for security vulnerabilities.

Use GitHub's private vulnerability reporting:
[Report a vulnerability](https://github.com/phantompunk/wisp/security/advisories/new)

## Response timeline

| Milestone | Target |
|---|---|
| Acknowledgement | Within 48 hours |
| Initial assessment | Within 7 days |
| Fix or mitigation | Within 90 days |
| Public disclosure | After fix is released |

## Scope

In scope: Go API, cryptographic primitives, authentication, relay, SvelteKit frontend, deploy configuration.

Out of scope: social engineering, DoS, vulnerabilities in upstream dependencies (report those to the upstream project).

## Recognition

Reporters of confirmed vulnerabilities will be credited in the patch release notes, unless they prefer anonymity.
```

---

## Summary of Changed Files

| File | Change |
|---|---|
| `internal/config/config.go` | Rename `CONFIDE_*` → `CONFIDE_*`; add `RegistrationOpen bool`; add `parseBool` helper; import `"strings"` |
| `internal/auth/handler.go` | Add `registrationOpen bool` param to `Handler`; add guard in `registerBegin` |
| `internal/middleware/security_headers.go` | Add `AppCSP` and `FormPageCSP` middleware funcs |
| `internal/server/server.go` | Wrap routes in `r.Route("/api", ...)` group; apply `cors.Handler` per group; apply CSP middlewares; pass `cfg.RegistrationOpen` to `auth.Handler` |
| `.env.example` | Rename all vars; add `CONFIDE_REGISTRATION_OPEN`, `CONFIDE_DOMAIN`; update display name default to `Confide` |
| `deploy/docker-compose.yml` | Remove `relay` busybox; replace `web` busybox with real build; add `caddy` service; rename all vars; rename Postgres user/db to `wisp`; add `caddy_data` + `caddy_config` volumes |
| `deploy/Dockerfile.web` | New — multi-stage SvelteKit build |
| `deploy/Caddyfile` | New — single-domain reverse proxy template |
| `web/src/routes/+page.svelte` | Replace redirect-only with real landing page; keep `goto('/dashboard')` for authenticated users |
| `docs/self-hosting.md` | New — complete operator guide |
| `LICENSE` | New — MIT |
| `README.md` | New — project readme |
| `SECURITY.md` | New — responsible disclosure policy |

---

## Testing exit criterion

1. **Env var rename:** Start the API with `CONFIDE_DATABASE_URL` and `CONFIDE_SECRET_KEY` set, `CONFIDE_*` vars absent. Verify startup succeeds. Remove `CONFIDE_DATABASE_URL` — verify fatal error names `CONFIDE_DATABASE_URL`.

2. **Registration lock:** Set `CONFIDE_REGISTRATION_OPEN=false`. `POST /api/auth/register/begin` → 403 `{"code":"registration_closed","message":"registration is closed"}`. Set to `true` → begin succeeds.

3. **CORS — API routes:** From origin matching `CONFIDE_CORS_ORIGIN`, preflight `OPTIONS /api/auth/login/begin` → `Access-Control-Allow-Origin` matches the configured origin. From a different origin → no CORS header returned.

4. **CORS — relay:** From any origin, `OPTIONS /relay/submit` → `Access-Control-Allow-Origin: *`.

5. **CSP — API routes:** `GET /api/forms` (authenticated) → `Content-Security-Policy: default-src 'self'` present.

6. **CSP — public schema:** `GET /api/f/{id}/schema` → strict CSP with `frame-ancestors 'none'` present.

7. **Landing page:** `GET /` without session → landing page renders (not redirect to `/login`). With valid session → redirects to `/dashboard`.

8. **Full compose stack:** On a real server with a domain and open ports 80/443:
   - `docker compose -f deploy/docker-compose.yml up -d` starts all services without error.
   - `https://your-domain.com` loads the landing page (Caddy TLS provisioned automatically).
   - `https://your-domain.com/register` completes passkey registration.
   - `https://your-domain.com/api/health` returns `{"status":"ok"}`.
   - Postgres port 5432 is not reachable from outside the host.

9. **Dockerfile.web builds cleanly:** `docker build -f deploy/Dockerfile.web -t wisp-web .` from repo root completes without error.
