<script lang="ts">
	import type { BuilderField } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	// field prop kept for prop consistency with other inputs; not used in render
	const { translation }: Props = $props();
</script>

<div style="margin: 4px 0; padding-top: 8px; border-top: 1px solid #e5e7eb;">
	{#if translation.label}
		<h2 style="margin: 0 0 4px; font-size: 1rem; font-weight: 700; color: #111;">{translation.label}</h2>
	{/if}
	{#if translation.helpText}
		<p style="margin: 0; font-size: 0.85rem; color: #6b7280;">{translation.helpText}</p>
	{/if}
</div>
