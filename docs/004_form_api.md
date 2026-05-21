# Phase 4 — Form API + Relay Service

**Status:** Planning
**Exit criterion:** Encrypted schema stored; `renderEncryptedSchema` fetchable unauthenticated; submission relayed and stored; all verifiable end-to-end without the server ever seeing plaintext.

---

## Design decisions (resolved before build)

### DQ3 — Relay queue durability
**Decision: Accept in-memory queue + client retry.**
A relay crash loses all queued submissions that have not yet been flushed. The client retries up to 3× with exponential backoff (1s, 2s, 4s) before surfacing an error. No write-ahead log in v1. The 60-second flush interval keeps the loss window small. This matches the recommendation in the design doc.

### Relay co-located in the API binary
The relay queue and flush goroutine run inside the same process as the API. There is no separate `cmd/relay/` binary, no internal HTTP endpoint, and no inter-process auth secret. The flusher calls `responses.Service.CreateBatch` directly in-process. The `POST /relay/submit` route is mounted on the same `PORT` as all other routes.

### Schema versioning in Phase 4
Each `PUT /api/forms/{id}` increments `schema_version` atomically. No immutable snapshots in Phase 4 — that is a Phase 5/6 concern (DQ4). The current schema is always the live one. Responses carry the `schema_version` at time of submission for future use.

### Form ownership disclosure
Every authenticated form/response endpoint verifies `form.account_id == session.account_id`. Return **404** (not 403) on mismatch — do not confirm form existence to a non-owner.

### Response count integrity
`response_count` on the `forms` table is incremented in the same transaction as the batch response insert during flush. `UPDATE forms SET response_count = response_count + N WHERE id = $1`.

### Cursor pagination for responses
Responses are ordered `received_at DESC, id DESC`. The cursor is a base64-encoded JSON `{"d":"<date>","i":"<id>"}`. The query uses `WHERE (received_at, id) < ($cursor_date, $cursor_id)`. Page size: 50 (configurable by caller up to 200).

---

## Step 1 — Database migration (0004)

Replace the `SELECT 1` placeholder in `migrations/0004_forms_responses.up.sql`:

```sql
CREATE TABLE forms (
  id                        TEXT PRIMARY KEY,
  account_id                TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  created_at                DATE NOT NULL,
  updated_at                DATE NOT NULL,
  status                    TEXT NOT NULL DEFAULT 'open',
  schema_version            INTEGER NOT NULL DEFAULT 1,
  response_count            INTEGER NOT NULL DEFAULT 0,
  encrypted_schema          BYTEA NOT NULL,
  render_encrypted_schema   BYTEA NOT NULL,
  public_form_key           BYTEA NOT NULL
);

CREATE INDEX idx_forms_account_id ON forms(account_id);

CREATE TABLE responses (
  id                   TEXT PRIMARY KEY,
  form_id              TEXT NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
  received_at          DATE NOT NULL,
  schema_version       INTEGER NOT NULL,
  encrypted_data       BYTEA NOT NULL,
  ephemeral_public_key BYTEA NOT NULL
);

CREATE INDEX idx_responses_form_id_received ON responses(form_id, received_at DESC, id DESC);
```

Also write the corresponding `0004_forms_responses.down.sql`:
```sql
DROP TABLE IF EXISTS responses;
DROP TABLE IF EXISTS forms;
```

---

## Step 2 — SQLC queries

### `internal/db/queries/forms.sql`

```sql
-- name: CreateForm :one
INSERT INTO forms (id, account_id, created_at, updated_at, status, schema_version,
                   response_count, encrypted_schema, render_encrypted_schema, public_form_key)
VALUES ($1, $2, CURRENT_DATE, CURRENT_DATE, 'open', 1, 0, $3, $4, $5)
RETURNING *;

-- name: GetFormByOwner :one
SELECT * FROM forms WHERE id = $1 AND account_id = $2;

-- name: GetFormPublic :one
SELECT id, status, schema_version, render_encrypted_schema, public_form_key
FROM forms WHERE id = $1;

-- name: ListFormsByAccount :many
SELECT id, status, schema_version, response_count, created_at, updated_at
FROM forms WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateFormSchema :one
UPDATE forms
SET encrypted_schema = $3,
    render_encrypted_schema = $4,
    schema_version = schema_version + 1,
    updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2
RETURNING schema_version;

-- name: UpdateFormStatus :exec
UPDATE forms SET status = $3, updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2;

-- name: DeleteForm :exec
DELETE FROM forms WHERE id = $1 AND account_id = $2;

-- name: IncrementResponseCount :exec
UPDATE forms SET response_count = response_count + $2 WHERE id = $1;
```

### `internal/db/queries/responses.sql`

```sql
-- name: CreateResponse :exec
INSERT INTO responses (id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key)
VALUES ($1, $2, CURRENT_DATE, $3, $4, $5);

-- name: ListResponses :many
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key
FROM responses
WHERE form_id = $1
  AND (received_at, id) < ($2, $3)
ORDER BY received_at DESC, id DESC
LIMIT $4;

-- name: GetResponse :one
SELECT * FROM responses WHERE id = $1 AND form_id = $2;

-- name: DeleteResponse :exec
DELETE FROM responses WHERE id = $1 AND form_id = $2;

-- name: FormExists :one
SELECT id FROM forms WHERE id = $1;
```

Run `sqlc generate` after writing these.

---

## Step 3 — `internal/forms/` package

### `service.go`

```go
type Service struct {
    db   DB        // queries.Queries interface (same pattern as auth)
    pool *pgxpool.Pool
}

// Request/response types
type CreateFormRequest struct {
    EncryptedSchema        []byte
    RenderEncryptedSchema  []byte
    PublicFormKey          []byte
}

type FormRecord struct {
    ID                    string
    AccountID             string
    CreatedAt             time.Time
    UpdatedAt             time.Time
    Status                string
    SchemaVersion         int32
    ResponseCount         int32
    EncryptedSchema       []byte
    RenderEncryptedSchema []byte
    PublicFormKey         []byte
}

type FormSummary struct {
    ID            string
    Status        string
    SchemaVersion int32
    ResponseCount int32
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type PublicFormRecord struct {
    ID                   string
    Status               string
    SchemaVersion        int32
    RenderEncryptedSchema []byte
    PublicFormKey        []byte
}

type UpdateSchemaRequest struct {
    EncryptedSchema       []byte
    RenderEncryptedSchema []byte
}

// Methods
func (s *Service) CreateForm(ctx context.Context, accountID string, req CreateFormRequest) (string, error)
func (s *Service) GetForm(ctx context.Context, accountID, formID string) (FormRecord, error)
func (s *Service) ListForms(ctx context.Context, accountID string) ([]FormSummary, error)
func (s *Service) UpdateFormSchema(ctx context.Context, accountID, formID string, req UpdateSchemaRequest) (int32, error)
func (s *Service) UpdateFormStatus(ctx context.Context, accountID, formID, status string) error
func (s *Service) DeleteForm(ctx context.Context, accountID, formID string) error
func (s *Service) GetPublicSchema(ctx context.Context, formID string) (PublicFormRecord, error)
```

Error sentinel: `ErrNotFound` (reuse or define in this package — 404 for both "doesn't exist" and "wrong owner").
Form ID generation: same as account ID — `crypto/rand` 16 bytes → base64url.

### `handler.go`

Mount under auth middleware for `/api/forms/*`:

| Method | Path | Handler |
|--------|------|---------|
| POST | `/api/forms` | createForm |
| GET | `/api/forms` | listForms |
| GET | `/api/forms/{id}` | getForm |
| PUT | `/api/forms/{id}` | updateFormSchema |
| PUT | `/api/forms/{id}/status` | updateFormStatus |
| DELETE | `/api/forms/{id}` | deleteForm |

Mount separately (no auth, with rate limiter and `Cache-Control: no-store`):

| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/f/{id}/schema` | getPublicSchema |

All binary fields (`encryptedSchema`, `renderEncryptedSchema`, `publicFormKey`) are transmitted as `base64.StdEncoding` strings in JSON, consistent with the existing auth API convention.

Request body for `POST /api/forms` and `PUT /api/forms/{id}`:
```json
{
  "encryptedSchema": "<base64>",
  "renderEncryptedSchema": "<base64>",
  "publicFormKey": "<base64>"
}
```

Response for `GET /api/f/{id}/schema`:
```json
{
  "renderEncryptedSchema": "<base64>",
  "publicFormKey": "<base64>",
  "schemaVersion": 1,
  "status": "open"
}
```

`Cache-Control: no-store, no-cache` on the public schema endpoint. No `Set-Cookie`. No logging of request body.

---

## Step 4 — `internal/responses/` package

### `service.go`

```go
type Service struct {
    db   DB
    pool *pgxpool.Pool
}

type ResponseCursor struct {
    Date string // "2006-01-02"
    ID   string
}

type ResponseRecord struct {
    ID                 string
    FormID             string
    ReceivedAt         time.Time
    SchemaVersion      int32
    EncryptedData      []byte
    EphemeralPublicKey []byte
}

type ListResponsesResult struct {
    Responses  []ResponseRecord
    NextCursor *ResponseCursor // nil if last page
}

// Methods
func (s *Service) ListResponses(ctx context.Context, accountID, formID string, cursor *ResponseCursor, limit int) (ListResponsesResult, error)
func (s *Service) GetResponse(ctx context.Context, accountID, formID, responseID string) (ResponseRecord, error)
func (s *Service) DeleteResponse(ctx context.Context, accountID, formID, responseID string) error

// Used by internal flush only — no account ownership check (formID verified by FK)
func (s *Service) CreateBatch(ctx context.Context, items []CreateResponseItem) error
```

`ListResponses` and `GetResponse` verify ownership by joining through forms: use `GetFormByOwner` first, then query responses. Or add a SQL query that joins — whichever is cleaner.

`CreateBatch` runs in a transaction: inserts all response rows, then calls `IncrementResponseCount` per form (group by formID, batch the counts).

### `handler.go`

Mounted under auth middleware alongside forms:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/forms/{id}/responses` | listResponses |
| GET | `/api/forms/{id}/responses/{rid}` | getResponse |
| DELETE | `/api/forms/{id}/responses/{rid}` | deleteResponse |

Query params for pagination: `?limit=50&after=<cursor>`. Cursor is base64 of `{"d":"2025-03-14","i":"abc123"}`.

Response fields `encryptedData` and `ephemeralPublicKey` are base64-encoded strings.

### Internal flush handler

Separate handler function (not mounted as a public route):
```
POST /internal/flush
Authorization: Bearer <RELAY_API_SECRET>
```

Body:
```json
{
  "submissions": [
    {
      "formId": "...",
      "encryptedData": "<base64>",
      "ephemeralPublicKey": "<base64>",
      "schemaVersion": 1
    }
  ]
}
```

Returns 200 `{"stored": N}` or 207 with partial errors.

The handler validates the auth header, decodes each item, calls `responses.Service.CreateBatch`. Forms that don't exist in the DB are silently dropped (the relay accepted the submission; we cannot surface an error to the respondent at flush time).

---

## Step 5 — `internal/relay/` — In-process relay

The relay lives inside the API binary. No separate process, no inter-process communication. Structure:

```
internal/relay/
├── queue.go
└── flusher.go
```

### `queue.go`

```go
type SubmissionItem struct {
    FormID             string
    EncryptedData      []byte
    EphemeralPublicKey []byte
    SchemaVersion      int32
}

type Queue struct {
    mu    sync.Mutex
    items []SubmissionItem
}

func (q *Queue) Enqueue(item SubmissionItem)
func (q *Queue) Drain() []SubmissionItem  // atomically swap, return old slice
func (q *Queue) Len() int
```

### `flusher.go`

```go
func StartFlusher(ctx context.Context, q *Queue, svc *responses.Service, interval time.Duration)
```

- Ticker at `interval` (default 60s, from `CONFIDE_RELAY_FLUSH_INTERVAL`).
- On tick: `items := q.Drain()`. If empty, skip.
- Calls `svc.CreateBatch(ctx, items)` directly — no HTTP, no serialization.
- On error: log count of dropped items (not content). No retry — client retry is the recovery mechanism.
- On success: log count flushed.
- Graceful shutdown: on `ctx.Done()`, do one final flush before exiting.

---

## Step 6 — API server updates

### `internal/server/server.go`

Inject `forms.Service`, `responses.Service`, and `relay.Queue` (alongside existing `auth.Service`):

```go
func New(
    auth *auth.Service,
    forms *forms.Service,
    responses *responses.Service,
    relayQueue *relay.Queue,
    cfg config.Config,
) http.Handler
```

Mount new routes:

```go
// Authenticated form + response routes
r.Group(func(r chi.Router) {
    r.Use(middleware.Authenticator(auth))
    r.Mount("/api/forms", formsRouter(forms, responses))
})

// Unauthenticated public schema — Cache-Control: no-store, no cookies
r.With(publicSchemaRateLimiter).Get("/api/f/{id}/schema", forms.GetPublicSchemaHandler(forms))

// Relay intake — no auth, no cookies, rate limited, no access logs
r.With(relayRateLimiter).Post("/relay/submit", relay.SubmitHandler(relayQueue))
```

### `cmd/api/main.go`

```go
q := &relay.Queue{}
formsvc := forms.NewService(db, pool)
ressvc := responses.NewService(db, pool)

go relay.StartFlusher(ctx, q, ressvc, cfg.RelayFlushInterval)

srv := server.New(authsvc, formsvc, ressvc, q, cfg)
```

### Rate limiters

- Public schema (`/api/f/{id}/schema`): 100 req/min per rotating IP hash.
- Relay submit (`/relay/submit`): 20 submissions / 10 min per rotating IP hash.
- Both reuse the existing `ratelimit.go` constructor with different window parameters.

### Access logs on `/relay/submit`

The relay submit route must **not** have Chi's logger middleware applied. Mount it outside any logger middleware group, or use a sub-router that skips it. The existing security headers middleware still applies.

---

## Step 7 — Frontend scaffolding

The crypto layer is already complete (Phase 1). Phase 4 adds the API client and two minimal route pages.

### `web/src/lib/forms.ts`

```typescript
// All functions assume auth store has valid masterKey and session cookie is set.

export async function createForm(
    masterKey: CryptoKey,
    schema: FormSchema
): Promise<{ formId: string; renderKey: CryptoKey }>

export async function getForm(
    masterKey: CryptoKey,
    formId: string
): Promise<{ schema: FormSchema; summary: FormSummary }>

export async function listForms(): Promise<FormSummary[]>

export async function updateFormSchema(
    masterKey: CryptoKey,
    formId: string,
    schema: FormSchema,
    existingRenderKey: CryptoKey
): Promise<{ schemaVersion: number }>

export async function updateFormStatus(
    formId: string,
    status: 'open' | 'closed'
): Promise<void>

export async function deleteForm(formId: string): Promise<void>

// Unauthenticated — for respondents
export async function getPublicSchema(
    formId: string,
    renderKey: CryptoKey  // parsed from URL fragment #rk=<base64url>
): Promise<{ schema: FormSchema; status: string }>

// Unauthenticated — for respondents
export async function submitResponse(
    formId: string,
    publicFormKeyBytes: ArrayBuffer,
    payload: ResponsePayload
): Promise<void>

export async function listResponses(
    formId: string,
    cursor?: string
): Promise<{ responses: EncryptedResponseRecord[]; nextCursor?: string }>

export async function decryptResponseRecord(
    masterKey: CryptoKey,
    formId: string,
    record: EncryptedResponseRecord
): Promise<ResponsePayload>
```

`createForm` flow:
1. Generate random `formId` (16 bytes, base64url).
2. Derive `formKey = deriveFormKey(masterKey, formId)`.
3. Derive `keypair = deriveFormKeypair(formKey)`.
4. `encryptedSchema = encryptSchema(schema, formKey)`.
5. Generate `renderKey` (AES-GCM, random, extractable).
6. `renderEncryptedSchema = encryptSchema(schema, renderKey)`.
7. Export `publicFormKey` as raw bytes.
8. POST `/api/forms` with all four blobs base64-encoded.
9. Return `{ formId, renderKey }`.

`submitResponse` flow:
1. Import `publicFormKeyBytes` as X25519 CryptoKey.
2. `{ encryptedData, ephemeralPublicKey } = encryptResponse(payload, publicFormKey)`.
3. POST `/relay/submit` with base64-encoded blobs. No cookies, no auth header.
4. Retry 3× on failure (1s, 2s, 4s backoff).

### `web/src/routes/f/[id]/+page.svelte`

Generic shell page. No SSR data. On mount:
1. Parse `#rk=<base64url>` from `window.location.hash`.
2. If absent: show "Invalid link" state.
3. Import raw bytes as AES-GCM key (`crypto.subtle.importKey`).
4. `getPublicSchema(formId, renderKey)`.
5. If status `'closed'`: show "This form is no longer accepting responses."
6. Otherwise: display schema title (Phase 6 will render the full form).

`+page.ts`:
```typescript
export const ssr = false;
export const prerender = false;
```

No `Set-Cookie`. Referrer-Policy: `no-referrer` (set via security headers middleware on `/api/f/*` and `/f/*` routes).

### `web/src/routes/(app)/forms/+page.svelte`

Minimal list view:
- Fetch `listForms()` on mount.
- Display: form ID (truncated), status, response count, created date.
- Buttons: toggle status, delete (with confirm dialog).
- "New Form" button → placeholder alert "Form builder coming in Phase 5."
- Replace dashboard placeholder with a link to `/forms`.

### `web/src/routes/(app)/dashboard/+page.svelte`

Replace current placeholder with a link to `/forms`.

---

## Step 8 — Config additions

### `.env.example`

```
# Relay (runs inside the API process)
CONFIDE_RELAY_FLUSH_INTERVAL=60s
```

### `internal/config/config.go`

Add to API config:
```go
RelayFlushInterval time.Duration // CONFIDE_RELAY_FLUSH_INTERVAL, default 60s
```

### `Makefile`

No new targets needed — `make dev` runs the single binary that includes the relay.

### `deploy/docker-compose.yml`

No relay service needed — the existing `api` service handles `/relay/submit`. Add the new env var to the `api` service:
```yaml
CONFIDE_RELAY_FLUSH_INTERVAL: 60s
```

---

## Testing exit criterion

All of these must be verifiable without any server decryption:

1. **Schema stored encrypted:** `SELECT encode(encrypted_schema, 'hex') FROM forms LIMIT 1;` — should be opaque bytes. `SELECT encode(render_encrypted_schema, 'hex') FROM forms LIMIT 1;` — also opaque.

2. **Public schema fetchable unauthenticated:**
   ```bash
   curl -s http://localhost:8080/api/f/<formId>/schema | jq .renderEncryptedSchema
   ```
   Returns a base64 blob. No auth required. No Set-Cookie in response headers.

3. **Submission stored encrypted:**
   ```bash
   curl -s -X POST http://localhost:8080/relay/submit \
     -H 'Content-Type: application/json' \
     -d '{"formId":"...","encryptedData":"...","ephemeralPublicKey":"...","schemaVersion":1}'
   # Wait 60s for flush, then:
   SELECT encode(encrypted_data, 'hex'), encode(ephemeral_public_key, 'hex') FROM responses LIMIT 1;
   ```

4. **No plaintext in DB:**
   ```bash
   pg_dump confide | grep -i "title\|label\|question\|answer"
   # Should return nothing from response/schema content
   ```

5. **Relay logs contain no submission content:**
   - Relay stdout/stderr should show only flush counts, not form IDs or payload content.

6. **Response decryptable by creator:**
   - Client-side: `decryptResponse(encryptedData, ephemeralPublicKey, formPrivateKey)` returns the original payload.

---

## File checklist

**New files:**
- `migrations/0004_forms_responses.up.sql` (replace placeholder)
- `migrations/0004_forms_responses.down.sql`
- `internal/db/queries/forms.sql`
- `internal/db/queries/responses.sql`
- `internal/forms/service.go`
- `internal/forms/handler.go`
- `internal/responses/service.go`
- `internal/responses/handler.go`
- `internal/relay/queue.go`
- `internal/relay/flusher.go`
- `internal/relay/handler.go`
- `web/src/lib/forms.ts`
- `web/src/routes/f/[id]/+page.svelte`
- `web/src/routes/f/[id]/+page.ts`
- `web/src/routes/(app)/forms/+page.svelte`

**Modified files:**
- `migrations/0004_forms_responses.up.sql` — replace SELECT 1
- `internal/server/server.go` — mount new routes, inject new services + relay queue
- `cmd/api/main.go` — instantiate forms + responses services, start relay flusher goroutine
- `internal/config/config.go` — add RelayFlushInterval
- `web/src/routes/(app)/dashboard/+page.svelte` — add link to /forms
- `.env.example` — relay flush interval var
- `deploy/docker-compose.yml` — add CONFIDE_RELAY_FLUSH_INTERVAL to api service

**Run after SQL changes:**
```bash
sqlc generate
```
