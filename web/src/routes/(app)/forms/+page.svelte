<script lang="ts">
	import { goto } from '$app/navigation';
	import { updateFormStatus, deleteForm, type FormSummary } from '$lib/forms';
	import { auth } from '$lib/stores/auth.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import DropdownMenu from '$lib/components/DropdownMenu.svelte';
	import DropdownMenuItem from '$lib/components/DropdownMenuItem.svelte';
	import DropdownMenuSeparator from '$lib/components/DropdownMenuSeparator.svelte';

	$effect(() => {
		const workspace = workspacesStore.active;
		const masterKey = auth.masterKey;
		if (masterKey && workspace) formsStore.load(masterKey, workspace.id);
	});

	let pendingDelete = $state<FormSummary | null>(null);
	let deleteLoading = $state(false);
	let deleteError = $state('');

	async function toggleStatus(form: FormSummary) {
		const next = form.status === 'open' ? 'closed' : 'open';
		try {
			await updateFormStatus(form.formId, next);
			formsStore.updateStatus(form.formId, next);
		} catch {
			alert('Failed to update status');
		}
	}

	function handleDelete(form: FormSummary) {
		pendingDelete = form;
		deleteError = '';
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		deleteLoading = true;
		deleteError = '';
		try {
			await deleteForm(pendingDelete.formId);
			formsStore.remove(pendingDelete.formId);
			pendingDelete = null;
		} catch {
			deleteError = 'Failed to delete form. Please try again.';
		} finally {
			deleteLoading = false;
		}
	}
</script>

<svelte:head>
	<title>Confide — Forms</title>
</svelte:head>

<ConfirmDialog
	open={!!pendingDelete}
	title="Delete form?"
	description={pendingDelete
		? `This will permanently delete the form and all ${pendingDelete.responseCount} response${pendingDelete.responseCount === 1 ? '' : 's'}. This cannot be undone.`
		: ''}
	loading={deleteLoading}
	error={deleteError}
	onconfirm={confirmDelete}
	oncancel={() => { pendingDelete = null; deleteError = ''; }}
/>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<div class="flex items-start justify-between mb-8 gap-4">
		<div class="min-w-0">
			<h1 class="text-2xl m-0 mb-1 text-[#e2e8f0] font-semibold">Forms</h1>
			{#if workspacesStore.active}
				<p class="m-0 text-sm text-[#4b6280]">{workspacesStore.active.name}</p>
			{/if}
		</div>
		<button
			onclick={() => {
				const ws = workspacesStore.active;
				goto(ws ? `/forms/new?workspaceId=${ws.id}` : '/forms/new');
			}}
			class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
		>+ New form</button>
	</div>

	{#if formsStore.loading && !formsStore.loaded}
		<p class="text-[#4b6280] text-base">Loading…</p>
	{:else if formsStore.error}
		<p class="text-error-light text-base">{formsStore.error}</p>
	{:else if formsStore.forms.length === 0}
		<div class="py-12 border border-dashed border-border rounded-lg text-center">
			<p class="m-0 mb-1 text-[#4b6280] text-base">No forms yet</p>
			<p class="m-0 text-[#374d63] text-base">Create your first form to get started</p>
			<button
				onclick={() => goto('/forms/new')}
				class="mt-4 px-4 py-2 bg-transparent text-[#93c5fd] border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
			>+ New form</button>
		</div>
	{:else}
		<!-- Mobile card list -->
		<div class="flex flex-col gap-2 sm:hidden">
			{#each formsStore.forms as form (form.formId)}
				<div class="p-4 border border-border-deep rounded-lg hover:bg-[#0d1926] transition-colors duration-75">
					<div class="flex items-center justify-between gap-2 mb-2">
						<button
						onclick={() => goto(`/forms/${form.formId}`)}
						class="text-[#c5d3e0] text-base truncate bg-transparent border-none cursor-pointer font-mono p-0 text-left hover:text-white hover:underline transition-colors duration-75"
					>
						{formsStore.formNames.get(form.formId) ?? '—'}
					</button>
						<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base
							{form.status === 'open'
								? 'bg-open-bg text-open-text border border-open-border'
								: 'bg-closed-bg text-closed-text border border-closed-border'}">
							{form.status}
						</span>
					</div>
					<p class="m-0 mb-3 text-[#4b6280] text-base">{form.responseCount} response{form.responseCount === 1 ? '' : 's'} · {form.createdAt}</p>
					<div class="flex justify-end">
						<DropdownMenu>
							{#snippet trigger(attrs)}
								<button
									{...attrs}
									class="px-2 py-1 bg-transparent text-[#4b6280] border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border hover:text-[#c5d3e0] transition-colors duration-100"
								>···</button>
							{/snippet}
							{#snippet children({ close })}
								<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/edit`); }}>Edit</DropdownMenuItem>
								<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/responses`); }}>Responses</DropdownMenuItem>
								<DropdownMenuItem onclick={() => { close(); toggleStatus(form); }}>{form.status === 'open' ? 'Close' : 'Open'}</DropdownMenuItem>
								<DropdownMenuSeparator />
								<DropdownMenuItem variant="destructive" onclick={() => { close(); handleDelete(form); }}>Delete</DropdownMenuItem>
							{/snippet}
						</DropdownMenu>
					</div>
				</div>
			{/each}
		</div>

		<!-- Desktop table -->
		<table class="hidden sm:table w-full border-collapse text-base">
			<thead>
				<tr class="border-b border-border-subtle text-[#4b6280]">
					<th class="text-left px-3 py-2.5 font-normal">Title</th>
					<th class="text-left px-3 py-2.5 font-normal">Form ID</th>
					<th class="text-left px-3 py-2.5 font-normal">Status</th>
					<th class="text-right px-3 py-2.5 font-normal">Responses</th>
					<th class="text-left px-3 py-2.5 font-normal">Created</th>
					<th class="px-3 py-2.5"></th>
				</tr>
			</thead>
			<tbody>
				{#each formsStore.forms as form (form.formId)}
					<tr class="border-b border-border-deep hover:bg-[#0d1926] transition-colors duration-75">
						<td class="p-3">
							<button
								onclick={() => goto(`/forms/${form.formId}`)}
								class="text-[#c5d3e0] text-base bg-transparent border-none cursor-pointer font-mono p-0 text-left hover:text-white hover:underline transition-colors duration-75"
							>
								{formsStore.formNames.get(form.formId) ?? '—'}
							</button>
						</td>
						<td class="p-3 text-[#4b6280] text-base">
							{form.formId.slice(0, 12)}…
						</td>
						<td class="p-3">
							<span class="px-2.5 py-0.5 rounded-full text-base
								{form.status === 'open'
									? 'bg-open-bg text-open-text border border-open-border'
									: 'bg-closed-bg text-closed-text border border-closed-border'}">
								{form.status}
							</span>
						</td>
						<td class="p-3 text-right text-[#c5d3e0] text-base tabular-nums">
							{form.responseCount}
						</td>
						<td class="p-3 text-[#4b6280] text-base">
							{form.createdAt}
						</td>
						<td class="p-3">
							<div class="flex justify-end">
								<DropdownMenu>
									{#snippet trigger(attrs)}
										<button
											{...attrs}
											class="px-2 py-1 bg-transparent text-[#4b6280] border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border hover:text-[#c5d3e0] transition-colors duration-100"
										>···</button>
									{/snippet}
									{#snippet children({ close })}
										<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/edit`); }}>Edit</DropdownMenuItem>
										<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/responses`); }}>Responses</DropdownMenuItem>
										<DropdownMenuItem onclick={() => { close(); toggleStatus(form); }}>{form.status === 'open' ? 'Close' : 'Open'}</DropdownMenuItem>
										<DropdownMenuSeparator />
										<DropdownMenuItem variant="destructive" onclick={() => { close(); handleDelete(form); }}>Delete</DropdownMenuItem>
									{/snippet}
								</DropdownMenu>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

</div>
</div>
