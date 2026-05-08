# Security Architecture

Confide is built on a "Zero-Knowledge" architecture. The platform operator is technically incapable of reading form content or identifying respondents. This document outlines the cryptographic protocols and security measures that enforce these guarantees.

## 1. Core Identity & Authentication

Confide uses **WebAuthn Passkeys** with the **PRF (Pseudo-Random Function) Extension** as the primary authentication and key derivation mechanism.

### 1.1 Account Identity

- **Opaque IDs:** Accounts are identified by a random 128-bit value (`accountId`). No email, phone number, or personally identifying information (PII) is required or stored.
- **Passkey-Bound:** Identity is tied to a physical authenticator (Security Key, Touch ID, Face ID).

### 1.2 The Master Key

At the root of every account is a 256-bit random **Master Key**. This key never leaves the user's device in plaintext. 

- **Wrapping:** During signup/login, the WebAuthn PRF extension produces a deterministic HMAC output (using a server-provided salt). This output is used as a Key-Encryption Key (KEK) to wrap the Master Key.
- **Storage:** The server stores the `wrappedMasterKey`. It can only be unwrapped by the client during an active WebAuthn session.
- **In-Memory Only:** The plaintext Master Key is held in JavaScript memory only. It is cleared on tab close, logout, or session expiry. It is never written to `localStorage` or `IndexedDB`.

### 1.3 Recovery Codes

Since there is no "Forgot Password" (no email), Confide generates 12 single-use recovery codes during onboarding. 

- **Derivation:** A `recoveryKey` is derived from these codes via HKDF-SHA256. 
- **Backup:** The Master Key is also stored wrapped with this `recoveryKey`.
- **Zero-Knowledge Recovery:** The server stores only a hash of the recovery codes to verify them; it never sees the plaintext codes or the `recoveryKey`.

---

## 2. Workspace & Collaborative Security

Forms in Confide belong to **Workspaces**. Access to a workspace is managed through a hierarchy of keys that allows for secure collaboration without a central authority.

### 2.1 Identity Keypairs

Every account has an **X25519 Identity Keypair**.

- The **Private Identity Key** is wrapped with the user's Master Key and stored on the server.
- The **Public Identity Key** is stored in plaintext and is visible to other workspace members.

### 2.2 Workspace Keys

Each workspace has a unique **Workspace Key** (AES-256).

- **Distribution:** To grant access, an existing admin wraps the Workspace Key using **ECIES** (X25519 ECDH + AES-KW) for the new member's Public Identity Key.
- **Access:** A member unwraps their copy of the Workspace Key using their Identity Private Key (which they first unwrap with their Master Key).

### 2.3 Form Keys

Each form has a **Form Key** derived deterministically from the Workspace Key:
`formKey = HKDF(workspaceKey, "confide-form-v1" || formId)`

---

## 3. Multiple Devices & Pairing

Users can add multiple passkeys or devices to a single account.

### 3.1 Multi-Passkey Support

An account can have multiple registered credentials. Each credential has its own unique `prf_salt` and its own `wrappedMasterKey` (wrapping the same underlying Master Key). This allows any of the registered devices to unlock the account independently.

### 3.2 Device Pairing (QR Transfer)

To add a new device without using recovery codes:

1. **Initiation:** The existing device generates a short-lived pairing token and displays it as a QR code.
2. **Key Exchange:** The new device scans the QR and sends its ephemeral X25519 Public Key to the server.
3. **Fingerprint Verification:** Both devices display a 4-word fingerprint (e.g., `river-token-lamp-frost`) derived from the exchange. The user must manually confirm they match.
4. **Transfer:** The existing device wraps the Master Key for the new device's public key (ECIES) and sends it to the server.
5. **Finalization:** The new device retrieves the wrapped key, decrypts it, and registers its own local passkey.

---

## 4. Respondent Privacy & Form Submission

The security model for respondents is designed to ensure absolute anonymity and end-to-end encryption.

### 4.1 Form Rendering (The Render Key)

The server cannot read the form schema (the questions).

- **Two-Key Model:** The schema is encrypted with a **Render Key** (separate from the Form Key).
- **Fragment Storage:** The Render Key is included in the share URL's fragment (e.g., `.../f/id#rk=...`). 
- **Client-Side Decryption:** Browser fragments are never sent to the server. The respondent's browser fetches the encrypted schema and decrypts it locally using the key from the URL.

### 4.2 Response Encryption

When a respondent submits a form:

1. **Key Exchange:** The client fetches the creator's **Public Form Key** (X25519).
2. **Encryption:** It generates an ephemeral keypair and performs an ECDH exchange to derive a shared secret. The response is encrypted using AES-256-GCM.
3. **Forward Secrecy:** The ephemeral private key is discarded immediately after encryption.

### 4.3 Anonymous Transport (The Relay)

To prevent network-level correlation (linking an IP to a submission time):

- **No Logs:** Access logs and IP logging are disabled on the submission endpoint.
- **Relay Queue:** Submissions are sent to a stateless **Relay Service** (separate from the main API).
- **Batching:** The Relay holds submissions in an in-memory queue and "flushes" them to the database in batches every 60 seconds.
- **Coarsened Metadata:** All timestamps stored in the database are date-only (no time) to further prevent correlation attacks.

---

## 5. Security Standards Summary

| Layer                    | Primitive                   |
| ------------------------ | --------------------------- |
| **Symmetric Encryption** | AES-256-GCM                 |
| **Key Wrapping**         | AES-KW (RFC 3394)           |
| **Asymmetric / ECDH**    | X25519                      |
| **Key Derivation**       | HKDF-SHA256                 |
| **Hashing**              | SHA-256                     |
| **Identity**             | WebAuthn (FIDO2 / Passkeys) |
