<script lang="ts">
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { createWorkspace, createProWorkspace, WorkspaceError } from '$lib/workspaces';
	import { ChevronsUpDown, Check, Plus } from '@lucide/svelte';
	import { getAppConfig } from '$lib/config';
	import { isManagedEdition } from '$lib/utils/plan';

	let dropdownOpen = $state(false);
	let showCreate = $state(false);
	let newName = $state('');
	let creating = $state(false);
	let createError = $state('');

	// Fixed-position dropdown coords (escapes sidebar's overflow-hidden)
	let triggerEl = $state<HTMLButtonElement | null>(null);
	let dropdownPos = $state({ left: 0, top: 0, width: 0 });

	let managed = $state(true);
	getAppConfig().then((c) => { managed = isManagedEdition(c.edition); });

	// True when the user already owns a free workspace and must upgrade to create another.
	const atFreeLimit = $derived(
		managed && workspacesStore.workspaces.some(w => w.plan === 'free' && w.role === 'owner')
	);

	function initials(name: string): string {
		const words = name.trim().split(/\s+/);
		if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
		return (words[0][0] + words[1][0]).toUpperCase();
	}

	function planLabel(plan: string, status: string): string {
		if (plan === 'pro') {
			if (status === 'past_due') return 'past due';
			if (status === 'canceled') return 'canceled';
			if (status === 'canceling') return 'cancels at period end';
			return 'pro';
		}
		return 'free';
	}

	function openDropdown() {
		if (sidebar.collapsed) {
			sidebar.toggle();
			return;
		}
		if (dropdownOpen) {
			closeDropdown();
			return;
		}
		if (triggerEl) {
			const rect = triggerEl.getBoundingClientRect();
			dropdownPos = { left: rect.left, top: rect.bottom + 4, width: rect.width };
		}
		dropdownOpen = true;
	}

	function closeDropdown() {
		dropdownOpen = false;
		showCreate = false;
		newName = '';
		createError = '';
	}

	function selectWorkspace(id: string) {
		workspacesStore.switchTo(id);
		closeDropdown();
	}

	async function handleCreate() {
		const name = newName.trim();
		if (!name || !auth.masterKey) return;
		creating = true;
		createError = '';
		try {
			if (atFreeLimit) {
				const { workspace, checkoutUrl } = await createProWorkspace(
					name,
					auth.masterKey,
					`${window.location.origin}/settings?tab=billing&upgraded=true`,
					`${window.location.origin}/forms`
				);
				workspacesStore.add(workspace);
				if (checkoutUrl) {
					window.location.href = checkoutUrl;
				} else {
					closeDropdown();
				}
			} else {
				const ws = await createWorkspace(name, auth.masterKey);
				workspacesStore.add(ws);
				closeDropdown();
			}
		} catch (e) {
			createError = e instanceof Error ? e.message : 'Failed to create workspace.';
		} finally {
			creating = false;
		}
	}
</script>

<!-- Click-outside overlay -->
{#if dropdownOpen}
	<div
		class="fixed inset-0 z-40"
		onclick={closeDropdown}
		role="presentation"
	></div>
{/if}

<div class="px-2 py-1.5">
	{#if sidebar.collapsed}
		<!-- Collapsed: just show initials pill -->
		<button
			onclick={openDropdown}
			title={workspacesStore.active?.name ?? 'Workspace'}
			class="w-full flex items-center justify-center py-1"
		>
			<span class="w-8 h-8 rounded-md flex items-center justify-center bg-surface-deep border border-border text-text-blue text-sm font-semibold select-none">
				{workspacesStore.active ? initials(workspacesStore.active.name) : '…'}
			</span>
		</button>
	{:else}
		<!-- Expanded: workspace name + plan + chevrons -->
		<button
			bind:this={triggerEl}
			onclick={openDropdown}
			class="w-full flex items-center gap-2 px-2 py-1.5 rounded-md hover:bg-surface transition-colors duration-100 group text-left"
		>
			<span class="shrink-0 w-7 h-7 rounded-md flex items-center justify-center bg-surface-deep border border-border text-text-blue text-sm font-semibold select-none">
				{#if workspacesStore.loading && !workspacesStore.loaded}
					…
				{:else}
					{workspacesStore.active ? initials(workspacesStore.active.name) : '?'}
				{/if}
			</span>

			<span class="flex-1 min-w-0">
				<span class="block text-text-body truncate leading-tight font-medium">
					{workspacesStore.active?.name ?? (workspacesStore.loading ? 'Loading…' : 'No workspace')}
				</span>
				{#if workspacesStore.active}
					<span class="block text-sm leading-tight capitalize
						{workspacesStore.active.plan === 'pro'
							? workspacesStore.active.planStatus === 'active' || workspacesStore.active.planStatus === 'canceling'
								? 'text-success-text-dark'
								: 'text-error-light'
							: 'text-muted-dim'}">
						{planLabel(workspacesStore.active.plan, workspacesStore.active.planStatus)}
					</span>
				{/if}
			</span>

			<ChevronsUpDown
				size={13}
				strokeWidth={1.75}
				class="shrink-0 text-muted-mid group-hover:text-muted-dim transition-colors duration-100"
			/>
		</button>
	{/if}
</div>

<!-- Dropdown (fixed-position to escape sidebar overflow-hidden) -->
{#if dropdownOpen && !sidebar.collapsed}
	<div
		style="position: fixed; left: {dropdownPos.left}px; top: {dropdownPos.top}px; width: {dropdownPos.width}px;"
		class="z-50 bg-canvas border border-border rounded-lg shadow-[0_8px_32px_var(--color-overlay)] overflow-hidden"
	>
		{#if showCreate}
			<!-- Create workspace form -->
			<div class="p-3">
				<p class="m-0 mb-2.5 text-sm text-muted-mid uppercase tracking-widest font-medium">New workspace</p>
				{#if atFreeLimit}
					<p class="m-0 mb-2 text-sm text-muted-dim leading-relaxed">
						Additional workspaces require a Pro plan. You'll be taken to checkout after creation.
					</p>
				{/if}
				<input
					type="text"
					placeholder="Workspace name"
					bind:value={newName}
					disabled={creating}
					onkeydown={e => { if (e.key === 'Enter') handleCreate(); if (e.key === 'Escape') { showCreate = false; } }}
					class="input-base w-full px-3 py-2 mb-2"
					autofocus
				/>
				{#if createError}
					<p class="m-0 mb-2 text-sm text-error-muted">{createError}</p>
				{/if}
				<div class="flex gap-2">
					<button
						onclick={handleCreate}
						disabled={creating || !newName.trim()}
						class="flex-1 py-1.5 text-sm text-white border-none rounded cursor-pointer font-mono transition-colors duration-100
							{creating || !newName.trim() ? 'bg-muted-mid cursor-not-allowed' : 'bg-primary hover:bg-primary-hover'}"
					>{creating ? 'Creating…' : atFreeLimit ? 'Create & subscribe' : 'Create'}</button>
					<button
						onclick={() => { showCreate = false; createError = ''; newName = ''; }}
						class="px-3 py-1.5 text-sm text-muted-dim bg-transparent border border-border-deep rounded cursor-pointer font-mono hover:text-text-body hover:border-border-subtle transition-colors duration-100"
					>Cancel</button>
				</div>
			</div>

		{:else}
			<!-- Workspace list -->
			<div class="py-1 max-h-[220px] overflow-y-auto">
				{#if workspacesStore.workspaces.length === 0}
					<p class="px-3 py-2 text-sm text-muted-mid">No workspaces</p>
				{:else}
					{#each workspacesStore.workspaces as ws (ws.id)}
						{@const isActive = ws.id === workspacesStore.active?.id}
						<button
							onclick={() => selectWorkspace(ws.id)}
							class="w-full flex items-center gap-2.5 px-3 py-2 text-left bg-transparent border-none cursor-pointer font-mono transition-colors duration-100
								{isActive ? 'bg-surface-hover' : 'hover:bg-surface-hover'}"
						>
							<span class="shrink-0 w-6 h-6 rounded flex items-center justify-center bg-canvas border border-border text-muted-dim text-sm font-semibold">
								{initials(ws.name)}
							</span>
							<span class="flex-1 min-w-0 text-text-body truncate">{ws.name}</span>
							<span class="shrink-0 text-sm text-muted-mid capitalize">{ws.plan}</span>
							{#if isActive}
								<Check size={12} strokeWidth={2.5} class="shrink-0 text-text-blue" />
							{:else}
								<span class="shrink-0 w-3"></span>
							{/if}
						</button>
					{/each}
				{/if}
			</div>

			<!-- New workspace -->
			<div class="border-t border-border">
				<button
					onclick={() => { showCreate = true; }}
					class="w-full flex items-center gap-2 px-3 py-2.5 text-left bg-transparent border-none cursor-pointer font-mono text-sm text-muted-dim hover:text-text-body hover:bg-surface-hover transition-colors duration-100"
				>
					<Plus size={13} strokeWidth={1.75} class="shrink-0" />
					New workspace
				</button>
			</div>
		{/if}
	</div>
{/if}
