<script lang="ts">
	import { page } from '$app/stores';
	import { LayoutGrid, FileText, Users, Settings, UserRound } from '@lucide/svelte';

	function isActive(href: string): boolean {
		const path = $page.url.pathname;
		return path === href || path.startsWith(href + '/');
	}

	const items = [
		{ href: '/dashboard', label: 'Dashboard', icon: LayoutGrid },
		{ href: '/forms',     label: 'Forms',     icon: FileText },
		{ href: '/team',      label: 'Team',       icon: Users },
		{ href: '/settings',  label: 'Settings',   icon: Settings },
		{ href: '/me',        label: 'Account',    icon: UserRound },
	];
</script>

<nav
	style="padding-bottom: env(safe-area-inset-bottom, 0px);"
	class="sm:hidden fixed bottom-0 left-0 right-0 z-40 bg-canvas border-t border-surface font-mono"
>
	<div class="flex items-stretch h-16">
		{#each items as item}
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
	</div>
</nav>
