<script lang="ts">
import type { BuilderField, ShortTextConfig } from "$lib/types/builder";
import type { AnswerValue } from "$lib/validation";
import FieldShell from "./FieldShell.svelte";

interface Props {
	field: BuilderField;
	translation: { label: string; helpText?: string; placeholder?: string };
	value: AnswerValue;
	error?: string | null;
	onchange: (v: AnswerValue) => void;
}

const { field, translation, value, error, onchange }: Props = $props();
const cfg = field.config as ShortTextConfig;
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<input
		type="text"
		value={String(value ?? '')}
		maxlength={cfg.maxLength}
		placeholder={translation.placeholder ?? ''}
		oninput={(e) => onchange(e.currentTarget.value)}
		class="form-input"
	/>
</FieldShell>
