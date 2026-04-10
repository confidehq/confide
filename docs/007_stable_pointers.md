# Phase 7 — Stable Share Pointers

## Problem

Today every click on **Publish** calls `publishForm()`, which generates a fresh random `renderKey` and re-encrypts the schema with it. This means:

1. **Share URLs break on every publish** — the `#rk=` fragment embeds the raw AES key, and a new key = a new URL.
2. **Auto-save is inconsistent** — `builder.svelte.ts:save()` also generates a temporary `renderKey` on first save, but it lives only in memory and is lost on page reload. The next save generates yet another key.
3. **"Rotate key"** and **"Publish"** are currently the same operation, which is wrong. Publish should be idempotent; rotation should be explicit and destructive.

## Goal

- Editing a form schema never changes the share URL.
- **Publish** = encrypt latest schema with the existing key + upload. The URL shown is always the same stable URL.
- **Rotate key** = generate a new key, re-encrypt, upload. Old share URLs die.
- The render key survives page reloads without storing raw key material client-side.

---

## Design: HKDF Derivation with Server-Stored Salt

Instead of storing the raw `renderKey`, derive it deterministically:

```
renderKey = HKDF(formKey, salt=renderKeySalt, info="wisp-render-key-v1")
```

- `renderKeySalt` — 16 random bytes generated client-side, stored server-side on the `forms` table.
- `formKey` — already stable (derived from `masterKey` + `formId`).
- `renderKey` — derived, never stored. Always re-derivable from `formKey` + `renderKeySalt`.

The share URL fragment `#rk=<base64url(renderKey)>` stays constant as long as the salt doesn't change.

**On rotate:** generate a new `renderKeySalt`, derive a new `renderKey`, re-encrypt the schema, upload both. The new share URL is different. Old URLs are immediately invalid.

The `renderKeySalt` is **owner-only** — it is returned in `GET /api/forms/{id}` (authenticated) but never in the public schema endpoint. Respondents have the derived key in the URL fragment and don't need the salt.

---

## Step-by-Step Implementation Plan

### Step 1 — Migration `0008_render_key_salt`

**File:** `migrations/0008_render_key_salt.up.sql`

```sql
ALTER TABLE forms ADD COLUMN render_key_salt BYTEA;
```

NULL = not yet published (pre-migration forms, or a form created but never published). The column is nullable; new code will populate it on first publish/save.

**File:** `migrations/0008_render_key_salt.down.sql`

```sql
ALTER TABLE forms DROP COLUMN render_key_salt;
```

---

### Step 2 — SQLC Queries

**File:** `internal/db/queries/forms.sql`

Three queries change:

**`CreateForm`** — add `render_key_salt` as `$6`:
```sql
-- name: CreateForm :one
INSERT INTO forms (
    id, account_id, created_at, updated_at, status, schema_version,
    response_count, encrypted_schema, render_encrypted_schema, public_form_key,
    render_key_salt
) VALUES (
    $1, $2, CURRENT_DATE, CURRENT_DATE, 'open', 1, 0, $3, $4, $5, $6
) RETURNING *;
```

**`UpdateFormSchema`** — add `render_key_salt` as `$5`:
```sql
-- name: UpdateFormSchema :one
UPDATE forms
SET encrypted_schema = $3,
    render_encrypted_schema = $4,
    render_key_salt = $5,
    schema_version = schema_version + 1,
    updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2
RETURNING schema_version;
```

`GetFormByOwner` uses `SELECT *` so it automatically picks up the new column — no SQL change needed.

Run `sqlc generate` after updating.

---

### Step 3 — Backend: Service + Handler

**`internal/forms/service.go`**

- `FormRecord` struct: add `RenderKeySalt []byte`
- `CreateForm(...)`: accept `renderKeySalt []byte` parameter, pass to `CreateFormParams`
- `UpdateFormSchema(...)`: accept `renderKeySalt []byte` parameter, pass to `UpdateFormSchemaParams`
- `formRecordFromDB(...)`: map `f.RenderKeySalt` to `FormRecord.RenderKeySalt`

**`internal/forms/handler.go`**

- `handleCreateForm` request body: add `renderKeySalt string` (base64), decode to `[]byte`, pass to service
- `handleUpdateFormSchema` request body: add `renderKeySalt string` (base64), decode to `[]byte`, pass to service
  - If omitted, use `""` / empty (guard: reject if both encrypted schema changed and salt missing)
- `handleGetForm` response: add `renderKeySalt: base64(record.RenderKeySalt)` (omit if nil, so `omitempty` is fine)

---

### Step 4 — Frontend: `crypto.ts`

Add one new exported function:

```typescript
/**
 * Derive the render key from the form key and a server-stored salt.
 * renderKey = HKDF(formKey, salt=renderKeySalt, info="wisp-render-key-v1")
 *
 * The derived key is extractable so it can be exported for the share URL fragment.
 */
export async function deriveRenderKey(
  formKey: CryptoKey,
  renderKeySalt: ArrayBuffer
): Promise<CryptoKey>
```

Implementation: use `toHkdfIkm(formKey)` then `crypto.subtle.deriveKey` with:
- algorithm: `{ name: 'HKDF', hash: 'SHA-256', salt: renderKeySalt, info: encode('wisp-render-key-v1') }`
- derived key: `{ name: 'AES-GCM', length: 256 }`, extractable: `true`, usages: `['encrypt', 'decrypt']`

Add `'wisp-render-key-v1'` to the `INFO` map for consistency.

---

### Step 5 — Frontend: `forms.ts`

**`FormRecord`** — add field:
```typescript
renderKeySalt: string | null;  // base64, null if never published
```

**`createForm`** — generate salt up front, derive renderKey from it:
```typescript
// Generate a stable salt for this form's render key
const saltBytes = crypto.getRandomValues(new Uint8Array(16));
const renderKey = await deriveRenderKey(formKey, saltBytes.buffer);
const renderEncryptedSchema = await encryptSchema(schema, renderKey);
// ...
body: { formId, encryptedSchema: ..., renderEncryptedSchema: ...,
        publicFormKey: ..., renderKeySalt: arrayBufferToBase64(saltBytes.buffer) }
// Return renderKeySalt alongside renderKey so builder store can remember it
return { formId, renderKey, renderKeySalt: saltBytes };
```

**`updateFormSchema`** — replace `renderKey: CryptoKey` param with `renderKeySalt: Uint8Array`:
```typescript
export async function updateFormSchema(
  masterKey: CryptoKey,
  formId: string,
  schema: FormSchema,
  renderKeySalt: Uint8Array           // ← was: renderKey: CryptoKey
): Promise<{ schemaVersion: number }>
```
Derive `renderKey = await deriveRenderKey(formKey, renderKeySalt.buffer)` internally, then encrypt.
Pass `renderKeySalt` in the request body as base64 (unchanged on regular saves).

**`getForm`** — decode and expose `renderKeySalt`:
```typescript
return { schema, record };  // record.renderKeySalt is already string|null from JSON
```

**`publishForm`** — replace with two separate operations:

```typescript
/**
 * First-time publish OR re-publish with the existing key.
 * Pass renderKeySalt=null on first publish to generate a new salt.
 * Pass the existing salt to re-encrypt with the same key (stable URL).
 */
export async function publishForm(
  masterKey: CryptoKey,
  formId: string,
  schema: BuilderSchema,
  existingRenderKeySalt: Uint8Array | null
): Promise<{ shareUrl: string; renderKeySalt: Uint8Array }>
```

Logic:
- If `existingRenderKeySalt` is null: generate `saltBytes = new Uint8Array(16)` random
- Else: use `existingRenderKeySalt` unchanged
- Derive `renderKey = await deriveRenderKey(formKey, saltBytes.buffer)`
- Encrypt schema both ways, PUT to server with `renderKeySalt` in body
- Export `renderKey` → base64url → build share URL
- Return `{ shareUrl, renderKeySalt: saltBytes }`

```typescript
/**
 * Explicit key rotation — generates a new salt and new share URL.
 * Old share URLs are immediately invalidated.
 */
export async function rotateRenderKey(
  masterKey: CryptoKey,
  formId: string,
  schema: BuilderSchema
): Promise<{ shareUrl: string; renderKeySalt: Uint8Array }>
```

This is just `publishForm(masterKey, formId, schema, null)` — passing null forces a new salt.
Can be a thin wrapper or the same function. Keep them separate in the call site for clarity.

---

### Step 6 — Frontend: `builder.svelte.ts`

**Replace** `currentRenderKey: CryptoKey | null` with `currentRenderKeySalt: Uint8Array | null`.

**`load()`**: after fetching the form, decode and store the salt:
```typescript
const { schema: loaded, record } = await getForm(masterKey, formId);
schema = loaded as BuilderSchema;
activeLocale = schema.defaultLocale;
if (record.renderKeySalt) {
  currentRenderKeySalt = base64ToUint8Array(record.renderKeySalt);
}
dirty = false;
```

**`save()`**: generate a salt on first save if we don't have one yet; use it for all subsequent saves:
```typescript
async function save(): Promise<void> {
  if (saving) return;
  saving = true;
  try {
    if (!currentRenderKeySalt) {
      currentRenderKeySalt = crypto.getRandomValues(new Uint8Array(16));
    }
    await updateFormSchema(masterKey, formId, schema, currentRenderKeySalt);
    lastSaved = new Date();
    dirty = false;
  } finally {
    saving = false;
  }
}
```

**Expose** the salt on the store interface so the builder page can read it:
```typescript
// In BuilderStore interface:
readonly renderKeySalt: Uint8Array | null;

// In returned object:
get renderKeySalt() { return currentRenderKeySalt; },
```

---

### Step 7 — Frontend: Builder page (`+page.svelte`)

**`handlePublish()`**: pass existing salt to `publishForm` (stable URL if already published):
```typescript
async function handlePublish() {
  if (!store || !auth.masterKey) return;
  publishing = true;
  publishError = '';
  try {
    await store.flushSave();
    const result = await publishForm(auth.masterKey, formId, store.schema, store.renderKeySalt);
    shareUrl = result.shareUrl;
    // Update store's salt if this was a first publish
    // (store.renderKeySalt was null, now result.renderKeySalt is set)
    store.setRenderKeySalt(result.renderKeySalt);
    publishModalOpen = true;
  } catch (err) {
    publishError = err instanceof Error ? err.message : 'Publish failed';
  } finally {
    publishing = false;
  }
}
```

**`handleRotateKey()`**: explicitly pass null to force a new salt:
```typescript
async function handleRotateKey() {
  if (!store || !auth.masterKey) return;
  publishing = true;
  publishError = '';
  try {
    const result = await rotateRenderKey(auth.masterKey, formId, store.schema);
    shareUrl = result.shareUrl;
    store.setRenderKeySalt(result.renderKeySalt);
  } catch (err) {
    publishError = err instanceof Error ? err.message : 'Key rotation failed';
  } finally {
    publishing = false;
  }
}
```

Add `setRenderKeySalt(salt: Uint8Array): void` to the `BuilderStore` interface and implement it as:
```typescript
function setRenderKeySalt(salt: Uint8Array): void {
  currentRenderKeySalt = salt;
}
```

---

## Summary of Changed Files

| File | Change |
|---|---|
| `migrations/0008_render_key_salt.up.sql` | New — add `render_key_salt BYTEA` column |
| `migrations/0008_render_key_salt.down.sql` | New — drop column |
| `internal/db/queries/forms.sql` | Update `CreateForm`, `UpdateFormSchema` params |
| `internal/db/gen/` | Regenerate via `sqlc generate` |
| `internal/forms/service.go` | Add `RenderKeySalt` to `FormRecord`; add param to `CreateForm`, `UpdateFormSchema` |
| `internal/forms/handler.go` | Accept/return `renderKeySalt` in create + update + get |
| `web/src/lib/crypto.ts` | Add `deriveRenderKey(formKey, salt)` |
| `web/src/lib/forms.ts` | Update `createForm`, `updateFormSchema`, `publishForm`; add `rotateRenderKey` |
| `web/src/lib/stores/builder.svelte.ts` | Replace `currentRenderKey` with `currentRenderKeySalt`; update `load`, `save`; expose `renderKeySalt` + `setRenderKeySalt` |
| `web/src/routes/(app)/forms/[id]/edit/+page.svelte` | Update `handlePublish`, `handleRotateKey` to use new signatures |

## Backward Compatibility

Existing forms in the DB have `render_key_salt = NULL`. When the owner next opens the builder:
- `store.load()` gets `renderKeySalt = null` (field missing or null in API response)
- First auto-save generates a new random salt, stores it server-side, re-encrypts
- First publish derives the key from that salt, presents a stable URL going forward
- Any old share URLs from pre-migration publishes (which used an unrelated random key) are already broken — no additional breakage beyond what already existed

## Non-goals

- No new UI beyond renaming "Rotate key" button label to clarify it invalidates old links.
- No migration of existing `render_encrypted_schema` data — old respondent links were never stable anyway.
- The `GetFormPublic` endpoint does not change — respondents continue to receive `render_encrypted_schema` and `public_form_key` only.
