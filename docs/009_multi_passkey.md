# Multi-Passkey Support

**Status:** Planning
**Exit criterion:** A user can register multiple passkeys (e.g., MacBook Touch ID + iPhone Face ID), name them, and delete them individually. Login works with any registered passkey. Losing one passkey does not require recovery — any other registered passkey can still log in.

---

## Problem

One credential is stored directly on the `accounts` table (`credential_id`, `public_key`, `prf_salt`, `wrapped_master_key`, `backup_eligible`). This means:

- A second passkey cannot be added — there's nowhere to store it.
- Losing the only passkey forces recovery (rekey), which is destructive and replaces the credential.
- `prf_salt` is per-credential, not per-account: each passkey produces a different PRF output, so each needs its own salt and its own wrapped copy of the master key.

---

## Design

### Crypto constraint

PRF output = HMAC(salt, passkey_secret). The salt is per-credential: you must use the same salt you registered with to get the same PRF output. Therefore:

- Each credential needs its own `prf_salt` (32 random bytes, server-generated at registration time).
- Each credential needs its own `wrapped_master_key` = AES-KW(prfOutput, masterKey).
- All credentials for the same account wrap the **same** master key. The key hierarchy is unchanged; only the outermost wrapping layer multiplies.

### Login with multiple passkeys

#### Targeted login (credential ID known)
- Client passes the `credential_id` of the passkey to use.
- Server looks up that credential, embeds its `prf_salt` in `prf.eval.first`.
- Returns that credential's `wrapped_master_key` after the ceremony.
- This is the primary path for returning users who have authenticated before (the credential ID is remembered client-side).

#### Discoverable login (no credential ID)
- WebAuthn spec **forbids** `prf.evalByCredential` when `allowCredentials` is empty — only `prf.eval.first` is valid.
- This means discoverable mode can only supply one PRF salt.
- Strategy: use the account's oldest credential's salt as `prf.eval.first`. If the user authenticates with a different passkey, the PRF output won't match that salt — but `LoginFinish` can detect this, look up the correct credential, and return the right `wrapped_master_key`.
- The client must handle the case where the returned `wrapped_master_key` corresponds to the actual credential used (identified from the assertion's `rawId`), not the one whose salt was in the extension. Concretely: the server always returns the `wrapped_master_key` for the credential that **actually signed** the assertion, regardless of which salt produced which PRF output. If the PRF output doesn't unwrap successfully on the client, it means the user used a different passkey than the one whose salt was embedded — they should fall back to targeted login.

Practical recommendation: discoverable login works reliably only when a single passkey is registered, or when the user happens to pick the primary credential. The settings UI should encourage users to use targeted login (select a passkey) when multiple are registered.

---

## Step-by-Step Implementation Plan

### Step 1 — Migration `0010_credentials`

**`migrations/0010_credentials.up.sql`**

```sql
-- Create the new credentials table (1-to-many with accounts)
CREATE TABLE credentials (
    id                 TEXT        PRIMARY KEY,
    account_id         TEXT        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    credential_id      BYTEA       NOT NULL UNIQUE,
    public_key         BYTEA       NOT NULL,
    prf_salt           BYTEA       NOT NULL,
    wrapped_master_key BYTEA       NOT NULL,
    backup_eligible    BOOLEAN     NOT NULL DEFAULT FALSE,
    name               TEXT        NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Migrate existing single credentials into the new table.
-- Use the account id as the credential row id (still unique since it's 1:1 now).
INSERT INTO credentials
    (id, account_id, credential_id, public_key, prf_salt, wrapped_master_key,
     backup_eligible, name, created_at)
SELECT
    id,
    id,
    credential_id,
    public_key,
    prf_salt,
    wrapped_master_key,
    backup_eligible,
    '',
    NOW()
FROM accounts;

-- Remove the credential columns that now live in the credentials table.
ALTER TABLE accounts
    DROP COLUMN credential_id,
    DROP COLUMN public_key,
    DROP COLUMN prf_salt,
    DROP COLUMN wrapped_master_key,
    DROP COLUMN backup_eligible;
```

**`migrations/0010_credentials.down.sql`**

```sql
-- Restore credential columns on accounts
ALTER TABLE accounts
    ADD COLUMN credential_id      BYTEA,
    ADD COLUMN public_key         BYTEA,
    ADD COLUMN prf_salt           BYTEA,
    ADD COLUMN wrapped_master_key BYTEA,
    ADD COLUMN backup_eligible    BOOLEAN NOT NULL DEFAULT FALSE;

-- Restore single credential per account (take the oldest one)
UPDATE accounts a
SET
    credential_id      = c.credential_id,
    public_key         = c.public_key,
    prf_salt           = c.prf_salt,
    wrapped_master_key = c.wrapped_master_key,
    backup_eligible    = c.backup_eligible
FROM (
    SELECT DISTINCT ON (account_id) *
    FROM credentials
    ORDER BY account_id, created_at ASC
) c
WHERE a.id = c.account_id;

-- Restore NOT NULL after backfill
ALTER TABLE accounts
    ALTER COLUMN credential_id      SET NOT NULL,
    ALTER COLUMN public_key         SET NOT NULL,
    ALTER COLUMN prf_salt           SET NOT NULL,
    ALTER COLUMN wrapped_master_key SET NOT NULL;

DROP TABLE credentials;
```

---

### Step 2 — SQLC Queries

**`internal/db/queries/auth.sql`** — remove queries that read/write the dropped columns, add new credential queries.

**Queries to remove / replace:**
- `GetAccountByCredentialID` — no longer needs to join credential data on the accounts table. Replace with `GetAccountByWebAuthnCredentialID` that queries `credentials`.
- `UpdateAccountCredential` — this updates the single credential on accounts. Replace with `CreateCredential` + `DeleteCredentialsByAccount` (used in recovery rekey).
- `ListCredentials` — currently selects `prf_salt` from accounts; replace with a query on the credentials table.

**New queries:**

```sql
-- name: CreateCredential :one
INSERT INTO credentials (
    id, account_id, credential_id, public_key, prf_salt,
    wrapped_master_key, backup_eligible, name, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
RETURNING *;

-- name: GetCredentialByWebAuthnID :one
SELECT c.*, a.recovery_wrapped_master, a.recovery_verifier
FROM credentials c
JOIN accounts a ON a.id = c.account_id
WHERE c.credential_id = $1;

-- name: ListCredentialsByAccount :many
SELECT id, account_id, credential_id, backup_eligible, name, created_at
FROM credentials
WHERE account_id = $1
ORDER BY created_at ASC;
-- Note: omit prf_salt, public_key, wrapped_master_key from list (not needed for UI)

-- name: GetCredentialForLogin :one
SELECT c.wrapped_master_key, c.prf_salt, c.account_id
FROM credentials c
WHERE c.credential_id = $1;

-- name: GetPrimaryCredentialByAccount :one
-- Used for discoverable login — returns oldest credential's prf_salt
SELECT prf_salt FROM credentials
WHERE account_id = $1
ORDER BY created_at ASC
LIMIT 1;

-- name: GetAllPrfSalts :many
-- Used for discoverable mode — returns all credential_id + prf_salt pairs
SELECT credential_id, prf_salt FROM credentials;

-- name: UpdateCredentialName :exec
UPDATE credentials SET name = $3
WHERE id = $1 AND account_id = $2;

-- name: DeleteCredential :exec
DELETE FROM credentials WHERE id = $1 AND account_id = $2;

-- name: CountCredentialsByAccount :one
SELECT COUNT(*) FROM credentials WHERE account_id = $1;

-- name: DeleteCredentialsByAccount :exec
DELETE FROM credentials WHERE account_id = $1;
```

Run `sqlc generate` after updating.

---

### Step 3 — Backend: Auth Service (`internal/auth/service.go`)

#### Registration

`RegisterFinish` currently stores credential fields on `accounts`. After migration:
- Extract the credential data (credentialID, publicKey, prfSalt, wrappedMasterKey, backupEligible) from the request.
- In a transaction: `CreateAccount` (no credential fields) + `CreateCredential` (first credential, `name = ""`).
- The `CreateAccount` query loses the five dropped columns; update `CreateAccountParams` accordingly.

#### Login — Begin

`loginBeginTargeted(credentialID)` currently calls `GetAccountByCredentialID`. Replace with `GetCredentialByWebAuthnID`, extract `prf_salt` from the credential row. Embed it in `prf.eval.first` as before.

`loginBeginDiscoverable()` currently takes the first account's `prf_salt` from `ListCredentials`. Replace with `GetAllPrfSalts()` and pick the first entry. (The comment about `evalByCredential` being forbidden in discoverable mode still applies — this is a known limitation.)

#### Login — Finish

`LoginFinish` currently returns `account.WrappedMasterKey`. After migration:
1. The assertion response contains the `rawId` of the credential that was actually used.
2. Call `GetCredentialForLogin(rawId)` → get `wrapped_master_key` and `account_id`.
3. Return that `wrapped_master_key` to the client.

This is correct even in discoverable mode where the client used a different passkey than the one whose salt was in `prf.eval.first` — the server returns the right wrapped key regardless.

#### Add-Credential Token

Mirror the `rekeyToken` pattern exactly. Add a new in-memory store `addCredToken` in a new file `internal/auth/add_cred.go`:

```go
type addCredTokenStore struct { /* same structure as rekeyTokenStore */ }
func (s *addCredTokenStore) issue(accountID string) (string, error)
func (s *addCredTokenStore) peek(token string) (string, error)   // validates, doesn't consume
func (s *addCredTokenStore) consume(token string) (string, error) // validates + removes
```

#### Add-Credential Flow

Two new methods on `Service`:

**`AddCredentialBegin(ctx, accountID, addCredToken) (*RegistrationResult, error)`**
1. `peek(addCredToken)` to validate.
2. Generate a new 32-byte `prfSalt`.
3. Call `s.wa.BeginRegistration(userFromAccount(account))` with the PRF extension.
4. Store challenge under key `"add-cred:" + accountID`.
5. Return options + new `prfSalt`.

**`AddCredentialFinish(ctx, accountID, addCredToken, req, wrappedMasterKey, name) (*AddCredentialResult, error)`**
1. `consume(addCredToken)` — one-time use.
2. Retrieve challenge by key `"add-cred:" + accountID`.
3. `s.wa.FinishRegistration(...)`.
4. `CreateCredential(...)` with `name`, `wrapped_master_key`, `backup_eligible` from the verified credential.
5. Return the new credential's `id` and `name`.

#### Rekey Flow (recovery)

`RekeyFinish` currently calls `UpdateAccountCredential` in a transaction. Replace with:
1. `DeleteCredentialsByAccount(accountID)` — wipe all existing credentials.
2. `CreateCredential(...)` — insert the single new one.
3. `DeleteRecoveryCodesByAccount` + `CreateRecoveryCodes` — unchanged.

All four operations remain in one transaction.

#### Credential Management Methods

```go
func (s *Service) ListCredentials(ctx, accountID) ([]CredentialSummary, error)
func (s *Service) RenameCredential(ctx, accountID, credID, name string) error
func (s *Service) DeleteCredential(ctx, accountID, credID string) error  // errors if last credential
```

---

### Step 4 — Backend: Auth Handler (`internal/auth/handler.go`)

#### Reauth — issue add-credential token

Modify `handleReauthFinish`. If the request body contains `"purpose": "add-credential"`, after a successful assertion:
- Issue an `addCredToken` (call `s.addCredTokens.issue(accountID)`).
- Include `"addCredentialToken": "<token>"` in the response JSON.
- If purpose is absent or anything else, return the existing empty 204 response.

#### New routes

Register under the authenticated middleware group:

```
POST /auth/credentials/add/begin     → handleAddCredentialBegin
POST /auth/credentials/add/finish    → handleAddCredentialFinish
GET  /auth/credentials               → handleListCredentials
PATCH /auth/credentials/{id}         → handleRenameCredential
DELETE /auth/credentials/{id}        → handleDeleteCredential
```

**`handleAddCredentialBegin`**
- Read `addCredentialToken` from body.
- Call `service.AddCredentialBegin(ctx, session.AccountID, token)`.
- Return `{ options, prfSalt: base64(salt) }`.

**`handleAddCredentialFinish`**
- Read `addCredentialToken`, registration response, `wrappedMasterKey` (base64), `name` from body.
- Call `service.AddCredentialFinish(...)`.
- Return `{ id, name, createdAt }`.

**`handleListCredentials`**
- Call `service.ListCredentials(ctx, session.AccountID)`.
- Return array of `{ id, name, createdAt, backupEligible, isCurrentSession }` where `isCurrentSession = credential_id == session.CredentialID`.

**`handleRenameCredential`**
- Read `name` from body.
- Call `service.RenameCredential(ctx, session.AccountID, chi.URLParam("id"), name)`.
- Return 204.

**`handleDeleteCredential`**
- Call `service.DeleteCredential(ctx, session.AccountID, chi.URLParam("id"))`.
- Return 204, or 409 if last credential.

---

### Step 5 — Frontend: `auth.ts`

#### Reauth with token

Add an optional `purpose` parameter to the existing `reauthenticate()` function (or add a new `reauthenticateForAddCredential()` function):

```typescript
export async function reauthenticateForAddCredential(
  credentialIdBase64: string
): Promise<string> // returns addCredentialToken
```

Calls `POST /api/auth/reauth/begin` (existing), completes assertion, then calls `POST /api/auth/reauth/finish` with `{ ..., purpose: "add-credential" }`. Returns the `addCredentialToken` from the response.

#### Add credential

```typescript
export async function addCredential(
  masterKey: CryptoKey,
  addCredentialToken: string,
  name: string
): Promise<{ id: string; name: string }>
```

Flow:
1. `POST /api/auth/credentials/add/begin` with `{ addCredentialToken }` → get `options`, `prfSalt`.
2. Decode `prfSalt` from base64 → `ArrayBuffer`.
3. `startRegistration(options)` via `@simplewebauthn/browser`.
4. Extract PRF output via `extractRegistrationKek(credential)` → `kek: CryptoKey`.
5. `wrapKey(masterKey, kek)` → `wrappedMasterKey: ArrayBuffer`.
6. `POST /api/auth/credentials/add/finish` with `{ addCredentialToken, registration: credential, wrappedMasterKey: arrayBufferToBase64(wrappedMasterKey), name }`.
7. Return `{ id, name }`.

#### List / rename / delete

```typescript
export async function listCredentials(): Promise<CredentialSummary[]>
export async function renameCredential(id: string, name: string): Promise<void>
export async function deleteCredential(id: string): Promise<void>
```

Simple fetch wrappers for the new management endpoints.

---

### Step 6 — Frontend: Settings UI

Add a "Passkeys" section to the account settings page. This is a new route or card within the existing settings:

**`/settings` or `/account`** — add a Passkeys card:

- List all passkeys: name (or "Unnamed passkey"), created date, backup-eligible badge, "This session" badge for the one used to log in.
- Inline rename: click name → editable input → save on blur/Enter.
- Delete button: disabled (tooltip: "You must have at least one passkey") when only one exists. Confirmation dialog before deleting.
- "Add passkey" button:
  1. Prompt for a name (optional, e.g., "MacBook Touch ID").
  2. Call `reauthenticateForAddCredential(currentCredentialId)` — user re-verifies with existing passkey.
  3. Call `addCredential(masterKey, token, name)`.
  4. On success, refresh the list.

---

### Step 7 — Frontend: Login UX (targeted mode)

The login page currently shows a single "Sign in with passkey" button. With multiple passkeys, users should be able to choose which one to use (though the browser's passkey picker usually handles this at the OS level for platform authenticators).

No change is strictly required for login — the existing flow works:
- Discoverable mode: user picks a passkey in the OS dialog, server returns the matching `wrapped_master_key`.
- If the user explicitly wants targeted mode, the client can pass `credentialId`.

The one case that breaks in discoverable mode: the PRF extension only sends one salt (`prf.eval.first`), so the PRF output may not match any credential. The `LoginFinish` server always returns the right `wrapped_master_key` for the credential that signed (identified via `rawId`), but the client's PRF output (derived with the wrong salt) won't successfully unwrap it.

**Mitigation:** On the client in `login()`:
1. After `loginFinish`, attempt to unwrap `wrappedMasterKey` using the PRF output from the assertion.
2. If unwrap fails (wrong salt was used), show an error message: "Sign in with a specific passkey" and re-run login in targeted mode using the `credential.rawId` from the failed attempt as the hint.
3. On the second attempt, the server sends the correct salt in `prf.eval.first`, the PRF output matches, unwrap succeeds.

This fallback makes discoverable multi-credential login work transparently in the common case.

---

## Summary of Changed Files

| File | Change |
|---|---|
| `migrations/0010_credentials.up.sql` | New — create `credentials` table, migrate data, drop columns from `accounts` |
| `migrations/0010_credentials.down.sql` | New — reverse migration |
| `internal/db/queries/auth.sql` | Remove old credential queries; add `CreateCredential`, `GetCredentialByWebAuthnID`, `GetCredentialForLogin`, `ListCredentialsByAccount`, `GetPrimaryCredentialByAccount`, `UpdateCredentialName`, `DeleteCredential`, `CountCredentialsByAccount`, `DeleteCredentialsByAccount` |
| `internal/db/gen/` | Regenerate via `sqlc generate` |
| `internal/auth/add_cred.go` | New — `addCredTokenStore` (mirrors `rekeyTokenStore`) |
| `internal/auth/service.go` | Update `RegisterFinish`, `LoginBeginTargeted`, `LoginBeginDiscoverable`, `LoginFinish`; add `AddCredentialBegin`, `AddCredentialFinish`, `ListCredentials`, `RenameCredential`, `DeleteCredential` |
| `internal/auth/rekey.go` | Replace `UpdateAccountCredential` with `DeleteCredentialsByAccount` + `CreateCredential` in transaction |
| `internal/auth/handler.go` | Modify `handleReauthFinish` for `purpose: add-credential`; add routes for credential management |
| `web/src/lib/auth.ts` | Add `reauthenticateForAddCredential`, `addCredential`, `listCredentials`, `renameCredential`, `deleteCredential`; update `loginFinish` fallback for wrong-salt case |
| `web/src/routes/(app)/settings/+page.svelte` | New Passkeys card with list/add/rename/delete |

---

## Non-Goals

- No UI to mark a "primary" passkey (oldest is always primary for discoverable login).
- No cross-device sync management — that's handled by the OS passkey manager.
- No passkey import/export.
- The rekey (recovery) flow remains destructive: it clears all existing credentials and installs one new one. This is intentional — if you're in recovery, you likely can't access your old passkeys.

---

## Backward Compatibility

Existing accounts each have exactly one credential. The migration moves it to the `credentials` table with `name = ''`. On first login after migration, everything works identically. The passkeys settings UI will show one unnamed passkey — the user can rename it.
