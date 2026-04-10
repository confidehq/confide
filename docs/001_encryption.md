# Phase 1 — Crypto Foundation

## Context

Confide is an E2E-encrypted anonymous form platform. Phase 1 builds the entire cryptographic layer (`src/lib/crypto.ts`) for the SvelteKit frontend before any application code is written. This is the security-critical foundation everything else depends on. Exit criterion: the module passes independent review by a second engineer.

---

## File Structure

```
web/                              ← new SvelteKit project root
├── package.json
├── svelte.config.js
├── vite.config.ts
├── tsconfig.json
├── vitest.setup.ts               ← Node Web Crypto smoke test
└── src/
    ├── app.d.ts
    ├── app.html
    └── lib/
        ├── types/
        │   └── crypto.ts         ← interface types (FormSchema, ResponsePayload, etc.)
        ├── crypto.ts             ← PRIMARY DELIVERABLE
        ├── crypto.test.ts        ← Vitest unit tests (~35 tests)
        ├── prf-detection.ts      ← DQ1 resolution: PRF browser detection
        └── routes/
            └── (dev)/
                └── prf-harness/
                    └── +page.svelte  ← manual browser test harness
```

---

## 1. SvelteKit Scaffold

- Directory: `web/` (matches the `web:` service name in the design's docker-compose)
- Package manager: `pnpm`
- Adapter: `@sveltejs/adapter-node`
- `strict: true` in tsconfig — mandatory for a security module
- Test environment: `node` (not `jsdom`) — Node 19+ ships a FIPS-validated Web Crypto API that supports X25519. Avoids jsdom's incomplete crypto implementation.
- Key deps: `@simplewebauthn/browser` (PRF harness only; not imported by `crypto.ts`)

---

## 2. Types (`src/lib/types/crypto.ts`)

Minimal types needed by the crypto interface. `FieldConfig` is intentionally `Record<string, unknown>` — the crypto layer treats form content as opaque bytes and must not be coupled to the domain model.

```typescript
export interface FormSchema {
  version: number;
  defaultLocale: string;
  locales: string[];
  layout: 'scroll' | 'steps' | 'convo';
  convoAllowEdit?: boolean;
  fields: Field[];
  translations: Record<string, Record<string, unknown>>;
}

export interface ResponsePayload {
  submittedAt: string;   // ISO 8601, client-generated
  locale: string;
  answers: Record<string, string | string[] | number | null>;
}

export interface EncryptedResponse {
  encryptedData: ArrayBuffer;
  ephemeralPublicKey: ArrayBuffer;  // raw X25519, 32 bytes
}
```

---

## 3. `src/lib/crypto.ts` — Implementation

### Public interface (exactly matching design §6.3)

```typescript
deriveFormKey(masterKey: CryptoKey, formId: string): Promise<CryptoKey>
deriveFormKeypair(formKey: CryptoKey): Promise<CryptoKeyPair>
deriveRecoveryKey(codes: string[]): Promise<CryptoKey>
wrapKey(key: CryptoKey, kek: CryptoKey): Promise<ArrayBuffer>
unwrapKey(wrapped: ArrayBuffer, kek: CryptoKey): Promise<CryptoKey>
encryptSchema(schema: FormSchema, key: CryptoKey): Promise<ArrayBuffer>
decryptSchema(blob: ArrayBuffer, key: CryptoKey): Promise<FormSchema>
encryptResponse(payload: ResponsePayload, recipientPublicKey: CryptoKey): Promise<EncryptedResponse>
decryptResponse(encryptedData: ArrayBuffer, ephemeralPublicKey: ArrayBuffer, formPrivateKey: CryptoKey): Promise<ResponsePayload>
hashForVerification(data: ArrayBuffer): Promise<ArrayBuffer>
```

### Cryptographic parameter decisions

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Symmetric algorithm | AES-256-GCM | Authenticated; Web Crypto native |
| AES-GCM IV | 12 bytes (random, prepended to output) | NIST SP 800-38D recommended |
| AES-GCM tag | 128 bits | Maximum tag length |
| Encrypted blob layout | `[12B IV][ciphertext+tag]` | Self-contained; IV never stored separately |
| Key wrapping | AES-KW (256-bit) | RFC 3394; deterministic authenticated wrap; no IV management |
| HKDF hash | SHA-256 | Specified by design doc |
| HKDF salt | `new Uint8Array(0)` for all derivations | masterKey is already pseudorandom; info strings provide domain separation |
| Asymmetric | X25519 via `{ name: "X25519" }` | Design spec; Chrome 113+, Firefox 130+ |
| ECDH API name | `{ name: "X25519", public: ... }` | Correct for X25519; `"ECDH"` is only for P-256/P-384/P-521 |

### HKDF info strings (must be exact and consistent)

| Derived key | Info string (UTF-8) |
|-------------|---------------------|
| `formKey` from masterKey | `"wisp-form-key-v1:{formId}"` |
| X25519 seed from formKey | `"wisp-form-keypair-seed-v1"` |
| `recoveryKey` from codes | `"wisp-recovery-key-v1"` |
| Response encryption key from ECDH shared secret | `"wisp-response-enc-key-v1"` |

### Key extractability decisions (security-sensitive)

| Key | Extractable | Reason |
|-----|-------------|--------|
| `masterKey` | `true` | Required for `crypto.subtle.wrapKey` cross-browser; AES-KW provides cryptographic protection |
| `formKey` | `false` | Never exported; only used in-memory |
| formKeypair private key | `true` at import | Required for X25519 seed → keypair derivation; access boundary is the CryptoKey object; private key never serialised |
| formKeypair public key | `true` | Exported to server as raw bytes for upload |
| `recoveryKey` (AES-KW) | `false` | Only used for wrap/unwrap |
| kek (PRF output) | `false` | Only used for wrap/unwrap |

Both extractability exceptions must be prominently commented in `crypto.ts` with rationale.

### Key function details

**`deriveFormKey`**: HKDF with masterKey as IKM → AES-256-GCM key. `usages: ["encrypt","decrypt","wrapKey","unwrapKey"]`

**`deriveFormKeypair`**:
1. `crypto.subtle.deriveBits` (HKDF, 256 bits, info="wisp-form-keypair-seed-v1") from `formKey`
2. `crypto.subtle.importKey("raw", seed, "X25519", true, ["deriveKey","deriveBits"])`
3. Public key extracted via `crypto.subtle.exportKey("raw", privateKey)` and re-imported

**`deriveRecoveryKey`**: IKM = `TextEncoder().encode(codes.join(""))` → HKDF → AES-KW-256 key. `usages: ["wrapKey","unwrapKey"]`

**`wrapKey`/`unwrapKey`**: `crypto.subtle.wrapKey("raw", key, kek, "AES-KW")` / `crypto.subtle.unwrapKey(...)`. AES-KW adds 8 bytes overhead (RFC 3394).

**`encryptResponse`**:
1. Generate ephemeral X25519 keypair (`extractable: true`)
2. `crypto.subtle.deriveKey({ name: "X25519", public: recipientPublicKey }, ephemeralPrivKey, { name: "HKDF" }, false, ["deriveKey"])` → sharedSecret
3. HKDF(sharedSecret) → AES-256-GCM encryptionKey
4. AES-GCM encrypt JSON payload, prepend 12-byte random IV
5. Export ephemeral public key as raw bytes (32 bytes)
6. Return `{ encryptedData, ephemeralPublicKey }`

**`decryptResponse`**:
1. Import `ephemeralPublicKey` bytes as X25519 public key
2. ECDH(formPrivateKey, ephemeralPublicKey) → sharedSecret → HKDF → decryptionKey
3. Strip IV, AES-GCM decrypt, JSON parse

**Recovery code generation** (internal helper, not in public interface):
- Character set: `A-Z0-9` (36 chars)
- Rejection sampling to eliminate modulo bias: reject bytes ≥ 252
- Format: `XXXX-XXXX` (4 chars, hyphen, 4 chars)

---

## 4. Unit Tests (`src/lib/crypto.test.ts`)

TDD approach: tests written before/alongside implementation. All async. No mocking of Web Crypto — always uses real implementation.

### Test groups (~35 total)

1. **`hashForVerification`** — 3 NIST FIPS 180-4 SHA-256 known-answer vectors (empty string, "abc", long string)
2. **`deriveFormKey`** — returns AES-GCM key; deterministic; different formIds produce different keys; different masterKeys produce different keys
3. **`deriveFormKeypair`** — returns X25519 pair; public key extractable; deterministic; different formKeys produce different keypairs; ECDH with derived pair works
4. **`deriveRecoveryKey`** — returns AES-KW key; deterministic; order-sensitive; different codes produce different keys
5. **`wrapKey`/`unwrapKey`** — wrapped length = original + 8 (AES-KW overhead); round-trip functional equivalence; tampered bytes throw
6. **`encryptSchema`/`decryptSchema`** — round-trip equality; output length = input + 28; each call produces different ciphertext (IV randomness); bit-flip in ciphertext throws; bit-flip in IV throws; wrong key throws
7. **`encryptResponse`/`decryptResponse`** — round-trip equality; ephemeralPublicKey is 32 bytes; wrong private key throws; tampered ciphertext throws; each call uses fresh ephemeral keypair
8. **Integration: full key hierarchy** — `masterKey → formKey → encryptSchema/decryptSchema`; `masterKey → formKey → formKeypair → encryptResponse/decryptResponse`; `masterKey → wrap → unwrap → formKey → formKeypair` full chain
9. **HKDF RFC 5869 Test Case 1** — byte-exact known-answer test to pin HKDF parameters

Coverage target: 100% line and function coverage on `crypto.ts`.

---

## 5. PRF Detection (`src/lib/prf-detection.ts`)

Resolves DQ1: block signup on unsupported browsers.

```typescript
export interface PRFSupportResult {
  supported: boolean;
  webAuthnSupported: boolean;
  platformAuthenticatorAvailable: boolean;
  reason: string | null;    // human-readable, shown in UI
}

export async function detectPRFSupport(): Promise<PRFSupportResult>
```

Detection layers:
1. **Static**: `typeof window.PublicKeyCredential !== 'undefined'`
2. **Platform authenticator**: `PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable()`
3. **PRF extension presence**: only determinable during a WebAuthn ceremony — detected during actual signup; if PRF output absent after ceremony, surface error: "Your browser or authenticator does not support WebAuthn PRF."

Supported browsers shown on block screen: Chrome/Edge 116+, Safari 17+, Firefox 119+. Block state is not dismissible.

---

## 6. PRF Browser Test Harness (`src/routes/(dev)/prf-harness/+page.svelte`)

Dev-only route guarded by a server-side `NODE_ENV !== 'development'` redirect. A sequential step-by-step UI (no Tailwind, inline styles fine) that manually exercises:

1. PRF support detection display
2. Registration simulation: credential create + PRF output → `wrapKey(masterKey, prfOutputAsKek)`
3. Assertion simulation: credential get → same PRF output → `unwrapKey(wrappedMasterKey, prfOutputAsKek)` → verify determinism
4. Full key hierarchy: masterKey → formKey → formKeypair → `encryptResponse` → `decryptResponse`
5. Recovery flow: 12 test codes → `deriveRecoveryKey` → `wrapKey` → `unwrapKey` → verify
6. Hash verification: `hashForVerification` output displayed in hex

Each step shows: PASS / FAIL / RUNNING badge + collapsible hex detail panel.

---

## 7. Implementation Sequence

1. Scaffold SvelteKit in `web/` — configure Vitest with Node environment, confirm `crypto.subtle.digest` works in tests
2. Write `src/lib/types/crypto.ts`
3. TDD `hashForVerification` — NIST vectors green first
4. Write internal HKDF helper functions
5. TDD `deriveFormKey` → `deriveFormKeypair` → `deriveRecoveryKey`
6. TDD `wrapKey` / `unwrapKey`
7. TDD `encryptSchema` / `decryptSchema`
8. TDD `encryptResponse` / `decryptResponse`
9. Write integration tests (full key hierarchy)
10. Add HKDF RFC 5869 known-answer test
11. Implement `prf-detection.ts`
12. Build PRF harness page (manual verification)
13. Run full coverage report — hit 100% function/line on `crypto.ts`
14. Prepare review package: `crypto.ts`, tests, types, detection module, written summary of parameter decisions and known limitations

---

## 8. Known Limitations (document for reviewer)

| Item | Status |
|------|--------|
| No AES-GCM additional authenticated data (AAD) | Phase 7 hardening candidate — bind AAD to formId/responseId to prevent cross-context substitution |
| `masterKey` extractable | Required for `wrapKey` cross-browser; AES-KW is the cryptographic guard |
| X25519 private key extractable at derivation | Only the seed (derived from formKey) is extractable; formKey is non-extractable |
| No explicit memory zeroing | Web Crypto CryptoKey objects are opaque; JS cannot zero memory; accepted per design §2.5 |
| PRF layer 2 detection requires user gesture | By design; detected during signup ceremony, not silently |

---

## Verification

```bash
cd web
pnpm install
pnpm test              # all ~35 tests pass
pnpm test -- --coverage  # 100% function + line coverage on crypto.ts
pnpm dev               # start dev server
# navigate to /prf-harness — run all 6 steps manually, all PASS
```
