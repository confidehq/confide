<script lang="ts">
	import type { BuilderSchema, BuilderField } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';
	import FieldRenderer from './FieldRenderer.svelte';

	interface Props {
		schema: BuilderSchema;
		locale: string;
	}

	const { schema, locale }: Props = $props();

	const translation = $derived(
		schema.translations[locale] ?? schema.translations[schema.defaultLocale]
	);

	// Split fields into steps for 'steps' layout
	function computeSteps(fields: BuilderField[]): BuilderField[][] {
		const groups: BuilderField[][] = [[]];
		for (const field of fields) {
			if (field.type === 'section_break') {
				groups.push([field]);
			} else {
				groups[groups.length - 1].push(field);
			}
		}
		return groups.filter((g) => g.length > 0);
	}

	const isSteps = $derived(schema.layout === 'steps');
	const orderedFields = $derived(getOrderedFields(schema, locale));
	const steps = $derived(computeSteps(orderedFields));
	const totalSteps = $derived(steps.length);

	let currentStep = $state(0);
	let answers = $state<Record<string, AnswerValue>>({});

	const isLastStep = $derived(currentStep === totalSteps - 1);
	const currentFields = $derived(isSteps ? (steps[currentStep] ?? []) : orderedFields);

	function fieldTranslation(fieldId: string) {
		return translation?.fields[fieldId] ?? { label: fieldId };
	}

	function setAnswer(fieldId: string, v: AnswerValue) {
		answers = { ...answers, [fieldId]: v };
	}
</script>

<div class="max-w-[600px] mt-10 mx-auto px-6 pb-20 font-[system-ui,sans-serif] text-[#111]">
	<!-- Preview banner -->
	<div class="mb-6 px-3.5 py-2 bg-[#fefce8] border border-[#fde047] rounded-md text-[0.8rem] text-[#854d0e]">
		Preview mode — responses will not be submitted
	</div>

	{#if isSteps}
		<p class="text-[0.8rem] text-[#9ca3af] m-0 mb-4">Step {currentStep + 1} of {totalSteps}</p>
	{/if}

	{#if !isSteps || currentStep === 0}
		<h1 class="text-[1.5rem] font-bold m-0 mb-2">{translation?.formTitle ?? ''}</h1>
		{#if translation?.formDescription}
			<p class="m-0 mb-8 text-[#4b5563]">{translation.formDescription}</p>
		{/if}
	{/if}

	<div class="flex flex-col gap-6">
		{#each currentFields as field (field.id)}
			<FieldRenderer
				{field}
				translation={fieldTranslation(field.id)}
				value={answers[field.id]}
				error={null}
				onchange={(v) => setAnswer(field.id, v)}
			/>
		{/each}
	</div>

	<div class="flex justify-between items-center mt-8">
		{#if isSteps && currentStep > 0}
			<button
				type="button"
				onclick={() => { currentStep = Math.max(currentStep - 1, 0); }}
				class="px-6 py-2.5 bg-white text-[#374151] border-[1.5px] border-[#d1d5db] rounded-md text-[0.9rem] font-[inherit] cursor-pointer hover:bg-[#f9fafb] transition-colors duration-100"
			>
				← Back
			</button>
		{:else}
			<span></span>
		{/if}

		{#if isSteps && !isLastStep}
			<button
				type="button"
				onclick={() => { currentStep = Math.min(currentStep + 1, totalSteps - 1); }}
				class="px-6 py-2.5 bg-[#1d4ed8] text-white border-none rounded-md text-[0.9rem] font-[inherit] cursor-pointer hover:bg-[#1e40af] transition-colors duration-100"
			>
				Next →
			</button>
		{:else}
			<button
				type="button"
				disabled
				class="px-6 py-2.5 bg-[#9ca3af] text-white border-none rounded-md text-[0.9rem] font-[inherit] cursor-not-allowed"
			>
				Submit
			</button>
		{/if}
	</div>
</div>
