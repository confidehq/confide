<script lang="ts">
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { FieldType } from '$lib/types/builder';
	import type { Component } from 'svelte';
	import {
		Type, AlignLeft, CircleDot, CheckSquare, ChevronDown,
		Calendar, Star, Minus, Heading1, ChevronRight, AlertCircle
	} from '@lucide/svelte';

	interface Props {
		store: ReturnType<typeof createBuilderStore>;
	}

	const { store }: Props = $props();

	const fieldTypes: Array<{ type: FieldType; label: string; icon: Component }> = [
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
</script>

<style>
	aside { display: none; }
	@media (min-width: 1440px) { aside { display: block; } }
</style>

<aside style="
	width: 240px;
	background: #1f2937;
	border-right: 1px solid #374151;
	padding: 16px;
	overflow-y: auto;
	flex-shrink: 0;
">
	<p style="margin: 0 0 12px; font-size: 0.875rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">
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
					font-size: 0.925rem;
					text-align: left;
					transition: background 0.1s;
				"
				onmouseenter={(e) => { (e.currentTarget as HTMLElement).style.background = '#374151'; }}
				onmouseleave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
			>
				<span style="width: 20px; display: flex; justify-content: center; flex-shrink: 0; color: #9ca3af;">
					<svelte:component this={icon} size={15} strokeWidth={1.75} />
				</span>
				<span>{label}</span>
			</button>
		{/each}
	</div>
</aside>
