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
	<div class="flex gap-1.5 items-center flex-wrap">
		{#each items as n}
			{#if cfg.shape === 'star'}
				<button
					type="button"
					onclick={() => select(n)}
					style="color: {n <= current ? 'var(--color-warning-border)' : 'var(--color-form-border)'};"
					class="bg-none border-none cursor-pointer text-3xl p-0.5 leading-none transition-colors duration-100"
					aria-label="Rate {n} out of {cfg.scale}"
				>
					{n <= current ? '★' : '☆'}
				</button>
			{:else}
				<button
					type="button"
					onclick={() => select(n)}
					style="
						border-color: {n <= current ? 'var(--color-form-primary)' : 'var(--color-form-border)'};
						background: {n <= current ? 'var(--color-form-primary)' : 'var(--color-form-bg)'};
						color: {n <= current ? 'white' : 'var(--color-form-text-mid)'};
					"
					class="w-10 h-10 border-[1.5px] rounded-full text-sm font-[inherit] cursor-pointer transition-[background,border-color,color] duration-100"
					aria-label="Rate {n} out of {cfg.scale}"
				>
					{n}
				</button>
			{/if}
		{/each}
	</div>
</FieldShell>
