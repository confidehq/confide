<script lang="ts">
import type { BuilderField, DropdownConfig } from "$lib/types/builder";
import type { AnswerValue } from "$lib/validation";
import FieldShell from "./FieldShell.svelte";

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
		class="form-input bg-white cursor-pointer"
	>
		<option value="">— Select —</option>
		{#each cfg.options as opt, i (opt.id)}
			<option value={opt.id} selected={value === opt.id}>{getLabel(i)}</option>
		{/each}
	</select>
</FieldShell>
