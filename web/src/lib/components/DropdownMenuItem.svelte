<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		onclick?: () => void;
		variant?: 'default' | 'destructive';
		children: Snippet;
	}

	let { onclick, variant = 'default', children }: Props = $props();
</script>

<button
	class="item"
	class:destructive={variant === 'destructive'}
	{onclick}
	role="menuitem"
	type="button"
>
	{@render children()}
</button>

<style>
	.item {
		display: flex;
		align-items: center;
		width: 100%;
		padding: 6px 8px;
		gap: 8px;
		background: transparent;
		border: none;
		border-radius: 5px;
		color: #c5d3e0;
		font-family: monospace;
		font-size: 0.875rem;
		line-height: 1.25rem;
		text-align: left;
		cursor: default;
		outline: none;
		transition: background 100ms, color 100ms;
	}

	.item:hover,
	.item:focus-visible {
		background: #1a2840;
		color: #e2e8f0;
	}

	.item.destructive {
		color: #f87171;
	}

	.item.destructive:hover,
	.item.destructive:focus-visible {
		background: #2d1515;
		color: #fca5a5;
	}
</style>
