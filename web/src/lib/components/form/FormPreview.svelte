<script lang="ts">
	import type { BuilderSchema, BuilderField } from '$lib/types/builder';
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
	const steps = $derived(computeSteps(schema.fields));
	const totalSteps = $derived(steps.length);

	let currentStep = $state(0);
	let answers = $state<Record<string, AnswerValue>>({});

	const isLastStep = $derived(currentStep === totalSteps - 1);
	const currentFields = $derived(isSteps ? (steps[currentStep] ?? []) : schema.fields);

	function fieldTranslation(fieldId: string) {
		return translation?.fields[fieldId] ?? { label: fieldId };
	}

	function setAnswer(fieldId: string, v: AnswerValue) {
		answers = { ...answers, [fieldId]: v };
	}
</script>

<div style="
	max-width: 600px; margin: 40px auto; padding: 0 24px 80px;
	font-family: system-ui, sans-serif; color: #111;
">
	<!-- Preview banner -->
	<div style="
		margin-bottom: 24px; padding: 8px 14px;
		background: #fefce8; border: 1px solid #fde047;
		border-radius: 6px; font-size: 0.8rem; color: #854d0e;
	">
		Preview mode — responses will not be submitted
	</div>

	{#if isSteps}
		<p style="font-size: 0.8rem; color: #9ca3af; margin: 0 0 16px;">Step {currentStep + 1} of {totalSteps}</p>
	{/if}

	{#if !isSteps || currentStep === 0}
		<h1 style="font-size: 1.5rem; font-weight: 700; margin: 0 0 8px;">{translation?.formTitle ?? ''}</h1>
		{#if translation?.formDescription}
			<p style="margin: 0 0 32px; color: #4b5563;">{translation.formDescription}</p>
		{/if}
	{/if}

	<div style="display: flex; flex-direction: column; gap: 24px;">
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

	<div style="display: flex; justify-content: space-between; align-items: center; margin-top: 32px;">
		{#if isSteps && currentStep > 0}
			<button
				type="button"
				onclick={() => { currentStep = Math.max(currentStep - 1, 0); }}
				style="padding: 10px 24px; background: white; color: #374151; border: 1.5px solid #d1d5db; border-radius: 6px; font-size: 0.9rem; font-family: inherit; cursor: pointer;"
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
				style="padding: 10px 24px; background: #1d4ed8; color: white; border: none; border-radius: 6px; font-size: 0.9rem; font-family: inherit; cursor: pointer;"
			>
				Next →
			</button>
		{:else}
			<button
				type="button"
				disabled
				style="padding: 10px 24px; background: #9ca3af; color: white; border: none; border-radius: 6px; font-size: 0.9rem; font-family: inherit; cursor: not-allowed;"
			>
				Submit
			</button>
		{/if}
	</div>
</div>
