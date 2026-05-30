<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		sublabel,
		pct,
		children,
		footer,
	}: {
		label: string;
		sublabel: string;
		pct?: number;
		children?: Snippet;
		footer?: Snippet;
	} = $props();
</script>

<div class="border border-border-canvas rounded-lg px-5 py-4 flex flex-col gap-3">
	<div>
		<p class="m-0 text-sm font-semibold text-text">{label}</p>
		<p class="m-0 text-xs text-subtle mt-0.5">{sublabel}</p>
	</div>
	<p class="m-0 text-3xl font-semibold tabular-nums text-text leading-none">
		{@render children?.()}
	</p>
	{#if pct !== undefined}
		<div class="h-1.5 bg-canvas rounded-full overflow-hidden">
			<div
				class="h-full rounded-full transition-all duration-300
					{pct >= 100 ? 'bg-error-light' : pct >= 80 ? 'bg-warning-text' : 'bg-text-blue'}"
				style="width: {pct}%"
			></div>
		</div>
	{:else if footer}
		{@render footer()}
	{/if}
</div>
