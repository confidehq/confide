<script lang="ts">
	import { page } from '$app/stores';
	import { LayoutGrid, FileText, Users, Menu, Settings, UserRound, Sun, Moon, X, Check, Plus, LogOut } from '@lucide/svelte';
	import { theme } from '$lib/stores/theme.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { logout } from '$lib/auth';
	import { goto } from '$app/navigation';
	import { createWorkspace, createProWorkspace } from '$lib/workspaces';

	let moreOpen = $state(false);
	let showWorkspacePicker = $state(false);
	let showCreateWorkspace = $state(false);
	let newWorkspaceName = $state('');
	let creating = $state(false);
	let createError = $state('');

	// True when the user already owns a free workspace and must upgrade to create another.
	const atFreeLimit = $derived(
		workspacesStore.workspaces.some(w => w.plan === 'free' && w.role === 'owner')
	);

	function isActive(href: string): boolean {
		const path = $page.url.pathname;
		return path === href || path.startsWith(href + '/');
	}

	let moreActive = $derived(isActive('/settings') || isActive('/me'));

	function openMore() {
		moreOpen = true;
		showWorkspacePicker = false;
		showCreateWorkspace = false;
	}

	function closeMore() {
		moreOpen = false;
		showWorkspacePicker = false;
		showCreateWorkspace = false;
		newWorkspaceName = '';
		createError = '';
	}

	function initials(name: string): string {
		const words = name.trim().split(/\s+/);
		if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
		return (words[0][0] + words[1][0]).toUpperCase();
	}

	function selectWorkspace(id: string) {
		workspacesStore.switchTo(id);
		closeMore();
	}

	async function handleCreate() {
		const name = newWorkspaceName.trim();
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
					closeMore();
				}
			} else {
				const ws = await createWorkspace(name, auth.masterKey);
				workspacesStore.add(ws);
				showCreateWorkspace = false;
				showWorkspacePicker = false;
				newWorkspaceName = '';
			}
		} catch (e) {
			createError = e instanceof Error ? e.message : 'Failed to create workspace.';
		} finally {
			creating = false;
		}
	}

	async function handleLogout() {
		closeMore();
		await logout();
		auth.clearAll();
		goto('/login');
	}

	const primaryItems = [
		{ href: '/dashboard', label: 'Dashboard', icon: LayoutGrid },
		{ href: '/forms',     label: 'Forms',     icon: FileText },
		{ href: '/team',      label: 'Team',       icon: Users },
	];
</script>

<!-- More sheet backdrop -->
{#if moreOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-40 bg-black/40"
		onclick={closeMore}
	></div>
{/if}

<!-- More bottom sheet -->
{#if moreOpen}
	<div
		style="padding-bottom: env(safe-area-inset-bottom, 0px);"
		class="sm:hidden fixed bottom-16 left-0 right-0 z-50 bg-canvas border-t border-border border-b border-border rounded-t-2xl font-mono
			animate-[slideUp_200ms_ease-out_both]"
	>
		<!-- Drag handle -->
		<div class="flex justify-center pt-2.5 pb-1">
			<div class="w-9 h-1 rounded-full bg-border-mid"></div>
		</div>

		<!-- Workspace row -->
		<div class="px-4 py-2 border-b border-surface">
			{#if showWorkspacePicker}
				<!-- Back header -->
				<div class="flex items-center gap-2 mb-3">
					<button
						onclick={() => { showWorkspacePicker = false; showCreateWorkspace = false; createError = ''; newWorkspaceName = ''; }}
						class="p-1 -ml-1 bg-transparent border-none cursor-pointer text-muted-dark hover:text-muted transition-colors duration-100"
					>
						<X size={16} strokeWidth={2} />
					</button>
					<span class="text-xs uppercase tracking-widest text-muted-mid font-medium">Workspaces</span>
				</div>

				{#if showCreateWorkspace}
					<div class="mb-3">
						<p class="m-0 mb-2 text-xs uppercase tracking-widest text-muted-mid font-medium">New workspace</p>
						{#if atFreeLimit}
							<p class="m-0 mb-2 text-sm text-muted-dim leading-relaxed">
								Additional workspaces require a Pro plan. You'll be taken to checkout after creation.
							</p>
						{/if}
						<input
							type="text"
							placeholder="Workspace name"
							bind:value={newWorkspaceName}
							disabled={creating}
							onkeydown={e => { if (e.key === 'Enter') handleCreate(); if (e.key === 'Escape') { showCreateWorkspace = false; } }}
							class="input-base w-full px-3 py-2 mb-2"
							autofocus
						/>
						{#if createError}
							<p class="m-0 mb-2 text-sm text-error-muted">{createError}</p>
						{/if}
						<div class="flex gap-2">
							<button
								onclick={handleCreate}
								disabled={creating || !newWorkspaceName.trim()}
								class="flex-1 py-2 text-sm text-white border-none rounded cursor-pointer font-mono transition-colors duration-100
									{creating || !newWorkspaceName.trim() ? 'bg-muted-mid cursor-not-allowed' : 'bg-primary hover:bg-primary-hover'}"
							>{creating ? 'Creating…' : atFreeLimit ? 'Create & subscribe' : 'Create'}</button>
							<button
								onclick={() => { showCreateWorkspace = false; createError = ''; newWorkspaceName = ''; }}
								class="px-3 py-2 text-sm text-muted-dim bg-transparent border border-border-deep rounded cursor-pointer font-mono hover:text-text-body transition-colors duration-100"
							>Cancel</button>
						</div>
					</div>
				{:else}
					<!-- Workspace list -->
					<div class="max-h-40 overflow-y-auto -mx-1 mb-2">
						{#each workspacesStore.workspaces as ws (ws.id)}
							{@const active = ws.id === workspacesStore.active?.id}
							<button
								onclick={() => selectWorkspace(ws.id)}
								class="w-full flex items-center gap-2.5 px-2 py-2 text-left bg-transparent border-none cursor-pointer font-mono rounded-md transition-colors duration-100
									{active ? 'bg-surface-hover' : 'hover:bg-surface-hover'}"
							>
								<span class="shrink-0 w-7 h-7 rounded flex items-center justify-center bg-canvas border border-border text-text-blue text-sm font-semibold">
									{initials(ws.name)}
								</span>
								<span class="flex-1 min-w-0 text-text-body text-sm truncate">{ws.name}</span>
								<span class="shrink-0 text-xs text-muted-mid capitalize">{ws.plan}</span>
								{#if active}
									<Check size={13} strokeWidth={2.5} class="shrink-0 text-text-blue" />
								{/if}
							</button>
						{/each}
					</div>
					<button
						onclick={() => { showCreateWorkspace = true; }}
						class="flex items-center gap-2 w-full px-2 py-2 bg-transparent border border-dashed border-border-mid rounded-md cursor-pointer font-mono text-sm text-muted-dim hover:text-text-body hover:border-border-subtle transition-colors duration-100"
					>
						<Plus size={13} strokeWidth={1.75} />
						New workspace
					</button>
				{/if}
			{:else}
				<!-- Workspace trigger -->
				<button
					onclick={() => { showWorkspacePicker = true; }}
					class="w-full flex items-center gap-2.5 rounded-lg hover:bg-surface transition-colors duration-100 -mx-1 px-1 py-1"
				>
					<span class="shrink-0 w-8 h-8 rounded-md flex items-center justify-center bg-surface-deep border border-border text-text-blue text-sm font-semibold select-none">
						{workspacesStore.active ? initials(workspacesStore.active.name) : '…'}
					</span>
					<span class="flex-1 min-w-0 text-left">
						<span class="block text-sm text-text-body font-medium truncate leading-tight">
							{workspacesStore.active?.name ?? 'No workspace'}
						</span>
						<span class="block text-xs text-muted-dim leading-tight capitalize">
							{workspacesStore.active?.plan ?? ''}
						</span>
					</span>
					<svg class="shrink-0 text-muted-mid" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
						<path d="M7 15l5 5 5-5M7 9l5-5 5 5"/>
					</svg>
				</button>
			{/if}
		</div>

		<!-- Action rows -->
		{#if !showWorkspacePicker}
			<div class="py-1.5">
				<a
					href="/settings"
					onclick={closeMore}
					class="flex items-center gap-3 px-4 py-3 no-underline text-text-body hover:bg-surface transition-colors duration-100"
				>
					<Settings size={18} strokeWidth={1.75} class="shrink-0 text-muted-dark" />
					<span class="text-sm">Settings</span>
				</a>

				<a
					href="/me"
					onclick={closeMore}
					class="flex items-center gap-3 px-4 py-3 no-underline text-text-body hover:bg-surface transition-colors duration-100"
				>
					<UserRound size={18} strokeWidth={1.75} class="shrink-0 text-muted-dark" />
					<span class="text-sm">Account</span>
				</a>

				<button
					onclick={() => theme.toggle()}
					class="flex items-center gap-3 px-4 py-3 w-full bg-transparent border-none cursor-pointer font-mono text-text-body hover:bg-surface transition-colors duration-100"
				>
					{#if theme.value === 'dark'}
						<Sun size={18} strokeWidth={1.75} class="shrink-0 text-muted-dark" />
						<span class="text-sm">Light mode</span>
					{:else}
						<Moon size={18} strokeWidth={1.75} class="shrink-0 text-muted-dark" />
						<span class="text-sm">Dark mode</span>
					{/if}
				</button>

				<div class="border-t border-surface mx-4 my-1"></div>

				<button
					onclick={handleLogout}
					class="flex items-center gap-3 px-4 py-3 w-full bg-transparent border-none cursor-pointer font-mono text-error-light hover:bg-danger-hover transition-colors duration-100"
				>
					<LogOut size={18} strokeWidth={1.75} class="shrink-0" />
					<span class="text-sm">Sign out</span>
				</button>
			</div>
		{/if}
	</div>
{/if}

<!-- Bottom nav bar -->
<nav
	style="padding-bottom: env(safe-area-inset-bottom, 0px);"
	class="sm:hidden fixed bottom-0 left-0 right-0 z-40 bg-canvas border-t border-surface font-mono"
>
	<div class="flex items-stretch h-16">
		{#each primaryItems as item}
			{@const active = isActive(item.href)}
			{@const Icon = item.icon}
			<a
				href={item.href}
				class="flex-1 flex flex-col items-center justify-center gap-1 no-underline transition-colors duration-100
					{active ? 'text-text-blue' : 'text-muted-dark'}"
			>
				<Icon size={20} strokeWidth={1.75} />
				<span class="text-[10px] leading-none">{item.label}</span>
			</a>
		{/each}

		<!-- More button -->
		<button
			onclick={moreOpen ? closeMore : openMore}
			class="flex-1 flex flex-col items-center justify-center gap-1 bg-transparent border-none cursor-pointer font-mono transition-colors duration-100
				{moreOpen || moreActive ? 'text-text-blue' : 'text-muted-dark'}"
		>
			{#if moreOpen}
				<X size={20} strokeWidth={1.75} />
				<span class="text-[10px] leading-none">Close</span>
			{:else}
				<Menu size={20} strokeWidth={1.75} />
				<span class="text-[10px] leading-none">More</span>
			{/if}
		</button>
	</div>
</nav>

<style>
	@keyframes slideUp {
		from { transform: translateY(100%); opacity: 0; }
		to   { transform: translateY(0);    opacity: 1; }
	}
</style>
