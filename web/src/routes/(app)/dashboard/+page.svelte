<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { listForms, getForm, type FormSummary } from '$lib/forms';
	import { listWorkspaces } from '$lib/workspaces';
	import { goto } from '$app/navigation';
	import { ArrowRight } from '@lucide/svelte';

	interface DashboardForm extends FormSummary {
		workspaceId: string;
		workspaceName: string;
	}

	let loading = $state(false);
	let loaded = $state(false);
	let error = $state('');
	let allForms = $state<DashboardForm[]>([]);
	let formNames = $state<Map<string, string>>(new Map());

	$effect(() => {
		if (auth.masterKey && !loaded && !loading) {
			loadAll(auth.masterKey);
		}
	});

	async function loadAll(masterKey: CryptoKey) {
		loading = true;
		error = '';
		try {
			const workspaces = await listWorkspaces();

			// Fetch forms from every workspace in parallel (personal workspace is included in the list)
			const sources = workspaces.map(w => ({ workspaceId: w.id, workspaceName: w.name }));

			const results = await Promise.allSettled(
				sources.map(s => listForms(s.workspaceId))
			);

			const merged: DashboardForm[] = [];
			results.forEach((r, i) => {
				if (r.status === 'fulfilled') {
					for (const f of r.value) {
						merged.push({ ...f, workspaceId: sources[i].workspaceId, workspaceName: sources[i].workspaceName });
					}
				}
			});

			// Sort by updatedAt descending
			merged.sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());
			allForms = merged;
			loaded = true;

			// Decrypt names best-effort
			if (merged.length > 0) {
				const top = merged.slice(0, 10);
				const nameResults = await Promise.allSettled(top.map(f => getForm(masterKey, f.formId)));
				const names = new Map<string, string>();
				nameResults.forEach((r, i) => {
					if (r.status === 'fulfilled') {
						const { schema } = r.value;
						const name = schema.name || schema.translations[schema.defaultLocale]?.formTitle;
						if (name) names.set(top[i].formId, name);
					}
				});
				formNames = names;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	const totalForms = $derived(allForms.length);
	const openForms = $derived(allForms.filter(f => f.status === 'open').length);
	const totalResponses = $derived(allForms.reduce((sum, f) => sum + f.responseCount, 0));
	const recentForms = $derived(allForms.slice(0, 5));

	function formName(formId: string): string {
		return formNames.get(formId) ?? '—';
	}

	function shortId(id: string): string {
		return id.slice(0, 8) + '…';
	}

	const stats = $derived([
		{ label: 'Total forms', value: loading ? '…' : String(totalForms) },
		{ label: 'Open', value: loading ? '…' : String(openForms) },
		{ label: 'Responses', value: loading ? '…' : String(totalResponses) },
	]);
</script>

<svelte:head>
	<title>Confide — Dashboard</title>
</svelte:head>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="flex items-start justify-between mb-8 gap-4">
		<div class="min-w-0">
			<h1 class="text-2xl m-0 mb-1.5 text-[#e2e8f0] font-semibold">Dashboard</h1>
			<p class="m-0 text-base text-[#374d63] truncate" title={auth.accountId ?? undefined}>
				{auth.accountId ? shortId(auth.accountId) : '—'}
			</p>
		</div>
		<button
			onclick={() => goto('/forms/new')}
			class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
		>+ New form</button>
	</div>

	<!-- Stats -->
	<div class="grid grid-cols-3 gap-2 sm:gap-3 mb-10">
		{#each stats as stat}
			<div class="px-4 py-4 sm:px-5 sm:py-5 border border-border-deep rounded-lg flex flex-col gap-2">
				<p class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-[#374d63]">{stat.label}</p>
				<p class="m-0 text-4xl sm:text-5xl text-[#c5d3e0] leading-none tabular-nums">{stat.value}</p>
			</div>
		{/each}
	</div>

	<!-- Recent forms -->
	<div>
		<div class="flex items-center justify-between mb-3">
			<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-[#374d63]">Recent forms</h2>
			{#if totalForms > 0}
				<a href="/forms" class="flex items-center gap-1 text-base text-[#4b6280] hover:text-[#93c5fd] no-underline transition-colors duration-100">
					View all <ArrowRight size={14} strokeWidth={1.75} />
				</a>
			{/if}
		</div>

		{#if loading}
			<div class="py-6 text-center text-[#374d63] text-base">Loading…</div>
		{:else if error}
			<div class="py-6 text-center text-error-muted text-base">{error}</div>
		{:else if recentForms.length === 0}
			<div class="py-10 border border-dashed border-border rounded-lg text-center">
				<p class="m-0 mb-1 text-[#4b6280] text-base">No forms yet</p>
				<p class="m-0 text-[#374d63] text-base">Create your first form to start collecting responses</p>
				<button
					onclick={() => goto('/forms/new')}
					class="mt-4 px-4 py-2 bg-transparent text-[#93c5fd] border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
				>+ New form</button>
			</div>
		{:else}
			<div class="border border-border-deep rounded-lg overflow-hidden">
				{#each recentForms as form, i (form.formId)}
					<div
						class="flex items-center gap-3 px-4 py-3.5 cursor-pointer hover:bg-[#1a2840] transition-colors duration-100
							{i < recentForms.length - 1 ? 'border-b border-border-deep' : ''}"
						onclick={() => goto(`/forms/${form.formId}/edit`)}
						role="button"
						tabindex="0"
						onkeydown={e => e.key === 'Enter' && goto(`/forms/${form.formId}/edit`)}
					>
						<!-- Status dot -->
						<span class="shrink-0 w-2 h-2 rounded-full
							{form.status === 'open' ? 'bg-[#4ade80]' : 'bg-[#374d63]'}">
						</span>

						<!-- Name + workspace -->
						<span class="flex-1 min-w-0 flex items-center gap-2 overflow-hidden">
							<span class="text-base text-[#c5d3e0] truncate">{formName(form.formId)}</span>
							<span class="shrink-0 hidden sm:inline px-2 py-0.5 rounded text-xs text-[#4b6280] bg-[#0f1e2e] border border-border-deep">
								{form.workspaceName}
							</span>
						</span>

						<!-- Responses -->
						<span class="shrink-0 text-base text-[#4b6280] tabular-nums hidden sm:block">
							{form.responseCount} {form.responseCount === 1 ? 'response' : 'responses'}
						</span>

						<!-- Status badge -->
						<span class="shrink-0 hidden sm:inline px-2.5 py-0.5 rounded-full text-base
							{form.status === 'open'
								? 'bg-open-bg text-open-text border border-open-border'
								: 'bg-closed-bg text-closed-text border border-closed-border'}">
							{form.status}
						</span>

						<!-- Responses (mobile) -->
						<span class="shrink-0 text-base text-[#4b6280] tabular-nums sm:hidden">
							{form.responseCount}r
						</span>

						<ArrowRight size={16} strokeWidth={1.5} class="shrink-0 text-[#374d63]" />
					</div>
				{/each}
			</div>
		{/if}
	</div>

</div>
</div>
