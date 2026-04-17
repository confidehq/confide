import { generateAndWrapWorkspaceKey, unwrapIdentityPrivateKey, rewrapWorkspaceKey } from '$lib/crypto';

// In-memory cache: workspaceId → decrypted workspace CryptoKey
const workspaceKeyCache = new Map<string, CryptoKey>();

export interface Workspace {
	id: string;
	name: string;
	slug: string;
	plan: 'free' | 'pro';
	planStatus: 'active' | 'past_due' | 'canceled';
	role: 'owner' | 'admin' | 'member' | 'viewer';
}

export interface WorkspaceMember {
	accountId: string;
	username: string;
	role: 'owner' | 'admin' | 'member' | 'viewer';
	joinedAt: string;
	status: 'active' | 'pending';
	lastSeen: string; // ISO date string, empty if never logged in
}

export class WorkspaceError extends Error {
	constructor(
		public code: string,
		message: string
	) {
		super(message);
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function bytesToBase64(bytes: Uint8Array): string {
	return btoa(String.fromCharCode(...bytes));
}

function base64ToBytes(b64: string): Uint8Array {
	return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
}

// ─── Identity key ─────────────────────────────────────────────────────────────

/**
 * Return the caller's X25519 identity public key bytes, creating and persisting
 * the keypair (private key AES-GCM-encrypted with masterKey) if none exists yet.
 */
async function getOrCreateIdentityKey(masterKey: CryptoKey): Promise<ArrayBuffer> {
	// Try to load an existing key first
	const res = await fetch('/api/account/identity-key');
	if (res.ok) {
		const body = await res.json();
		return base64ToBytes(body.identityPublicKey).buffer as ArrayBuffer;
	}
	if (res.status !== 404) {
		throw new WorkspaceError('identity_key_fetch', `Failed to fetch identity key (${res.status})`);
	}

	// Generate a new X25519 identity keypair
	const keypair = (await crypto.subtle.generateKey(
		{ name: 'X25519' },
		true,
		['deriveKey', 'deriveBits']
	)) as CryptoKeyPair;

	const publicKeyBytes = new Uint8Array(
		await crypto.subtle.exportKey('raw', keypair.publicKey)
	);

	// Wrap private key: export as PKCS8, then AES-GCM encrypt with masterKey
	const privateKeyPkcs8 = await crypto.subtle.exportKey('pkcs8', keypair.privateKey);
	const iv = crypto.getRandomValues(new Uint8Array(12));
	const encrypted = await crypto.subtle.encrypt(
		{ name: 'AES-GCM', iv, tagLength: 128 },
		masterKey,
		privateKeyPkcs8
	);
	const wrapped = new Uint8Array(12 + encrypted.byteLength);
	wrapped.set(iv, 0);
	wrapped.set(new Uint8Array(encrypted), 12);

	const put = await fetch('/api/account/identity-key', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			identityPublicKey: bytesToBase64(publicKeyBytes),
			wrappedIdentityPrivateKey: bytesToBase64(wrapped)
		})
	});
	if (!put.ok) {
		throw new WorkspaceError('identity_key_save', `Failed to save identity key (${put.status})`);
	}

	return publicKeyBytes.buffer as ArrayBuffer;
}

// ─── API ─────────────────────────────────────────────────────────────────────

export async function renameWorkspace(id: string, name: string): Promise<void> {
	const res = await fetch(`/api/workspaces/${id}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
	if (!res.ok && res.status !== 204) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'rename_failed';
		const message = (body as { message?: string }).message ?? `Failed to rename workspace (${res.status})`;
		throw new WorkspaceError(code, message);
	}
}

export async function deleteWorkspace(id: string): Promise<void> {
	const res = await fetch(`/api/workspaces/${id}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'delete_failed';
		const message = (body as { message?: string }).message ?? `Failed to delete workspace (${res.status})`;
		throw new WorkspaceError(code, message);
	}
}

export async function listWorkspaces(): Promise<Workspace[]> {
	const res = await fetch('/api/workspaces');
	if (!res.ok) throw new WorkspaceError('list_failed', `Failed to load workspaces (${res.status})`);
	const body = await res.json();
	return body.workspaces ?? [];
}

export async function listMembers(workspaceId: string): Promise<WorkspaceMember[]> {
	const res = await fetch(`/api/workspaces/${workspaceId}/members`);
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'list_failed';
		const message = (body as { message?: string }).message ?? `Failed to load members (${res.status})`;
		throw new WorkspaceError(code, message);
	}
	const body = await res.json();
	return body.members ?? [];
}

export async function updateMemberRole(
	workspaceId: string,
	accountId: string,
	role: string
): Promise<void> {
	const res = await fetch(`/api/workspaces/${workspaceId}/members/${accountId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ role })
	});
	if (!res.ok && res.status !== 204) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'update_failed';
		const message = (body as { message?: string }).message ?? `Failed to update role (${res.status})`;
		throw new WorkspaceError(code, message);
	}
}

export async function removeMember(workspaceId: string, accountId: string): Promise<void> {
	const res = await fetch(`/api/workspaces/${workspaceId}/members/${accountId}`, {
		method: 'DELETE'
	});
	if (!res.ok && res.status !== 204) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'remove_failed';
		const message = (body as { message?: string }).message ?? `Failed to remove member (${res.status})`;
		throw new WorkspaceError(code, message);
	}
}

// ─── Invitations ──────────────────────────────────────────────────────────────

export interface WorkspaceInvitation {
	id: string;
	workspaceId: string;
	email: string;
	role: 'owner' | 'admin' | 'member' | 'viewer';
	expiresAt: string;
	createdAt: string;
}

export async function createInvitation(
	workspaceId: string,
	email: string,
	role: string
): Promise<WorkspaceInvitation> {
	const res = await fetch(`/api/workspaces/${workspaceId}/invitations`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, role })
	});
	const body = await res.json().catch(() => ({}));
	if (!res.ok) {
		const code = (body as { code?: string }).code ?? 'invite_failed';
		const message = (body as { message?: string }).message ?? `Failed to send invitation (${res.status})`;
		throw new WorkspaceError(code, message);
	}
	return body as WorkspaceInvitation;
}

export async function listInvitations(workspaceId: string): Promise<WorkspaceInvitation[]> {
	const res = await fetch(`/api/workspaces/${workspaceId}/invitations`);
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'list_failed';
		const message = (body as { message?: string }).message ?? `Failed to load invitations (${res.status})`;
		throw new WorkspaceError(code, message);
	}
	const body = await res.json();
	return body.invitations ?? [];
}

export async function revokeInvitation(workspaceId: string, inviteId: string): Promise<void> {
	const res = await fetch(`/api/workspaces/${workspaceId}/invitations/${inviteId}`, {
		method: 'DELETE'
	});
	if (!res.ok && res.status !== 204) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'revoke_failed';
		const message = (body as { message?: string }).message ?? `Failed to revoke invitation (${res.status})`;
		throw new WorkspaceError(code, message);
	}
}

/**
 * Ensure the current account has an X25519 identity keypair on the server.
 * Call this after accepting a workspace invitation so the granting admin can
 * find the new member's public key when distributing the workspace key.
 */
export async function ensureIdentityKey(masterKey: CryptoKey): Promise<void> {
	await getOrCreateIdentityKey(masterKey);
}

// ─── Invitation acceptance ────────────────────────────────────────────────────

export interface InvitePreview {
	id: string;
	workspaceName: string;
	inviterUsername: string;
	role: string;
	expiresAt: string;
}

export async function resolveInvitation(token: string): Promise<InvitePreview> {
	const res = await fetch(`/api/invitations/${token}`);
	if (res.status === 404) throw new WorkspaceError('not_found', 'Invitation not found.');
	if (res.status === 410) throw new WorkspaceError('expired', 'This invitation has expired or already been used.');
	if (!res.ok) throw new WorkspaceError('resolve_failed', `Failed to load invitation (${res.status})`);
	return res.json() as Promise<InvitePreview>;
}

export async function acceptInvitation(token: string): Promise<void> {
	const res = await fetch(`/api/invitations/${token}/accept`, { method: 'POST' });
	if (res.status === 204 || res.ok) return;
	const body = await res.json().catch(() => ({}));
	const code = (body as { code?: string }).code ?? 'accept_failed';
	const message = (body as { message?: string }).message ?? `Failed to accept invitation (${res.status})`;
	throw new WorkspaceError(code, message);
}

// ─── Key distribution ─────────────────────────────────────────────────────────

export interface PendingGrant {
	accountId: string;
	username: string;
}

export interface MemberIdentityKey {
	accountId: string;
	identityPublicKey: string; // base64
}

export async function listPendingGrants(workspaceId: string): Promise<PendingGrant[]> {
	const res = await fetch(`/api/workspaces/${workspaceId}/pending-key-grants`);
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'list_failed';
		const message = (body as { message?: string }).message ?? `Failed to load pending grants (${res.status})`;
		throw new WorkspaceError(code, message);
	}
	const body = await res.json();
	return body.pending ?? [];
}

export async function listMemberIdentityKeys(workspaceId: string): Promise<MemberIdentityKey[]> {
	const res = await fetch(`/api/workspaces/${workspaceId}/members/identity-keys`);
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		const code = (body as { code?: string }).code ?? 'list_failed';
		const message = (body as { message?: string }).message ?? `Failed to load identity keys (${res.status})`;
		throw new WorkspaceError(code, message);
	}
	const body = await res.json();
	return body.members ?? [];
}

export async function grantMemberKey(
	workspaceId: string,
	accountId: string,
	wrappedWorkspaceKey: string,
	ephemeralPublicKey: string
): Promise<void> {
	const res = await fetch(`/api/workspaces/${workspaceId}/member-key`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ accountId, wrappedWorkspaceKey, ephemeralPublicKey })
	});
	if (res.status === 204 || res.ok) return;
	const body = await res.json().catch(() => ({}));
	const code = (body as { code?: string }).code ?? 'grant_failed';
	const message = (body as { message?: string }).message ?? `Failed to grant key (${res.status})`;
	throw new WorkspaceError(code, message);
}

/**
 * Grant the workspace key to a member who has no key entry yet.
 *
 * Flow:
 *   1. Fetch own identity keypair (public + encrypted private)
 *   2. Decrypt identity private key with masterKey
 *   3. Fetch own wrapped workspace key
 *   4. ECIES-decrypt it, then ECIES-re-encrypt for the target member
 *   5. POST the newly wrapped key to the server
 */
export async function grantKey(
	workspaceId: string,
	targetAccountId: string,
	targetIdentityPubKeyB64: string,
	masterKey: CryptoKey
): Promise<void> {
	// 1. Own identity keypair
	const ikRes = await fetch('/api/account/identity-key');
	if (!ikRes.ok) throw new WorkspaceError('identity_key', 'Failed to load your identity key');
	const ikBody = await ikRes.json() as { wrappedIdentityPrivateKey: string };

	// 2. Decrypt identity private key
	const wrappedPrivBlob = base64ToBytes(ikBody.wrappedIdentityPrivateKey).buffer as ArrayBuffer;
	const identityPrivKey = await unwrapIdentityPrivateKey(wrappedPrivBlob, masterKey);

	// 3. Own workspace key
	const wkRes = await fetch(`/api/workspaces/${workspaceId}/member-key`);
	if (!wkRes.ok) throw new WorkspaceError('workspace_key', 'Failed to load workspace key');
	const wkBody = await wkRes.json() as { wrappedWorkspaceKey: string; ephemeralPublicKey: string };

	const wrappedKey = base64ToBytes(wkBody.wrappedWorkspaceKey).buffer as ArrayBuffer;
	const ephPub = base64ToBytes(wkBody.ephemeralPublicKey).buffer as ArrayBuffer;
	const recipientPub = base64ToBytes(targetIdentityPubKeyB64).buffer as ArrayBuffer;

	// 4. Re-wrap for recipient
	const { wrappedWorkspaceKey, ephemeralPublicKey } = await rewrapWorkspaceKey(
		wrappedKey,
		ephPub,
		identityPrivKey,
		recipientPub
	);

	// 5. POST
	await grantMemberKey(
		workspaceId,
		targetAccountId,
		bytesToBase64(new Uint8Array(wrappedWorkspaceKey)),
		bytesToBase64(new Uint8Array(ephemeralPublicKey))
	);
}

/**
 * Decrypt and return the caller's workspace AES-256-GCM key.
 *
 * Fetches the caller's wrapped workspace key entry, decrypts the identity
 * private key with masterKey, then ECIES-decrypts the workspace key.
 * Result is cached in memory for the lifetime of the page.
 */
export async function loadWorkspaceKey(workspaceId: string, masterKey: CryptoKey): Promise<CryptoKey> {
	const cached = workspaceKeyCache.get(workspaceId);
	if (cached) return cached;

	// Own identity private key
	const ikRes = await fetch('/api/account/identity-key');
	if (!ikRes.ok) throw new WorkspaceError('identity_key', 'Failed to load identity key');
	const ikBody = await ikRes.json() as { wrappedIdentityPrivateKey: string };
	const wrappedPrivBlob = base64ToBytes(ikBody.wrappedIdentityPrivateKey).buffer as ArrayBuffer;
	const identityPrivKey = await unwrapIdentityPrivateKey(wrappedPrivBlob, masterKey);

	// Own workspace key entry
	const wkRes = await fetch(`/api/workspaces/${workspaceId}/member-key`);
	if (!wkRes.ok) throw new WorkspaceError('workspace_key', 'No workspace key — not yet granted access');
	const wkBody = await wkRes.json() as { wrappedWorkspaceKey: string; ephemeralPublicKey: string };

	const wrappedKey = base64ToBytes(wkBody.wrappedWorkspaceKey).buffer as ArrayBuffer;
	const ephPub = base64ToBytes(wkBody.ephemeralPublicKey).buffer as ArrayBuffer;

	// ECIES-decrypt: ephPub × identityPrivKey → HKDF → AES-GCM decrypt raw workspace key bytes
	// Reuse the same decrypt path as rewrapWorkspaceKey but just import the raw bytes
	const ephPubKey = await crypto.subtle.importKey('raw', ephPub, { name: 'X25519' }, false, []);
	const sharedSecret = await crypto.subtle.deriveKey(
		{ name: 'X25519', public: ephPubKey },
		identityPrivKey,
		{ name: 'HKDF' },
		false,
		['deriveKey']
	);
	const encoder = new TextEncoder();
	const decKey = await crypto.subtle.deriveKey(
		{
			name: 'HKDF',
			hash: 'SHA-256',
			salt: new ArrayBuffer(0),
			info: encoder.encode('confide-workspace-key-v1')
		},
		sharedSecret,
		{ name: 'AES-GCM', length: 256 },
		false,
		['decrypt']
	);
	const wrappedBytes = new Uint8Array(wrappedKey);
	const iv = wrappedBytes.slice(0, 12);
	const ciphertext = wrappedBytes.slice(12);
	const rawWorkspaceKey = await crypto.subtle.decrypt(
		{ name: 'AES-GCM', iv, tagLength: 128 },
		decKey,
		ciphertext
	);

	const workspaceKey = await crypto.subtle.importKey(
		'raw',
		rawWorkspaceKey,
		{ name: 'AES-GCM', length: 256 },
		true,
		['encrypt', 'decrypt']
	);

	workspaceKeyCache.set(workspaceId, workspaceKey);
	return workspaceKey;
}

/**
 * Create a new workspace. Generates a workspace AES-GCM key, wraps it for the
 * caller's identity public key via ECIES, then POSTs to the API.
 */
export async function createWorkspace(name: string, masterKey: CryptoKey): Promise<Workspace> {
	const identityPublicKey = await getOrCreateIdentityKey(masterKey);
	const { wrappedWorkspaceKey, ephemeralPublicKey } =
		await generateAndWrapWorkspaceKey(identityPublicKey);

	const res = await fetch('/api/workspaces', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			name,
			wrappedWorkspaceKey: bytesToBase64(new Uint8Array(wrappedWorkspaceKey)),
			ephemeralPublicKey: bytesToBase64(new Uint8Array(ephemeralPublicKey))
		})
	});

	const body = await res.json().catch(() => ({}));
	if (!res.ok) {
		const code = (body as { code?: string }).code ?? 'create_failed';
		const message = (body as { message?: string }).message ?? `Failed to create workspace (${res.status})`;
		throw new WorkspaceError(code, message);
	}

	return body as Workspace;
}

/**
 * Generate and upload a workspace key for the caller's personal workspace.
 * Called once during registration after the identity key is created.
 */
export async function setupPersonalWorkspaceKey(masterKey: CryptoKey, accountId: string): Promise<void> {
	const workspaces = await listWorkspaces();
	const personal = workspaces.find((w) => w.role === 'owner');
	if (!personal) return;

	const identityPublicKey = await getOrCreateIdentityKey(masterKey);
	const { wrappedWorkspaceKey, ephemeralPublicKey } =
		await generateAndWrapWorkspaceKey(identityPublicKey);

	const res = await fetch(`/api/workspaces/${personal.id}/member-key`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			accountId,
			wrappedWorkspaceKey: bytesToBase64(new Uint8Array(wrappedWorkspaceKey)),
			ephemeralPublicKey: bytesToBase64(new Uint8Array(ephemeralPublicKey))
		})
	});
	if (!res.ok && res.status !== 204) {
		throw new WorkspaceError('setup_key_failed', `Failed to set up workspace key (${res.status})`);
	}
}
