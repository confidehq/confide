<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { listForms, getForm, type FormSummary } from '$lib/forms';
	import { listWorkspaces, createWorkspace, deleteWorkspace, leaveWorkspace, type Workspace, WorkspaceError } from '$lib/workspaces';
	import { ArrowRight, Building2, MoreHorizontal, Trash2, LogOut, X } from '@lucide/svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let workspaces = $state<Workspace[]>([]);
	let loading = $state(true);
	let error = $state('');

	// forms and names keyed by workspace ID
	let workspaceForms = $state<Map<string, FormSummary[]>>(new Map());
	let workspaceNames = $state<Map<string, Map<string, string>>>(new Map());

	// New workspace form
	let showCreate = $state(false);
	let newName = $state('');
	let creating = $state(false);
	let createError = $state('');

	// Dropdown menu
	let openMenuId = $state<string | null>(null);

	// Delete confirm
	let deleteTarget = $state<Workspace | null>(null);
	let deleting = $state(false);
	let deleteError = $state('');

	// Leave confirm
	let leaveTarget = $state<Workspace | null>(null);
	let leaving = $state(false);
	let leaveError = $state('');

	function formsFor(wsId: string): FormSummary[] {
		return workspaceForms.get(wsId) ?? [];
	}

	function formName(wsId: string, formId: string): string {
		return workspaceNames.get(wsId)?.get(formId) ?? '—';
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

	// ─── New workspace ────────────────────────────────────────────────────────

	async function handleCreate() {
		const name = newName.trim();
		if (!name) return;
		if (!auth.masterKey) { createError = 'Session expired — please re-authenticate.'; return; }
		creating = true;
		createError = '';
		try {
			const ws = await createWorkspace(name, auth.masterKey);
			workspaces = [...workspaces, ws];
			loadWorkspaceForms(ws); // non-blocking — populates empty list immediately
			newName = '';
			showCreate = false;
		} catch (e) {
			if (e instanceof WorkspaceError && e.code === 'plan_limit') {
				createError = 'Free plan allows only one workspace. Upgrade to create more.';
			} else {
				createError = e instanceof Error ? e.message : 'Failed to create workspace.';
			}
		} finally {
			creating = false;
		}
	}

	function cancelCreate() {
		showCreate = false;
		newName = '';
		createError = '';
	}

	// ─── Delete workspace ─────────────────────────────────────────────────────

	async function handleDelete() {
		if (!deleteTarget) return;
		deleting = true;
		deleteError = '';
		try {
			await deleteWorkspace(deleteTarget.id);
			workspaces = workspaces.filter(w => w.id !== deleteTarget!.id);
			deleteTarget = null;
		} catch (e) {
			deleteError = e instanceof Error ? e.message : 'Failed to delete workspace.';
		} finally {
			deleting = false;
		}
	}

	async function handleLeave() {
		if (!leaveTarget || !auth.accountId) return;
		leaving = true;
		leaveError = '';
		try {
			await leaveWorkspace(leaveTarget.id, auth.accountId);
			workspaces = workspaces.filter(w => w.id !== leaveTarget!.id);
			leaveTarget = null;
		} catch (e) {
			leaveError = e instanceof Error ? e.message : 'Failed to leave workspace.';
		} finally {
			leaving = false;
		}
	}

	async function loadWorkspaceForms(ws: Workspace) {
		const forms = await listForms(ws.id);
		workspaceForms = new Map(workspaceForms).set(ws.id, forms);

		if (auth.masterKey && forms.length > 0) {
			const results = await Promise.allSettled(
				forms.map(f => getForm(auth.masterKey!, f.formId))
			);
			const names = new Map<string, string>();
			results.forEach((r, i) => {
				if (r.status === 'fulfilled') {
					const { schema } = r.value;
					const name = schema.translations[schema.defaultLocale]?.formTitle;
					if (name) names.set(forms[i].formId, name);
				}
			});
			workspaceNames = new Map(workspaceNames).set(ws.id, names);
		}
	}

	onMount(async () => {
		try {
			workspaces = await listWorkspaces();
			await Promise.all(workspaces.map(loadWorkspaceForms));
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load workspaces';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Confide — Workspaces</title>
</svelte:head>

<ConfirmDialog
	open={!!deleteTarget}
	title="Delete workspace?"
	description={deleteTarget ? `This will permanently delete "${deleteTarget.name}" and all its forms. This cannot be undone.` : ''}
	loading={deleting}
	error={deleteError}
	onconfirm={handleDelete}
	oncancel={() => { deleteTarget = null; deleteError = ''; }}
/>

<ConfirmDialog
	open={!!leaveTarget}
	title="Leave workspace?"
	description={leaveTarget ? `You will lose access to "${leaveTarget.name}" and all its forms.` : ''}
	loading={leaving}
	error={leaveError}
	confirmLabel="Leave"
	loadingLabel="Leaving…"
	onconfirm={handleLeave}
	oncancel={() => { leaveTarget = null; leaveError = ''; }}
/>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Page header -->
	<div class="flex items-start justify-between mb-8 gap-4">
		<h1 class="text-2xl m-0 text-text font-semibold">All Workspaces</h1>
		{#if !showCreate}
			<button
				onclick={() => { showCreate = true; }}
				class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
			>+ New workspace</button>
		{/if}
	</div>

	<!-- Inline create form -->
	{#if showCreate}
		<div class="mb-8 p-5 border border-border-canvas rounded-lg flex flex-col gap-4">
			<div class="flex items-center justify-between gap-2">
				<span class="text-base text-text font-semibold">New workspace</span>
				<button
					onclick={cancelCreate}
					class="bg-transparent border-none cursor-pointer text-subtle hover:text-text transition-colors duration-100 p-1 rounded"
					aria-label="Cancel"
				><X size={16} strokeWidth={1.75} /></button>
			</div>

			<input
				type="text"
				placeholder="Workspace name"
				bind:value={newName}
				disabled={creating}
				onkeydown={e => { if (e.key === 'Enter') handleCreate(); if (e.key === 'Escape') cancelCreate(); }}
				class="input-base w-full text-base px-3 py-2.5"
				autofocus
			/>

			{#if createError}
				<p class="m-0 text-base text-danger-light">{createError}</p>
			{/if}

			<div class="flex gap-2">
				<button
					onclick={handleCreate}
					disabled={creating || !newName.trim()}
					class="px-4 py-2 text-white border-none rounded cursor-pointer font-mono text-base transition-colors duration-100
						{creating || !newName.trim() ? 'bg-muted cursor-not-allowed' : 'bg-primary hover:bg-primary-hover'}"
				>{creating ? 'Creating…' : 'Create workspace'}</button>
				<button
					onclick={cancelCreate}
					class="px-4 py-2 bg-transparent text-subtle border border-border-canvas rounded cursor-pointer font-mono text-base hover:text-text hover:border-border-canvas transition-colors duration-100"
				>Cancel</button>
			</div>
		</div>
	{/if}

	{#if loading}
		<p class="text-subtle text-base">Loading…</p>
	{:else if error}
		<p class="text-danger text-base">{error}</p>
	{:else if workspaces.length === 0}
		<div class="py-12 border border-dashed border-border-canvas rounded-lg text-center">
			<p class="m-0 text-subtle text-base">No workspaces found</p>
		</div>
	{:else}
		<div class="flex flex-col gap-8">
			{#each workspaces as ws (ws.id)}
				{@const forms = formsFor(ws.id)}
				{@const formsLoading = !workspaceForms.has(ws.id)}

				<div>
					<!-- Workspace header -->
					<div class="flex items-center gap-3 mb-4">
						<Building2 size={18} strokeWidth={1.75} class="shrink-0 text-subtle" />
						<h2 class="m-0 text-xl font-semibold text-text truncate min-w-0">{ws.name}</h2>

						<!-- Plan badge -->
						<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base border
							{ws.plan === 'pro'
								? ws.planStatus === 'active' || ws.planStatus === 'canceling'
									? 'bg-open-bg text-open-text border-open-border'
									: 'bg-closed-bg text-closed-text border-closed-border'
								: 'text-subtle border-border-canvas bg-transparent'}">
							{planLabel(ws)}
						</span>

						<!-- Role badge -->
						<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base text-subtle border border-border-canvas">
							{ws.role}
						</span>

						<div class="ml-auto flex items-center gap-2 shrink-0">
							<!-- New form button -->
							<button
								onclick={() => goto(`/forms/new?workspaceId=${ws.id}`)}
								class="px-3 py-1.5 bg-transparent text-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
							>+ New form</button>

							<!-- Dropdown menu -->
							<div class="relative">
								{#if openMenuId === ws.id}
									<div
										class="fixed inset-0 z-10"
										onclick={() => (openMenuId = null)}
										role="presentation"
									></div>
								{/if}
								<button
									onclick={() => (openMenuId = openMenuId === ws.id ? null : ws.id)}
									class="flex items-center justify-center w-8 h-8 bg-transparent border rounded cursor-pointer text-subtle transition-colors duration-100
										{openMenuId === ws.id
											? 'text-text border-border-canvas bg-canvas'
											: 'border-border-canvas hover:text-text hover:border-border-canvas'}"
									aria-label="Workspace options"
									aria-expanded={openMenuId === ws.id}
								><MoreHorizontal size={16} strokeWidth={1.75} /></button>

								{#if openMenuId === ws.id}
									<div class="absolute right-0 top-[calc(100%+5px)] z-20 min-w-[180px] bg-base border border-surface rounded-lg shadow-[0_8px_24px_var(--color-overlay-light)] overflow-hidden py-1">
										{#if ws.role === 'owner'}
											<button
												onclick={() => { openMenuId = null; deleteTarget = ws; deleteError = ''; }}
												class="flex items-center gap-2.5 w-full px-3.5 py-2.5 bg-transparent border-none cursor-pointer font-mono text-sm text-danger text-left transition-colors duration-100 hover:bg-danger-bg-dark"
											>
												<Trash2 size={13} strokeWidth={1.75} />
												Delete workspace…
											</button>
										{:else}
											<button
												onclick={() => { openMenuId = null; leaveTarget = ws; leaveError = ''; }}
												class="flex items-center gap-2.5 w-full px-3.5 py-2.5 bg-transparent border-none cursor-pointer font-mono text-sm text-danger text-left transition-colors duration-100 hover:bg-danger-bg-dark"
											>
												<LogOut size={13} strokeWidth={1.75} />
												Leave workspace…
											</button>
										{/if}
									</div>
								{/if}
							</div>
						</div>
					</div>

					<!-- Forms list -->
					{#if formsLoading}
						<div class="py-5 text-subtle text-base">Loading forms…</div>
					{:else if forms.length === 0}
						<div class="py-8 border border-dashed border-border-canvas rounded-lg text-center">
							<p class="m-0 text-subtle text-base">No forms in this workspace</p>
						</div>
					{:else}
						<div class="border border-border-canvas rounded-lg overflow-hidden">
							{#each forms as form, i (form.formId)}
								<div
									class="flex items-center gap-3 px-4 py-3.5 cursor-pointer hover:bg-border-card transition-colors duration-100
										{i < forms.length - 1 ? 'border-b border-border-canvas' : ''}"
									onclick={() => goto(`/forms/${form.formId}`)}
									role="button"
									tabindex="0"
									onkeydown={e => e.key === 'Enter' && goto(`/forms/${form.formId}`)}
								>
									<span class="shrink-0 w-2 h-2 rounded-full
										{form.status === 'open' ? 'bg-success' : 'bg-muted'}">
									</span>

									<span class="flex-1 min-w-0 text-base text-text truncate">
										{formName(ws.id, form.formId)}
									</span>

									<span class="shrink-0 text-base text-subtle tabular-nums hidden sm:block">
										{form.responseCount} {form.responseCount === 1 ? 'response' : 'responses'}
									</span>

									<StatusBadge status={form.status} class="hidden sm:inline" />

									<span class="shrink-0 text-base text-subtle tabular-nums sm:hidden">
										{form.responseCount}r
									</span>

									<ArrowRight size={16} strokeWidth={1.5} class="shrink-0 text-subtle" />
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}

</div>
</div>
