<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { listForms, getForm, type FormSummary } from '$lib/forms';
	import { goto } from '$app/navigation';
	import { ArrowRight } from '@lucide/svelte';

	let loading = $state(false);
	let currentWorkspaceId = $state<string | null>(null);
	let allForms = $state<FormSummary[]>([]);
	let formNames = $state<Map<string, string>>(new Map());

	$effect(() => {
		const workspace = workspacesStore.active;
		const masterKey = auth.masterKey;
		if (masterKey && workspace && workspace.id !== currentWorkspaceId) {
			loadWorkspace(masterKey, workspace.id);
		}
	});

	async function loadWorkspace(masterKey: CryptoKey, workspaceId: string) {
		loading = true;
		currentWorkspaceId = workspaceId;
		allForms = [];
		formNames = new Map();
		try {
			const forms = await listForms(workspaceId);
			if (currentWorkspaceId !== workspaceId) return;

			forms.sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());
			allForms = forms;

			if (forms.length > 0) {
				const top = forms.slice(0, 10);
				const nameResults = await Promise.allSettled(top.map(f => getForm(masterKey, f.formId)));
				if (currentWorkspaceId !== workspaceId) return;
				const names = new Map<string, string>();
				nameResults.forEach((r, i) => {
					if (r.status === 'fulfilled') {
						const { schema } = r.value;
						const name = schema.translations[schema.defaultLocale]?.formTitle;
						if (name) names.set(top[i].formId, name);
					}
				});
				formNames = names;
			}
		} catch {
			// non-fatal
		} finally {
			if (currentWorkspaceId === workspaceId) loading = false;
		}
	}

	const totalForms = $derived(allForms.length);
	const openForms = $derived(allForms.filter(f => f.status === 'open').length);
	const totalResponses = $derived(allForms.reduce((sum, f) => sum + f.responseCount, 0));
	const recentForms = $derived(allForms.slice(0, 5));

	function formName(formId: string): string {
		return formNames.get(formId) ?? '—';
	}

	const stats = $derived([
		{ label: 'Forms', value: loading ? '…' : String(totalForms) },
		{ label: 'Open', value: loading ? '…' : String(openForms) },
		{ label: 'Responses', value: loading ? '…' : String(totalResponses) },
	]);

	function newFormHref(): string {
		const ws = workspacesStore.active;
		return ws ? `/forms/new?workspaceId=${ws.id}` : '/forms/new';
	}
</script>

<svelte:head>
	<title>Confide — Dashboard</title>
</svelte:head>

<div class="flex justify-center max-w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-7xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="flex items-start justify-between mb-8 gap-4">
		<div class="min-w-0">
			<h1 class="text-2xl m-0 mb-1 text-text-bright font-semibold">Dashboard</h1>
			{#if workspacesStore.active}
				<p class="m-0 text-sm text-muted-dim flex items-center gap-1.5">
					<span>{workspacesStore.active.name}</span>
					<span class="text-border-mid">·</span>
					<span class="capitalize
						{workspacesStore.active.plan === 'pro' && workspacesStore.active.planStatus === 'active'
							? 'text-success-text-dark'
							: 'text-muted-dim'}">
						{workspacesStore.active.plan}
					</span>
				</p>
			{:else if workspacesStore.loading}
				<p class="m-0 text-sm text-muted-mid">Loading…</p>
			{/if}
		</div>
		<button
			onclick={() => goto(newFormHref())}
			class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
		>+ New form</button>
	</div>

	<!-- Stats -->
	<div class="grid grid-cols-3 gap-2 sm:gap-3 mb-10">
		{#each stats as stat}
			<div class="px-4 py-4 sm:px-5 sm:py-5 border border-border-deep rounded-lg flex flex-col gap-2">
				<p class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">{stat.label}</p>
				<p class="m-0 text-4xl sm:text-5xl text-text-body leading-none tabular-nums">{stat.value}</p>
			</div>
		{/each}
	</div>

	<!-- Recent forms -->
	<div>
		<div class="flex items-center justify-between mb-3">
			<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Recent forms</h2>
			{#if totalForms > 0}
				<a href="/forms" class="flex items-center gap-1 text-base text-muted-dim hover:text-text-blue no-underline transition-colors duration-100">
					View all <ArrowRight size={14} strokeWidth={1.75} />
				</a>
			{/if}
		</div>

		{#if loading && recentForms.length === 0}
			<div class="py-6 text-center text-muted-mid text-base">Loading…</div>
		{:else if recentForms.length === 0 && !loading}
			<div class="py-10 border border-dashed border-border rounded-lg text-center">
				<p class="m-0 mb-1 text-muted-dim text-base">No forms yet</p>
				<p class="m-0 text-muted-mid text-base">Create your first form to start collecting responses</p>
				<button
					onclick={() => goto(newFormHref())}
					class="mt-4 px-4 py-2 bg-transparent text-text-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
				>+ New form</button>
			</div>
		{:else}
			<div class="border border-border-deep rounded-lg overflow-hidden">
				{#each recentForms as form, i (form.formId)}
					<div
						class="flex items-center gap-3 px-4 py-3.5 cursor-pointer hover:bg-border-card transition-colors duration-100
							{i < recentForms.length - 1 ? 'border-b border-border-deep' : ''}"
						onclick={() => goto(`/forms/${form.formId}`)}
						role="button"
						tabindex="0"
						onkeydown={e => e.key === 'Enter' && goto(`/forms/${form.formId}`)}
					>
						<span class="shrink-0 w-2 h-2 rounded-full
							{form.status === 'open' ? 'bg-success-text-dark' : 'bg-muted-mid'}">
						</span>

						<span class="flex-1 min-w-0 overflow-hidden">
							<span class="text-base text-text-body truncate block">{formName(form.formId)}</span>
						</span>

						<span class="shrink-0 text-base text-muted-dim tabular-nums hidden sm:block">
							{form.responseCount} {form.responseCount === 1 ? 'response' : 'responses'}
						</span>

						<span class="shrink-0 hidden sm:inline px-2.5 py-0.5 rounded-full text-base
							{form.status === 'open'
								? 'bg-open-bg text-open-text border border-open-border'
								: 'bg-closed-bg text-closed-text border border-closed-border'}">
							{form.status}
						</span>

						<span class="shrink-0 text-base text-muted-dim tabular-nums sm:hidden">
							{form.responseCount}r
						</span>

						<ArrowRight size={16} strokeWidth={1.5} class="shrink-0 text-muted-mid" />
					</div>
				{/each}
			</div>
		{/if}
	</div>

</div>
</div>
