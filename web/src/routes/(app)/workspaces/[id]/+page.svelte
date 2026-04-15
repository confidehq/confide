<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { listForms, getForm, setWorkspaceFormKey, updateFormStatus, deleteForm, type FormSummary } from '$lib/forms';
	import { listWorkspaces, loadWorkspaceKey, type Workspace } from '$lib/workspaces';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	const workspaceId = $derived($page.params.id);

	let workspace = $state<Workspace | null>(null);
	let forms = $state<FormSummary[]>([]);
	let formNames = $state<Map<string, string>>(new Map());
	let loading = $state(true);
	let error = $state('');

	let pendingDelete = $state<FormSummary | null>(null);
	let deleteLoading = $state(false);
	let deleteError = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			const [allWorkspaces, rawForms] = await Promise.all([
				listWorkspaces(),
				listForms(workspaceId)
			]);
			workspace = allWorkspaces.find(w => w.id === workspaceId) ?? null;
			forms = rawForms;

			// Decrypt names in background (best-effort)
			if (auth.masterKey && rawForms.length > 0) {
				// Try to load the workspace key (non-owners may have it via key grant)
				let wsKey: CryptoKey | undefined;
				try { wsKey = await loadWorkspaceKey(workspaceId as string, auth.masterKey); } catch { /* not granted yet */ }

				const results = await Promise.allSettled(
					rawForms.map(f => getForm(auth.masterKey!, f.formId, wsKey))
				);
				const names = new Map<string, string>();
				results.forEach((r, i) => {
					if (r.status === 'fulfilled') {
						const { schema } = r.value;
						const name = schema.name || schema.translations[schema.defaultLocale]?.formTitle;
						if (name) names.set(rawForms[i].formId, name);
					}
				});
				formNames = names;

				// Lazy migration: set workspaceWrappedFormKey for forms that don't have it yet
				// Only the form creator can do this (deriveFormKey only works with creator's masterKey)
				if (wsKey) {
					for (let i = 0; i < rawForms.length; i++) {
						const result = results[i];
						const rawForm = rawForms[i];
						if (result.status === 'fulfilled' && !result.value.record.workspaceWrappedFormKey) {
							setWorkspaceFormKey(auth.masterKey!, rawForm.formId, wsKey).catch(() => {});
						}
					}
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load workspace';
		} finally {
			loading = false;
		}
	}

	async function toggleStatus(form: FormSummary) {
		const next = form.status === 'open' ? 'closed' : 'open';
		try {
			await updateFormStatus(form.formId, next);
			forms = forms.map(f => f.formId === form.formId ? { ...f, status: next } : f);
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
			forms = forms.filter(f => f.formId !== pendingDelete!.formId);
			const names = new Map(formNames);
			names.delete(pendingDelete.formId);
			formNames = names;
			pendingDelete = null;
		} catch {
			deleteError = 'Failed to delete form. Please try again.';
		} finally {
			deleteLoading = false;
		}
	}

	function formName(formId: string): string {
		return formNames.get(formId) ?? '—';
	}

	function planLabel(ws: Workspace): string {
		if (ws.plan === 'pro') {
			if (ws.planStatus === 'past_due') return 'Pro · past due';
			if (ws.planStatus === 'canceled') return 'Pro · canceled';
			return 'Pro';
		}
		return 'Free';
	}

	$effect(() => {
		if (auth.masterKey) {
			workspaceId;
			load();
		}
	});
</script>

<svelte:head>
	<title>Confide — {workspace?.name ?? 'Workspace'}</title>
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

	{#if loading}
		<p class="text-muted-dim text-base">Loading…</p>
	{:else if error}
		<p class="text-error-light text-base">{error}</p>
	{:else}
		<!-- Header -->
		<div class="flex items-center justify-between mb-8 gap-4">
			<div class="flex items-center gap-3 min-w-0">
				<h1 class="text-2xl m-0 text-text-bright font-semibold truncate">
					{workspace?.name ?? workspaceId}
				</h1>
				{#if workspace}
					<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border
						{workspace.plan === 'pro'
							? workspace.planStatus === 'active'
								? 'bg-open-bg text-open-text border-open-border'
								: 'bg-closed-bg text-closed-text border-closed-border'
							: 'text-muted-dim border-border-deep bg-transparent'}">
						{planLabel(workspace)}
					</span>
					<span class="shrink-0 hidden sm:inline px-2.5 py-0.5 rounded-full text-base text-muted-mid border border-border-deep">
						{workspace.role}
					</span>
				{/if}
			</div>
			<button
				onclick={() => goto(`/forms/new?workspaceId=${workspaceId}`)}
				class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
			>+ New form</button>
		</div>

		<!-- Forms -->
		{#if forms.length === 0}
			<div class="py-12 border border-dashed border-border rounded-lg text-center">
				<p class="m-0 mb-1 text-muted-dim text-base">No forms yet</p>
				<p class="m-0 text-muted-mid text-base">Create your first form to get started</p>
				<button
					onclick={() => goto(`/forms/new?workspaceId=${workspaceId}`)}
					class="mt-4 px-4 py-2 bg-transparent text-text-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
				>+ New form</button>
			</div>
		{:else}
			<!-- Mobile card list -->
			<div class="flex flex-col gap-2 sm:hidden">
				{#each forms as form (form.formId)}
					<div class="p-4 border border-border-deep rounded-lg">
						<div class="flex items-center justify-between gap-2 mb-2">
							<span class="text-text-body text-base truncate">{formName(form.formId)}</span>
							<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base
								{form.status === 'open'
									? 'bg-open-bg text-open-text border border-open-border'
									: 'bg-closed-bg text-closed-text border border-closed-border'}">
								{form.status}
							</span>
						</div>
						<p class="m-0 mb-3 text-muted-dim text-base">
							{form.responseCount} response{form.responseCount === 1 ? '' : 's'} · {form.createdAt}
						</p>
						<div class="flex gap-2 flex-wrap">
							<button
								onclick={() => goto(`/forms/${form.formId}/edit`)}
								class="px-3 py-1.5 bg-transparent text-text-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
							>Edit</button>
							<button
								onclick={() => goto(`/forms/${form.formId}/responses`)}
								class="px-3 py-1.5 bg-transparent text-[#a3e635] border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
							>Responses</button>
							<button
								onclick={() => toggleStatus(form)}
								class="px-3 py-1.5 bg-transparent text-muted-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
							>{form.status === 'open' ? 'Close' : 'Open'}</button>
							<button
								onclick={() => handleDelete(form)}
								class="px-3 py-1.5 bg-transparent text-error-light border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border-danger-dark transition-colors duration-100"
							>Delete</button>
						</div>
					</div>
				{/each}
			</div>

			<!-- Desktop table -->
			<table class="hidden sm:table w-full border-collapse text-base">
				<thead>
					<tr class="border-b border-border-subtle text-muted-dim">
						<th class="text-left px-3 py-2.5 font-normal">Title</th>
						<th class="text-left px-3 py-2.5 font-normal">Form ID</th>
						<th class="text-left px-3 py-2.5 font-normal">Status</th>
						<th class="text-right px-3 py-2.5 font-normal">Responses</th>
						<th class="text-left px-3 py-2.5 font-normal">Created</th>
						<th class="px-3 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each forms as form (form.formId)}
						<tr class="border-b border-border-deep">
							<td class="p-3 text-text-body text-base">{formName(form.formId)}</td>
							<td class="p-3 text-muted-dim text-base">{form.formId.slice(0, 12)}…</td>
							<td class="p-3">
								<span class="px-2.5 py-0.5 rounded-full text-base
									{form.status === 'open'
										? 'bg-open-bg text-open-text border border-open-border'
										: 'bg-closed-bg text-closed-text border border-closed-border'}">
									{form.status}
								</span>
							</td>
							<td class="p-3 text-right text-text-body text-base tabular-nums">{form.responseCount}</td>
							<td class="p-3 text-muted-dim text-base">{form.createdAt}</td>
							<td class="p-3 whitespace-nowrap">
								<div class="flex gap-2 justify-end">
									<button
										onclick={() => goto(`/forms/${form.formId}/edit`)}
										class="px-3 py-1.5 bg-transparent text-text-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
									>Edit</button>
									<button
										onclick={() => goto(`/forms/${form.formId}/responses`)}
										class="px-3 py-1.5 bg-transparent text-[#a3e635] border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
									>Responses ({form.responseCount})</button>
									<button
										onclick={() => toggleStatus(form)}
										class="px-3 py-1.5 bg-transparent text-muted-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
									>{form.status === 'open' ? 'Close' : 'Open'}</button>
									<button
										onclick={() => handleDelete(form)}
										class="px-3 py-1.5 bg-transparent text-error-light border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border-danger-dark transition-colors duration-100"
									>Delete</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{/if}

</div>
</div>
