/**
 * Confide — WebAuthn ceremony orchestration.
 *
 * Bridges @simplewebauthn/browser ceremonies with the Confide Go API and the
 * crypto module. All base64 conversions are handled here so callers deal only
 * with typed values.
 *
 * Encoding conventions:
 *   - The Go backend uses base64.StdEncoding (standard with padding) for binary fields.
 *   - @simplewebauthn/browser uses Base64URLString (URL-safe, no padding) for credential IDs.
 *   - PRF outputs from simplewebauthn are Base64URLString.
 */

import {
	startRegistration,
	startAuthentication
} from '@simplewebauthn/browser';
import type {
	RegistrationResponseJSON,
	AuthenticationResponseJSON
} from '@simplewebauthn/browser';

import {
	wrapKey,
	unwrapKey,
	deriveRecoveryKey,
	hashForVerification,
	generateRecoveryCode,
	parseRecoveryCode
} from '$lib/crypto';

import type {
	RegisterBeginResponse,
	LoginBeginResponse,
	LoginFinishResponse,
	ReauthBeginResponse,
	ReauthFinishResponse,
	RecoverResponse,
	RekeyBeginResponse,
	RekeyFinishResponse,
	AddCredentialBeginResponse,
	AddCredentialFinishResponse,
	CredentialSummary
} from '$lib/types/auth';

// ─── Base64 Helpers ───────────────────────────────────────────────────────────

/** base64url string → Uint8Array. */
export function base64urlToBytes(b64url: string): Uint8Array {
	const pad = (4 - (b64url.length % 4)) % 4;
	const b64 = b64url.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat(pad);
	return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
}

/** Uint8Array → base64 standard string. */
export function bytesToBase64(bytes: Uint8Array): string {
	return btoa(String.fromCharCode(...bytes));
}

/** base64 standard string → Uint8Array. */
export function base64ToBytes(b64: string): Uint8Array {
	return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
}

/** ArrayBuffer → base64 standard string. */
function bufToBase64(buf: ArrayBuffer): string {
	return bytesToBase64(new Uint8Array(buf));
}

// ─── PRF Key Derivation ───────────────────────────────────────────────────────

/**
 * WebAuthn PRF extension output shape.
 * `first` may be a Base64URLString (older simplewebauthn) or an ArrayBuffer
 * (v13+ passes through the browser's raw type).
 */
interface PRFExtensionResults {
	prf?: {
		enabled?: boolean;
		results?: {
			first?: string | ArrayBuffer | ArrayBufferView;
		};
	};
}

/**
 * Import 32 bytes of PRF output as an AES-KW key (the key-encryption key).
 * The PRF output is already pseudorandom; it is used directly as key material.
 */
async function prfToKek(prfBytes: ArrayBuffer): Promise<CryptoKey> {
	return crypto.subtle.importKey('raw', prfBytes, { name: 'AES-KW' }, false, [
		'wrapKey',
		'unwrapKey'
	]);
}

/**
 * Normalise a PRF output value (string | ArrayBuffer | ArrayBufferView) to ArrayBuffer.
 * simplewebauthn v13 passes the browser's raw ArrayBuffer; older versions used base64url strings.
 */
function prfOutputToBuffer(value: string | ArrayBuffer | ArrayBufferView): ArrayBuffer {
	if (typeof value === 'string') {
		return base64urlToBytes(value).buffer as ArrayBuffer;
	}
	if (value instanceof ArrayBuffer) {
		return value;
	}
	// ArrayBufferView (Uint8Array etc.)
	return value.buffer as ArrayBuffer;
}

/** Extract PRF output from a registration response and import as AES-KW KEK. */
async function extractRegistrationKek(credential: RegistrationResponseJSON): Promise<CryptoKey> {
	const exts = credential.clientExtensionResults as PRFExtensionResults;
	const first = exts?.prf?.results?.first;
	if (first == null) {
		throw new Error(
			'PRF output absent. Your browser or authenticator does not support WebAuthn PRF. ' +
				'Please use Chrome/Edge 116+, Safari 17+, or Firefox 119+ with a compatible authenticator.'
		);
	}
	return prfToKek(prfOutputToBuffer(first));
}

/** Extract PRF output from an authentication response and import as AES-KW KEK. */
async function extractAuthenticationKek(credential: AuthenticationResponseJSON): Promise<CryptoKey> {
	const exts = credential.clientExtensionResults as PRFExtensionResults;
	const first = exts?.prf?.results?.first;
	if (first == null) {
		throw new Error(
			'PRF output absent. Your browser or authenticator does not support WebAuthn PRF.'
		);
	}
	return prfToKek(prfOutputToBuffer(first));
}

/**
 * The go-webauthn library wraps credential options under a `publicKey` key:
 *   { publicKey: { challenge, rp, user, ... } }
 * simplewebauthn's startRegistration/startAuthentication expect the inner object.
 */
function unwrapPublicKey<T>(options: unknown): T {
	return (options as { publicKey: T }).publicKey;
}

// ─── API Helpers ──────────────────────────────────────────────────────────────

async function apiPost<T>(path: string, body: unknown): Promise<T> {
	const res = await fetch(path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify(body)
	});
	const data = await res.json();
	if (!res.ok) {
		throw new Error((data as { message?: string }).message ?? `HTTP ${res.status}`);
	}
	return data as T;
}

async function apiGet<T>(path: string): Promise<T> {
	const res = await fetch(path, { credentials: 'include' });
	const data = await res.json();
	if (!res.ok) {
		throw new Error((data as { message?: string }).message ?? `HTTP ${res.status}`);
	}
	return data as T;
}

async function apiDelete(path: string): Promise<void> {
	const res = await fetch(path, { method: 'DELETE', credentials: 'include' });
	if (!res.ok && res.status !== 204) {
		const data = await res.json().catch(() => ({}));
		throw new Error((data as { message?: string }).message ?? `HTTP ${res.status}`);
	}
}

// ─── Registration ─────────────────────────────────────────────────────────────

export interface RegisterResult {
	masterKey: CryptoKey;
	accountId: string;
	credentialId: string; // Base64URLString
	recoveryCode: string; // single GHRK-XXXX-...-XXXX string
}

/**
 * Complete the full registration ceremony.
 *
 * Steps:
 *   1. POST /api/auth/register/begin → accountId + PRF salt + WebAuthn options
 *   2. startRegistration → WebAuthn credential with PRF output
 *   3. PRF → KEK; generate master key; wrap with KEK
 *   4. Generate 12 recovery codes; derive recovery key; wrap master key
 *   5. POST /api/auth/register/finish with all blobs
 */
export async function register(username: string): Promise<RegisterResult> {
	// Step 1: begin
	const begin = await apiPost<RegisterBeginResponse>('/api/auth/register/begin', { username });

	// Step 2: WebAuthn ceremony — convert PRF salt string → ArrayBuffer
	const optionsJSON = unwrapPublicKey<Parameters<typeof startRegistration>[0]['optionsJSON']>(begin.options);
	const prf = (optionsJSON.extensions as { prf?: { eval?: { first?: unknown } } })?.prf;
	if (prf?.eval?.first && typeof prf.eval.first === 'string') {
		(prf.eval as { first: ArrayBuffer }).first = base64urlToBytes(prf.eval.first).buffer as ArrayBuffer;
	}
	const credential = await startRegistration({ optionsJSON });

	// Step 3: PRF → KEK → wrap new master key
	const kek = await extractRegistrationKek(credential);
	const masterKey = await crypto.subtle.generateKey(
		{ name: 'AES-GCM', length: 256 },
		true,
		['encrypt', 'decrypt', 'wrapKey', 'unwrapKey']
	);
	const wrappedMasterKey = await wrapKey(masterKey, kek);

	// Step 4: generate recovery code + wrap master key with recovery key
	const recoveryCode = generateRecoveryCode(); // GHRK-XXXX-...-XXXX
	const segments = parseRecoveryCode(recoveryCode); // 12 key segments
	const recoveryKey = await deriveRecoveryKey(segments);
	const recoveryWrappedMasterKey = await wrapKey(masterKey, recoveryKey);

	// Compute recovery verifier: SHA-256 of all segments joined
	const enc = new TextEncoder();
	const recoveryVerifier = await hashForVerification(enc.encode(segments.join('')));

	// Compute per-segment hashes for server storage (one per segment)
	const codeHashes = await Promise.all(
		segments.map((seg) => hashForVerification(enc.encode(seg)))
	);

	// Step 5: finish registration
	const finish = await apiPost<{ accountId: string }>('/api/auth/register/finish', {
		accountId: begin.accountId,
		username,
		prfSalt: begin.prfSalt,
		wrappedMasterKey: bufToBase64(wrappedMasterKey),
		recoveryWrappedMasterKey: bufToBase64(recoveryWrappedMasterKey),
		recoveryVerifier: bufToBase64(recoveryVerifier),
		recoveryCodes: codeHashes.map((h) => bufToBase64(h)),
		credential: JSON.parse(JSON.stringify(credential))
	});

	return {
		masterKey,
		accountId: finish.accountId,
		credentialId: credential.id,
		recoveryCode
	};
}

// ─── Login ────────────────────────────────────────────────────────────────────

export interface LoginResult {
	masterKey: CryptoKey;
	accountId: string;
	credentialId: string; // Base64URLString from assertion (use to repopulate localStorage)
}

/**
 * Complete the login ceremony.
 *
 * credentialId is optional:
 *   - Provided (stored in localStorage): targeted mode using prf.eval.first (Chrome 116+).
 *   - Absent (new device / password manager): discoverable mode using prf.evalByCredential
 *     (Chrome 132+ / 1Password). The server embeds all registered credentials' PRF salts.
 *
 * Always returns credentialId from the assertion so the caller can repopulate localStorage.
 */
export async function login(credentialId?: string | null, username?: string): Promise<LoginResult> {
	// Step 1: begin — prefer username (server looks up correct PRF salt); fall back to credentialId
	let body: Record<string, string> = {};
	if (username) {
		body = { username };
	} else if (credentialId) {
		body = { credentialIdBase64: bytesToBase64(base64urlToBytes(credentialId)) };
	}
	const begin = await apiPost<LoginBeginResponse>('/api/auth/login/begin', body);

	// Step 2: WebAuthn ceremony — convert PRF salt strings → ArrayBuffer
	const optionsJSON = unwrapPublicKey<Parameters<typeof startAuthentication>[0]['optionsJSON']>(begin.options);
	const prf = (optionsJSON.extensions as { prf?: { eval?: { first?: unknown }; evalByCredential?: Record<string, { first?: unknown }> } })?.prf;
	if (prf?.eval?.first && typeof prf.eval.first === 'string') {
		// Targeted mode: convert eval.first
		(prf.eval as { first: ArrayBuffer }).first = base64urlToBytes(prf.eval.first).buffer as ArrayBuffer;
	} else if (prf?.evalByCredential) {
		// Discoverable mode: convert each entry in evalByCredential
		for (const entry of Object.values(prf.evalByCredential)) {
			if (entry?.first && typeof entry.first === 'string') {
				(entry as { first: ArrayBuffer }).first = base64urlToBytes(entry.first as string).buffer as ArrayBuffer;
			}
		}
	}
	const credential = await startAuthentication({ optionsJSON });

	// Step 3: finish — challengeKey replaces credentialIdBase64
	const finish = await apiPost<LoginFinishResponse>('/api/auth/login/finish', {
		challengeKey: begin.challengeKey,
		credential: JSON.parse(JSON.stringify(credential))
	});

	// Step 4: PRF → KEK; unwrap master key
	const kek = await extractAuthenticationKek(credential);
	const masterKey = await unwrapKey(base64ToBytes(finish.wrappedMasterKey).buffer as ArrayBuffer, kek);

	return { masterKey, accountId: finish.accountId, credentialId: credential.id };
}

/**
 * Re-derive the master key for an existing valid session after a page refresh.
 *
 * Runs a WebAuthn assertion ceremony against /api/auth/reauth/* (authenticated
 * endpoints) to get PRF output, then unwraps the master key locally.
 * No new server session is created — the existing session cookie is reused.
 */
export async function reauthenticate(): Promise<LoginResult> {
	// Step 1: begin — server reads accountID from session cookie, no body needed
	const begin = await apiPost<ReauthBeginResponse>('/api/auth/reauth/begin', {});

	// Step 2: WebAuthn ceremony — convert PRF salt string → ArrayBuffer
	const optionsJSON = unwrapPublicKey<Parameters<typeof startAuthentication>[0]['optionsJSON']>(begin.options);
	const prf = (optionsJSON.extensions as { prf?: { eval?: { first?: unknown } } })?.prf;
	if (prf?.eval?.first && typeof prf.eval.first === 'string') {
		(prf.eval as { first: ArrayBuffer }).first = base64urlToBytes(prf.eval.first).buffer as ArrayBuffer;
	}
	const credential = await startAuthentication({ optionsJSON });

	// Step 3: finish — verify assertion server-side, return wrappedMasterKey
	const finish = await apiPost<ReauthFinishResponse>('/api/auth/reauth/finish', {
		challengeKey: begin.challengeKey,
		credential: JSON.parse(JSON.stringify(credential))
	});

	// Step 4: PRF → KEK; unwrap master key
	const kek = await extractAuthenticationKek(credential);
	const masterKey = await unwrapKey(base64ToBytes(finish.wrappedMasterKey).buffer as ArrayBuffer, kek);

	return { masterKey, accountId: finish.accountId, credentialId: credential.id };
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

export interface RecoverResult {
	masterKey: CryptoKey;
	accountId: string;
	rekeyToken: string;
}

/**
 * Recover a master key using username + recovery code.
 *
 * Parses the GHRK-XXXX-...-XXXX string into 12 segments.
 * Sends the first segment to the server (which burns it).
 * Derives the recovery key from all 12 segments locally and unwraps.
 */
export async function recover(username: string, recoveryCode: string): Promise<RecoverResult> {
	const segments = parseRecoveryCode(recoveryCode);

	const res = await apiPost<RecoverResponse>('/api/auth/recover', {
		username,
		code: segments[0] // first segment is burned server-side
	});

	const recoveryKey = await deriveRecoveryKey(segments);
	const masterKey = await unwrapKey(
		base64ToBytes(res.recoveryWrappedMasterKey).buffer as ArrayBuffer,
		recoveryKey
	);

	return { masterKey, accountId: res.accountId, rekeyToken: res.rekeyToken };
}

// ─── Rekey ────────────────────────────────────────────────────────────────────

export interface RekeyResult {
	credentialId: string; // new Base64URLString credential ID
}

/**
 * Register a new WebAuthn credential after recovery.
 *
 * Requires a valid rekeyToken (issued by /auth/recover).
 * Generates a fresh PRF salt and wraps the existing master key under the new credential.
 */
export async function rekey(masterKey: CryptoKey, rekeyToken: string): Promise<RekeyResult> {
	// Step 1: begin rekey registration
	const begin = await apiPost<RekeyBeginResponse>('/api/auth/recover/rekey/begin', { rekeyToken });

	// Step 2: WebAuthn ceremony — convert PRF salt string → ArrayBuffer
	const rekeyOptionsJSON = unwrapPublicKey<Parameters<typeof startRegistration>[0]['optionsJSON']>(begin.options);
	const rekeyPrf = (rekeyOptionsJSON.extensions as { prf?: { eval?: { first?: unknown } } })?.prf;
	if (rekeyPrf?.eval?.first && typeof rekeyPrf.eval.first === 'string') {
		(rekeyPrf.eval as { first: ArrayBuffer }).first = base64urlToBytes(rekeyPrf.eval.first).buffer as ArrayBuffer;
	}
	const credential = await startRegistration({ optionsJSON: rekeyOptionsJSON });

	// Step 3: PRF → KEK; wrap master key with new credential's PRF
	const kek = await extractRegistrationKek(credential);
	const wrappedMasterKey = await wrapKey(masterKey, kek);

	// Generate fresh recovery code + wrap master key
	const recoveryCode = generateRecoveryCode();
	const segments = parseRecoveryCode(recoveryCode);
	const recoveryKey = await deriveRecoveryKey(segments);
	const recoveryWrappedMasterKey = await wrapKey(masterKey, recoveryKey);

	const enc = new TextEncoder();
	const recoveryVerifier = await hashForVerification(enc.encode(segments.join('')));
	const codeHashes = await Promise.all(
		segments.map((seg) => hashForVerification(enc.encode(seg)))
	);

	// Use the PRF salt the server embedded in the registration options (must match).
	const prfSalt = begin.prfSalt;

	// Step 4: finish rekey
	await apiPost<RekeyFinishResponse>('/api/auth/recover/rekey/finish', {
		rekeyToken,
		prfSalt,
		wrappedMasterKey: bufToBase64(wrappedMasterKey),
		recoveryWrappedMasterKey: bufToBase64(recoveryWrappedMasterKey),
		recoveryVerifier: bufToBase64(recoveryVerifier),
		recoveryCodes: codeHashes.map((h) => bufToBase64(h)),
		credential: JSON.parse(JSON.stringify(credential))
	});

	return { credentialId: credential.id };
}

// ─── Session Management ───────────────────────────────────────────────────────

export interface SessionInfo {
	id: string;
	createdAt: string;
	lastSeen: string;
}

export async function listSessions(): Promise<SessionInfo[]> {
	return apiGet<SessionInfo[]>('/api/auth/sessions');
}

export async function revokeSession(sessionId: string): Promise<void> {
	return apiDelete(`/api/auth/sessions/${sessionId}`);
}

export async function logout(): Promise<void> {
	await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' });
}

export async function getMe(): Promise<{ accountId: string; username?: string }> {
	return apiGet<{ accountId: string; username?: string }>('/api/auth/me');
}

// ─── Reauth for Add Credential ────────────────────────────────────────────────

/**
 * Re-authenticate with an existing passkey to obtain a short-lived token
 * authorizing a new passkey registration.
 *
 * Returns the addCredentialToken to pass to addCredential().
 */
export async function reauthenticateForAddCredential(): Promise<string> {
	const begin = await apiPost<ReauthBeginResponse>('/api/auth/reauth/begin', {});

	const optionsJSON = unwrapPublicKey<Parameters<typeof startAuthentication>[0]['optionsJSON']>(begin.options);
	const prf = (optionsJSON.extensions as { prf?: { eval?: { first?: unknown } } })?.prf;
	if (prf?.eval?.first && typeof prf.eval.first === 'string') {
		(prf.eval as { first: ArrayBuffer }).first = base64urlToBytes(prf.eval.first).buffer as ArrayBuffer;
	}
	const credential = await startAuthentication({ optionsJSON });

	const finish = await apiPost<ReauthFinishResponse>('/api/auth/reauth/finish', {
		challengeKey: begin.challengeKey,
		credential: JSON.parse(JSON.stringify(credential)),
		purpose: 'add-credential'
	});

	if (!finish.addCredentialToken) {
		throw new Error('Server did not return an add-credential token');
	}
	return finish.addCredentialToken;
}

// ─── Add Credential ───────────────────────────────────────────────────────────

/**
 * Register a new passkey for the current account.
 *
 * Requires a valid addCredentialToken from reauthenticateForAddCredential().
 * The masterKey is the currently unlocked key — it will be wrapped under the
 * new credential's PRF output.
 */
export async function addCredential(
	masterKey: CryptoKey,
	addCredentialToken: string,
	name: string
): Promise<AddCredentialFinishResponse> {
	// Step 1: begin — server generates a fresh PRF salt
	const begin = await apiPost<AddCredentialBeginResponse>('/api/auth/credentials/add/begin', {
		addCredentialToken
	});

	// Step 2: WebAuthn ceremony — convert PRF salt string → ArrayBuffer
	const optionsJSON = unwrapPublicKey<Parameters<typeof startRegistration>[0]['optionsJSON']>(begin.options);
	const prf = (optionsJSON.extensions as { prf?: { eval?: { first?: unknown } } })?.prf;
	if (prf?.eval?.first && typeof prf.eval.first === 'string') {
		(prf.eval as { first: ArrayBuffer }).first = base64urlToBytes(prf.eval.first).buffer as ArrayBuffer;
	}
	const credential = await startRegistration({ optionsJSON });

	// Step 3: PRF → KEK; wrap the existing master key under the new credential
	const kek = await extractRegistrationKek(credential);
	const wrappedMasterKey = await wrapKey(masterKey, kek);

	// Step 4: finish
	return apiPost<AddCredentialFinishResponse>('/api/auth/credentials/add/finish', {
		addCredentialToken,
		prfSalt: begin.prfSalt,
		wrappedMasterKey: bufToBase64(wrappedMasterKey),
		name,
		credential: JSON.parse(JSON.stringify(credential))
	});
}

// ─── Credential Management ────────────────────────────────────────────────────

export async function listCredentials(): Promise<CredentialSummary[]> {
	return apiGet<CredentialSummary[]>('/api/auth/credentials');
}

export async function renameCredential(id: string, name: string): Promise<void> {
	const res = await fetch(`/api/auth/credentials/${id}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ name })
	});
	if (!res.ok && res.status !== 204) {
		const data = await res.json().catch(() => ({}));
		throw new Error((data as { message?: string }).message ?? `HTTP ${res.status}`);
	}
}

export async function deleteCredential(id: string): Promise<void> {
	return apiDelete(`/api/auth/credentials/${id}`);
}

export async function deleteAccount(): Promise<void> {
	return apiDelete('/api/auth/account');
}

// ─── Recovery Code Rotation ───────────────────────────────────────────────────

/**
 * Generate a fresh recovery code and register it with the server.
 * The existing recovery codes are invalidated and replaced atomically.
 * Returns the new GHRK-XXXX-…-XXXX recovery code string for the caller to save.
 */
export async function rotateRecoveryCode(masterKey: CryptoKey): Promise<string> {
	const recoveryCode = generateRecoveryCode();
	const segments = parseRecoveryCode(recoveryCode);
	const recoveryKey = await deriveRecoveryKey(segments);
	const recoveryWrappedMasterKey = await wrapKey(masterKey, recoveryKey);

	const enc = new TextEncoder();
	const recoveryVerifier = await hashForVerification(enc.encode(segments.join('')));
	const codeHashes = await Promise.all(
		segments.map((seg) => hashForVerification(enc.encode(seg)))
	);

	await apiPost<Record<string, never>>('/api/auth/recovery-code/rotate', {
		recoveryWrappedMasterKey: bufToBase64(recoveryWrappedMasterKey),
		recoveryVerifier: bufToBase64(recoveryVerifier),
		recoveryCodes: codeHashes.map((h) => bufToBase64(h))
	});

	return recoveryCode;
}
