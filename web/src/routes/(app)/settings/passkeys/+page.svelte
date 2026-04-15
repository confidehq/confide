<script lang="ts">
	import { onMount } from 'svelte';
	import { KeyRound, Plus, Pencil, Trash2, ShieldCheck } from '@lucide/svelte';
	import {
		listCredentials,
		renameCredential,
		deleteCredential,
		reauthenticateForAddCredential,
		addCredential
	} from '$lib/auth';
	import type { CredentialSummary } from '$lib/types/auth';

	import { auth } from '$lib/stores/auth.svelte';

	let credentials = $state<CredentialSummary[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Rename state
	let editingId = $state<string | null>(null);
	let editingName = $state('');
	let saving = $state(false);

	// Delete state
	let deletingId = $state<string | null>(null);
	let confirmDeleteId = $state<string | null>(null);

	// Add passkey state
	let addStep = $state<'idle' | 'naming' | 'reauth' | 'registering' | 'done'>('idle');
	let newName = $state('');
	let addError = $state<string | null>(null);

	onMount(async () => {
		await load();
	});

	async function load() {
		loading = true;
		error = null;
		try {
			credentials = await listCredentials();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load passkeys.';
		} finally {
			loading = false;
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
			error = err instanceof Error ? err.message : 'Failed to rename passkey.';
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
			error = err instanceof Error ? err.message : 'Failed to delete passkey.';
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
</script>

<svelte:head>
	<title>Confide — Passkeys</title>
</svelte:head>

<div class="font-mono max-w-7xl mx-auto px-4 pt-12 pb-12 sm:p-8 sm:pb-12">
	<div class="mb-7 flex items-center justify-between gap-4">
		<h1 class="text-2xl m-0 text-text-bright">Passkeys</h1>
		{#if addStep === 'idle'}
			<button
				onclick={() => (addStep = 'naming')}
				class="flex items-center gap-1.5 px-3 py-1.5 border border-border-deep rounded-md text-sm text-text-body hover:bg-surface-mid transition-colors cursor-pointer bg-transparent font-mono"
			>
				<Plus size={14} strokeWidth={2} />
				Add passkey
			</button>
		{/if}
	</div>

	{#if error}
		<p class="text-error-light text-sm mb-4">{error}</p>
	{/if}

	<!-- Add passkey panel -->
	{#if addStep === 'naming'}
		<div class="mb-6 p-4 border border-border-deep rounded-md flex flex-col gap-3">
			<p class="text-text-body text-sm m-0">Name your new passkey (optional):</p>
			<input
				type="text"
				bind:value={newName}
				placeholder="e.g. MacBook Touch ID"
				class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-1.5 text-sm text-text-body placeholder-muted-dim focus:outline-none focus:border-border-focus"
			/>
			{#if addError}
				<p class="text-error-light text-xs m-0">{addError}</p>
			{/if}
			<div class="flex gap-2">
				<button
					onclick={handleAddPasskey}
					class="px-3 py-1.5 bg-[#1a2f4a] border border-surface-3 rounded text-sm text-text-body hover:bg-[#1e3555] transition-colors cursor-pointer font-mono"
				>
					Continue
				</button>
				<button
					onclick={() => { addStep = 'idle'; addError = null; newName = ''; }}
					class="px-3 py-1.5 bg-transparent border border-border-subtle rounded text-sm text-muted-dim hover:text-muted-blue transition-colors cursor-pointer font-mono"
				>
					Cancel
				</button>
			</div>
		</div>
	{:else if addStep === 'reauth'}
		<div class="mb-6 p-4 border border-border-deep rounded-md">
			<p class="text-muted-blue text-sm animate-pulse m-0">Verifying your existing passkey…</p>
		</div>
	{:else if addStep === 'registering'}
		<div class="mb-6 p-4 border border-border-deep rounded-md">
			<p class="text-muted-blue text-sm animate-pulse m-0">Registering new passkey…</p>
		</div>
	{/if}

	<!-- Passkey list -->
	{#if loading}
		<p class="text-muted-blue text-base">Loading passkeys…</p>
	{:else if credentials.length === 0}
		<p class="text-muted-blue text-base">No passkeys found.</p>
	{:else}
		<div class="flex flex-col gap-1.5">
			{#each credentials as cred (cred.id)}
				<div class="flex items-start justify-between gap-3 px-4 py-3 border border-border-deep rounded-md">
					<div class="flex items-start gap-3 min-w-0">
						<div class="text-muted-dim shrink-0 mt-0.5">
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
									class="font-mono bg-surface-input border border-surface-3 rounded px-2 py-0.5 text-sm text-text-body focus:outline-none"
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
										class="text-xs text-muted-dim hover:text-muted-blue cursor-pointer bg-transparent border-none font-mono p-0"
									>
										Cancel
									</button>
								</div>
							{:else}
								<div class="flex items-center gap-2">
									<span class="text-text-body text-sm">
										{cred.name || 'Unnamed passkey'}
									</span>
									{#if cred.isCurrentSession}
										<span class="text-[10px] px-1.5 py-0.5 bg-[#0d2a1a] border border-border-success-dark rounded text-[#4a9060] leading-none">
											This session
										</span>
									{/if}
									{#if cred.backupEligible}
										<span title="Backup eligible" class="text-[#3a7a5a]">
											<ShieldCheck size={12} strokeWidth={2} />
										</span>
									{/if}
								</div>
								<div class="text-muted-dim text-xs mt-0.5">Added {formatDate(cred.createdAt)}</div>
							{/if}
						</div>
					</div>

					<div class="flex items-center gap-1.5 shrink-0">
						{#if editingId !== cred.id}
							<button
								onclick={() => startEdit(cred)}
								title="Rename"
								class="p-1.5 text-muted-dim hover:text-muted-blue transition-colors cursor-pointer bg-transparent border-none rounded"
							>
								<Pencil size={14} strokeWidth={1.75} />
							</button>
						{/if}

						{#if confirmDeleteId === cred.id}
							<div class="flex items-center gap-1.5">
								<span class="text-xs text-muted-blue">Delete?</span>
								<button
									onclick={() => confirmDelete(cred.id)}
									disabled={deletingId === cred.id}
									class="px-2 py-0.5 border border-border-danger-dark rounded text-xs text-error-light hover:bg-danger-hover transition-colors cursor-pointer bg-transparent font-mono"
								>
									{deletingId === cred.id ? '…' : 'Yes'}
								</button>
								<button
									onclick={cancelDelete}
									class="px-2 py-0.5 border border-border-subtle rounded text-xs text-muted-dim hover:text-muted-blue transition-colors cursor-pointer bg-transparent font-mono"
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
										? 'text-muted-mid cursor-not-allowed'
										: 'text-muted-dim hover:text-error-light'}"
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
