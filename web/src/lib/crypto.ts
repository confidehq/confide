/**
 * GhostForm — E2E-encrypted anonymous form platform
 * Cryptographic foundation module (Phase 1)
 *
 * All functions are async and use the Web Crypto API (crypto.subtle).
 * No third-party crypto dependencies.
 *
 * Cryptographic parameters:
 *   Symmetric:   AES-256-GCM  (NIST AEAD, Web Crypto native)
 *   IV:          12 bytes random, prepended to every blob  [IV][ciphertext+tag]
 *   Tag:         128 bits (maximum)
 *   Key wrap:    AES-KW 256-bit  (RFC 3394, deterministic authenticated)
 *   HKDF hash:   SHA-256
 *   Asymmetric:  X25519  (Chrome 113+, Firefox 130+)
 *
 * Known limitations (Phase 7 candidates):
 *   - No AAD: AES-GCM additional authenticated data not yet bound to
 *     formId/responseId.  Cross-context substitution not currently prevented.
 *   - No memory zeroing: Web Crypto CryptoKey objects are opaque; JS
 *     cannot zero memory.  Accepted per design §2.5.
 */

import type { FormSchema, ResponsePayload, EncryptedResponse } from './types/crypto.ts';

// ---------------------------------------------------------------------------
// Internal constants
// ---------------------------------------------------------------------------

const AES_ALGORITHM = 'AES-GCM' as const;
const AES_KEY_LENGTH = 256;
const AES_IV_BYTES = 12;
const HKDF_HASH = 'SHA-256' as const;

/**
 * HKDF info strings — must be exact and consistent across all deployments.
 * Changing any of these is a breaking change that invalidates all stored keys.
 */
const encoder = new TextEncoder();
const encode = (s: string) => encoder.encode(s);

const INFO = {
	formKey: (formId: string) => encode(`wisp-form-key-v1:${formId}`),
	keypairSeed: () => encode('wisp-form-keypair-seed-v1'),
	recoveryKey: () => encode('wisp-recovery-key-v1'),
	responseEncKey: () => encode('wisp-response-enc-key-v1')
} as const;

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/**
 * Export an AES key's raw bytes and re-import as an HKDF IKM key.
 *
 * Web Crypto requires HKDF keys to be imported with algorithm "HKDF".
 * AES-GCM keys cannot be used directly as HKDF base keys.
 * The key MUST be extractable for this operation (this is a Web Crypto
 * API constraint, not a security weakness — see extractability notes below).
 */
async function toHkdfIkm(key: CryptoKey): Promise<CryptoKey> {
	const rawBytes = await crypto.subtle.exportKey('raw', key);
	return crypto.subtle.importKey('raw', rawBytes, 'HKDF', false, ['deriveKey', 'deriveBits']);
}

/**
 * Import raw bytes as an HKDF IKM key (for non-CryptoKey sources).
 */
async function importHkdfIkm(ikm: BufferSource): Promise<CryptoKey> {
	return crypto.subtle.importKey('raw', ikm, 'HKDF', false, ['deriveKey', 'deriveBits']);
}

/**
 * Derive an AES-GCM-256 key via HKDF from an HKDF IKM key.
 *
 * Salt is always empty (zero-length Uint8Array). The masterKey is already
 * pseudorandom; info strings provide domain separation between derived keys.
 */
async function hkdfDeriveAesKey(
	hkdfKey: CryptoKey,
	info: Uint8Array,
	usages: KeyUsage[],
	extractable: boolean
): Promise<CryptoKey> {
	return crypto.subtle.deriveKey(
		{
			name: 'HKDF',
			hash: HKDF_HASH,
			salt: new Uint8Array(0),
			info
		},
		hkdfKey,
		{ name: AES_ALGORITHM, length: AES_KEY_LENGTH },
		extractable,
		usages
	);
}

/**
 * Derive raw bits via HKDF from an HKDF IKM key.
 */
async function hkdfDeriveBits(hkdfKey: CryptoKey, info: Uint8Array, bits: number): Promise<ArrayBuffer> {
	return crypto.subtle.deriveBits(
		{
			name: 'HKDF',
			hash: HKDF_HASH,
			salt: new Uint8Array(0),
			info
		},
		hkdfKey,
		bits
	);
}

// ---------------------------------------------------------------------------
// Public interface — key derivation
// ---------------------------------------------------------------------------

/**
 * Derive a per-form AES-256-GCM key from the user's master key.
 *
 * masterKey extractability note:
 *   masterKey MUST be extractable for two reasons:
 *   1. `wrapKey()` requires it (Web Crypto API constraint)
 *   2. `deriveFormKey` exports raw bytes to re-import as HKDF IKM
 *   AES-KW provides the cryptographic protection; extractability is an API
 *   requirement, not a security weakness.
 *
 * formKey extractability note:
 *   formKey MUST be extractable so its raw bytes can be used as HKDF IKM
 *   in `deriveFormKeypair`. Web Crypto does not allow AES-GCM keys to be
 *   used directly as HKDF base keys — export+reimport is required.
 *   The raw bytes are never persisted; they live only in-memory during derivation.
 *   Security is preserved because formKey is always re-derivable from masterKey.
 */
export async function deriveFormKey(masterKey: CryptoKey, formId: string): Promise<CryptoKey> {
	const hkdfKey = await toHkdfIkm(masterKey);
	return hkdfDeriveAesKey(
		hkdfKey,
		INFO.formKey(formId),
		['encrypt', 'decrypt', 'wrapKey', 'unwrapKey'],
		true // SECURITY NOTE: extractable required for deriveFormKeypair — see above
	);
}

/**
 * PKCS8 header for an X25519 private key (RFC 8410 DER/ASN.1 encoding).
 *
 * OneAsymmetricKey SEQUENCE (46 bytes):
 *   version        INTEGER 0
 *   algorithm      SEQUENCE { OID 1.3.101.110 }   -- id-X25519
 *   privateKey     OCTET STRING { OCTET STRING (32 bytes) }
 *
 * Hex: 302e020100300506032b656e04220420  (16 bytes)
 *
 * This prefix is prepended to 32 raw seed bytes to form a valid PKCS8
 * X25519 private key that can be imported via crypto.subtle.importKey('pkcs8').
 * Web Crypto does not support importKey('raw', ...) for X25519 private keys
 * (raw format is reserved for public keys).
 */
const X25519_PKCS8_PREFIX = new Uint8Array([
	0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06,
	0x03, 0x2b, 0x65, 0x6e, 0x04, 0x22, 0x04, 0x20
]);

/**
 * Derive an X25519 keypair deterministically from a formKey.
 *
 * Algorithm:
 *   1. Export formKey raw bytes → import as HKDF IKM
 *   2. HKDF-deriveBits(256) → 32-byte seed
 *   3. Wrap seed in PKCS8 header → importKey('pkcs8', ...) → private key
 *   4. exportKey('jwk', privateKey) → extract 'x' (public key component)
 *   5. importKey('jwk', {kty:'OKP', crv:'X25519', x}) → public key
 *
 * Private key extractability note:
 *   The private key must be extractable so the JWK 'x' (public key) component
 *   can be extracted in step 4. The raw private key is never serialised to
 *   storage; it exists only in-memory during derivation.
 */
export async function deriveFormKeypair(formKey: CryptoKey): Promise<CryptoKeyPair> {
	// Step 1: export formKey raw bytes → HKDF IKM
	// SECURITY NOTE: formKey must be extractable; raw bytes are never persisted
	const hkdfKey = await toHkdfIkm(formKey);

	// Step 2: derive 256-bit seed
	const seedBits = await hkdfDeriveBits(hkdfKey, INFO.keypairSeed(), 256);

	// Step 3: wrap seed in PKCS8 header and import as X25519 private key
	// Web Crypto raw format for X25519 is only for PUBLIC keys; use pkcs8 for private.
	const pkcs8 = new Uint8Array(X25519_PKCS8_PREFIX.length + 32);
	pkcs8.set(X25519_PKCS8_PREFIX, 0);
	pkcs8.set(new Uint8Array(seedBits), X25519_PKCS8_PREFIX.length);

	// SECURITY NOTE: extractable=true required to extract public key via JWK export
	const privateKey = await crypto.subtle.importKey(
		'pkcs8',
		pkcs8,
		{ name: 'X25519' },
		true,
		['deriveKey', 'deriveBits']
	);

	// Step 4-5: export JWK to get public key 'x' component, then import as public key
	const jwk = await crypto.subtle.exportKey('jwk', privateKey) as JsonWebKey & { x: string };
	const publicKey = await crypto.subtle.importKey(
		'jwk',
		{ kty: 'OKP', crv: 'X25519', x: jwk.x },
		{ name: 'X25519' },
		true, // public key must be extractable for upload to server
		[]
	);

	return { privateKey, publicKey };
}

/**
 * Derive an AES-KW-256 key from a list of recovery codes.
 *
 * IKM = UTF-8 bytes of codes.join("").
 * Order-sensitive: [A, B] ≠ [B, A].
 */
export async function deriveRecoveryKey(codes: string[]): Promise<CryptoKey> {
	const ikmBytes = encode(codes.join(''));
	const ikmKey = await importHkdfIkm(ikmBytes);
	return crypto.subtle.deriveKey(
		{
			name: 'HKDF',
			hash: HKDF_HASH,
			salt: new Uint8Array(0),
			info: INFO.recoveryKey()
		},
		ikmKey,
		{ name: 'AES-KW', length: AES_KEY_LENGTH },
		false, // recoveryKey is only used for wrap/unwrap; never exported
		['wrapKey', 'unwrapKey']
	);
}

// ---------------------------------------------------------------------------
// Public interface — key wrapping
// ---------------------------------------------------------------------------

/**
 * Wrap a CryptoKey using AES-KW (RFC 3394).
 *
 * AES-KW is deterministic and authenticated; it adds 8 bytes of overhead.
 * No IV management required (AES-KW does not use an IV).
 */
export async function wrapKey(key: CryptoKey, kek: CryptoKey): Promise<ArrayBuffer> {
	return crypto.subtle.wrapKey('raw', key, kek, 'AES-KW');
}

/**
 * Unwrap a wrapped key using AES-KW.
 * Returns an AES-256-GCM key suitable for encrypt/decrypt/wrapKey/unwrapKey.
 *
 * SECURITY: unwrapped masterKey is extractable — see deriveFormKey comment.
 */
export async function unwrapKey(wrapped: ArrayBuffer, kek: CryptoKey): Promise<CryptoKey> {
	return crypto.subtle.unwrapKey(
		'raw',
		wrapped,
		kek,
		'AES-KW',
		{ name: AES_ALGORITHM, length: AES_KEY_LENGTH },
		true, // masterKey MUST be extractable — see design §6.3 and deriveFormKey comment
		['encrypt', 'decrypt', 'wrapKey', 'unwrapKey']
	);
}

// ---------------------------------------------------------------------------
// Public interface — schema encryption
// ---------------------------------------------------------------------------

/**
 * Encrypt a FormSchema with an AES-256-GCM key.
 *
 * Output layout: [12B random IV][ciphertext + 16B GCM tag]
 * Total overhead: 28 bytes (12 IV + 16 tag).
 */
export async function encryptSchema(schema: FormSchema, key: CryptoKey): Promise<ArrayBuffer> {
	const iv = crypto.getRandomValues(new Uint8Array(AES_IV_BYTES));
	const plaintext = encode(JSON.stringify(schema));

	const ciphertext = await crypto.subtle.encrypt(
		{ name: AES_ALGORITHM, iv, tagLength: 128 },
		key,
		plaintext
	);

	const blob = new Uint8Array(AES_IV_BYTES + ciphertext.byteLength);
	blob.set(iv, 0);
	blob.set(new Uint8Array(ciphertext), AES_IV_BYTES);
	return blob.buffer;
}

/**
 * Decrypt a FormSchema blob encrypted with encryptSchema.
 * Throws DOMException on authentication failure (wrong key, tampered data).
 */
export async function decryptSchema(blob: ArrayBuffer, key: CryptoKey): Promise<FormSchema> {
	const bytes = new Uint8Array(blob);
	const iv = bytes.slice(0, AES_IV_BYTES);
	const ciphertext = bytes.slice(AES_IV_BYTES);

	const plaintext = await crypto.subtle.decrypt(
		{ name: AES_ALGORITHM, iv, tagLength: 128 },
		key,
		ciphertext
	);

	return JSON.parse(new TextDecoder().decode(plaintext)) as FormSchema;
}

// ---------------------------------------------------------------------------
// Public interface — response encryption (ECIES-like with X25519 + AES-GCM)
// ---------------------------------------------------------------------------

/**
 * Encrypt a ResponsePayload for a recipient's X25519 public key.
 *
 * Uses an ephemeral X25519 keypair (ECIES pattern):
 *   1. Generate ephemeral X25519 keypair
 *   2. ECDH(ephemeralPrivKey, recipientPublicKey) → shared secret
 *   3. HKDF(sharedSecret) → AES-256-GCM encryption key
 *   4. AES-GCM encrypt JSON payload
 *   5. Return { encryptedData: [IV][ciphertext], ephemeralPublicKey: raw 32 bytes }
 *
 * A fresh ephemeral keypair is generated per call, providing message-level
 * forward secrecy.
 */
export async function encryptResponse(
	payload: ResponsePayload,
	recipientPublicKey: CryptoKey
): Promise<EncryptedResponse> {
	// Step 1: ephemeral keypair
	const ephemeral = await crypto.subtle.generateKey(
		{ name: 'X25519' },
		true, // extractable so we can export the public key
		['deriveKey', 'deriveBits']
	);

	// Step 2: ECDH shared secret
	// NOTE: X25519 uses { name: "X25519" }, NOT "ECDH" — "ECDH" is only for
	// P-256/P-384/P-521 named curves.
	const sharedSecret = await crypto.subtle.deriveKey(
		{ name: 'X25519', public: recipientPublicKey },
		ephemeral.privateKey,
		{ name: 'HKDF' },
		false,
		['deriveKey']
	);

	// Step 3: HKDF → AES-256-GCM encryption key
	const encryptionKey = await hkdfDeriveAesKey(
		sharedSecret,
		INFO.responseEncKey(),
		['encrypt'],
		false
	);

	// Step 4: AES-GCM encrypt
	const iv = crypto.getRandomValues(new Uint8Array(AES_IV_BYTES));
	const plaintext = encode(JSON.stringify(payload));
	const ciphertext = await crypto.subtle.encrypt(
		{ name: AES_ALGORITHM, iv, tagLength: 128 },
		encryptionKey,
		plaintext
	);

	const encryptedData = new Uint8Array(AES_IV_BYTES + ciphertext.byteLength);
	encryptedData.set(iv, 0);
	encryptedData.set(new Uint8Array(ciphertext), AES_IV_BYTES);

	// Step 5: export ephemeral public key (raw X25519 = 32 bytes)
	const ephemeralPublicKey = await crypto.subtle.exportKey('raw', ephemeral.publicKey);

	return { encryptedData: encryptedData.buffer, ephemeralPublicKey };
}

/**
 * Decrypt an EncryptedResponse using the form's X25519 private key.
 * Throws DOMException on authentication failure (wrong key, tampered data).
 */
export async function decryptResponse(
	encryptedData: ArrayBuffer,
	ephemeralPublicKey: ArrayBuffer,
	formPrivateKey: CryptoKey
): Promise<ResponsePayload> {
	// Import ephemeral public key
	const ephemeralPubKey = await crypto.subtle.importKey(
		'raw',
		ephemeralPublicKey,
		{ name: 'X25519' },
		true,
		[]
	);

	// ECDH shared secret
	const sharedSecret = await crypto.subtle.deriveKey(
		{ name: 'X25519', public: ephemeralPubKey },
		formPrivateKey,
		{ name: 'HKDF' },
		false,
		['deriveKey']
	);

	// HKDF → AES-256-GCM decryption key
	const decryptionKey = await hkdfDeriveAesKey(
		sharedSecret,
		INFO.responseEncKey(),
		['decrypt'],
		false
	);

	// AES-GCM decrypt
	const bytes = new Uint8Array(encryptedData);
	const iv = bytes.slice(0, AES_IV_BYTES);
	const ciphertext = bytes.slice(AES_IV_BYTES);

	const plaintext = await crypto.subtle.decrypt(
		{ name: AES_ALGORITHM, iv, tagLength: 128 },
		decryptionKey,
		ciphertext
	);

	return JSON.parse(new TextDecoder().decode(plaintext)) as ResponsePayload;
}

// ---------------------------------------------------------------------------
// Public interface — hashing
// ---------------------------------------------------------------------------

/**
 * SHA-256 hash for integrity verification (NIST FIPS 180-4).
 * Accepts any BufferSource (ArrayBuffer, TypedArray, DataView).
 */
export async function hashForVerification(data: BufferSource): Promise<ArrayBuffer> {
	return crypto.subtle.digest('SHA-256', data);
}

// ---------------------------------------------------------------------------
// Internal — recovery code generation
// ---------------------------------------------------------------------------

/**
 * Character set for recovery codes: A-Z0-9 (36 characters).
 * Rejection sampling: bytes ≥ 252 are discarded to eliminate modulo bias.
 * floor(256 / 36) * 36 = 252 — the rejection threshold.
 */
const RECOVERY_CHARSET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
const RECOVERY_CHARSET_LEN = RECOVERY_CHARSET.length; // 36
const REJECTION_THRESHOLD = 252; // 7 * 36 = 252

function generateCodeSegment(length: number): string {
	let result = '';
	while (result.length < length) {
		const bytes = crypto.getRandomValues(new Uint8Array(length * 2));
		for (const byte of bytes) {
			if (byte < REJECTION_THRESHOLD) {
				result += RECOVERY_CHARSET[byte % RECOVERY_CHARSET_LEN];
				if (result.length === length) break;
			}
		}
	}
	return result;
}

/**
 * Generate a single recovery code string in GHRK-XXXX-XXXX-...-XXXX format.
 *
 * Structure: fixed prefix "GHRK" + 12 random 4-character segments, dash-separated.
 * The 12 random segments together form the key material for recovery key derivation.
 * Not part of the core crypto interface; exported for use in auth flows.
 */
export function generateRecoveryCode(): string {
	const segments = Array.from({ length: 12 }, () => generateCodeSegment(4));
	return `GHRK-${segments.join('-')}`;
}

/**
 * Parse a recovery code string into its 12 key segments.
 * Strips whitespace, uppercases, then splits on '-' and drops the 'GHRK' prefix.
 * Throws if the format is invalid.
 */
export function parseRecoveryCode(code: string): string[] {
	const parts = code.toUpperCase().replace(/\s/g, '').split('-');
	if (parts[0] !== 'GHRK' || parts.length !== 13) {
		throw new Error('Invalid recovery code — expected GHRK-XXXX-XXXX-...-XXXX (12 segments)');
	}
	return parts.slice(1);
}
