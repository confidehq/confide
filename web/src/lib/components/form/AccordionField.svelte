<script lang="ts">
	import type { BuilderField } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	// field kept for prop consistency
	const { translation }: Props = $props();
</script>

<details class="border border-[#e5e7eb] rounded-md overflow-hidden">
	<summary class="px-4 py-3 text-sm font-semibold text-[#374151] cursor-pointer select-none list-none flex items-center gap-2">
		<span class="text-xs text-[#9ca3af]">▶</span>
		{translation.label}
	</summary>
	{#if translation.helpText}
		<div class="px-4 py-3 border-t border-[#e5e7eb] text-sm text-[#4b5563] leading-relaxed">
			{translation.helpText}
		</div>
	{/if}
</details>
