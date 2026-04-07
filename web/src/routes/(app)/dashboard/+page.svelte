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

<div class="font-mono max-w-[720px] p-8 pb-12">

	<!-- Header -->
	<div class="flex items-center justify-between mb-9">
		<div>
			<h1 class="text-[1.6rem] m-0 mb-1 text-[#e2e8f0]">Dashboard</h1>
			<p class="m-0 text-[0.875rem] text-[#4b6280]">{auth.accountId ?? '—'}</p>
		</div>
		<button
			onclick={handleLogout}
			class="px-3.5 py-1.5 bg-transparent text-[#8899aa] border border-border-subtle rounded cursor-pointer font-mono text-[0.925rem] hover:text-muted transition-colors duration-100"
		>
			Sign out
		</button>
	</div>

	<!-- Stats row -->
	<div class="grid grid-cols-3 gap-3 mb-8">
		{#each [
			{ label: 'Total forms', value: loading ? '…' : String(totalForms) },
			{ label: 'Open', value: loading ? '…' : String(openForms) },
			{ label: 'Responses', value: loading ? '…' : String(totalResponses) },
		] as stat}
			<div class="px-5 py-4 border border-border-deep rounded-md">
				<p class="m-0 mb-1.5 text-[0.78rem] font-semibold tracking-[0.08em] uppercase text-[#4b6280]">{stat.label}</p>
				<p class="m-0 text-[1.85rem] text-[#c5d3e0]">{stat.value}</p>
			</div>
		{/each}
	</div>

	<!-- Quick links -->
	<div class="flex flex-col gap-1.5">
		<a
			href="/forms"
			class="flex items-center justify-between px-4 py-3.5 border border-border-deep rounded-md no-underline text-[#c5d3e0] text-[0.975rem] transition-[border-color,background] duration-100 hover:border-border-subtle hover:bg-[#1a2840]"
		>
			<span>Forms</span>
			<span class="text-[#4b6280] text-[0.875rem]">→</span>
		</a>
		<a
			href="/settings/sessions"
			class="flex items-center justify-between px-4 py-3.5 border border-border-deep rounded-md no-underline text-[#c5d3e0] text-[0.975rem] transition-[border-color,background] duration-100 hover:border-border-subtle hover:bg-[#1a2840]"
		>
			<span>Sessions</span>
			<span class="text-[#4b6280] text-[0.875rem]">→</span>
		</a>
	</div>

</div>
