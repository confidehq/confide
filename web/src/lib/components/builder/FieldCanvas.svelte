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
	style="
		flex: 1;
		overflow-y: auto;
		padding: 24px 320px 100px 24px;
		background: {store.mode === 'preview' ? '#f9fafb' : '#111827'};
		min-width: 0;
	"
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
	<div style="max-width: 680px; margin: 0 auto; width: 100%;">
		<!-- Form title and description -->
		<div
			onclick={(e) => { e.stopPropagation(); store.setSelectedField(null); }}
			style="margin-bottom: 20px;"
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
				style="
					display: block; width: 100%; box-sizing: border-box;
					background: transparent; border: none; outline: none;
					resize: none; overflow: hidden;
					color: {store.activeTranslation?.formTitle ? '#f9fafb' : '#4b5563'};
					font-size: 1.75rem; font-weight: 600; font-family: inherit;
					padding: 2px 4px; margin-bottom: 6px;
				"
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
				style="
					display: block; width: 100%; box-sizing: border-box;
					background: transparent; border: none; outline: none;
					resize: none; overflow: hidden;
					color: {store.activeTranslation?.formDescription ? '#9ca3af' : '#374151'};
					font-size: 1.025rem; font-family: inherit;
					padding: 2px 4px;
				"
			></textarea>
		</div>

		<!-- Backdrop to close popover -->
		{#if insertSlot !== null}
			<div
				onclick={closeSlot}
				style="position: fixed; inset: 0; z-index: 40;"
			></div>
		{/if}

		{#if fields.length === 0}
		<div style="
			display: flex; flex-direction: column; align-items: center; justify-content: center;
			min-height: 300px;
			border: 2px dashed #374151;
			border-radius: 8px;
			color: #6b7280;
		">
			<button
				onclick={(e) => openSlot(e, -1)}
				style="
					display: flex; align-items: center; gap: 8px;
					background: transparent; border: 1px dashed #374151;
					border-radius: 6px; color: #6b7280; cursor: pointer;
					font-family: monospace; font-size: 0.925rem;
					padding: 10px 18px;
					transition: color 0.1s, border-color 0.1s;
				"
				onmouseenter={(e) => { (e.currentTarget as HTMLElement).style.color = '#9ca3af'; (e.currentTarget as HTMLElement).style.borderColor = '#4b5563'; }}
				onmouseleave={(e) => { (e.currentTarget as HTMLElement).style.color = '#6b7280'; (e.currentTarget as HTMLElement).style.borderColor = '#374151'; }}
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
			style="display: flex; flex-direction: column; gap: 12px; min-height: 100px;"
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
					style="position: relative;"
					onmouseenter={() => hoveredSlot = fieldIndex}
					onmouseleave={() => hoveredSlot = null}
					role="none"
				>
					<button
						onclick={(e) => openSlot(e, fieldIndex)}
						style="
							position: absolute; left: -26px; top: 50%; transform: translateY(-50%);
							display: flex; align-items: center; justify-content: center;
							width: 18px; height: 18px;
							background: {insertSlot === fieldIndex ? '#1f2937' : 'transparent'};
							border: 1px solid {hoveredSlot === fieldIndex || insertSlot === fieldIndex ? '#374151' : 'transparent'};
							border-radius: 50%; cursor: pointer;
							color: {hoveredSlot === fieldIndex || insertSlot === fieldIndex ? '#6b7280' : 'transparent'};
							transition: all 0.1s; padding: 0;
						"
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
						style="
							display: flex; align-items: center; gap: 12px;
							padding: 8px 12px;
							border: 1px solid {isSelected ? '#1d4ed8' : '#374151'};
							border-radius: 6px;
							background: {isSelected ? '#1e3a8a22' : 'transparent'};
							cursor: pointer;
						"
					>
						<span style="color: #6b7280; cursor: grab; font-size: 0.925rem;">⠿</span>
						<div style="flex: 1; height: 1px; background: #374151; position: relative; display: flex; align-items: center; justify-content: center;">
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
								style="
									position: relative; z-index: 1;
									background: #111827; border: none; outline: none;
									color: #9ca3af; font-size: 0.875rem;
									font-family: monospace; text-align: center;
									padding: 0 8px; resize: none; overflow: hidden;
									width: auto; min-width: 80px; max-width: 200px;
								"
							></textarea>
						</div>
						<button
							onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
							style="
								background: transparent; border: none; color: #6b7280;
								cursor: pointer; font-size: 1.15rem; padding: 2px 6px;
								font-family: monospace;
							"
							aria-label="Delete field"
						>
							×
						</button>
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
						style="
							padding: 8px 12px 8px 12px;
							border: 1px solid {isSelected ? '#1d4ed8' : '#2a3341'};
							border-radius: 6px;
							background: {isSelected ? '#1e3a8a22' : '#1a2233'};
							cursor: pointer;
						"
					>
						<div style="display: flex; align-items: center; gap: 8px; margin-bottom: 6px;">
							<span style="color: #4b5563; cursor: grab; font-size: 0.925rem; flex-shrink: 0;">⠿</span>
							<span style="
								padding: 1px 6px;
								background: #1f2937; color: #6b7280;
								border-radius: 9999px; font-size: 0.75rem; flex-shrink: 0;
							">{headingLevel === 0 ? 'paragraph' : `h${headingLevel}`}</span>
							<span style="flex: 1;"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								style="background: transparent; border: none; color: #4b5563; cursor: pointer; font-size: 1.15rem; padding: 2px 6px; font-family: monospace; flex-shrink: 0;"
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
							style="
								display: block; width: 100%; box-sizing: border-box;
								background: transparent; border: none; outline: none;
								resize: none; overflow: hidden; cursor: text;
								color: {label ? '#f9fafb' : '#374151'};
								font-size: {headingSizes[headingLevel]};
								font-weight: {headingWeights[headingLevel]};
								font-family: inherit; padding: 2px 4px; margin-bottom: 2px;
								line-height: 1.3;
							"
						></textarea>
						<textarea
							rows={1}
							value={helpText}
							placeholder={defaultHelpText || 'Help text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'helpText', el.value); }}
							style="
								display: block; width: 100%; box-sizing: border-box;
								background: transparent; border: none; outline: none;
								resize: none; overflow: hidden; cursor: text;
								color: {helpText ? '#9ca3af' : '#374151'};
								font-size: 1rem; font-family: inherit; padding: 2px 4px;
								line-height: 1.6;
							"
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
							padding: 10px 12px;
							border-left: 3px solid {accentColors.border};
							border-top: 1px solid {isSelected ? '#1d4ed8' : accentColors.border + '44'};
							border-right: 1px solid {isSelected ? '#1d4ed8' : accentColors.border + '44'};
							border-bottom: 1px solid {isSelected ? '#1d4ed8' : accentColors.border + '44'};
							border-radius: 0 6px 6px 0;
							background: {accentColors.bg};
							cursor: pointer;
						"
					>
						<div style="display: flex; align-items: center; gap: 8px; margin-bottom: 6px;">
							<span style="color: #4b5563; cursor: grab; font-size: 0.925rem; flex-shrink: 0;">⠿</span>
							<span style="
								padding: 1px 7px;
								background: {accentColors.badgeBg}; color: {accentColors.badge};
								border: 1px solid {accentColors.border}44;
								border-radius: 9999px; font-size: 0.75rem; flex-shrink: 0;
							">{accentVariant}</span>
							<span style="flex: 1;"></span>
							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								style="background: transparent; border: none; color: #4b5563; cursor: pointer; font-size: 1.15rem; padding: 2px 6px; font-family: monospace; flex-shrink: 0;"
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
							style="
								display: block; width: 100%; box-sizing: border-box;
								background: transparent; border: none; outline: none;
								resize: none; overflow: hidden; cursor: text;
								color: {label ? accentColors.badge : '#4b5563'};
								font-size: 1rem; font-weight: 600; font-family: inherit;
								padding: 2px 4px; margin-bottom: 2px;
							"
						></textarea>
						<textarea
							rows={1}
							value={helpText}
							placeholder={defaultHelpText || 'Body text…'}
							onclick={(e) => e.stopPropagation()}
							onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
							oninput={(e) => { const el = e.target as HTMLTextAreaElement; autoGrow(el); store.updateTranslation(field.id, 'helpText', el.value); }}
							style="
								display: block; width: 100%; box-sizing: border-box;
								background: transparent; border: none; outline: none;
								resize: none; overflow: hidden; cursor: text;
								color: {helpText ? '#cbd5e1' : '#4b5563'};
								font-size: 0.95rem; font-family: inherit; padding: 2px 4px;
								line-height: 1.6;
							"
						></textarea>
					</div>

				{:else}
					<!-- Regular field card: vertical layout with inline editable label + help text -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="
							padding: 14px 16px;
							border: 1px solid {isSelected ? '#1d4ed8' : '#374151'};
							border-radius: 6px;
							background: {isSelected ? '#1e3a8a22' : '#1f2937'};
							cursor: pointer;
						"
					>
						<!-- Top row: drag handle, type badge, required badge, warning, delete -->
						<div style="display: flex; align-items: center; gap: 8px; margin-bottom: 12px;">
							<span style="color: #6b7280; cursor: grab; font-size: 0.925rem; flex-shrink: 0;">⠿</span>

							<span style="
								padding: 2px 8px;
								background: #374151;
								color: #9ca3af;
								border-radius: 9999px;
								font-size: 0.8rem;
								flex-shrink: 0;
							">
								{FIELD_TYPE_LABELS[field.type] ?? field.type}
							</span>

							<span style="flex: 1;"></span>

							{#if !label}
								<span
									title="Missing translation for {store.activeLocale}"
									style="color: #f59e0b; font-size: 0.975rem; flex-shrink: 0;"
								>
									⚠
								</span>
							{/if}

							{#if field.required}
								<span style="
									padding: 2px 6px;
									background: #1e3a8a;
									color: #93c5fd;
									border-radius: 9999px;
									font-size: 0.75rem;
									flex-shrink: 0;
								">
									required
								</span>
							{/if}

							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								style="
									background: transparent; border: none; color: #6b7280;
									cursor: pointer; font-size: 1.15rem; padding: 2px 6px;
									font-family: monospace; flex-shrink: 0;
								"
								aria-label="Delete field"
							>
								×
							</button>
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
							style="
								display: block; width: 100%; box-sizing: border-box;
								background: transparent; border: none;
								border-bottom: 1px solid transparent;
								outline: none; resize: none; overflow: hidden;
								color: {label ? '#e5e7eb' : '#6b7280'};
								font-size: 1.025rem; font-family: inherit;
								padding: 2px 4px; margin-bottom: 8px;
								cursor: text;
							"
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
							style="
								display: block; width: 100%; box-sizing: border-box;
								background: transparent; border: none;
								outline: none; resize: none; overflow: hidden;
								color: {helpText ? '#9ca3af' : '#4b5563'};
								font-size: 0.9rem; font-family: inherit;
								padding: 2px 4px;
								cursor: text;
							"
						></textarea>

						<!-- Placeholder inline editor (text fields only) -->
						{#if hasPlaceholder}
							<div style="margin-top: 12px;">
								<div style="
									background: #0f1623;
									border: 1px solid #2a3341;
									border-radius: 4px;
									padding: 2px 6px 2px;
								">
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
										style="
											display: block; width: 100%; box-sizing: border-box;
											background: transparent; border: none;
											outline: none; resize: none; overflow: hidden;
											color: {placeholder ? '#6b7280' : '#374151'};
											font-size: 0.9rem; font-family: inherit;
											padding: 4px 0;
											cursor: text; font-style: italic;
										"
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
								style="margin-top: 12px; border-top: 1px solid #2a3341; padding-top: 12px; display: flex; flex-direction: column; gap: 2px;"
							>
								{#each optionLabels as optLabel, i}
									{@const opt = (field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig).options?.[i]}
									<div
										style="
											display: flex; align-items: center; gap: 8px;
											padding: 4px 6px; border-radius: 4px;
											transition: background 0.1s;
										"
										onmouseenter={(e) => { (e.currentTarget as HTMLElement).style.background = '#1a2436'; }}
										onmouseleave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
										role="none"
									>
										<!-- Type indicator -->
										{#if isMultiple}
											<span style="
												display: inline-block; flex-shrink: 0;
												width: 13px; height: 13px;
												border: 1.5px solid #4b5563; border-radius: 50%;
											"></span>
										{:else if isCheckbox}
											<span style="
												display: inline-block; flex-shrink: 0;
												width: 13px; height: 13px;
												border: 1.5px solid #4b5563; border-radius: 3px;
											"></span>
										{:else}
											<span style="color: #4b5563; font-size: 0.8rem; font-family: monospace; flex-shrink: 0; width: 14px; text-align: right;">{i + 1}.</span>
										{/if}
										<input
											type="text"
											value={optLabel}
											placeholder="Option {i + 1}"
											onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
											oninput={(e) => setOptionLabel(field.id, i, (e.target as HTMLInputElement).value)}
											style="
												flex: 1; min-width: 0;
												background: transparent; border: none; outline: none;
												color: {optLabel ? '#d1d5db' : '#4b5563'};
												font-size: 0.95rem; font-family: inherit;
												padding: 1px 0;
											"
										/>
										<button
											onclick={(e) => { e.stopPropagation(); if (opt) removeOption(field.id, opt.id); }}
											style="
												background: transparent; border: none; color: #374151;
												cursor: pointer; font-family: monospace; font-size: 1.15rem;
												padding: 0 2px; flex-shrink: 0; line-height: 1;
												transition: color 0.1s;
											"
											onmouseenter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#6b7280'; }}
											onmouseleave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#374151'; }}
											aria-label="Remove option"
										>×</button>
									</div>
								{/each}
								<button
									onclick={(e) => { e.stopPropagation(); focusField(field.id); addOption(field.id); }}
									style="
										align-self: flex-start;
										background: transparent; border: none;
										color: #4b5563; font-size: 0.875rem;
										cursor: pointer; font-family: inherit;
										padding: 4px 6px; margin-top: 2px;
										border-radius: 4px;
										transition: color 0.1s;
									"
									onmouseenter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#9ca3af'; }}
									onmouseleave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#4b5563'; }}
								>+ Add option</button>
							</div>
						{/if}

						<!-- Rating preview -->
						{#if isRating}
							{@const cfg = field.config as RatingConfig}
							{@const scale = cfg.scale ?? 5}
							{@const activeUp = ratingHover?.fieldId === field.id ? ratingHover.value : 0}
							<div style="margin-top: 12px; border-top: 1px solid #2a3341; padding-top: 12px;">
								<div
									style="display: flex; gap: 6px; flex-wrap: wrap; align-items: center;"
									onmouseleave={() => ratingHover = null}
									role="none"
								>
									{#each { length: scale } as _, i}
										{@const lit = i < activeUp}
										{#if cfg.shape === 'number'}
											<span
												style="
													display: inline-flex; align-items: center; justify-content: center;
													width: 32px; height: 32px;
													border: 1px solid {lit ? '#3b82f6' : '#2a3341'}; border-radius: 6px;
													background: {lit ? '#1e3a5f' : '#0f1623'};
													color: {lit ? '#93c5fd' : '#6b7280'}; font-size: 0.925rem; font-family: monospace;
													cursor: default; transition: background 0.1s, border-color 0.1s, color 0.1s;
												"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>{i + 1}</span>
										{:else}
											<span
												style="
													color: {lit ? '#f59e0b' : '#4b5563'}; font-size: 1.6rem; line-height: 1;
													cursor: default; transition: color 0.1s;
												"
												onmouseenter={() => ratingHover = { fieldId: field.id, value: i + 1 }}
												role="none"
											>★</span>
										{/if}
									{/each}
									<span style="color: #374151; font-size: 0.8rem; margin-left: 4px;">/ {scale}</span>
								</div>
							</div>
						{/if}

						<!-- Date / time preview -->
						{#if isDateTime}
							{@const cfg = field.config as import('$lib/types/builder').DateTimeConfig}
							{@const mode = cfg.mode ?? 'date'}
							<div style="margin-top: 12px; border-top: 1px solid #2a3341; padding-top: 12px;">
								<div style="display: flex; gap: 8px;">
									{#if mode === 'date' || mode === 'datetime'}
										<div style="
											flex: 1; display: flex; align-items: center; gap: 8px;
											background: #0f1623; border: 1px solid #2a3341;
											border-radius: 4px; padding: 6px 10px;
										">
											<span style="color: #374151; display: flex; flex-shrink: 0;">
												<Calendar size={14} strokeWidth={1.75} />
											</span>
											<span style="color: #374151; font-size: 0.9rem; font-family: monospace; letter-spacing: 0.04em;">
												MM / DD / YYYY
											</span>
										</div>
									{/if}
									{#if mode === 'time' || mode === 'datetime'}
										<div style="
											flex: {mode === 'datetime' ? '0 0 auto' : '1'}; display: flex; align-items: center; gap: 8px;
											background: #0f1623; border: 1px solid #2a3341;
											border-radius: 4px; padding: 6px 10px;
										">
											<span style="color: #374151; display: flex; flex-shrink: 0;">
												<Clock size={14} strokeWidth={1.75} />
											</span>
											<span style="color: #374151; font-size: 0.9rem; font-family: monospace; letter-spacing: 0.04em;">
												HH : MM
											</span>
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
		<div style="
			position: fixed;
			top: {popoverAnchor.top}px;
			left: {popoverAnchor.left}px;
			background: #1a2233;
			border: 1px solid #2a3341;
			border-radius: 8px;
			padding: 6px;
			z-index: 50;
			box-shadow: 0 8px 32px rgba(0,0,0,0.5);
			display: grid;
			grid-template-columns: repeat(2, 1fr);
			gap: 3px;
			width: 280px;
		">
			{#each fieldPalette as item}
				<button
					onclick={() => pickField(item.type)}
					style="
						display: flex; align-items: center; gap: 8px;
						padding: 7px 10px;
						background: transparent; border: none; border-radius: 5px;
						color: #9ca3af; cursor: pointer;
						font-family: monospace; font-size: 0.9rem; text-align: left;
						transition: background 0.1s, color 0.1s;
					"
					onmouseenter={(e) => { (e.currentTarget as HTMLElement).style.background = '#1e2b3c'; (e.currentTarget as HTMLElement).style.color = '#d1d5db'; }}
					onmouseleave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; (e.currentTarget as HTMLElement).style.color = '#9ca3af'; }}
				>
					<span style="flex-shrink: 0; color: #4b6280;">
						<svelte:component this={item.icon} size={14} strokeWidth={1.75} />
					</span>
					{item.label}
				</button>
			{/each}
		</div>
	{/if}
</main>
