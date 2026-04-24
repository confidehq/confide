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
	<div class="flex flex-col gap-2">
		{#each cfg.options as opt, i (opt.id)}
			<label class="flex items-center gap-2.5 cursor-pointer text-base text-form-text-mid">
				<input
					type="checkbox"
					checked={checked.includes(opt.id)}
					onchange={() => toggle(opt.id)}
					class="accent-form-primary w-4 h-4 shrink-0"
				/>
				{getLabel(i)}
			</label>
		{/each}
	</div>
</FieldShell>
