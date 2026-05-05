<script lang="ts">
	import { dndzone } from 'svelte-dnd-action';
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { BuilderField, MultipleChoiceConfig, CheckboxesConfig, DropdownConfig, ChoiceOption, RatingConfig, FieldType } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
	import FormPreview from '$lib/components/form/FormPreview.svelte';
	import type { Component } from 'svelte';
	import {
		Type, AlignLeft, CircleDot, CheckSquare, ChevronDown,
		Calendar, Clock, Star, Minus, Heading1, ChevronRight, AlertCircle, Plus, TriangleAlert, Trash2, GripVertical
	} from '@lucide/svelte';

	const fieldPalette: Array<{ type: FieldType; label: string; icon: Component }> = [
		{ type: 'short_text', label: 'Short text', icon: Type },
		{ type: 'long_text', label: 'Long text', icon: AlignLeft },
		{ type: 'multiple_choice', label: 'Multiple choice', icon: CircleDot },
		{ type: 'checkboxes', label: 'Checkboxes', icon: CheckSquare },
		{ type: 'dropdown', label: 'Dropdown', icon: ChevronDown },
		{ type: 'date_time', label: 'Date / time', icon: Calendar },
		{ type: 'rating', label: 'Rating', icon: Star },
		{ type: 'section_break', label: 'Section break', icon: Minus },
		{ type: 'heading', label: 'Heading', icon: Heading1 },
		{ type: 'accordion', label: 'Accordion', icon: ChevronRight },
		{ type: 'accent', label: 'Accent block', icon: AlertCircle }
	];

	let insertSlot = $state<number | null>(null);
	let hoveredSlot = $state<number | null>(null);
	let popoverAnchor = $state<{ top: number; left: number } | null>(null);
	let ratingHover = $state<{ fieldId: string; value: number } | null>(null);

	function openSlot(e: MouseEvent, afterIndex: number, anchor: 'left' | 'above' = 'left') {
		e.stopPropagation();
		const btn = e.currentTarget as HTMLElement;
		const rect = btn.getBoundingClientRect();
		const popoverH = 230;
		const popoverW = 288;
		const margin = 8;
		let left: number, top: number;
		if (anchor === 'above') {
			left = Math.max(margin, Math.min(rect.left + rect.width / 2 - popoverW / 2, window.innerWidth - popoverW - margin));
			top = rect.top - popoverH - 8 > margin ? rect.top - popoverH - 8 : rect.bottom + 8;
		} else {
			left = Math.max(margin, rect.left - popoverW - 8);
			top = Math.max(margin, Math.min(
				rect.top + rect.height / 2 - popoverH / 2,
				window.innerHeight - popoverH - margin
			));
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
		short_text: 'Short text',
		long_text: 'Long text',
		multiple_choice: 'Multiple choice',
		checkboxes: 'Checkboxes',
		dropdown: 'Dropdown',
		date_time: 'Date / time',
		rating: 'Rating',
		section_break: 'Section break',
		heading: 'Heading',
		accordion: 'Accordion',
		accent: 'Accent block'
	};

	// Local state for dnd — holds shadow items during drag without touching the store.
	// Synced from the store via $effect; committed back only on finalize.
	let fields = $state<BuilderField[]>([]);

	$effect(() => {
		fields = getOrderedFields(store.schema, store.activeLocale);
	});

	function handleDndConsider(e: CustomEvent<{ items: BuilderField[] }>) {
		fields = e.detail.items;
	}

	function handleDndFinalize(e: CustomEvent<{ items: BuilderField[] }>) {
		store.reorderFields(e.detail.items);
	}

	function getLabel(fieldId: string): string {
		return store.schema.translations[store.activeLocale]?.fields[fieldId]?.label ?? '';
	}

	function getHelpText(fieldId: string): string {
		return store.schema.translations[store.activeLocale]?.fields[fieldId]?.helpText ?? '';
	}

	function getDefaultLabel(fieldId: string): string {
		if (store.activeLocale === store.schema.defaultLocale) return '';
		return store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]?.label ?? '';
	}

	function getDefaultHelpText(fieldId: string): string {
		if (store.activeLocale === store.schema.defaultLocale) return '';
		return store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]?.helpText ?? '';
	}

	function getPlaceholder(fieldId: string): string {
		return store.schema.translations[store.activeLocale]?.fields[fieldId]?.placeholder ?? '';
	}

	function getDefaultPlaceholder(fieldId: string): string {
		if (store.activeLocale === store.schema.defaultLocale) return '';
		return store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]?.placeholder ?? '';
	}

	function autoGrow(el: HTMLTextAreaElement) {
		el.style.height = 'auto';
		el.style.height = el.scrollHeight + 'px';
	}

	function growable(el: HTMLTextAreaElement, value: string) {
		autoGrow(el);
		return { update() { autoGrow(el); } };
	}

	function focusField(fieldId: string) {
		store.setSelectedField(fieldId);
	}

	function getOptionLabels(fieldId: string): string[] {
		const field = store.schema.fields.find((f) => f.id === fieldId);
		if (!field) return [];
		const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig;
		const count = cfg.options?.length ?? 0;
		const translated = store.schema.translations[store.activeLocale]?.fields[fieldId]?.options;
		return Array.from({ length: count }, (_, i) => translated?.[i] ?? '');
	}

	function setOptionLabel(fieldId: string, index: number, value: string) {
		const field = store.schema.fields.find((f) => f.id === fieldId);
		if (!field) return;
		const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig;
		const count = cfg.options?.length ?? 0;
		const current = store.schema.translations[store.activeLocale]?.fields[fieldId]?.options ?? Array(count).fill('');
		const updated = [...current];
		while (updated.length <= index) updated.push('');
		updated[index] = value;
		store.updateTranslation(fieldId, 'options', updated as unknown as string);
	}

	function addOption(fieldId: string) {
		const field = store.schema.fields.find((f) => f.id === fieldId);
		if (!field) return;
		const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig;
		const options = cfg.options ?? [];
		const newOpt: ChoiceOption = { id: crypto.randomUUID(), order: options.length };
		store.updateFieldConfig(fieldId, { options: [...options, newOpt] } as Partial<MultipleChoiceConfig>);
	}

	function removeOption(fieldId: string, optId: string) {
		const field = store.schema.fields.find((f) => f.id === fieldId);
		if (!field) return;
		const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig;
		const removedIndex = (cfg.options ?? []).findIndex((o) => o.id === optId);
		const options = (cfg.options ?? [])
			.filter((o) => o.id !== optId)
			.map((o, i) => ({ ...o, order: i }));
		store.updateFieldConfig(fieldId, { options } as Partial<MultipleChoiceConfig>);
		// Trim the translation options array to match
		if (removedIndex !== -1) {
			const current = store.schema.translations[store.activeLocale]?.fields[fieldId]?.options ?? [];
			const updated = current.filter((_, i) => i !== removedIndex);
			store.updateTranslation(fieldId, 'options', updated as unknown as string);
		}
	}
</script>

<main
	style="background: {store.mode === 'preview' ? 'var(--color-form-surface)' : 'var(--color-surface)'};"
	class="flex-1 overflow-y-auto px-4 pt-6 pb-24 sm:px-6 min-w-0"
	onclick={() => { store.setSelectedField(null); closeSlot(); }}
	role="presentation"
>
	{#if store.mode === 'preview'}
		<FormPreview schema={store.schema} locale={store.activeLocale} />
	{:else}
{@const defaultLocaleTitle = store.activeLocale !== store.schema.defaultLocale
	? (store.schema.translations[store.schema.defaultLocale]?.formTitle ?? '')
		: ''}
{@const defaultLocaleDesc = store.activeLocale !== store.schema.defaultLocale
	? (store.schema.translations[store.schema.defaultLocale]?.formDescription ?? '')
		: ''}
	<div class="max-w-4xl mx-auto w-full">
		<!-- Form title and description -->
		<div
			onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
			class="mb-5"
		>
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
				style="color: {store.activeTranslation?.formTitle ? 'var(--color-text-bright)' : 'var(--color-text-subtle)'};"
				class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-3xl font-semibold font-[inherit] px-1 py-0.5 mb-1.5"
			></textarea>
			<textarea
				rows={1}
				use:growable={store.activeTranslation?.formDescription ?? ''}
				value={store.activeTranslation?.formDescription ?? ''}
				placeholder={defaultLocaleDesc || 'Form description…'}
				onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
				oninput={(e) => {
					const el = e.target as HTMLTextAreaElement;
					autoGrow(el);
					store.updateTranslation(null, 'formDescription', el.value);
				}}
				style="color: {store.activeTranslation?.formDescription ? 'var(--color-muted)' : 'var(--color-border)'};"
				class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-base font-[inherit] px-1 py-0.5"
			></textarea>
		</div>

		<!-- Backdrop to close popover -->
		{#if insertSlot !== null}
			<div onclick={closeSlot} class="fixed inset-0 z-40"></div>
		{/if}

		{#if fields.length === 0}
		<div class="flex flex-col items-center justify-center min-h-72 border-2 border-dashed border-border rounded-lg text-muted-dark">
			<button
				onclick={(e) => openSlot(e, -1)}
				class="flex items-center gap-2 bg-transparent border border-dashed border-border rounded-md text-muted-dark cursor-pointer font-mono text-sm px-4 py-2.5 transition-[color,border-color] duration-100 hover:text-muted hover:border-text-subtle"
			>
				<Plus size={14} strokeWidth={2} />
				Add first field
			</button>
		</div>
	{:else}
		<div
			use:dndzone={{ items: fields, flipDurationMs: 150 }}
			onconsider={handleDndConsider}
			onfinalize={handleDndFinalize}
			class="flex flex-col gap-3 min-h-24"
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
					class="relative"
					onmouseenter={() => hoveredSlot = fieldIndex}
					onmouseleave={() => hoveredSlot = null}
					role="none"
				>
					<button
						onclick={(e) => openSlot(e, fieldIndex)}
						style="
							background: {insertSlot === fieldIndex ? 'var(--color-surface)' : 'transparent'};
							border-color: {hoveredSlot === fieldIndex || insertSlot === fieldIndex ? 'var(--color-border)' : 'transparent'};
							color: {hoveredSlot === fieldIndex || insertSlot === fieldIndex ? 'var(--color-muted-dark)' : 'transparent'};
						"
						class="absolute left-[-26px] top-1/2 -translate-y-1/2 flex items-center justify-center w-4 h-4 border rounded-full cursor-pointer transition-all duration-100 p-0"
						aria-label="Add field here"
					>
						<Plus size={10} strokeWidth={2.5} />
					</button>
				{#if isSectionBreak}
					<!-- Section break: horizontal rule with inline-editable label -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border)'}; background: {isSelected ? 'var(--color-surface-selected)' : 'transparent'};"
						class="flex items-center gap-3 px-3 py-2 border rounded-md cursor-pointer"
					>
						<span class="text-muted-dark cursor-grab flex"><GripVertical size={15} strokeWidth={1.75} /></span>
						<div class="flex-1 h-px bg-border relative flex items-center justify-center">
							<textarea
								rows={1}
								value={label}
								placeholder={defaultLabel || 'Section label…'}
								onclick={(e) => { e.stopPropagation(); focusField(field.id); }}
								onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
								oninput={(e) => {
									const el = e.target as HTMLTextAreaElement;
									autoGrow(el);
									store.updateTranslation(field.id, 'label', el.value);
								}}
								class="relative z-[1] bg-canvas border-none outline-none text-muted text-sm font-mono text-center px-2 py-0 resize-none overflow-hidden w-auto min-w-20 max-w-48"
							></textarea>
						</div>
						<button
							onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
							class="bg-transparent border-none text-muted-dark cursor-pointer flex items-center px-1.5 py-0.5 hover:text-muted transition-colors duration-100"
							aria-label="Delete field" title="Delete field"
						><Trash2 size={15} strokeWidth={1.75} /></button>
					</div>
				{:else if isHeading}
					{@const headingLevel = (field.config as import('$lib/types/builder').HeadingConfig).level ?? 2}
					{@const headingSizes = ['0.9375rem', '1.6rem', '1.2rem', '1rem']}
					{@const headingWeights = ['400', '700', '700', '600']}
					<!-- Heading block -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border-field)'}; background: {isSelected ? 'var(--color-surface-selected)' : 'var(--color-surface)'};"
						class="px-3 py-2 border rounded-md cursor-pointer"
					>
						<div class="flex items-center gap-2 mb-1.5">
							<span class="text-muted-dark cursor-grab shrink-0 flex"><GripVertical size={15} strokeWidth={1.75} /></span>
							<span class="px-1.5 py-px bg-surface text-muted-dark rounded-full text-xs shrink-0">{headingLevel === 0 ? 'paragraph' : `h${headingLevel}`}</span>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-muted-dark cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-muted transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>
						<textarea
							rows={1}
							value={label}
							placeholder={defaultLabel || 'Heading text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'label', el.value); }}
							style="color: {label ? 'var(--color-text-bright)' : 'var(--color-border)'}; font-size: {headingSizes[headingLevel]}; font-weight: {headingWeights[headingLevel]};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text font-[inherit] px-1 py-0.5 mb-0.5 leading-tight"
						></textarea>
						<textarea
							rows={1}
							value={helpText}
							placeholder={defaultHelpText || 'Help text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'helpText', el.value); }}
							style="color: {helpText ? 'var(--color-muted)' : 'var(--color-border)'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text text-base font-[inherit] px-1 py-0.5 leading-relaxed"
						></textarea>
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
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="
							border-left: 3px solid {accentColors.border};
							border-top: 1px solid {isSelected ? 'var(--color-primary)' : 'var(--color-info-border-subtle)'};
							border-right: 1px solid {isSelected ? 'var(--color-primary)' : 'var(--color-info-border-subtle)'};
							border-bottom: 1px solid {isSelected ? 'var(--color-primary)' : 'var(--color-info-border-subtle)'};
							background: {accentColors.bg};
						"
						class="px-3 py-2.5 rounded-r-md cursor-pointer"
					>
						<div class="flex items-center gap-2 mb-1.5">
							<span class="text-muted-dark cursor-grab shrink-0 flex"><GripVertical size={15} strokeWidth={1.75} /></span>
							<span
								style="background: {accentColors.badgeBg}; color: {accentColors.badge}; border-color: {accentColors.border}44;"
								class="px-1.5 py-px border rounded-full text-xs shrink-0"
							>{accentVariant}</span>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-muted-dark cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-muted transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>
						<textarea
							rows={1}
							value={label}
							placeholder={defaultLabel || 'Title…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'label', el.value); }}
							style="color: {label ? accentColors.badge : 'var(--color-text-subtle)'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text text-base font-semibold font-[inherit] px-1 py-0.5 mb-0.5"
						></textarea>
						<textarea
							rows={1}
							value={helpText}
							placeholder={defaultHelpText || 'Body text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'helpText', el.value); }}
							style="color: {helpText ? 'var(--color-text-note)' : 'var(--color-text-subtle)'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text text-sm font-[inherit] px-1 py-0.5 leading-relaxed"
						></textarea>
					</div>

				{:else}
					<!-- Regular field card: vertical layout with inline editable label + help text -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? 'var(--color-primary)' : 'var(--color-border)'}; background: {isSelected ? 'var(--color-surface-selected)' : 'var(--color-surface)'};"
						class="px-4 py-3.5 border rounded-md cursor-pointer"
					>
						<!-- Top row: drag handle, type badge, required badge, warning, delete -->
						<div class="flex items-center gap-2 mb-3">
							<span class="text-muted-dark cursor-grab shrink-0 flex"><GripVertical size={15} strokeWidth={1.75} /></span>

							<span class="px-2 py-0.5 bg-border text-muted rounded-full text-xs shrink-0">
								{FIELD_TYPE_LABELS[field.type] ?? field.type}
							</span>

							<span class="flex-1"></span>

							{#if !label}
								<span title="Missing translation for {store.activeLocale}" class="text-warning-border shrink-0 flex">
									<TriangleAlert size={15} strokeWidth={1.75} />
								</span>
							{/if}

							<button
								onclick={(e) => { e.stopPropagation(); store.updateField(field.id, { required: !field.required }); }}
								title={field.required ? 'Mark as optional' : 'Mark as required'}
								style={field.required ? 'background: var(--color-info-bg-dark); color: var(--color-text-blue);' : 'background: transparent; color: var(--color-muted-dark);'}
								class="px-1.5 py-0.5 border-none rounded-full text-xs shrink-0 cursor-pointer font-mono transition-colors duration-100 hover:opacity-80"
							>{field.required ? 'required' : 'optional'}</button>

							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-muted-dark cursor-pointer flex items-center px-1.5 py-0.5 shrink-0 hover:text-muted transition-colors duration-100"
								aria-label="Delete field" title="Delete field"
							><Trash2 size={15} strokeWidth={1.75} /></button>
						</div>

						<!-- Label inline editor -->
						<textarea
							rows={1}
							value={label}
							placeholder={defaultLabel || 'Label…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => {
								const el = e.target as HTMLTextAreaElement;
								autoGrow(el);
								store.updateTranslation(field.id, 'label', el.value);
							}}
							style="color: {label ? 'var(--color-text)' : 'var(--color-muted-dark)'};"
							class="block w-full box-border bg-transparent border-none border-b border-b-transparent outline-none resize-none overflow-hidden text-base font-[inherit] px-1 py-0.5 mb-2 cursor-text"
						></textarea>

						<!-- Help text inline editor -->
						<textarea
							rows={1}
							use:growable={helpText}
							value={helpText}
							placeholder={defaultHelpText || 'Add help text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => {
								const el = e.target as HTMLTextAreaElement;
								autoGrow(el);
								store.updateTranslation(field.id, 'helpText', el.value);
							}}
							style="color: {helpText ? 'var(--color-muted)' : 'var(--color-text-subtle)'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-sm font-[inherit] px-1 py-0.5 cursor-text"
						></textarea>

						<!-- Placeholder inline editor (text fields only) -->
						{#if hasPlaceholder}
							<div class="mt-3">
								<div class="bg-surface-input border border-border-field rounded px-1.5 pt-0.5 pb-0.5">
									<textarea
										rows={1}
										value={placeholder}
										placeholder={defaultPlaceholder || 'e.g. Enter your answer…'}
										onclick={(e) => e.stopPropagation()}
										onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
										oninput={(e) => {
											const el = e.target as HTMLTextAreaElement;
											autoGrow(el);
											store.updateTranslation(field.id, 'placeholder', el.value);
										}}
										style="color: {placeholder ? 'var(--color-muted-dark)' : 'var(--color-border)'};"
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
								class="mt-3 border-t border-border-field pt-3 flex flex-col gap-0.5"
							>
								{#each optionLabels as optLabel, i}
									{@const opt = (field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig).options?.[i]}
									<div
										class="flex items-center gap-2 px-1.5 py-1 rounded transition-[background] duration-100 hover:bg-surface-item-hover"
										role="none"
									>
										<!-- Type indicator -->
										{#if isMultiple}
											<span class="inline-block shrink-0 w-3 h-3 border-[1.5px] border-text-subtle rounded-full"></span>
										{:else if isCheckbox}
											<span class="inline-block shrink-0 w-3 h-3 border-[1.5px] border-text-subtle rounded-sm"></span>
										{:else}
											<span class="text-muted-dark text-xs font-mono shrink-0 w-3.5 text-right">{i + 1}.</span>
										{/if}
										<input
											type="text"
											value={optLabel}
											placeholder="Option {i + 1}"
											onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
											oninput={(e) => setOptionLabel(field.id, i, (e.target as HTMLInputElement).value)}
											style="color: {optLabel ? 'var(--color-text-dim)' : 'var(--color-text-subtle)'};"
											class="flex-1 min-w-0 bg-transparent border-none outline-none text-sm font-[inherit] py-px"
										/>
										<button
											onclick={(e) => { e.stopPropagation(); if (opt) removeOption(field.id, opt.id); }}
											class="bg-transparent border-none text-border cursor-pointer flex items-center px-0.5 shrink-0 hover:text-muted-dark transition-colors duration-100"
											aria-label="Remove option" title="Remove option"
										><Trash2 size={15} strokeWidth={1.75} /></button>
									</div>
								{/each}
								{#if isMultiple && (field.config as MultipleChoiceConfig).allowOther}
									<div class="flex items-center gap-2 px-1.5 py-1 rounded opacity-50 select-none">
										<span class="inline-block shrink-0 w-3 h-3 border-[1.5px] border-text-subtle rounded-full"></span>
										<span class="text-sm font-[inherit] text-text-subtle py-px">Other…</span>
									</div>
								{/if}
								<button
									onclick={(e) => { e.stopPropagation(); focusField(field.id); addOption(field.id); }}
									class="self-start bg-transparent border-none text-muted-dark text-sm cursor-pointer font-[inherit] px-1.5 py-1 mt-0.5 rounded transition-colors duration-100 hover:text-muted"
								>+ Add option</button>
							</div>
						{/if}

						<!-- Rating preview -->
						{#if isRating}
							{@const cfg = field.config as RatingConfig}
							{@const scale = cfg.scale ?? 5}
							{@const activeUp = ratingHover?.fieldId === field.id ? ratingHover.value : 0}
							<div class="mt-3 border-t border-border-field pt-3">
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
													border-color: {lit ? 'var(--color-info-border)' : 'var(--color-border-field)'};
													background: var(--color-surface-subtle);
													color: {lit ? 'var(--color-text-blue)' : 'var(--color-muted-dark)'};
												"
												class="inline-flex items-center justify-center w-8 h-8 border rounded-md text-sm font-mono cursor-default transition-[background,border-color,color] duration-100"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>{i + 1}</span>
										{:else}
											<span
												style="color: {lit ? 'var(--color-warning-border)' : 'var(--color-text-subtle)'};"
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
							<div class="mt-3 border-t border-border-field pt-3">
								<div class="flex gap-2">
									{#if mode === 'date' || mode === 'datetime'}
										<div class="flex-1 flex items-center gap-2 bg-surface-input border border-border-field rounded px-2.5 py-1.5">
											<span class="text-border flex shrink-0">
												<Calendar size={14} strokeWidth={1.75} />
											</span>
											<span class="text-border text-sm font-mono tracking-[0.04em]">MM / DD / YYYY</span>
										</div>
									{/if}
									{#if mode === 'time' || mode === 'datetime'}
										<div
											style="flex: {mode === 'datetime' ? '0 0 auto' : '1'};"
											class="flex items-center gap-2 bg-surface-input border border-border-field rounded px-2.5 py-1.5"
										>
											<span class="text-border flex shrink-0">
												<Clock size={14} strokeWidth={1.75} />
											</span>
											<span class="text-border text-sm font-mono tracking-[0.04em]">HH : MM</span>
										</div>
									{/if}
								</div>
							</div>
						{/if}
					</div>
				{/if}

			</div>
			{/each}
		</div>
	{/if}

	<!-- Add field button -->
	<button
		onclick={(e) => openSlot(e, fields.length - 1, 'above')}
		class="mt-4 flex items-center justify-center gap-2 w-full px-4 py-3 bg-transparent border border-dashed border-border rounded-md text-muted-dark cursor-pointer font-mono text-sm transition-[color,border-color] duration-100 hover:text-muted hover:border-text-subtle"
	>
		<Plus size={14} strokeWidth={2} />
		Add field
	</button>
	</div>
	{/if}

	<!-- Field type popover (fixed, dismisses on backdrop click) -->
	{#if insertSlot !== null && popoverAnchor}
		<div
			style="top: {popoverAnchor.top}px; left: {popoverAnchor.left}px;"
			class="fixed bg-surface border border-border-field rounded-lg p-1.5 z-50 shadow-[0_8px_32px_var(--color-overlay)] grid grid-cols-2 gap-0.5 w-72"
		>
			{#each fieldPalette as item}
				<button
					onclick={() => pickField(item.type)}
					class="flex items-center gap-2 px-2.5 py-1.5 bg-transparent border-none rounded-md text-muted cursor-pointer font-mono text-sm text-left transition-[background,color] duration-100 hover:bg-surface-popover-hover hover:text-text-dim"
				>
					<span class="shrink-0 text-muted-dim">
						<svelte:component this={item.icon} size={14} strokeWidth={1.75} />
					</span>
					{item.label}
				</button>
			{/each}
		</div>
	{/if}
</main>
