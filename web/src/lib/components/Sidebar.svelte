<script lang="ts">
	import { page } from '$app/stores';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { onMount } from 'svelte';
	import type { Component } from 'svelte';
	import { LayoutGrid, FileText, Settings, ChevronLeft, ChevronRight, MessageSquare } from '@lucide/svelte';

	const links: Array<{ href: string; label: string; icon: Component }> = [
		{ href: '/dashboard', label: 'Dashboard', icon: LayoutGrid },
		{ href: '/forms', label: 'Forms', icon: FileText },
		{ href: '/settings/sessions', label: 'Settings', icon: Settings }
	];

	let version = $state('dev');
	let commit = $state('');

	onMount(async () => {
		try {
			const res = await fetch('/api/health');
			if (res.ok) {
				const data = await res.json();
				version = data.version ?? 'dev';
				commit = data.commit ?? '';
			}
		} catch {
			// leave defaults
		}
	});

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
				<ChevronRight size={16} strokeWidth={1.75} />
			</button>
		{:else}
			<span style="color: #d1d5db; font-size: 1.1rem; font-weight: 600; letter-spacing: -0.02em;">confide</span>
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
				<ChevronLeft size={16} strokeWidth={1.75} />
			</button>
		{/if}
	</div>

	<!-- Nav links -->
	<div style="flex: 1; overflow: hidden; display: flex; flex-direction: column; justify-content: space-between;">
		<div style="padding: 8px 0;">
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
						font-size: 0.95rem;
						box-sizing: border-box;
						width: 100%;
					"
				>
					<span style="flex-shrink: 0; display: flex; align-items: center; color: {active ? '#93c5fd' : '#4b5563'};">
						<svelte:component this={link.icon} size={18} strokeWidth={1.75} />
					</span>
					{#if !sidebar.collapsed}
						<span style="overflow: hidden; text-overflow: ellipsis;">{link.label}</span>
					{/if}
				</a>
			{/each}
		</div>

		<!-- Bottom links + version -->
		<div style="padding-bottom: 4px;">
			<a
				href="https://feedback.useconfide.app/"
				target="_blank"
				rel="noopener noreferrer"
				style="
					display: flex;
					align-items: center;
					gap: 10px;
					padding: 0 {sidebar.collapsed ? 0 : 14}px;
					height: 40px;
					justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};
					text-decoration: none;
					color: #6b7280;
					background: transparent;
					border-left: 2px solid transparent;
					white-space: nowrap;
					overflow: hidden;
					transition: color 120ms;
					font-size: 0.95rem;
					box-sizing: border-box;
					width: 100%;
				"
			>
				<span style="flex-shrink: 0; display: flex; align-items: center; color: #4b5563;">
					<MessageSquare size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span style="overflow: hidden; text-overflow: ellipsis;">Leave Feedback</span>
				{/if}
			</a>
			{#if !sidebar.collapsed}
				<div
					title={commit || undefined}
					style="
						padding: 8px 0 12px;
						text-align: center;
						color: #374151;
						font-size: 0.925rem;
						white-space: nowrap;
						overflow: hidden;
						text-overflow: ellipsis;
						cursor: default;
					"
				>{version}</div>
			{/if}
		</div>
	</div>

</nav>
