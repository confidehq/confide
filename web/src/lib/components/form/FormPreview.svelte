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

<div class="w-full max-w-3xl mt-8 mx-auto pb-20 px-6 sm:px-0 font-[system-ui,sans-serif] text-form-text">
	<!-- Preview banner -->
	<div class="mb-6 px-3.5 py-2 bg-form-preview-bg border border-form-preview-border rounded-md text-sm text-form-preview-text">
		Preview mode — responses will not be submitted
	</div>

	{#if isSteps}
		<p class="text-sm text-form-muted-light m-0 mb-4">Step {currentStep + 1} of {totalSteps}</p>
	{/if}

	{#if !isSteps || currentStep === 0}
		<div class="mb-8 sm:mb-10">
			{#if translation?.formHeadline}
				<p class="m-0 mb-2 sm:mb-3 text-sm font-semibold uppercase tracking-widest text-form-muted">{translation.formHeadline}</p>
			{/if}
			<h1 class="text-3xl sm:text-4xl font-bold m-0 mb-3 sm:mb-4 leading-tight whitespace-pre-wrap">{translation?.formTitle ?? ''}</h1>
			{#if translation?.formDescription}
				<div class="m-0 text-base leading-relaxed text-form-text-dim rich-html">{@html translation.formDescription}</div>
			{/if}
		</div>
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

	<div class="flex items-center mt-8
		{isSteps && !isLastStep ? 'justify-between' : 'justify-start gap-3'}">
		{#if isSteps && currentStep > 0}
			<button
				type="button"
				onclick={() => { currentStep = Math.max(currentStep - 1, 0); }}
				class="flex-1 sm:flex-none px-6 py-3 sm:py-2.5 bg-form-bg text-form-text-mid border-[1.5px] border-form-border rounded-md text-base font-[inherit] cursor-pointer hover:bg-form-canvas transition-colors duration-100"
			>
				← Back
			</button>
		{/if}

		{#if isSteps && !isLastStep}
			<button
				type="button"
				onclick={() => { currentStep = Math.min(currentStep + 1, totalSteps - 1); }}
				class="flex-1 sm:flex-none px-6 py-3 sm:py-2.5 bg-form-primary text-white border-none rounded-md text-base font-[inherit] cursor-pointer hover:bg-form-primary-hover transition-colors duration-100"
			>
				Next →
			</button>
		{:else}
			<button
				type="button"
				disabled
				class="w-full sm:w-auto px-8 py-3.5 sm:py-3 bg-form-muted-light text-white border-none rounded-md text-base font-[inherit] cursor-not-allowed inline-flex items-center justify-center gap-2"
			>
				{#if schema.submitButtonIcon}
					{@const BtnIcon = iconMap[schema.submitButtonIcon]}
					<BtnIcon size={13} strokeWidth={2} class="opacity-70" />
				{/if}
				{translation?.submitButtonText || 'Submit'}
			</button>
		{/if}
	</div>

	{#if schema.legalText || schema.showWatermark !== false}
		<div class="mt-10 pt-4 border-t border-form-border flex flex-col gap-3">
			{#if schema.legalText}
				<div class="m-0 text-xs text-form-muted leading-relaxed rich-html text-center">{@html schema.legalText}</div>
			{/if}
			{#if schema.showWatermark !== false}
				<a href="https://useconfide.app" target="_blank" rel="noopener noreferrer" class="sm:hidden flex justify-center items-center gap-1.5 text-xs text-form-muted no-underline hover:text-form-text-mid transition-colors duration-100">
					Made with
					<img src="/favicon.svg" alt="" class="w-4 h-4" />
					<span class="font-medium text-form-text-mid">Confide</span>
				</a>
				<div class="hidden sm:flex justify-end">
					<a href="https://useconfide.app" target="_blank" rel="noopener noreferrer">
						<img src="/watermark.svg" alt="Powered by Confide" class="w-[100px] opacity-70 hover:opacity-100 transition-opacity duration-100" />
					</a>
				</div>
			{/if}
		</div>
	{/if}
</div>
