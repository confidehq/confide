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
	import { Users, Ellipsis } from '@lucide/svelte';

	function timeAgo(dateStr: string): string {
		const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
		if (seconds < 60) return 'just now';
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return minutes === 1 ? '1 min ago' : `${minutes} mins ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return hours === 1 ? 'about an hour ago' : `${hours} hours ago`;
		const days = Math.floor(hours / 24);
		if (days < 7) return days === 1 ? 'yesterday' : `${days} days ago`;
		const weeks = Math.floor(days / 7);
		if (weeks < 4) return weeks === 1 ? 'a week ago' : `${weeks} weeks ago`;
		const months = Math.floor(days / 30);
		if (months < 12) return months === 1 ? 'a month ago' : `${months} months ago`;
		const years = Math.floor(days / 365);
		return years === 1 ? 'a year ago' : `${years} years ago`;
	}

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
			<h1 class="text-2xl m-0 mb-1 text-text-bright font-semibold">Forms</h1>
			{#if workspacesStore.active}
				<p class="m-0 text-sm text-muted-dim">{workspacesStore.active.name}</p>
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
		<p class="text-muted-dim text-base">Loading…</p>
	{:else if formsStore.error}
		<p class="text-error-light text-base">{formsStore.error}</p>
	{:else if formsStore.forms.length === 0}
		<div class="py-12 border border-dashed border-border rounded-lg text-center">
			<p class="m-0 mb-1 text-muted-dim text-base">No forms yet</p>
			<p class="m-0 text-muted-mid text-base">Create your first form to get started</p>
			<button
				onclick={() => goto('/forms/new')}
				class="mt-4 px-4 py-2 bg-transparent text-text-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100"
			>+ New form</button>
		</div>
	{:else}
		<div class="border border-border-deep rounded-lg overflow-hidden">
			{#each formsStore.forms as form, i (form.formId)}
				<div
					class="flex items-center gap-6 px-4 py-3.5 cursor-pointer hover:bg-border-card transition-colors duration-100
						{i < formsStore.forms.length - 1 ? 'border-b border-border-deep' : ''}"
					onclick={() => goto(`/forms/${form.formId}`)}
					role="button"
					tabindex="0"
					onkeydown={e => e.key === 'Enter' && goto(`/forms/${form.formId}`)}
				>
					<!-- Name + status badge -->
					<div class="flex-1 min-w-0 flex items-center gap-4">
						<span class="text-base text-text-body truncate">
							{formsStore.formNames.get(form.formId) ?? '—'}
						</span>
						<span class="shrink-0 px-2.5 py-0.5 rounded-full text-base
							{form.status === 'open'
								? 'bg-open-bg text-open-text border border-open-border'
								: 'bg-closed-bg text-closed-text border border-closed-border'}">
							{form.status}
						</span>
					</div>

					<!-- Response count -->
					<span class="shrink-0 flex items-center gap-1.5 text-base text-muted-dim tabular-nums">
						<Users size={13} strokeWidth={1.75} />
						{form.responseCount}
					</span>

					<!-- Relative time (hidden on small screens) -->
					<span class="shrink-0 hidden sm:block text-base text-muted-dim">
						{timeAgo(form.updatedAt)}
					</span>

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
									class="p-1 bg-transparent text-muted-dim border-none rounded cursor-pointer hover:text-text-body transition-colors duration-100"
								><Ellipsis size={16} strokeWidth={1.75} /></button>
							{/snippet}
							{#snippet children({ close })}
								<DropdownMenuItem onclick={() => { close(); goto(`/forms/${form.formId}/edit`); }}>Edit</DropdownMenuItem>
								<DropdownMenuItem onclick={() => { close(); toggleStatus(form); }}>{form.status === 'open' ? 'Close' : 'Open'}</DropdownMenuItem>
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
