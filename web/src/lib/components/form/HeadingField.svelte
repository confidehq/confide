<script lang="ts">
	import type { BuilderField, HeadingConfig } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	const { field, translation }: Props = $props();

	const level = $derived((field.config as HeadingConfig).level ?? 2);
</script>

{#if level === 0}
	<p class="m-0 text-[0.9375rem] text-[#4b5563] leading-relaxed">{translation.label}</p>
{:else if level === 1}
	<h1 class="m-0 text-[1.75rem] font-bold text-[#111]">{translation.label}</h1>
{:else if level === 2}
	<h2 class="m-0 text-[1.25rem] font-bold text-[#111]">{translation.label}</h2>
{:else}
	<h3 class="m-0 text-base font-bold text-[#374151]">{translation.label}</h3>
{/if}
{#if translation.helpText}
	<p class="mt-1 m-0 text-[0.875rem] text-[#6b7280] leading-relaxed">{translation.helpText}</p>
{/if}
