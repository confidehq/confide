<script lang="ts">
	import type { BuilderField } from '$lib/types/builder';

	interface Props {
		field: BuilderField;
		translation: { label: string; helpText?: string };
	}

	// field kept for prop consistency
	const { translation }: Props = $props();
</script>

<details style="border: 1px solid #e5e7eb; border-radius: 6px; overflow: hidden;">
	<summary style="
		padding: 12px 16px;
		font-size: 0.9rem;
		font-weight: 600;
		color: #374151;
		cursor: pointer;
		user-select: none;
		list-style: none;
		display: flex;
		align-items: center;
		gap: 8px;
	">
		<span style="font-size: 0.7rem; color: #9ca3af;">▶</span>
		{translation.label}
	</summary>
	{#if translation.helpText}
		<div style="padding: 12px 16px; border-top: 1px solid #e5e7eb; font-size: 0.875rem; color: #4b5563; line-height: 1.6;">
			{translation.helpText}
		</div>
	{/if}
</details>
