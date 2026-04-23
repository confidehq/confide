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
		ShieldAlert,
		Copy,
		Check,
		RefreshCw
	} from '@lucide/svelte';
	import {
		listCredentials,
		renameCredential,
		deleteCredential,
		reauthenticateForAddCredential,
		addCredential,
		listSessions,
		revokeSession,
		deleteAccount,
		rotateRecoveryCode,
		reauthenticate
	} from '$lib/auth';
	import type { CredentialSummary, SessionInfo } from '$lib/types/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { goto } from '$app/navigation';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

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

	// ─── Account deletion ─────────────────────────────────────────────────────────

	let showDeleteAccountConfirm = $state(false);
	let deletingAccount = $state(false);
	let deleteAccountError = $state('');

	async function handleDeleteAccount() {
		deletingAccount = true;
		deleteAccountError = '';
		try {
			await deleteAccount();
			auth.clearAll();
			await goto('/login');
		} catch (err) {
			deleteAccountError = err instanceof Error ? err.message : 'Failed to delete account.';
			deletingAccount = false;
		}
	}

	// ─── Recovery codes ───────────────────────────────────────────────────────────

	let recoveryDialogOpen = $state(false);
	let recoveryCode = $state<string | null>(null);
	let generatingCode = $state(false);
	let codeGenError = $state<string | null>(null);
	let codeCopied = $state(false);

	async function handleGenerateRecoveryCode() {
		generatingCode = true;
		codeGenError = null;
		try {
			let mk = auth.masterKey;
			if (!mk) {
				const result = await reauthenticate();
				mk = result.masterKey;
				auth.setSession(mk, result.accountId, result.credentialId);
			}
			recoveryCode = await rotateRecoveryCode(mk);
		} catch (err) {
			codeGenError = err instanceof Error ? err.message : 'Failed to generate recovery code.';
		} finally {
			generatingCode = false;
		}
	}

	async function copyCode() {
		if (!recoveryCode) return;
		await navigator.clipboard.writeText(recoveryCode);
		codeCopied = true;
		setTimeout(() => (codeCopied = false), 2000);
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

	function avatarInitials(): string {
		const name = auth.username ?? auth.accountId ?? '';
		return name.slice(0, 2).toUpperCase() || '??';
	}

	const roleBadge: Record<string, { label: string; color: string; border: string }> = {
		owner:  { label: 'owner',  color: '#a78bfa', border: '#4c1d95' },
		admin:  { label: 'admin',  color: '#60a5fa', border: '#1e3a5f' },
		member: { label: 'member', color: 'var(--color-muted-dim)', border: '#243447' },
		viewer: { label: 'viewer', color: 'var(--color-muted-dim)', border: '#243447' }
	};

	const planBadge: Record<string, { label: string; color: string }> = {
		pro:  { label: 'Pro',  color: 'var(--color-warning-border)' },
		free: { label: 'Free', color: 'var(--color-muted-dim)' }
	};
</script>

<svelte:head>
	<title>Confide — Me</title>
</svelte:head>

<ConfirmDialog
	open={showDeleteAccountConfirm}
	title="Delete account?"
	description="This will permanently delete your account, all your workspaces, forms, and responses. This cannot be undone."
	loading={deletingAccount}
	error={deleteAccountError}
	onconfirm={handleDeleteAccount}
	oncancel={() => { showDeleteAccountConfirm = false; deleteAccountError = ''; }}
/>

<!-- Recovery code dialog -->
{#if recoveryDialogOpen}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background: var(--color-overlay); backdrop-filter: blur(2px);"
		onclick={(e) => { if (e.target === e.currentTarget) { recoveryDialogOpen = false; } }}
		onkeydown={(e) => { if (e.key === 'Escape') recoveryDialogOpen = false; }}
		role="presentation"
	>
		<div
			class="font-mono w-full max-w-md flex flex-col gap-5"
			style="background: var(--color-surface-subtle); border: 1px solid var(--color-border-deep); border-radius: 10px; padding: 1.5rem; box-shadow: 0 24px 48px -12px rgba(0,0,0,0.7);"
			role="dialog"
			aria-modal="true"
			aria-labelledby="recovery-dialog-title"
		>
			<!-- Header -->
			<div class="flex items-center justify-between gap-3">
				<div class="flex items-center gap-2.5">
					<span class="shrink-0 flex items-center justify-center w-7 h-7 rounded-md bg-surface-deep border border-border">
						<ShieldAlert size={14} strokeWidth={1.75} class="text-muted-dim" />
					</span>
					<h2 id="recovery-dialog-title" class="m-0 text-base font-semibold text-text-bright">Recovery Code</h2>
				</div>
				<button
					onclick={() => { recoveryDialogOpen = false; }}
					class="p-1.5 text-muted-dim hover:text-muted-blue bg-transparent border-none cursor-pointer rounded transition-colors duration-100"
					aria-label="Close"
				>×</button>
			</div>

			{#if recoveryCode}
				<!-- Code is generated — show it prominently -->
				<p class="m-0 text-sm text-muted-dim leading-relaxed">
					Store this code somewhere safe. If you lose all your passkeys, this is your only way back into your account.
				</p>

				<div class="flex flex-col gap-2">
					<div
						class="px-4 py-4 rounded-lg border border-border-deep select-all font-mono text-sm text-text-body break-all leading-loose"
						style="background: var(--color-surface-deep); letter-spacing: 0.05em;"
					>
						{recoveryCode}
					</div>
					<button
						onclick={copyCode}
						class="flex items-center justify-center gap-2 w-full py-2 text-sm font-medium rounded border cursor-pointer font-mono transition-colors duration-100
							{codeCopied
								? 'bg-success-bg border-success-border text-success-text-dark'
								: 'bg-transparent border-border-deep text-muted-dim hover:text-text-body hover:border-border-subtle'}"
					>
						{#if codeCopied}
							<Check size={13} strokeWidth={2} />
							Copied
						{:else}
							<Copy size={13} strokeWidth={1.75} />
							Copy to clipboard
						{/if}
					</button>
				</div>

				<p class="m-0 text-xs text-muted-mid leading-relaxed">
					This code replaces any previous recovery code. Write it down or store it in a password manager — it won't be shown again.
				</p>
			{:else}
				<!-- No code generated yet -->
				<p class="m-0 text-sm text-muted-dim leading-relaxed">
					Recovery codes let you regain access to your account if you lose all your passkeys. Generate one and keep it somewhere safe.
				</p>
			{/if}

			{#if codeGenError}
				<p class="m-0 text-sm text-error-light">{codeGenError}</p>
			{/if}

			<div class="h-px bg-border-deep"></div>

			<button
				onclick={handleGenerateRecoveryCode}
				disabled={generatingCode}
				class="flex items-center justify-center gap-2 w-full py-2.5 text-sm font-medium rounded border cursor-pointer font-mono transition-colors duration-100
					bg-transparent border-border-deep text-muted-dim hover:text-text-body hover:border-border-subtle
					disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{#if generatingCode}
					Generating…
				{:else}
					<RefreshCw size={13} strokeWidth={1.75} />
					{recoveryCode ? 'Generate fresh recovery code' : 'Generate recovery code'}
				{/if}
			</button>
		</div>
	</div>
{/if}

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- ─── Header ──────────────────────────────────────────────────────────── -->
	<div class="flex items-center gap-4 mb-8">
		<div class="w-12 h-12 rounded-lg bg-surface-deep border border-border-deep flex items-center justify-center text-base font-semibold text-muted-dim shrink-0 select-none">
			{avatarInitials()}
		</div>
		<div>
			<h1 class="text-2xl m-0 mb-0.5 text-text-bright font-semibold">{auth.username ?? 'Account'}</h1>
			{#if auth.accountId}
				<p class="m-0 text-sm text-muted-dim">{auth.accountId}</p>
			{/if}
		</div>
	</div>

	<!-- ─── Two-column grid on large screens ────────────────────────────────── -->
	<div class="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)] gap-8 lg:gap-10 items-start">

		<!-- ── Left column ─────────────────────────────────────────────────── -->
		<div>
			<!-- Workspaces -->
			<div class="flex items-center justify-between mb-3">
				<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">
					<span class="inline-flex items-center gap-2">
						<LayoutGrid size={14} strokeWidth={1.75} />
						Workspaces
					</span>
				</h2>
			</div>

			{#if !workspacesStore.loaded && workspacesStore.loading}
				<p class="text-muted-dim text-base">Loading…</p>
			{:else if workspacesStore.workspaces.length === 0}
				<div class="py-10 border border-dashed border-border rounded-lg text-center">
					<p class="m-0 text-muted-dim text-base">No workspaces</p>
				</div>
			{:else}
				<div class="border border-border-deep rounded-lg overflow-hidden">
					{#each workspacesStore.workspaces as ws, i (ws.id)}
						{@const role = roleBadge[ws.role] ?? roleBadge.member}
						{@const plan = planBadge[ws.plan] ?? planBadge.free}
						<div class="flex items-center gap-3 px-4 py-3.5
							{i < workspacesStore.workspaces.length - 1 ? 'border-b border-border-deep' : ''}">
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2">
									<span class="text-base text-text-body truncate">{ws.name}</span>
									<span
										class="text-[10px] px-1.5 py-0.5 rounded border leading-none shrink-0"
										style="color: {role.color}; border-color: {role.border}; background: {role.border}22;"
									>
										{role.label}
									</span>
								</div>
								<p class="m-0 text-sm text-muted-dim mt-0.5">{ws.slug}</p>
							</div>
							<span class="text-sm shrink-0 font-semibold" style="color: {plan.color};">{plan.label}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- ── Right column ────────────────────────────────────────────────── -->
		<div class="flex flex-col gap-8">

			<!-- Sessions -->
			<div>
				<div class="flex items-center justify-between mb-3">
					<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">
						<span class="inline-flex items-center gap-2">
							<Monitor size={14} strokeWidth={1.75} />
							Active Sessions
						</span>
					</h2>
				</div>

				{#if sessionsError}
					<p class="text-error-light text-base mb-3">{sessionsError}</p>
				{/if}

				{#if sessionsLoading}
					<p class="text-muted-dim text-base">Loading…</p>
				{:else if sessions.length === 0}
					<div class="py-10 border border-dashed border-border rounded-lg text-center">
						<p class="m-0 text-muted-dim text-base">No active sessions</p>
					</div>
				{:else}
					<div class="border border-border-deep rounded-lg overflow-hidden">
						{#each sessions as session, i (session.id)}
							<div class="flex items-center gap-3 px-4 py-3.5
								{i < sessions.length - 1 ? 'border-b border-border-deep' : ''}">
								<div class="text-muted-dim shrink-0" title={session.userAgent ?? 'Unknown device'}>
									{#if isMobile(session.userAgent)}
										<Smartphone size={16} strokeWidth={1.75} />
									{:else}
										<Monitor size={16} strokeWidth={1.75} />
									{/if}
								</div>
								<div class="flex-1 min-w-0">
									<span class="text-base text-text-body truncate block">{session.id.slice(0, 16)}…</span>
									<p class="m-0 text-sm text-muted-dim mt-0.5">
										Created {session.createdAt} · Last seen {session.lastSeen}
									</p>
								</div>
								<button
									onclick={() => handleRevoke(session.id)}
									disabled={revoking === session.id || sessions.length <= 1}
									title={sessions.length <= 1 ? 'Cannot revoke your only session' : 'Revoke session'}
									class="shrink-0 px-3 py-1.5 bg-transparent border rounded cursor-pointer font-mono text-base transition-[color,border-color] duration-100
										{revoking === session.id || sessions.length <= 1
											? 'text-muted-mid border-border-mid cursor-not-allowed'
											: 'text-error-light border-border-danger-dark hover:bg-danger-hover'}"
								>
									{revoking === session.id ? '…' : 'Revoke'}
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Passkeys -->
			<div>
				<div class="flex items-center justify-between mb-3">
					<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">
						<span class="inline-flex items-center gap-2">
							<KeyRound size={14} strokeWidth={1.75} />
							Passkeys
						</span>
					</h2>
					{#if addStep === 'idle'}
						<button
							onclick={() => (addStep = 'naming')}
							class="flex items-center gap-1.5 px-3 py-1.5 bg-transparent text-text-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
						>
							<Plus size={13} strokeWidth={2} />
							Add passkey
						</button>
					{/if}
				</div>

				{#if credsError}
					<p class="text-error-light text-base mb-3">{credsError}</p>
				{/if}

				<!-- Add passkey panel -->
				{#if addStep === 'naming'}
					<div class="mb-3 p-4 border border-border-deep rounded-lg flex flex-col gap-3">
						<p class="text-text-body text-base m-0">Name your new passkey (optional):</p>
						<input
							type="text"
							bind:value={newName}
							placeholder="e.g. MacBook Touch ID"
							class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2 text-base text-text-body placeholder-muted-dim focus:outline-none focus:border-border-focus"
						/>
						{#if addError}
							<p class="text-error-light text-base m-0">{addError}</p>
						{/if}
						<div class="flex gap-2">
							<button
								onclick={handleAddPasskey}
								class="px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
							>
								Continue
							</button>
							<button
								onclick={() => { addStep = 'idle'; addError = null; newName = ''; }}
								class="px-4 py-2 bg-transparent text-muted-dim border border-border-subtle rounded cursor-pointer font-mono text-base hover:text-muted-blue hover:border-border transition-colors duration-100"
							>
								Cancel
							</button>
						</div>
					</div>
				{:else if addStep === 'reauth'}
					<div class="mb-3 p-4 border border-border-deep rounded-lg">
						<p class="text-muted-dim text-base animate-pulse m-0">Verifying your existing passkey…</p>
					</div>
				{:else if addStep === 'registering'}
					<div class="mb-3 p-4 border border-border-deep rounded-lg">
						<p class="text-muted-dim text-base animate-pulse m-0">Registering new passkey…</p>
					</div>
				{/if}

				{#if credsLoading}
					<p class="text-muted-dim text-base">Loading…</p>
				{:else if credentials.length === 0}
					<div class="py-10 border border-dashed border-border rounded-lg text-center">
						<p class="m-0 text-muted-dim text-base">No passkeys</p>
					</div>
				{:else}
					<div class="border border-border-deep rounded-lg overflow-hidden">
						{#each credentials as cred, i (cred.id)}
							<div class="flex items-center gap-3 px-4 py-3.5
								{i < credentials.length - 1 ? 'border-b border-border-deep' : ''}">
								<div class="text-muted-dim shrink-0">
									<KeyRound size={16} strokeWidth={1.75} />
								</div>

								<div class="flex-1 min-w-0">
									{#if editingId === cred.id}
										<input
											type="text"
											bind:value={editingName}
											onkeydown={(e) => {
												if (e.key === 'Enter') saveEdit(cred.id);
												if (e.key === 'Escape') cancelEdit();
											}}
											class="font-mono bg-surface-input border border-surface-3 rounded px-2 py-1 text-base text-text-body focus:outline-none w-full"
										/>
										<div class="flex gap-3 mt-1.5">
											<button
												onclick={() => saveEdit(cred.id)}
												disabled={saving}
												class="text-sm text-[#7aadcf] hover:text-[#a8d4ef] cursor-pointer bg-transparent border-none font-mono p-0"
											>
												{saving ? 'Saving…' : 'Save'}
											</button>
											<button
												onclick={cancelEdit}
												class="text-sm text-muted-dim hover:text-muted-blue cursor-pointer bg-transparent border-none font-mono p-0"
											>
												Cancel
											</button>
										</div>
									{:else}
										<div class="flex items-center gap-2 flex-wrap">
											<span class="text-base text-text-body">{cred.name || 'Unnamed passkey'}</span>
											{#if cred.isCurrentSession}
												<span class="text-[10px] px-1.5 py-0.5 bg-[#0d2a1a] border border-border-success-dark rounded text-[#4a9060] leading-none shrink-0">
													This session
												</span>
											{/if}
											{#if cred.backupEligible}
												<span title="Backup eligible" class="text-[#3a7a5a] shrink-0">
													<ShieldCheck size={12} strokeWidth={2} />
												</span>
											{/if}
										</div>
										<p class="m-0 text-sm text-muted-dim mt-0.5">Added {formatDate(cred.createdAt)}</p>
									{/if}
								</div>

								<div class="flex items-center gap-0.5 shrink-0">
									{#if editingId !== cred.id}
										<button
											onclick={() => startEdit(cred)}
											title="Rename"
											class="p-2 text-muted-mid hover:text-muted-blue transition-colors cursor-pointer bg-transparent border-none rounded"
										>
											<Pencil size={14} strokeWidth={1.75} />
										</button>
									{/if}

									{#if confirmDeleteId === cred.id}
										<div class="flex items-center gap-1.5 ml-1">
											<span class="text-sm text-muted-dim">Delete?</span>
											<button
												onclick={() => confirmDelete(cred.id)}
												disabled={deletingId === cred.id}
												class="px-2 py-1 border border-border-danger-dark rounded text-sm text-error-light hover:bg-danger-hover transition-colors cursor-pointer bg-transparent font-mono"
											>
												{deletingId === cred.id ? '…' : 'Yes'}
											</button>
											<button
												onclick={cancelDelete}
												class="px-2 py-1 border border-border-subtle rounded text-sm text-muted-dim hover:text-muted-blue transition-colors cursor-pointer bg-transparent font-mono"
											>
												No
											</button>
										</div>
									{:else}
										<button
											onclick={() => promptDelete(cred.id)}
											disabled={credentials.length <= 1}
											title={credentials.length <= 1 ? 'You must have at least one passkey' : 'Delete passkey'}
											class="p-2 transition-colors cursor-pointer bg-transparent border-none rounded
												{credentials.length <= 1
													? 'text-muted-mid cursor-not-allowed'
													: 'text-muted-mid hover:text-error-light'}"
										>
											<Trash2 size={14} strokeWidth={1.75} />
										</button>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Recovery -->
			<div>
				<div class="flex items-center justify-between mb-3">
					<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">
						<span class="inline-flex items-center gap-2">
							<ShieldAlert size={14} strokeWidth={1.75} />
							Recovery
						</span>
					</h2>
				</div>

				<div class="border border-border-deep rounded-lg px-4 py-4 flex items-center justify-between gap-4">
					<div>
						<p class="m-0 text-base text-text-body">Recovery codes</p>
						<p class="m-0 mt-0.5 text-sm text-muted-dim">Use a recovery code to regain access if you lose all your passkeys.</p>
					</div>
					<button
						onclick={() => { recoveryDialogOpen = true; codeGenError = null; }}
						class="shrink-0 px-4 py-2 bg-transparent text-muted-dim border border-border-deep rounded
							cursor-pointer font-mono text-base hover:text-text-body hover:border-border-subtle
							transition-colors duration-100"
					>
						View
					</button>
				</div>
			</div>

		</div>
	</div>

	<!-- ─── Danger zone ─────────────────────────────────────────────────────── -->
	<div class="mt-10 pt-8 border-t border-border-deep">
		<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Danger zone</h2>
		<div class="border border-border-danger-deep rounded-lg px-4 py-4 flex items-center justify-between gap-4 max-w-2xl">
			<div>
				<p class="m-0 text-base text-text-body">Delete account</p>
				<p class="m-0 mt-0.5 text-sm text-muted-dim">Permanently delete your account and all its data.</p>
			</div>
			<button
				onclick={() => { showDeleteAccountConfirm = true; deleteAccountError = ''; }}
				class="shrink-0 px-4 py-2 bg-transparent text-error-light border border-border-danger-dark rounded
					cursor-pointer font-mono text-base hover:bg-danger-bg-dark transition-colors duration-100"
			>
				Delete account
			</button>
		</div>
	</div>

</div>
</div>
