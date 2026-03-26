/**
 * Confide — Form API client (Phase 4)
 *
 * All authenticated functions require a valid session cookie (set by login/register).
 * Binary blobs are transmitted as base64 standard encoding, matching the Go backend.
 */

import {
	deriveFormKey,
	deriveRenderKey,
	deriveFormKeypair,
	encryptSchema,
	decryptSchema,
	encryptResponse,
	decryptResponse
} from './crypto';
import type { FormSchema, ResponsePayload } from './types/crypto';
import type { BuilderSchema } from './types/builder';

export type { FormSchema, ResponsePayload };

// ─── Types ────────────────────────────────────────────────────────────────────

export interface FormSummary {
	formId: string;
	status: 'open' | 'closed';
	schemaVersion: number;
	responseCount: number;
	createdAt: string;
	updatedAt: string;
}

export interface FormRecord extends FormSummary {
	encryptedSchema: string;
	renderEncryptedSchema: string;
	publicFormKey: string;
	renderKeySalt: string | null; // base64, null if never published
}

export interface EncryptedResponseRecord {
	id: string;
	receivedAt: string;
	schemaVersion: number;
	encryptedData: string;
	ephemeralPublicKey: string;
}

export interface ListResponsesResult {
	responses: EncryptedResponseRecord[];
	nextCursor?: string;
}

// ─── Form management (authenticated) ─────────────────────────────────────────

/**
 * Create a new form. Derives the formKey, generates a renderKeySalt,
 * derives a stable renderKey from it, encrypts the schema twice, and uploads everything.
 *
 * Returns the formId, derived renderKey, and renderKeySalt.
 * The renderKeySalt is stored on the server; the renderKey is embedded in share URLs.
 */
export async function createForm(
	masterKey: CryptoKey,
	schema: FormSchema
): Promise<{ formId: string; renderKey: CryptoKey; renderKeySalt: Uint8Array }> {
	// Generate a stable form ID client-side so we can derive the formKey.
	const formId = randomBase64url(16);

	const formKey = await deriveFormKey(masterKey, formId);
	const keypair = await deriveFormKeypair(formKey);

	// Encrypt schema for the owner (with formKey).
	const encryptedSchema = await encryptSchema(schema, formKey);

	// Generate a random salt and derive a stable renderKey from it.
	const renderKeySalt = crypto.getRandomValues(new Uint8Array(16));
	const renderKey = await deriveRenderKey(formKey, renderKeySalt.buffer as ArrayBuffer);
	const renderEncryptedSchema = await encryptSchema(schema, renderKey);

	// Export public key as raw bytes (32 bytes for X25519).
	const publicFormKeyBytes = await crypto.subtle.exportKey('raw', keypair.publicKey);

	const res = await fetch('/api/forms', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			formId,
			encryptedSchema: arrayBufferToBase64(encryptedSchema),
			renderEncryptedSchema: arrayBufferToBase64(renderEncryptedSchema),
			publicFormKey: arrayBufferToBase64(publicFormKeyBytes),
			renderKeySalt: arrayBufferToBase64(renderKeySalt.buffer as ArrayBuffer)
		})
	});

	if (!res.ok) throw new ApiError(res.status, await res.json());

	return { formId, renderKey, renderKeySalt };
}

/**
 * Fetch and decrypt a form's schema. Only the owner can call this.
 */
export async function getForm(
	masterKey: CryptoKey,
	formId: string
): Promise<{ schema: FormSchema; record: FormRecord }> {
	const res = await fetch(`/api/forms/${formId}`);
	if (!res.ok) throw new ApiError(res.status, await res.json());

	const record: FormRecord = await res.json();
	const formKey = await deriveFormKey(masterKey, formId);
	const schema = await decryptSchema(base64ToArrayBuffer(record.encryptedSchema), formKey);

	return { schema, record };
}

/**
 * List all forms for the authenticated account (no schema decryption).
 */
export async function listForms(): Promise<FormSummary[]> {
	const res = await fetch('/api/forms');
	if (!res.ok) throw new ApiError(res.status, await res.json());
	const body = await res.json();
	return body.forms ?? [];
}

/**
 * Replace a form's encrypted schema. Bumps schema_version server-side.
 * The renderKeySalt must be the currently active one — the renderKey is derived from it.
 * Pass the same salt on every edit to keep share URLs stable; pass a new salt to rotate.
 */
export async function updateFormSchema(
	masterKey: CryptoKey,
	formId: string,
	schema: FormSchema,
	renderKeySalt: Uint8Array
): Promise<{ schemaVersion: number }> {
	const formKey = await deriveFormKey(masterKey, formId);
	const encryptedSchema = await encryptSchema(schema, formKey);
	const renderKey = await deriveRenderKey(formKey, renderKeySalt.buffer as ArrayBuffer);
	const renderEncryptedSchema = await encryptSchema(schema, renderKey);

	const res = await fetch(`/api/forms/${formId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			encryptedSchema: arrayBufferToBase64(encryptedSchema),
			renderEncryptedSchema: arrayBufferToBase64(renderEncryptedSchema),
			renderKeySalt: arrayBufferToBase64(renderKeySalt.buffer as ArrayBuffer)
		})
	});

	if (!res.ok) throw new ApiError(res.status, await res.json());
	return res.json();
}

/**
 * Toggle a form open or closed.
 */
export async function updateFormStatus(formId: string, status: 'open' | 'closed'): Promise<void> {
	const res = await fetch(`/api/forms/${formId}/status`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ status })
	});
	if (!res.ok) throw new ApiError(res.status, await res.json());
}

/**
 * Hard-delete a form and all its responses.
 */
export async function deleteForm(formId: string): Promise<void> {
	const res = await fetch(`/api/forms/${formId}`, { method: 'DELETE' });
	if (!res.ok) throw new ApiError(res.status, await res.json());
}

// ─── Response management (authenticated) ──────────────────────────────────────

/**
 * Paginated list of encrypted response records. Decrypt each with decryptResponseRecord.
 */
export async function listResponses(
	formId: string,
	cursor?: string,
	limit = 50
): Promise<ListResponsesResult> {
	const params = new URLSearchParams({ limit: String(limit) });
	if (cursor) params.set('after', cursor);

	const res = await fetch(`/api/forms/${formId}/responses?${params}`);
	if (!res.ok) throw new ApiError(res.status, await res.json());
	return res.json();
}

/**
 * Fetch a single encrypted response.
 */
export async function getResponseRecord(
	formId: string,
	responseId: string
): Promise<EncryptedResponseRecord> {
	const res = await fetch(`/api/forms/${formId}/responses/${responseId}`);
	if (!res.ok) throw new ApiError(res.status, await res.json());
	return res.json();
}

/**
 * Decrypt an encrypted response record using the form's X25519 private key.
 */
export async function decryptResponseRecord(
	masterKey: CryptoKey,
	formId: string,
	record: EncryptedResponseRecord
): Promise<ResponsePayload> {
	const formKey = await deriveFormKey(masterKey, formId);
	const keypair = await deriveFormKeypair(formKey);

	return decryptResponse(
		base64ToArrayBuffer(record.encryptedData),
		base64ToArrayBuffer(record.ephemeralPublicKey),
		keypair.privateKey
	);
}

/**
 * Hard-delete a single response.
 */
export async function deleteResponse(formId: string, responseId: string): Promise<void> {
	const res = await fetch(`/api/forms/${formId}/responses/${responseId}`, { method: 'DELETE' });
	if (!res.ok) throw new ApiError(res.status, await res.json());
}

// ─── Public (unauthenticated) ─────────────────────────────────────────────────

/**
 * Fetch and decrypt the form schema using the renderKey from the URL fragment.
 * The renderKey is never sent to the server — only used locally.
 */
export async function getPublicSchema(
	formId: string,
	renderKey: CryptoKey
): Promise<{ schema: FormSchema; status: string; schemaVersion: number; publicFormKey: ArrayBuffer }> {
	const res = await fetch(`/api/f/${formId}/schema`, { credentials: 'omit' });
	if (!res.ok) throw new ApiError(res.status, await res.json());

	const body = await res.json();
	const schema = await decryptSchema(base64ToArrayBuffer(body.renderEncryptedSchema), renderKey);
	const publicFormKey = base64ToArrayBuffer(body.publicFormKey);

	return { schema, status: body.status, schemaVersion: body.schemaVersion, publicFormKey };
}

/**
 * Submit a response anonymously via the relay endpoint.
 * No cookies are sent. Retries 3× with exponential backoff on failure.
 */
export async function submitResponse(
	formId: string,
	publicFormKeyBytes: ArrayBuffer,
	payload: ResponsePayload,
	schemaVersion: number
): Promise<void> {
	const publicFormKey = await crypto.subtle.importKey(
		'raw',
		publicFormKeyBytes,
		{ name: 'X25519' },
		false,
		[]
	);

	const { encryptedData, ephemeralPublicKey } = await encryptResponse(payload, publicFormKey);

	const body = JSON.stringify({
		formId,
		encryptedData: arrayBufferToBase64(encryptedData),
		ephemeralPublicKey: arrayBufferToBase64(ephemeralPublicKey),
		schemaVersion
	});

	for (let attempt = 0; attempt < 3; attempt++) {
		if (attempt > 0) await sleep(1000 * 2 ** (attempt - 1)); // 1s, 2s
		const res = await fetch('/relay/submit', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'omit',
			body
		});
		if (res.ok) return;
		if (attempt === 2) throw new Error(`Submission failed after 3 attempts (${res.status})`);
	}
}

/**
 * Publish a form using the existing renderKeySalt (stable URL) or a new one (first publish).
 *
 * Pass existingRenderKeySalt=null on first publish to generate a new salt.
 * Pass the existing salt on subsequent publishes — the share URL will be identical.
 *
 * Returns the share URL and the renderKeySalt (store it so future publishes stay stable).
 */
export async function publishForm(
	masterKey: CryptoKey,
	formId: string,
	schema: BuilderSchema,
	existingRenderKeySalt: Uint8Array | null
): Promise<{ shareUrl: string; renderKeySalt: Uint8Array }> {
	const salt = existingRenderKeySalt ?? crypto.getRandomValues(new Uint8Array(16));
	const formKey = await deriveFormKey(masterKey, formId);

	const encryptedSchema = await encryptSchema(schema as FormSchema, formKey);
	const renderKey = await deriveRenderKey(formKey, salt.buffer as ArrayBuffer);
	const renderEncryptedSchema = await encryptSchema(schema as FormSchema, renderKey);

	const res = await fetch(`/api/forms/${formId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			encryptedSchema: arrayBufferToBase64(encryptedSchema),
			renderEncryptedSchema: arrayBufferToBase64(renderEncryptedSchema),
			renderKeySalt: arrayBufferToBase64(salt.buffer as ArrayBuffer)
		})
	});

	if (!res.ok) throw new ApiError(res.status, await res.json());

	const renderKeyRaw = await crypto.subtle.exportKey('raw', renderKey);
	const shareUrl = `${window.location.origin}/f/${formId}#rk=${arrayBufferToBase64url(renderKeyRaw)}`;

	return { shareUrl, renderKeySalt: salt };
}

/**
 * Rotate the render key — generates a new salt and a new share URL.
 * All previously shared links are immediately invalidated.
 */
export async function rotateRenderKey(
	masterKey: CryptoKey,
	formId: string,
	schema: BuilderSchema
): Promise<{ shareUrl: string; renderKeySalt: Uint8Array }> {
	return publishForm(masterKey, formId, schema, null);
}

/**
 * Fetch a specific versioned schema snapshot and decrypt it with formKey.
 * Used by the response viewer (Phase 6).
 */
export async function getSchemaVersion(
	masterKey: CryptoKey,
	formId: string,
	version: number
): Promise<BuilderSchema> {
	const res = await fetch(`/api/forms/${formId}/schema-versions/${version}`);
	if (!res.ok) throw new ApiError(res.status, await res.json());

	const body = await res.json();
	const formKey = await deriveFormKey(masterKey, formId);
	const schema = await decryptSchema(base64ToArrayBuffer(body.encryptedSchema), formKey);
	return schema as BuilderSchema;
}

/**
 * Import a renderKey from raw base64url bytes (parsed from #rk=<base64url> fragment).
 */
export async function importRenderKey(base64url: string): Promise<CryptoKey> {
	const bytes = base64urlToArrayBuffer(base64url);
	return crypto.subtle.importKey('raw', bytes, { name: 'AES-GCM', length: 256 }, false, [
		'decrypt'
	]);
}

/**
 * Export a renderKey to base64url for embedding in share URLs.
 */
export async function exportRenderKey(key: CryptoKey): Promise<string> {
	const raw = await crypto.subtle.exportKey('raw', key);
	return arrayBufferToBase64url(raw);
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

export class ApiError extends Error {
	constructor(
		public status: number,
		public body: unknown
	) {
		super(`API error ${status}`);
	}
}

function arrayBufferToBase64(buf: ArrayBuffer): string {
	const bytes = new Uint8Array(buf);
	let binary = '';
	for (const b of bytes) binary += String.fromCharCode(b);
	return btoa(binary);
}

function base64ToArrayBuffer(b64: string): ArrayBuffer {
	const binary = atob(b64);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
	return bytes.buffer;
}

function arrayBufferToBase64url(buf: ArrayBuffer): string {
	return arrayBufferToBase64(buf).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function base64urlToArrayBuffer(s: string): ArrayBuffer {
	const b64 = s.replace(/-/g, '+').replace(/_/g, '/');
	return base64ToArrayBuffer(b64);
}

function randomBase64url(bytes: number): string {
	const buf = crypto.getRandomValues(new Uint8Array(bytes));
	return arrayBufferToBase64url(buf.buffer);
}

function sleep(ms: number): Promise<void> {
	return new Promise((r) => setTimeout(r, ms));
}
