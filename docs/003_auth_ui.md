# Phase 3 — Auth UI + Onboarding

## Context

Phase 2 delivered a complete Go API for WebAuthn registration, login, recovery, and session management. Phase 3 wires a SvelteKit frontend to those endpoints, adds the mandatory recovery code onboarding UX, and implements the re-auth gate (session valid, masterKey gone from memory). One small backend addition is required: a `POST /auth/recover/rekey` pair to allow credential replacement after recovery (the design doc requires this but Phase 2 left no endpoint for it).

**Exit criterion:** A non-technical user can sign up, save their recovery codes, close the tab, and log back in without any guidance.

---

## Route Structure

```
web/src/routes/
├── +page.svelte                         ← redirect: /dashboard or /login
├── (auth)/
│   ├── +layout.svelte                   ← redirect away if already authed
│   ├── signup/+page.svelte              ← 5-step registration wizard
│   ├── login/+page.svelte               ← WebAuthn assertion + no-cred fallback
│   └── recover/+page.svelte             ← recovery code → rekey flow
└── (app)/
    ├── +layout.ts                       ← export const ssr = false
    ├── +layout.svelte                   ← re-auth modal overlay
    ├── dashboard/+page.svelte           ← Phase 4 placeholder
    └── settings/sessions/+page.svelte  ← session list + revoke
```

Route groups keep URL paths clean (`/dashboard` not `/app/dashboard`) and allow per-group layouts without SSR issues around localStorage.

---

## New Files

### Frontend

| File | Purpose |
|------|---------|
| `web/src/lib/types/auth.ts` | TypeScript interfaces for API responses, error types |
| `web/src/lib/stores/auth.svelte.ts` | Runes-based state: masterKey (memory), accountId + credentialId (localStorage) |
| `web/src/lib/auth.ts` | Ceremony orchestration: register, login, recover, rekey |
| `web/vite.config.ts` | Add `server.proxy`: `/api` → `http://localhost:8080` (dev only) |
| All routes listed above | — |

### Backend

| File | Purpose |
|------|---------|
| `internal/auth/rekey.go` | `rekeyTokenStore`, `RekeyBegin`, `RekeyFinish` methods |
| `migrations/0005_rekey.up.sql` | No-op (no schema change needed — UPDATE existing accounts row) |

### Modified Backend Files

| File | Change |
|------|--------|
| `internal/auth/service.go` | Add `rekeyTokenStore` to Service; modify `Recover()` to return `rekeyToken`; extend `DB` interface |
| `internal/auth/handler.go` | Add `rekeyBegin`/`rekeyFinish` handlers (unauthenticated routes); modify `recover_` response |
| `internal/db/queries/auth.sql` | Add `UpdateAccountCredential` and `DeleteRecoveryCodesByAccount` queries |
| `internal/db/queries/auth.sql.go` | Regenerated via `sqlc generate` |

---

## Key Implementation Details

### `web/src/lib/stores/auth.svelte.ts`
Must be `.svelte.ts` (not `.ts`) — Svelte 5 runes are compiler transforms, they don't work in plain `.ts` files.

```typescript
let _masterKey = $state<CryptoKey | null>(null);
let _accountId = $state<string | null>(
  typeof localStorage !== 'undefined' ? localStorage.getItem('confide.accountId') : null
);
let _credentialId = $state<string | null>(
  typeof localStorage !== 'undefined' ? localStorage.getItem('confide.credentialId') : null
);

export const auth = {
  get masterKey() { return _masterKey; },
  get accountId() { return _accountId; },
  get credentialId() { return _credentialId; },
  get hasStoredCredential() { return _credentialId !== null; },

  setSession(masterKey: CryptoKey, accountId: string, credentialId: string) { ... },
  clearMasterKey() { _masterKey = null; },
  clearAll() { /* clear state + localStorage */ }
};
```

### `web/src/lib/auth.ts` — ceremony orchestration

**PRF output → AES-KW key (the critical bridge):**
```typescript
async function prfToKek(prfBytes: ArrayBuffer): Promise<CryptoKey> {
  return crypto.subtle.importKey('raw', prfBytes, { name: 'AES-KW' }, false, ['wrapKey', 'unwrapKey']);
}
```
PRF result from `@simplewebauthn/browser` is a `Base64URLString` at `credential.clientExtensionResults?.prf?.results?.first`. Decode via `base64url → Uint8Array → .buffer` before passing to `prfToKek`.

**Registration ceremony:**
1. POST `/api/auth/register/begin` → `{ accountId, options }`
2. `startRegistration({ optionsJSON: options })` from `@simplewebauthn/browser`
3. Extract PRF output → `kek = await prfToKek(...)`
4. `masterKey = await crypto.subtle.generateKey(...)` (AES-GCM-256, extractable)
5. `wrappedMasterKey = await wrapKey(masterKey, kek)`
6. `codes = Array.from({ length: 12 }, () => generateRecoveryCode())`
7. `recoveryKey = await deriveRecoveryKey(codes)`
8. `recoveryWrappedMasterKey = await wrapKey(masterKey, recoveryKey)`
9. `recoveryVerifier = await hashForVerification(new TextEncoder().encode(codes.join('')))`
10. `codeHashes = await Promise.all(codes.map(c => hashForVerification(encode(normalize(c)))))`
    - normalize = `.toUpperCase().replace(/-/g, '')` (matches server-side `Recover()`)
11. POST `/api/auth/register/finish` with credential + all base64-encoded blobs
12. Returns `{ masterKey, accountId, credentialId: credential.id, codes }`

**Login ceremony:**
1. POST `/api/auth/login/begin` with `{ credentialIdBase64 }` → `{ options, wrappedMasterKey }`
   - Note: login/begin response contains the wrappedMasterKey (included in the assertion options)
2. `startAuthentication({ optionsJSON: options })`
3. Extract PRF output → `kek`, then `masterKey = await unwrapKey(base64ToBuf(wrappedMasterKey), kek)`
4. POST `/api/auth/login/finish` with credential

**Recovery:**
- Server `/auth/recover` only needs one code (to burn) but returns `recoveryWrappedMasterKey`
- Client must locally enter all 12 codes to `deriveRecoveryKey(allCodes)` and `unwrapKey()`
- Security model: server proves possession of one code; full key unwrap proves possession of all 12
- After unwrap → call rekey flow

### Signup Wizard — 5 steps

| Step | State | Notes |
|------|-------|-------|
| `checking` | PRF detect on mount | Block with hard error if unsupported |
| `briefing` | Mandatory scroll-through | `IntersectionObserver` on bottom sentinel; button disabled until scrolled |
| `creating` | WebAuthn ceremony | User gesture → triggers register() |
| `recovery` | Code display + 3-code verification | Verify inputs normalize before compare; no skip path |
| `success` | Redirect | 2s delay or button |

The briefing step is **not a checkbox**. The "Continue" button is disabled until an `IntersectionObserver` fires on a sentinel `<div>` at the bottom of the briefing card. Tracks that the user scrolled to the end.

Recovery code verification: pick 3 random indices (e.g. 2, 6, 10) at wizard start, fixed for the session. User must type each correctly (after normalization) before completing signup.

### Re-auth Gate — `(app)/+layout.svelte`

```svelte
<script lang="ts">
  import { auth } from '$lib/stores/auth.svelte.ts';
  let showReauth = $state(false);

  $effect(() => {
    if (auth.masterKey === null && auth.credentialId !== null) showReauth = true;
    else if (auth.masterKey === null && auth.credentialId === null) goto('/login');
  });
</script>

{#if showReauth}
  <!-- overlay modal — not a redirect, user sees their destination behind it -->
{/if}
{@render children()}
```

Detection: `masterKey === null` (tab refreshed) + `credentialId !== null` (registered on this device) → show re-auth. The modal calls `login(credentialId)` on button click, sets masterKey, dismisses.

### Backend — `POST /auth/recover/rekey`

Two routes (both unauthenticated), added alongside existing `/auth/recover` in `handler.go`:
- `POST /auth/recover/rekey/begin` → WebAuthn registration challenge
- `POST /auth/recover/rekey/finish` → verify + update account credential

**Authorization:** `/auth/recover` now additionally returns a short-lived `rekeyToken` (128-bit random, SHA-256 stored, 10-min TTL). Both rekey endpoints require this token — it's proof the caller burned a recovery code. Without it, anyone knowing an `accountId` could initiate a registration ceremony for an account they don't own.

**`rekeyFinish` DB transaction:**
```sql
UPDATE accounts SET credential_id=$2, public_key=$3, prf_salt=$4,
    wrapped_master_key=$5, recovery_wrapped_master=$6, recovery_verifier=$7
WHERE id=$1;

DELETE FROM recovery_codes WHERE account_id=$1;

INSERT INTO recovery_codes (id, account_id, code_hash, used, created_at) VALUES ...;
```

New queries needed in `internal/db/queries/auth.sql`:
```sql
-- name: UpdateAccountCredential :exec
UPDATE accounts SET credential_id=$2, public_key=$3, prf_salt=$4,
    wrapped_master_key=$5, recovery_wrapped_master=$6, recovery_verifier=$7 WHERE id=$1;

-- name: DeleteRecoveryCodesByAccount :exec
DELETE FROM recovery_codes WHERE account_id=$1;
```

After adding, run `sqlc generate` before implementing the service methods.

---

## Implementation Sequence

| # | Deliverable | First Working State |
|---|-------------|---------------------|
| 1 | `vite.config.ts` proxy + `types/auth.ts` | API calls don't CORS-fail in dev |
| 2 | `stores/auth.svelte.ts` + `auth.ts` helpers | Store is reactive, base64 helpers work |
| 3 | Backend: SQL queries + `sqlc generate` + `rekey.go` + handler/service changes | Full backend auth surface complete |
| 4 | `(auth)/login/+page.svelte` + root redirect | Can log in with an existing account |
| 5 | `(auth)/signup/+page.svelte` (all 5 steps) | Full signup + recovery code save flow |
| 6 | `(app)/+layout.svelte` + `+layout.ts` + `dashboard/+page.svelte` | Re-auth gate works on refresh |
| 7 | `(auth)/recover/+page.svelte` | Full recovery + rekey flow |
| 8 | `(app)/settings/sessions/+page.svelte` | Session list + revoke + logout |

---

## Styling

Vanilla CSS with inline styles (consistent with existing `prf-harness`). No CSS framework. Phase 3 is about correctness of the auth flow; visual polish is Phase 7.

---

## Verification

1. **Signup flow:** `pnpm dev`, go to `/signup`, verify PRF gate blocks non-supporting browsers (simulate by patching `detectPRFSupport` to return false). Complete signup on a supporting browser, verify 12 codes are shown, verify 3-code check works, verify redirect to dashboard.

2. **Re-auth gate:** After signup, hard-refresh the page. Verify re-auth modal appears. Complete WebAuthn assertion, verify modal dismisses and dashboard is visible.

3. **Login on a new session:** Open a private window (no localStorage), go to `/login`, verify "no credential on this device" state with recovery link.

4. **Recovery flow:** Use the recovery endpoint test (from Phase 2) to get a known `recoveryWrappedMasterKey`. On the UI, enter the 12 codes, verify masterKey is successfully unwrapped, verify new credential is registered.

5. **Session management:** Log in, open `/settings/sessions`, verify session list loads, revoke a session, verify it disappears. Test logout button clears state and redirects.

6. **Backend rekey tests:** Add to `internal/auth/service_test.go` — recover + rekey happy path, expired rekeyToken → 401, tampered rekeyToken → 401.

