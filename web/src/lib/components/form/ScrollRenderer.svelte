<script lang="ts">
	import type { BuilderSchema, BuilderField } from '$lib/types/builder';
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
			const firstErrField = schema.fields.find((f) => allErrors[f.id]);
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

<div style="max-width: 600px; margin: 40px auto; padding: 0 24px 80px; font-family: system-ui, sans-serif; color: #111;">
	<h1 style="font-size: 1.5rem; font-weight: 700; margin: 0 0 8px;">{translation?.formTitle ?? ''}</h1>
	{#if translation?.formDescription}
		<p style="margin: 0 0 32px; color: #4b5563;">{translation.formDescription}</p>
	{/if}

	<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} novalidate>
		<div aria-hidden="true" style="position: absolute; left: -9999px; top: -9999px; width: 1px; height: 1px; overflow: hidden;">
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
		<div style="display: flex; flex-direction: column; gap: 24px;">
			{#each schema.fields as field (field.id)}
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

		<button
			type="submit"
			disabled={submitting}
			style="margin-top: 32px; padding: 12px 32px; background: {submitting ? '#9ca3af' : '#1d4ed8'}; color: white; border: none; border-radius: 6px; font-size: 1rem; font-family: inherit; cursor: {submitting ? 'not-allowed' : 'pointer'};"
		>
			{submitting ? 'Submitting…' : 'Submit'}
		</button>
	</form>
</div>
