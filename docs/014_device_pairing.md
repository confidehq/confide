# 014 — Cross-Device Login via QR Code Pairing

## Problem

Passkeys are bound to a device or platform keychain (e.g., iCloud Keychain, Google Password Manager). When a user wants to log in on a new device or a different platform, they cannot simply reuse their existing passkey. Even if WebAuthn authentication could be satisfied, the master key — which is derived from the passkey's PRF output — cannot be recovered without either an existing authenticated session or the recovery code.

The goal of device pairing is to allow a logged-in device to securely transfer encrypted key material to a new device, enabling the new device to register its own passkey and reach a full authenticated session — without the server ever seeing the master key.

## Design Goals

- Server learns nothing about the master key at any point in the flow
- Short-lived, single-use pairing tokens (5-minute TTL)
- No recovery codes required
- Reuses existing ECIES key-wrapping primitives already in use for workspace keys
- New device ends the flow with its own passkey — no persistent dependency on the existing device

## Cryptographic Model

The core primitive is a one-sided ECIES key exchange using the new device's ephemeral keypair:

1. **Existing device** displays a QR code containing only a server-issued pairing token.
2. **New device** scans the QR, generates an ephemeral X25519 keypair, and registers its ephemeral public key with the server under the pairing token.
3. **Existing device** polls, receives the new device's ephemeral public key, displays a fingerprint for user confirmation, then ECIES-wraps the master key with `new_pub` and posts the ciphertext to the server.
4. **New device** retrieves the ciphertext, decrypts it with `new_priv` to recover the master key, then creates a new passkey, derives the PRF key, and wraps the master key for that passkey — completing a standard credential registration.

The server is a dumb relay for the wrapped key blob. It cannot decrypt it.

The ECIES wrap/unwrap reuses `encryptForRecipient` / `decryptFromSender` already in `crypto.ts`, which is the same pattern used by `generateAndWrapWorkspaceKey` in `workspaces.ts`.

### Fingerprint Verification

Before the existing device calls `/fulfill`, it must display a human-readable fingerprint derived from the new device's ephemeral public key and require explicit user confirmation. The new device displays the same fingerprint independently so the user can compare both screens.

**Fingerprint derivation:** `SHA-256(new_pub)[0:6]` — 48 bits encoded as 4 short English words from a fixed 4096-word list (12 bits per word). Example: `river-token-lamp-frost`.

This prevents an attacker who intercepted the QR from successfully completing the pairing: the attacker's own ephemeral pubkey would produce a different fingerprint that the user would not recognise.

## Flow

### Existing Device (logged in — Settings → Passkeys → "Add New Device")

```
1. Call POST /api/auth/pairing
   ← { token, expiresAt, shortCode }

2. Encode QR payload:
   { token }
   Also display shortCode below the QR as a manual-entry fallback.

3. Display QR code and begin polling:
   GET /api/auth/pairing/{token}

4. Poll returns new device's ephemeral pubkey (new_pub):
   fingerprint = sha256(new_pub)[0:6] → 4 words
   Display: "Does your other device show: river-token-lamp-frost?"
   Wait for user to tap "Yes, these match".

5. On confirmation:
   wrapped_master = ECIES_encrypt(new_pub, masterKey)
   POST /api/auth/pairing/{token}/fulfill
   { wrappedMasterKey: base64(wrapped_master) }

6. Show in-app notification and confirmation: "New device successfully added"
```

### New Device (unauthenticated — navigates to /pair)

```
1. Scan QR code to extract { token }
   OR enter the shortCode manually on the /pair page.

2. Generate own ephemeral keypair:
   new_priv, new_pub = X25519 generateKey()

3. Compute fingerprint to display to user:
   fingerprint = sha256(new_pub)[0:6] → 4 words
   Show: "On your other device, confirm these words: river-token-lamp-frost"

4. POST /api/auth/pairing/{token}/request
   { ephemeralPublicKey: base64(new_pub) }
   If response is 409: "This pairing request was already accepted by another device."

5. Poll GET /api/auth/pairing/{token} for wrapped master key

6. Receive wrapped_master_key:
   master_key = ECIES_decrypt(new_priv, wrapped_master_key)

7. Create new passkey via WebAuthn registration ceremony:
   credential, prf_key = createPasskeyWithPRF()

8. Wrap master key for new passkey:
   wrapped_master_for_passkey = AES_wrap(prf_key, master_key)

9. POST /api/auth/pairing/{token}/complete
   {
     credentialId,
     credentialPublicKey,
     wrappedMasterKey: base64(wrapped_master_for_passkey),
     prfSalt
   }
   ← { sessionToken, credentialId, wrappedMasterKey, sessionId }

10. Redirect to dashboard — full session established
```

## API Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `POST` | `/api/auth/pairing` | Required | Create pairing session; returns `{ token, expiresAt, shortCode }` |
| `POST` | `/api/auth/pairing/{token}/request` | None | New device submits ephemeral public key; 409 if already requested |
| `POST` | `/api/auth/pairing/{token}/fulfill` | Required | Existing device submits wrapped master key after fingerprint confirmation |
| `GET`  | `/api/auth/pairing/{token}` | None | Poll for pairing state |
| `POST` | `/api/auth/pairing/{token}/complete` | None | New device submits new passkey credential + wrapped master key; issues session |

### Pairing Session States

```
created → requested → fulfilled → completed
                                → expired (after 5 min)
```

### GET /api/auth/pairing/{token} — Response Shape

```json
{
  "state": "created" | "requested" | "fulfilled" | "completed" | "expired",
  "newDevicePublicKey": "<base64 — present when state = requested or fulfilled>",
  "wrappedMasterKey": "<base64 — present when state = fulfilled>"
}
```

### POST /api/auth/pairing/{token}/complete — Response Shape

Matches the `LoginFinish` response shape:

```json
{
  "sessionToken":    "<base64url>",
  "credentialId":    "<string>",
  "wrappedMasterKey": "<base64>",
  "sessionId":       "<string>"
}
```

## Backend Implementation

### Pairing Session Store

Pairing sessions are stored in-memory using the same pattern as the existing challenge store in `internal/auth/service.go`. No database table is needed.

```go
type pairingSession struct {
    token             string
    accountID         string
    state             string    // created, requested, fulfilled, completed
    newDevicePubKey   []byte
    wrappedMasterKey  []byte
    attemptCount      int       // incremented on each /request attempt; capped at 5
    expiresAt         time.Time
}
```

A background goroutine cleans up expired sessions every minute.

**Short code** (`shortCode`): a 8-character alphanumeric string (uppercase letters + digits, excluding ambiguous characters like 0/O, 1/I/L). Generated at session creation alongside the token. The server maintains a short-code → token index for lookup on `/pair?code=XXXX` without exposing the full token in the URL.

### Rate Limiting

`POST /api/auth/pairing/{token}/request` increments `attemptCount` on every call. If `attemptCount` reaches 5 the token is invalidated and subsequent requests return 429. This caps brute-force attempts against an intercepted QR code.

`POST /api/auth/pairing/{token}/complete` follows the same cap: if `attemptCount` for complete attempts reaches 5 the session is invalidated.

### First-Writer-Wins on `/request`

The `/request` handler checks the session state under a mutex before writing `newDevicePubKey`:

```go
s.mu.Lock()
defer s.mu.Unlock()
if session.state != "created" {
    return 409  // already claimed
}
session.state = "requested"
session.newDevicePubKey = body.EphemeralPublicKey
```

A second caller after the state transitions to `requested` receives 409.

### Complete Endpoint — Shared Helper

`POST /api/auth/pairing/{token}/complete` calls a shared `Service` method:

```go
func (s *Service) issueCredentialAndSession(
    ctx context.Context,
    accountID string,
    reg *protocol.ParsedCredentialCreationData,
    wrappedMasterKey []byte,
    name string,
) (*CredentialSessionResult, error)
```

This method:
1. Derives a new random `prfSalt` for the credential.
2. Calls `db.CreateCredential(...)` in a transaction.
3. Calls `db.CreateSession(...)` in the same transaction.
4. Returns `{ sessionToken, credentialId, wrappedMasterKey, sessionId }`.

`AddCredentialFinish` (existing authenticated flow) also calls this helper instead of duplicating the logic.

### Error on Expired Session

When the server restarts mid-pairing or the token TTL elapses, the `/pair` page receives a 404 or a `state: "expired"` poll response. The UI shows:

> "Pairing expired. Please start over on your other device."

### In-App Notification

When `/complete` succeeds, `issueCredentialAndSession` triggers an in-app notification for the account:

> "A new device was added to your account. If this wasn't you, go to Settings → Passkeys to remove it."

This reuses whatever in-app notification mechanism the project already has.

## Frontend Implementation

### QR Library

Use `qrcode` (MIT license). No other candidates need evaluation.

### New Route: `/pair`

A dedicated unauthenticated route handles the new device side of the flow.

Stages:
1. **Scan / Enter** — camera QR scanner with a "Enter code instead" toggle that shows a text field accepting the 8-character short code.
2. **Fingerprint** — display the 4-word fingerprint with instructions: "On your signed-in device, confirm these words match before tapping Yes."
3. **Waiting** — "Waiting for approval on your other device…" spinner while polling.
4. **Create passkey** — WebAuthn registration prompt triggered automatically when `wrappedMasterKey` is received.
5. **Done** — redirect to dashboard.

On any poll response of `state: "expired"`:
> "Pairing expired. Please start over on your other device."

### Settings — Passkeys Tab

The existing passkeys settings page gains an "Add New Device" button:

1. Calls `POST /api/auth/pairing` → receives `{ token, expiresAt, shortCode }`.
2. Renders QR code from `{ token }` using `qrcode`.
3. Displays `shortCode` below the QR: "Or enter this code on the other device: **AB3K-7M2P**".
4. Polls `GET /api/auth/pairing/{token}` every 2 seconds.
5. When `state = "requested"`:
   - Derives 4-word fingerprint from `newDevicePublicKey`.
   - Shows: "Does your other device show these words?" with the 4 words and **Yes, confirm** / **No, cancel** buttons.
6. On **Yes**: ECIES-wraps master key with `newDevicePublicKey`, calls `/fulfill`.
7. On **No** or timeout: calls `DELETE /api/auth/pairing/{token}` (or just abandons; token expires naturally).
8. Shows confirmation banner: "New device successfully added."

### Polling

Both devices poll every 2 seconds. At most 150 requests per device over the 5-minute window. Switch to SSE if polling proves noisy at scale.

### Fingerprint Helper (`web/src/lib/auth.ts`)

```typescript
export async function pairingFingerprint(pubKeyBytes: ArrayBuffer): Promise<string> {
    const hash = await crypto.subtle.digest('SHA-256', pubKeyBytes);
    const bytes = new Uint8Array(hash).slice(0, 6); // 48 bits → 4 × 12-bit words
    return [0, 1, 2, 3]
        .map(i => WORD_LIST[(bytes[i * 6 / 8 | 0] << 8 | bytes[i * 6 / 8 + 1 | 0]) >> (4 - (i % 2) * 4) & 0xFFF])
        .join('-');
}
```

`WORD_LIST` is a 4096-entry array of short English words bundled with the client.

## Security Considerations

| Concern | Mitigation |
|---------|-----------|
| QR code photographed by attacker | Attacker can call `/request` first, but the existing device shows the fingerprint of whoever submitted the pubkey — a mismatched fingerprint alerts the user before any key material is released |
| Pairing token brute-forced | Tokens are 32 random bytes (256-bit); per-token attempt cap of 5 on `/request` and `/complete` |
| Replay of fulfilled session | Token transitions to `completed` after first successful `/complete` call; subsequent calls are rejected |
| Two callers race on `/request` | First writer wins under mutex; second caller receives 409 |
| Man-in-the-middle on wrapped key | ECIES wrapping binds the ciphertext to the new device's ephemeral keypair; the fingerprint confirmation step ensures the existing device wraps for the legitimate device's pubkey |
| Session fixation on `/complete` | The account ID comes from the server-side pairing session, not from the request body — caller cannot claim a different account |
| Server restart loses pairing session | In-memory store; user sees "Pairing expired. Please start over on your other device." — acceptable for a 5-minute flow |

## Relationship to Existing Features

- `internal/auth/add_cred.go` — `AddCredentialFinish` and the pairing `/complete` handler both call the shared `issueCredentialAndSession` helper extracted into `internal/auth/service.go`
- `web/src/lib/crypto.ts` — `encryptForRecipient` / `decryptFromSender` are the ECIES primitives reused for the master key wrap/unwrap step
- `web/src/lib/workspaces.ts` — `generateAndWrapWorkspaceKey` uses the same pattern for reference
- `docs/009_multi_passkey.md` — device pairing is the primary use case that motivated multi-passkey support; the `credentials` table and `CreateCredential` query are already in place
