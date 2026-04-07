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
	<div class="flex flex-col gap-2">
		{#each cfg.options as opt, i (opt.id)}
			<label class="flex items-center gap-2.5 cursor-pointer text-[0.9rem] text-[#374151]">
				<input
					type="radio"
					name={field.id}
					checked={value === opt.id}
					onchange={() => onchange(opt.id)}
					class="accent-[#1d4ed8] shrink-0"
				/>
				{getLabel(i)}
			</label>
		{/each}
		{#if cfg.allowOther}
			<label class="flex items-center gap-2.5 cursor-pointer text-[0.9rem] text-[#374151]">
				<input
					type="radio"
					name={field.id}
					checked={isOther}
					onchange={() => onchange(`other:${otherText}`)}
					class="accent-[#1d4ed8] shrink-0"
				/>
				Other:
				<input
					type="text"
					value={otherText}
					oninput={handleOtherText}
					onfocus={() => onchange(`other:${otherText}`)}
					placeholder="Please specify"
					class="flex-1 px-2 py-1 border border-[#d1d5db] rounded text-[0.85rem] font-[inherit]"
				/>
			</label>
		{/if}
	</div>
</FieldShell>
