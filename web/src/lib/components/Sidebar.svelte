<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { ChevronLeft, ChevronRight, MessageSquare, LogOut, LayoutGrid, FileText, Users, UserRound, Settings } from '@lucide/svelte';
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
			'box-border w-full transition-[color,background] duration-100 border-l-2',
			active
				? 'text-text-bright bg-surface border-primary-hover'
				: 'text-muted-dark bg-transparent border-transparent hover:text-muted'
		].join(' ');
	}

	function accountInitials(): string {
		const name = auth.username ?? auth.accountId ?? '';
		return name.slice(0, 2).toUpperCase() || '??';
	}

	function accountLabel(): string {
		if (auth.username) return auth.username;
		const id = auth.accountId ?? '';
		return id ? id.slice(0, 12) + '…' : 'Account';
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
	<div class="h-[52px] relative flex items-center justify-center shrink-0 border-b border-surface overflow-hidden whitespace-nowrap">
		{#if sidebar.collapsed}
			<button
				onclick={() => sidebar.toggle()}
				title="Expand sidebar"
				class="bg-transparent border-none cursor-pointer text-muted-dark flex items-center p-1 rounded hover:text-muted transition-colors duration-100"
			>
				<ChevronRight size={16} strokeWidth={1.75} />
			</button>
		{:else}
			<span class="text-text-dim text-xl font-semibold tracking-tight select-none">confide</span>
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
	<div class="shrink-0 border-b border-surface">
		<WorkspaceSwitcher />
	</div>

	<!-- Nav links -->
	<div class="flex-1 overflow-y-auto overflow-x-hidden flex flex-col justify-between">
		<div class="py-2">

			<!-- Dashboard -->
			<a
				href="/dashboard"
				title="Dashboard"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/dashboard'))}
			>
				<span class="shrink-0 flex items-center {isActive('/dashboard') ? 'text-text-blue' : 'text-muted-dark'}">
					<LayoutGrid size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Dashboard</span>
				{/if}
			</a>

			<!-- Forms -->
			<a
				href="/forms"
				title="Forms"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/forms'))}
			>
				<span class="shrink-0 flex items-center {isActive('/forms') ? 'text-text-blue' : 'text-muted-dark'}">
					<FileText size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Forms</span>
				{/if}
			</a>

			<!-- Team -->
			<a
				href="/team"
				title="Team"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/team'))}
			>
				<span class="shrink-0 flex items-center {isActive('/team') ? 'text-text-blue' : 'text-muted-dark'}">
					<Users size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Team</span>
				{/if}
			</a>

			<!-- Settings -->
			<a
				href="/settings"
				title="Settings"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class={linkClass(isActive('/settings'))}
			>
				<span class="shrink-0 flex items-center {isActive('/settings') ? 'text-text-blue' : 'text-muted-dark'}">
					<Settings size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Settings</span>
				{/if}
			</a>

		</div>

		<!-- Bottom links + account -->
		<div class="pb-1">
			<a
				href="https://feedback.useconfide.app/"
				title="Leave Feedback"
				target="_blank"
				rel="noopener noreferrer"
				onclick={() => sidebar.closeMobile()}
				style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
				class="flex items-center gap-2.5 h-10 no-underline text-muted-dark bg-transparent
					border-l-2 border-transparent whitespace-nowrap overflow-hidden
					box-border w-full hover:text-muted transition-colors duration-100"
			>
				<span class="shrink-0 flex items-center text-muted-dark">
					<MessageSquare size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Leave Feedback</span>
				{/if}
			</a>

			<!-- Account button + popover -->
			<div class="relative">
				{#if accountMenuOpen}
					<div
						class="absolute bottom-full left-2 right-2 mb-1.5 bg-surface-input border border-border-mid rounded-md shadow-lg overflow-hidden z-50"
					>
						<a
							href="/me"
							onclick={() => { accountMenuOpen = false; sidebar.closeMobile(); }}
							class="flex items-center gap-2.5 px-3 py-2.5 text-text-body hover:bg-surface-mid no-underline transition-colors duration-100 w-full"
						>
							<UserRound size={15} strokeWidth={1.75} class="text-muted-dim shrink-0" />
							Profile
						</a>
						<div class="border-t border-border-mid"></div>
						<button
							onclick={handleLogout}
							class="flex items-center gap-2.5 px-3 py-2.5 text-error-light hover:bg-danger-hover w-full bg-transparent border-none cursor-pointer font-mono transition-colors duration-100"
						>
							<LogOut size={15} strokeWidth={1.75} class="shrink-0" />
							Sign out
						</button>
					</div>
				{/if}

				<button
					onclick={() => (accountMenuOpen = !accountMenuOpen)}
					title="My Account"
					style="justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
					class="flex items-center gap-3 w-full bg-transparent border-none cursor-pointer font-mono
						transition-colors duration-100 border-t border-surface
						{sidebar.collapsed ? 'p-3' : 'px-3 py-3'}
						{isActive('/me') ? 'bg-surface' : 'hover:bg-surface'}"
				>
					<span class="shrink-0 w-8 h-8 rounded-full flex items-center justify-center bg-surface-deep border border-border
						{isActive('/me') ? 'text-text-blue border-primary' : 'text-muted-dim'}">
						<UserRound size={17} strokeWidth={1.5} />
					</span>
					{#if !sidebar.collapsed}
						<span class="text-sm font-medium text-text-body leading-tight">My Account</span>
					{/if}
				</button>
			</div>

			{#if !sidebar.collapsed}
				<div
					title={commit || undefined}
					class="py-2 pb-3 text-center text-border whitespace-nowrap overflow-hidden text-ellipsis cursor-default"
				>{version}</div>
			{/if}
		</div>
	</div>
</nav>
