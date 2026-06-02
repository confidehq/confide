<script lang="ts">
import { onMount } from "svelte";
import { goto } from "$app/navigation";
import { page } from "$app/stores";
import {
	createForm,
	deleteForm,
	type FormSummary,
	getForm,
	updateFormSchema,
	updateFormStatus,
} from "$lib/forms";
import { auth } from "$lib/stores/auth.svelte";
import { formsStore } from "$lib/stores/forms.svelte";
import { workspacesStore } from "$lib/stores/workspaces.svelte";
import { deleteWorkspace, loadWorkspaceKey } from "$lib/workspaces";

// Clean up a workspace that was created for Pro checkout but then cancelled.
onMount(async () => {
	const cancelWsId = $page.url.searchParams.get("cancel_ws");
	if (!cancelWsId) return;
	// Remove the param from the URL immediately so a refresh doesn't re-trigger.
	goto("/forms", { replaceState: true });
	try {
		await deleteWorkspace(cancelWsId);
		workspacesStore.remove(cancelWsId);
	} catch {
		/* already gone or inaccessible */
	}
});

import { Check, Ellipsis, Link, Pencil, Users } from "@lucide/svelte";
import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
import DropdownMenu from "$lib/components/DropdownMenu.svelte";
import DropdownMenuItem from "$lib/components/DropdownMenuItem.svelte";
import DropdownMenuSeparator from "$lib/components/DropdownMenuSeparator.svelte";
import StatusBadge from "$lib/components/StatusBadge.svelte";
import WorkspaceHeader from "$lib/components/WorkspaceHeader.svelte";

function timeAgo(dateStr: string): string {
	const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
	if (seconds < 60) return "just now";
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return minutes === 1 ? "1 min ago" : `${minutes} mins ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24)
		return hours === 1 ? "about an hour ago" : `${hours} hours ago`;
	const days = Math.floor(hours / 24);
	if (days < 7) return days === 1 ? "yesterday" : `${days} days ago`;
	const weeks = Math.floor(days / 7);
	if (weeks < 4) return weeks === 1 ? "a week ago" : `${weeks} weeks ago`;
	const months = Math.floor(days / 30);
	if (months < 12) return months === 1 ? "a month ago" : `${months} months ago`;
	const years = Math.floor(days / 365);
	return years === 1 ? "a year ago" : `${years} years ago`;
}

$effect(() => {
	const workspace = workspacesStore.active;
	const masterKey = auth.masterKey;
	if (masterKey && workspace) formsStore.load(masterKey, workspace.id);
});

$effect(() => {
	const workspace = workspacesStore.active;
	const masterKey = auth.masterKey;
	function handleVisibility() {
		if (!document.hidden && masterKey && workspace) {
			formsStore.load(masterKey, workspace.id);
		}
	}
	document.addEventListener("visibilitychange", handleVisibility);
	return () => document.removeEventListener("visibilitychange", handleVisibility);
});

let pendingDelete = $state<FormSummary | null>(null);
let deleteLoading = $state(false);
let deleteError = $state("");
let copiedId = $state<string | null>(null);
let duplicatingId = $state<string | null>(null);
let renamingId = $state<string | null>(null);
let renameValue = $state("");
let renameSaving = $state(false);
let renameError = $state("");
let renameInputEl = $state<HTMLInputElement | null>(null);

$effect(() => {
	if (renamingId && renameInputEl) renameInputEl.focus();
});

async function copyLink(e: MouseEvent, formId: string) {
	e.stopPropagation();
	const url = formsStore.shareUrls.get(formId);
	if (!url) return;
	await navigator.clipboard.writeText(url);
	copiedId = formId;
	setTimeout(() => {
		copiedId = null;
	}, 2000);
}

async function toggleStatus(form: FormSummary) {
	const next = form.status === "open" ? "closed" : "open";
	try {
		await updateFormStatus(form.formId, next);
		formsStore.updateStatus(form.formId, next);
	} catch {
		alert("Failed to update status");
	}
}

async function handleDuplicate(form: FormSummary) {
	const masterKey = auth.masterKey;
	const ws = workspacesStore.active;
	if (!masterKey || !ws) return;
	duplicatingId = form.formId;
	try {
		const wsKey = await loadWorkspaceKey(ws.id, masterKey);
		const { schema } = await getForm(masterKey, form.formId, wsKey);
		await createForm(masterKey, schema, ws.id, wsKey);
		formsStore.invalidate();
		await formsStore.load(masterKey, ws.id);
	} catch {
		alert("Failed to duplicate form. Please try again.");
	} finally {
		duplicatingId = null;
	}
}

function handleRename(form: FormSummary) {
	renameValue = formsStore.formNames.get(form.formId) ?? "";
	renameError = "";
	renamingId = form.formId;
}

async function saveRename() {
	const masterKey = auth.masterKey;
	const ws = workspacesStore.active;
	if (!masterKey || !ws || !renamingId) return;
	const trimmed = renameValue.trim();
	if (!trimmed) return;
	renameSaving = true;
	renameError = "";
	try {
		const wsKey = await loadWorkspaceKey(ws.id, masterKey);
		const { schema, formKey } = await getForm(masterKey, renamingId, wsKey);
		const updated = {
			...schema,
			translations: {
				...schema.translations,
				[schema.defaultLocale]: {
					...schema.translations[schema.defaultLocale],
					formTitle: trimmed,
				},
			},
		};
		await updateFormSchema(masterKey, renamingId, updated, formKey);
		formsStore.updateName(renamingId, trimmed);
		renamingId = null;
	} catch (e) {
		renameError = e instanceof Error ? e.message : "Failed to rename";
	} finally {
		renameSaving = false;
	}
}

function handleDelete(form: FormSummary) {
	pendingDelete = form;
	deleteError = "";
}

async function confirmDelete() {
	if (!pendingDelete) return;
	deleteLoading = true;
	deleteError = "";
	try {
		await deleteForm(pendingDelete.formId);
		formsStore.remove(pendingDelete.formId);
		pendingDelete = null;
	} catch {
		deleteError = "Failed to delete form. Please try again.";
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
		<div>
			<h1 class="text-2xl m-0 mb-1 text-text font-semibold">Forms</h1>
			<p class="m-0 text-sm text-subtle">Create and manage your encrypted forms</p>
		</div>
		{#if workspacesStore.active?.status !== 'pending'}
			<button
				onclick={() => {
					const ws = workspacesStore.active;
					goto(ws ? `/forms/new?workspaceId=${ws.id}` : '/forms/new');
				}}
				class="shrink-0 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100"
			>+ New form</button>
		{/if}
	</div>

	<WorkspaceHeader />

	{#if workspacesStore.active?.status === 'pending'}
		<div class="py-14 border border-dashed border-border-canvas rounded-lg text-center px-6">
			<p class="m-0 mb-1 text-text text-base font-medium">Access pending approval</p>
			<p class="m-0 text-subtle text-sm mt-1.5 max-w-sm mx-auto">
				A workspace admin needs to grant you access before you can view forms and workspace content.
			</p>
		</div>
	{:else if formsStore.loading && !formsStore.loaded}
		<p class="text-subtle text-base">Loading…</p>
	{:else if formsStore.error}
		<p class="text-danger text-base">{formsStore.error}</p>
	{:else if formsStore.forms.length === 0}
		<div class="py-12 border border-dashed border-border-canvas rounded-lg text-center">
			<p class="m-0 mb-1 text-subtle text-base">No forms yet</p>
			<p class="m-0 text-subtle text-base">Create your first form to get started</p>
			<button
				onclick={() => goto('/forms/new')}
				class="mt-4 px-4 py-2 bg-transparent text-text border border-border-canvas rounded cursor-pointer font-mono text-base hover:border-border-canvas transition-colors duration-100"
			>+ New form</button>
		</div>
	{:else}
		<div class="border border-border-canvas rounded-lg overflow-hidden">
			{#each formsStore.forms as form, i (form.formId)}
				<div
					class="flex items-center gap-6 px-4 py-3.5 cursor-pointer hover:bg-surface transition-colors duration-75
						{i < formsStore.forms.length - 1 ? 'border-b border-border-canvas' : ''}"
					onclick={() => goto(`/forms/${form.formId}`)}
					role="button"
					tabindex="0"
					onkeydown={e => e.key === 'Enter' && goto(`/forms/${form.formId}`)}
				>
					<!-- Name + description + status badge -->
					<div class="flex-1 min-w-0 flex flex-col">
						{#if renamingId === form.formId}
							<div onclick={e => e.stopPropagation()} role="none">
								<input
									bind:this={renameInputEl}
									type="text"
									bind:value={renameValue}
									onkeydown={(e) => {
										e.stopPropagation();
										if (e.key === 'Enter') saveRename();
										if (e.key === 'Escape') { renamingId = null; }
									}}
									disabled={renameSaving}
									class="w-full bg-transparent border-b border-border outline-none text-base text-text font-mono disabled:opacity-60"
								/>
								{#if renameError}
									<span class="text-xs text-danger">{renameError}</span>
								{/if}
							</div>
						{:else}
							<div class="flex items-center gap-2 min-w-0">
								<span class="text-base text-text truncate">
									{formsStore.formNames.get(form.formId) ?? '—'}
								</span>
								<StatusBadge status={form.status} />
							</div>
							{#if formsStore.formDescriptions.get(form.formId)}
								<span class="text-sm text-muted truncate">
									{formsStore.formDescriptions.get(form.formId)?.replace(/<[^>]*>/g, '') ?? ''}
								</span>
							{/if}
						{/if}
					</div>

					<!-- Response count -->
					<span class="shrink-0 flex items-center gap-1.5 text-subtle tabular-nums">
						<Users size={13} strokeWidth={1.75} />
						{form.responseCount}
					</span>

					<!-- Relative time (hidden on small screens) -->
					<span class="shrink-0 hidden sm:block text-subtle">
						{timeAgo(form.updatedAt)}
					</span>

					<!-- Copy share link -->
					{#if formsStore.shareUrls.has(form.formId)}
						<button
							onclick={e => copyLink(e, form.formId)}
							onkeydown={e => e.stopPropagation()}
							title="Copy share link"
							class="shrink-0 p-1 bg-transparent border-none rounded cursor-pointer transition-colors duration-100
								{copiedId === form.formId ? 'text-success' : 'text-subtle hover:text-text'}"
						>
							{#if copiedId === form.formId}
								<Check size={15} strokeWidth={2} />
							{:else}
								<Link size={15} strokeWidth={1.75} />
							{/if}
						</button>
					{/if}

					<!-- Edit link -->
					<button
						onclick={e => { e.stopPropagation(); goto(`/forms/${form.formId}/edit`); }}
						title="Edit form"
						class="shrink-0 p-1 bg-transparent border-none rounded cursor-pointer text-subtle hover:text-text transition-colors duration-100"
					><Pencil size={15} strokeWidth={1.75} /></button>

					<!-- Actions menu -->
					<div
						class="shrink-0"
						onclick={e => e.stopPropagation()}
						onkeydown={e => e.stopPropagation()}
						role="none"
					>
						<DropdownMenu>
							{#snippet trigger(attrs)}
								<button
									{...attrs}
									class="p-1 bg-transparent text-subtle border-none rounded cursor-pointer hover:text-text transition-colors duration-100"
								><Ellipsis size={16} strokeWidth={1.75} /></button>
							{/snippet}
							{#snippet children({ close })}
								<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/edit`); }}>Edit</DropdownMenuItem>
								<DropdownMenuItem onclick={() => { close(); handleRename(form); }}>Rename</DropdownMenuItem>
								<DropdownMenuItem onclick={() => { close(); handleDuplicate(form); }} disabled={duplicatingId === form.formId}>{duplicatingId === form.formId ? 'Duplicating…' : 'Duplicate'}</DropdownMenuItem>
								{#if form.status === 'draft'}
									<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/edit`); }}>Publish</DropdownMenuItem>
								{:else}
									<DropdownMenuItem onclick={() => { close(); toggleStatus(form); }}>{form.status === 'open' ? 'Close' : 'Open'}</DropdownMenuItem>
								{/if}
								<DropdownMenuSeparator />
								<DropdownMenuItem variant="destructive" onclick={() => { close(); handleDelete(form); }}>Delete</DropdownMenuItem>
							{/snippet}
						</DropdownMenu>
					</div>
				</div>
			{/each}
		</div>
	{/if}

</div>
</div>
