<script lang="ts">
	import type { BuilderSchema, BuilderField } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
	import type { ResponsePayload } from '$lib/types/crypto';
	import { submitResponse } from '$lib/forms';
	import { validateAll } from '$lib/validation';
	import type { AnswerValue } from '$lib/validation';
	import FieldRenderer from './FieldRenderer.svelte';

	interface Props {
		schema: BuilderSchema;
		formId: string;
		publicFormKey: ArrayBuffer;
		schemaVersion: number;
		locale: string;
		honeypotFields: string[];
		loadToken: string;
		onsubmitted: () => void;
	}

	const { schema, formId, publicFormKey, schemaVersion, locale, honeypotFields, loadToken, onsubmitted }: Props = $props();

	const translation = $derived(
		schema.translations[locale] ?? schema.translations[schema.defaultLocale]
	);

	const orderedFields = $derived(getOrderedFields(schema, locale));

	let answers = $state<Record<string, AnswerValue>>({});
	let errors = $state<Record<string, string>>({});
	let submitting = $state(false);
	let submitError = $state<string | null>(null);
	let honeypotValues = $state<Record<string, string>>({});

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

	async function handleSubmit() {
		const allErrors = validateAll(schema.fields, answers);
		if (Object.keys(allErrors).length > 0) {
			errors = allErrors;
			const firstErrField = orderedFields.find((f) => allErrors[f.id]);
			if (firstErrField) {
				document
					.getElementById(`field-${firstErrField.id}`)
					?.scrollIntoView({ behavior: 'smooth', block: 'center' });
			}
			return;
		}

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

<div class="max-w-[600px] mt-10 mx-auto px-6 pb-20 font-[system-ui,sans-serif] text-[#111]">
	<h1 class="text-[1.5rem] font-bold m-0 mb-2">{translation?.formTitle ?? ''}</h1>
	{#if translation?.formDescription}
		<p class="m-0 mb-8 text-[#4b5563]">{translation.formDescription}</p>
	{/if}

	<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} novalidate>
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

		<div class="flex flex-col gap-6">
			{#each orderedFields as field (field.id)}
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
			<p class="mt-6 m-0 text-[#ef4444] text-[0.9rem]">{submitError}</p>
		{/if}

		<button
			type="submit"
			disabled={submitting}
			class="mt-8 px-8 py-3 text-white border-none rounded-md text-base font-[inherit] transition-colors duration-100
				{submitting ? 'bg-[#9ca3af] cursor-not-allowed' : 'bg-[#1d4ed8] hover:bg-[#1e40af] cursor-pointer'}"
		>
			{submitting ? 'Submitting…' : 'Submit'}
		</button>
	</form>
</div>
