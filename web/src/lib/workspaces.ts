import { generateAndWrapWorkspaceKey } from '$lib/crypto';

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
