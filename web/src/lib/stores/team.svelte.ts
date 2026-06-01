/**
 * Confide team store (Svelte 5 runes).
 *
 * Workspace-aware: the cache is keyed to the currently-active workspace.
 * Switching workspaces resets the cache and triggers a fresh load.
 * Navigating back to /team reuses cached data — no re-fetch.
 */

import {
	listInvitations,
	listMemberIdentityKeys,
	listMembers,
	type MemberIdentityKey,
	type WorkspaceInvitation,
	type WorkspaceMember,
} from "$lib/workspaces";

let _workspaceId = $state<string | null>(null);
let _members = $state<WorkspaceMember[]>([]);
let _invitations = $state<WorkspaceInvitation[]>([]);
let _identityKeys = $state<Map<string, string>>(new Map());
let _loaded = $state(false);
let _loading = $state(false);
let _error = $state("");

export const teamStore = {
	get members() {
		return _members;
	},
	get invitations() {
		return _invitations;
	},
	get identityKeys() {
		return _identityKeys;
	},
	get loaded() {
		return _loaded;
	},
	get loading() {
		return _loading;
	},
	get error() {
		return _error;
	},
	get workspaceId() {
		return _workspaceId;
	},

	/** Load team data for a workspace. No-ops if already loaded for the same workspace. */
	async load(workspaceId: string, isAdmin: boolean) {
		if (_workspaceId === workspaceId && (_loaded || _loading)) return;

		_workspaceId = workspaceId;
		_loaded = false;
		_loading = true;
		_error = "";
		_members = [];
		_invitations = [];
		_identityKeys = new Map();

		const tasks: Promise<unknown>[] = [
			listMembers(workspaceId),
			listInvitations(workspaceId),
		];
		if (isAdmin) tasks.push(listMemberIdentityKeys(workspaceId));

		const [membersResult, invitationsResult, keysResult] =
			await Promise.allSettled(tasks);

		// Guard against a concurrent workspace switch overtaking this load
		if (_workspaceId !== workspaceId) return;

		if (membersResult.status === "fulfilled") {
			_members = membersResult.value as WorkspaceMember[];
		} else {
			_error =
				membersResult.reason instanceof Error
					? membersResult.reason.message
					: "Failed to load members";
		}

		if (invitationsResult.status === "fulfilled") {
			_invitations = invitationsResult.value as WorkspaceInvitation[];
		}

		if (keysResult?.status === "fulfilled") {
			const keyList = keysResult.value as MemberIdentityKey[];
			_identityKeys = new Map(
				keyList.map((k) => [k.accountId, k.identityPublicKey]),
			);
		}

		_loaded = true;
		_loading = false;
	},

	/** Force a re-fetch on next load() call (e.g. after inviting or removing a member). */
	invalidate() {
		_loaded = false;
	},

	updateMember(accountId: string, patch: Partial<WorkspaceMember>) {
		_members = _members.map((m) =>
			m.accountId === accountId ? { ...m, ...patch } : m,
		);
	},

	removeMember(accountId: string) {
		_members = _members.filter((m) => m.accountId !== accountId);
	},

	addInvitation(inv: WorkspaceInvitation) {
		_invitations = [..._invitations, inv];
	},

	removeInvitation(invitationId: string) {
		_invitations = _invitations.filter((i) => i.id !== invitationId);
	},

	setIdentityKey(accountId: string, key: string) {
		const next = new Map(_identityKeys);
		next.set(accountId, key);
		_identityKeys = next;
	},

	/** Wipe everything — call on logout. */
	clear() {
		_workspaceId = null;
		_members = [];
		_invitations = [];
		_identityKeys = new Map();
		_loaded = false;
		_loading = false;
		_error = "";
	},
};
