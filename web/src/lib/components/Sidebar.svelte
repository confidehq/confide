<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { Settings, ChevronLeft, ChevronRight, MessageSquare, LogOut, LayoutGrid, FileText, Users } from '@lucide/svelte';
	import { logout } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte';

	let version = $state('dev');
	let commit = $state('');

	async function handleLogout() {
		await logout();
		auth.clearAll();
		goto('/login');
	}

	function isActive(href: string): boolean {
		const path = $page.url.pathname;
		return path === href || path.startsWith(href + '/');
	}

	function linkClass(active: boolean): string {
		return [
			'flex items-center gap-2.5 h-10 no-underline whitespace-nowrap overflow-hidden',
			'text-sm box-border w-full transition-[color,background] duration-100 border-l-2',
			active
				? 'text-[#f9fafb] bg-surface-2 border-primary-hover'
				: 'text-muted-dark bg-transparent border-transparent hover:text-muted'
		].join(' ');
	}

	import { onMount } from 'svelte';

	onMount(async () => {
		try {
			const res = await fetch('/api/health');
			if (res.ok) {
				const data = await res.json();
				version = data.version ?? 'dev';
				commit = data.commit ?? '';
			}
		} catch { /* leave defaults */ }
	});
</script>

<nav
	style="width: {sidebar.width}px;"
	class="fixed top-0 left-0 h-screen bg-canvas flex flex-col z-20 overflow-hidden transition-[width,transform] duration-200 ease-linear font-mono
		{sidebar.mobileOpen ? 'translate-x-0' : '-translate-x-full'} sm:translate-x-0"
>
	<!-- Logo / wordmark + collapse toggle -->
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

	<!-- Workspace switcher -->
	<div class="shrink-0 border-b border-surface-2">
		<WorkspaceSwitcher />
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

			<!-- Forms -->
			<a
				href="/forms"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/forms'))}
			>
				<span class="shrink-0 flex items-center {isActive('/forms') ? 'text-[#93c5fd]' : 'text-muted-dark'}">
					<FileText size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Forms</span>
				{/if}
			</a>

			<!-- Team -->
			<a
				href="/team"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/team'))}
			>
				<span class="shrink-0 flex items-center {isActive('/team') ? 'text-[#93c5fd]' : 'text-muted-dark'}">
					<Users size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Team</span>
				{/if}
			</a>

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
