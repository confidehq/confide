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

<aside class="hidden xl:flex w-[240px] bg-base border border-border-canvas rounded-xl my-2 ml-2 p-4 overflow-y-auto shrink-0 flex-col">
	<p class="m-0 mb-3 text-sm text-subtle uppercase tracking-[0.05em]">Fields</p>

	<div class="flex flex-col gap-1">
		{#each fieldTypes as { type, label, icon }}
			<button
				onclick={() => store.addField(type)}
				class="flex items-center gap-2.5 px-3 py-2 bg-transparent text-text border border-border rounded-md cursor-pointer font-mono text-sm text-left transition-[background] duration-100 hover:bg-border"
			>
				<span class="w-5 flex justify-center shrink-0 text-subtle">
					<svelte:component this={icon} size={15} strokeWidth={1.75} />
				</span>
				<span>{label}</span>
			</button>
		{/each}
	</div>
</aside>
