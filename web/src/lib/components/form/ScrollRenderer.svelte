<script lang="ts">
	import type { BuilderSchema, BuilderField, ChoiceOption } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
	import type { ResponsePayload } from '$lib/types/crypto';
	import { submitResponse } from '$lib/forms';
	import { validateAll } from '$lib/validation';
	import type { AnswerValue } from '$lib/validation';
	import FieldRenderer from './FieldRenderer.svelte';
	import { Languages, Lock } from '@lucide/svelte';

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

<div class="w-full max-w-3xl mt-8 mx-auto pb-20 px-6 sm:px-0 font-[system-ui,sans-serif] text-form-text">
	{#if locales.length > 1}
		<div class="flex justify-end mb-4">
			<!-- Mobile: compact with locale codes -->
			<div class="relative inline-flex items-center sm:hidden">
				<Languages class="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-form-muted pointer-events-none" />
				<select
					value={locale}
					onchange={(e) => onlocalechange((e.target as HTMLSelectElement).value)}
					class="appearance-none pl-7 pr-5 py-1 text-xs rounded-md border border-form-border bg-form-surface text-form-text-mid cursor-pointer focus:outline-none focus:ring-2 focus:ring-form-primary/40 uppercase"
				>
					{#each locales as code (code)}
						<option value={code}>{code.toUpperCase()}</option>
					{/each}
				</select>
				<svg class="absolute right-1.5 top-1/2 -translate-y-1/2 w-2.5 h-2.5 text-form-muted pointer-events-none" viewBox="0 0 12 12" fill="none">
					<path d="M2.5 4.5L6 8l3.5-3.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</div>
			<!-- Desktop: full language names -->
			<div class="relative hidden sm:inline-flex items-center">
				<Languages class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-form-muted pointer-events-none" />
				<select
					value={locale}
					onchange={(e) => onlocalechange((e.target as HTMLSelectElement).value)}
					class="appearance-none pl-8 pr-7 py-1.5 text-sm rounded-md border border-form-border bg-form-surface text-form-text-mid cursor-pointer focus:outline-none focus:ring-2 focus:ring-form-primary/40"
				>
					{#each locales as code (code)}
						<option value={code}>
							{new Intl.DisplayNames([code, 'en'], { type: 'language' }).of(code) ?? code}
						</option>
					{/each}
				</select>
				<svg class="absolute right-2 top-1/2 -translate-y-1/2 w-3 h-3 text-form-muted pointer-events-none" viewBox="0 0 12 12" fill="none">
					<path d="M2.5 4.5L6 8l3.5-3.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</div>
		</div>
	{/if}

	<div class="mb-8 sm:mb-10">
		{#if translation?.formHeadline}
			<p class="m-0 mb-2 sm:mb-3 text-sm font-semibold uppercase tracking-widest text-form-muted">{translation.formHeadline}</p>
		{/if}
		<h1 class="text-3xl sm:text-4xl font-bold m-0 mb-3 sm:mb-4 leading-tight whitespace-pre-wrap">{translation?.formTitle ?? ''}</h1>
		{#if translation?.formDescription}
			<div class="m-0 text-base leading-relaxed text-form-text-dim rich-html">{@html translation.formDescription}</div>
		{/if}
	</div>

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
			class="mt-8 w-full sm:w-auto px-8 py-3.5 sm:py-3 text-white border-none rounded-md text-base font-[inherit] transition-colors duration-100
				{submitting ? 'bg-form-muted-light cursor-not-allowed' : 'bg-form-primary hover:bg-form-primary-hover cursor-pointer'}"
		>
			<span class="inline-flex items-center justify-center gap-2">
				<Lock size={13} strokeWidth={2} class="opacity-70" />
				{submitting ? 'Submitting…' : (translation?.submitButtonText || 'Submit')}
			</span>
		</button>
	</form>

	{#if schema.showWatermark !== false}
		<a href="https://useconfide.app" target="_blank" rel="noopener noreferrer" class="sm:hidden flex justify-center items-center gap-1.5 mt-8 pt-4 border-t border-form-border text-xs text-form-muted no-underline hover:text-form-text-mid transition-colors duration-100">
			Made with
			<img src="/favicon.svg" alt="" class="w-4 h-4" />
			<span class="font-medium text-form-text-mid">Confide</span>
		</a>
		<div class="hidden sm:flex justify-end mt-6">
			<a href="https://useconfide.app" target="_blank" rel="noopener noreferrer">
				<img src="/watermark.svg" alt="Powered by Confide" class="w-[100px] opacity-70 hover:opacity-100 transition-opacity duration-100" />
			</a>
		</div>
	{/if}
</div>
