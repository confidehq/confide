<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { ChevronLeft, ChevronRight, MessageSquare, LogOut, LayoutGrid, FileText, Users, User } from '@lucide/svelte';
	import { logout } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte';

	let version = $state('dev');
	let commit = $state('');
	let accountMenuOpen = $state(false);

	async function handleLogout() {
		accountMenuOpen = false;
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

	function accountInitials(): string {
		const id = auth.accountId ?? '';
		return id.slice(0, 2).toUpperCase() || '??';
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

<!-- Click-outside overlay -->
{#if accountMenuOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-30" onclick={() => (accountMenuOpen = false)}></div>
{/if}

<nav
	style="width: {sidebar.width}px;"
	class="fixed top-0 left-0 h-screen bg-canvas flex flex-col z-40 overflow-hidden transition-[width,transform] duration-200 ease-linear font-mono
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

		</div>

		<!-- Bottom links + account -->
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

			<!-- Account button + dropdown -->
			<div class="relative">
				{#if accountMenuOpen}
					<div
						class="absolute bottom-full left-2 right-2 mb-1.5 bg-[#0d1520] border border-[#1e3347] rounded-md shadow-lg overflow-hidden z-50"
					>
						<a
							href="/me"
							onclick={() => { accountMenuOpen = false; sidebar.closeMobile(); }}
							class="flex items-center gap-2.5 px-3 py-2.5 text-sm text-[#c5d3e0] hover:bg-[#1a2535] no-underline transition-colors duration-100 w-full"
						>
							<User size={15} strokeWidth={1.75} class="text-[#4b6280] shrink-0" />
							Profile
						</a>
						<div class="border-t border-[#1e3347]"></div>
						<button
							onclick={handleLogout}
							class="flex items-center gap-2.5 px-3 py-2.5 text-sm text-[#f87171] hover:bg-[#1a0e0e] w-full bg-transparent border-none cursor-pointer font-mono transition-colors duration-100"
						>
							<LogOut size={15} strokeWidth={1.75} class="shrink-0" />
							Sign out
						</button>
					</div>
				{/if}

				<button
					onclick={() => (accountMenuOpen = !accountMenuOpen)}
					style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
					class="flex items-center gap-2.5 h-10 w-full bg-transparent border-none border-l-2 border-transparent
						whitespace-nowrap overflow-hidden text-sm box-border cursor-pointer font-mono
						text-muted-dark hover:text-muted transition-colors duration-100
						{isActive('/me') ? 'text-[#f9fafb] bg-surface-2 border-l-2 border-primary-hover' : ''}"
				>
					<span
						class="shrink-0 w-[18px] h-[18px] rounded flex items-center justify-center text-[9px] font-semibold leading-none
							{isActive('/me') ? 'bg-[#1e3a5f] text-[#93c5fd]' : 'bg-[#0f1e30] text-[#4b6280]'}"
					>
						{accountInitials()}
					</span>
					{#if !sidebar.collapsed}
						<span class="overflow-hidden text-ellipsis">
							{auth.accountId ? auth.accountId.slice(0, 12) + '…' : 'Account'}
						</span>
					{/if}
				</button>
			</div>

			{#if !sidebar.collapsed}
				<div
					title={commit || undefined}
					class="py-2 pb-3 text-center text-border text-sm whitespace-nowrap overflow-hidden text-ellipsis cursor-default"
				>{version}</div>
			{/if}
		</div>
	</div>
</nav>
