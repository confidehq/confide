<script lang="ts">
	import { dndzone } from 'svelte-dnd-action';
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { BuilderField } from '$lib/types/builder';
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

	function hasTranslation(field: BuilderField): boolean {
		const t = store.schema.translations[store.activeLocale];
		if (!t) return false;
		const ft = t.fields[field.id];
		return !!ft?.label;
	}

	function handleDndConsider(e: CustomEvent<{ items: BuilderField[] }>) {
		store.reorderFields(e.detail.items);
	}

	function handleDndFinalize(e: CustomEvent<{ items: BuilderField[] }>) {
		store.reorderFields(e.detail.items);
	}

	// Derived: get current fields as a reactive list for dnd
	let fields = $derived([...store.schema.fields]);
</script>

<main
	style="
		flex: 1;
		overflow-y: auto;
		padding: 24px;
		background: {store.mode === 'preview' ? '#f9fafb' : '#111827'};
		min-width: 0;
	"
	onclick={() => store.setSelectedField(null)}
	role="presentation"
>
	{#if store.mode === 'preview'}
		<FormPreview schema={store.schema} locale={store.activeLocale} />
	{:else if fields.length === 0}
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
				{@const hasTrans = hasTranslation(field)}
				{@const isSectionBreak = field.type === 'section_break'}
				{@const label = store.schema.translations[store.activeLocale]?.fields[field.id]?.label
					|| store.schema.translations[store.schema.defaultLocale]?.fields[field.id]?.label
					|| ''}

				{#if isSectionBreak}
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
						<div style="flex: 1; height: 1px; background: #374151; position: relative;">
							{#if label}
								<span style="
									position: absolute; left: 50%; top: 50%;
									transform: translate(-50%, -50%);
									background: #111827; padding: 0 8px;
									font-size: 0.75rem; color: #9ca3af;
								">{label}</span>
							{/if}
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
					<div
						role="button"
						tabindex="0"
						onclick={(e) => { e.stopPropagation(); store.setSelectedField(field.id); }}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') store.setSelectedField(field.id); }}
						style="
							display: flex; align-items: center; gap: 12px;
							padding: 10px 12px;
							border: 1px solid {isSelected ? '#1d4ed8' : '#374151'};
							border-radius: 6px;
							background: {isSelected ? '#1e3a8a22' : '#1f2937'};
							cursor: pointer;
						"
					>
						<!-- Drag handle -->
						<span style="color: #6b7280; cursor: grab; font-size: 0.8rem; flex-shrink: 0;">⠿</span>

						<!-- Field type badge -->
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

						<!-- Field label -->
						<span style="
							flex: 1; font-size: 0.85rem;
							color: {label ? '#d1d5db' : '#6b7280'};
							overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
							min-width: 0;
						">
							{label || '(no label)'}
						</span>

						<!-- Missing translation warning -->
						{#if !hasTrans}
							<span
								title="Missing translation for {store.activeLocale}"
								style="color: #f59e0b; font-size: 0.85rem; flex-shrink: 0;"
							>
								⚠
							</span>
						{/if}

						<!-- Required badge -->
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

						<!-- Delete button -->
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
				{/if}
			{/each}
		</div>
	{/if}
</main>
