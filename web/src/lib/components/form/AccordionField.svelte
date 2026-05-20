<script lang="ts">
	import type { BuilderField } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	// field kept for prop consistency
	const { translation }: Props = $props();
</script>

<details class="border border-form-border-light rounded-md overflow-hidden">
	<summary class="px-4 py-3 text-base font-semibold text-form-text-mid cursor-pointer select-none list-none flex items-center gap-2">
		<span class="text-sm text-form-muted-light">▶</span>
		{translation.label}
	</summary>
	{#if translation.helpText}
		<div class="px-4 py-3 border-t border-form-border-light text-base text-form-text-dim leading-relaxed">
			{@html translation.helpText}
		</div>
	{/if}
</details>
