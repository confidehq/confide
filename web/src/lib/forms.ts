/**
 * Wisp — Form API client (Phase 4)
 *
 * All authenticated functions require a valid session cookie (set by login/register).
 * Binary blobs are transmitted as base64 standard encoding, matching the Go backend.
 */

import {
	deriveFormKey,
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
 * Create a new form. Derives the formKey, encrypts the schema twice
 * (once with formKey for the owner, once with a random renderKey for respondents),
 * derives the X25519 keypair, and uploads everything.
 *
 * Returns the formId and the renderKey (embed in share URL as #rk=<base64url>).
 */
export async function createForm(
	masterKey: CryptoKey,
	schema: FormSchema
): Promise<{ formId: string; renderKey: CryptoKey }> {
	// Generate a stable form ID client-side so we can derive the formKey.
	const formId = randomBase64url(16);

	const formKey = await deriveFormKey(masterKey, formId);
	const keypair = await deriveFormKeypair(formKey);

	// Encrypt schema for the owner (with formKey).
	const encryptedSchema = await encryptSchema(schema, formKey);

	// Generate a random renderKey and encrypt schema for respondents.
	const renderKey = await crypto.subtle.generateKey(
		{ name: 'AES-GCM', length: 256 },
		true,
		['encrypt', 'decrypt']
	);
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
			publicFormKey: arrayBufferToBase64(publicFormKeyBytes)
		})
	});

	if (!res.ok) throw new ApiError(res.status, await res.json());

	return { formId, renderKey };
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
 * The renderKey must be the currently active one (used when the form was published).
 */
export async function updateFormSchema(
	masterKey: CryptoKey,
	formId: string,
	schema: FormSchema,
	renderKey: CryptoKey
): Promise<{ schemaVersion: number }> {
	const formKey = await deriveFormKey(masterKey, formId);
	const encryptedSchema = await encryptSchema(schema, formKey);
	const renderEncryptedSchema = await encryptSchema(schema, renderKey);

	const res = await fetch(`/api/forms/${formId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			encryptedSchema: arrayBufferToBase64(encryptedSchema),
			renderEncryptedSchema: arrayBufferToBase64(renderEncryptedSchema)
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
): Promise<{ schema: FormSchema; status: string; schemaVersion: number }> {
	const res = await fetch(`/api/f/${formId}/schema`, { credentials: 'omit' });
	if (!res.ok) throw new ApiError(res.status, await res.json());

	const body = await res.json();
	const schema = await decryptSchema(base64ToArrayBuffer(body.renderEncryptedSchema), renderKey);

	return { schema, status: body.status, schemaVersion: body.schemaVersion };
}

/**
 * Submit a response anonymously via the relay endpoint.
 * No cookies are sent. Retries 3× with exponential backoff on failure.
 */
export async function submitResponse(
	formId: string,
	publicFormKeyBytes: ArrayBuffer,
	payload: ResponsePayload
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
		schemaVersion: 1
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
 * Publish (or re-publish) a form: generates a fresh renderKey, re-encrypts
 * the schema for respondents, and PUTs both encrypted schemas + publicFormKey.
 * Returns the share URL and the renderKey (embed in share URL as #rk=<base64url>).
 */
export async function publishForm(
	masterKey: CryptoKey,
	formId: string,
	schema: BuilderSchema
): Promise<{ shareUrl: string; renderKey: CryptoKey }> {
	const formKey = await deriveFormKey(masterKey, formId);

	// Encrypt schema for the owner.
	const encryptedSchema = await encryptSchema(schema as FormSchema, formKey);

	// Generate fresh renderKey and encrypt for respondents.
	const renderKey = await crypto.subtle.generateKey(
		{ name: 'AES-GCM', length: 256 },
		true,
		['encrypt', 'decrypt']
	);
	const renderEncryptedSchema = await encryptSchema(schema as FormSchema, renderKey);

	const res = await fetch(`/api/forms/${formId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			encryptedSchema: arrayBufferToBase64(encryptedSchema),
			renderEncryptedSchema: arrayBufferToBase64(renderEncryptedSchema)
		})
	});

	if (!res.ok) throw new ApiError(res.status, await res.json());

	// Export renderKey as base64url for the URL fragment.
	const renderKeyRaw = await crypto.subtle.exportKey('raw', renderKey);
	const renderKeyB64url = arrayBufferToBase64url(renderKeyRaw);
	const shareUrl = `${window.location.origin}/f/${formId}#rk=${renderKeyB64url}`;

	return { shareUrl, renderKey };
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
