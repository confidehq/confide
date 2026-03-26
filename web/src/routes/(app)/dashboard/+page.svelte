<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { logout } from '$lib/auth';
	import { listForms } from '$lib/forms';
	import { goto } from '$app/navigation';
	import type { FormSummary } from '$lib/forms';

	let forms = $state<FormSummary[]>([]);
	let loading = $state(true);

	const totalForms = $derived(forms.length);
	const openForms = $derived(forms.filter(f => f.status === 'open').length);
	const totalResponses = $derived(forms.reduce((sum, f) => sum + f.responseCount, 0));

	onMount(async () => {
		try {
			forms = await listForms();
		} finally {
			loading = false;
		}
	});

	async function handleLogout() {
		await logout();
		auth.clearAll();
		goto('/login');
	}
</script>

<svelte:head>
	<title>Confide — Dashboard</title>
</svelte:head>

<div style="font-family: monospace; max-width: 720px; padding: 32px 32px 48px;">

	<!-- Header -->
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 36px;">
		<div>
			<h1 style="font-size: 1.4rem; margin: 0 0 4px; color: #e2e8f0;">Dashboard</h1>
			<p style="margin: 0; font-size: 0.75rem; color: #4b6280;">{auth.accountId ?? '—'}</p>
		</div>
		<button
			onclick={handleLogout}
			style="padding: 7px 14px; background: transparent; color: #8899aa; border: 1px solid #2d3f55; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.8rem;"
		>
			Sign out
		</button>
	</div>

	<!-- Stats row -->
	<div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-bottom: 32px;">
		{#each [
			{ label: 'Total forms', value: loading ? '…' : String(totalForms) },
			{ label: 'Open', value: loading ? '…' : String(openForms) },
			{ label: 'Responses', value: loading ? '…' : String(totalResponses) },
		] as stat}
			<div style="padding: 16px 20px; border: 1px solid #1e2d3e; border-radius: 6px;">
				<p style="margin: 0 0 6px; font-size: 0.68rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: #4b6280;">{stat.label}</p>
				<p style="margin: 0; font-size: 1.6rem; color: #c5d3e0;">{stat.value}</p>
			</div>
		{/each}
	</div>

	<!-- Quick links -->
	<div style="display: flex; flex-direction: column; gap: 6px;">
		<a href="/forms" style="
			display: flex; align-items: center; justify-content: space-between;
			padding: 13px 16px; border: 1px solid #1e2d3e; border-radius: 6px;
			text-decoration: none; color: #c5d3e0; font-size: 0.85rem;
			transition: border-color 120ms, background 120ms;
		"
		onmouseenter={e => { (e.currentTarget as HTMLElement).style.borderColor = '#2d3f55'; (e.currentTarget as HTMLElement).style.background = '#1a2840'; }}
		onmouseleave={e => { (e.currentTarget as HTMLElement).style.borderColor = '#1e2d3e'; (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
		>
			<span>Forms</span>
			<span style="color: #4b6280; font-size: 0.75rem;">→</span>
		</a>
		<a href="/settings/sessions" style="
			display: flex; align-items: center; justify-content: space-between;
			padding: 13px 16px; border: 1px solid #1e2d3e; border-radius: 6px;
			text-decoration: none; color: #c5d3e0; font-size: 0.85rem;
			transition: border-color 120ms, background 120ms;
		"
		onmouseenter={e => { (e.currentTarget as HTMLElement).style.borderColor = '#2d3f55'; (e.currentTarget as HTMLElement).style.background = '#1a2840'; }}
		onmouseleave={e => { (e.currentTarget as HTMLElement).style.borderColor = '#1e2d3e'; (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
		>
			<span>Sessions</span>
			<span style="color: #4b6280; font-size: 0.75rem;">→</span>
		</a>
	</div>

</div>
