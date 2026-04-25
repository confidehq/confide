<script lang="ts">
	import type { BuilderSchema, BuilderField, ChoiceOption } from '$lib/types/builder';
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
		pgpPublicKey: string | null;
		schemaVersion: number;
		locale: string;
		locales: string[];
		honeypotFields: string[];
		loadToken: string;
		onsubmitted: () => void;
		onlocalechange: (code: string) => void;
	}

	const { schema, formId, publicFormKey, pgpPublicKey, schemaVersion, locale, locales, honeypotFields, loadToken, onsubmitted, onlocalechange }: Props = $props();

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

	function resolveChoiceLabel(field: BuilderField, optionId: string): string {
		const config = field.config as { options: ChoiceOption[] };
		const sorted = [...config.options].sort((a, b) => a.order - b.order);
		const idx = sorted.findIndex((o) => o.id === optionId);
		return translation?.fields[field.id]?.options?.[idx] ?? optionId;
	}

	function formatResponseForEmail(payload: ResponsePayload): string {
		const lines: string[] = [
			`Form response — ${new Date(payload.submittedAt).toLocaleString()}`,
			'',
		];
		for (const field of orderedFields) {
			if (field.type === 'section_break' || field.type === 'heading') continue;
			const label = translation?.fields[field.id]?.label ?? field.id;
			const raw = payload.answers[field.id];
			if (raw === undefined || raw === null) continue;
			const isChoice =
				field.type === 'multiple_choice' ||
				field.type === 'checkboxes' ||
				field.type === 'dropdown';
			let value: string;
			if (isChoice) {
				const ids = Array.isArray(raw) ? raw : [String(raw)];
				value = ids.map((id) => resolveChoiceLabel(field, id)).join(', ');
			} else {
				value = Array.isArray(raw) ? raw.join(', ') : String(raw);
			}
			lines.push(label);
			lines.push(value);
			lines.push('');
		}
		return lines.join('\n');
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
			const pgpPlaintext = pgpPublicKey ? formatResponseForEmail(payload) : undefined;
			await submitResponse(formId, publicFormKey, payload, schemaVersion, loadToken, honeypotValues, pgpPublicKey, pgpPlaintext);
			onsubmitted();
		} catch (err) {
			submitError = err instanceof Error ? err.message : 'Submission failed. Please try again.';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="max-w-[600px] mt-10 mx-auto px-6 pb-20 font-[system-ui,sans-serif] text-form-text">
	{#if locales.length > 1}
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
	<h1 class="text-3xl font-bold m-0 mb-2">{translation?.formTitle ?? ''}</h1>
	{#if translation?.formDescription}
		<p class="m-0 mb-8 text-form-text-dim">{translation.formDescription}</p>
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
			<p class="mt-6 m-0 text-danger-border text-base">{submitError}</p>
		{/if}

		<button
			type="submit"
			disabled={submitting}
			class="mt-8 px-8 py-3 text-white border-none rounded-md text-base font-[inherit] transition-colors duration-100
				{submitting ? 'bg-form-muted-light cursor-not-allowed' : 'bg-form-primary hover:bg-form-primary-hover cursor-pointer'}"
		>
			{submitting ? 'Submitting…' : 'Submit'}
		</button>
	</form>
</div>
