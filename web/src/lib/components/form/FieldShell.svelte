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
	<label class="text-[0.9rem] font-semibold text-[#111]">
		{label}{#if required}<span class="text-[#ef4444] ml-0.5" aria-hidden="true">*</span>{/if}
	</label>
	{#if helpText}
		<p class="m-0 text-[0.8rem] text-[#6b7280]">{helpText}</p>
	{/if}
	{@render children()}
	{#if error}
		<p class="m-0 text-[0.75rem] text-[#ef4444]" role="alert">{error}</p>
	{/if}
</div>
