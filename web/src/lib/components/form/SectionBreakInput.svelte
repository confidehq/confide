<script lang="ts">
	import type { BuilderField } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	// field prop kept for prop consistency with other inputs; not used in render
	const { translation }: Props = $props();
</script>

<div class="my-1 pt-2 border-t border-form-border-light">
	{#if translation.label}
		<h2 class="m-0 mb-1 text-base font-bold text-form-text">{translation.label}</h2>
	{/if}
	{#if translation.helpText}
		<p class="m-0 text-sm text-form-muted">{translation.helpText}</p>
	{/if}
</div>
