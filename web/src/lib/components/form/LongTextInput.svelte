<script lang="ts">
	import FieldShell from './FieldShell.svelte';
	import type { BuilderField, LongTextConfig } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string; placeholder?: string };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
	const cfg = field.config as LongTextConfig;
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<textarea
		rows={cfg.minRows ?? 4}
		maxlength={cfg.maxLength}
		placeholder={translation.placeholder ?? ''}
		oninput={(e) => onchange(e.currentTarget.value)}
		class="form-input resize-y"
	>{value ?? ''}</textarea>
</FieldShell>
