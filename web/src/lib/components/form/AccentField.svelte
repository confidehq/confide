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

<div style="
	padding: 12px 16px;
	border-left: 4px solid {styles.border};
	background: {styles.bg};
	border-radius: 4px;
	font-size: 0.875rem;
	color: {styles.color};
	line-height: 1.6;
">
	<p style="margin: 0; font-weight: 600;">{translation.label}</p>
	{#if translation.helpText}
		<p style="margin: 4px 0 0;">{translation.helpText}</p>
	{/if}
</div>
