# Phase 2 — Go API Skeleton + Database

**Exit criterion:** Full signup → login → recovery → session revocation flow working via curl/integration tests. No form endpoints yet.

---

## Key findings from existing repo

- Module: `github.com/phantompunk/wisp`, Go 1.25.5
- `cmd/`, `internal/`, `migrations/` are all empty — greenfield
- No existing dependencies in `go.mod`

---

## Repository structure after Phase 2

```
├── cmd/api/main.go
├── internal/
│   ├── config/config.go
│   ├── db/db.go  +  queries/{auth.sql.go,models.go}
│   ├── auth/{handler,service,webauthn,session,recovery}.go
│   ├── middleware/{auth,ratelimit,security_headers}.go
│   └── server/server.go
├── migrations/
│   ├── 0001_accounts.{up,down}.sql
│   ├── 0002_recovery_codes.{up,down}.sql
│   ├── 0003_sessions.{up,down}.sql
│   └── 0004_forms_responses.{up,down}.sql  ← placeholder
├── sqlc.yaml
├── deploy/{docker-compose.yml,Dockerfile.api}
└── .env.example
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-webauthn/webauthn v0.11.x` | WebAuthn server verification, challenge management |
| `github.com/jackc/pgx/v5` | PostgreSQL driver (native, no `database/sql`) |
| `github.com/golang-migrate/migrate/v4` | Migration runner with embedded FS |
| `sqlc-dev/sqlc v1.27.x` | SQL→Go codegen (tool, not runtime dep) |
| `github.com/go-chi/chi/v5` | HTTP router (stdlib-compatible) |
| `github.com/go-chi/httprate` | Rotating IP hash rate limiter |

---

## Database Schema

### `accounts`

```sql
CREATE TABLE accounts (
    id                      TEXT  PRIMARY KEY,          -- 22-char random base64url
    created_at              DATE  NOT NULL,             -- date-only (threat model §1.4)
    credential_id           BYTEA NOT NULL UNIQUE,      -- WebAuthn credential ID
    public_key              BYTEA NOT NULL,             -- COSE-encoded public key
    prf_salt                BYTEA NOT NULL,             -- 32 bytes, server-generated
    wrapped_master_key      BYTEA NOT NULL,             -- 40 bytes (AES-KW of 32-byte key)
    recovery_wrapped_master BYTEA NOT NULL,             -- 40 bytes
    recovery_verifier       BYTEA NOT NULL              -- 32 bytes: SHA-256(recoveryKey)
);

CREATE INDEX idx_accounts_credential_id ON accounts (credential_id);
```

### `recovery_codes`

```sql
CREATE TABLE recovery_codes (
    id          TEXT    PRIMARY KEY,
    account_id  TEXT    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    code_hash   BYTEA   NOT NULL,    -- SHA-256(normalised code), 32 bytes
    used        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  DATE    NOT NULL
);

CREATE INDEX idx_recovery_codes_account_id ON recovery_codes (account_id);
CREATE INDEX idx_recovery_codes_hash       ON recovery_codes (code_hash);
```

### `sessions`

```sql
CREATE TABLE sessions (
    id          TEXT  PRIMARY KEY,
    account_id  TEXT  NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL UNIQUE,   -- SHA-256(256-bit random token)
    created_at  DATE  NOT NULL,
    last_seen   DATE  NOT NULL           -- rolling; expire if > 30 days ago
);

CREATE INDEX idx_sessions_account_id ON sessions (account_id);
CREATE INDEX idx_sessions_token_hash ON sessions (token_hash);
```

### `forms_responses` (placeholder)

```sql
-- Phase 4 will add forms and responses tables.
SELECT 1;
```

**Schema notes:**

- All IDs are application-generated random base64url strings — no `SERIAL` or `BIGSERIAL`. Sequential IDs leak creation order and record count to a partial-read attacker.
- All binary fields are `BYTEA`. Base64 encoding before storage adds CPU cost, increases size ~33%, and creates encoding version risk. pgx handles `BYTEA` ↔ `[]byte` natively.
- All timestamps are `DATE` (date-only). Coarsened per threat model §1.4 to avoid timing correlation attacks.
- AES-KW overhead: wrapping a 32-byte AES-256 key produces exactly 40 bytes (RFC 3394: N×8 + 8). The `wrapped_master_key` and `recovery_wrapped_master` fields are always 40 bytes. `recovery_verifier`, `token_hash`, and `code_hash` are always 32 bytes (SHA-256 output).

---

## sqlc Configuration

`sqlc.yaml` at the repo root:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/db/queries/"
    schema: "migrations/"
    gen:
      go:
        package: "queries"
        out: "internal/db/queries"
        emit_json_tags: false
        emit_db_tags: false
        emit_prepared_queries: false
        null_style: "option"
```

Key queries (`internal/db/queries/auth.sql`):

```sql
-- name: CreateAccount :one
INSERT INTO accounts (
    id, created_at, credential_id, public_key, prf_salt,
    wrapped_master_key, recovery_wrapped_master, recovery_verifier
) VALUES (
    $1, CURRENT_DATE, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetAccountByCredentialID :one
SELECT * FROM accounts WHERE credential_id = $1;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, account_id, token_hash, created_at, last_seen)
VALUES ($1, $2, $3, CURRENT_DATE, CURRENT_DATE)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.*, a.id AS account_id, a.wrapped_master_key
FROM sessions s
JOIN accounts a ON a.id = s.account_id
WHERE s.token_hash = $1
  AND s.last_seen > CURRENT_DATE - INTERVAL '30 days';

-- name: TouchSession :exec
UPDATE sessions SET last_seen = CURRENT_DATE WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: ListSessionsByAccount :many
SELECT id, created_at, last_seen FROM sessions WHERE account_id = $1;

-- name: CreateRecoveryCodes :copyfrom
INSERT INTO recovery_codes (id, account_id, code_hash, used, created_at)
VALUES ($1, $2, $3, false, CURRENT_DATE);

-- name: GetUnusedRecoveryCode :one
SELECT * FROM recovery_codes
WHERE account_id = $1 AND code_hash = $2 AND used = FALSE
LIMIT 1;

-- name: BurnRecoveryCode :exec
UPDATE recovery_codes SET used = TRUE WHERE id = $1;

-- name: CountUnusedRecoveryCodes :one
SELECT COUNT(*) FROM recovery_codes WHERE account_id = $1 AND used = FALSE;
```

---

## Configuration

All config from environment variables. Missing required vars cause an immediate fatal error before any port opens.

| Variable | Default | Required |
|----------|---------|----------|
| `CONFIDE_DATABASE_URL` | — | ✓ |
| `CONFIDE_BIND_ADDR` | `:8080` | |
| `CONFIDE_CORS_ORIGIN` | `http://localhost:3000` | |
| `CONFIDE_HMAC_KEY` | — | ✓ (base64url-encoded 32 bytes) |
| `CONFIDE_RP_ID` | `localhost` | |
| `CONFIDE_RP_ORIGIN` | `http://localhost:3000` | |
| `CONFIDE_RP_DISPLAY_NAME` | `Confide` | |
| `CONFIDE_ENV` | `development` | |

---

## API Endpoints

```
POST   /auth/register/begin
POST   /auth/register/finish
POST   /auth/login/begin          ← accepts { credentialIdBase64 }, returns prfSalt
POST   /auth/login/finish
POST   /auth/recover
POST   /auth/logout               ← session required
GET    /auth/sessions             ← session required
DELETE /auth/sessions/:id         ← session required

GET    /health
```

### Register flow

**Begin:** Server generates a random `accountId` (22-char base64url) and 32-byte `prfSalt`. Calls `webauthn.BeginRegistration` with PRF extension. Returns `{ accountId, challenge, prfSalt }`. Challenge stored in in-memory store keyed by `accountId`.

**Finish:** Client sends `{ accountId, credential, wrappedMasterKey, recoveryWrappedMasterKey, recoveryVerifier, recoveryCodes[12 hashes] }`. Server verifies attestation, writes `accounts` row, writes 12 `recovery_codes` rows, issues session cookie, returns `{ accountId }`.

### Login flow (single-round)

Client stores `credentialId` in `localStorage` after registration.

**Begin:** Accepts `{ credentialIdBase64 }`. Looks up account, includes `prfSalt` in assertion challenge PRF extension. Returns WebAuthn challenge.

**Finish:** Verifies assertion. Returns session cookie + `{ wrappedMasterKey }` so the client can unwrap the master key using the PRF output.

This avoids the complexity of a two-round PRF discovery flow. The `credentialId` is not PII.

### Recovery

Input: `{ accountId, code }`. Steps:
1. Normalise code (uppercase, strip hyphens).
2. Compute `SHA-256(normalised)`.
3. Look up matching unused `recovery_codes` row.
4. If not found → 401 (rate-limited: 5 attempts / 5 min).
5. Burn the code (`used = TRUE`). Return `{ recoveryWrappedMasterKey }`.
6. No session issued — client must register a new passkey after recovery (Phase 3 UX).

### Binary encoding

All binary fields in JSON bodies use standard base64 (matching `@simplewebauthn/browser`). The DB stores raw `BYTEA`. The service layer encodes/decodes at the boundary.

### Session tokens

`crypto/rand.Read(32 bytes)` → base64url-encode for cookie → store `SHA-256(token)` in DB only.

Cookie: `HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=2592000` (30 days, rolling).

Expiry enforced in SQL: `last_seen > CURRENT_DATE - INTERVAL '30 days'`.

---

## HTTP Middleware Stack

Applied globally, outermost to innermost:

1. **`security_headers`** — `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store`
2. **`ratelimit`** (on `/auth/*`) — `HMAC-SHA256(clientIP, rotatingKey)` key function. Key rotates every 15 minutes using two slots (current + previous); requests valid against either slot are accepted. Raw IP never persisted beyond the HMAC call.
3. **`auth`** (session middleware, authenticated routes only) — reads session cookie → `SHA-256(token)` → DB lookup → attaches `accountId` to `context.Context`. Returns 401 if absent, invalid, or expired.

Error responses: JSON `{ "code": "machine_readable_string", "message": "human text" }`. Go error messages never appear in production responses.

---

## WebAuthn Configuration

```go
wconfig := &webauthn.Config{
    RPID:          cfg.RPID,
    RPDisplayName: cfg.RPDisplayName,
    RPOrigins:     []string{cfg.RPOrigin},
    AuthenticatorSelection: protocol.AuthenticatorSelection{
        UserVerification:   protocol.VerificationRequired,
        RequireResidentKey: protocol.ResidentKeyRequirementRequired,
    },
    Timeout: 60000,
    Debug:   cfg.Env == "development",
}
```

Challenge store: in-memory map with 5-minute TTL. Phase 7 swaps this for a Redis-backed store for multi-instance deployments.

---

## Docker Compose

`deploy/docker-compose.yml`:

```yaml
version: "3.9"

services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: confide
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: confide
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U confide"]
      interval: 5s
      timeout: 3s
      retries: 10
    networks: [internal]

  api:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.api
    restart: unless-stopped
    depends_on:
      db:
        condition: service_healthy
    environment:
      CONFIDE_DATABASE_URL: postgresql://confide:${DB_PASSWORD}@db:5432/confide?sslmode=disable
      CONFIDE_BIND_ADDR: :8080
      CONFIDE_CORS_ORIGIN: ${CORS_ORIGIN:-http://localhost:3000}
      CONFIDE_HMAC_KEY: ${HMAC_KEY}
      CONFIDE_RP_ID: ${RP_ID:-localhost}
      CONFIDE_RP_ORIGIN: ${RP_ORIGIN:-http://localhost:3000}
      CONFIDE_RP_DISPLAY_NAME: ${RP_DISPLAY_NAME:-Confide}
      CONFIDE_ENV: ${ENV:-development}
    ports:
      - "8080:8080"
    networks: [internal, external]

  relay:
    image: busybox
    command: ["sh", "-c", "echo 'relay: Phase 4' && sleep infinity"]
    networks: [external]

  web:
    image: busybox
    command: ["sh", "-c", "echo 'web: Phase 3' && sleep infinity"]
    networks: [external]

volumes:
  pgdata:

networks:
  internal:
    internal: true
  external:
```

`deploy/Dockerfile.api`:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api ./cmd/api

FROM scratch
COPY --from=builder /build/api /api
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/api"]
```

Migrations are embedded in the binary via `//go:embed`. No external filesystem access at runtime. Binary runs from `scratch` base image.

---

## Testing Strategy

**No mock databases.** All integration tests hit a real PostgreSQL instance at `CONFIDE_TEST_DATABASE_URL` (default: `postgresql://confide:confide@localhost:5432/confide_test`).

**Transaction-per-test:** Each test runs in a transaction deferred `Rollback()`. No table truncation between tests.

**WebAuthn:** The `webauthn.WebAuthn` interface is injected as a dependency. Integration tests use a mock that returns pre-computed valid credentials. No browser required.

Test file locations:

```
internal/
├── auth/
│   ├── service_test.go     ← business logic (mock DB interface)
│   └── handler_test.go     ← HTTP handlers (httptest.NewRecorder)
├── db/
│   └── db_test.go          ← migrations smoke test, query compilation
└── middleware/
    └── ratelimit_test.go   ← key rotation, limit enforcement
```

Coverage targets for Phase 2:

- Registration: happy path, tampered attestation → 400, duplicate credential → 409
- Login: valid assertion, wrong credential → 401, expired session → 401
- Recovery: valid code, burned code → 401, rate limit enforcement
- Sessions: list, touch (rolling last_seen), revoke specific session, revoke via logout
- Middleware: security headers present, rate limit 429 after threshold, session cookie missing → 401

---

## Implementation Sequence

| Step | Deliverable | Acceptance criterion |
|------|-------------|----------------------|
| 1 | `go.mod` — add all deps | `go mod tidy` clean, `go build ./...` succeeds |
| 2 | `internal/config/config.go` | Missing required var returns error; unit test green |
| 3 | `migrations/*.sql` | Manual `psql` apply succeeds; no syntax errors |
| 4 | `internal/db/db.go` | Migration test against test DB green |
| 5 | `sqlc.yaml` + `internal/db/queries/auth.sql` | `sqlc generate` produces compiling Go |
| 6 | `internal/auth/service.go` (stubs) + test file | All tests red (stubs), test structure complete |
| 7 | `internal/auth/webauthn.go` + register impl | Registration tests green |
| 8 | Login + session issuance | Login and session tests green |
| 9 | Recovery + code burn | Recovery tests green |
| 10 | `internal/auth/handler.go` + middleware + router | Handler tests green |
| 11 | `cmd/api/main.go` | Server starts, connects to DB, runs migrations |
| 12 | `deploy/Dockerfile.api` + `docker-compose.yml` | `curl /health` returns `{"status":"ok"}` |
| 13 | Full integration test | register → login → list sessions → logout → recover all pass |

---

## Design Decisions

**sqlc over GORM/ent** — Schema is the authoritative source of truth. Generated code is fully auditable. No runtime reflection, no struct tag magic.

**chi over gin/echo/fiber** — Handlers are stdlib `http.HandlerFunc`. Tests use `httptest` directly. No framework-specific test helpers.

**golang-migrate with embedded FS** — Binary carries its own migrations. Self-hosters running the Docker image need no separate migration step. Postgres wraps each migration in a transaction (DDL is transactional in Postgres).

**SHA-256 for session tokens, not bcrypt** — Session tokens are 256-bit random values. bcrypt and argon2 solve brute-forcing cheap secrets; they add no security when the secret already has 256 bits of entropy.

**`DATE` not `TIMESTAMPTZ`** — Coarsened per threat model §1.4 to prevent timing correlation across events.

**Per-account `prfSalt`** — In v1, one account has exactly one credential. When Phase v2 adds multiple passkeys per account, a `credentials` table with per-credential `prf_salt` will be introduced via migration.

---

## Out of Scope for Phase 2

- Form endpoints (`/api/forms/*`) — Phase 4
- Response endpoints — Phase 4
- Relay process (`cmd/relay/`) — Phase 4
- SvelteKit UI — Phase 3
- `publicFormKey` / encrypted schema storage — Phase 4
- `responses` table DDL — Phase 4
- Multi-instance Redis challenge store — Phase 7
