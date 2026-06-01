/**
 * Confide crypto.ts — unit tests
 *
 * ~35 tests across 9 groups.
 * All tests are async and use the real Web Crypto API (no mocking).
 * Environment: node (FIPS-validated Web Crypto, X25519 supported)
 */

import { beforeAll, describe, expect, it } from "vitest";
import {
	decryptResponse,
	decryptSchema,
	deriveFormKey,
	deriveFormKeypair,
	deriveRecoveryKey,
	encryptResponse,
	encryptSchema,
	generateRecoveryCode,
	hashForVerification,
	unwrapKey,
	wrapKey,
} from "./crypto.ts";
import type { FormSchema, ResponsePayload } from "./types/crypto.ts";

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const TEST_FORM_ID = "test-form-abc123";

const SAMPLE_SCHEMA: FormSchema = {
	version: 1,
	defaultLocale: "en",
	locales: ["en", "es"],
	layout: "scroll",
	fields: [
		{ id: "q1", type: "short_text", required: false, order: 0, config: {} },
		{
			id: "q2",
			type: "multiple_choice",
			required: false,
			order: 1,
			config: {
				options: [
					{ id: "opt1", order: 0 },
					{ id: "opt2", order: 1 },
				],
			},
		},
	],
	translations: {
		en: {
			formTitle: "Test Form",
			formDescription: "",
			fields: {
				q1: { label: "What is your name?" },
				q2: { label: "Favorite color?", options: ["red", "blue"] },
			},
		},
		es: {
			formTitle: "Formulario de prueba",
			formDescription: "",
			fields: {
				q1: { label: "¿Cómo te llamas?" },
				q2: { label: "¿Color favorito?", options: ["rojo", "azul"] },
			},
		},
	},
};

const SAMPLE_RESPONSE: ResponsePayload = {
	submittedAt: "2026-03-19T00:00:00.000Z",
	locale: "en",
	answers: { q1: "Alice", q2: "blue", q3: null, q4: 42 },
};

async function generateMasterKey(): Promise<CryptoKey> {
	return crypto.subtle.generateKey(
		{ name: "AES-GCM", length: 256 },
		true, // extractable required for wrapKey
		["encrypt", "decrypt", "wrapKey", "unwrapKey"],
	);
}

// ---------------------------------------------------------------------------
// 1. hashForVerification — NIST FIPS 180-4 SHA-256 known-answer vectors
// ---------------------------------------------------------------------------

// Helper: safely encode a string to an ArrayBuffer (avoids TextEncoder buffer over-allocation)
function toArrayBuffer(str: string): ArrayBuffer {
	const bytes = new TextEncoder().encode(str);
	// Use slice to ensure buffer byteLength === byte count (TextEncoder may over-allocate)
	return bytes.buffer.slice(
		bytes.byteOffset,
		bytes.byteOffset + bytes.byteLength,
	);
}

describe("hashForVerification", () => {
	it("empty input matches NIST SHA-256 vector", async () => {
		const result = await hashForVerification(new Uint8Array(0));
		const hex = Array.from(new Uint8Array(result))
			.map((b) => b.toString(16).padStart(2, "0"))
			.join("");
		expect(hex).toBe(
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		);
	});

	it('"abc" matches SHA-256 vector', async () => {
		// Verified against OpenSSL, Python hashlib, and Node.js Web Crypto
		const input = new Uint8Array([0x61, 0x62, 0x63]); // 'abc' in ASCII/UTF-8
		const result = await hashForVerification(input);
		const hex = Array.from(new Uint8Array(result))
			.map((b) => b.toString(16).padStart(2, "0"))
			.join("");
		expect(hex).toBe(
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		);
	});

	it("448-bit message matches NIST SHA-256 vector", async () => {
		// "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq"
		const input = toArrayBuffer(
			"abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
		);
		const result = await hashForVerification(input);
		const hex = Array.from(new Uint8Array(result))
			.map((b) => b.toString(16).padStart(2, "0"))
			.join("");
		expect(hex).toBe(
			"248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1",
		);
	});

	it("returns 32-byte (256-bit) digest", async () => {
		const result = await hashForVerification(toArrayBuffer("test"));
		expect(result.byteLength).toBe(32);
	});
});

// ---------------------------------------------------------------------------
// 2. deriveFormKey
// ---------------------------------------------------------------------------

describe("deriveFormKey", () => {
	let masterKey: CryptoKey;
	beforeAll(async () => {
		masterKey = await generateMasterKey();
	});

	it("returns a CryptoKey with AES-GCM algorithm", async () => {
		const key = await deriveFormKey(masterKey, TEST_FORM_ID);
		expect(key.type).toBe("secret");
		expect((key.algorithm as AesKeyAlgorithm).name).toBe("AES-GCM");
		expect((key.algorithm as AesKeyAlgorithm).length).toBe(256);
	});

	it("is deterministic — same inputs produce equivalent key", async () => {
		const k1 = await deriveFormKey(masterKey, TEST_FORM_ID);
		const k2 = await deriveFormKey(masterKey, TEST_FORM_ID);
		// Both keys should decrypt what the other encrypts
		const iv = crypto.getRandomValues(new Uint8Array(12));
		const plaintext = new TextEncoder().encode("hello");
		const ciphertext = await crypto.subtle.encrypt(
			{ name: "AES-GCM", iv },
			k1,
			plaintext,
		);
		const decrypted = await crypto.subtle.decrypt(
			{ name: "AES-GCM", iv },
			k2,
			ciphertext,
		);
		expect(new TextDecoder().decode(decrypted)).toBe("hello");
	});

	it("different formIds produce different keys", async () => {
		const k1 = await deriveFormKey(masterKey, "form-aaa");
		const k2 = await deriveFormKey(masterKey, "form-bbb");
		const iv = crypto.getRandomValues(new Uint8Array(12));
		const plaintext = new TextEncoder().encode("data");
		const ciphertext = await crypto.subtle.encrypt(
			{ name: "AES-GCM", iv },
			k1,
			plaintext,
		);
		await expect(
			crypto.subtle.decrypt({ name: "AES-GCM", iv }, k2, ciphertext),
		).rejects.toThrow();
	});

	it("different masterKeys produce different formKeys", async () => {
		const mk2 = await generateMasterKey();
		const k1 = await deriveFormKey(masterKey, TEST_FORM_ID);
		const k2 = await deriveFormKey(mk2, TEST_FORM_ID);
		const iv = crypto.getRandomValues(new Uint8Array(12));
		const plaintext = new TextEncoder().encode("data");
		const ciphertext = await crypto.subtle.encrypt(
			{ name: "AES-GCM", iv },
			k1,
			plaintext,
		);
		await expect(
			crypto.subtle.decrypt({ name: "AES-GCM", iv }, k2, ciphertext),
		).rejects.toThrow();
	});

	it("formKey is extractable (required for HKDF IKM re-import in deriveFormKeypair)", async () => {
		// SECURITY NOTE: extractable=true is required so deriveFormKeypair can export raw bytes
		// for use as HKDF IKM. Raw bytes are never persisted; formKey is always re-derivable.
		const key = await deriveFormKey(masterKey, TEST_FORM_ID);
		expect(key.extractable).toBe(true);
	});
});

// ---------------------------------------------------------------------------
// 3. deriveFormKeypair
// ---------------------------------------------------------------------------

describe("deriveFormKeypair", () => {
	let masterKey: CryptoKey;
	let formKey: CryptoKey;

	beforeAll(async () => {
		masterKey = await generateMasterKey();
		formKey = await deriveFormKey(masterKey, TEST_FORM_ID);
	});

	it("returns an X25519 keypair", async () => {
		const kp = await deriveFormKeypair(formKey);
		expect(kp.privateKey.type).toBe("private");
		expect(kp.publicKey.type).toBe("public");
		expect(kp.privateKey.algorithm.name).toBe("X25519");
		expect(kp.publicKey.algorithm.name).toBe("X25519");
	});

	it("public key is extractable", async () => {
		const kp = await deriveFormKeypair(formKey);
		expect(kp.publicKey.extractable).toBe(true);
	});

	it("is deterministic — same formKey produces same keypair", async () => {
		const kp1 = await deriveFormKeypair(formKey);
		const kp2 = await deriveFormKeypair(formKey);
		const pub1 = await crypto.subtle.exportKey("raw", kp1.publicKey);
		const pub2 = await crypto.subtle.exportKey("raw", kp2.publicKey);
		expect(new Uint8Array(pub1)).toEqual(new Uint8Array(pub2));
	});

	it("different formKeys produce different keypairs", async () => {
		const fk2 = await deriveFormKey(masterKey, "other-form");
		const kp1 = await deriveFormKeypair(formKey);
		const kp2 = await deriveFormKeypair(fk2);
		const pub1 = await crypto.subtle.exportKey("raw", kp1.publicKey);
		const pub2 = await crypto.subtle.exportKey("raw", kp2.publicKey);
		expect(new Uint8Array(pub1)).not.toEqual(new Uint8Array(pub2));
	});

	it("ECDH with derived keypair produces usable shared secret", async () => {
		const kp = await deriveFormKeypair(formKey);
		// Generate a second keypair to do ECDH with
		const ephemeral = (await crypto.subtle.generateKey(
			{ name: "X25519" },
			true,
			["deriveKey", "deriveBits"],
		)) as CryptoKeyPair;
		// Both directions should yield the same bits
		const bits1 = await crypto.subtle.deriveBits(
			{ name: "X25519", public: kp.publicKey },
			ephemeral.privateKey,
			256,
		);
		const bits2 = await crypto.subtle.deriveBits(
			{ name: "X25519", public: ephemeral.publicKey },
			kp.privateKey,
			256,
		);
		expect(new Uint8Array(bits1)).toEqual(new Uint8Array(bits2));
	});
});

// ---------------------------------------------------------------------------
// 4. deriveRecoveryKey
// ---------------------------------------------------------------------------

describe("deriveRecoveryKey", () => {
	const CODES = ["ABCD-1234", "EFGH-5678", "IJKL-9012"];

	it("returns an AES-KW key", async () => {
		const key = await deriveRecoveryKey(CODES);
		expect(key.type).toBe("secret");
		expect((key.algorithm as AesKeyAlgorithm).name).toBe("AES-KW");
		expect((key.algorithm as AesKeyAlgorithm).length).toBe(256);
	});

	it("is deterministic", async () => {
		const k1 = await deriveRecoveryKey(CODES);
		const k2 = await deriveRecoveryKey(CODES);
		// Verify by wrap/unwrap round-trip with both keys
		const masterKey = await generateMasterKey();
		const wrapped = await crypto.subtle.wrapKey("raw", masterKey, k1, "AES-KW");
		const unwrapped = await crypto.subtle.unwrapKey(
			"raw",
			wrapped,
			k2,
			"AES-KW",
			{ name: "AES-GCM", length: 256 },
			true,
			["encrypt", "decrypt"],
		);
		expect(unwrapped).toBeDefined();
	});

	it("is order-sensitive: [A,B] ≠ [B,A]", async () => {
		const k1 = await deriveRecoveryKey(["CODE-A", "CODE-B"]);
		const k2 = await deriveRecoveryKey(["CODE-B", "CODE-A"]);
		const masterKey = await generateMasterKey();
		const wrapped = await crypto.subtle.wrapKey("raw", masterKey, k1, "AES-KW");
		await expect(
			crypto.subtle.unwrapKey(
				"raw",
				wrapped,
				k2,
				"AES-KW",
				{ name: "AES-GCM", length: 256 },
				true,
				["encrypt", "decrypt"],
			),
		).rejects.toThrow();
	});

	it("different codes produce different keys", async () => {
		const k1 = await deriveRecoveryKey(["AAAA-BBBB"]);
		const k2 = await deriveRecoveryKey(["CCCC-DDDD"]);
		const masterKey = await generateMasterKey();
		const wrapped = await crypto.subtle.wrapKey("raw", masterKey, k1, "AES-KW");
		await expect(
			crypto.subtle.unwrapKey(
				"raw",
				wrapped,
				k2,
				"AES-KW",
				{ name: "AES-GCM", length: 256 },
				true,
				["encrypt", "decrypt"],
			),
		).rejects.toThrow();
	});

	it("recoveryKey is not extractable", async () => {
		const key = await deriveRecoveryKey(CODES);
		expect(key.extractable).toBe(false);
	});
});

// ---------------------------------------------------------------------------
// 5. wrapKey / unwrapKey
// ---------------------------------------------------------------------------

describe("wrapKey / unwrapKey", () => {
	let masterKey: CryptoKey;
	let kek: CryptoKey;

	beforeAll(async () => {
		masterKey = await generateMasterKey();
		kek = await deriveRecoveryKey(["WRAP-TEST-CODE"]);
	});

	it("wrapped length equals raw key length + 8 bytes (AES-KW overhead)", async () => {
		const wrapped = await wrapKey(masterKey, kek);
		// AES-256 key = 32 bytes raw; AES-KW adds 8 bytes → 40 bytes
		expect(wrapped.byteLength).toBe(40);
	});

	it("round-trip: unwrapped key decrypts what original encrypted", async () => {
		const wrapped = await wrapKey(masterKey, kek);
		const unwrapped = await unwrapKey(wrapped, kek);

		const iv = crypto.getRandomValues(new Uint8Array(12));
		const plaintext = new TextEncoder().encode("wrap-test");
		const ciphertext = await crypto.subtle.encrypt(
			{ name: "AES-GCM", iv },
			masterKey,
			plaintext,
		);
		const decrypted = await crypto.subtle.decrypt(
			{ name: "AES-GCM", iv },
			unwrapped,
			ciphertext,
		);
		expect(new TextDecoder().decode(decrypted)).toBe("wrap-test");
	});

	it("tampered wrapped bytes cause unwrapKey to throw", async () => {
		const wrapped = await wrapKey(masterKey, kek);
		const tampered = new Uint8Array(wrapped);
		tampered[0] ^= 0xff;
		await expect(unwrapKey(tampered.buffer, kek)).rejects.toThrow();
	});
});

// ---------------------------------------------------------------------------
// 6. encryptSchema / decryptSchema
// ---------------------------------------------------------------------------

describe("encryptSchema / decryptSchema", () => {
	let formKey: CryptoKey;

	beforeAll(async () => {
		const masterKey = await generateMasterKey();
		formKey = await deriveFormKey(masterKey, TEST_FORM_ID);
	});

	it("round-trips schema with equality", async () => {
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey);
		const recovered = await decryptSchema(blob, formKey);
		expect(recovered).toEqual(SAMPLE_SCHEMA);
	});

	it("output length = plaintext length + 28 bytes (12 IV + 16 GCM tag)", async () => {
		const plaintext = JSON.stringify(SAMPLE_SCHEMA);
		const plaintextBytes = new TextEncoder().encode(plaintext).byteLength;
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey);
		expect(blob.byteLength).toBe(plaintextBytes + 28);
	});

	it("each call produces different ciphertext (random IV)", async () => {
		const blob1 = await encryptSchema(SAMPLE_SCHEMA, formKey);
		const blob2 = await encryptSchema(SAMPLE_SCHEMA, formKey);
		expect(new Uint8Array(blob1)).not.toEqual(new Uint8Array(blob2));
	});

	it("bit-flip in ciphertext body causes decryptSchema to throw", async () => {
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey);
		const tampered = new Uint8Array(blob);
		tampered[20] ^= 0x01; // flip a byte in ciphertext (after 12-byte IV)
		await expect(decryptSchema(tampered.buffer, formKey)).rejects.toThrow();
	});

	it("bit-flip in IV causes decryptSchema to throw", async () => {
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey);
		const tampered = new Uint8Array(blob);
		tampered[3] ^= 0x01; // flip a byte in the IV
		await expect(decryptSchema(tampered.buffer, formKey)).rejects.toThrow();
	});

	it("wrong key causes decryptSchema to throw", async () => {
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey);
		const wrongMaster = await generateMasterKey();
		const wrongKey = await deriveFormKey(wrongMaster, TEST_FORM_ID);
		await expect(decryptSchema(blob, wrongKey)).rejects.toThrow();
	});
});

// ---------------------------------------------------------------------------
// 7. encryptResponse / decryptResponse
// ---------------------------------------------------------------------------

describe("encryptResponse / decryptResponse", () => {
	let keypair: CryptoKeyPair;

	beforeAll(async () => {
		const masterKey = await generateMasterKey();
		const formKey = await deriveFormKey(masterKey, TEST_FORM_ID);
		keypair = await deriveFormKeypair(formKey);
	});

	it("round-trips response payload with equality", async () => {
		const encrypted = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		const recovered = await decryptResponse(
			encrypted.encryptedData,
			encrypted.ephemeralPublicKey,
			keypair.privateKey,
		);
		expect(recovered).toEqual(SAMPLE_RESPONSE);
	});

	it("ephemeralPublicKey is 32 bytes (raw X25519)", async () => {
		const encrypted = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		expect(encrypted.ephemeralPublicKey.byteLength).toBe(32);
	});

	it("each call uses a fresh ephemeral keypair (different ciphertext)", async () => {
		const e1 = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		const e2 = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		expect(new Uint8Array(e1.ephemeralPublicKey)).not.toEqual(
			new Uint8Array(e2.ephemeralPublicKey),
		);
		expect(new Uint8Array(e1.encryptedData)).not.toEqual(
			new Uint8Array(e2.encryptedData),
		);
	});

	it("wrong private key causes decryptResponse to throw", async () => {
		const encrypted = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		// Generate a completely different keypair
		const wrongKeypair = (await crypto.subtle.generateKey(
			{ name: "X25519" },
			true,
			["deriveKey", "deriveBits"],
		)) as CryptoKeyPair;
		await expect(
			decryptResponse(
				encrypted.encryptedData,
				encrypted.ephemeralPublicKey,
				wrongKeypair.privateKey,
			),
		).rejects.toThrow();
	});

	it("tampered ciphertext causes decryptResponse to throw", async () => {
		const encrypted = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		const tampered = new Uint8Array(encrypted.encryptedData);
		tampered[20] ^= 0xff;
		await expect(
			decryptResponse(
				tampered.buffer,
				encrypted.ephemeralPublicKey,
				keypair.privateKey,
			),
		).rejects.toThrow();
	});
});

// ---------------------------------------------------------------------------
// 8. Integration: full key hierarchy
// ---------------------------------------------------------------------------

describe("Integration: full key hierarchy", () => {
	it("masterKey → formKey → encryptSchema/decryptSchema", async () => {
		const masterKey = await generateMasterKey();
		const formKey = await deriveFormKey(masterKey, TEST_FORM_ID);
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey);
		const recovered = await decryptSchema(blob, formKey);
		expect(recovered).toEqual(SAMPLE_SCHEMA);
	});

	it("masterKey → formKey → formKeypair → encryptResponse/decryptResponse", async () => {
		const masterKey = await generateMasterKey();
		const formKey = await deriveFormKey(masterKey, TEST_FORM_ID);
		const keypair = await deriveFormKeypair(formKey);
		const encrypted = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		const recovered = await decryptResponse(
			encrypted.encryptedData,
			encrypted.ephemeralPublicKey,
			keypair.privateKey,
		);
		expect(recovered).toEqual(SAMPLE_RESPONSE);
	});

	it("masterKey → wrap → unwrap → formKey → formKeypair full chain", async () => {
		const masterKey = await generateMasterKey();
		const kek = await deriveRecoveryKey(["INTG-TEST-AAAA", "INTG-TEST-BBBB"]);

		// Wrap and unwrap master key
		const wrapped = await wrapKey(masterKey, kek);
		const restoredMasterKey = await unwrapKey(wrapped, kek);

		// Derive form key from restored master key
		const formKey = await deriveFormKey(restoredMasterKey, TEST_FORM_ID);
		const keypair = await deriveFormKeypair(formKey);

		// Full encrypt/decrypt cycle
		const encrypted = await encryptResponse(SAMPLE_RESPONSE, keypair.publicKey);
		const recovered = await decryptResponse(
			encrypted.encryptedData,
			encrypted.ephemeralPublicKey,
			keypair.privateKey,
		);
		expect(recovered).toEqual(SAMPLE_RESPONSE);
	});

	it("schema key hierarchy: derive determinism across formKey re-derivation", async () => {
		const masterKey = await generateMasterKey();
		const formKey1 = await deriveFormKey(masterKey, TEST_FORM_ID);
		const blob = await encryptSchema(SAMPLE_SCHEMA, formKey1);

		// Re-derive formKey from same master key
		const formKey2 = await deriveFormKey(masterKey, TEST_FORM_ID);
		const recovered = await decryptSchema(blob, formKey2);
		expect(recovered).toEqual(SAMPLE_SCHEMA);
	});
});

// ---------------------------------------------------------------------------
// 9. HKDF RFC 5869 Test Case 1 — byte-exact known-answer test
// ---------------------------------------------------------------------------

describe("HKDF RFC 5869 Test Case 1", () => {
	/**
	 * RFC 5869 Test Case 1:
	 * Hash = SHA-256
	 * IKM  = 0x0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b (22 octets)
	 * salt = 0x000102030405060708090a0b0c (13 octets)
	 * info = 0xf0f1f2f3f4f5f6f7f8f9 (10 octets)
	 * L    = 42 octets
	 *
	 * Expected OKM:
	 * 3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5b
	 * f34007208d5b887185865
	 */
	it("produces byte-exact OKM for RFC 5869 Test Case 1", async () => {
		const ikm = new Uint8Array([
			0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,
			0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,
		]);
		const salt = new Uint8Array([
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b,
			0x0c,
		]);
		const info = new Uint8Array([
			0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9,
		]);

		const expectedOkm = new Uint8Array([
			0x3c, 0xb2, 0x5f, 0x25, 0xfa, 0xac, 0xd5, 0x7a, 0x90, 0x43, 0x4f, 0x64,
			0xd0, 0x36, 0x2f, 0x2a, 0x2d, 0x2d, 0x0a, 0x90, 0xcf, 0x1a, 0x5a, 0x4c,
			0x5d, 0xb0, 0x2d, 0x56, 0xec, 0xc4, 0xc5, 0xbf, 0x34, 0x00, 0x72, 0x08,
			0xd5, 0xb8, 0x87, 0x18, 0x58, 0x65,
		]);

		// Import IKM as HKDF key
		const hkdfKey = await crypto.subtle.importKey("raw", ikm, "HKDF", false, [
			"deriveBits",
		]);

		// Derive bits using exact RFC 5869 parameters
		const okm = await crypto.subtle.deriveBits(
			{ name: "HKDF", hash: "SHA-256", salt, info },
			hkdfKey,
			42 * 8, // 42 bytes = 336 bits
		);

		expect(new Uint8Array(okm)).toEqual(expectedOkm);
	});
});

// ---------------------------------------------------------------------------
// 10. generateRecoveryCode (internal helper)
// ---------------------------------------------------------------------------

describe("generateRecoveryCode", () => {
	it("produces GHRK-XXXX-...-XXXX format with 12 segments", () => {
		const code = generateRecoveryCode();
		expect(code).toMatch(/^GHRK(-[A-Z0-9]{4}){12}$/);
	});

	it("generates unique codes", () => {
		const codes = new Set(
			Array.from({ length: 100 }, () => generateRecoveryCode()),
		);
		expect(codes.size).toBe(100);
	});

	it("only uses valid charset characters", () => {
		for (let i = 0; i < 20; i++) {
			const code = generateRecoveryCode();
			const stripped = code.replaceAll("-", "");
			for (const char of stripped) {
				expect("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789").toContain(char);
			}
		}
	});
});
