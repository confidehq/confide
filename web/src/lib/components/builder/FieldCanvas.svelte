<script lang="ts">
	import { dndzone } from 'svelte-dnd-action';
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { BuilderField, MultipleChoiceConfig, CheckboxesConfig, DropdownConfig, ChoiceOption, RatingConfig } from '$lib/types/builder';
	import { getOrderedFields } from '$lib/types/builder';
	import FormPreview from '$lib/components/form/FormPreview.svelte';

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
		padding: 24px 320px 24px 24px;
		background: {store.mode === 'preview' ? '#f9fafb' : '#111827'};
		min-width: 0;
	"
	onclick={() => store.setSelectedField(null)}
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
					font-size: 1.5rem; font-weight: 600; font-family: inherit;
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
					font-size: 0.9rem; font-family: inherit;
					padding: 2px 4px;
				"
			></textarea>
		</div>

		{#if fields.length === 0}
		<div style="
			display: flex; flex-direction: column; align-items: center; justify-content: center;
			min-height: 300px;
			border: 2px dashed #374151;
			border-radius: 8px;
			color: #6b7280;
			font-size: 0.9rem;
		">
			<p style="margin: 0 0 8px;">No fields yet</p>
			<p style="margin: 0; font-size: 0.8rem;">Click a field type in the palette to add it</p>
		</div>
	{:else}
		<div
			use:dndzone={{ items: fields, flipDurationMs: 150 }}
			onconsider={handleDndConsider}
			onfinalize={handleDndFinalize}
			style="display: flex; flex-direction: column; gap: 6px; min-height: 100px;"
		>
			{#each fields as field (field.id)}
				{@const isSelected = store.selectedFieldId === field.id}
				{@const isSectionBreak = field.type === 'section_break'}
				{@const hasPlaceholder = field.type === 'short_text' || field.type === 'long_text'}
				{@const hasOptions = field.type === 'multiple_choice' || field.type === 'checkboxes' || field.type === 'dropdown'}
				{@const isRating = field.type === 'rating'}
				{@const label = getLabel(field.id)}
				{@const helpText = getHelpText(field.id)}
				{@const placeholder = getPlaceholder(field.id)}
				{@const defaultLabel = getDefaultLabel(field.id)}
				{@const defaultHelpText = getDefaultHelpText(field.id)}
				{@const defaultPlaceholder = getDefaultPlaceholder(field.id)}

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
						<span style="color: #6b7280; cursor: grab; font-size: 0.8rem;">⠿</span>
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
									color: #9ca3af; font-size: 0.75rem;
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
								cursor: pointer; font-size: 1rem; padding: 2px 6px;
								font-family: monospace;
							"
							aria-label="Delete field"
						>
							×
						</button>
					</div>
				{:else}
					<!-- Regular field card: vertical layout with inline editable label + help text -->
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="
							padding: 10px 12px;
							border: 1px solid {isSelected ? '#1d4ed8' : '#374151'};
							border-radius: 6px;
							background: {isSelected ? '#1e3a8a22' : '#1f2937'};
							cursor: pointer;
						"
					>
						<!-- Top row: drag handle, type badge, required badge, warning, delete -->
						<div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px;">
							<span style="color: #6b7280; cursor: grab; font-size: 0.8rem; flex-shrink: 0;">⠿</span>

							<span style="
								padding: 2px 8px;
								background: #374151;
								color: #9ca3af;
								border-radius: 9999px;
								font-size: 0.7rem;
								flex-shrink: 0;
							">
								{FIELD_TYPE_LABELS[field.type] ?? field.type}
							</span>

							<span style="flex: 1;"></span>

							{#if !label}
								<span
									title="Missing translation for {store.activeLocale}"
									style="color: #f59e0b; font-size: 0.85rem; flex-shrink: 0;"
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
									font-size: 0.65rem;
									flex-shrink: 0;
								">
									required
								</span>
							{/if}

							<button
								onclick={(e) => { e.stopPropagation(); store.removeField(field.id); }}
								style="
									background: transparent; border: none; color: #6b7280;
									cursor: pointer; font-size: 1rem; padding: 2px 6px;
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
								font-size: 0.9rem; font-family: inherit;
								padding: 2px 4px; margin-bottom: 4px;
								cursor: text;
							"
						></textarea>

						<!-- Help text inline editor -->
						<textarea
							rows={1}
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
								font-size: 0.78rem; font-family: inherit;
								padding: 2px 4px;
								cursor: text;
							"
						></textarea>

						<!-- Placeholder inline editor (text fields only) -->
						{#if hasPlaceholder}
							<textarea
								rows={1}
								value={placeholder}
								placeholder={defaultPlaceholder || 'Add placeholder…'}
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
									border-top: 1px dashed #374151;
									outline: none; resize: none; overflow: hidden;
									color: {placeholder ? '#6b7280' : '#374151'};
									font-size: 0.75rem; font-family: monospace;
									padding: 4px 4px 2px;
									cursor: text; font-style: italic;
								"
							></textarea>
						{/if}

						<!-- Options inline editor (multiple_choice / checkboxes) -->
						{#if hasOptions}
							{@const optionLabels = getOptionLabels(field.id)}
							<div
								onclick={(e) => e.stopPropagation()}
								style="margin-top: 8px; border-top: 1px solid #374151; padding-top: 8px; display: flex; flex-direction: column; gap: 4px;"
							>
								{#each optionLabels as optLabel, i}
									{@const opt = (field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig).options?.[i]}
									<div style="display: flex; align-items: center; gap: 6px;">
										<span style="color: #4b5563; font-size: 0.7rem; flex-shrink: 0; width: 14px; text-align: right;">{i + 1}.</span>
										<input
											type="text"
											value={optLabel}
											placeholder="Option {i + 1}"
											onfocus={(e) => { e.stopPropagation(); focusField(field.id); }}
											oninput={(e) => setOptionLabel(field.id, i, (e.target as HTMLInputElement).value)}
											style="
												flex: 1; min-width: 0;
												background: transparent; border: none;
												border-bottom: 1px solid #374151;
												outline: none;
												color: {optLabel ? '#d1d5db' : '#6b7280'};
												font-size: 0.8rem; font-family: inherit;
												padding: 2px 4px;
											"
										/>
										<button
											onclick={(e) => { e.stopPropagation(); if (opt) removeOption(field.id, opt.id); }}
											style="background: transparent; border: none; color: #4b5563; cursor: pointer; font-family: monospace; font-size: 0.9rem; padding: 0 2px; flex-shrink: 0; line-height: 1;"
											aria-label="Remove option"
										>×</button>
									</div>
								{/each}
								<button
									onclick={(e) => { e.stopPropagation(); focusField(field.id); addOption(field.id); }}
									style="
										align-self: flex-start;
										background: transparent; border: none;
										color: #6b7280; font-size: 0.75rem;
										cursor: pointer; font-family: inherit;
										padding: 2px 4px; margin-top: 2px;
									"
								>+ Add option</button>
							</div>
						{/if}

						<!-- Rating shape preview -->
						{#if isRating}
							{@const cfg = field.config as RatingConfig}
							<div style="margin-top: 8px; border-top: 1px solid #374151; padding-top: 8px; display: flex; gap: 4px; flex-wrap: wrap;">
								{#each { length: cfg.scale ?? 5 } as _, i}
									{#if cfg.shape === 'number'}
										<span style="
											display: inline-flex; align-items: center; justify-content: center;
											width: 28px; height: 28px;
											border: 1px solid #374151; border-radius: 4px;
											color: #6b7280; font-size: 0.75rem; font-family: monospace;
										">{i + 1}</span>
									{:else}
										<span style="color: #6b7280; font-size: 1.1rem; line-height: 1;">★</span>
									{/if}
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			{/each}
		</div>
	{/if}
	</div>
	{/if}
</main>
