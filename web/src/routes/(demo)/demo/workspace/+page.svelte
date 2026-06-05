<script lang="ts">
import { ArrowRight, Building2, Lock } from "@lucide/svelte";
import StatusBadge from "$lib/components/StatusBadge.svelte";
import { DEMO_FORMS, DEMO_FORM_NAMES, DEMO_WORKSPACE } from "$lib/demo/data";

const ws = DEMO_WORKSPACE;
const forms = DEMO_FORMS;
</script>

<svelte:head>
	<title>Workspace — Demo</title>
</svelte:head>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Page header -->
	<div class="flex items-start justify-between mb-8 gap-4">
		<h1 class="text-2xl m-0 text-text font-semibold">All Workspaces</h1>
	</div>

	<!-- Workspace -->
	<div>
		<!-- Workspace header -->
		<div class="flex items-center gap-3 mb-4">
			<Building2 size={18} strokeWidth={1.75} class="shrink-0 text-subtle" />
			<h2 class="m-0 text-xl font-semibold text-text truncate min-w-0">{ws.name}</h2>

			<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border bg-open-bg text-open-text border-open-border">
				Pro
			</span>

			<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base text-subtle border border-border-canvas">
				{ws.role}
			</span>
		</div>

		<!-- Forms list -->
		<div class="border border-border-canvas rounded-lg overflow-hidden">
			{#each forms as form, i (form.formId)}
				{@const isActive = form.formId === 'demo-form-1'}
				{#if isActive}
					<a
						href="/demo/responses"
						class="flex items-center gap-3 px-4 py-3.5 no-underline hover:bg-surface transition-colors duration-75
							{i < forms.length - 1 ? 'border-b border-border-canvas' : ''}"
					>
						<span class="shrink-0 w-2 h-2 rounded-full bg-success"></span>
						<span class="flex-1 min-w-0 text-base text-text truncate font-medium">
							{DEMO_FORM_NAMES[form.formId]}
						</span>
						<span class="shrink-0 text-base text-subtle tabular-nums hidden sm:block">
							{form.responseCount} responses
						</span>
						<StatusBadge status={form.status} class="hidden sm:inline" />
						<span class="shrink-0 text-base text-subtle tabular-nums sm:hidden">{form.responseCount}r</span>
						<ArrowRight size={16} strokeWidth={1.5} class="shrink-0 text-subtle" />
					</a>
				{:else}
					<div
						class="flex items-center gap-3 px-4 py-3.5 select-none
							{i < forms.length - 1 ? 'border-b border-border-canvas' : ''}"
						title="Not available in this demo"
					>
						<span class="shrink-0 w-2 h-2 rounded-full {form.status === 'open' ? 'bg-success' : 'bg-muted'}"></span>
						<span class="flex-1 min-w-0 text-base text-text truncate">
							{DEMO_FORM_NAMES[form.formId]}
						</span>
						<span class="shrink-0 text-base text-subtle tabular-nums hidden sm:block">
							{form.responseCount} {form.responseCount === 1 ? 'response' : 'responses'}
						</span>
						<StatusBadge status={form.status} class="hidden sm:inline" />
						<span class="shrink-0 text-base text-subtle tabular-nums sm:hidden">{form.responseCount}r</span>
						<Lock size={14} strokeWidth={1.75} class="shrink-0 text-muted" />
					</div>
				{/if}
			{/each}
		</div>
		<p class="mt-3 text-sm text-muted">
			Click <strong class="text-subtle">Anonymous Incident Report</strong> to see the response inbox.
		</p>
	</div>

</div>
</div>
