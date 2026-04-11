<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { onMount } from 'svelte';
	import { LayoutGrid, Settings, ChevronLeft, ChevronRight, ChevronDown, MessageSquare, LogOut, Building2 } from '@lucide/svelte';
	import { logout } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { listWorkspaces, type Workspace } from '$lib/workspaces';

	let version = $state('dev');
	let commit = $state('');
	let workspaces = $state<Workspace[]>([]);
	let workspacesOpen = $state(true);

	async function handleLogout() {
		await logout();
		auth.clearAll();
		goto('/login');
	}

	onMount(async () => {
		try {
			const res = await fetch('/api/health');
			if (res.ok) {
				const data = await res.json();
				version = data.version ?? 'dev';
				commit = data.commit ?? '';
			}
		} catch { /* leave defaults */ }

		try {
			workspaces = await listWorkspaces();
		} catch { /* non-fatal */ }
	});

	function isActive(href: string): boolean {
		const path = $page.url.pathname;
		return path === href || path.startsWith(href + '/');
	}

	const workspacesActive = $derived(
		$page.url.pathname === '/workspaces' || $page.url.pathname.startsWith('/workspaces/')
	);

	// The parent Workspaces row toggles open/close; keep it open when on a workspace sub-page
	$effect(() => {
		if ($page.url.pathname.startsWith('/workspaces/')) workspacesOpen = true;
	});

	// Shared link row styles
	function linkClass(active: boolean): string {
		return [
			'flex items-center gap-2.5 h-10 no-underline whitespace-nowrap overflow-hidden',
			'text-sm box-border w-full transition-[color,background] duration-100 border-l-2',
			active
				? 'text-[#f9fafb] bg-surface-2 border-primary-hover'
				: 'text-muted-dark bg-transparent border-transparent hover:text-muted'
		].join(' ');
	}
</script>

<nav
	style="width: {sidebar.width}px;"
	class="fixed top-0 left-0 h-screen bg-canvas flex flex-col z-20 overflow-hidden transition-[width,transform] duration-200 ease-linear font-mono
		{sidebar.mobileOpen ? 'translate-x-0' : '-translate-x-full'} sm:translate-x-0"
>
	<!-- Logo / wordmark + toggle -->
	<div class="h-[52px] relative flex items-center justify-center shrink-0 border-b border-surface-2 overflow-hidden whitespace-nowrap">
		{#if sidebar.collapsed}
			<button
				onclick={() => sidebar.toggle()}
				title="Expand sidebar"
				class="bg-transparent border-none cursor-pointer text-muted-dark flex items-center p-1 rounded hover:text-muted transition-colors duration-100"
			>
				<ChevronRight size={16} strokeWidth={1.75} />
			</button>
		{:else}
			<span class="text-text-dim text-lg font-semibold tracking-tight select-none">confide</span>
			<button
				onclick={() => sidebar.toggle()}
				title="Collapse sidebar"
				class="absolute right-2 bg-transparent border-none cursor-pointer text-muted-dark flex items-center p-1 rounded shrink-0 hover:text-muted transition-colors duration-100"
			>
				<ChevronLeft size={16} strokeWidth={1.75} />
			</button>
		{/if}
	</div>

	<!-- Nav links -->
	<div class="flex-1 overflow-y-auto overflow-x-hidden flex flex-col justify-between">
		<div class="py-2">

			<!-- Dashboard -->
			<a
				href="/dashboard"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/dashboard'))}
			>
				<span class="shrink-0 flex items-center {isActive('/dashboard') ? 'text-[#93c5fd]' : 'text-muted-dark'}">
					<LayoutGrid size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Dashboard</span>
				{/if}
			</a>

			<!-- Workspaces (expandable) -->
			<div>
				<!-- Parent row -->
				<button
					onclick={() => {
						if (sidebar.collapsed) { sidebar.toggle(); return; }
						workspacesOpen = !workspacesOpen;
						goto('/workspaces');
						sidebar.closeMobile();
					}}
					style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
					class="flex items-center gap-2.5 h-10 whitespace-nowrap overflow-hidden w-full bg-transparent cursor-pointer font-mono
						text-sm box-border border-l-2 transition-[color,background] duration-100
						{workspacesActive
							? 'text-[#f9fafb] bg-surface-2 border-primary-hover'
							: 'text-muted-dark border-transparent hover:text-muted'}"
				>
					<span class="shrink-0 flex items-center {workspacesActive ? 'text-[#93c5fd]' : 'text-muted-dark'}">
						<Building2 size={18} strokeWidth={1.75} />
					</span>
					{#if !sidebar.collapsed}
						<span class="flex-1 text-left overflow-hidden text-ellipsis">Workspaces</span>
						<span class="shrink-0 flex items-center text-muted-dark transition-transform duration-150 {workspacesOpen ? 'rotate-0' : '-rotate-90'}">
							<ChevronDown size={14} strokeWidth={1.75} />
						</span>
					{/if}
				</button>

				<!-- Nested workspace items -->
				{#if workspacesOpen && !sidebar.collapsed}
					<div class="overflow-hidden">
						{#if workspaces.length === 0}
							<span class="flex items-center h-8 pl-[42px] pr-3 text-[#374d63] text-sm whitespace-nowrap overflow-hidden">
								No workspaces
							</span>
						{:else}
							{#each workspaces as ws (ws.id)}
								{@const wsActive = $page.url.pathname === `/workspaces/${ws.id}`}
								<a
									href="/workspaces/{ws.id}"
									onclick={() => sidebar.closeMobile()}
									class="flex items-center h-8 pl-[42px] pr-3 no-underline whitespace-nowrap overflow-hidden
										text-sm w-full transition-colors duration-100 border-l-2
										{wsActive
											? 'text-[#c5d3e0] bg-surface-2 border-primary-hover'
											: 'text-[#374d63] border-transparent hover:text-muted-dark'}"
								>
									<span class="truncate">{ws.name}</span>
								</a>
							{/each}
						{/if}
					</div>
				{/if}
			</div>

			<!-- Settings -->
			<a
				href="/settings/sessions"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/settings'))}
			>
				<span class="shrink-0 flex items-center {isActive('/settings') ? 'text-[#93c5fd]' : 'text-muted-dark'}">
					<Settings size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Settings</span>
				{/if}
			</a>

		</div>

		<!-- Bottom links + version -->
		<div class="pb-1">
			<a
				href="https://feedback.useconfide.app/"
				target="_blank"
				rel="noopener noreferrer"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class="flex items-center gap-2.5 h-10 no-underline text-muted-dark bg-transparent
					border-l-2 border-transparent whitespace-nowrap overflow-hidden
					text-sm box-border w-full hover:text-muted transition-colors duration-100"
			>
				<span class="shrink-0 flex items-center text-muted-dark">
					<MessageSquare size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Leave Feedback</span>
				{/if}
			</a>
			<button
				onclick={handleLogout}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class="flex items-center gap-2.5 h-10 w-full bg-transparent border-l-2 border-transparent
					whitespace-nowrap overflow-hidden text-sm box-border cursor-pointer font-mono
					text-[#f87171] hover:text-[#fca5a5] transition-colors duration-100"
			>
				<span class="shrink-0 flex items-center text-[#f87171]">
					<LogOut size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Sign out</span>
				{/if}
			</button>
			{#if !sidebar.collapsed}
				<div
					title={commit || undefined}
					class="py-2 pb-3 text-center text-border text-sm whitespace-nowrap overflow-hidden text-ellipsis cursor-default"
				>{version}</div>
			{/if}
		</div>
	</div>
</nav>
