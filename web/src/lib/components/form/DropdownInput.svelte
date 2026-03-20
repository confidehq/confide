<script lang="ts">
	import FieldShell from './FieldShell.svelte';
	import type { BuilderField, DropdownConfig } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string; options?: string[] };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
	const cfg = field.config as DropdownConfig;

	function getLabel(idx: number): string {
		return translation.options?.[idx] ?? `Option ${idx + 1}`;
	}
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<select
		value={String(value ?? '')}
		onchange={(e) => onchange(e.currentTarget.value || null)}
		style="width: 100%; padding: 8px 12px; border: 1.5px solid #d1d5db; border-radius: 6px; font-size: 0.9rem; font-family: inherit; background: white; cursor: pointer; box-sizing: border-box;"
	>
		<option value="">— Select —</option>
		{#each cfg.options as opt, i (opt.id)}
			<option value={opt.id} selected={value === opt.id}>{getLabel(i)}</option>
		{/each}
	</select>
</FieldShell>
