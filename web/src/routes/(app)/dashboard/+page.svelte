<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';
	import { goto } from '$app/navigation';
	import { ArrowRight, Building2 } from '@lucide/svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	function planLabel(plan: string, planStatus: string): string {
		if (plan === 'pro') {
			if (planStatus === 'past_due') return 'Pro · past due';
			if (planStatus === 'canceled') return 'Pro · canceled';
			if (planStatus === 'canceling') return 'Pro · cancels at period end';
			return 'Pro';
		}
		return 'Free';
	}

	$effect(() => {
		const workspace = workspacesStore.active;
		const masterKey = auth.masterKey;
		if (masterKey && workspace && workspace.status === 'active') {
			formsStore.load(masterKey, workspace.id);
		}
	});

	const sortedForms = $derived(
		[...formsStore.forms].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
	);

	const totalForms = $derived(sortedForms.length);
	const openForms = $derived(sortedForms.filter(f => f.status === 'open').length);
	const totalResponses = $derived(sortedForms.reduce((sum, f) => sum + f.responseCount, 0));
	const recentForms = $derived(sortedForms.slice(0, 5));

	const stats = $derived([
		{ label: 'Forms', value: formsStore.loading ? '…' : String(totalForms) },
		{ label: 'Open', value: formsStore.loading ? '…' : String(openForms) },
		{ label: 'Responses', value: formsStore.loading ? '…' : String(totalResponses) },
	]);

	function newFormHref(): string {
		const ws = workspacesStore.active;
		return ws ? `/forms/new?workspaceId=${ws.id}` : '/forms/new';
	}
</script>

<svelte:head>
	<title>Confide — Dashboard</title>
</svelte:head>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="flex items-start justify-between mb-8 gap-4">
		<div>
			<h1 class="text-2xl m-0 mb-1 text-text font-semibold">Dashboard</h1>
			<p class="m-0 text-sm text-subtle">An overview of your workspace activity</p>
		</div>
		{#if workspacesStore.active?.status !== 'pending'}
			<button
				onclick={() => goto(newFormHref())}
				class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
			>+ New form</button>
		{/if}
	</div>

	{#if workspacesStore.active}
		{@const ws = workspacesStore.active}
		<div class="flex items-center gap-3 mb-4">
			<Building2 size={18} strokeWidth={1.75} class="shrink-0 text-subtle" />
			<span class="text-xl font-semibold text-text truncate min-w-0">{ws.name}</span>
			<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border
				{ws.plan === 'pro'
					? ws.planStatus === 'active' || ws.planStatus === 'canceling'
						? 'bg-open-bg text-open-text border-open-border'
						: 'bg-closed-bg text-closed-text border-closed-border'
					: 'text-subtle border-border bg-transparent'}">
				{planLabel(ws.plan, ws.planStatus)}
			</span>
			<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base text-subtle border border-border">
				{ws.role}
			</span>
		</div>
	{/if}

	{#if workspacesStore.active?.status === 'pending'}
		<!-- Pending approval state -->
		<div class="py-14 border border-dashed border-border rounded-lg text-center px-6">
			<p class="m-0 mb-1 text-text text-base font-medium">Access pending approval</p>
			<p class="m-0 text-subtle text-sm mt-1.5 max-w-sm mx-auto">
				A workspace admin needs to grant you access before you can view forms and workspace content.
			</p>
		</div>
	{:else}
		<!-- Stats -->
		<div class="grid grid-cols-3 gap-2 sm:gap-3 mb-10">
			{#each stats as stat}
				<div class="px-4 py-4 sm:px-5 sm:py-5 border border-border rounded-lg flex flex-col gap-2">
					<p class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-subtle">{stat.label}</p>
					<p class="m-0 text-4xl sm:text-5xl text-text leading-none tabular-nums">{stat.value}</p>
				</div>
			{/each}
		</div>

		<!-- Recent forms -->
		<div>
			<div class="flex items-center justify-between mb-3">
				<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-subtle">Recent forms</h2>
				{#if totalForms > 0}
					<a href="/forms" class="flex items-center gap-1 text-base text-subtle hover:text-text no-underline transition-colors duration-100">
						View all <ArrowRight size={14} strokeWidth={1.75} />
					</a>
				{/if}
			</div>

			{#if formsStore.loading && recentForms.length === 0}
				<div class="py-6 text-center text-subtle text-base">Loading…</div>
			{:else if recentForms.length === 0 && !formsStore.loading}
				<div class="py-10 border border-dashed border-border rounded-lg text-center">
					<p class="m-0 mb-1 text-subtle text-base">No forms yet</p>
					<p class="m-0 text-subtle text-base">Create your first form to start collecting responses</p>
					<button
						onclick={() => goto(newFormHref())}
						class="mt-4 px-4 py-2 bg-transparent text-text border border-border rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
					>+ New form</button>
				</div>
			{:else}
				<div class="border border-border rounded-lg overflow-hidden">
					{#each recentForms as form, i (form.formId)}
						<div
							class="flex items-center gap-3 px-4 py-3.5 cursor-pointer hover:bg-surface transition-colors duration-75
								{i < recentForms.length - 1 ? 'border-b border-border' : ''}"
							onclick={() => goto(`/forms/${form.formId}`)}
							role="button"
							tabindex="0"
							onkeydown={e => e.key === 'Enter' && goto(`/forms/${form.formId}`)}
						>
							<span class="shrink-0 w-2 h-2 rounded-full
								{form.status === 'open' ? 'bg-success' : 'bg-muted'}">
							</span>

							<span class="flex-1 min-w-0 overflow-hidden">
								<span class="text-base text-text truncate block">{formsStore.formNames.get(form.formId) ?? '—'}</span>
							</span>

							<span class="shrink-0 text-base text-subtle tabular-nums hidden sm:block">
								{form.responseCount} {form.responseCount === 1 ? 'response' : 'responses'}
							</span>

							<StatusBadge status={form.status} class="hidden sm:inline" />

							<span class="shrink-0 text-base text-subtle tabular-nums sm:hidden">
								{form.responseCount}r
							</span>

							<ArrowRight size={16} strokeWidth={1.5} class="shrink-0 text-subtle" />
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

</div>
</div>
