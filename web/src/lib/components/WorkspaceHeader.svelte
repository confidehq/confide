<script lang="ts">
import { Building2 } from "@lucide/svelte";
import { workspacesStore } from "$lib/stores/workspaces.svelte";
import { planLabel } from "$lib/utils/plan";
</script>

{#if workspacesStore.active}
	{@const ws = workspacesStore.active}
	<div class="flex items-center gap-3 mb-4">
		<Building2 size={18} strokeWidth={1.75} class="shrink-0 text-subtle" />
		<span class="text-xl font-semibold text-text truncate min-w-0">{ws.name}</span>
		<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border
			{ws.plan === 'pro'
				? ws.planStatus === 'active' || ws.planStatus === 'canceling'
					? 'bg-open-bg text-open-text border-open-border'
					: 'bg-closed-bg text-closed-text border-closed-border'
				: 'text-subtle border-border-canvas bg-transparent'}">
			{planLabel(ws.plan, ws.planStatus)}
		</span>
		<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base text-subtle border border-border-canvas">
			{ws.role}
		</span>
	</div>
{/if}
