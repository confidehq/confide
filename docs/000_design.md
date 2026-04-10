# Confide — Design Document

> Private, anonymous form collection — as easy as Google Forms, as secure as Signal.

**Status:** Draft v0.4 — replaced BLAKE3/WASM with SHA-256 (Web Crypto native) for v1  
**Sections:** 8 of 8 complete  

---

## Table of Contents

1. [Problem Statement & Goals](#1-problem-statement--goals)
2. [Authentication & Identity](#2-authentication--identity)
3. [Form Builder & Data Model](#3-form-builder--data-model)
4. [Submission Flow & Anonymous Transport](#4-submission-flow--anonymous-transport)
5. [API Design & Backend Architecture](#5-api-design--backend-architecture)
6. [Frontend Architecture](#6-frontend-architecture)
7. [Privacy Policy & Legal Architecture](#7-privacy-policy--legal-architecture)
8. [Phased Delivery & Roadmap](#8-phased-delivery--roadmap)

---

## 1. Problem Statement & Goals

### 1.1 Problem statement

Existing form tools — Google Forms, Typeform, Jotform — are built on a surveillance-compatible model. They log IP addresses, attach cookies, require respondent authentication in many cases, and store plaintext responses on infrastructure that is opaque to users and legally accessible via subpoena. The operator can read everything. Third-party scripts are common.

This is fine for collecting pizza orders. It is not acceptable for collecting reports of workplace abuse, testimonials from LGBTQ+ community members in hostile regions, feedback from patients, or data from whistleblowers. The people most in need of simple form tooling are the people most harmed by the current privacy model.

### 1.2 Goals

| ID | Goal |
|----|------|
| G1 | **Zero respondent tracking.** No IP logging, no fingerprinting, no cookies, no session identifiers attached to submissions. A submitted response must be cryptographically unlinkable to the device or network that sent it. |
| G2 | **End-to-end encrypted responses.** Response data is encrypted client-side before transmission using keys derived from and held only by the form creator. The platform operator stores and serves ciphertext only — we are technically incapable of reading responses. |
| G3 | **Legally resistant architecture.** A subpoena of our servers must yield only encrypted blobs with no identifying metadata. We cannot produce what we do not have. Data minimization is a design constraint, not a policy pledge. |
| G4 | **Usable by non-technical users.** Privacy must not require expertise. Form creation, sharing, and response collection must be as intuitive as Google Forms. Cryptographic complexity is entirely invisible to both creators and respondents. |
| G5 | **Self-hostable and auditable.** Organizations with elevated threat models (NGOs, journalists, clinics) can run the full stack on their own infrastructure. The codebase is open source. Security claims must be verifiable by independent audit, not by trusting us. |
| G6 | **No ads, ever.** Monetization via paid plans only. No advertising network integrations, no analytics pixels, no third-party scripts of any kind on form pages. Revenue model must never create incentives to retain or monetize user data. |

### 1.3 Non-goals (v1)

| ID | Non-goal |
|----|----------|
| NG1 | Tor / no-JS form submission. Desirable long-term but increases respondent UX complexity significantly. Deferred to v2. |
| NG2 | Real-time collaboration on form editing (multi-creator, concurrent). Single-owner forms in v1. |
| NG3 | Payment collection, file uploads with server-side processing, or any feature that requires the server to read response content. |
| NG4 | Email notifications on new responses. Email delivery requires metadata (timing, volume) that weakens the privacy model. Considered for v2 with careful design. |

### 1.4 Threat model

We define "respondent" as the person filling out a form, and "creator" as the account holder who built and distributed it. Protection levels differ by actor.

| Adversary | What they can access | Our defence |
|-----------|----------------------|-------------|
| Platform operator (us) | Ciphertext blobs, anonymous submission timestamps | E2E encryption. Keys never leave the creator's client. |
| Government / legal subpoena | Same as operator — we can only hand over what we store | No IP logs. No plaintext. |
| Third-party trackers | Nothing — no third-party scripts on any form page | Strict CSP. Zero external resources on `/f/*` routes. |
| Malicious form creator | Response content (they hold decryption keys by design) | Out of scope. Respondent trusts the creator by choosing to respond. |

### 1.5 Success criteria

| ID | Criterion |
|----|-----------|
| S1 | A network-level observer watching all traffic to our servers cannot link any submission to any respondent's IP address or session. |
| S2 | A full database dump of our servers yields zero plaintext response content. |
| S3 | A non-technical user can create and share a form in under 3 minutes without reading any documentation. |
| S4 | An independent security auditor can verify all privacy claims from the open-source codebase alone, without trusting any of our infrastructure. |

---

## 2. Authentication & Identity

### 2.1 Design principles

Authentication must satisfy three constraints simultaneously: (1) no email, phone number, or personally identifying information is required or stored; (2) the encryption key that unlocks response data is never transmitted to or stored by the server; (3) account recovery is possible without any backdoor.

These constraints rule out password-based auth (requires a server-side verifier), email magic links (requires PII), and OAuth (requires a third-party identity provider who logs the authentication). Passkeys with the WebAuthn PRF extension satisfy all three.

### 2.2 Account identity

Accounts have no human-readable identifier. On signup, the server generates an `accountId` — a random 128-bit value encoded as a URL-safe base64 string. This is the only identifier the server knows. No username, no email, no phone number is accepted or stored at any point.

```
account {
  id              string     // random 128-bit opaque ID
  createdAt       timestamp  // coarse (date only, not time) to limit metadata
  credentialId    bytes      // WebAuthn credential ID
  publicKey       bytes      // WebAuthn public key (COSE-encoded)
  wrappedMasterKey bytes     // master key encrypted by PRF output (see 2.3)
  recoveryVerifier bytes     // SHA-256 hash of recovery key (see 2.4)
  sessionTokens   []session
}
```

### 2.3 Passkey authentication & key derivation

We use the **WebAuthn PRF extension** (pseudo-random function) — a deterministic HMAC output derived from the passkey credential and a server-supplied salt. This PRF output never leaves the authenticator's security boundary and is available only during an active WebAuthn assertion.

The PRF output is used as a key-encryption key (KEK) to unwrap a stored `masterKey`. The `masterKey` is the root of all form response encryption. It lives only in memory during an active session.

**Signup flow**

1. Client requests signup. Server returns a new `accountId`, a random PRF salt, and a WebAuthn registration challenge.
2. User creates a passkey. The authenticator returns a credential + a PRF output seeded with our salt.
3. Client generates a random 256-bit `masterKey`. Wraps it with AES-256-GCM using the PRF output as KEK.
4. Client sends to server: credential public key, `wrappedMasterKey`, and `recoveryVerifier`. Server never sees the PRF output or plaintext `masterKey`.
5. Server issues a signed session token. Client holds `masterKey` in memory for the session duration.

**Login flow**

1. User taps "Sign in with passkey." Server returns a WebAuthn assertion challenge + the stored PRF salt for that credential.
2. Authenticator signs the challenge and produces the same deterministic PRF output as signup (same credential + same salt).
3. Server verifies assertion signature. Returns `wrappedMasterKey` to client.
4. Client unwraps `masterKey` using PRF output. Holds it in memory. Session begins.

### 2.4 Recovery codes

On signup, after the passkey is created, the client generates 12 single-use recovery codes. These are the _only_ way to regain access if the passkey is lost. The model is deliberately identical to Signal's safety numbers — familiar to the target audience.

**Generation:** Client generates 12 random codes (8 characters each, alphanumeric, uppercase, hyphen-separated for readability e.g. `XKCD-7F2A`). A `recoveryKey` is derived from the concatenated codes via HKDF-SHA256. The `masterKey` is also wrapped with this `recoveryKey` and stored server-side as `recoveryWrappedMasterKey`.

**Server-side storage:** Server stores only: `recoveryVerifier` (SHA-256 of the recoveryKey), `recoveryWrappedMasterKey`, and a hashed index of each code (to enforce single-use). Plaintext codes are never transmitted to the server.

**Recovery flow:** User enters any unused recovery code. Client derives `recoveryKey`, server verifies against `recoveryVerifier`, returns `recoveryWrappedMasterKey`. Client unwraps `masterKey`, registers a new passkey, re-wraps and uploads new `wrappedMasterKey`. Used code is burned server-side.

> ⚠️ **Total loss:** If both the passkey and all 12 recovery codes are lost, the account and all response data are permanently inaccessible. No support-assisted recovery path exists. This must be communicated clearly and repeatedly during onboarding. The UI must not allow users to skip the recovery code step.

### 2.5 Session management

| Property | Value |
|----------|-------|
| Session token | Random 256-bit token, stored as HttpOnly + Secure + SameSite=Strict cookie |
| Inactivity expiry | 30 days rolling. Reset on each authenticated request. |
| masterKey in memory | Held in JS memory only. Cleared on tab close, explicit logout, or session expiry. Never written to localStorage or IndexedDB. |
| Concurrent sessions | Allowed. User can view and revoke individual sessions from settings. |
| Server-side session record | SHA-256 hash of token only. Coarse creation timestamp (date, not time). No IP, no user-agent stored. |

### 2.6 PRF extension compatibility & fallback

WebAuthn PRF is supported in Chrome 116+, Safari 17+, and Firefox 119+. It requires a CTAP2.1 authenticator (all modern platform authenticators qualify). Hardware keys require FIDO2 firmware.

> ⚠️ **Open design question — PRF unavailable (DQ1):** If the user's device does not support PRF, we cannot derive the KEK from the passkey. Recommendation: block signup on unsupported browsers with a supported browser list shown on the signup page. Decide before implementing the auth layer.

### 2.7 Key hierarchy summary

```
Passkey (device-bound, never extractable)
  └─ PRF output (derived per-assertion, ephemeral)
       └─ wrappedMasterKey (stored server-side, AES-256-GCM)
            └─ masterKey (in memory during session only)
                 └─ formKey[n] (per-form, derived via HKDF)
                      └─ response ciphertext (stored server-side)

Recovery path:
  12 recovery codes
    └─ recoveryKey (HKDF-SHA256 of codes)
         └─ recoveryWrappedMasterKey (stored server-side, AES-256-GCM)
              └─ masterKey (same root, unlocked via recovery)
```

Per-form keys are derived from the masterKey using HKDF with the formId as context. This means compromising one form's key does not compromise others, and key rotation on a single form is possible without re-encrypting everything.

---

## 3. Form Builder & Data Model

### 3.1 Encryption boundary

Both the form schema and all response data are encrypted client-side before transmission. The server stores and serves opaque blobs. This means the form title, field labels, helper text, and option values are all inaccessible to the operator — a form titled "Abuse report — internal HR" reveals nothing in a server breach or subpoena.

The only unencrypted metadata stored server-side per form:

```
form_record {
  id              string     // random 128-bit opaque ID (used in share URL)
  accountId       string     // owner reference
  createdAt       date       // date only, no time
  updatedAt       date       // date only, no time
  status          enum       // open | closed
  responseCount   int        // plaintext counter (no content)
  encryptedSchema bytes      // AES-256-GCM, key = formKey (see 2.7)
  schemaVersion   int        // for future migration support
}
```

### 3.2 Form schema structure

The decrypted schema is a JSON document versioned by `schemaVersion`. It contains all form configuration, field definitions, translations, and layout settings. It is serialised, encrypted, and stored as a single blob — there are no relational schema tables for form content.

```
FormSchema {
  version              int
  defaultLocale        string          // e.g. "en"
  locales              string[]        // e.g. ["en", "fr", "ar"]
  layout               "scroll" | "steps" | "convo"
  convoAllowEdit       bool?           // convo mode only — allow editing most recent answer (default false)
  fields               Field[]
  translations         Record<locale, TranslationMap>
}

Field {
  id              string          // stable UUID, never changes
  type            FieldType
  required        bool
  order           int
  config          FieldConfig     // type-specific (see 3.3)
}

TranslationMap {
  formTitle              string
  formDescription        string
  convoCompletionMessage string?   // convo mode only — final bot bubble after last answer
  fields: {
    [fieldId]: {
      label       string           // in convo mode: the bot's message text
      helpText    string?          // in convo mode: rendered as a second follow-up bubble
      placeholder string?
      options     string[]?        // for choice fields; in convo mode: rendered as quick-reply chips
    }
  }
}
```

### 3.3 Field types — v1

| Type | Config fields | Response value type |
|------|---------------|---------------------|
| `short_text` | maxLength, placeholder | `string` |
| `long_text` | maxLength, minRows | `string` |
| `multiple_choice` | options[], allowOther | `string` (option id) |
| `checkboxes` | options[], minSelect, maxSelect | `string[]` (option ids) |
| `dropdown` | options[], searchable | `string` (option id) |
| `date_time` | mode: date \| time \| datetime, min, max | `string` (ISO 8601) |
| `rating` | scale (1–5 or 1–10), shape: star \| number | `int` |
| `section_break` | — (display only, no config) | null (not included in response) |

> ℹ️ **`section_break` in convo mode:** In `convo` layout, `section_break` fields are not rendered as visual dividers. Instead they act as a natural pause point — the next field's label appears as a new message bubble after a brief delay, simulating a typing indicator. The `label` translation slot on a `section_break` is repurposed as optional bot copy shown in the pause bubble (e.g. "Thanks, one more thing…"). If left empty, no pause bubble is shown.

### 3.4 Multi-language model

All user-facing strings live in the `translations` map keyed by locale. The form schema contains no inline strings — all labels, help text, option values, and titles are stored exclusively in the translation map. This ensures adding a new language never requires restructuring the field definitions.

**Creator workflow:** Creator builds the form in their default locale. The builder UI exposes a language switcher — selecting a non-default locale shows the same field structure with empty translation slots. Creator fills them in manually. Fields with missing translations fall back to the default locale at render time, never showing a blank label.

**Respondent language detection:** When a respondent opens a form, the client reads `navigator.language` and matches against available locales. If a match exists, that locale is used silently. If not, the form renders in the default locale. A locale switcher is shown in the form footer only if multiple locales are configured. No locale preference is sent to the server.

**RTL support:** Locales flagged as RTL (Arabic, Hebrew, Persian, Urdu) automatically set `dir="rtl"` on the form container. All layout components must be tested against RTL. This is a v1 requirement, not a stretch goal — these communities are a primary target audience.

### 3.5 Layout modes

**Scroll mode:** All fields rendered on a single page. Respondent scrolls top to bottom. Best for short forms or when respondents need to review all questions before answering.

**Steps mode:** Fields are grouped into pages separated by `section_break` fields. One page shown at a time with prev/next navigation. Best for longer forms or sensitive topics where showing all questions at once is disorienting.

**Convo mode:** Fields and responses are presented as a DM/SMS-style conversation thread. Questions appear as incoming message bubbles (left-aligned, muted background) and respondent answers appear as outgoing message bubbles (right-aligned, accent background) after submission. One field is active at a time — the next question appears only after the current one is answered, with a brief typing-indicator animation before each new bubble renders. This layout is best for sensitive or personal topics where a clinical form layout feels cold or bureaucratic.

Key behavioural rules for convo mode:

- **No back navigation.** Once a response bubble is sent it is committed — consistent with SMS conventions and prevents respondents from second-guessing answers in ways that increase anxiety. The creator's `schema.convoAllowEdit` boolean (default `false`) can enable an edit affordance on the most recent bubble only.
- **Field labels become message copy.** The `label` translation slot is the bot's message text, not a form label. Creators should write labels as natural conversational prompts ("What happened, in your own words?") rather than field names ("Description").
- **`helpText` becomes a follow-up bubble.** If a field has `helpText` set, it renders as a second message bubble from the bot below the primary prompt, creating a two-part question pattern common in supportive conversation contexts.
- **Choice fields render as quick-reply chips** below the active bubble, not as a dropdown or radio group. Chips disappear after selection and the chosen option is reflected as an outgoing bubble.
- **Rating fields render as an inline tap-target row** of stars or numbers within the bubble, not as a separate component.
- **`section_break` fields render as a pause** — a typing indicator followed by an optional transitional message bubble (using the `label` slot), then the next field. If `label` is empty, the pause is silent with no bubble rendered.
- **Scroll behaviour:** As the conversation grows, the thread auto-scrolls to the latest bubble. Older bubbles remain visible above — the respondent can scroll up to review the conversation but cannot edit prior answers (unless `convoAllowEdit` is true).
- **Submission:** There is no explicit submit button. The form submits automatically when the last field is answered. A final bot bubble confirms receipt using the form's `convoCompletionMessage` translation slot (e.g. "Thank you. Your response has been recorded.").
- **RTL support:** Bubble alignment flips in RTL locales — outgoing bubbles align left, incoming bubbles align right, consistent with RTL messaging convention.

### 3.6 Response data model

Each submission is encrypted client-side using the `formKey` (derived from masterKey + formId via HKDF). The plaintext response payload is a flat JSON map of fieldId → value before encryption.

```
// Plaintext response (never leaves client unencrypted)
ResponsePayload {
  submittedAt     string    // ISO 8601, client-generated
  locale          string    // locale used when filling out
  answers: {
    [fieldId]:    string | string[] | int | null
  }
}

// Server-side response record
response_record {
  id              string    // random opaque ID
  formId          string
  receivedAt      date      // date only, no time
  encryptedData   bytes     // AES-256-GCM of ResponsePayload
  schemaVersion   int       // schema version at time of submission
}
```

The `receivedAt` field is the only server-generated timestamp and is coarsened to date-only to prevent timing correlation attacks. The client-generated `submittedAt` inside the encrypted payload is full precision and visible only to the creator.

### 3.7 Form builder — frontend architecture

**Drag and drop:** Built with `svelte-dnd-action` — keyboard accessible, touch-friendly, no jQuery dependency. Field order is stored as the `order` integer on each Field. Reordering is an optimistic local update; schema is re-encrypted and synced on each save.

**Save model:** Auto-save on a 2-second debounce after any change. Save encrypts the full schema client-side and PUTs the blob to the server. No partial field updates — the schema is always replaced atomically. A "last saved" indicator is shown in the builder toolbar.

**Preview mode:** Builder has a toggle between Edit and Preview. Preview renders the form exactly as a respondent would see it, using the currently selected preview locale. Preview responses are not submitted.

> ⚠️ **Open design question — schema versioning on live forms (DQ4):** If a creator edits a form after responses have been collected, field IDs in existing responses may reference fields that no longer exist, or new fields will have no answer in old responses. We track `schemaVersion` on both the form and each response record to handle this. The response viewer must gracefully render partial responses against older schema versions. A warning should be shown in the builder when editing a form with existing responses.

### 3.8 Share URL design

Form URLs follow the pattern:

```
https://confide.app/f/<formId>
```

The `formId` is the only information encoded in the URL. There is no decryption key in the URL — the respondent's client fetches the encrypted schema, but cannot decrypt it. Decryption happens only on the creator's authenticated client. See Section 4 for the submission flow.

---

## 4. Submission Flow & Anonymous Transport

### 4.1 The rendering problem

The form schema is encrypted with the creator's `formKey`. The server cannot decrypt it. Yet a respondent's browser — with no account and no credentials — must render a fully functional form. This is the central rendering paradox of the architecture.

The solution: the creator publishes a separate `renderKey` — a symmetric key used exclusively to encrypt the schema for public rendering. This key is embedded in the share URL fragment, which is never sent to the server by the browser. The server stores a `renderEncryptedSchema` alongside the master-encrypted schema. The respondent's client decrypts the schema locally using the fragment key, renders the form, and the server never sees either key or plaintext.

### 4.2 Render key design

**Key generation:** When a creator publishes a form, the creator's client generates a random 256-bit `renderKey`. It re-encrypts the plaintext schema with this key using AES-256-GCM, producing `renderEncryptedSchema`. Both the master-encrypted and render-encrypted schemas are stored server-side. The `renderKey` itself is never sent to the server.

**Share URL structure:**

```
https://confide.app/f/<formId>#rk=<base64url(renderKey)>
```

The `#rk=` fragment is processed entirely by the browser. HTTP requests never include the fragment — it is not logged by our servers, CDN, or any intermediary. A respondent sharing the URL shares the render key, which is intentional and necessary.

**Key rotation:** A creator can rotate the render key at any time. Rotation generates a new `renderKey`, re-encrypts the schema, and produces a new share URL. Old URLs with the previous key stop working immediately.

> ℹ️ **Security note — two-key model:** The `renderKey` encrypts only the schema (field labels, questions, options). Response data is always encrypted with the `formKey` derived from the creator's `masterKey`. Knowing the `renderKey` (i.e. having the form link) does not allow reading any submitted responses. The separation is intentional and must be preserved.

### 4.3 Respondent rendering flow

1. Browser requests `/f/<formId>`. Server returns a generic shell HTML page — identical for every form, no form-specific content. The `#rk=...` fragment is never transmitted.
2. Client JS parses the fragment, extracts `renderKey`. If fragment is absent or malformed, the page shows a generic "invalid link" message — no information about whether the form exists.
3. Client fetches `GET /api/f/<formId>/schema` — returns `renderEncryptedSchema` and form status. No authentication required. Request carries no cookies.
4. Client decrypts `renderEncryptedSchema` using `renderKey` entirely in-browser via Web Crypto API. Renders the form from the decrypted schema JSON.
5. Locale is determined from `navigator.language` matched against available locales in the schema. No locale preference is sent to the server.

### 4.4 Response encryption & submission

1. Respondent submits the form. Client validates required fields locally. No server round-trip for validation.
2. Client constructs the `ResponsePayload` JSON (see 3.6). Generates a fresh random 256-bit `responseKey`.
3. Client fetches the creator's `publicFormKey` from the server — an X25519 public key derived from the `formKey` and published by the creator at form publish time. The private half never leaves the creator's client.
4. Client performs an X25519 ECDH exchange between its ephemeral keypair and the creator's `publicFormKey`. Derives a shared secret, feeds it through HKDF-SHA256 to produce the encryption key. Encrypts payload with AES-256-GCM. Discards the ephemeral private key immediately.
5. The submission body contains: `formId`, `encryptedPayload`, `ephemeralPublicKey`, `schemaVersion`. Nothing else. No session token, no cookies, no identifying headers.
6. Submission is POSTed to the relay endpoint. The relay queues and flushes to the API on a fixed interval.

### 4.5 Relay endpoint — timing attack mitigation

A direct POST to the API would produce server access logs timestamped to the second — potentially correlatable with other traffic. The relay endpoint breaks this link.

```
Respondent browser
  └─ POST /relay/submit   (stateless, no auth, no logging)
       └─ Relay service   (Go, separate process/container)
            └─ In-memory queue
                 └─ Batch flush every N seconds → internal API
                      └─ Database write (date-only timestamp)
```

**Relay properties:** The relay accepts the submission body, appends nothing (no IP, no timestamp, no request ID), and places it in an in-memory queue. Access logs on the relay endpoint are explicitly disabled — a deliberate operational policy enforced in infrastructure config. The relay is a separate Go process with no database access.

**Flush interval:** Default flush interval: 60 seconds. Configurable per deployment. On flush, all queued submissions are written in a single batch transaction. `receivedAt` is set to the current date only at write time.

**Relay failure & respondent UX:** If the relay returns a non-2xx response or times out, the client shows an inline error: "Your response could not be submitted. Please try again." The form remains filled — the respondent does not lose their answers. No local storage is used. The client retries up to 3 times with exponential backoff before surfacing the error.

> ⚠️ **Open design question — relay queue durability (DQ3):** The in-memory queue is lost if the relay process crashes before flushing. Accept client retry as the recovery mechanism (recommended for v1), or add write-ahead log. Decide before relay is built.

### 4.6 Headers & network hygiene on form pages

| Header | Value & rationale |
|--------|-------------------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'`. No external resources of any kind on `/f/*` routes. |
| `Referrer-Policy` | `no-referrer`. Prevents the form URL (containing the render key fragment) from leaking via referrer headers. |
| `Permissions-Policy` | Disable camera, microphone, geolocation, payment, USB. |
| `Cache-Control` | `no-store` on the schema endpoint. The encrypted schema blob must not be cached by intermediaries. |
| `Set-Cookie` | No cookies set on `/f/*` routes or `/relay/*` routes. Ever. |

### 4.7 Full data flow summary

```
Creator publishes form
  └─ generates renderKey
  └─ re-encrypts schema → renderEncryptedSchema
  └─ publishes publicFormKey (X25519)
  └─ uploads both to server
  └─ share URL = /f/<formId>#rk=<renderKey>

Respondent opens link
  └─ browser never sends #fragment to server
  └─ fetches renderEncryptedSchema (no cookies, no auth)
  └─ decrypts schema with renderKey in-browser
  └─ renders form locally

Respondent submits
  └─ ECDH(ephemeral privkey, publicFormKey) → sharedSecret
  └─ HKDF(sharedSecret) → encryptionKey
  └─ AES-256-GCM(payload, encryptionKey) → encryptedPayload
  └─ POST to relay (no IP logged, no cookies)
  └─ relay queues → batch flush every 60s
  └─ DB write: encryptedPayload + formId + date only

Creator views responses
  └─ authenticated session, masterKey in memory
  └─ derives formKey via HKDF(masterKey, formId)
  └─ derives privFormKey from formKey
  └─ ECDH(privFormKey, ephemeralPublicKey) → sharedSecret
  └─ decrypts each response locally
```

---

## 5. API Design & Backend Architecture

### 5.1 Service topology

The backend is split into two Go processes with strictly separated responsibilities. This separation enforces at the process level that the relay has zero database access and that the API has no exposure to raw submission traffic.

```
confide/
├── cmd/
│   ├── api/          # authenticated API + internal flush endpoint
│   └── relay/        # unauthenticated submission intake + queue
├── internal/
│   ├── auth/         # WebAuthn, session management
│   ├── crypto/       # key wrapping helpers (no business logic)
│   ├── forms/        # form CRUD, schema storage
│   ├── responses/    # response storage, retrieval
│   ├── relay/        # queue, batch flush logic
│   └── db/           # database access layer (api only)
├── migrations/       # SQL migrations (versioned)
└── deploy/
    ├── docker-compose.yml
    └── k8s/          # optional Kubernetes manifests
```

**API service:** Handles all authenticated creator operations. Has full database access. Exposes one internal-only flush endpoint consumed by the relay. Never directly reachable from form pages.

**Relay service:** Accepts unauthenticated submissions only. No database. No logging. Holds an in-memory queue and calls the API flush endpoint on interval. No other responsibilities.

### 5.2 API endpoints

**Auth**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/register/begin` | Returns WebAuthn registration challenge + PRF salt |
| POST | `/auth/register/finish` | Stores credential, wrappedMasterKey, recoveryVerifier. Returns session. |
| POST | `/auth/login/begin` | Returns WebAuthn assertion challenge + PRF salt for credential |
| POST | `/auth/login/finish` | Verifies assertion. Returns session + wrappedMasterKey. |
| POST | `/auth/recover` | Verifies recovery code hash. Returns recoveryWrappedMasterKey. Burns code. |
| POST | `/auth/logout` | Revokes session token server-side. Clears cookie. |

**Forms (authenticated)**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/forms` | Create form. Accepts encryptedSchema + publicFormKey. Returns formId. |
| GET | `/api/forms` | List forms for account. Returns formId, status, responseCount, dates. No schema. |
| GET | `/api/forms/:id` | Returns encryptedSchema + metadata. Creator decrypts client-side. |
| PUT | `/api/forms/:id` | Replace encryptedSchema + renderEncryptedSchema. Bumps schemaVersion. |
| PUT | `/api/forms/:id/status` | Toggle form open/closed. |
| DELETE | `/api/forms/:id` | Hard delete form + all associated response records. Irreversible. |

**Responses (authenticated)**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/forms/:id/responses` | Paginated list of encrypted response blobs. Cursor-based pagination. |
| GET | `/api/forms/:id/responses/:rid` | Single encrypted response blob + ephemeralPublicKey. |
| DELETE | `/api/forms/:id/responses/:rid` | Hard delete single response. Irreversible. |

**Public (unauthenticated)**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/f/:id/schema` | Returns renderEncryptedSchema + form status. No auth. No logging. Cache-Control: no-store. |
| POST | `/relay/submit` | Relay intake only. No auth, no logging. Queued for batch flush. |

### 5.3 Database schema

PostgreSQL. All binary fields storing encrypted data use `bytea`. Timestamps stored as `date` (not `timestamptz`) wherever coarsening is required by the threat model.

```sql
-- Accounts
CREATE TABLE accounts (
  id                       TEXT PRIMARY KEY,       -- random 128-bit base64url
  created_at               DATE NOT NULL,
  credential_id            BYTEA NOT NULL UNIQUE,
  public_key               BYTEA NOT NULL,
  prf_salt                 BYTEA NOT NULL,
  wrapped_master_key       BYTEA NOT NULL,         -- encrypted by PRF output
  recovery_wrapped_master  BYTEA NOT NULL,         -- encrypted by recoveryKey
  recovery_verifier        BYTEA NOT NULL          -- SHA-256(recoveryKey)
);

-- Recovery codes (hashed, single-use)
CREATE TABLE recovery_codes (
  id          TEXT PRIMARY KEY,
  account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  code_hash   BYTEA NOT NULL,                      -- SHA-256(code)
  used        BOOLEAN NOT NULL DEFAULT FALSE
);

-- Sessions
CREATE TABLE sessions (
  id          TEXT PRIMARY KEY,
  account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  token_hash  BYTEA NOT NULL UNIQUE,               -- SHA-256(sessionToken)
  created_at  DATE NOT NULL,
  last_seen   DATE NOT NULL
);

-- Forms
CREATE TABLE forms (
  id                        TEXT PRIMARY KEY,      -- random 128-bit base64url
  account_id                TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  created_at                DATE NOT NULL,
  updated_at                DATE NOT NULL,
  status                    TEXT NOT NULL DEFAULT 'open',
  schema_version            INTEGER NOT NULL DEFAULT 1,
  response_count            INTEGER NOT NULL DEFAULT 0,
  encrypted_schema          BYTEA NOT NULL,        -- encrypted by formKey
  render_encrypted_schema   BYTEA NOT NULL,        -- encrypted by renderKey
  public_form_key           BYTEA NOT NULL         -- X25519 public key
);

-- Responses
CREATE TABLE responses (
  id                   TEXT PRIMARY KEY,
  form_id              TEXT NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
  received_at          DATE NOT NULL,
  schema_version       INTEGER NOT NULL,
  encrypted_data       BYTEA NOT NULL,             -- AES-256-GCM
  ephemeral_public_key BYTEA NOT NULL              -- X25519 ephemeral pubkey
);
```

> ℹ️ **Schema design note:** There are no joins between response data and any user-identifying table at query time. A response record contains only: its own ID, a form ID, a date, a schema version, and two opaque byte arrays. An attacker with read-only database access cannot correlate a response to an account without also compromising the forms table.

### 5.4 Rate limiting & abuse prevention

Rate limiting is applied without logging IP addresses. We use a sliding window counter keyed on a short-lived HMAC of the client IP — the IP is never stored, only the hash, which rotates every 15 minutes.

| Endpoint | Limit |
|----------|-------|
| `/relay/submit` | 20 submissions / 10 min per rotating IP hash. Silently queued if exceeded. |
| `/auth/*` | 5 attempts / 5 min per rotating IP hash. Hard 429 on breach. |
| `/api/f/:id/schema` | 100 req / min per rotating IP hash. |
| `/api/*` (authenticated) | 200 req / min per session token hash. |

### 5.5 Self-hosting deployment

Self-hosting is a first-class deployment target. The full stack ships as a `docker-compose.yml` with four services:

```yaml
services:
  api:        # Go API binary, port 8080 (internal only)
  relay:      # Go relay binary, port 8081 (public-facing)
  web:        # SvelteKit frontend, port 3000 (public-facing)
  db:         # PostgreSQL 16, port 5432 (internal only)

  # Reverse proxy (nginx or Caddy) sits in front:
  # confide.example.com       → web:3000
  # confide.example.com/api   → api:8080
  # confide.example.com/relay → relay:8081
```

**Configuration:** All config via environment variables. A single `.env` file covers: database URL, relay flush interval, rate limit windows, CORS origin, and a server-side HMAC key used exclusively for the rotating IP hash rate limiter.

**Migrations:** Versioned SQL migrations in `/migrations`, applied automatically on API startup via an embedded migration runner.

**Backups:** Documented `pg_dump` backup strategy in the self-hosting guide. Backups contain only encrypted blobs — a leaked backup file is no more useful to an attacker than a leaked live database.

---

## 6. Frontend Architecture

### 6.1 Technology choices

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Framework | SvelteKit 2 | File-based routing with clean route segmentation between dashboard and form runtime. Server-side rendering used only for the generic shell HTML — no sensitive data ever passes through SSR. Svelte's compiled output produces significantly smaller bundles than React, making the 150KB form runtime budget comfortable to hit. |
| Crypto | Web Crypto API | Native browser API. No third-party crypto library. Keys are non-extractable CryptoKey objects where possible. Zero additional bundle weight. |
| WebAuthn | `@simplewebauthn/browser` | Handles PRF extension, credential management, and cross-browser normalization. |
| Drag and drop | `svelte-dnd-action` | Lightweight, accessible, touch-friendly DnD for Svelte. No jQuery. Keyboard reorder support required. |
| State management | Svelte stores | Built-in, zero-dependency reactive stores. The masterKey is held in a writable store scoped to the session — never serialised, never persisted, cleared on logout. |
| Styling | Tailwind CSS | Consistent design system across dashboard and form runtime. |

### 6.2 Route architecture

```
src/routes/
├── (marketing)/              # Public marketing site
│   ├── +page.svelte          # Landing page
│   └── pricing/+page.svelte
│
├── (auth)/                   # Signup / login / recovery
│   ├── signup/+page.svelte
│   ├── login/+page.svelte
│   └── recover/+page.svelte
│
├── (dashboard)/              # Authenticated creator area
│   ├── +layout.svelte        # Auth gate — redirects if no session
│   ├── +layout.server.ts     # Server-side session validation
│   ├── forms/+page.svelte    # Form list
│   ├── forms/new/+page.svelte
│   ├── forms/[id]/
│   │   ├── edit/+page.svelte        # Form builder
│   │   └── responses/+page.svelte  # Response viewer
│   └── settings/+page.svelte
│
└── f/                        # Form runtime — isolated segment
    └── [id]/
        ├── +page.svelte      # Respondent-facing form shell (generic, no form data)
        └── +page.server.ts   # Returns empty load — no form-specific SSR data
```

SvelteKit convention notes: `+page.server.ts` load functions run only on the server. The form runtime's `+page.server.ts` intentionally returns nothing — all form data is fetched and decrypted client-side after the shell loads. The dashboard's `+layout.server.ts` validates the session cookie and redirects to `/login` if absent or expired.

### 6.3 Crypto layer

All cryptographic operations are isolated in a single module at `lib/crypto.ts`. Nothing outside this module calls Web Crypto directly. This makes the crypto surface auditable in isolation and replaceable without touching application code.

```typescript
// lib/crypto.ts — public interface (implementations use Web Crypto only)

// Key derivation
deriveFormKey(masterKey: CryptoKey, formId: string): Promise<CryptoKey>
deriveFormKeypair(formKey: CryptoKey): Promise<CryptoKeyPair>  // X25519
deriveRecoveryKey(codes: string[]): Promise<CryptoKey>          // HKDF-SHA256

// Wrapping / unwrapping
wrapKey(key: CryptoKey, kek: CryptoKey): Promise<ArrayBuffer>
unwrapKey(wrapped: ArrayBuffer, kek: CryptoKey): Promise<CryptoKey>

// Schema encryption
encryptSchema(schema: FormSchema, key: CryptoKey): Promise<ArrayBuffer>
decryptSchema(blob: ArrayBuffer, key: CryptoKey): Promise<FormSchema>

// Response encryption (respondent-side, ephemeral ECDH)
encryptResponse(
  payload: ResponsePayload,
  recipientPublicKey: CryptoKey
): Promise<{ encryptedData: ArrayBuffer; ephemeralPublicKey: ArrayBuffer }>

// Response decryption (creator-side)
decryptResponse(
  encryptedData: ArrayBuffer,
  ephemeralPublicKey: ArrayBuffer,
  formPrivateKey: CryptoKey
): Promise<ResponsePayload>

// Verification
hashForVerification(data: ArrayBuffer): Promise<ArrayBuffer>  // SHA-256 via Web Crypto
```

> ✅ **DQ2 resolved — SHA-256 for verification hashes:** All verification hashes (recovery verifier, recovery code index, session token hash) use SHA-256 via the native Web Crypto API (`crypto.subtle.digest`). BLAKE3 via WASM is deferred out of v1. SHA-256 is well-audited, universally supported, and carries zero additional bundle weight.

### 6.4 Session & key store

The `masterKey` must survive Svelte component lifecycles and navigation between dashboard routes, but must never be written to any persistent storage.

```typescript
// store/session.ts
import { writable } from 'svelte/store';

interface SessionState {
  masterKey: CryptoKey | null   // in-memory only, never persisted
  accountId: string | null
}

const { subscribe, set, update } = writable<SessionState>({
  masterKey: null,
  accountId: null
});

export const session = {
  subscribe,
  setSession: (masterKey: CryptoKey, accountId: string) => set({ masterKey, accountId }),
  clearSession: () => set({ masterKey: null, accountId: null })
};

// Cleared on:
// - explicit logout
// - window/tab close (memory is freed by GC)
// - session expiry detected by API 401 response
```

> ℹ️ **Tab re-open behaviour:** When a user reopens a tab after closing it, the session cookie may still be valid but the `masterKey` is gone. The dashboard layout detects a null `masterKey` with a valid session and shows a lightweight re-auth prompt: "Confirm your identity to continue." This must feel seamless, not like an error.

### 6.5 Form builder

**Component structure:** Three-panel layout: a field palette on the left (drag source), a canvas in the centre (drop target, ordered field list), and a properties panel on the right (config for the selected field). On mobile the properties panel collapses into a bottom sheet.

**Local schema state:** The builder maintains the decrypted `FormSchema` in a local Svelte writable store. Every edit is applied optimistically to local state via reactive assignments. A 2-second debounced save encrypts the full schema and PUTs it to the API.

**Translation editor:** A locale switcher in the builder toolbar changes the editing context. In a non-default locale, each field shows its translation slot alongside the default locale value as a reference. Missing translations are highlighted in amber. The builder never allows publishing a form where the default locale has any empty required strings.

**Publish flow:** Publish is a distinct action from save. On publish: (1) client generates `renderKey`, (2) re-encrypts schema for render, (3) derives and uploads `publicFormKey`, (4) server sets status to open, (5) share URL is presented to creator with a copy button. The share URL is never shown in plaintext in any server-rendered page — it is assembled client-side from the formId + renderKey.

### 6.6 Response viewer

**Decryption model:** The viewer fetches paginated encrypted response blobs from the API. Each blob is decrypted client-side using the creator's `formKey`. Decryption happens in a Web Worker to avoid blocking the main thread on large response sets.

**Schema version handling:** Each response carries a `schemaVersion`. The viewer fetches the encrypted schema at that version from the server (versions are immutable snapshots stored on each PUT). Responses are always rendered against the schema they were submitted under.

**Export:** Responses can be exported to CSV entirely client-side — decrypt all responses in the browser, serialise to CSV, trigger a file download. No server involvement. The server never sees plaintext at export time. Export of encrypted blobs (for offline archival) is also available.

### 6.7 Form runtime — respondent experience

**Shell page strategy:** The `/f/[id]` route is a SvelteKit server-rendered route that returns a completely generic HTML shell — identical markup for every form ID. The `+page.server.ts` load function returns no form-specific data. The shell loads the client bundle which reads the URL fragment and begins the fetch-decrypt-render sequence.

**No external resources — ever:** The form runtime bundle contains zero external font requests, zero analytics, zero error tracking. Fonts are self-hosted. The CSP on `/f/*` enforces this at the HTTP level.

**Submission state machine:** The form UI has four states: `loading`, `filling`, `submitting`, and `done` or `error`. Transitions are one-way. The done state shows a generic confirmation with no submission ID or timestamp.

In `convo` mode the state machine is extended:

- `loading` → schema is fetched and decrypted, the first question bubble renders with a typing indicator animation before appearing.
- `filling` → one field is active at a time. The respondent types or selects a response in an input anchored to the bottom of the screen (SMS-style). Sending an answer transitions the input to the next field; the previous answer is frozen as an outgoing bubble in the thread above.
- `submitting` → triggered automatically when the last field is answered. No button press required. A typing indicator appears from the bot, followed by the `convoCompletionMessage` bubble, then the encrypted payload is posted to the relay.
- `done` → the completion bubble is already visible in the thread. No separate confirmation screen. The input area is replaced with a "This conversation is complete" notice.
- `error` → an inline error bubble appears in the thread from the bot: "Something went wrong. Please try sending again." The last answer input is re-enabled for retry.

**Accessibility:** WCAG 2.1 AA minimum. All form fields have associated labels. Error states use both colour and text. Focus management is handled explicitly in steps mode. RTL layout is tested with VoiceOver (Safari) and NVDA (Firefox).

### 6.8 CSP strategy by route segment

| Segment | Content-Security-Policy |
|---------|------------------------|
| `/f/*` | `default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; form-action 'none'` |
| `/relay/*` | No HTML served. JSON API only. CORS restricted to same origin. |
| `/dashboard/*` | `default-src 'self'; script-src 'self' 'nonce-{n}'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'` |
| `/(marketing)` | Standard. No tracking scripts regardless. |

### 6.9 Bundle & performance constraints

**Form runtime bundle:** Hard budget: 150KB gzipped. Svelte's compiled output has no runtime framework overhead — the base bundle is significantly lighter than an equivalent React app, giving comfortable headroom. No UI library. No date picker library (use native `input[type=date]`). No analytics. Budget enforced via `vite-bundle-analyzer` output checked in CI.

**Dashboard bundle:** Soft budget: 400KB gzipped. `svelte-dnd-action` and the BLAKE3 WASM module are removed from v1 scope — all hashing uses native Web Crypto. Svelte stores replace Zustand at zero additional bundle cost. Code-split aggressively — builder and response viewer are loaded lazily via SvelteKit's `import()` dynamic imports.

---

## 7. Privacy Policy & Legal Architecture

> ⚠️ **This section requires legal counsel before publication.** This document describes intended architecture and policy positions. It is not legal advice and does not constitute a final privacy policy. All positions marked "needs legal counsel" must be reviewed by a qualified attorney familiar with US privacy law, ECPA, and the specific threat model of anonymity-preserving services before any public commitments are made.

### 7.1 Jurisdiction & incorporation

Confide is incorporated in the United States. Data is hosted on US infrastructure. The US has broad surveillance authorities under ECPA, the Stored Communications Act, and FISA. These are not hypothetical concerns for a product serving journalists, non-profits, and vulnerable communities.

**Why US jurisdiction is still defensible:** The architecture is specifically designed so that legal compulsion of our servers yields nothing useful. A lawful subpoena under 18 U.S.C. § 2703 requires us to produce stored communications — we can only produce encrypted blobs we cannot decrypt. A National Security Letter can compel us to produce account records — we have only an opaque random ID with no PII attached. The legal resistance is architectural, not contractual.

**What remains a legal risk:** A court could order us to modify our software to log data going forward — a "prospective" surveillance order. This is the residual legal risk no technical architecture eliminates. Mitigation: open source codebase allows independent verification that no such modification has been made. Self-hosting removes this risk entirely for those deployments.

### 7.2 Data classification

| Tier | What we store | Can we read it? |
|------|---------------|-----------------|
| Operational | Account IDs (opaque), session token hashes, form IDs, response counts, date-only timestamps, WebAuthn public keys, encrypted key blobs | Yes — this is plaintext metadata required to operate the service. It contains no PII by design. |
| Encrypted | Form schemas (field labels, questions, options), response payloads (all answer data), render-encrypted schemas | No — encrypted client-side with keys we never possess. We store ciphertext only. |
| Never stored | IP addresses, email addresses, phone numbers, names, user agents, precise timestamps, geolocation, device fingerprints, cookies on form pages | Cannot — we never collect it. Enforced architecturally in the relay and API. |

### 7.3 Personal data & privacy law applicability

Our position is that creator accounts do not constitute personal data under CCPA or GDPR because accounts are pseudonymous by design — no name, email, phone number, or other identifier is collected or linked.

> ⚠️ **Needs legal counsel — pseudonymity & GDPR Art. 4:** GDPR Art. 4(5) defines pseudonymisation as processing that means data can no longer be attributed to a specific person without additional information. Regulators may still consider pseudonymous data as personal data if re-identification is possible in principle. Counsel should confirm whether passkey credential IDs — which are device-bound and potentially linkable to a device — constitute personal data under EU law.

Respondents are fully anonymous — no account, no identifier, no linkable metadata. Respondent data is not personal data by any reasonable interpretation.

### 7.4 Government data request handling

The specific response policy requires legal counsel and is not finalised. The following principles are settled:

| ID | Principle |
|----|-----------|
| P1 | We will produce only what we actually possess. If a request demands plaintext response content, we will comply by producing the encrypted blob — which is all we have. |
| P2 | We will review all requests for legal validity before complying. Informal requests, requests lacking proper legal authority, and overbroad requests will be challenged or declined pending proper process. |
| P3 | We will notify affected users of any data request where legally permitted to do so. |
| P4 | We will never voluntarily assist with surveillance beyond what is legally compelled. We will never build monitoring capabilities, back doors, or key escrow systems. |

> ⚠️ **Needs legal counsel — CALEA applicability:** The Communications Assistance for Law Enforcement Act (CALEA) imposes wiretap assistance obligations on certain telecommunications carriers and broadband providers. Whether CALEA applies to a form-collection SaaS is unsettled. Counsel should advise on whether our architecture falls outside CALEA's scope.

### 7.5 What a subpoena actually yields

| What is demanded | What we can produce | Usefulness to adversary |
|-----------------|---------------------|------------------------|
| Identity of account holder | Opaque random account ID, creation date (date only), WebAuthn credential ID | None without device access. Credential ID is meaningless without the physical authenticator. |
| Form content (questions, fields) | Encrypted schema blob. Unreadable without creator's masterKey. | None. AES-256-GCM ciphertext with no known plaintext attack surface. |
| Response content | Encrypted response blobs + ephemeral public keys. Unreadable without creator's formKey. | None. Each response uses a unique ECDH shared secret — no bulk decryption possible. |
| Identity of respondents | Nothing. No IP logs, no cookies, no session data, no timestamps beyond date-only. | None. Respondent anonymity is technically enforced, not policy-based. |
| When responses were submitted | Date only (e.g. "2025-03-14"). No time, no timezone. | Minimal. A date alone is insufficient for timing correlation. |
| Encryption keys | Wrapped key blobs only. Unreadable without the PRF output from the creator's physical passkey device. | None without physical device seizure. |

### 7.6 Public compliance posture

**What we promise — and can back up:** We never store IP addresses. We never store plaintext form content or responses. We never sell, share, or monetise any data. We cannot read your responses. We cannot identify your respondents. These are architectural facts, not policy pledges — verifiable in our open-source codebase.

**What we explicitly do not promise:** We do not promise protection from a court order requiring prospective logging. We do not promise protection if a creator's device is physically seized. We do not promise anonymity if a respondent voluntarily includes identifying information in a response. We do not promise protection from a nation-state adversary with resources beyond our threat model.

**User education — in-product:** A dedicated "How we protect you" page explains the threat model in plain language — no jargon, no marketing. It links to the relevant open-source code for each claim. A condensed version appears in the form footer for respondents.

**User education — creator onboarding:** During signup, creators are shown a plain-language summary of the key model, the consequences of losing their passkey and recovery codes, and what a subpoena of our servers would yield. This is not a checkbox — it is a mandatory read-through step before the account is created.

### 7.7 Terms of service constraints

| Standard ToS clause | Our position |
|--------------------|-|
| "We may access your content to enforce our policies" | Omitted. We are technically incapable of accessing content. |
| "We may share data with law enforcement upon valid request" | Modified: "We will produce only what we store, which is encrypted and non-identifying. See Section 7.5." |
| "You grant us a licence to use your content" | Omitted. We cannot read content. A licence to use content we cannot access is meaningless and misleading. |
| "We use cookies and analytics to improve the service" | Omitted for form pages. Modified for dashboard: session cookies only, no analytics, no third-party scripts. |
| "Account recovery via email verification" | Omitted. Recovery is via codes only. Total loss of credentials results in permanent account loss — stated explicitly. |

### 7.8 Open legal questions tracker

| ID   | Question                                                     |
| ---- | ------------------------------------------------------------ |
| L1   | GDPR applicability to pseudonymous accounts and WebAuthn credential IDs. |
| L2   | CALEA applicability. Determine whether Confide qualifies as a covered entity. |
| L3   | Government data request response policy. Finalise the specific legal posture with qualified counsel. |
| L4   | Liability for form content. Define the limits of our liability for forms used for illegal purposes, given we cannot monitor or moderate content. |
| L5   | Transparency report obligations. Determine whether any applicable law requires periodic disclosure of data requests received. |

---

## 8. Phased Delivery & Roadmap

### 8.1 Guiding principle

Every feature deferred to v2 is a deliberate choice, not a failure. v1 must be complete, secure, and honest — a smaller surface that does exactly what it promises with no half-implemented privacy properties. A v1 that ships with a partially implemented threat model is worse than no product at all, because it creates false confidence in users who need real protection.

### 8.2 v1 scope — in

**Authentication & identity**
- Passkey signup with WebAuthn PRF key derivation
- No email, no phone — opaque account ID only
- 12 single-use Signal-style recovery codes
- 30-day rolling sessions, re-auth on masterKey loss
- Session list + individual session revocation
- Mandatory recovery code onboarding step — cannot be skipped

**Encryption & zero-knowledge**
- E2E encrypted form schemas (AES-256-GCM, formKey)
- E2E encrypted responses (X25519 ECDH + AES-256-GCM per submission)
- renderKey in URL fragment — schema decrypted in-browser only
- Per-form key derivation via HKDF from masterKey
- Client-side response decryption in Web Worker
- Client-side CSV export — server never sees plaintext at export time

**Anonymous transport**
- Relay service — no IP logging, no access logs, batch flush every 60s
- Date-only timestamps on all stored records
- Strict CSP on `/f/*` — zero external resources on form pages
- No cookies on form or relay routes
- Rotating IP hash rate limiting — raw IPs never stored

**Form builder**
- Drag-and-drop builder with keyboard accessibility
- 8 field types: short text, long text, multiple choice, checkboxes, dropdown, date/time, rating, section break
- Scroll, steps, and convo layout modes
- Multi-language support with manual translation editor
- RTL layout support (Arabic, Hebrew, Persian, Urdu)
- 2-second debounce auto-save, explicit publish action
- Preview mode (edit vs respondent view toggle)
- renderKey rotation (invalidates old share URLs)
- Open/close form toggle

**Infrastructure**
- Go API + Go relay, two-process architecture
- PostgreSQL with versioned migrations
- Self-hosting via docker-compose (four services)
- 150KB gzipped form runtime bundle budget (CI enforced)
- Paid plans only — no ads, no freemium tier

### 8.3 v1 scope — explicitly out

| Feature | Reason deferred |
|---------|----------------|
| Tor / no-JS form submission | Requires significant respondent UX changes and a separate server-side rendering path. Clean v2 feature. |
| Email / push notifications on new responses | Email metadata (timing, volume) weakens the privacy model. Requires careful design before inclusion. |
| Multiple passkeys per account | Adds key management complexity. Single passkey + recovery codes is sufficient for v1. |
| Multi-creator collaboration on forms | Requires key sharing between accounts — a non-trivial cryptographic design problem. Single-owner in v1. |
| File upload fields | Requires server-side handling of binary content. Cannot be done without breaking zero-knowledge — needs architectural solution. |
| Conditional logic / branching | High builder complexity. Field type coverage is more impactful for v1 users. |
| Warrant canary | Legal complexity around maintenance obligations. Deferred pending counsel review. |
| Transparency reports | No requests to report in early operation. Build the reporting infrastructure in v2 once patterns are clear. |
| AI-assisted translation | Requires sending schema content to a third-party API — breaks schema confidentiality. Needs on-device model or local inference solution. |

### 8.4 Build sequence

**Phase 1 — Crypto foundation**

`lib/crypto.ts` — all primitives implemented and unit tested. WebAuthn PRF flow working end-to-end in a browser test harness. Key derivation hierarchy verified against known test vectors. All verification hashes use `crypto.subtle.digest('SHA-256', ...)` — no WASM dependencies.

_Exit criterion: independent review of crypto module by a second engineer. No application code written until this passes._

**Phase 2 — Go API skeleton + database**

PostgreSQL schema deployed. All auth endpoints implemented (`/auth/*`). Session management. WebAuthn server-side verification using go-webauthn. No form endpoints yet.

_Exit criterion: full signup → login → recovery → session revocation flow working via curl/Postman._

**Phase 3 — Auth UI + onboarding**

Signup, login, recovery flows in SvelteKit. Mandatory recovery code display and confirmation step. Re-auth prompt (masterKey gone, session valid). PRF browser compatibility gate.

_Exit criterion: a non-technical user can sign up, save their recovery codes, close the tab, and log back in without any guidance._

**Phase 4 — Form API + relay service**

All form and response endpoints. Go relay process — queue, flush interval, internal API call. Rate limiting on all public endpoints. Schema versioning on PUT.

_Exit criterion: encrypted schema stored, renderEncryptedSchema fetchable unauthenticated, submission relayed and stored, all verifiable without the server ever seeing plaintext._

**Phase 5 — Form builder**

Three-panel builder UI. All 8 field types. Drag-and-drop via `svelte-dnd-action`. Auto-save + publish flow. Translation editor. Preview mode. renderKey rotation.

_Exit criterion: creator can build, translate, preview, publish and share a multi-language form in under 3 minutes._

**Phase 6 — Form runtime + response viewer**

Respondent-facing form shell. Fragment key parsing. Schema decryption and rendering. Submission flow and state machine. Web Worker decryption. Response viewer with schema version handling. CSV export.

_Exit criterion: full end-to-end — form created, shared, submitted anonymously, and responses read by creator with zero plaintext ever touching the server._

**Phase 7 — Hardening & accessibility**

WCAG 2.1 AA audit. RTL layout testing. Bundle size enforcement in CI. CSP violation reporting endpoint. Penetration test of auth and relay endpoints. Load test of relay flush under concurrent submissions.

_Exit criterion: no critical or high findings unresolved. Bundle budgets passing. RTL verified on Arabic and Hebrew._

**Phase 8 — Self-hosting + launch**

docker-compose packaging. Self-hosting documentation. "How we protect you" page live. Legal review of ToS and privacy policy complete. Paid plan billing integration. Closed beta with 10–20 non-profit and journalist partners.

_Exit criterion: a self-hoster can go from zero to a running instance using only the public documentation._

### 8.5 Open design questions — consolidated tracker

| ID | Phase | Question |
|----|-------|----------|
| DQ1 | Phase 1 | **PRF unavailable fallback.** Block signup on unsupported browsers, or fall back to password-derived KEK? Recommendation: block with browser list. Decide before crypto layer is finalised. |
| ~~DQ2~~ | ~~Phase 1~~ | ~~**BLAKE3 vs SHA-256 for verification hashes.**~~ **Resolved:** SHA-256 via Web Crypto native. BLAKE3/WASM removed from v1 scope entirely. |
| DQ3 | Phase 4 | **Relay queue durability.** In-memory queue lost on relay crash. Accept client retry as the recovery mechanism (recommended for v1), or add write-ahead log. Decide before relay is built. |
| DQ4 | Phase 5 | **Schema versioning on live forms.** Snapshot model recommended — each PUT stores an immutable versioned blob. Decide before builder ships. |
| DQ5 | Phase 8 | **Legal questions L1–L5.** GDPR applicability, CALEA, government request policy, content liability, transparency reports. All must be resolved with qualified counsel before public launch. |

### 8.6 v2 candidates

| Feature | Notes |
|---------|-------|
| Multiple passkeys | Add laptop + YubiKey + phone. Requires wrapping masterKey under multiple KEKs and a device management UI. |
| Conditional logic | Show/hide fields based on previous answers. Evaluated client-side only — no server involvement needed. |
| Tor / no-JS submissions | A separate static HTML form rendering path that functions without JavaScript. Significant but high-impact for the highest-risk users. |
| Webhook delivery | Push encrypted blobs to a creator-defined endpoint on each submission. Creator decrypts on their own server. |
| Warrant canary | Regular signed attestation that no secret legal orders have been received. Implement once legal obligations are understood. |
| Transparency reports | Annual aggregate reporting: requests received, complied with, challenged. |
| File upload (E2E) | Encrypted file attachments in responses. Requires chunked client-side encryption and separate blob storage. |
| Multi-creator collaboration | Share form access with another account using public-key cryptography to transfer the formKey securely. |

### 8.7 Definition of done — v1

v1 is done when all of the following are simultaneously true:

| ID | Criterion |
|----|-----------|
| S1 | A full database dump yields zero plaintext response content and zero PII. |
| S2 | A network observer watching all traffic to our servers cannot link any submission to any respondent's IP address or session. |
| S3 | A non-technical user can create, translate, and share a form in under 3 minutes without reading documentation. |
| S4 | An independent auditor can verify all privacy claims from the open-source codebase alone. |
| S5 | A self-hoster can go from zero to a running instance using only public documentation. |
| S6 | Legal review of ToS and privacy policy is complete and signed off by qualified counsel. |
| S7 | Closed beta with at least 10 non-profit or journalist partners has completed with no critical privacy findings. |

---

*Confide Design Document — Draft v0.1*  
*All open design questions (DQ1–DQ5) and legal questions (L1–L5) must be resolved before the phases that depend on them.*
