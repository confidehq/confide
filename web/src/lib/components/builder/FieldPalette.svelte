<script lang="ts">
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { FieldType } from '$lib/types/builder';

	interface Props {
		store: ReturnType<typeof createBuilderStore>;
	}

	const { store }: Props = $props();

	const fieldTypes: Array<{ type: FieldType; label: string; icon: string }> = [
		{ type: 'short_text', label: 'Short text', icon: 'Aa' },
		{ type: 'long_text', label: 'Long text', icon: '¶' },
		{ type: 'multiple_choice', label: 'Multiple choice', icon: '◉' },
		{ type: 'checkboxes', label: 'Checkboxes', icon: '☑' },
		{ type: 'dropdown', label: 'Dropdown', icon: '▾' },
		{ type: 'date_time', label: 'Date / time', icon: '📅' },
		{ type: 'rating', label: 'Rating', icon: '★' },
		{ type: 'section_break', label: 'Section break', icon: '—' },
		{ type: 'heading', label: 'Heading', icon: 'H' },
		{ type: 'accordion', label: 'Accordion', icon: '▸' },
		{ type: 'accent', label: 'Accent block', icon: '!' }
	];
</script>

<aside style="
	width: 240px;
	background: #1f2937;
	border-right: 1px solid #374151;
	padding: 16px;
	overflow-y: auto;
	flex-shrink: 0;
">
	<p style="margin: 0 0 12px; font-size: 0.75rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">
		Fields
	</p>

	<div style="display: flex; flex-direction: column; gap: 4px;">
		{#each fieldTypes as { type, label, icon }}
			<button
				onclick={() => store.addField(type)}
				style="
					display: flex; align-items: center; gap: 10px;
					padding: 8px 12px;
					background: transparent;
					color: #d1d5db;
					border: 1px solid #374151;
					border-radius: 6px;
					cursor: pointer;
					font-family: monospace;
					font-size: 0.8rem;
					text-align: left;
					transition: background 0.1s;
				"
				onmouseenter={(e) => { (e.currentTarget as HTMLElement).style.background = '#374151'; }}
				onmouseleave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
			>
				<span style="width: 20px; text-align: center; flex-shrink: 0; font-size: 0.95rem;">{icon}</span>
				<span>{label}</span>
			</button>
		{/each}
	</div>
</aside>
