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
	<p style="margin: 0; font-size: 0.9375rem; color: #4b5563; line-height: 1.6;">{translation.label}</p>
{:else if level === 1}
	<h1 style="margin: 0; font-size: 1.75rem; font-weight: 700; color: #111;">{translation.label}</h1>
{:else if level === 2}
	<h2 style="margin: 0; font-size: 1.25rem; font-weight: 700; color: #111;">{translation.label}</h2>
{:else}
	<h3 style="margin: 0; font-size: 1rem; font-weight: 700; color: #374151;">{translation.label}</h3>
{/if}
{#if translation.helpText}
	<p style="margin: 4px 0 0; font-size: 0.875rem; color: #6b7280; line-height: 1.6;">{translation.helpText}</p>
{/if}
