<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { updateFormStatus, deleteForm, type FormSummary } from '$lib/forms';
	import { auth } from '$lib/stores/auth.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';

	onMount(async () => {
		if (auth.masterKey) await formsStore.load(auth.masterKey);
	});

	async function toggleStatus(form: FormSummary) {
		const next = form.status === 'open' ? 'closed' : 'open';
		try {
			await updateFormStatus(form.formId, next);
			formsStore.updateStatus(form.formId, next);
		} catch {
			alert('Failed to update status');
		}
	}

	async function handleDelete(form: FormSummary) {
		if (!confirm(`Delete this form and all ${form.responseCount} response(s)? This cannot be undone.`)) return;
		try {
			await deleteForm(form.formId);
			formsStore.remove(form.formId);
		} catch {
			alert('Failed to delete form');
		}
	}
</script>

<svelte:head>
	<title>Confide — Forms</title>
</svelte:head>

<div class="font-mono max-w-[960px] p-8 pb-12">
	<div class="flex items-center justify-between mb-7">
		<h1 class="text-[1.6rem] m-0 text-[#e2e8f0]">Forms</h1>
		<button
			onclick={() => goto('/forms/new')}
			class="px-4 py-2 bg-primary-hover text-white border-none rounded cursor-pointer font-mono text-[0.975rem] hover:bg-primary transition-colors duration-100"
		>
			+ New form
		</button>
	</div>

	{#if formsStore.loading}
		<p class="text-[#8899aa] text-[1.025rem]">Loading…</p>
	{:else if formsStore.error}
		<p class="text-error-light text-[1.025rem]">{formsStore.error}</p>
	{:else if formsStore.forms.length === 0}
		<div class="px-8 py-12 border border-dashed border-border rounded-lg text-center text-[#8899aa]">
			<p class="m-0 mb-2 text-[1.1rem]">No forms yet</p>
			<p class="m-0 text-[0.925rem]">Create your first form to get started</p>
		</div>
	{:else}
		<table class="w-full border-collapse text-[0.975rem]">
			<thead>
				<tr class="border-b border-border-subtle text-muted-blue">
					<th class="text-left px-3 py-2 font-normal">Title</th>
					<th class="text-left px-3 py-2 font-normal">Form ID</th>
					<th class="text-left px-3 py-2 font-normal">Status</th>
					<th class="text-right px-3 py-2 font-normal">Responses</th>
					<th class="text-left px-3 py-2 font-normal">Created</th>
					<th class="px-3 py-2"></th>
				</tr>
			</thead>
			<tbody>
				{#each formsStore.forms as form (form.formId)}
					<tr class="border-b border-border-deep">
						<td class="p-3 text-[#c5d3e0] text-[0.975rem]">
							{formsStore.formNames.get(form.formId) ?? '—'}
						</td>
						<td class="p-3 text-[#4b6280] text-[0.925rem]">
							{form.formId.slice(0, 12)}…
						</td>
						<td class="p-3">
							<span class="px-2 py-0.5 rounded-full text-[0.875rem]
								{form.status === 'open'
									? 'bg-open-bg text-open-text border border-open-border'
									: 'bg-closed-bg text-closed-text border border-closed-border'}">
								{form.status}
							</span>
						</td>
						<td class="p-3 text-right text-[#c5d3e0]">
							{form.responseCount}
						</td>
						<td class="p-3 text-muted-blue">
							{form.createdAt}
						</td>
						<td class="p-3 whitespace-nowrap">
							<div class="flex gap-2 justify-end">
								<button
									onclick={() => goto(`/forms/${form.formId}/edit`)}
									class="px-2.5 py-1 bg-transparent text-[#93c5fd] border border-border-subtle rounded cursor-pointer font-mono text-[0.875rem] hover:border-border transition-colors duration-100"
								>
									Edit
								</button>
								<button
									onclick={() => goto(`/forms/${form.formId}/responses`)}
									class="px-2.5 py-1 bg-transparent text-[#a3e635] border border-border-subtle rounded cursor-pointer font-mono text-[0.875rem] hover:border-border transition-colors duration-100"
								>
									Responses ({form.responseCount})
								</button>
								<button
									onclick={() => toggleStatus(form)}
									class="px-2.5 py-1 bg-transparent text-[#8899aa] border border-border-subtle rounded cursor-pointer font-mono text-[0.875rem] hover:border-border transition-colors duration-100"
								>
									{form.status === 'open' ? 'Close' : 'Open'}
								</button>
								<button
									onclick={() => handleDelete(form)}
									class="px-2.5 py-1 bg-transparent text-error-light border border-border-subtle rounded cursor-pointer font-mono text-[0.875rem] hover:border-[#7f1d1d] transition-colors duration-100"
								>
									Delete
								</button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>
