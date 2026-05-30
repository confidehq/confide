<script lang="ts">
	import { X, TriangleAlert } from '@lucide/svelte';

	interface Props {
		open: boolean;
		title: string;
		description?: string;
		confirmLabel?: string;
		loadingLabel?: string;
		loading?: boolean;
		error?: string;
		onconfirm: () => void;
		oncancel: () => void;
	}

	let {
		open,
		title,
		description,
		confirmLabel = 'Delete',
		loadingLabel,
		loading = false,
		error = '',
		onconfirm,
		oncancel
	}: Props = $props();

	function handleBackdrop(e: MouseEvent) {
		if (e.target === e.currentTarget && !loading) oncancel();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && !loading) oncancel();
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 backdrop"
		onclick={handleBackdrop}
		onkeydown={handleKeydown}
		role="presentation"
	>
		<div
			class="dialog font-mono w-full max-w-sm flex flex-col gap-5"
			role="dialog"
			aria-modal="true"
			aria-labelledby="confirm-dialog-title"
		>
			<!-- Header -->
			<div class="flex items-start justify-between gap-3">
				<div class="flex items-center gap-2.5 min-w-0">
					<span class="shrink-0 flex items-center justify-center w-7 h-7 rounded-md bg-danger-dark border border-danger-dim">
						<TriangleAlert size={14} strokeWidth={1.75} class="text-danger" />
					</span>
					<h2
						id="confirm-dialog-title"
						class="m-0 text-base font-semibold text-text leading-snug"
					>{title}</h2>
				</div>
				<button
					onclick={oncancel}
					disabled={loading}
					class="shrink-0 w-7 h-7 flex items-center justify-center rounded bg-transparent border-none cursor-pointer
						text-subtle hover:text-text hover:bg-surface transition-colors duration-100
						disabled:pointer-events-none"
					aria-label="Cancel"
				>
					<X size={15} strokeWidth={1.75} />
				</button>
			</div>

			<!-- Description -->
			{#if description}
				<p class="m-0 text-sm text-subtle leading-relaxed">{description}</p>
			{/if}

			<!-- Error -->
			{#if error}
				<p class="m-0 text-sm text-error-light">{error}</p>
			{/if}

			<!-- Divider -->
			<div class="h-px bg-border-deep"></div>

			<!-- Actions -->
			<div class="flex gap-2 justify-end">
				<button
					onclick={oncancel}
					disabled={loading}
					class="px-4 py-2 bg-transparent text-subtle border border-border-canvas rounded cursor-pointer
						font-mono text-sm hover:text-text hover:border-muted transition-colors duration-100
						disabled:pointer-events-none"
				>Cancel</button>
				<button
					onclick={onconfirm}
					disabled={loading}
					class="px-4 py-2 border-none rounded cursor-pointer font-mono text-sm transition-colors duration-100
						{loading
							? 'bg-muted text-subtle cursor-not-allowed'
							: 'bg-danger-dark text-danger hover:bg-danger-dim hover:text-base'}"
				>{loading ? (loadingLabel ?? 'Deleting…') : confirmLabel}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		background: var(--color-overlay);
		backdrop-filter: blur(2px);
		animation: fade-in 120ms ease-out both;
	}

	.dialog {
		background: var(--color-canvas);
		border: 1px solid var(--color-border);
		border-radius: 10px;
		padding: 1.25rem;
		box-shadow:
			0 0 0 1px rgba(255,255,255,0.04) inset,
			0 24px 48px -12px rgba(0, 0, 0, 0.7),
			0 8px 16px -4px var(--color-overlay-light);
		animation: slide-up 150ms cubic-bezier(0.34, 1.3, 0.64, 1) both;
	}

	@keyframes fade-in {
		from { opacity: 0; }
		to   { opacity: 1; }
	}

	@keyframes slide-up {
		from { opacity: 0; transform: translateY(8px) scale(0.97); }
		to   { opacity: 1; transform: translateY(0)   scale(1);    }
	}
</style>
