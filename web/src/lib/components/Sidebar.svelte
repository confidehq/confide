<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { ChevronLeft, ChevronRight, MessageSquare, LogOut, LayoutGrid, FileText, Users, UserRound, Settings, Sun, Moon } from '@lucide/svelte';
	import { logout } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { theme } from '$lib/stores/theme.svelte';
	import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte';

	let version = $state('dev');
	let commit = $state('');
	let accountMenuOpen = $state(false);
	let accountButtonEl = $state<HTMLButtonElement | null>(null);
	let popoverStyle = $state('');

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
			'flex items-center h-10 no-underline overflow-hidden pl-[14px]',
			'box-border w-full transition-[color,background] duration-100 border-l-2',
			active
				? 'text-text-bright bg-surface border-primary-hover'
				: 'text-muted-dark bg-transparent border-transparent hover:text-muted'
		].join(' ');
	}

	const textStyle = $derived(
		sidebar.collapsed
			? 'max-width:0px;opacity:0;margin-left:0px'
			: 'max-width:200px;opacity:1;margin-left:10px'
	);
	const textClass = 'overflow-hidden whitespace-nowrap transition-[max-width,opacity,margin-left] duration-200 ease-linear';

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
	import { tooltip } from '$lib/actions/tooltip';

	$effect(() => {
		if (accountMenuOpen && accountButtonEl) {
			const rect = accountButtonEl.getBoundingClientRect();
			popoverStyle = [
				`bottom: ${window.innerHeight - rect.top + 6}px`,
				`left: ${rect.left + 8}px`,
				`width: ${Math.max(rect.width - 16, 160)}px`,
			].join('; ');
		}
	});

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

<!-- Account popover + click-outside (rendered outside nav to escape overflow-hidden) -->
{#if accountMenuOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-[45]" onclick={() => (accountMenuOpen = false)}></div>
	<div
		style={popoverStyle}
		class="fixed z-50 bg-surface-input border border-border-mid rounded-md shadow-[0_2px_12px_var(--color-overlay-light)] overflow-hidden"
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

<nav
	style="width: {sidebar.width}px;"
	class="hidden sm:flex sm:flex-col fixed top-0 left-0 h-screen bg-canvas z-40 overflow-hidden transition-[width] duration-200 ease-linear font-mono"
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
				use:tooltip={sidebar.collapsed ? 'Dashboard' : null}
				onclick={() => sidebar.closeMobile()}
				class={linkClass(isActive('/dashboard'))}
			>
				<span class="shrink-0 flex items-center {isActive('/dashboard') ? 'text-text-blue' : 'text-muted-dark'}">
					<LayoutGrid size={18} strokeWidth={1.75} />
				</span>
				<span class={textClass} style={textStyle}>Dashboard</span>
			</a>

			<!-- Forms -->
			<a
				href="/forms"
				use:tooltip={sidebar.collapsed ? 'Forms' : null}
				onclick={() => sidebar.closeMobile()}
				class={linkClass(isActive('/forms'))}
			>
				<span class="shrink-0 flex items-center {isActive('/forms') ? 'text-text-blue' : 'text-muted-dark'}">
					<FileText size={18} strokeWidth={1.75} />
				</span>
				<span class={textClass} style={textStyle}>Forms</span>
			</a>

			<!-- Team -->
			<a
				href="/team"
				use:tooltip={sidebar.collapsed ? 'Team' : null}
				onclick={() => sidebar.closeMobile()}
				class={linkClass(isActive('/team'))}
			>
				<span class="shrink-0 flex items-center {isActive('/team') ? 'text-text-blue' : 'text-muted-dark'}">
					<Users size={18} strokeWidth={1.75} />
				</span>
				<span class={textClass} style={textStyle}>Team</span>
			</a>

			<!-- Settings -->
			<a
				href="/settings"
				use:tooltip={sidebar.collapsed ? 'Settings' : null}
				onclick={() => sidebar.closeMobile()}
				class={linkClass(isActive('/settings'))}
			>
				<span class="shrink-0 flex items-center {isActive('/settings') ? 'text-text-blue' : 'text-muted-dark'}">
					<Settings size={18} strokeWidth={1.75} />
				</span>
				<span class={textClass} style={textStyle}>Settings</span>
			</a>

		</div>

		<!-- Bottom links + account -->
		<div class="pb-1">
			<!-- Theme toggle -->
			<button
				onclick={() => theme.toggle()}
				use:tooltip={sidebar.collapsed ? (theme.value === 'dark' ? 'Light mode' : 'Dark mode') : null}
				class="flex items-center h-10 w-full bg-transparent border-none border-l-2 border-transparent pl-[14px]
					cursor-pointer font-mono text-muted-dark overflow-hidden
					hover:text-muted transition-colors duration-100"
			>
				<span class="shrink-0 flex items-center text-muted-dark">
					{#if theme.value === 'dark'}
						<Sun size={18} strokeWidth={1.75} />
					{:else}
						<Moon size={18} strokeWidth={1.75} />
					{/if}
				</span>
				<span class="{textClass} text-sm" style={textStyle}>
					{theme.value === 'dark' ? 'Light mode' : 'Dark mode'}
				</span>
			</button>

			<a
				href="https://feedback.useconfide.app/"
				use:tooltip={sidebar.collapsed ? 'Request Feature' : null}
				target="_blank"
				rel="noopener noreferrer"
				onclick={() => sidebar.closeMobile()}
				class="flex items-center h-10 no-underline text-muted-dark bg-transparent pl-[14px]
					border-l-2 border-transparent overflow-hidden
					box-border w-full hover:text-muted transition-colors duration-100"
			>
				<span class="shrink-0 flex items-center text-muted-dark">
					<MessageSquare size={18} strokeWidth={1.75} />
				</span>
				<span class={textClass} style={textStyle}>Request Feature</span>
			</a>

			<!-- Account button -->
			<div>
				<button
					bind:this={accountButtonEl}
					onclick={() => (accountMenuOpen = !accountMenuOpen)}
					use:tooltip={sidebar.collapsed ? 'My Account' : null}
					class="flex items-center gap-0 w-full bg-transparent border-none cursor-pointer font-mono
						transition-colors duration-100 border-t border-surface px-[10px] py-3
						{isActive('/me') ? 'bg-surface' : 'hover:bg-surface'}"
				>
					<span class="shrink-0 w-8 h-8 rounded-full flex items-center justify-center bg-surface-deep border border-border
						{isActive('/me') ? 'text-text-blue border-primary' : 'text-muted-dim'}">
						<UserRound size={17} strokeWidth={1.5} />
					</span>
					<span
						class="overflow-hidden whitespace-nowrap transition-[max-width,opacity,margin-left] duration-200 ease-linear text-sm font-medium text-text-body leading-tight"
						style={textStyle}
					>My Account</span>
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
