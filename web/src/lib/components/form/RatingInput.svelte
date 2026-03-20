<script lang="ts">
	import FieldShell from './FieldShell.svelte';
	import type { BuilderField, RatingConfig } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
	const cfg = field.config as RatingConfig;

	const current = $derived(typeof value === 'number' ? (value as number) : 0);
	const items = $derived(Array.from({ length: cfg.scale }, (_, i) => i + 1));

	function select(n: number) {
		// clicking the active rating clears it
		onchange(current === n ? null : n);
	}
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<div style="display: flex; gap: 6px; align-items: center; flex-wrap: wrap;">
		{#each items as n}
			{#if cfg.shape === 'star'}
				<button
					type="button"
					onclick={() => select(n)}
					style="background: none; border: none; cursor: pointer; font-size: 1.5rem; padding: 2px; color: {n <= current ? '#f59e0b' : '#d1d5db'}; line-height: 1;"
					aria-label="Rate {n} out of {cfg.scale}"
				>
					{n <= current ? '★' : '☆'}
				</button>
			{:else}
				<button
					type="button"
					onclick={() => select(n)}
					style="width: 36px; height: 36px; border: 1.5px solid {n <= current ? '#1d4ed8' : '#d1d5db'}; border-radius: 50%; background: {n <= current ? '#1d4ed8' : 'white'}; color: {n <= current ? 'white' : '#374151'}; font-size: 0.8rem; font-family: inherit; cursor: pointer;"
					aria-label="Rate {n} out of {cfg.scale}"
				>
					{n}
				</button>
			{/if}
		{/each}
	</div>
</FieldShell>
