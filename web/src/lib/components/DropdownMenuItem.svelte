<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		onclick?: () => void;
		variant?: 'default' | 'destructive';
		disabled?: boolean;
		children: Snippet;
	}

	let { onclick, variant = 'default', disabled = false, children }: Props = $props();
</script>

<button
	class="item"
	class:destructive={variant === 'destructive'}
	class:disabled
	{onclick}
	{disabled}
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
		color: var(--color-text-body);
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
		background: var(--color-surface-hover);
		color: var(--color-text-bright);
	}

	.item.destructive {
		color: var(--color-error-light);
	}

	.item.destructive:hover,
	.item.destructive:focus-visible {
		background: var(--color-danger-bg-dark);
		color: var(--color-danger-text-dark);
	}

	.item.disabled,
	.item.disabled:hover,
	.item.disabled:focus-visible {
		opacity: 0.5;
		cursor: default;
		background: transparent;
		color: var(--color-text-body);
	}
</style>
