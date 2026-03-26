<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';

	const links = [
		{
			href: '/dashboard',
			label: 'Dashboard',
			icon: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>`
		},
		{
			href: '/forms',
			label: 'Forms',
			icon: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="16" y2="17"/></svg>`
		},
		{
			href: '/settings/sessions',
			label: 'Settings',
			icon: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`
		}
	];

	function isActive(href: string): boolean {
		const path = $page.url.pathname;
		if (href === '/forms') {
			return path === '/forms' || (path.startsWith('/forms/') && !path.startsWith('/forms/new'));
		}
		return path === href || path.startsWith(href + '/');
	}
</script>

<nav style="
	position: fixed;
	top: 0; left: 0;
	height: 100vh;
	width: {sidebar.width}px;
	background: #111827;
	display: flex;
	flex-direction: column;
	z-index: 20;
	overflow: hidden;
	transition: width 200ms ease;
	font-family: monospace;
">
	<!-- Logo / wordmark + toggle -->
	<div style="
		height: 52px;
		display: flex;
		align-items: center;
		padding: 0 {sidebar.collapsed ? 0 : 12}px;
		justify-content: {sidebar.collapsed ? 'center' : 'space-between'};
		flex-shrink: 0;
		border-bottom: 1px solid #1f2937;
		overflow: hidden;
		white-space: nowrap;
		gap: 8px;
	">
		{#if sidebar.collapsed}
			<button
				onclick={() => sidebar.toggle()}
				title="Expand sidebar"
				style="
					background: transparent; border: none; cursor: pointer;
					color: #4b5563; display: flex; align-items: center;
					padding: 4px; border-radius: 4px;
					transition: color 120ms;
				"
			>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="9 18 15 12 9 6"/>
				</svg>
			</button>
		{:else}
			<span style="color: #d1d5db; font-size: 0.95rem; font-weight: 600; letter-spacing: -0.02em;">confide</span>
			<button
				onclick={() => sidebar.toggle()}
				title="Collapse sidebar"
				style="
					background: transparent; border: none; cursor: pointer;
					color: #4b5563; display: flex; align-items: center;
					padding: 4px; border-radius: 4px;
					transition: color 120ms; flex-shrink: 0;
				"
			>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="15 18 9 12 15 6"/>
				</svg>
			</button>
		{/if}
	</div>

	<!-- Nav links -->
	<div style="flex: 1; padding: 8px 0; overflow: hidden;">
		{#each links as link}
			{@const active = isActive(link.href)}
			<a
				href={link.href}
				style="
					display: flex;
					align-items: center;
					gap: 10px;
					padding: 0 {sidebar.collapsed ? 0 : 14}px;
					height: 40px;
					justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};
					text-decoration: none;
					color: {active ? '#f9fafb' : '#6b7280'};
					background: {active ? '#1f2937' : 'transparent'};
					border-left: 2px solid {active ? '#1d4ed8' : 'transparent'};
					white-space: nowrap;
					overflow: hidden;
					transition: color 120ms, background 120ms;
					font-size: 0.82rem;
					box-sizing: border-box;
					width: 100%;
				"
			>
				<span style="flex-shrink: 0; display: flex; align-items: center; color: {active ? '#93c5fd' : '#4b5563'};">
					{@html link.icon}
				</span>
				{#if !sidebar.collapsed}
					<span style="overflow: hidden; text-overflow: ellipsis;">{link.label}</span>
				{/if}
			</a>
		{/each}
	</div>

</nav>
