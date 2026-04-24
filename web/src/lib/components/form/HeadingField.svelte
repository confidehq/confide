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
	<p class="m-0 text-base text-form-text-dim leading-relaxed">{translation.label}</p>
{:else if level === 1}
	<h1 class="m-0 text-4xl font-bold text-form-text">{translation.label}</h1>
{:else if level === 2}
	<h2 class="m-0 text-2xl font-bold text-form-text">{translation.label}</h2>
{:else}
	<h3 class="m-0 text-lg font-bold text-form-text-mid">{translation.label}</h3>
{/if}
{#if translation.helpText}
	<p class="mt-1 m-0 text-base text-form-muted leading-relaxed">{translation.helpText}</p>
{/if}
