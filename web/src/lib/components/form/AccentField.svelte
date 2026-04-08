<script lang="ts">
	import type { BuilderField, AccentConfig } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	const { field, translation }: Props = $props();

	const variant = $derived((field.config as AccentConfig).variant ?? 'note');

	const styles = $derived({
		note:    { border: '#3b82f6', bg: '#eff6ff', color: '#1e40af' },
		warning: { border: '#f59e0b', bg: '#fffbeb', color: '#92400e' },
		danger:  { border: '#ef4444', bg: '#fef2f2', color: '#991b1b' },
		success: { border: '#22c55e', bg: '#f0fdf4', color: '#166534' },
	}[variant]);
</script>

<div
	style="border-left-color: {styles.border}; background: {styles.bg}; color: {styles.color};"
	class="px-4 py-3 border-l-4 rounded text-sm leading-relaxed"
>
	<p class="m-0 font-semibold">{translation.label}</p>
	{#if translation.helpText}
		<p class="mt-1 m-0">{translation.helpText}</p>
	{/if}
</div>
