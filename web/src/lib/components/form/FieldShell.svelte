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

<div class="flex flex-col gap-1.5 w-full">
	<label class="text-base font-semibold text-form-text">
		{label}{#if required}<span class="text-danger-border ml-0.5" aria-hidden="true">*</span>{/if}
	</label>
	{#if helpText}
		<p class="m-0 text-sm text-form-muted">{helpText}</p>
	{/if}
	{@render children()}
	{#if error}
		<p class="m-0 text-sm text-danger-border" role="alert">{error}</p>
	{/if}
</div>
