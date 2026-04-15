<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { resolveInvitation, acceptInvitation, ensureIdentityKey, WorkspaceError, type InvitePreview } from '$lib/workspaces';

	type PageState = 'loading' | 'preview' | 'accepting' | 'pending' | 'already_member' | 'error';

	let pageState = $state<PageState>('loading');
	let preview = $state<InvitePreview | null>(null);
	let errorMsg = $state('');

	const token = $derived(page.params.token as string);
	const isLoggedIn = $derived(auth.masterKey !== null);

	onMount(async () => {
		try {
			preview = await resolveInvitation(token);
			pageState = 'preview';
		} catch (err) {
			errorMsg = err instanceof Error ? err.message : 'Invitation not found or expired.';
			pageState = 'error';
		}
	});

	async function accept() {
		pageState = 'accepting';
		try {
			await acceptInvitation(token);
			// Ensure an identity keypair exists so the admin can grant the workspace key
			if (auth.masterKey) await ensureIdentityKey(auth.masterKey);
			pageState = 'pending';
		} catch (err) {
			if (err instanceof WorkspaceError && err.code === 'conflict') {
				pageState = 'already_member';
				return;
			}
			if (err instanceof WorkspaceError && err.code === 'unauthorized') {
				goto(`/login?next=/invite/${token}`);
				return;
			}
			errorMsg = err instanceof Error ? err.message : 'Failed to accept invitation.';
			pageState = 'error';
		}
	}

	function formatExpiry(iso: string): string {
		const d = new Date(iso);
		return d.toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' });
	}
</script>

<svelte:head>
	<title>Confide — Workspace Invitation</title>
</svelte:head>

<div class="font-mono max-w-[480px] mx-auto mt-20 px-6">
	<h1 class="text-2xl mb-8">Confide</h1>

	{#if pageState ==='loading'}
		<p class="text-muted text-sm">Loading invitation…</p>

	{:else if pageState ==='error'}
		<div class="p-5 border border-danger-text rounded-md bg-danger-dark text-error-muted text-sm">
			{errorMsg}
		</div>
		<p class="text-xs text-muted-dark mt-6">
			<a href="/login" class="text-text-blue">Sign in</a> or
			<a href="/signup" class="text-text-blue">create an account</a>
		</p>

	{:else if pageState ==='already_member'}
		<div class="p-5 border border-border rounded-md bg-[#0d0d0d] text-sm text-muted">
			You're already a member of <strong class="text-text">{preview?.workspaceName}</strong>.
		</div>
		<p class="text-xs text-muted-dark mt-4">
			<a href="/dashboard" class="text-text-blue">Go to dashboard</a>
		</p>

	{:else if pageState ==='pending'}
		<div class="p-5 border border-success-text rounded-md bg-success-bg-deep text-success-text-dark text-sm">
			<strong>You've joined {preview?.workspaceName}.</strong>
			<p class="mt-2 text-[#86efac]">
				Your access is pending approval from a workspace admin. You'll be able to view workspace
				content once they grant you access.
			</p>
		</div>
		<p class="text-xs text-muted-dark mt-4">
			<a href="/dashboard" class="text-text-blue">Go to dashboard</a>
		</p>

	{:else if preview}
		<!-- Preview card -->
		<div class="p-6 border border-border rounded-md bg-[#0d0d0d] mb-6">
			<p class="text-muted text-sm mb-1">You've been invited to join</p>
			<h2 class="text-xl text-text mb-4">{preview.workspaceName}</h2>

			<div class="grid grid-cols-2 gap-y-3 text-sm mb-5">
				<span class="text-muted">Invited by</span>
				<span class="text-text">{preview.inviterUsername}</span>
				<span class="text-muted">Role</span>
				<span class="text-text capitalize">{preview.role}</span>
				<span class="text-muted">Expires</span>
				<span class="text-text">{formatExpiry(preview.expiresAt)}</span>
			</div>

			{#if isLoggedIn}
				<button
					onclick={accept}
					disabled={pageState ==='accepting'}
					class="w-full py-3 text-white border-none rounded-md font-mono text-sm
						{pageState ==='accepting' ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
				>
					{pageState ==='accepting' ? 'Accepting…' : 'Accept invitation'}
				</button>
			{:else}
				<div class="flex gap-3">
					<a
						href="/signup?invite={token}"
						class="flex-1 py-3 text-center text-white bg-primary hover:bg-primary-hover rounded-md font-mono text-sm no-underline"
					>
						Create account
					</a>
					<a
						href="/login?next=/invite/{token}"
						class="flex-1 py-3 text-center text-muted bg-surface hover:text-text border border-border rounded-md font-mono text-sm no-underline transition-colors duration-100"
					>
						Sign in
					</a>
				</div>
			{/if}
		</div>

		<p class="text-xs text-muted-dark">
			By accepting you agree to Confide's terms of service.
		</p>
	{/if}
</div>
