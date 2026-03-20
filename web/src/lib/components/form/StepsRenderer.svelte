<script lang="ts">
	import type { BuilderSchema, BuilderField } from '$lib/types/builder';
	import type { ResponsePayload } from '$lib/types/crypto';
	import { submitResponse } from '$lib/forms';
	import { validateAnswer } from '$lib/validation';
	import type { AnswerValue } from '$lib/validation';
	import FieldRenderer from './FieldRenderer.svelte';

	interface Props {
		schema: BuilderSchema;
		formId: string;
		publicFormKey: ArrayBuffer;
		schemaVersion: number;
		locale: string;
		onsubmitted: () => void;
	}

	const { schema, formId, publicFormKey, schemaVersion, locale, onsubmitted }: Props = $props();

	const translation = $derived(
		schema.translations[locale] ?? schema.translations[schema.defaultLocale]
	);

	// Split fields into steps: each section_break starts a new step
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

	const steps = $derived(computeSteps(schema.fields));
	const totalSteps = $derived(steps.length);

	let currentStep = $state(0);
	let answers = $state<Record<string, AnswerValue>>({});
	let errors = $state<Record<string, string>>({});
	let submitting = $state(false);
	let submitError = $state<string | null>(null);

	const isLastStep = $derived(currentStep === totalSteps - 1);
	const currentFields = $derived(steps[currentStep] ?? []);

	function fieldTranslation(fieldId: string) {
		return translation?.fields[fieldId] ?? { label: fieldId };
	}

	function setAnswer(fieldId: string, v: AnswerValue) {
		answers = { ...answers, [fieldId]: v };
		if (errors[fieldId]) {
			const next = { ...errors };
			delete next[fieldId];
			errors = next;
		}
	}

	function validateCurrentStep(): boolean {
		const stepErrors: Record<string, string> = {};
		for (const field of currentFields) {
			if (field.type === 'section_break') continue;
			const err = validateAnswer(field, answers[field.id]);
			if (err) stepErrors[field.id] = err;
		}
		errors = { ...errors, ...stepErrors };
		return Object.keys(stepErrors).length === 0;
	}

	function handleNext() {
		if (!validateCurrentStep()) return;
		currentStep = Math.min(currentStep + 1, totalSteps - 1);
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	function handleBack() {
		currentStep = Math.max(currentStep - 1, 0);
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	async function handleSubmit() {
		if (!validateCurrentStep()) return;
		submitting = true;
		submitError = null;
		try {
			const payload: ResponsePayload = {
				submittedAt: new Date().toISOString(),
				locale,
				answers: Object.fromEntries(
					Object.entries(answers).filter(([, v]) => v !== undefined && v !== null)
				) as ResponsePayload['answers']
			};
			await submitResponse(formId, publicFormKey, payload, schemaVersion);
			onsubmitted();
		} catch (err) {
			submitError = err instanceof Error ? err.message : 'Submission failed. Please try again.';
		} finally {
			submitting = false;
		}
	}
</script>

<div style="max-width: 600px; margin: 40px auto; padding: 0 24px 80px; font-family: system-ui, sans-serif; color: #111;">
	<p style="font-size: 0.8rem; color: #9ca3af; margin: 0 0 16px;">Step {currentStep + 1} of {totalSteps}</p>

	{#if currentStep === 0}
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
				error={errors[field.id] ?? null}
				onchange={(v) => setAnswer(field.id, v)}
			/>
		{/each}
	</div>

	{#if submitError}
		<p style="margin: 24px 0 0; color: #ef4444; font-size: 0.9rem;">{submitError}</p>
	{/if}

	<div style="display: flex; justify-content: space-between; align-items: center; margin-top: 32px;">
		{#if currentStep > 0}
			<button
				type="button"
				onclick={handleBack}
				style="padding: 10px 24px; background: white; color: #374151; border: 1.5px solid #d1d5db; border-radius: 6px; font-size: 0.9rem; font-family: inherit; cursor: pointer;"
			>
				← Back
			</button>
		{:else}
			<span></span>
		{/if}

		{#if isLastStep}
			<button
				type="button"
				onclick={handleSubmit}
				disabled={submitting}
				style="padding: 10px 24px; background: {submitting ? '#9ca3af' : '#1d4ed8'}; color: white; border: none; border-radius: 6px; font-size: 0.9rem; font-family: inherit; cursor: {submitting ? 'not-allowed' : 'pointer'};"
			>
				{submitting ? 'Submitting…' : 'Submit'}
			</button>
		{:else}
			<button
				type="button"
				onclick={handleNext}
				style="padding: 10px 24px; background: #1d4ed8; color: white; border: none; border-radius: 6px; font-size: 0.9rem; font-family: inherit; cursor: pointer;"
			>
				Next →
			</button>
		{/if}
	</div>
</div>
