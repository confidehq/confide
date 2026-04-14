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

The core primitive is an ephemeral ECIES key exchange:

1. **Existing device** generates an ephemeral keypair and displays a QR code containing its ephemeral public key and a server-issued pairing token.
2. **New device** scans the QR, generates its own ephemeral keypair, and registers its ephemeral public key with the server under the pairing token.
3. **Existing device** receives the new device's ephemeral public key, ECIES-wraps the master key with it, and posts the ciphertext to the server.
4. **New device** retrieves the ciphertext, decrypts it with its ephemeral private key to recover the master key, then creates a new passkey, derives the PRF key, and wraps the master key for that passkey — completing a standard credential registration.

The server is a dumb relay for the wrapped key blob. It cannot decrypt it.

This pattern is identical to how workspace keys are wrapped and transferred in the existing codebase (`generateAndWrapWorkspaceKey` / `grantMemberKey` in `workspaces.ts`).

## Flow

### Existing Device (logged in — Settings → Passkeys → "Add New Device")

```
1. Call POST /api/auth/pairing
   ← { token, expiresAt }

2. Generate ephemeral keypair:
   ephem_priv, ephem_pub = generateEphemeralKeypair()

3. Encode QR payload:
   { token, pubKey: base64(ephem_pub) }

4. Display QR code and begin polling:
   GET /api/auth/pairing/{token}

5. Poll returns new device's ephemeral pubkey (new_pub):
   wrapped_master = ECIES_encrypt(new_pub, masterKey)

6. POST /api/auth/pairing/{token}/fulfill
   { wrappedMasterKey: base64(wrapped_master) }

7. Show confirmation: "New device successfully added"
```

### New Device (unauthenticated — navigates to /pair or login page)

```
1. Scan QR code (or enter token manually):
   Extract { token, pubKey: existing_ephem_pub }

2. Generate own ephemeral keypair:
   new_priv, new_pub = generateEphemeralKeypair()

3. POST /api/auth/pairing/{token}/request
   { ephemeralPublicKey: base64(new_pub) }

4. Poll GET /api/auth/pairing/{token} for wrapped master key

5. Receive wrapped_master_key:
   master_key = ECIES_decrypt(new_priv, wrapped_master_key)

6. Create new passkey via WebAuthn registration ceremony:
   credential, prf_key = createPasskeyWithPRF()

7. Wrap master key for new passkey:
   wrapped_master_for_passkey = AES_wrap(prf_key, master_key)

8. POST /api/auth/pairing/{token}/complete
   {
     credentialId,
     credentialPublicKey,
     wrappedMasterKey: base64(wrapped_master_for_passkey),
     prfSalt
   }
   ← session token

9. Redirect to dashboard — full session established
```

## API Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `POST` | `/api/auth/pairing` | Required | Create pairing session; returns `{ token, expiresAt }` |
| `POST` | `/api/auth/pairing/{token}/request` | None | New device submits ephemeral public key |
| `POST` | `/api/auth/pairing/{token}/fulfill` | Required | Existing device submits wrapped master key |
| `GET`  | `/api/auth/pairing/{token}` | None | Poll for pairing state (new device's pubkey or wrapped key) |
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
  "newDevicePublicKey": "<base64 — present when state = requested>",
  "wrappedMasterKey": "<base64 — present when state = fulfilled>"
}
```

## Backend Implementation

### Pairing Session Store

Pairing sessions can be stored in-memory using the same pattern as the existing challenge store in `internal/auth/service.go`. No database table is needed.

```go
type pairingSession struct {
    token              string
    accountID          uuid.UUID
    state              string    // created, requested, fulfilled, completed
    newDevicePubKey    []byte
    wrappedMasterKey   []byte
    expiresAt          time.Time
}
```

A background goroutine (or lazy expiry on read) cleans up expired sessions.

### Complete Endpoint

`POST /api/auth/pairing/{token}/complete` is unauthenticated but gated by the pairing token. It:

1. Validates the token exists, is in `fulfilled` state, and is not expired
2. Looks up the account ID from the session
3. Inserts the new WebAuthn credential (reuses `AddCredential` logic from `add_cred.go`)
4. Stores the wrapped master key for the new credential
5. Issues a session token
6. Marks the pairing session as `completed`

This endpoint is essentially `add_cred.go`'s finish handler, but bootstrapped by a pairing token instead of an authenticated session.

## Frontend Implementation

### New Route: `/pair`

A dedicated unauthenticated route at `/pair` handles the new device side of the flow. It is separate from the main login page to keep concerns clean.

Stages:
1. **Scan** — camera view or manual token entry field
2. **Waiting** — "Approve this on your other device" with a spinner
3. **Create passkey** — WebAuthn registration prompt
4. **Done** — redirect to dashboard

### Settings — Passkeys Tab

The existing passkeys settings page gains an "Add New Device" button that triggers the existing-device side of the flow:

1. Calls `POST /api/auth/pairing`
2. Renders a QR code (using a lightweight AGPL-compatible library)
3. Polls for new device connection
4. On connect, wraps master key and fulfills
5. Shows confirmation

### QR Library

Candidate: `qrcode` (MIT) or `@bitjson/qr-code` (MIT). Verify AGPL compatibility before adding. Alternatively, a pure-SVG QR generator with no dependencies can be written in ~150 lines.

### Polling

Both devices poll `GET /api/auth/pairing/{token}` every 2 seconds. The session has a 5-minute TTL, so at most 150 requests per pairing attempt. Consider switching to SSE if polling proves noisy at scale.

## Security Considerations

| Concern | Mitigation |
|---------|-----------|
| QR code photographed by attacker | Token alone grants nothing — attacker still needs to be the first to POST an ephemeral pubkey before the legitimate new device does; existing device sees the request before fulfilling |
| Pairing token brute-forced | Tokens are 32 random bytes (256-bit); rate-limit `/api/auth/pairing/{token}` endpoints |
| Replay of fulfilled session | Token transitions to `completed` after first successful `/complete` call; subsequent calls are rejected |
| Man-in-the-middle on wrapped key | ECIES wrapping binds the ciphertext to the new device's ephemeral keypair; a MITM who swaps the pubkey would produce a blob only they can decrypt — but the existing device should display the new device's fingerprint for optional manual verification |
| Session fixation on `/complete` | The account ID comes from the server-side pairing session, not from the request body — caller cannot claim a different account |

## Open Questions

1. **Manual token entry fallback** — should `/pair` support entering the token as text for environments where camera access is unavailable (e.g., desktop browser scanning not supported)? This would require displaying the token on the existing device alongside the QR.

2. **Approval confirmation UX** — should the existing device show a fingerprint or device name from the new device before fulfilling, so the user can confirm it's their own device? Adds a step but increases confidence against QR interception.

3. **Notification on new device added** — should an email be sent to the account holder when a new device is paired, as a security alert?

## Relationship to Existing Features

- `internal/auth/add_cred.go` — the complete endpoint reuses this logic; consider extracting the credential-insertion step into a shared function
- `web/src/lib/workspaces.ts` — `generateAndWrapWorkspaceKey` uses the same ECIES pattern; the pairing wrap/unwrap on the client can reuse the same `wrapWithECIES` / `unwrapWithECIES` helpers
- `docs/009_multi_passkey.md` — prior design for multi-passkey support; device pairing is the primary use case that motivated it
