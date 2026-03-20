<script lang="ts">
	import FieldShell from './FieldShell.svelte';
	import type { BuilderField, MultipleChoiceConfig } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string; options?: string[] };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
	const cfg = field.config as MultipleChoiceConfig;

	const isOther = $derived(typeof value === 'string' && (value as string).startsWith('other:'));
	let otherText = $state(
		typeof value === 'string' && (value as string).startsWith('other:')
			? (value as string).slice(6)
			: ''
	);

	function getLabel(idx: number): string {
		return translation.options?.[idx] ?? `Option ${idx + 1}`;
	}

	function handleOtherText(e: Event) {
		otherText = (e.currentTarget as HTMLInputElement).value;
		onchange(`other:${otherText}`);
	}
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<div style="display: flex; flex-direction: column; gap: 8px;">
		{#each cfg.options as opt, i (opt.id)}
			<label style="display: flex; align-items: center; gap: 10px; cursor: pointer; font-size: 0.9rem; color: #374151;">
				<input
					type="radio"
					name={field.id}
					checked={value === opt.id}
					onchange={() => onchange(opt.id)}
					style="accent-color: #1d4ed8; flex-shrink: 0;"
				/>
				{getLabel(i)}
			</label>
		{/each}
		{#if cfg.allowOther}
			<label style="display: flex; align-items: center; gap: 10px; cursor: pointer; font-size: 0.9rem; color: #374151;">
				<input
					type="radio"
					name={field.id}
					checked={isOther}
					onchange={() => onchange(`other:${otherText}`)}
					style="accent-color: #1d4ed8; flex-shrink: 0;"
				/>
				Other:
				<input
					type="text"
					value={otherText}
					oninput={handleOtherText}
					onfocus={() => onchange(`other:${otherText}`)}
					placeholder="Please specify"
					style="flex: 1; padding: 4px 8px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 0.85rem; font-family: inherit;"
				/>
			</label>
		{/if}
	</div>
</FieldShell>
