<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		trigger: Snippet<[{ onclick: (e: MouseEvent) => void; 'aria-expanded': boolean; 'aria-haspopup': true }]>;
		children: Snippet<[{ close: () => void }]>;
		align?: 'start' | 'end';
	}

	let { trigger, children, align = 'end' }: Props = $props();

	let open = $state(false);
	let pos = $state<{ top: number; left: number } | null>(null);

	function toggle(e: MouseEvent) {
		if (open) {
			close();
		} else {
			const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
			pos = align === 'end'
				? { top: rect.bottom + 6, left: rect.right }
				: { top: rect.bottom + 6, left: rect.left };
			open = true;
		}
	}

	function close() {
		open = false;
		pos = null;
	}
</script>

{@render trigger({ onclick: toggle, 'aria-expanded': open, 'aria-haspopup': true })}

{#if open && pos}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-40" onclick={close}></div>
	<div
		class="menu fixed z-50 min-w-[9rem] font-mono"
		style="top: {pos.top}px; left: {pos.left}px; transform: translateX({align === 'end' ? '-100%' : '0'});"
		role="menu"
		aria-orientation="vertical"
	>
		{@render children({ close })}
	</div>
{/if}

<style>
	.menu {
		background: var(--color-surface-subtle);
		border: 1px solid var(--color-border-deep);
		border-radius: 8px;
		padding: 4px;
		box-shadow:
			0 4px 12px -2px var(--color-overlay-light),
			0 2px 4px -1px rgba(0, 0, 0, 0.2);
		animation: dropdown-in 120ms cubic-bezier(0.16, 1, 0.3, 1) both;
		transform-origin: top right;
	}

	@keyframes dropdown-in {
		from {
			opacity: 0;
			scale: 0.95;
			translate: 0 -4px;
		}
		to {
			opacity: 1;
			scale: 1;
			translate: 0 0;
		}
	}
</style>
