<script lang="ts">
	import { onMount } from 'svelte';
	import {
		KeyRound,
		Plus,
		Pencil,
		Trash2,
		ShieldCheck,
		Smartphone,
		Monitor,
		LayoutGrid,
		User
	} from '@lucide/svelte';
	import {
		listCredentials,
		renameCredential,
		deleteCredential,
		reauthenticateForAddCredential,
		addCredential,
		listSessions,
		revokeSession
	} from '$lib/auth';
	import type { CredentialSummary, SessionInfo } from '$lib/types/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';

	// ─── Passkeys ────────────────────────────────────────────────────────────────

	let credentials = $state<CredentialSummary[]>([]);
	let credsLoading = $state(true);
	let credsError = $state<string | null>(null);

	let editingId = $state<string | null>(null);
	let editingName = $state('');
	let saving = $state(false);

	let deletingId = $state<string | null>(null);
	let confirmDeleteId = $state<string | null>(null);

	let addStep = $state<'idle' | 'naming' | 'reauth' | 'registering'>('idle');
	let newName = $state('');
	let addError = $state<string | null>(null);

	// ─── Sessions ────────────────────────────────────────────────────────────────

	let sessions = $state<SessionInfo[]>([]);
	let sessionsLoading = $state(true);
	let sessionsError = $state<string | null>(null);
	let revoking = $state<string | null>(null);

	// ─── Lifecycle ───────────────────────────────────────────────────────────────

	onMount(async () => {
		await Promise.all([loadCredentials(), loadSessions()]);
	});

	// ─── Passkey functions ────────────────────────────────────────────────────────

	async function loadCredentials() {
		credsLoading = true;
		credsError = null;
		try {
			credentials = await listCredentials();
		} catch (err) {
			credsError = err instanceof Error ? err.message : 'Failed to load passkeys.';
		} finally {
			credsLoading = false;
		}
	}

	function startEdit(cred: CredentialSummary) {
		editingId = cred.id;
		editingName = cred.name;
	}

	function cancelEdit() {
		editingId = null;
		editingName = '';
	}

	async function saveEdit(id: string) {
		saving = true;
		try {
			await renameCredential(id, editingName);
			credentials = credentials.map((c) => (c.id === id ? { ...c, name: editingName } : c));
			editingId = null;
		} catch (err) {
			credsError = err instanceof Error ? err.message : 'Failed to rename passkey.';
		} finally {
			saving = false;
		}
	}

	function promptDelete(id: string) {
		confirmDeleteId = id;
	}

	function cancelDelete() {
		confirmDeleteId = null;
	}

	async function confirmDelete(id: string) {
		deletingId = id;
		confirmDeleteId = null;
		try {
			await deleteCredential(id);
			credentials = credentials.filter((c) => c.id !== id);
		} catch (err) {
			credsError = err instanceof Error ? err.message : 'Failed to delete passkey.';
		} finally {
			deletingId = null;
		}
	}

	async function handleAddPasskey() {
		addError = null;
		addStep = 'reauth';
		try {
			const token = await reauthenticateForAddCredential();
			addStep = 'registering';
			const mk = auth.masterKey;
			if (!mk) throw new Error('Master key not available. Please re-authenticate.');
			const result = await addCredential(mk, token, newName);
			credentials = [
				...credentials,
				{
					id: result.id,
					name: result.name || newName,
					createdAt: result.createdAt,
					backupEligible: false,
					isCurrentSession: false
				}
			];
			addStep = 'idle';
			newName = '';
		} catch (err) {
			addError = err instanceof Error ? err.message : 'Failed to add passkey.';
			addStep = 'naming';
		}
	}

	// ─── Session functions ────────────────────────────────────────────────────────

	async function loadSessions() {
		sessionsLoading = true;
		sessionsError = null;
		try {
			sessions = await listSessions();
		} catch (err) {
			sessionsError = err instanceof Error ? err.message : 'Failed to load sessions.';
		} finally {
			sessionsLoading = false;
		}
	}

	async function handleRevoke(sessionId: string) {
		revoking = sessionId;
		try {
			await revokeSession(sessionId);
			sessions = sessions.filter((s) => s.id !== sessionId);
		} catch (err) {
			sessionsError = err instanceof Error ? err.message : 'Failed to revoke session.';
		} finally {
			revoking = null;
		}
	}

	// ─── Helpers ─────────────────────────────────────────────────────────────────

	function isMobile(ua: string | undefined): boolean {
		if (!ua) return false;
		return /Mobile|Android|iPhone|iPad|iPod/i.test(ua);
	}

	function formatDate(iso: string): string {
		try {
			return new Date(iso).toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			});
		} catch {
			return iso;
		}
	}

	const roleBadge: Record<string, { label: string; color: string; border: string }> = {
		owner:  { label: 'owner',  color: '#a78bfa', border: '#4c1d95' },
		admin:  { label: 'admin',  color: '#60a5fa', border: '#1e3a5f' },
		member: { label: 'member', color: '#4b6280', border: '#243447' },
		viewer: { label: 'viewer', color: '#4b6280', border: '#243447' }
	};

	const planBadge: Record<string, { label: string; color: string }> = {
		pro:  { label: 'Pro',  color: '#f59e0b' },
		free: { label: 'Free', color: '#4b6280' }
	};
</script>

<svelte:head>
	<title>Confide — Me</title>
</svelte:head>

<div class="font-mono max-w-2xl mx-auto px-4 pt-12 pb-16 sm:p-8 sm:pb-16">

	<!-- ─── Account ─────────────────────────────────────────────────────────── -->
	<div class="mb-10">
		<div class="flex items-center gap-3 mb-6">
			<div class="w-10 h-10 rounded-md bg-[#0f1e30] border border-[#1e3347] flex items-center justify-center text-[#4b6280]">
				<User size={18} strokeWidth={1.75} />
			</div>
			<div>
				<h1 class="text-xl m-0 text-[#e2e8f0]">Profile</h1>
				{#if auth.accountId}
					<div class="text-xs text-[#4b6280] mt-0.5 font-mono">{auth.accountId}</div>
				{/if}
			</div>
		</div>
	</div>

	<!-- ─── Workspaces ──────────────────────────────────────────────────────── -->
	<section class="mb-10">
		<div class="flex items-center gap-2 mb-4">
			<LayoutGrid size={15} strokeWidth={1.75} class="text-[#4b6280]" />
			<h2 class="text-sm font-semibold text-[#8899aa] uppercase tracking-wider m-0">Workspaces</h2>
		</div>

		{#if !workspacesStore.loaded && workspacesStore.loading}
			<p class="text-[#8899aa] text-sm">Loading workspaces…</p>
		{:else if workspacesStore.workspaces.length === 0}
			<p class="text-[#8899aa] text-sm">No workspaces.</p>
		{:else}
			<div class="flex flex-col gap-1.5">
				{#each workspacesStore.workspaces as ws (ws.id)}
					{@const role = roleBadge[ws.role] ?? roleBadge.member}
					{@const plan = planBadge[ws.plan] ?? planBadge.free}
					<div class="flex items-center justify-between gap-3 px-4 py-3 border border-border-deep rounded-md">
						<div class="min-w-0">
							<div class="flex items-center gap-2">
								<span class="text-[#c5d3e0] text-sm truncate">{ws.name}</span>
								<span
									class="text-[10px] px-1.5 py-0.5 rounded border leading-none shrink-0"
									style="color: {role.color}; border-color: {role.border}; background: {role.border}22;"
								>
									{role.label}
								</span>
							</div>
							<div class="text-[#4b6280] text-xs mt-0.5">{ws.slug}</div>
						</div>
						<span class="text-[10px] shrink-0" style="color: {plan.color};">{plan.label}</span>
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<!-- ─── Sessions ───────────────────────────────────────────────────────── -->
	<section class="mb-10">
		<div class="flex items-center gap-2 mb-4">
			<Monitor size={15} strokeWidth={1.75} class="text-[#4b6280]" />
			<h2 class="text-sm font-semibold text-[#8899aa] uppercase tracking-wider m-0">Active Sessions</h2>
		</div>

		{#if sessionsError}
			<p class="text-error-light text-sm mb-3">{sessionsError}</p>
		{/if}

		{#if sessionsLoading}
			<p class="text-[#8899aa] text-sm">Loading sessions…</p>
		{:else if sessions.length === 0}
			<p class="text-[#8899aa] text-sm">No active sessions.</p>
		{:else}
			<div class="flex flex-col gap-1.5">
				{#each sessions as session (session.id)}
					<div class="flex items-start justify-between gap-3 px-4 py-3 border border-border-deep rounded-md">
						<div class="flex items-start gap-3 min-w-0">
							<div class="text-[#4b6280] shrink-0 mt-0.5" title={session.userAgent ?? 'Unknown device'}>
								{#if isMobile(session.userAgent)}
									<Smartphone size={18} strokeWidth={1.75} />
								{:else}
									<Monitor size={18} strokeWidth={1.75} />
								{/if}
							</div>
							<div class="min-w-0">
								<div class="text-[#c5d3e0] text-sm">{session.id.slice(0, 12)}…</div>
								<div class="text-[#4b6280] text-xs mt-0.5">Created {session.createdAt}</div>
								<div class="text-[#4b6280] text-xs">Last seen {session.lastSeen}</div>
							</div>
						</div>
						<button
							onclick={() => handleRevoke(session.id)}
							disabled={revoking === session.id || sessions.length <= 1}
							title={sessions.length <= 1 ? 'Cannot revoke your only session' : 'Revoke session'}
							class="shrink-0 px-3 py-1 bg-transparent border rounded cursor-pointer font-mono text-sm transition-[color,border-color] duration-100
								{revoking === session.id || sessions.length <= 1
									? 'text-[#2a3a4a] border-[#243447] cursor-not-allowed'
									: 'text-error-light border-[#7f1d1d] hover:bg-[#1a0e0e]'}"
						>
							{revoking === session.id ? 'Revoking…' : 'Revoke'}
						</button>
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<!-- ─── Passkeys ───────────────────────────────────────────────────────── -->
	<section>
		<div class="flex items-center justify-between gap-4 mb-4">
			<div class="flex items-center gap-2">
				<KeyRound size={15} strokeWidth={1.75} class="text-[#4b6280]" />
				<h2 class="text-sm font-semibold text-[#8899aa] uppercase tracking-wider m-0">Passkeys</h2>
			</div>
			{#if addStep === 'idle'}
				<button
					onclick={() => (addStep = 'naming')}
					class="flex items-center gap-1.5 px-3 py-1.5 border border-border-deep rounded-md text-sm text-[#c5d3e0] hover:bg-[#1a2535] transition-colors cursor-pointer bg-transparent font-mono"
				>
					<Plus size={13} strokeWidth={2} />
					Add passkey
				</button>
			{/if}
		</div>

		{#if credsError}
			<p class="text-error-light text-sm mb-3">{credsError}</p>
		{/if}

		<!-- Add passkey panel -->
		{#if addStep === 'naming'}
			<div class="mb-4 p-4 border border-border-deep rounded-md flex flex-col gap-3">
				<p class="text-[#c5d3e0] text-sm m-0">Name your new passkey (optional):</p>
				<input
					type="text"
					bind:value={newName}
					placeholder="e.g. MacBook Touch ID"
					class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-1.5 text-sm text-[#c5d3e0] placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070]"
				/>
				{#if addError}
					<p class="text-error-light text-xs m-0">{addError}</p>
				{/if}
				<div class="flex gap-2">
					<button
						onclick={handleAddPasskey}
						class="px-3 py-1.5 bg-[#1a2f4a] border border-[#2a4a6a] rounded text-sm text-[#c5d3e0] hover:bg-[#1e3555] transition-colors cursor-pointer font-mono"
					>
						Continue
					</button>
					<button
						onclick={() => { addStep = 'idle'; addError = null; newName = ''; }}
						class="px-3 py-1.5 bg-transparent border border-border-subtle rounded text-sm text-[#4b6280] hover:text-[#8899aa] transition-colors cursor-pointer font-mono"
					>
						Cancel
					</button>
				</div>
			</div>
		{:else if addStep === 'reauth'}
			<div class="mb-4 p-4 border border-border-deep rounded-md">
				<p class="text-[#8899aa] text-sm animate-pulse m-0">Verifying your existing passkey…</p>
			</div>
		{:else if addStep === 'registering'}
			<div class="mb-4 p-4 border border-border-deep rounded-md">
				<p class="text-[#8899aa] text-sm animate-pulse m-0">Registering new passkey…</p>
			</div>
		{/if}

		{#if credsLoading}
			<p class="text-[#8899aa] text-sm">Loading passkeys…</p>
		{:else if credentials.length === 0}
			<p class="text-[#8899aa] text-sm">No passkeys found.</p>
		{:else}
			<div class="flex flex-col gap-1.5">
				{#each credentials as cred (cred.id)}
					<div class="flex items-start justify-between gap-3 px-4 py-3 border border-border-deep rounded-md">
						<div class="flex items-start gap-3 min-w-0">
							<div class="text-[#4b6280] shrink-0 mt-0.5">
								<KeyRound size={18} strokeWidth={1.75} />
							</div>
							<div class="min-w-0">
								{#if editingId === cred.id}
									<input
										type="text"
										bind:value={editingName}
										onkeydown={(e) => {
											if (e.key === 'Enter') saveEdit(cred.id);
											if (e.key === 'Escape') cancelEdit();
										}}
										class="font-mono bg-[#0d1520] border border-[#2a4a6a] rounded px-2 py-0.5 text-sm text-[#c5d3e0] focus:outline-none"
									/>
									<div class="flex gap-2 mt-1">
										<button
											onclick={() => saveEdit(cred.id)}
											disabled={saving}
											class="text-xs text-[#7aadcf] hover:text-[#a8d4ef] cursor-pointer bg-transparent border-none font-mono p-0"
										>
											{saving ? 'Saving…' : 'Save'}
										</button>
										<button
											onclick={cancelEdit}
											class="text-xs text-[#4b6280] hover:text-[#8899aa] cursor-pointer bg-transparent border-none font-mono p-0"
										>
											Cancel
										</button>
									</div>
								{:else}
									<div class="flex items-center gap-2">
										<span class="text-[#c5d3e0] text-sm">
											{cred.name || 'Unnamed passkey'}
										</span>
										{#if cred.isCurrentSession}
											<span class="text-[10px] px-1.5 py-0.5 bg-[#0d2a1a] border border-[#1a5030] rounded text-[#4a9060] leading-none">
												This session
											</span>
										{/if}
										{#if cred.backupEligible}
											<span title="Backup eligible" class="text-[#3a7a5a]">
												<ShieldCheck size={12} strokeWidth={2} />
											</span>
										{/if}
									</div>
									<div class="text-[#4b6280] text-xs mt-0.5">Added {formatDate(cred.createdAt)}</div>
								{/if}
							</div>
						</div>

						<div class="flex items-center gap-1.5 shrink-0">
							{#if editingId !== cred.id}
								<button
									onclick={() => startEdit(cred)}
									title="Rename"
									class="p-1.5 text-[#4b6280] hover:text-[#8899aa] transition-colors cursor-pointer bg-transparent border-none rounded"
								>
									<Pencil size={14} strokeWidth={1.75} />
								</button>
							{/if}

							{#if confirmDeleteId === cred.id}
								<div class="flex items-center gap-1.5">
									<span class="text-xs text-[#8899aa]">Delete?</span>
									<button
										onclick={() => confirmDelete(cred.id)}
										disabled={deletingId === cred.id}
										class="px-2 py-0.5 border border-[#7f1d1d] rounded text-xs text-error-light hover:bg-[#1a0e0e] transition-colors cursor-pointer bg-transparent font-mono"
									>
										{deletingId === cred.id ? '…' : 'Yes'}
									</button>
									<button
										onclick={cancelDelete}
										class="px-2 py-0.5 border border-border-subtle rounded text-xs text-[#4b6280] hover:text-[#8899aa] transition-colors cursor-pointer bg-transparent font-mono"
									>
										No
									</button>
								</div>
							{:else}
								<button
									onclick={() => promptDelete(cred.id)}
									disabled={credentials.length <= 1}
									title={credentials.length <= 1 ? 'You must have at least one passkey' : 'Delete passkey'}
									class="p-1.5 transition-colors cursor-pointer bg-transparent border-none rounded
										{credentials.length <= 1
											? 'text-[#2a3a4a] cursor-not-allowed'
											: 'text-[#4b6280] hover:text-error-light'}"
								>
									<Trash2 size={14} strokeWidth={1.75} />
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</section>

</div>
