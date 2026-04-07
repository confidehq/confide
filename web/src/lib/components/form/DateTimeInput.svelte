<script lang="ts">
	import FieldShell from './FieldShell.svelte';
	import type { BuilderField, DateTimeConfig } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
	const cfg = field.config as DateTimeConfig;
	const inputType = cfg.mode === 'datetime' ? 'datetime-local' : cfg.mode;
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<input
		type={inputType}
		value={String(value ?? '')}
		min={cfg.min}
		max={cfg.max}
		onchange={(e) => onchange(e.currentTarget.value || null)}
		class="form-input"
	/>
</FieldShell>
