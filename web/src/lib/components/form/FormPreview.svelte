<script lang="ts">
import {
	Bell,
	CircleCheck,
	Info,
	Lock,
	Shield,
	Star,
	TriangleAlert,
	Zap,
} from "@lucide/svelte";
import type {
	AccentIcon,
	BuilderField,
	BuilderSchema,
} from "$lib/types/builder";
import { getOrderedFields } from "$lib/types/builder";
import type { AnswerValue } from "$lib/validation";
import FieldRenderer from "./FieldRenderer.svelte";

const iconMap: Record<AccentIcon, typeof Lock> = {
	lock: Lock,
	shield: Shield,
	check: CircleCheck,
	info: Info,
	alert: TriangleAlert,
	star: Star,
	bell: Bell,
	zap: Zap,
};

interface Props {
	schema: BuilderSchema;
	locale: string;
}

const { schema, locale }: Props = $props();

const translation = $derived(
	schema.translations[locale] ?? schema.translations[schema.defaultLocale],
);

// Split fields into steps for 'steps' layout
function computeSteps(fields: BuilderField[]): BuilderField[][] {
	const groups: BuilderField[][] = [[]];
	for (const field of fields) {
		if (field.type === "section_break") {
			groups.push([field]);
		} else {
			groups[groups.length - 1].push(field);
		}
	}
	return groups.filter((g) => g.length > 0);
}

const isSteps = $derived(schema.layout === "steps");
const orderedFields = $derived(getOrderedFields(schema, locale));
const steps = $derived(computeSteps(orderedFields));
const totalSteps = $derived(steps.length);

let currentStep = $state(0);
let answers = $state<Record<string, AnswerValue>>({});

const isLastStep = $derived(currentStep === totalSteps - 1);
const currentFields = $derived(
	isSteps ? (steps[currentStep] ?? []) : orderedFields,
);

function fieldTranslation(fieldId: string) {
	return translation?.fields[fieldId] ?? { label: fieldId };
}

function setAnswer(fieldId: string, v: AnswerValue) {
	answers = { ...answers, [fieldId]: v };
}
</script>

<div class="w-full max-w-3xl mt-10 mx-auto pb-20 font-[system-ui,sans-serif] text-form-text">
	<!-- Preview banner -->
	<div class="mb-6 px-3.5 py-2 bg-form-preview-bg border border-form-preview-border rounded-md text-sm text-form-preview-text">
		Preview mode — responses will not be submitted
	</div>

	{#if isSteps}
		<p class="text-sm text-form-muted-light m-0 mb-4">Step {currentStep + 1} of {totalSteps}</p>
	{/if}

	{#if !isSteps || currentStep === 0}
		{#if translation?.formHeadline}
			<p class="m-0 mb-1 text-sm font-semibold uppercase tracking-widest text-form-muted">{translation.formHeadline}</p>
		{/if}
		<h1 class="text-3xl font-bold m-0 mb-2 whitespace-pre-wrap">{translation?.formTitle ?? ''}</h1>
		{#if translation?.formDescription}
			<div class="m-0 mb-8 text-form-text-dim rich-html">{@html translation.formDescription}</div>
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
				class="px-6 py-2.5 bg-form-bg text-form-text-mid border-[1.5px] border-form-border rounded-md text-base font-[inherit] cursor-pointer hover:bg-form-canvas transition-colors duration-100"
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
				class="px-6 py-2.5 bg-form-primary text-white border-none rounded-md text-base font-[inherit] cursor-pointer hover:bg-form-primary-hover transition-colors duration-100"
			>
				Next →
			</button>
		{:else}
			<button
				type="button"
				disabled
				class="px-6 py-2.5 bg-form-muted-light text-white border-none rounded-md text-base font-[inherit] cursor-not-allowed inline-flex items-center gap-2"
			>
				{#if schema.submitButtonIcon}
					{@const BtnIcon = iconMap[schema.submitButtonIcon]}
					<svelte:component this={BtnIcon} size={13} strokeWidth={2} class="opacity-70" />
				{/if}
				{translation?.submitButtonText || 'Submit'}
			</button>
		{/if}
	</div>

	{#if schema.legalText}
		<div class="mt-10 pt-4 border-t border-form-border">
			<p class="m-0 text-xs text-form-muted leading-relaxed">{schema.legalText}</p>
		</div>
	{/if}
</div>
