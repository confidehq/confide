<script lang="ts">
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { MultipleChoiceConfig, CheckboxesConfig, DropdownConfig } from '$lib/types/builder';

	interface Props {
		store: ReturnType<typeof createBuilderStore>;
		fieldId: string | null;
	}

	const { store, fieldId }: Props = $props();

	const isNonDefaultLocale = $derived(store.activeLocale !== store.schema.defaultLocale);

	const defaultLocaleTranslation = $derived(
		fieldId
			? store.schema.translations[store.schema.defaultLocale]?.fields[fieldId]
			: null
	);

	const currentFieldTranslation = $derived(
		fieldId
			? store.schema.translations[store.activeLocale]?.fields[fieldId]
			: null
	);

	const selectedField = $derived(
		fieldId ? store.schema.fields.find((f) => f.id === fieldId) : null
	);

	const hasOptions = $derived(
		selectedField?.type === 'multiple_choice' ||
		selectedField?.type === 'checkboxes' ||
		selectedField?.type === 'dropdown'
	);

	const optionsCount = $derived(
		hasOptions
			? ((selectedField!.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig).options?.length ?? 0)
			: 0
	);

	function getOptionLabel(index: number): string {
		return currentFieldTranslation?.options?.[index] ?? '';
	}

	function setOptionLabel(index: number, value: string) {
		if (!fieldId) return;
		const current = currentFieldTranslation?.options ?? Array(optionsCount).fill('');
		const newOptions = [...current];
		while (newOptions.length <= index) newOptions.push('');
		newOptions[index] = value;
		store.updateTranslation(fieldId, 'options', newOptions as unknown as string);
	}

	function getDefaultOptionLabel(index: number): string {
		return defaultLocaleTranslation?.options?.[index] ?? '';
	}

	function inputStyle(isEmpty: boolean): string {
		return `
			width: 100%; padding: 6px 10px;
			background: #111827;
			border: 1px solid ${isEmpty ? '#92400e' : '#374151'};
			border-radius: 4px;
			color: #d1d5db;
			font-family: monospace; font-size: 0.8rem;
			outline: none; box-sizing: border-box;
		`;
	}
</script>

<div style="display: flex; flex-direction: column; gap: 14px;">
	<!-- Label -->
	<div>
		<label style="display: block; font-size: 0.75rem; color: #9ca3af; margin-bottom: 4px;">
			Label {#if !currentFieldTranslation?.label}<span style="color: #f59e0b;">*</span>{/if}
		</label>
		<textarea
			value={currentFieldTranslation?.label ?? ''}
			oninput={(e) => fieldId && store.updateTranslation(fieldId, 'label', (e.target as HTMLTextAreaElement).value)}
			rows={2}
			style={inputStyle(!currentFieldTranslation?.label)}
		></textarea>
		{#if isNonDefaultLocale && defaultLocaleTranslation?.label}
			<p style="margin: 4px 0 0; font-size: 0.72rem; color: #6b7280;">
				{store.schema.defaultLocale}: {defaultLocaleTranslation.label}
			</p>
		{/if}
	</div>

	<!-- Help text -->
	<div>
		<label style="display: block; font-size: 0.75rem; color: #9ca3af; margin-bottom: 4px;">Help text</label>
		<textarea
			value={currentFieldTranslation?.helpText ?? ''}
			oninput={(e) => fieldId && store.updateTranslation(fieldId, 'helpText', (e.target as HTMLTextAreaElement).value)}
			rows={2}
			style={inputStyle(false)}
		></textarea>
		{#if isNonDefaultLocale && defaultLocaleTranslation?.helpText}
			<p style="margin: 4px 0 0; font-size: 0.72rem; color: #6b7280;">
				{store.schema.defaultLocale}: {defaultLocaleTranslation.helpText}
			</p>
		{/if}
	</div>

	<!-- Placeholder (for text fields) -->
	{#if selectedField?.type === 'short_text' || selectedField?.type === 'long_text'}
		<div>
			<label style="display: block; font-size: 0.75rem; color: #9ca3af; margin-bottom: 4px;">Placeholder</label>
			<input
				type="text"
				value={currentFieldTranslation?.placeholder ?? ''}
				oninput={(e) => fieldId && store.updateTranslation(fieldId, 'placeholder', (e.target as HTMLInputElement).value)}
				style={inputStyle(false)}
			/>
			{#if isNonDefaultLocale && defaultLocaleTranslation?.placeholder}
				<p style="margin: 4px 0 0; font-size: 0.72rem; color: #6b7280;">
					{store.schema.defaultLocale}: {defaultLocaleTranslation.placeholder}
				</p>
			{/if}
		</div>
	{/if}

	<!-- Options (for choice fields) -->
	{#if hasOptions}
		<div>
			<label style="display: block; font-size: 0.75rem; color: #9ca3af; margin-bottom: 4px;">Options</label>
			<div style="display: flex; flex-direction: column; gap: 6px;">
				{#each { length: optionsCount } as _, i}
					<div>
						<input
							type="text"
							placeholder="Option {i + 1}"
							value={getOptionLabel(i)}
							oninput={(e) => setOptionLabel(i, (e.target as HTMLInputElement).value)}
							style={inputStyle(!getOptionLabel(i))}
						/>
						{#if isNonDefaultLocale && getDefaultOptionLabel(i)}
							<p style="margin: 2px 0 0; font-size: 0.72rem; color: #6b7280;">
								{store.schema.defaultLocale}: {getDefaultOptionLabel(i)}
							</p>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
