<script lang="ts">
import {
	AlertCircle,
	AlignLeft,
	Bell,
	Calendar,
	CheckSquare,
	ChevronDown,
	ChevronRight,
	CircleCheck,
	CircleDot,
	Clock,
	Copy,
	GripVertical,
	Heading1,
	Info,
	Lock,
	Minus,
	Plus,
	Shield,
	Star,
	Trash2,
	TriangleAlert,
	Type,
	Zap,
} from "@lucide/svelte";
import type { Component } from "svelte";
import { tick } from "svelte";
import { dndzone } from "svelte-dnd-action";
import FormPreview from "$lib/components/form/FormPreview.svelte";
import type { createBuilderStore } from "$lib/stores/builder.svelte";
import type {
	AccentIcon,
	BuilderField,
	CheckboxesConfig,
	ChoiceOption,
	DropdownConfig,
	FieldType,
	MultipleChoiceConfig,
	RatingConfig,
} from "$lib/types/builder";
import { getOrderedFields } from "$lib/types/builder";

const submitIconMap: Record<AccentIcon, typeof Lock> = {
	lock: Lock,
	shield: Shield,
	check: CircleCheck,
	info: Info,
	alert: TriangleAlert,
	star: Star,
	bell: Bell,
	zap: Zap,
};

import RichEditable from "./RichEditable.svelte";

const fieldPalette: Array<{ type: FieldType; label: string; icon: Component }> =
	[
		{ type: "short_text", label: "Short text", icon: Type },
		{ type: "long_text", label: "Long text", icon: AlignLeft },
		{ type: "multiple_choice", label: "Multiple choice", icon: CircleDot },
		{ type: "checkboxes", label: "Checkboxes", icon: CheckSquare },
		{ type: "dropdown", label: "Dropdown", icon: ChevronDown },
		{ type: "date_time", label: "Date / time", icon: Calendar },
		{ type: "rating", label: "Rating", icon: Star },
		{ type: "section_break", label: "Section break", icon: Minus },
		{ type: "heading", label: "Heading", icon: Heading1 },
		{ type: "accordion", label: "Accordion", icon: ChevronRight },
		{ type: "accent", label: "Accent block", icon: AlertCircle },
	];

let insertSlot = $state<number | null>(null);
let popoverAnchor = $state<{ top: number; left: number } | null>(null);
let ratingHover = $state<{ fieldId: string; value: number } | null>(null);
let legalTextActive = $state(false);

function openSlot(
	e: MouseEvent,
	afterIndex: number,
	anchor: "left" | "above" = "left",
) {
	e.stopPropagation();
	const btn = e.currentTarget as HTMLElement;
	const rect = btn.getBoundingClientRect();
	const popoverH = 230;
	const popoverW = 288;
	const margin = 8;
	let left: number, top: number;
	if (anchor === "above") {
		left = Math.max(
			margin,
			Math.min(
				rect.left + rect.width / 2 - popoverW / 2,
				window.innerWidth - popoverW - margin,
			),
		);
		top =
			rect.top - popoverH - 8 > margin
				? rect.top - popoverH - 8
				: rect.bottom + 8;
	} else {
		left = Math.max(margin, rect.left - popoverW - 8);
		top = Math.max(
			margin,
			Math.min(
				rect.top + rect.height / 2 - popoverH / 2,
				window.innerHeight - popoverH - margin,
			),
		);
	}
	insertSlot = afterIndex;
	popoverAnchor = { top, left };
}

function closeSlot() {
	insertSlot = null;
	popoverAnchor = null;
}

function pickField(type: FieldType) {
	if (insertSlot === null) return;
	store.addFieldAt(type, insertSlot);
	closeSlot();
}

interface Props {
	store: ReturnType<typeof createBuilderStore>;
}

const { store }: Props = $props();

const FIELD_TYPE_LABELS: Record<string, string> = {
	short_text: "Short text",
	long_text: "Long text",
	multiple_choice: "Multiple choice",
	checkboxes: "Checkboxes",
	dropdown: "Dropdown",
	date_time: "Date / time",
	rating: "Rating",
	section_break: "Section break",
	heading: "Heading",
	accordion: "Accordion",
	accent: "Accent block",
};

// Local state for dnd — holds shadow items during drag without touching the store.
// Synced from the store via $effect; committed back only on finalize.
let fields = $state<BuilderField[]>([]);
// Drag is only allowed when the pointer went down on a field's top toolbar.
let dragEnabled = $state(false);

$effect(() => {
	fields = getOrderedFields(store.schema, store.activeLocale);
});

// Reset drag permission on any pointer release anywhere on the page.
$effect(() => {
	function reset() { dragEnabled = false; }
	window.addEventListener('pointerup', reset);
	return () => window.removeEventListener('pointerup', reset);
});

// Scroll newly added fields into view.
let prevFieldIds = new Set<string>();
$effect(() => {
	const currentIds = new Set(fields.map((f) => f.id));
	const addedId = [...currentIds].find((id) => !prevFieldIds.has(id));
	if (addedId && prevFieldIds.size > 0) {
		tick().then(() => {
			document
				.querySelector(`[data-field-id="${addedId}"]`)
				?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
		});
	}
	prevFieldIds = currentIds;
});

function handleDndConsider(e: CustomEvent<{ items: BuilderField[] }>) {
	fields = e.detail.items;
}

function handleDndFinalize(e: CustomEvent<{ items: BuilderField[] }>) {
	dragEnabled = false;
	store.reorderFields(e.detail.items);
}

function getLabel(fieldId: string): string {
	return (
		store.schema.translations[store.activeLocale]?.fields[fieldId]?.label ?? ""
	);
}

function getHelpText(fieldId: string): string {
	return (
		store.schema.translations[store.activeLocale]?.fields[fieldId]?.helpText ??
		""
	);
}

function getDefaultLabel(fieldId: string): string {
	if (store.activeLocale === store.schema.defaultLocale) return "";
	return (
		store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]
			?.label ?? ""
	);
}

function getDefaultHelpText(fieldId: string): string {
	if (store.activeLocale === store.schema.defaultLocale) return "";
	return (
		store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]
			?.helpText ?? ""
	);
}

function getPlaceholder(fieldId: string): string {
	return (
		store.schema.translations[store.activeLocale]?.fields[fieldId]
			?.placeholder ?? ""
	);
}

function getDefaultPlaceholder(fieldId: string): string {
	if (store.activeLocale === store.schema.defaultLocale) return "";
	return (
		store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]
			?.placeholder ?? ""
	);
}

function autoGrow(el: HTMLTextAreaElement) {
	el.style.height = "auto";
	el.style.height = el.scrollHeight + "px";
}

function growable(el: HTMLTextAreaElement, value: string) {
	autoGrow(el);
	return {
		update() {
			autoGrow(el);
		},
	};
}

function getOptionLabels(fieldId: string): string[] {
	const field = store.schema.fields.find((f) => f.id === fieldId);
	if (!field) return [];
	const cfg = field.config as
		| MultipleChoiceConfig
		| CheckboxesConfig
		| DropdownConfig;
	const count = cfg.options?.length ?? 0;
	const translated =
		store.schema.translations[store.activeLocale]?.fields[fieldId]?.options;
	return Array.from({ length: count }, (_, i) => translated?.[i] ?? "");
}

function getDefaultOptionLabel(fieldId: string, index: number): string {
	if (store.activeLocale === store.schema.defaultLocale) return "";
	return (
		store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]
			?.options?.[index] ?? ""
	);
}

function setOptionLabel(fieldId: string, index: number, value: string) {
	const field = store.schema.fields.find((f) => f.id === fieldId);
	if (!field) return;
	const cfg = field.config as
		| MultipleChoiceConfig
		| CheckboxesConfig
		| DropdownConfig;
	const count = cfg.options?.length ?? 0;
	const current =
		store.schema.translations[store.activeLocale]?.fields[fieldId]?.options ??
		Array(count).fill("");
	const updated = [...current];
	while (updated.length <= index) updated.push("");
	updated[index] = value;
	store.updateTranslation(fieldId, "options", updated as unknown as string);
}

function addOption(fieldId: string) {
	const field = store.schema.fields.find((f) => f.id === fieldId);
	if (!field) return;
	const cfg = field.config as
		| MultipleChoiceConfig
		| CheckboxesConfig
		| DropdownConfig;
	const options = cfg.options ?? [];
	const newOpt: ChoiceOption = {
		id: crypto.randomUUID(),
		order: options.length,
	};
	store.updateFieldConfig(fieldId, {
		options: [...options, newOpt],
	} as Partial<MultipleChoiceConfig>);
}

function removeOption(fieldId: string, optId: string) {
	const field = store.schema.fields.find((f) => f.id === fieldId);
	if (!field) return;
	const cfg = field.config as
		| MultipleChoiceConfig
		| CheckboxesConfig
		| DropdownConfig;
	const removedIndex = (cfg.options ?? []).findIndex((o) => o.id === optId);
	const options = (cfg.options ?? [])
		.filter((o) => o.id !== optId)
		.map((o, i) => ({ ...o, order: i }));
	store.updateFieldConfig(fieldId, {
		options,
	} as Partial<MultipleChoiceConfig>);
	// Trim the translation options array to match
	if (removedIndex !== -1) {
		const current =
			store.schema.translations[store.activeLocale]?.fields[fieldId]?.options ??
			[];
		const updated = current.filter((_, i) => i !== removedIndex);
		store.updateTranslation(fieldId, "options", updated as unknown as string);
	}
}
</script>

<main
	style="background: {store.mode === 'preview' ? 'var(--color-form-canvas)' : 'var(--color-canvas)'};"
	class="flex-1 overflow-y-auto px-4 pt-6 pb-48 sm:px-6 min-w-0"
	onclick={() => { store.setSelectedField(null); store.setSubmitButtonSelected(false); closeSlot(); }}
	role="presentation"
>
	{#if store.mode === 'preview'}
		<FormPreview schema={store.schema} locale={store.activeLocale} />
	{:else}
{@const defaultLocaleHeadline = store.activeLocale !== store.schema.defaultLocale
	? (store.schema.translations[store.schema.defaultLocale]?.formHeadline ?? '')
		: ''}
{@const defaultLocaleTitle = store.activeLocale !== store.schema.defaultLocale
	? (store.schema.translations[store.schema.defaultLocale]?.formTitle ?? '')
		: ''}
{@const defaultLocaleDesc = store.activeLocale !== store.schema.defaultLocale
	? (store.schema.translations[store.schema.defaultLocale]?.formDescription ?? '')
		: ''}
	<div class="max-w-4xl mx-auto w-full">
		<!-- Form headline, title and description -->
		<div
			onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
			class="mb-5"
		>
			<textarea
				rows={1}
				use:growable={store.activeTranslation?.formHeadline ?? ''}
				value={store.activeTranslation?.formHeadline ?? ''}
				placeholder={defaultLocaleHeadline || 'Headline…'}
				onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
				oninput={(e) => {
					const el = e.target as HTMLTextAreaElement;
					autoGrow(el);
					store.updateTranslation(null, 'formHeadline', el.value);
				}}
				style="color: {store.activeTranslation?.formHeadline ? 'var(--color-subtle)' : 'var(--color-border)'};"
				class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-sm font-semibold uppercase tracking-widest font-[inherit] px-1 py-0.5 mb-1"
			></textarea>
			<textarea
				rows={1}
				use:growable={store.activeTranslation?.formTitle ?? ''}
				value={store.activeTranslation?.formTitle ?? ''}
				placeholder={defaultLocaleTitle || 'Form title…'}
				onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
				oninput={(e) => {
					const el = e.target as HTMLTextAreaElement;
					autoGrow(el);
					store.updateTranslation(null, 'formTitle', el.value);
				}}
				style="color: {store.activeTranslation?.formTitle ? 'var(--color-text)' : 'var(--color-text)'};"
				class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-3xl font-semibold font-[inherit] px-1 py-0.5 mb-1.5"
			></textarea>
			<div class="group/desc relative">
				<RichEditable
					value={store.activeTranslation?.formDescription ?? ''}
					placeholder={defaultLocaleDesc || 'Form description…'}
					onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
					onfocus={() => store.setSelectedField(null)}
					style="color: {store.activeTranslation?.formDescription ? 'var(--color-subtle)' : 'var(--color-border)'};"
					class="block w-full box-border text-base font-[inherit] px-1 py-0.5"
					onchange={(html) => store.updateTranslation(null, 'formDescription', html)}
				/>
				{#if store.activeTranslation?.formDescription}
					<button
						onclick={(e) => { e.stopPropagation(); store.updateTranslation(null, 'formDescription', ''); }}
						class="absolute top-1 right-0 p-1 opacity-0 group-hover/desc:opacity-100 transition-opacity text-[var(--color-border)] hover:text-[var(--color-subtle)] cursor-pointer bg-transparent border-none"
						title="Clear description"
					>
						<Trash2 size={14} strokeWidth={1.75} />
					</button>
				{/if}
			</div>
		</div>

		<!-- Backdrop to close popover -->
		{#if insertSlot !== null}
			<div onclick={closeSlot} class="fixed inset-0 z-40"></div>
		{/if}

		{#if fields.length === 0}
		<div class="flex flex-col items-center justify-center min-h-72 border-2 border-dashed border-border rounded-lg text-subtle">
			<button
				onclick={(e) => openSlot(e, -1)}
				class="flex items-center gap-2 bg-transparent border border-dashed border-border rounded-md text-subtle cursor-pointer font-mono text-sm px-4 py-2.5 transition-[color,border-color] duration-100 hover:text-subtle hover:border-text-subtle"
			>
				<Plus size={14} strokeWidth={2} />
				Add first field
			</button>
		</div>
	{:else}
		<div
			use:dndzone={{ items: fields, flipDurationMs: 150, dragDisabled: !dragEnabled }}
			onconsider={handleDndConsider}
			onfinalize={handleDndFinalize}
			class="flex flex-col gap-5 min-h-24"
		>
			{#each fields as field, fieldIndex (field.id)}
				{@const isSelected = store.selectedFieldId === field.id}
				{@const isSectionBreak = field.type === 'section_break'}
				{@const hasPlaceholder = field.type === 'short_text' || field.type === 'long_text'}
				{@const hasOptions = field.type === 'multiple_choice' || field.type === 'checkboxes' || field.type === 'dropdown'}
				{@const isRating = field.type === 'rating'}
				{@const isDateTime = field.type === 'date_time'}
				{@const isHeading = field.type === 'heading'}
				{@const isAccent = field.type === 'accent'}
				{@const label = getLabel(field.id)}
				{@const helpText = getHelpText(field.id)}
				{@const placeholder = getPlaceholder(field.id)}
				{@const defaultLabel = getDefaultLabel(field.id)}
				{@const defaultHelpText = getDefaultHelpText(field.id)}
				{@const defaultPlaceholder = getDefaultPlaceholder(field.id)}
				<div
					data-field-id={field.id}
					class="relative group/card"
					role="none"
				>
				<!-- Insert before zone: only on first card, sits in space above -->
				{#if fieldIndex === 0}
					<div
						class="absolute left-6 right-6 h-5 flex items-center z-20 group/insert-top cursor-pointer"
						style="bottom: 100%;"
						onclick={(e) => { e.stopPropagation(); openSlot(e, -1, 'above'); }}
						role="none"
					>
						<div class="flex-1 h-px opacity-0 group-hover/insert-top:opacity-30 transition-opacity duration-150" style="background: var(--color-primary);"></div>
						<div
							class="opacity-0 group-hover/insert-top:opacity-100 shrink-0 mx-2 w-5 h-5 rounded-full transition-opacity duration-150 flex items-center justify-center p-0 shadow-sm pointer-events-none"
							style="background: var(--color-primary); color: white;"
						>
							<Plus size={10} strokeWidth={2.5} />
						</div>
						<div class="flex-1 h-px opacity-0 group-hover/insert-top:opacity-30 transition-opacity duration-150" style="background: var(--color-primary);"></div>
					</div>
				{/if}
				{#if isSectionBreak}
					<!-- Section break -->
					<div
						role="none"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border)'}; cursor: default;"
						class="px-3 pt-2 pb-3 border rounded-md"
					>
						<div class="flex items-center gap-2 mb-2 cursor-grab" onpointerdown={() => { dragEnabled = true; }}>
							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								class="bg-transparent border-none text-subtle cursor-grab shrink-0 flex p-0"
								aria-label="Field settings"
							><GripVertical size={15} strokeWidth={1.75} /></button>
							<span class="px-1.5 py-px bg-border text-subtle rounded-full text-xs font-mono shrink-0 select-none">section</span>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.duplicateField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Duplicate field" title="Duplicate field"
							><Copy size={15} strokeWidth={1.75} /></button>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>
						<RichEditable
							value={label}
							placeholder={defaultLabel || 'Section label…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
							onkeydown={(e) => { if (e.key === 'Enter') e.preventDefault(); }}
							style="color: {label ? 'var(--color-subtle)' : undefined};"
							class="block w-full box-border text-sm font-mono font-[inherit] px-1 py-0.5 mb-2.5 cursor-text"
							onchange={(html) => store.updateTranslation(field.id, 'label', html)}
						/>
						<div class="border-t border-dashed" style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border)'}; opacity: 0.6;"></div>
					</div>
				{:else if isHeading}
					{@const headingCfg = field.config as import('$lib/types/builder').HeadingConfig}
					{@const headingLevel = headingCfg.level ?? 2}
					{@const headingSizes = ['0.9375rem', '2rem', '1.35rem', '1rem', '0.9375rem']}
					{@const headingWeights = ['400', '700', '700', '600', '400']}
					{@const headingBadge = headingLevel === 0 ? 'paragraph' : headingLevel === 4 ? 'caption' : `h${headingLevel}`}
					<!-- Heading block -->
					<div
						role="none"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border)'}; background: {isSelected ? 'var(--color-canvas)' : 'var(--color-canvas)'}; cursor: default;"
						class="px-3 py-2 border rounded-md"
					>
						<div class="flex items-center gap-2 mb-1.5 cursor-grab" onpointerdown={() => { dragEnabled = true; }}>
							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								class="bg-transparent border-none text-subtle cursor-grab shrink-0 flex p-0"
								aria-label="Field settings"
							><GripVertical size={15} strokeWidth={1.75} /></button>
							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								class="px-1.5 py-px bg-surface text-subtle rounded-full text-xs shrink-0 border-none font-mono cursor-pointer hover:bg-border transition-colors duration-100"
								aria-label="Open field settings"
							>{headingBadge}</button>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.duplicateField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Duplicate field" title="Duplicate field"
							><Copy size={15} strokeWidth={1.75} /></button>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>
						<RichEditable
							value={label}
							placeholder={defaultLabel || 'Heading text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
							onkeydown={(e) => { if (e.key === 'Enter') e.preventDefault(); }}
							style="color: {headingLevel === 4 ? (label ? 'var(--color-subtle)' : 'var(--color-border)') : (label ? 'var(--color-text)' : undefined)}; font-size: {headingSizes[headingLevel]}; font-weight: {headingWeights[headingLevel]};"
							class="block w-full box-border cursor-text font-[inherit] px-1 py-0.5 leading-relaxed"
							onchange={(html) => store.updateTranslation(field.id, 'label', html)}
						/>
					</div>

				{:else if isAccent}
					{@const accentVariant = (field.config as import('$lib/types/builder').AccentConfig).variant ?? 'note'}
					{@const accentColors = {
						note:    { border: 'var(--color-info-border)',    bg: 'var(--color-info-bg-subtle)',     badge: 'var(--color-info-border)',    badgeBg: 'var(--color-info-bg-dark)' },
						warning: { border: 'var(--color-warning-border)', bg: 'var(--color-warning-bg-subtle)',  badge: 'var(--color-warning-border)', badgeBg: 'var(--color-warning-bg-dark)' },
						danger:  { border: 'var(--color-danger-border)',  bg: 'var(--color-danger-bg-subtle)',   badge: 'var(--color-danger-border)',  badgeBg: 'var(--color-danger-bg-deep)' },
						success: { border: 'var(--color-success-border)', bg: 'var(--color-success-bg-subtle)',  badge: 'var(--color-success-border)', badgeBg: 'var(--color-success-bg-deep)' },
					}[accentVariant]}
					<!-- Accent block -->
					<div
						role="none"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						style="
							border-left: 3px solid {accentColors.border};
							border-top: 1px solid {isSelected ? 'var(--color-primary)' : 'var(--color-info-border-subtle)'};
							border-right: 1px solid {isSelected ? 'var(--color-primary)' : 'var(--color-info-border-subtle)'};
							border-bottom: 1px solid {isSelected ? 'var(--color-primary)' : 'var(--color-info-border-subtle)'};
							background: {accentColors.bg};
							cursor: default;
						"
						class="px-3 py-2.5 rounded-r-md"
					>
						<div class="flex items-center gap-2 mb-1.5 cursor-grab" onpointerdown={() => { dragEnabled = true; }}>
							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								class="bg-transparent border-none text-subtle cursor-grab shrink-0 flex p-0"
								aria-label="Field settings"
							><GripVertical size={15} strokeWidth={1.75} /></button>
							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								style="background: {accentColors.badgeBg}; color: {accentColors.badge}; border-color: {accentColors.border}44;"
								class="px-1.5 py-px border rounded-full text-xs shrink-0 font-mono cursor-pointer hover:opacity-80 transition-opacity duration-100"
								aria-label="Open field settings"
							>{accentVariant}</button>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.duplicateField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Duplicate field" title="Duplicate field"
							><Copy size={15} strokeWidth={1.75} /></button>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>
						<RichEditable
							value={label}
							placeholder={defaultLabel || 'Title…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
							onkeydown={(e) => { if (e.key === 'Enter') e.preventDefault(); }}
							style="color: {label ? accentColors.badge : undefined};"
							class="block w-full box-border cursor-text text-base font-semibold font-[inherit] px-1 py-0.5 mb-0.5"
							onchange={(html) => store.updateTranslation(field.id, 'label', html)}
						/>
						<RichEditable
							value={helpText}
							placeholder={defaultHelpText || 'Body text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
							style="color: {helpText ? 'var(--color-text)' : 'var(--color-text)'};"
							class="block w-full box-border cursor-text text-sm font-[inherit] px-1 py-0.5 leading-relaxed"
							onchange={(html) => store.updateTranslation(field.id, 'helpText', html)}
						/>
					</div>

				{:else}
					<!-- Regular field card: vertical layout with inline editable label + help text -->
					<div
						role="none"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border)'}; background: {isSelected ? 'var(--color-canvas)' : 'var(--color-canvas)'}; cursor: default;"
						class="px-4 py-3.5 border rounded-md"
					>
						<!-- Top row: drag handle, type badge, required badge, warning, delete -->
						<div class="flex items-center gap-2 mb-3 cursor-grab" onpointerdown={() => { dragEnabled = true; }}>
							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								class="bg-transparent border-none text-subtle cursor-grab shrink-0 flex p-0"
								aria-label="Field settings"
							><GripVertical size={15} strokeWidth={1.75} /></button>

							<button
								onclick={(e) => { e.stopPropagation(); store.setSelectedField(store.selectedFieldId === field.id ? null : field.id); }}
								class="px-2 py-0.5 bg-surface text-subtle rounded-full text-xs shrink-0 border-none font-mono cursor-pointer hover:bg-border transition-colors duration-100"
								aria-label="Open field settings"
							>
								{FIELD_TYPE_LABELS[field.type] ?? field.type}
							</button>

							<span class="flex-1"></span>

							{#if !label}
								<span title="Missing translation for {store.activeLocale}" class="text-warn shrink-0 flex">
									<TriangleAlert size={15} strokeWidth={1.75} />
								</span>
							{/if}

							<button
								onclick={(e) => { e.stopPropagation(); store.updateField(field.id, { required: !field.required }); }}
								title={field.required ? 'Mark as optional' : 'Mark as required'}
								style={field.required ? 'background: var(--color-info-dark); color: var(--color-info);' : 'background: transparent; color: var(--color-subtle);'}
								class="px-1.5 py-0.5 border-none rounded-full text-xs shrink-0 cursor-pointer font-mono transition-colors duration-100 hover:opacity-80"
							>{field.required ? 'required' : 'optional'}</button>

							<button
								onclick={(e) => { e.stopPropagation(); store.duplicateField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Duplicate field" title="Duplicate field"
							><Copy size={15} strokeWidth={1.75} /></button>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-subtle cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>

						<!-- Label inline editor -->
						<RichEditable
							value={label}
							placeholder={defaultLabel || 'Label…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
							onkeydown={(e) => { if (e.key === 'Enter') e.preventDefault(); }}
							style="color: var(--color-text);"
							class="block w-full box-border text-base font-[inherit] px-1 py-0.5 mb-2 cursor-text rounded transition-colors duration-100"
							onchange={(html) => store.updateTranslation(field.id, 'label', html)}
						/>

						<!-- Help text inline editor -->
						<RichEditable
							value={helpText}
							placeholder={defaultHelpText || 'Add help text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
							style="color: {helpText ? 'var(--color-subtle)' : 'var(--color-text)'};"
							class="block w-full box-border text-sm font-[inherit] px-1 py-0.5 cursor-text rounded transition-colors duration-100"
							onchange={(html) => store.updateTranslation(field.id, 'helpText', html)}
						/>

						<!-- Placeholder inline editor (text fields only) -->
						{#if hasPlaceholder}
							<div class="mt-3">
								<div class="bg-surface border border-border rounded px-1.5 pt-0.5 pb-0.5">
									<textarea
										rows={1}
										value={placeholder}
										placeholder={defaultPlaceholder || 'e.g. Enter your answer…'}
										onclick={(e) => e.stopPropagation()}
										onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
										oninput={(e) => {
											const el = e.target as HTMLTextAreaElement;
											autoGrow(el);
											store.updateTranslation(field.id, 'placeholder', el.value);
										}}
										style="color: {placeholder ? 'var(--color-subtle)' : 'var(--color-muted)'};"
										class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-sm font-[inherit] py-1 cursor-text italic"
									></textarea>
								</div>
							</div>
						{/if}

						<!-- Options inline editor (multiple_choice / checkboxes / dropdown) -->
						{#if hasOptions}
							{@const optionLabels = getOptionLabels(field.id)}
							{@const isMultiple = field.type === 'multiple_choice'}
							{@const isCheckbox = field.type === 'checkboxes'}
							<div
								onclick={(e) => e.stopPropagation()}
								class="mt-3 border-t border-border pt-3 flex flex-col gap-0.5"
							>
								{#each optionLabels as optLabel, i}
									{@const opt = (field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig).options?.[i]}
									<div
										class="flex items-center gap-2 px-1.5 py-1 rounded transition-[background] duration-100 hover:bg-surface"
										role="none"
									>
										<!-- Type indicator -->
										{#if isMultiple}
											<span class="inline-block shrink-0 w-3 h-3 border-[1.5px] border-text-subtle rounded-full"></span>
										{:else if isCheckbox}
											<span class="inline-block shrink-0 w-3 h-3 border-[1.5px] border-text-subtle rounded-sm"></span>
										{:else}
											<span class="text-subtle text-xs font-mono shrink-0 w-3.5 text-right">{i + 1}.</span>
										{/if}
										<input
											type="text"
											value={optLabel}
											placeholder={getDefaultOptionLabel(field.id, i) || `Option ${i + 1}`}
											onfocus={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
											oninput={(e) => setOptionLabel(field.id, i, (e.target as HTMLInputElement).value)}
											style="color: {optLabel ? 'var(--color-text)' : 'var(--color-text)'};"
											class="flex-1 min-w-0 bg-transparent border-none outline-none text-sm font-[inherit] py-px"
										/>
										<button
											onclick={(e) => { e.stopPropagation(); if (opt) removeOption(field.id, opt.id); }}
											class="bg-transparent border-none text-border cursor-pointer flex items-center px-0.5 shrink-0 hover:text-subtle transition-colors duration-100"
											aria-label="Remove option" title="Remove option"
										><Trash2 size={15} strokeWidth={1.75} /></button>
									</div>
								{/each}
								{#if isMultiple && (field.config as MultipleChoiceConfig).allowOther}
									<div class="flex items-center gap-2 px-1.5 py-1 rounded opacity-50 select-none">
										<span class="inline-block shrink-0 w-3 h-3 border-[1.5px] border-text-subtle rounded-full"></span>
										<span class="text-sm font-[inherit] text-text py-px">Other…</span>
									</div>
								{/if}
								<button
									onclick={(e) => { e.stopPropagation(); addOption(field.id); }}
									class="self-start bg-transparent border-none text-subtle text-sm cursor-pointer font-[inherit] px-1.5 py-1 mt-0.5 rounded transition-colors duration-100 hover:text-subtle"
								>+ Add option</button>
							</div>
						{/if}

						<!-- Rating preview -->
						{#if isRating}
							{@const cfg = field.config as RatingConfig}
							{@const scale = cfg.scale ?? 5}
							{@const activeUp = ratingHover?.fieldId === field.id ? ratingHover.value : 0}
							<div class="mt-3 border-t border-border pt-3">
								<div
									class="flex gap-1.5 flex-wrap items-center"
									onmouseleave={() => ratingHover = null}
									role="none"
								>
									{#each { length: scale } as _, i}
										{@const lit = i < activeUp}
										{#if cfg.shape === 'number'}
											<span
												style="
													border-color: {lit ? 'var(--color-info-border)' : 'var(--color-border)'};
													background: var(--color-canvas);
													color: {lit ? 'var(--color-text)' : 'var(--color-subtle)'};
												"
												class="inline-flex items-center justify-center w-8 h-8 border rounded-md text-sm font-mono cursor-default transition-[background,border-color,color] duration-100"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>{i + 1}</span>
										{:else}
											<span
												style="color: {lit ? 'var(--color-warning-border)' : 'var(--color-text)'};"
												class="text-2xl leading-none cursor-default transition-colors duration-100"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>★</span>
										{/if}
									{/each}
									<span class="text-border text-xs ml-1">/ {scale}</span>
								</div>
							</div>
						{/if}

						<!-- Date / time preview -->
						{#if isDateTime}
							{@const cfg = field.config as import('$lib/types/builder').DateTimeConfig}
							{@const mode = cfg.mode ?? 'date'}
							<div class="mt-3 border-t border-border pt-3">
								<div class="flex gap-2">
									{#if mode === 'date' || mode === 'datetime'}
										<div class="flex-1 flex items-center gap-2 bg-surface border border-border rounded px-2.5 py-1.5">
											<span class="text-muted flex shrink-0">
												<Calendar size={14} strokeWidth={1.75} />
											</span>
											<span class="text-muted text-sm font-mono tracking-[0.04em]">MM / DD / YYYY</span>
										</div>
									{/if}
									{#if mode === 'time' || mode === 'datetime'}
										<div
											style="flex: {mode === 'datetime' ? '0 0 auto' : '1'};"
											class="flex items-center gap-2 bg-surface border border-border rounded px-2.5 py-1.5"
										>
											<span class="text-muted flex shrink-0">
												<Clock size={14} strokeWidth={1.75} />
											</span>
											<span class="text-muted text-sm font-mono tracking-[0.04em]">HH : MM</span>
										</div>
									{/if}
								</div>
							</div>
						{/if}
					</div>
				{/if}

				<!-- Insert after zone: sits in the gap below this card -->
				<div
					class="absolute left-6 right-6 h-5 flex items-center z-20 group/insert cursor-pointer"
					style="top: 100%;"
					onclick={(e) => { e.stopPropagation(); openSlot(e, fieldIndex, 'above'); }}
					role="none"
				>
					<div class="flex-1 h-px opacity-0 group-hover/insert:opacity-30 transition-opacity duration-150" style="background: var(--color-primary);"></div>
					<div
						class="opacity-0 group-hover/insert:opacity-100 shrink-0 mx-2 w-5 h-5 rounded-full transition-opacity duration-150 flex items-center justify-center p-0 shadow-sm pointer-events-none"
						style="background: var(--color-primary); color: white;"
					>
						<Plus size={10} strokeWidth={2.5} />
					</div>
					<div class="flex-1 h-px opacity-0 group-hover/insert:opacity-30 transition-opacity duration-150" style="background: var(--color-primary);"></div>
				</div>

			</div>
			{/each}
		</div>
	{/if}

	</div>
	{/if}

	<!-- Submit button inline editor -->
	{#if store.mode !== 'preview' && fields.length > 0}
		<div
			data-field-id="__submit__"
			class="max-w-4xl mx-auto w-full mt-14 mb-6"
			onclick={(e) => { e.stopPropagation(); store.setSubmitButtonSelected(true); }}
			role="none"
		>
			<div
				class="inline-flex items-center gap-2 px-8 py-3 rounded-md transition-[outline] duration-100"
				class:bg-form-primary={!store.submitButtonSelected}
				class:bg-form-primary-hover={store.submitButtonSelected}
				style={store.submitButtonSelected ? 'outline: 2px solid var(--color-primary); outline-offset: 2px;' : ''}
			>
				{#if store.schema.submitButtonIcon}
					{@const BtnIcon = submitIconMap[store.schema.submitButtonIcon]}
					<BtnIcon size={14} class="opacity-80 text-white shrink-0" />
				{/if}
				<input
					type="text"
					value={store.activeTranslation?.submitButtonText ?? ''}
					placeholder="Submit"
					oninput={(e) => store.updateTranslation(null, 'submitButtonText', (e.target as HTMLInputElement).value)}
					style="width: {Math.max((store.activeTranslation?.submitButtonText || 'Submit').length, 6)}ch"
					class="bg-transparent border-none outline-none text-base font-[inherit] text-white text-center cursor-text placeholder:text-white min-w-[6ch]"
				/>
			</div>
		</div>
	{/if}

	<!-- Legal / Impressum inline editor -->
	{#if store.mode !== 'preview'}
		<div class="max-w-4xl mx-auto w-full mt-4">
			{#if store.schema.legalText || legalTextActive}
				<div class="group/legal relative">
					<RichEditable
						value={store.schema.legalText ?? ''}
						placeholder="Legal / Impressum…"
						onclick={(e) => e.stopPropagation()}
						onfocus={() => { legalTextActive = true; store.setSelectedField(null); }}
						style="color: {store.schema.legalText ? 'var(--color-subtle)' : 'var(--color-border)'}; text-align: center;"
						class="block w-full box-border text-xs font-[inherit] px-1 py-0.5"
						onchange={(html) => store.setLegalText(html || undefined)}
					/>
					{#if store.schema.legalText}
						<button
							onclick={(e) => { e.stopPropagation(); store.setLegalText(undefined); legalTextActive = false; }}
							class="absolute top-0 right-0 p-1 opacity-0 group-hover/legal:opacity-100 transition-opacity text-[var(--color-border)] hover:text-[var(--color-subtle)] cursor-pointer bg-transparent border-none"
							title="Remove legal text"
						>
							<Trash2 size={14} strokeWidth={1.75} />
						</button>
					{/if}
				</div>
			{:else}
				<button
					onclick={(e) => { e.stopPropagation(); legalTextActive = true; }}
					class="w-full bg-transparent border-none text-[var(--color-border)] text-xs cursor-pointer font-[inherit] py-1 text-center hover:text-[var(--color-subtle)] transition-colors duration-150"
				>
					+ Legal / Impressum
				</button>
			{/if}
		</div>
	{/if}

	<!-- Field type popover (fixed, dismisses on backdrop click) -->
	{#if insertSlot !== null && popoverAnchor}
		<div
			style="top: {popoverAnchor.top}px; left: {popoverAnchor.left}px;"
			class="fixed bg-canvas border border-border rounded-lg p-1.5 z-50 shadow-[0_8px_32px_var(--color-overlay)] grid grid-cols-2 gap-0.5 w-72"
		>
			{#each fieldPalette as item}
				{@const ItemIcon = item.icon}
				<button
					onclick={() => pickField(item.type)}
					class="flex items-center gap-2 px-2.5 py-1.5 bg-transparent border-none rounded-md text-subtle cursor-pointer font-mono text-sm text-left transition-[background,color] duration-100 hover:bg-surface hover:text-text"
				>
					<span class="shrink-0 text-subtle">
						<ItemIcon size={14} strokeWidth={1.75} />
					</span>
					{item.label}
				</button>
			{/each}
		</div>
	{/if}
</main>

