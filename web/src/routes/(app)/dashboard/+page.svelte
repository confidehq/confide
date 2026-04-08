<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { listForms } from '$lib/forms';
	import { goto } from '$app/navigation';
	import { FileText, Settings } from '@lucide/svelte';
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

</script>

<svelte:head>
	<title>Confide — Dashboard</title>
</svelte:head>

<div class="font-mono max-w-7xl mx-auto px-4 pt-12 pb-12 sm:p-8 sm:pb-16">

	<!-- Header -->
	<div class="flex items-center justify-between mb-8">
		<div>
			<h1 class="text-2xl sm:text-2xl m-0 mb-1 text-[#e2e8f0]">Dashboard</h1>
			<p class="m-0 text-xs text-[#4b6280] truncate max-w-[200px] sm:max-w-none">{auth.accountId ?? '—'}</p>
		</div>
		<button
			onclick={() => goto('/forms/new')}
			class="shrink-0 px-3 py-1.5 bg-primary text-white border-none rounded cursor-pointer font-mono text-sm hover:bg-primary-hover transition-colors duration-100"
		>+ New form</button>
	</div>

	<!-- Stats -->
	<div class="grid grid-cols-3 gap-2 sm:gap-4 mb-8">
		{#each [
			{ label: 'Total forms', value: loading ? '…' : String(totalForms) },
			{ label: 'Open', value: loading ? '…' : String(openForms) },
			{ label: 'Responses', value: loading ? '…' : String(totalResponses) },
		] as stat}
			<div class="px-4 py-4 sm:px-6 sm:py-5 border border-border-deep rounded-lg">
				<p class="m-0 mb-2 text-xs sm:text-xs font-semibold tracking-[0.1em] uppercase text-[#4b6280]">{stat.label}</p>
				<p class="m-0 text-3xl sm:text-4xl text-[#c5d3e0] leading-none">{stat.value}</p>
			</div>
		{/each}
	</div>

	<!-- Nav cards -->
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
		<a
			href="/forms"
			class="group flex items-start gap-4 p-5 sm:p-6 border border-border-deep rounded-lg no-underline transition-[border-color,background] duration-100 hover:border-border-subtle hover:bg-[#1a2840]"
		>
			<div class="shrink-0 mt-0.5 text-[#4b6280] group-hover:text-[#93c5fd] transition-colors duration-100">
				<FileText size={20} strokeWidth={1.75} />
			</div>
			<div>
				<p class="m-0 mb-1 text-[#c5d3e0] text-sm">Forms</p>
				<p class="m-0 text-[#4b6280] text-sm">Manage your forms and view responses</p>
			</div>
		</a>
		<a
			href="/settings/sessions"
			class="group flex items-start gap-4 p-5 sm:p-6 border border-border-deep rounded-lg no-underline transition-[border-color,background] duration-100 hover:border-border-subtle hover:bg-[#1a2840]"
		>
			<div class="shrink-0 mt-0.5 text-[#4b6280] group-hover:text-[#93c5fd] transition-colors duration-100">
				<Settings size={20} strokeWidth={1.75} />
			</div>
			<div>
				<p class="m-0 mb-1 text-[#c5d3e0] text-sm">Sessions</p>
				<p class="m-0 text-[#4b6280] text-sm">View and revoke active login sessions</p>
			</div>
		</a>
	</div>

</div>
