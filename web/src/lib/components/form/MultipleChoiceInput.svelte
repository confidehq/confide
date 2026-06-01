<script lang="ts">
import type { BuilderField, MultipleChoiceConfig } from "$lib/types/builder";
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
const cfg = field.config as MultipleChoiceConfig;

const isOther = $derived(
	typeof value === "string" && (value as string).startsWith("other:"),
);
let otherText = $state(
	typeof value === "string" && (value as string).startsWith("other:")
		? (value as string).slice(6)
		: "",
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
			<label class="flex items-center gap-2.5 cursor-pointer text-base text-form-text-mid">
				<input
					type="radio"
					name={field.id}
					checked={value === opt.id}
					onchange={() => onchange(opt.id)}
					class="accent-form-primary shrink-0"
				/>
				{getLabel(i)}
			</label>
		{/each}
		{#if cfg.allowOther}
			<label class="flex items-center gap-2.5 cursor-pointer text-base text-form-text-mid">
				<input
					type="radio"
					name={field.id}
					checked={isOther}
					onchange={() => onchange(`other:${otherText}`)}
					class="accent-form-primary shrink-0"
				/>
				Other:
				<input
					type="text"
					value={otherText}
					oninput={handleOtherText}
					onfocus={() => onchange(`other:${otherText}`)}
					placeholder="Please specify"
					class="flex-1 px-2 py-1 border border-form-border rounded text-base font-[inherit]"
				/>
			</label>
		{/if}
	</div>
</FieldShell>
