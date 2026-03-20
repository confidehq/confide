<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		label: string;
		required?: boolean;
		helpText?: string;
		error?: string | null;
		children: Snippet;
	}

	const { label, required, helpText, error, children }: Props = $props();
</script>

<div style="display: flex; flex-direction: column; gap: 6px; width: 100%;">
	<label style="font-size: 0.9rem; font-weight: 600; color: #111;">
		{label}{#if required}<span style="color: #ef4444; margin-left: 2px;" aria-hidden="true">*</span>{/if}
	</label>
	{#if helpText}
		<p style="margin: 0; font-size: 0.8rem; color: #6b7280;">{helpText}</p>
	{/if}
	{@render children()}
	{#if error}
		<p style="margin: 0; font-size: 0.75rem; color: #ef4444;" role="alert">{error}</p>
	{/if}
</div>
