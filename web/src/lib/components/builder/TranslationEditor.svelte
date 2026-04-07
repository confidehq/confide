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
</script>

<div class="flex flex-col gap-3.5">
	<!-- Label -->
	<div>
		<label class="block text-[0.75rem] text-muted mb-1">
			Label {#if !currentFieldTranslation?.label}<span class="text-[#f59e0b]">*</span>{/if}
		</label>
		<textarea
			value={currentFieldTranslation?.label ?? ''}
			oninput={(e) => fieldId && store.updateTranslation(fieldId, 'label', (e.target as HTMLTextAreaElement).value)}
			rows={2}
			class="input-base {!currentFieldTranslation?.label ? '!border-[#92400e]' : ''}"
		></textarea>
		{#if isNonDefaultLocale && defaultLocaleTranslation?.label}
			<p class="mt-1 m-0 text-[0.82rem] text-muted-dark">
				{store.schema.defaultLocale}: {defaultLocaleTranslation.label}
			</p>
		{/if}
	</div>

	<!-- Help text -->
	<div>
		<label class="block text-[0.75rem] text-muted mb-1">Help text</label>
		<textarea
			value={currentFieldTranslation?.helpText ?? ''}
			oninput={(e) => fieldId && store.updateTranslation(fieldId, 'helpText', (e.target as HTMLTextAreaElement).value)}
			rows={2}
			class="input-base"
		></textarea>
		{#if isNonDefaultLocale && defaultLocaleTranslation?.helpText}
			<p class="mt-1 m-0 text-[0.82rem] text-muted-dark">
				{store.schema.defaultLocale}: {defaultLocaleTranslation.helpText}
			</p>
		{/if}
	</div>

	<!-- Placeholder (for text fields) -->
	{#if selectedField?.type === 'short_text' || selectedField?.type === 'long_text'}
		<div>
			<label class="block text-[0.75rem] text-muted mb-1">Placeholder</label>
			<input
				type="text"
				value={currentFieldTranslation?.placeholder ?? ''}
				oninput={(e) => fieldId && store.updateTranslation(fieldId, 'placeholder', (e.target as HTMLInputElement).value)}
				class="input-base"
			/>
			{#if isNonDefaultLocale && defaultLocaleTranslation?.placeholder}
				<p class="mt-1 m-0 text-[0.82rem] text-muted-dark">
					{store.schema.defaultLocale}: {defaultLocaleTranslation.placeholder}
				</p>
			{/if}
		</div>
	{/if}

	<!-- Options (for choice fields) -->
	{#if hasOptions}
		<div>
			<label class="block text-[0.75rem] text-muted mb-1">Options</label>
			<div class="flex flex-col gap-1.5">
				{#each { length: optionsCount } as _, i}
					<div>
						<input
							type="text"
							placeholder="Option {i + 1}"
							value={getOptionLabel(i)}
							oninput={(e) => setOptionLabel(i, (e.target as HTMLInputElement).value)}
							class="input-base text-[0.8rem] {!getOptionLabel(i) ? '!border-[#92400e]' : ''}"
						/>
						{#if isNonDefaultLocale && getDefaultOptionLabel(i)}
							<p class="mt-0.5 m-0 text-[0.82rem] text-muted-dark">
								{store.schema.defaultLocale}: {getDefaultOptionLabel(i)}
							</p>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
