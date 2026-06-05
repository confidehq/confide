<script lang="ts">
import { ShieldCheck } from "@lucide/svelte";
import { goto } from "$app/navigation";
import FieldRenderer from "$lib/components/form/FieldRenderer.svelte";
import { DEMO_FORM_SCHEMA } from "$lib/demo/data";
import { getOrderedFields } from "$lib/types/builder";
import type { AnswerValue } from "$lib/validation";
import { validateAll } from "$lib/validation";

const schema = DEMO_FORM_SCHEMA;
const locale = "en";
const translation = schema.translations[locale];
const orderedFields = getOrderedFields(schema, locale);

let answers = $state<Record<string, AnswerValue>>({});
let errors = $state<Record<string, string>>({});
let submitting = $state(false);

function setAnswer(fieldId: string, v: AnswerValue) {
	answers = { ...answers, [fieldId]: v };
	if (errors[fieldId]) {
		const next = { ...errors };
		delete next[fieldId];
		errors = next;
	}
}

function handleSubmit(e: Event) {
	e.preventDefault();
	const allErrors = validateAll(schema.fields, answers);
	if (Object.keys(allErrors).length > 0) {
		errors = allErrors;
		const firstErrField = orderedFields.find((f) => allErrors[f.id]);
		if (firstErrField) {
			document
				.getElementById(`field-${firstErrField.id}`)
				?.scrollIntoView({ behavior: "smooth", block: "center" });
		}
		return;
	}
	submitting = true;
	// Simulate a brief submission delay for realism
	setTimeout(() => goto("/demo/submitted"), 600);
}
</script>

<svelte:head>
	<title>Anonymous Incident Report — Demo</title>
</svelte:head>

<div class="form-shell">
	<div class="form-container">
		<!-- Form header -->
		<div class="form-header">
			<h1 class="form-title">{translation.formTitle}</h1>
			{#if translation.formDescription}
				<p class="form-desc">{translation.formDescription}</p>
			{/if}
		</div>

		<!-- Fields -->
		<form onsubmit={handleSubmit} novalidate>
			<div class="fields">
				{#each orderedFields as field (field.id)}
					<FieldRenderer
						{field}
						translation={translation.fields[field.id] ?? { label: field.id }}
						value={answers[field.id]}
						error={errors[field.id] ?? null}
						onchange={(v) => setAnswer(field.id, v)}
					/>
				{/each}
			</div>

			<div class="submit-row">
				<button type="submit" class="submit-btn" disabled={submitting}>
					{submitting ? "Submitting…" : (translation.submitButtonText ?? "Submit")}
				</button>
				<span class="encrypted-note">
					<ShieldCheck size={13} strokeWidth={2} />
					Encrypted locally before submission
				</span>
			</div>
		</form>
	</div>
</div>

<style>
	:global(html), :global(body) {
		background: #fff;
		color: #111;
	}

	.form-shell {
		background: #fff;
		min-height: 100%;
		padding: 2rem 1.5rem 5rem;
	}

	.form-container {
		max-width: 640px;
		margin: 0 auto;
	}

	.form-header {
		margin-bottom: 2.5rem;
	}

	.form-title {
		font-size: 1.875rem;
		font-weight: 700;
		margin: 0 0 0.75rem;
		color: #111;
		line-height: 1.25;
	}

	.form-desc {
		font-size: 0.9375rem;
		color: #6b7280;
		margin: 0;
		line-height: 1.6;
	}

	.fields {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.submit-row {
		margin-top: 2.5rem;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.submit-btn {
		padding: 0.75rem 1.75rem;
		background: #2563ea;
		color: #fff;
		border: none;
		border-radius: 8px;
		font-size: 0.9375rem;
		font-family: inherit;
		font-weight: 600;
		cursor: pointer;
		transition: background 0.1s;
		align-self: flex-start;
	}

	.submit-btn:hover:not(:disabled) {
		background: #1d4ed8;
	}

	.submit-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.encrypted-note {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.8125rem;
		color: #9ca3af;
	}
</style>
