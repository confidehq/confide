<script lang="ts">
	import FieldShell from './FieldShell.svelte';
	import type { BuilderField, CheckboxesConfig } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string; options?: string[] };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
	const cfg = field.config as CheckboxesConfig;
	const checked = $derived<string[]>(Array.isArray(value) ? (value as string[]) : []);

	function getLabel(idx: number): string {
		return translation.options?.[idx] ?? `Option ${idx + 1}`;
	}

	function toggle(optId: string) {
		const current = [...checked];
		const idx = current.indexOf(optId);
		if (idx >= 0) {
			current.splice(idx, 1);
		} else {
			current.push(optId);
		}
		onchange(current);
	}
</script>

<FieldShell label={translation.label} required={field.required} helpText={translation.helpText} {error}>
	<div style="display: flex; flex-direction: column; gap: 8px;">
		{#each cfg.options as opt, i (opt.id)}
			<label style="display: flex; align-items: center; gap: 10px; cursor: pointer; font-size: 0.9rem; color: #374151;">
				<input
					type="checkbox"
					checked={checked.includes(opt.id)}
					onchange={() => toggle(opt.id)}
					style="accent-color: #1d4ed8; width: 16px; height: 16px; flex-shrink: 0;"
				/>
				{getLabel(i)}
			</label>
		{/each}
	</div>
</FieldShell>
