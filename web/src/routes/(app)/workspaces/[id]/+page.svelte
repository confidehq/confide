<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { listForms, getForm, setWorkspaceFormKey, updateFormStatus, deleteForm, type FormSummary } from '$lib/forms';
	import { listWorkspaces, loadWorkspaceKey, getWorkspaceSettings, updateWorkspaceSettings, type Workspace } from '$lib/workspaces';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	const workspaceId = $derived($page.params.id);

	let workspace = $state<Workspace | null>(null);
	let forms = $state<FormSummary[]>([]);
	let formNames = $state<Map<string, string>>(new Map());
	let loading = $state(true);
	let error = $state('');

	let pendingDelete = $state<FormSummary | null>(null);
	let deleteLoading = $state(false);
	let deleteError = $state('');

	let legalText = $state('');
	let legalTextSaving = $state(false);
	let legalTextError = $state('');
	let legalTextSaved = $state(false);
	let legalTextTimer: ReturnType<typeof setTimeout> | null = null;


async function load() {
		loading = true;
		error = '';
		try {
			const [allWorkspaces, rawForms, settings] = await Promise.all([
				listWorkspaces(),
				listForms(workspaceId),
				getWorkspaceSettings(workspaceId).catch(() => ({ legalText: '' }))
			]);
			workspace = allWorkspaces.find(w => w.id === workspaceId) ?? null;
			legalText = settings.legalText;
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
						const name = schema.translations[schema.defaultLocale]?.formTitle;
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

	async function saveLegalText() {
		legalTextSaving = true;
		legalTextError = '';
		try {
			await updateWorkspaceSettings(workspaceId, { legalText });
			legalTextSaved = true;
			if (legalTextTimer) clearTimeout(legalTextTimer);
			legalTextTimer = setTimeout(() => { legalTextSaved = false; }, 2000);
		} catch {
			legalTextError = 'Failed to save — please try again.';
		} finally {
			legalTextSaving = false;
		}
	}

	function planLabel(ws: Workspace): string {
		if (ws.plan === 'pro') {
			if (ws.planStatus === 'past_due') return 'Pro · past due';
			if (ws.planStatus === 'canceled') return 'Pro · canceled';
			if (ws.planStatus === 'canceling') return 'Pro · cancels at period end';
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
		<p class="text-subtle text-base">Loading…</p>
	{:else if error}
		<p class="text-error-light text-base">{error}</p>
	{:else}
		<!-- Header -->
		<div class="flex items-center justify-between mb-8 gap-4">
			<div class="flex items-center gap-3 min-w-0">
				<h1 class="text-2xl m-0 text-text font-semibold truncate">
					{workspace?.name ?? workspaceId}
				</h1>
				{#if workspace}
					<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border
						{workspace.plan === 'pro'
							? workspace.planStatus === 'active'
								? 'bg-open-bg text-open-text border-open-border'
								: workspace.planStatus === 'canceling'
									? 'bg-open-bg text-open-text border-open-border'
									: 'bg-closed-bg text-closed-text border-closed-border'
							: 'text-subtle border-border-canvas bg-transparent'}">
						{planLabel(workspace)}
					</span>
					<span class="shrink-0 hidden sm:inline px-2.5 py-0.5 rounded-full text-base text-subtle border border-border-canvas">
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
			<div class="py-12 border border-dashed border-border-canvas rounded-lg text-center">
				<p class="m-0 mb-1 text-subtle text-base">No forms yet</p>
				<p class="m-0 text-subtle text-base">Create your first form to get started</p>
				<button
					onclick={() => goto(`/forms/new?workspaceId=${workspaceId}`)}
					class="mt-4 px-4 py-2 bg-transparent text-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
				>+ New form</button>
			</div>
		{:else}
			<!-- Mobile card list -->
			<div class="flex flex-col gap-2 sm:hidden">
				{#each forms as form (form.formId)}
					<div class="p-4 border border-border-canvas rounded-lg">
						<div class="flex items-center justify-between gap-2 mb-2">
							<span class="text-text text-base truncate">{formName(form.formId)}</span>
							<StatusBadge status={form.status} />
						</div>
						<p class="m-0 mb-3 text-subtle text-base">
							{form.responseCount} response{form.responseCount === 1 ? '' : 's'} · {form.createdAt}
						</p>
						<div class="flex gap-2 flex-wrap">
							<button
								onclick={() => goto(`/forms/${form.formId}/edit`)}
								class="px-3 py-1.5 bg-transparent text-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
							>Edit</button>
							<button
								onclick={() => goto(`/forms/${form.formId}/responses`)}
								class="px-3 py-1.5 bg-transparent text-open-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
							>Responses</button>
							<button
								onclick={() => toggleStatus(form)}
								class="px-3 py-1.5 bg-transparent text-subtle border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
							>{form.status === 'open' ? 'Close' : 'Open'}</button>
							<button
								onclick={() => handleDelete(form)}
								class="px-3 py-1.5 bg-transparent text-error-light border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas-danger-dark transition-colors duration-100"
							>Delete</button>
						</div>
					</div>
				{/each}
			</div>

			<!-- Desktop table -->
			<table class="hidden sm:table w-full border-collapse text-base">
				<thead>
					<tr class="border-b border-border-canvas text-subtle">
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
						<tr class="border-b border-border-canvas">
							<td class="p-3 text-text text-base">{formName(form.formId)}</td>
							<td class="p-3 text-subtle text-base">{form.formId.slice(0, 12)}…</td>
							<td class="p-3">
								<StatusBadge status={form.status} />
							</td>
							<td class="p-3 text-right text-text text-base tabular-nums">{form.responseCount}</td>
							<td class="p-3 text-subtle text-base">{form.createdAt}</td>
							<td class="p-3 whitespace-nowrap">
								<div class="flex gap-2 justify-end">
									<button
										onclick={() => goto(`/forms/${form.formId}/edit`)}
										class="px-3 py-1.5 bg-transparent text-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
									>Edit</button>
									<button
										onclick={() => goto(`/forms/${form.formId}/responses`)}
										class="px-3 py-1.5 bg-transparent text-open-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
									>Responses ({form.responseCount})</button>
									<button
										onclick={() => toggleStatus(form)}
										class="px-3 py-1.5 bg-transparent text-subtle border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
									>{form.status === 'open' ? 'Close' : 'Open'}</button>
									<button
										onclick={() => handleDelete(form)}
										class="px-3 py-1.5 bg-transparent text-error-light border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas-danger-dark transition-colors duration-100"
									>Delete</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}

	{/if}

	<!-- Workspace settings -->
	{#if workspace && (workspace.role === 'owner' || workspace.role === 'admin')}
		<div class="mt-12 pt-8 border-t border-border-canvas">
			<h2 class="text-lg m-0 mb-6 text-text font-semibold">Settings</h2>

			<div class="max-w-xl">
				<div class="mb-1">
					<label class="block text-base text-text mb-1" for="ws-legal-text">Default legal text / Impressum</label>
					<p class="m-0 mb-2 text-base text-subtle">Shown as a footer on all forms in this workspace. Individual forms can override this.</p>
				</div>
				<textarea
					id="ws-legal-text"
					rows={4}
					value={legalText}
					oninput={(e) => { legalText = (e.target as HTMLTextAreaElement).value; legalTextSaved = false; }}
					placeholder="e.g. © 2025 Acme Inc. · Privacy Policy · Impressum"
					class="block w-full px-3 py-2 bg-canvas border border-border-canvas rounded-md font-mono text-base text-text outline-none resize-none focus:border-border-canvas transition-colors"
				></textarea>
				<div class="flex items-center gap-3 mt-2">
					<button
						onclick={saveLegalText}
						disabled={legalTextSaving}
						class="px-4 py-1.5 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100 disabled:opacity-50 disabled:cursor-not-allowed"
					>{legalTextSaving ? 'Saving…' : legalTextSaved ? 'Saved' : 'Save'}</button>
					{#if legalTextError}
						<p class="m-0 text-base text-error-light">{legalTextError}</p>
					{/if}
				</div>
			</div>
		</div>
	{/if}

</div>
</div>
