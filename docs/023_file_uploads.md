# Plan: File Upload Support (Photos, Videos, Files)

## Context

Confide needs to support file attachments on form responses. Public form respondents should be able to attach photos, videos, and documents alongside their encrypted form submission. Files must follow the same end-to-end encryption model already in place: encrypted in the browser before the server receives any bytes. Storage is pluggable (local disk default, S3-compatible optional). Upload size and total workspace storage are capped based on plan tier.

**Constraints:**
- Uploaders: public form respondents only (unauthenticated, like `/relay/submit`)
- Storage: local disk default, S3-compatible configurable via env
- Free plan: 10 MB/file, 500 MB total workspace storage
- Pro plan: 50 MB/file, 10 GB total workspace storage

---

## Architecture

### E2E Encryption
Files follow the same X25519 + AES-256-GCM pattern as form responses:
1. Browser already has the form's `publicFormKey` from `/api/f/{id}/schema`
2. Browser generates ephemeral X25519 key pair per upload
3. ECDH key agreement → AES-256-GCM key (via HKDF, same as response encryption)
4. File bytes encrypted in browser; metadata (filename, MIME type, plaintext size) encrypted separately with the same key
5. Server receives only ciphertext + `ephemeralPublicKey` — never sees plaintext

### Upload Flow (Two-Phase)
1. **Upload phase**: Browser encrypts file → `POST /relay/upload` → server returns `attachmentId` (UUID)
2. **Submit phase**: Browser includes `attachmentIds: ["abc", ...]` as a plaintext field in the existing relay submit JSON payload alongside `formId`, `encryptedData`, etc.
3. Server links attachments to the response record during `CreateBatch`

The attachment IDs are just UUIDs — no plaintext file content is ever exposed.

### Quota Check (Before Accepting Upload)
The handler must know the workspace plan before streaming:
1. Query `forms JOIN workspaces` by `formId` → get `workspace_id` + `plan`
2. Look up `workspace_storage_usage.bytes_used`
3. Reject with HTTP 507 (Insufficient Storage) if upload would exceed quota
4. After successful write, atomically increment `bytes_used` via upsert

---

## Database Changes

New migration (`migrations/0017_attachments.up.sql`):

```sql
CREATE TABLE attachments (
    id                   TEXT PRIMARY KEY,
    form_id              TEXT NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    response_id          TEXT REFERENCES responses(id) ON DELETE SET NULL,
    encrypted_metadata   BYTEA NOT NULL,      -- encrypted {filename, mimeType, plaintextSize}
    ephemeral_public_key BYTEA NOT NULL,
    storage_backend      TEXT NOT NULL DEFAULT 'local',
    storage_key          TEXT NOT NULL,       -- relative path or S3 key
    encrypted_size       BIGINT NOT NULL,     -- bytes stored on disk/S3
    uploaded_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at           TIMESTAMPTZ,         -- mirrors form response_ttl
    linked_at            TIMESTAMPTZ          -- when response_id was set
);

CREATE INDEX idx_attachments_form_id  ON attachments(form_id);
CREATE INDEX idx_attachments_response ON attachments(response_id) WHERE response_id IS NOT NULL;
CREATE INDEX idx_attachments_expires  ON attachments(expires_at)  WHERE expires_at IS NOT NULL;

CREATE TABLE workspace_storage_usage (
    workspace_id  TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    bytes_used    BIGINT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

New sqlc queries (`internal/db/queries/attachments.sql`):
- `InsertAttachment` — insert new unlinked attachment
- `LinkAttachmentsToResponse` — bulk `UPDATE response_id WHERE id = ANY($1) AND form_id = $2`
- `GetAttachment` — by ID, join form to get workspace_id for auth checks
- `DeleteAttachment` — by ID
- `GetExpiredAttachments` — `WHERE expires_at < NOW()`, paginated (for reaper)
- `UpsertWorkspaceStorageUsage` — atomic increment via `INSERT ON CONFLICT DO UPDATE`
- `DecrementWorkspaceStorageUsage` — called on delete
- `GetWorkspaceStorageUsage` — for the stats endpoint

---

## New Package: `internal/storage`

```
internal/storage/
  storage.go       — Backend interface
  local/
    local.go       — filesystem implementation
  s3/
    s3.go          — S3-compatible implementation (AWS SDK v2)
  factory.go       — NewBackend(cfg) dispatches on CONFIDE_STORAGE_BACKEND
```

**Interface:**
```go
type Backend interface {
    Put(ctx context.Context, key string, r io.Reader, size int64) error
    Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
    Delete(ctx context.Context, key string) error
}
```

**Local backend**: writes to `CONFIDE_STORAGE_DIR` (default `./data/uploads`). Files stored as `{formId}/{attachmentId}` to keep form data co-located.

**S3 backend**: AWS SDK v2 `PutObject`/`GetObject`/`DeleteObject`. Supports MinIO and Cloudflare R2 via custom endpoint.

---

## New Package: `internal/attachments`

```
internal/attachments/
  service.go   — business logic (quota check, store, link, delete, purge)
  handler.go   — HTTP handlers
```

**Plan limits (constants in service.go):**

| | Free | Pro |
|---|---|---|
| Max file size | 10 MB | 50 MB |
| Total workspace storage | 500 MB | 10 GB |

### Handlers

**`POST /relay/upload`** — unauthenticated, open CORS, rate-limited

Request: `multipart/form-data`
| Field | Type | Description |
|---|---|---|
| `formId` | text | Form identifier |
| `encryptedData` | base64 | Encrypted file bytes (AES-256-GCM) |
| `encryptedMetadata` | base64 | Encrypted `{filename, mimeType, plaintextSize}` |
| `ephemeralPublicKey` | base64 | X25519 ephemeral public key |
| `loadToken` | text | Optional bot guard velocity token |

Response `202`:
```json
{ "attachmentId": "abc123" }
```

Errors: `400` bad request, `413` file too large, `507` quota exceeded, `429` rate limit

**`GET /api/attachments/{id}`** — authenticated, workspace member required

Streams encrypted blob from storage backend. `Content-Type: application/octet-stream`.

**`DELETE /api/attachments/{id}`** — authenticated, workspace admin or owner required

**`GET /api/workspaces/{id}/storage`** — authenticated, workspace member required

```json
{ "bytesUsed": 1048576, "bytesLimit": 524288000, "plan": "free" }
```

---

## Changes to Existing Files

### `internal/relay/handler.go`
Add optional `attachmentIds []string` to the relay submit request struct. After `CreateBatch` succeeds, call `attachmentSvc.Link(ctx, req.AttachmentIds, req.FormID, responseID)`.

### `internal/relay/queue.go` (`SubmissionItem`)
Add `AttachmentIDs []string` field.

### `internal/responses/service.go` (`CreateBatch`)
Return the newly-created `responseID` so the relay handler can pass it to `Link`.

### `internal/middleware/ratelimit.go`
Add `UploadRateLimit`: 10 uploads per 10 minutes per IP (tighter than `RelayRateLimit`).

### `internal/reaper/reaper.go`
Add `attachmentSvc.PurgeExpired(ctx)` in the reaper loop. Purge deletes from the storage backend first, then removes the DB row, then decrements `workspace_storage_usage`.

### `internal/server/server.go`
- `POST /relay/upload` — open CORS + `UploadRateLimit` (alongside existing relay routes)
- `GET /api/attachments/{id}` + `DELETE /api/attachments/{id}` — authenticated routes
- `GET /api/workspaces/{id}/storage` — authenticated, workspace member

### `internal/config/config.go`
Add storage config fields (see Environment Variables below).

### `cmd/api/app/app.go`
Initialize storage backend and attachment service; inject into server.

---

## Frontend Changes (`ui/`)

### Form Builder
- New field type: `file` with `allowedTypes` (images / video / documents / any) and `maxFiles` options
- Field config encrypted with the rest of the form schema

### Form Renderer (Public)
- File picker with drag-and-drop
- Client-side size validation before encrypting (user-friendly error)
- Per-file: encrypt with Web Crypto → `POST /relay/upload` → collect `attachmentId`
- Pass `attachmentIds` alongside encrypted payload in relay submit
- Upload progress indicator

### Response Viewer (Authenticated)
- Detect attachment IDs in decrypted response payload
- `GET /api/attachments/{id}` → decrypt in-browser → offer download or inline preview
- Image thumbnail preview via `createObjectURL` after decryption

### Workspace Settings
- Storage usage meter using `GET /api/workspaces/{id}/storage`

---

## New Environment Variables

| Variable | Default | Description |
|---|---|---|
| `CONFIDE_STORAGE_BACKEND` | `local` | `local` or `s3` |
| `CONFIDE_STORAGE_DIR` | `./data/uploads` | Root directory for local storage |
| `CONFIDE_S3_ENDPOINT` | _(empty)_ | Custom endpoint for MinIO / R2 |
| `CONFIDE_S3_BUCKET` | _(required if s3)_ | S3 bucket name |
| `CONFIDE_S3_REGION` | `us-east-1` | S3 region |
| `CONFIDE_S3_ACCESS_KEY_ID` | _(required if s3)_ | S3 access key |
| `CONFIDE_S3_SECRET_ACCESS_KEY` | _(required if s3)_ | S3 secret key |

---

## Implementation Phases

1. **DB + sqlc** — migration file → `sqlc generate` → verify generated models
2. **Storage package** — interface + local backend + S3 backend + factory
3. **Attachments package** — service + handlers (upload, download, delete, storage stats)
4. **Relay changes** — `attachmentIds` in submit payload, link after response creation
5. **Server wiring** — new routes, rate limiter, config, app init
6. **Reaper extension** — attachment expiry purge
7. **Frontend** — file field type, renderer encryption, response viewer decryption, storage meter

---

## Verification

1. Create a form with a file upload field
2. Open the public form as an unauthenticated user — attach a photo → submit
3. Verify: `attachments` row exists, `workspace_storage_usage.bytes_used` incremented, encrypted blob on disk
4. Log in as workspace owner → open response → download attachment → confirm decrypted file matches original
5. Test 10 MB free limit: submit a file over 10 MB on a free workspace → expect `413`
6. Test quota: fill storage past 500 MB → expect `507`
7. Test reaper: set `expires_at` to the past → run reaper → file deleted from disk + DB row gone
8. Test S3 backend: `CONFIDE_STORAGE_BACKEND=s3` with local MinIO → repeat upload/download
