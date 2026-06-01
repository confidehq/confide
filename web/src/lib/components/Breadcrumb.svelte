<script lang="ts">
export interface BreadcrumbItem {
	label: string;
	href?: string;
	onclick?: () => void;
}

interface Props {
	items: BreadcrumbItem[];
}

let { items }: Props = $props();
</script>

<nav aria-label="Breadcrumb" class="flex items-center gap-0 min-w-0 overflow-hidden">
	{#each items as item, i}
		{@const isLast = i === items.length - 1}

		{#if i > 0}
			<span class="shrink-0 mx-1.5 text-border-mid select-none" aria-hidden="true">/</span>
		{/if}

		{#if item.onclick && !isLast}
			<button
				onclick={item.onclick}
				class="shrink-0 text-sm text-subtle bg-transparent border-none p-0 cursor-pointer whitespace-nowrap hover:text-subtle transition-colors duration-100"
			>{item.label}</button>
		{:else if item.href && !isLast}
			<a
				href={item.href}
				class="shrink-0 text-sm text-subtle no-underline whitespace-nowrap hover:text-subtle transition-colors duration-100"
			>{item.label}</a>
		{:else}
			<span
				class="text-sm whitespace-nowrap overflow-hidden text-ellipsis min-w-0
					{isLast ? 'text-text' : 'text-subtle shrink-0'}"
				title={item.label}
			>{item.label}</span>
		{/if}
	{/each}
</nav>
