<script lang="ts">
	import { X, TriangleAlert } from '@lucide/svelte';

	interface Props {
		open: boolean;
		title: string;
		description?: string;
		confirmLabel?: string;
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
					<span class="shrink-0 flex items-center justify-center w-7 h-7 rounded-md bg-[#2d1515] border border-[#7f1d1d]">
						<TriangleAlert size={14} strokeWidth={1.75} class="text-[#f87171]" />
					</span>
					<h2
						id="confirm-dialog-title"
						class="m-0 text-base font-semibold text-[#e2e8f0] leading-snug"
					>{title}</h2>
				</div>
				<button
					onclick={oncancel}
					disabled={loading}
					class="shrink-0 w-7 h-7 flex items-center justify-center rounded bg-transparent border-none cursor-pointer
						text-[#374d63] hover:text-[#c5d3e0] hover:bg-[#1a2840] transition-colors duration-100
						disabled:pointer-events-none"
					aria-label="Cancel"
				>
					<X size={15} strokeWidth={1.75} />
				</button>
			</div>

			<!-- Description -->
			{#if description}
				<p class="m-0 text-sm text-[#4b6280] leading-relaxed">{description}</p>
			{/if}

			<!-- Error -->
			{#if error}
				<p class="m-0 text-sm text-[#f87171]">{error}</p>
			{/if}

			<!-- Divider -->
			<div class="h-px bg-[#1e3048]"></div>

			<!-- Actions -->
			<div class="flex gap-2 justify-end">
				<button
					onclick={oncancel}
					disabled={loading}
					class="px-4 py-2 bg-transparent text-[#4b6280] border border-[#1e3048] rounded cursor-pointer
						font-mono text-sm hover:text-[#c5d3e0] hover:border-[#374d63] transition-colors duration-100
						disabled:pointer-events-none"
				>Cancel</button>
				<button
					onclick={onconfirm}
					disabled={loading}
					class="px-4 py-2 border-none rounded cursor-pointer font-mono text-sm transition-colors duration-100
						{loading
							? 'bg-[#374d63] text-[#4b6280] cursor-not-allowed'
							: 'bg-[#991b1b] text-[#fecaca] hover:bg-[#b91c1c]'}"
				>{loading ? 'Deleting…' : confirmLabel}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		background: rgba(0, 0, 0, 0.65);
		backdrop-filter: blur(2px);
		animation: fade-in 120ms ease-out both;
	}

	.dialog {
		background: #0d1b2a;
		border: 1px solid #1e3048;
		border-radius: 10px;
		padding: 1.25rem;
		box-shadow:
			0 0 0 1px rgba(255,255,255,0.04) inset,
			0 24px 48px -12px rgba(0, 0, 0, 0.7),
			0 8px 16px -4px rgba(0, 0, 0, 0.4);
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
