<script lang="ts">
	import type { BuilderField } from '$lib/types/builder';
	import type { AnswerValue } from '$lib/validation';
	import ShortTextInput from './ShortTextInput.svelte';
	import LongTextInput from './LongTextInput.svelte';
	import MultipleChoiceInput from './MultipleChoiceInput.svelte';
	import CheckboxesInput from './CheckboxesInput.svelte';
	import DropdownInput from './DropdownInput.svelte';
	import DateTimeInput from './DateTimeInput.svelte';
	import RatingInput from './RatingInput.svelte';
	import SectionBreakInput from './SectionBreakInput.svelte';
	import HeadingField from './HeadingField.svelte';
	import AccordionField from './AccordionField.svelte';
	import AccentField from './AccentField.svelte';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string; placeholder?: string; options?: string[] };
		value: AnswerValue;
		error?: string | null;
		onchange: (v: AnswerValue) => void;
	}

	const { field, translation, value, error, onchange }: Props = $props();
</script>

<div id="field-{field.id}">
	{#if field.type === 'section_break'}
		<SectionBreakInput {field} {translation} />
	{:else if field.type === 'short_text'}
		<ShortTextInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'long_text'}
		<LongTextInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'multiple_choice'}
		<MultipleChoiceInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'checkboxes'}
		<CheckboxesInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'dropdown'}
		<DropdownInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'date_time'}
		<DateTimeInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'rating'}
		<RatingInput {field} {translation} {value} {error} {onchange} />
	{:else if field.type === 'heading'}
		<HeadingField {field} {translation} />
	{:else if field.type === 'accordion'}
		<AccordionField {field} {translation} />
	{:else if field.type === 'accent'}
		<AccentField {field} {translation} />
	{/if}
</div>
