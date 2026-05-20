<script lang="ts">
	import type { BuilderField, AccentConfig } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	const { field, translation }: Props = $props();

	const variant = $derived((field.config as AccentConfig).variant ?? 'note');

	const styles = $derived({
		note:    { border: 'var(--color-info-border)',    bg: 'var(--color-info-bg)',    color: 'var(--color-info-text)' },
		warning: { border: 'var(--color-warning-border)', bg: 'var(--color-warning-bg)', color: 'var(--color-warning-text)' },
		danger:  { border: 'var(--color-danger-border)',  bg: 'var(--color-danger-bg)',  color: 'var(--color-danger-text)' },
		success: { border: 'var(--color-success-border)', bg: 'var(--color-success-bg)', color: 'var(--color-success-text)' },
	}[variant]);
</script>

<div
	style="border-left-color: {styles.border}; background: {styles.bg}; color: {styles.color};"
	class="px-4 py-3 border-l-4 rounded text-base leading-relaxed"
>
	<p class="m-0 font-semibold">{@html translation.label}</p>
	{#if translation.helpText}
		<p class="mt-1 m-0">{@html translation.helpText}</p>
	{/if}
</div>
