<script lang="ts">
import { ArrowRight, Building2, Lock } from "@lucide/svelte";
import StatusBadge from "$lib/components/StatusBadge.svelte";
import { DEMO_FORMS, DEMO_FORM_NAMES, DEMO_WORKSPACE } from "$lib/demo/data";

const ws = DEMO_WORKSPACE;
const recentForms = DEMO_FORMS;

const stats = [
	{ label: "Forms", value: "3" },
	{ label: "Open", value: "2" },
	{ label: "Responses", value: "78" },
];
</script>

<svelte:head>
	<title>Dashboard — Demo</title>
</svelte:head>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="flex items-start justify-between mb-8 gap-4">
		<div>
			<h1 class="text-2xl m-0 mb-1 text-text font-semibold">Dashboard</h1>
			<p class="m-0 text-sm text-subtle">An overview of your workspace activity</p>
		</div>
		<a
			href="/signup"
			class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100 no-underline"
		>Get started free</a>
	</div>

	<!-- Workspace name + badges -->
	<div class="flex items-center gap-3 mb-4">
		<Building2 size={18} strokeWidth={1.75} class="shrink-0 text-subtle" />
		<span class="text-xl font-semibold text-text truncate min-w-0">{ws.name}</span>
		<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border bg-open-bg text-open-text border-open-border">
			Pro
		</span>
		<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base text-subtle border border-border-canvas">
			{ws.role}
		</span>
	</div>

	<!-- Stats -->
	<div class="grid grid-cols-3 gap-2 sm:gap-3 mb-10">
		{#each stats as stat}
			<div class="px-4 py-4 sm:px-5 sm:py-5 border border-border-canvas rounded-lg flex flex-col gap-2">
				<p class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-subtle">{stat.label}</p>
				<p class="m-0 text-4xl sm:text-5xl text-text leading-none tabular-nums">{stat.value}</p>
			</div>
		{/each}
	</div>

	<!-- Recent forms -->
	<div>
		<div class="flex items-center justify-between mb-3">
			<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-subtle">Recent forms</h2>
		</div>

		<div class="border border-border-canvas rounded-lg overflow-hidden">
			{#each recentForms as form, i (form.formId)}
				{@const isActive = form.formId === 'demo-form-1'}
				{#if isActive}
					<a
						href="/demo/responses"
						class="flex items-center gap-3 px-4 py-3.5 no-underline hover:bg-surface transition-colors duration-75
							{i < recentForms.length - 1 ? 'border-b border-border-canvas' : ''}"
					>
						<span class="shrink-0 w-2 h-2 rounded-full bg-success"></span>
						<span class="flex-1 min-w-0 overflow-hidden">
							<span class="text-base text-text truncate block font-medium">{DEMO_FORM_NAMES[form.formId]}</span>
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
						class="flex items-center gap-3 px-4 py-3.5 opacity-40 cursor-not-allowed select-none
							{i < recentForms.length - 1 ? 'border-b border-border-canvas' : ''}"
						title="Not available in this demo"
					>
						<span class="shrink-0 w-2 h-2 rounded-full {form.status === 'open' ? 'bg-success' : 'bg-muted'}"></span>
						<span class="flex-1 min-w-0 overflow-hidden">
							<span class="text-base text-text truncate block">{DEMO_FORM_NAMES[form.formId] ?? '—'}</span>
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
	</div>

	<!-- CTA -->
	<div class="mt-12 py-8 border border-dashed border-border-canvas rounded-lg text-center px-6">
		<p class="m-0 mb-1 text-text text-base font-semibold">Ready to get started?</p>
		<p class="m-0 text-subtle text-sm mt-1.5 max-w-sm mx-auto mb-5">
			Create your own workspace, build encrypted forms, and start collecting confidential responses.
		</p>
		<a
			href="/signup"
			class="inline-flex items-center gap-2 px-5 py-2.5 bg-primary text-white no-underline rounded font-mono text-base hover:bg-primary-hover transition-colors duration-100"
		>
			Create free account <ArrowRight size={14} strokeWidth={1.75} />
		</a>
	</div>

</div>
</div>
