# Browser Compatibility Findings — WebAuthn & Web Crypto

## Summary

Confide uses two browser APIs that have uneven cross-browser support:
- **WebAuthn PRF extension** — required for form owner login/registration
- **X25519 key agreement** — required for respondent form submission

The sections below document every issue found, its root cause, and its fix status.

---

## 1. Origin mismatch in `.env`

**Symptom:** `Error validating origin` on login or registration in local dev.

**Root cause:** `.env` had `CONFIDE_RP_ORIGIN=https://localhost:3000` (HTTPS) while the local dev server runs on plain HTTP. The go-webauthn library compares the origin in the browser's `clientDataJSON` against `RPOrigins` — the scheme mismatch causes every ceremony to fail.

**Fix:** Change `.env` to `CONFIDE_RP_ORIGIN=http://localhost:3000`.

**Affected environments:** Local development only. Production correctly uses HTTPS.

---

## 2. WebAuthn PRF — form owner login/registration

**Symptom:** `PRF output absent. Your browser or authenticator does not support WebAuthn PRF.`

**What PRF is used for:** The authenticator's PRF output is the only source of key material for the master key. Without it, the form owner cannot register, log in, or access any encrypted data.

### 2a. PRF salt encoding — Firefox and Safari

**Root cause:** Firefox and Safari require the PRF salt in `eval.first` to be an `ArrayBuffer` or `Uint8Array`, not a base64url string. The original code converted the server's base64url string to `.buffer` (raw `ArrayBuffer`). Safari in particular handles `Uint8Array` more reliably than a raw `ArrayBuffer` as a `BufferSource` argument.

**Fix:** Changed `convertRegistrationPrfSalts` and the equivalent login/reauth paths in `auth.ts` to assign `base64urlToBytes(...)` (a `Uint8Array`) directly, rather than calling `.buffer` on it.

### 2b. Platform authenticator (iCloud Keychain) — macOS + Safari/Firefox

**Root cause:** Safari and Firefox on macOS delegate to the macOS native passkey API (iCloud Keychain). Chrome uses its own passkey implementation. Full PRF support for the macOS platform authenticator requires macOS 14.4+ (Safari 17.4+). On macOS 14.0–14.3 with Safari 17.0–17.3, the PRF extension is accepted by the browser but the platform authenticator does not return PRF output.

**Workaround:** Use Chrome or Chromium-based browsers for form owner operations until Safari's platform authenticator PRF support is more consistent. Upgrading to macOS 14.4+ (Safari 17.4) improves the situation.

**No code fix available** — this is a platform/browser limitation, not a code defect.

### Browser support matrix for PRF

| Browser | Min version | Notes |
|---|---|---|
| Chrome / Edge | 116+ | Most reliable; own passkey stack |
| Firefox | 119+ | Works; depends on authenticator |
| Safari | 17.4+ (macOS 14.4+) | Inconsistent with platform authenticator |
| Zen (Firefox fork) | Depends on base version | Verify via `about:support` |

---

## 3. X25519 — respondent form submission

**Symptom:** `The operation is not supported` when a respondent clicks Submit in Safari.

**What X25519 is used for:** Each response is encrypted with a fresh ephemeral X25519 keypair using an ECIES-like scheme. The respondent never authenticates — this is pure Web Crypto.

### 3a. `deriveKey(X25519 → HKDF)` not supported in Safari

**Root cause:** The original `encryptResponse` and `decryptResponse` used `crypto.subtle.deriveKey` with `{ name: 'X25519' }` targeting `{ name: 'HKDF' }` as the derived key type. Safari does not support deriving an HKDF key directly from an X25519 operation.

**Fix:** Replaced with a two-step approach supported by all browsers:
1. `crypto.subtle.deriveBits({ name: 'X25519', ... }, privateKey, 256)` → raw shared secret bytes
2. `crypto.subtle.importKey('raw', bits, 'HKDF', false, ['deriveKey'])` → HKDF IKM key

### 3b. `salt: new ArrayBuffer(0)` rejected by Safari in HKDF

**Root cause:** The `hkdfDeriveAesKey` and `hkdfDeriveBits` helpers used `salt: new ArrayBuffer(0)` in the HKDF params. Safari's Web Crypto implementation rejects a plain `ArrayBuffer` as a `BufferSource` argument, throwing `NotSupportedError`. Chrome and Firefox accept both `ArrayBuffer` and `Uint8Array`.

**Fix:** Change `salt: new ArrayBuffer(0)` to `salt: new Uint8Array(0)` in both helpers. The HKDF output is identical (same zero-length salt) so no existing encrypted data is affected.

### Browser support matrix for X25519

| Browser | Min version | Notes |
|---|---|---|
| Chrome / Edge | 113+ | Full support |
| Firefox | 130+ | Full support |
| Safari | 17.4+ (macOS 14.4+) | Supported but `deriveKey(→HKDF)` not supported; `ArrayBuffer` as `BufferSource` not supported |

---

## 4. Fix status

| Issue | Status |
|---|---|
| `.env` RP origin mismatch | Not yet applied (`.env` intentionally excluded from changes) |
| PRF salt as `Uint8Array` (auth.ts) | Applied |
| X25519 `deriveBits` + `importKey` (crypto.ts) | Applied |
| HKDF `salt: new Uint8Array(0)` (crypto.ts) | Pending — interrupted before apply |

---

## 5. Recommended minimum browser versions

For **form owners** (login, registration, response viewing):
- Chrome / Edge 116+
- Firefox 119+ with a compatible authenticator (CTAP2.1 hardware key, or platform authenticator on a supported OS)
- Safari 17.4+ on macOS 14.4+

For **respondents** (form filling and submission):
- Any browser with Web Crypto support
- Chrome 113+, Firefox 130+, Safari 17.4+
