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

<nav
	style="width: {sidebar.width}px;"
	class="fixed top-0 left-0 h-screen bg-canvas flex flex-col z-20 overflow-hidden transition-[width,transform] duration-200 ease-linear font-mono
		{sidebar.mobileOpen ? 'translate-x-0' : '-translate-x-full'} sm:translate-x-0"
>
	<!-- Logo / wordmark + toggle -->
	<div
		style="padding: 0 {sidebar.collapsed ? 0 : 12}px; justify-content: {sidebar.collapsed ? 'center' : 'space-between'};"
		class="h-[52px] flex items-center shrink-0 border-b border-surface-2 overflow-hidden whitespace-nowrap gap-2"
	>
		{#if sidebar.collapsed}
			<button
				onclick={() => sidebar.toggle()}
				title="Expand sidebar"
				class="bg-transparent border-none cursor-pointer text-muted-dark flex items-center p-1 rounded hover:text-muted transition-colors duration-100"
			>
				<ChevronRight size={16} strokeWidth={1.75} />
			</button>
		{:else}
			<span class="text-text-dim text-[1.1rem] font-semibold tracking-tight">confide</span>
			<button
				onclick={() => sidebar.toggle()}
				title="Collapse sidebar"
				class="bg-transparent border-none cursor-pointer text-muted-dark flex items-center p-1 rounded shrink-0 hover:text-muted transition-colors duration-100"
			>
				<ChevronLeft size={16} strokeWidth={1.75} />
			</button>
		{/if}
	</div>

	<!-- Nav links -->
	<div class="flex-1 overflow-hidden flex flex-col justify-between">
		<div class="py-2">
			{#each links as link}
				{@const active = isActive(link.href)}
				<a
					href={link.href}
					onclick={() => sidebar.closeMobile()}
					style="padding: 0 {sidebar.collapsed ? 0 : 14}px; justify-content: {sidebar.collapsed ? 'center' : 'flex-start'};"
					class="flex items-center gap-2.5 h-10 no-underline whitespace-nowrap overflow-hidden
						text-[0.95rem] box-border w-full transition-[color,background] duration-100
						{active
							? 'text-[#f9fafb] bg-surface-2 border-l-2 border-primary-hover'
							: 'text-muted-dark bg-transparent border-l-2 border-transparent hover:text-muted'}"
				>
					<span class="shrink-0 flex items-center {active ? 'text-[#93c5fd]' : 'text-muted-dark'}">
						<svelte:component this={link.icon} size={18} strokeWidth={1.75} />
					</span>
					{#if !sidebar.collapsed}
						<span class="overflow-hidden text-ellipsis">{link.label}</span>
					{/if}
				</a>
			{/each}
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
					text-[0.95rem] box-border w-full hover:text-muted transition-colors duration-100"
			>
				<span class="shrink-0 flex items-center text-muted-dark">
					<MessageSquare size={18} strokeWidth={1.75} />
				</span>
				{#if !sidebar.collapsed}
					<span class="overflow-hidden text-ellipsis">Leave Feedback</span>
				{/if}
			</a>
			{#if !sidebar.collapsed}
				<div
					title={commit || undefined}
					class="py-2 pb-3 text-center text-border text-[0.925rem] whitespace-nowrap overflow-hidden text-ellipsis cursor-default"
				>{version}</div>
			{/if}
		</div>
	</div>
</nav>
