<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { resolveInvitation, acceptInvitation, ensureIdentityKey, WorkspaceError, type InvitePreview } from '$lib/workspaces';
	import { getAppConfig } from '$lib/config';
	import faviconSvg from '$lib/assets/favicon.svg?raw';

	type PageState = 'loading' | 'preview' | 'accepting' | 'pending' | 'already_member' | 'error';

	let pageState = $state<PageState>('loading');
	let preview = $state<InvitePreview | null>(null);
	let errorMsg = $state('');
	let registrationOpen = $state(true);

	const token = $derived(page.params.token as string);
	const isLoggedIn = $derived(auth.masterKey !== null);
	const autoAccept = $derived(page.url.searchParams.get('auto_accept') === '1');

	onMount(async () => {
		getAppConfig().then(c => { registrationOpen = c.registrationOpen; }).catch(() => {});
		try {
			preview = await resolveInvitation(token);
			if (isLoggedIn && autoAccept) {
				await accept();
				return;
			}
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
				goto(`/login?next=${encodeURIComponent(`/invite/${token}?auto_accept=1`)}`);
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

<div class="min-h-screen flex flex-col items-center justify-center px-4 font-mono">
	<div class="w-full max-w-[360px]">

		<!-- Logo + heading -->
		<div class="flex flex-col items-center mb-8">
			<a href="https://useconfide.app" class="w-14 h-14 mb-1 [&>svg]:w-full [&>svg]:h-full block">{@html faviconSvg}</a>
			<h1 class="text-xl font-semibold text-text tracking-tight">Workspace Invitation</h1>
			<p class="text-sm text-subtle mt-1.5">You've been invited to join a workspace.</p>
		</div>

		{#if pageState === 'loading' || (isLoggedIn && autoAccept && pageState === 'accepting')}
			<div class="bg-canvas border border-border rounded-xl p-6">
				<p class="text-subtle text-sm text-center">Loading invitation…</p>
			</div>

		{:else if pageState === 'error'}
			<div class="bg-canvas border border-border rounded-xl p-6">
				<p class="text-danger text-sm text-center">{errorMsg}</p>
			</div>
			<p class="text-xs text-subtle text-center mt-4">
				<a href="/login" class="text-text hover:underline">Sign in</a>
				{#if registrationOpen}
				or <a href="/signup" class="text-text hover:underline">create an account</a>
				{/if}
			</p>

		{:else if pageState === 'already_member'}
			<div class="bg-canvas border border-border rounded-xl p-6 text-center">
				<p class="text-sm text-subtle">
					You're already a member of <strong class="text-text">{preview?.workspaceName}</strong>.
				</p>
				<a href="/dashboard" class="text-text hover:underline text-xs mt-3 block">Go to dashboard</a>
			</div>

		{:else if pageState === 'pending'}
			<div class="bg-canvas border border-success-dim rounded-xl p-6">
				<p class="text-success text-sm font-medium">You've joined {preview?.workspaceName}.</p>
				<p class="text-sm text-subtle mt-2">
					Your access is pending approval from a workspace admin. You'll be able to view workspace content once they grant you access.
				</p>
				<a href="/dashboard" class="text-text hover:underline text-xs mt-4 block">Go to dashboard</a>
			</div>

		{:else if preview}
			<div class="bg-canvas border border-border rounded-xl p-6">
				<p class="text-sm text-subtle mb-1">You've been invited to join</p>
				<h2 class="text-base font-semibold text-text mb-5">{preview.workspaceName}</h2>

				<div class="grid grid-cols-2 gap-y-3 text-sm mb-6">
					<span class="text-subtle">Invited by</span>
					<span class="text-text">{preview.inviterUsername}</span>
					<span class="text-subtle">Role</span>
					<span class="text-text capitalize">{preview.role}</span>
					<span class="text-subtle">Expires</span>
					<span class="text-text">{formatExpiry(preview.expiresAt)}</span>
				</div>

				{#if isLoggedIn}
					<button
						onclick={accept}
						disabled={pageState === 'accepting'}
						class="w-full py-3 text-white border-none rounded-lg font-mono text-sm font-medium
							{pageState === 'accepting' ? 'bg-muted cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}
							transition-colors duration-100"
					>
						{pageState === 'accepting' ? 'Accepting…' : 'Accept invitation'}
					</button>
				{:else}
					{#if registrationOpen}
					<div class="flex gap-3">
						<a
							href="/signup?invite={token}"
							class="flex-1 py-3 text-center text-white bg-primary hover:bg-primary-hover rounded-lg font-mono text-sm font-medium no-underline transition-colors duration-100"
						>
							Create account
						</a>
						<a
							href="/login?next={encodeURIComponent(`/invite/${token}?auto_accept=1`)}"
							class="flex-1 py-3 text-center text-subtle hover:text-text bg-transparent border border-border rounded-lg font-mono text-sm font-medium no-underline transition-colors duration-100"
						>
							Sign in
						</a>
					</div>
					{:else}
					<a
						href="/login?next={encodeURIComponent(`/invite/${token}?auto_accept=1`)}"
						class="w-full py-3 text-center text-white bg-primary hover:bg-primary-hover rounded-lg font-mono text-sm font-medium no-underline transition-colors duration-100 block"
					>
						Sign in to accept
					</a>
					{/if}
				{/if}
			</div>

			<p class="text-xs text-subtle text-center mt-4">
				By accepting you agree to Confide's terms of service.
			</p>
		{/if}

	</div>
</div>
