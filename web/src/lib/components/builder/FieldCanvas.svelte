<script lang="ts">
	import { dndzone } from 'svelte-dnd-action';
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { BuilderField, MultipleChoiceConfig, CheckboxesConfig, DropdownConfig, ChoiceOption, RatingConfig, FieldType } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
	import FormPreview from '$lib/components/form/FormPreview.svelte';
	import type { Component } from 'svelte';
	import {
		Type, AlignLeft, CircleDot, CheckSquare, ChevronDown,
		Calendar, Clock, Star, Minus, Heading1, ChevronRight, AlertCircle, Plus
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

	function openSlot(e: MouseEvent, afterIndex: number) {
		e.stopPropagation();
		const btn = e.currentTarget as HTMLElement;
		const rect = btn.getBoundingClientRect();
		const popoverH = 230;
		const popoverW = 280;
		const margin = 8;
		let top: number;
		let left: number;
		if (window.innerWidth < 1440) {
			// Palette is hidden — float popover to the left of the field card
			left = Math.max(margin, rect.left - popoverW - 8);
			top = Math.max(margin, Math.min(
				rect.top + rect.height / 2 - popoverH / 2,
				window.innerHeight - popoverH - margin
			));
		} else {
			// Palette is visible — keep original below/above positioning
			top = rect.bottom + 6 + popoverH > window.innerHeight
				? rect.top - popoverH - 6
				: rect.bottom + 6;
			left = rect.left + popoverW > window.innerWidth
				? window.innerWidth - popoverW - margin
				: rect.left;
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
	style="background: {store.mode === 'preview' ? '#f9fafb' : '#111827'};"
	class="flex-1 overflow-y-auto px-6 pt-6 pb-24 pr-[320px] min-w-0"
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
	<div class="max-w-[680px] mx-auto w-full">
		<!-- Form title and description -->
		<div
			onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
			class="mb-5"
		>
			<textarea
				rows={1}
				value={store.activeTranslation?.formTitle ?? ''}
				placeholder={defaultLocaleTitle || 'Form title…'}
				onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
				oninput={(e) => {
					const el = e.target as HTMLTextAreaElement;
					autoGrow(el);
					store.updateTranslation(null, 'formTitle', el.value);
				}}
				style="color: {store.activeTranslation?.formTitle ? '#f9fafb' : '#4b5563'};"
				class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-[1.75rem] font-semibold font-[inherit] px-1 py-0.5 mb-1.5"
			></textarea>
			<textarea
				rows={1}
				value={store.activeTranslation?.formDescription ?? ''}
				placeholder={defaultLocaleDesc || 'Form description…'}
				onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
				oninput={(e) => {
					const el = e.target as HTMLTextAreaElement;
					autoGrow(el);
					store.updateTranslation(null, 'formDescription', el.value);
				}}
				style="color: {store.activeTranslation?.formDescription ? '#9ca3af' : '#374151'};"
				class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-[1.025rem] font-[inherit] px-1 py-0.5"
			></textarea>
		</div>

		<!-- Backdrop to close popover -->
		{#if insertSlot !== null}
			<div onclick={closeSlot} class="fixed inset-0 z-40"></div>
		{/if}

		{#if fields.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[300px] border-2 border-dashed border-border rounded-lg text-muted-dark">
			<button
				onclick={(e) => openSlot(e, -1)}
				class="flex items-center gap-2 bg-transparent border border-dashed border-border rounded-md text-muted-dark cursor-pointer font-mono text-[0.925rem] px-4 py-2.5 transition-[color,border-color] duration-100 hover:text-muted hover:border-[#4b5563]"
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
			class="flex flex-col gap-3 min-h-[100px]"
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
					class="relative"
					onmouseenter={() => hoveredSlot = fieldIndex}
					onmouseleave={() => hoveredSlot = null}
					role="none"
				>
					<button
						onclick={(e) => openSlot(e, fieldIndex)}
						style="
							background: {insertSlot === fieldIndex ? '#1f2937' : 'transparent'};
							border-color: {hoveredSlot === fieldIndex || insertSlot === fieldIndex ? '#374151' : 'transparent'};
							color: {hoveredSlot === fieldIndex || insertSlot === fieldIndex ? '#6b7280' : 'transparent'};
						"
						class="absolute left-[-26px] top-1/2 -translate-y-1/2 flex items-center justify-center w-[18px] h-[18px] border rounded-full cursor-pointer transition-all duration-100 p-0"
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
						style="border-color: {isSelected ? '#1d4ed8' : '#374151'}; background: {isSelected ? '#1e3a8a22' : 'transparent'};"
						class="flex items-center gap-3 px-3 py-2 border rounded-md cursor-pointer"
					>
						<span class="text-muted-dark cursor-grab text-[0.925rem]">⠿</span>
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
								class="relative z-[1] bg-canvas border-none outline-none text-muted text-[0.875rem] font-mono text-center px-2 py-0 resize-none overflow-hidden w-auto min-w-[80px] max-w-[200px]"
							></textarea>
						</div>
						<button
							onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
							class="bg-transparent border-none text-muted-dark cursor-pointer text-[1.15rem] px-1.5 py-0.5 font-mono"
							aria-label="Delete field"
						>×</button>
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
						style="border-color: {isSelected ? '#1d4ed8' : '#2a3341'}; background: {isSelected ? '#1e3a8a22' : '#1a2233'};"
						class="px-3 py-2 border rounded-md cursor-pointer"
					>
						<div class="flex items-center gap-2 mb-1.5">
							<span class="text-muted-dark cursor-grab text-[0.925rem] shrink-0">⠿</span>
							<span class="px-1.5 py-px bg-surface-2 text-muted-dark rounded-full text-[0.75rem] shrink-0">{headingLevel === 0 ? 'paragraph' : `h${headingLevel}`}</span>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-muted-dark cursor-pointer text-[1.15rem] px-1.5 py-0.5 font-mono shrink-0"
								aria-label="Delete field"
							>×</button>
						</div>
						<textarea
							rows={1}
							value={label}
							placeholder={defaultLabel || 'Heading text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'label', el.value); }}
							style="color: {label ? '#f9fafb' : '#374151'}; font-size: {headingSizes[headingLevel]}; font-weight: {headingWeights[headingLevel]};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text font-[inherit] px-1 py-0.5 mb-0.5 leading-tight"
						></textarea>
						<textarea
							rows={1}
							value={helpText}
							placeholder={defaultHelpText || 'Help text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'helpText', el.value); }}
							style="color: {helpText ? '#9ca3af' : '#374151'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text text-base font-[inherit] px-1 py-0.5 leading-relaxed"
						></textarea>
					</div>

				{:else if isAccent}
					{@const accentVariant = (field.config as import('$lib/types/builder').AccentConfig).variant ?? 'note'}
					{@const accentColors = {
						note:    { border: '#3b82f6', bg: '#1e3a5f22', badge: '#3b82f6', badgeBg: '#1e3a5f' },
						warning: { border: '#f59e0b', bg: '#451a0322', badge: '#f59e0b', badgeBg: '#451a03' },
						danger:  { border: '#ef4444', bg: '#450a0a22', badge: '#ef4444', badgeBg: '#450a0a' },
						success: { border: '#22c55e', bg: '#052e1622', badge: '#22c55e', badgeBg: '#052e16' },
					}[accentVariant]}
					<!-- Accent block -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="
							border-left: 3px solid {accentColors.border};
							border-top: 1px solid {isSelected ? '#1d4ed8' : accentColors.border + '44'};
							border-right: 1px solid {isSelected ? '#1d4ed8' : accentColors.border + '44'};
							border-bottom: 1px solid {isSelected ? '#1d4ed8' : accentColors.border + '44'};
							background: {accentColors.bg};
						"
						class="px-3 py-2.5 rounded-r-md cursor-pointer"
					>
						<div class="flex items-center gap-2 mb-1.5">
							<span class="text-muted-dark cursor-grab text-[0.925rem] shrink-0">⠿</span>
							<span
								style="background: {accentColors.badgeBg}; color: {accentColors.badge}; border-color: {accentColors.border}44;"
								class="px-1.5 py-px border rounded-full text-[0.75rem] shrink-0"
							>{accentVariant}</span>
							<span class="flex-1"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-muted-dark cursor-pointer text-[1.15rem] px-1.5 py-0.5 font-mono shrink-0"
								aria-label="Delete field"
							>×</button>
						</div>
						<textarea
							rows={1}
							value={label}
							placeholder={defaultLabel || 'Title…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'label', el.value); }}
							style="color: {label ? accentColors.badge : '#4b5563'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text text-base font-semibold font-[inherit] px-1 py-0.5 mb-0.5"
						></textarea>
						<textarea
							rows={1}
							value={helpText}
							placeholder={defaultHelpText || 'Body text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'helpText', el.value); }}
							style="color: {helpText ? '#cbd5e1' : '#4b5563'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden cursor-text text-[0.95rem] font-[inherit] px-1 py-0.5 leading-relaxed"
						></textarea>
					</div>

				{:else}
					<!-- Regular field card: vertical layout with inline editable label + help text -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="border-color: {isSelected ? '#1d4ed8' : '#374151'}; background: {isSelected ? '#1e3a8a22' : '#1f2937'};"
						class="px-4 py-3.5 border rounded-md cursor-pointer"
					>
						<!-- Top row: drag handle, type badge, required badge, warning, delete -->
						<div class="flex items-center gap-2 mb-3">
							<span class="text-muted-dark cursor-grab text-[0.925rem] shrink-0">⠿</span>

							<span class="px-2 py-0.5 bg-border text-muted rounded-full text-[0.8rem] shrink-0">
								{FIELD_TYPE_LABELS[field.type] ?? field.type}
							</span>

							<span class="flex-1"></span>

							{#if !label}
								<span
									title="Missing translation for {store.activeLocale}"
									class="text-[#f59e0b] text-[0.975rem] shrink-0"
								>⚠</span>
							{/if}

							{#if field.required}
								<span class="px-1.5 py-0.5 bg-[#1e3a8a] text-[#93c5fd] rounded-full text-[0.75rem] shrink-0">
									required
								</span>
							{/if}

							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								class="bg-transparent border-none text-muted-dark cursor-pointer text-[1.15rem] px-1.5 py-0.5 font-mono shrink-0"
								aria-label="Delete field"
							>×</button>
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
							style="color: {label ? '#e5e7eb' : '#6b7280'};"
							class="block w-full box-border bg-transparent border-none border-b border-b-transparent outline-none resize-none overflow-hidden text-[1.025rem] font-[inherit] px-1 py-0.5 mb-2 cursor-text"
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
							style="color: {helpText ? '#9ca3af' : '#4b5563'};"
							class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-[0.9rem] font-[inherit] px-1 py-0.5 cursor-text"
						></textarea>

						<!-- Placeholder inline editor (text fields only) -->
						{#if hasPlaceholder}
							<div class="mt-3">
								<div class="bg-[#0f1623] border border-[#2a3341] rounded px-1.5 pt-0.5 pb-0.5">
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
										style="color: {placeholder ? '#6b7280' : '#374151'};"
										class="block w-full box-border bg-transparent border-none outline-none resize-none overflow-hidden text-[0.9rem] font-[inherit] py-1 cursor-text italic"
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
								class="mt-3 border-t border-[#2a3341] pt-3 flex flex-col gap-0.5"
							>
								{#each optionLabels as optLabel, i}
									{@const opt = (field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig).options?.[i]}
									<div
										class="flex items-center gap-2 px-1.5 py-1 rounded transition-[background] duration-100 hover:bg-[#1a2436]"
										role="none"
									>
										<!-- Type indicator -->
										{#if isMultiple}
											<span class="inline-block shrink-0 w-[13px] h-[13px] border-[1.5px] border-[#4b5563] rounded-full"></span>
										{:else if isCheckbox}
											<span class="inline-block shrink-0 w-[13px] h-[13px] border-[1.5px] border-[#4b5563] rounded-sm"></span>
										{:else}
											<span class="text-muted-dark text-[0.8rem] font-mono shrink-0 w-[14px] text-right">{i + 1}.</span>
										{/if}
										<input
											type="text"
											value={optLabel}
											placeholder="Option {i + 1}"
											onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
											oninput={(e) => setOptionLabel(field.id, i, (e.target as HTMLInputElement).value)}
											style="color: {optLabel ? '#d1d5db' : '#4b5563'};"
											class="flex-1 min-w-0 bg-transparent border-none outline-none text-[0.95rem] font-[inherit] py-px"
										/>
										<button
											onclick={(e) => { e.stopPropagation(); if (opt) removeOption(field.id, opt.id); }}
											class="bg-transparent border-none text-border cursor-pointer font-mono text-[1.15rem] px-0.5 shrink-0 leading-none hover:text-muted-dark transition-colors duration-100"
											aria-label="Remove option"
										>×</button>
									</div>
								{/each}
								<button
									onclick={(e) => { e.stopPropagation(); focusField(field.id); addOption(field.id); }}
									class="self-start bg-transparent border-none text-muted-dark text-[0.875rem] cursor-pointer font-[inherit] px-1.5 py-1 mt-0.5 rounded transition-colors duration-100 hover:text-muted"
								>+ Add option</button>
							</div>
						{/if}

						<!-- Rating preview -->
						{#if isRating}
							{@const cfg = field.config as RatingConfig}
							{@const scale = cfg.scale ?? 5}
							{@const activeUp = ratingHover?.fieldId === field.id ? ratingHover.value : 0}
							<div class="mt-3 border-t border-[#2a3341] pt-3">
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
													border-color: {lit ? '#3b82f6' : '#2a3341'};
													background: {lit ? '#1e3a5f' : '#0f1623'};
													color: {lit ? '#93c5fd' : '#6b7280'};
												"
												class="inline-flex items-center justify-center w-8 h-8 border rounded-md text-[0.925rem] font-mono cursor-default transition-[background,border-color,color] duration-100"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>{i + 1}</span>
										{:else}
											<span
												style="color: {lit ? '#f59e0b' : '#4b5563'};"
												class="text-[1.6rem] leading-none cursor-default transition-colors duration-100"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>★</span>
										{/if}
									{/each}
									<span class="text-border text-[0.8rem] ml-1">/ {scale}</span>
								</div>
							</div>
						{/if}

						<!-- Date / time preview -->
						{#if isDateTime}
							{@const cfg = field.config as import('$lib/types/builder').DateTimeConfig}
							{@const mode = cfg.mode ?? 'date'}
							<div class="mt-3 border-t border-[#2a3341] pt-3">
								<div class="flex gap-2">
									{#if mode === 'date' || mode === 'datetime'}
										<div class="flex-1 flex items-center gap-2 bg-[#0f1623] border border-[#2a3341] rounded px-2.5 py-1.5">
											<span class="text-border flex shrink-0">
												<Calendar size={14} strokeWidth={1.75} />
											</span>
											<span class="text-border text-[0.9rem] font-mono tracking-[0.04em]">MM / DD / YYYY</span>
										</div>
									{/if}
									{#if mode === 'time' || mode === 'datetime'}
										<div
											style="flex: {mode === 'datetime' ? '0 0 auto' : '1'};"
											class="flex items-center gap-2 bg-[#0f1623] border border-[#2a3341] rounded px-2.5 py-1.5"
										>
											<span class="text-border flex shrink-0">
												<Clock size={14} strokeWidth={1.75} />
											</span>
											<span class="text-border text-[0.9rem] font-mono tracking-[0.04em]">HH : MM</span>
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
	</div>
	{/if}

	<!-- Field type popover (fixed, dismisses on backdrop click) -->
	{#if insertSlot !== null && popoverAnchor}
		<div
			style="top: {popoverAnchor.top}px; left: {popoverAnchor.left}px;"
			class="fixed bg-[#1a2233] border border-[#2a3341] rounded-lg p-1.5 z-50 shadow-[0_8px_32px_rgba(0,0,0,0.5)] grid grid-cols-2 gap-0.5 w-[280px]"
		>
			{#each fieldPalette as item}
				<button
					onclick={() => pickField(item.type)}
					class="flex items-center gap-2 px-2.5 py-1.5 bg-transparent border-none rounded-md text-muted cursor-pointer font-mono text-[0.9rem] text-left transition-[background,color] duration-100 hover:bg-[#1e2b3c] hover:text-text-dim"
				>
					<span class="shrink-0 text-[#4b6280]">
						<svelte:component this={item.icon} size={14} strokeWidth={1.75} />
					</span>
					{item.label}
				</button>
			{/each}
		</div>
	{/if}
</main>
