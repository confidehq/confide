<script lang="ts">
	let {
		label,
		id,
		value = $bindable(''),
		type = 'text',
		placeholder = '',
		help,
		readonly = false,
	}: {
		label: string;
		id: string;
		value?: string;
		type?: string;
		placeholder?: string;
		help?: string;
		readonly?: boolean;
	} = $props();
</script>

<div class="flex flex-col gap-1.5">
	<label for={id} class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
		{label}
	</label>
	{#if readonly}
		<p class="m-0 px-3 py-2.5 border border-border-mid rounded text-base text-muted-dim bg-surface-read select-all break-all">
			{value}
		</p>
	{:else}
		<input
			{id}
			{type}
			bind:value
			{placeholder}
			class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
				placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
		/>
	{/if}
	{#if help}
		<p class="m-0 text-xs text-muted-mid">{help}</p>
	{/if}
</div>
