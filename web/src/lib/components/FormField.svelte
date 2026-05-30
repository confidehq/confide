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
	<label for={id} class="text-sm font-semibold tracking-[0.08em] uppercase text-subtle">
		{label}
	</label>
	{#if readonly}
		<p class="m-0 px-3 py-2.5 border border-border rounded text-base text-subtle bg-canvas select-all break-all">
			{value}
		</p>
	{:else}
		<input
			{id}
			{type}
			bind:value
			{placeholder}
			class="font-mono bg-canvas border border-border rounded px-3 py-2.5 text-base text-text
				placeholder-subtle focus:outline-none focus:border-border transition-colors duration-100"
		/>
	{/if}
	{#if help}
		<p class="m-0 text-xs text-subtle">{help}</p>
	{/if}
</div>
