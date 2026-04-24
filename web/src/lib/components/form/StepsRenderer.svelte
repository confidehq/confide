<script lang="ts">
	import type { BuilderSchema, BuilderField } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
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
		locales: string[];
		honeypotFields: string[];
		loadToken: string;
		onsubmitted: () => void;
		onlocalechange: (code: string) => void;
	}

	const { schema, formId, publicFormKey, schemaVersion, locale, locales, honeypotFields, loadToken, onsubmitted, onlocalechange }: Props = $props();

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

	const steps = $derived(computeSteps(getOrderedFields(schema, locale)));
	const totalSteps = $derived(steps.length);

	let currentStep = $state(0);
	let answers = $state<Record<string, AnswerValue>>({});
	let errors = $state<Record<string, string>>({});
	let submitting = $state(false);
	let submitError = $state<string | null>(null);
	let honeypotValues = $state<Record<string, string>>({});

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
			await submitResponse(formId, publicFormKey, payload, schemaVersion, loadToken, honeypotValues);
			onsubmitted();
		} catch (err) {
			submitError = err instanceof Error ? err.message : 'Submission failed. Please try again.';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="max-w-[600px] mt-10 mx-auto px-6 pb-20 font-[system-ui,sans-serif] text-form-text">
	<div aria-hidden="true" class="absolute left-[-9999px] top-[-9999px] w-px h-px overflow-hidden">
		{#each honeypotFields as name (name)}
			<input
				type="text"
				{name}
				tabindex="-1"
				autocomplete="off"
				value={honeypotValues[name] ?? ''}
				oninput={(e) => { honeypotValues = { ...honeypotValues, [name]: (e.target as HTMLInputElement).value }; }}
			/>
		{/each}
	</div>

	{#if locales.length > 1 && currentStep === 0}
		<div class="flex justify-center gap-1.5 mb-6">
			{#each locales as code (code)}
				<button
					onclick={() => onlocalechange(code)}
					class="px-3 py-1 rounded-full text-sm font-medium transition-colors duration-100 cursor-pointer border-none
						{locale === code
							? 'bg-form-primary text-white'
							: 'bg-form-surface text-form-muted hover:bg-form-border-light hover:text-form-text-mid'}"
				>
					{new Intl.DisplayNames([code, 'en'], { type: 'language' }).of(code) ?? code}
				</button>
			{/each}
		</div>
	{/if}

	<p class="text-sm text-form-muted-light m-0 mb-4">Step {currentStep + 1} of {totalSteps}</p>

	{#if currentStep === 0}
		<h1 class="text-3xl font-bold m-0 mb-2">{translation?.formTitle ?? ''}</h1>
		{#if translation?.formDescription}
			<p class="m-0 mb-8 text-form-text-dim">{translation.formDescription}</p>
		{/if}
	{/if}

	<div class="flex flex-col gap-6">
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
		<p class="mt-6 m-0 text-danger-border text-base">{submitError}</p>
	{/if}

	<div class="flex justify-between items-center mt-8">
		{#if currentStep > 0}
			<button
				type="button"
				onclick={handleBack}
				class="px-6 py-2.5 bg-form-bg text-form-text-mid border-[1.5px] border-form-border rounded-md text-base font-[inherit] cursor-pointer hover:bg-form-surface transition-colors duration-100"
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
				class="px-6 py-2.5 text-white border-none rounded-md text-base font-[inherit] transition-colors duration-100
					{submitting ? 'bg-form-muted-light cursor-not-allowed' : 'bg-form-primary hover:bg-form-primary-hover cursor-pointer'}"
			>
				{submitting ? 'Submitting…' : 'Submit'}
			</button>
		{:else}
			<button
				type="button"
				onclick={handleNext}
				class="px-6 py-2.5 bg-form-primary text-white border-none rounded-md text-base font-[inherit] cursor-pointer hover:bg-form-primary-hover transition-colors duration-100"
			>
				Next →
			</button>
		{/if}
	</div>
</div>
