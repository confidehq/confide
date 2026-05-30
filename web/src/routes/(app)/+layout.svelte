<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { reauthenticate, getMe, isPasskeyCancelled } from '$lib/auth';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import BottomNav from '$lib/components/BottomNav.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let showReauth = $state(false);
	let reauthError = $state<string | null>(null);
	let reauthLoading = $state(false);

	$effect(() => {
		if (auth.masterKey === null && auth.credentialId !== null) {
			// Verify the server session is still alive before showing the reauth overlay.
			// If it's expired, clear stored credentials and send to /login.
			getMe()
				.then(() => { showReauth = true; })
				.catch(() => {
					auth.clearAll();
					goto('/login');
				});
		} else if (auth.masterKey === null && auth.credentialId === null) {
			goto('/login');
		}
	});

	// Load workspaces eagerly — only needs a valid session cookie, no masterKey
	$effect(() => {
		if (auth.credentialId !== null) {
			workspacesStore.load().catch(() => {});
		}
	});

	// Fetch username once on session start
	$effect(() => {
		if (auth.credentialId !== null && auth.username === null) {
			getMe().then((me) => auth.setUsername(me.username ?? null)).catch(() => {});
		}
	});

	async function handleReauth() {
		reauthError = null;
		reauthLoading = true;
		try {
			const result = await reauthenticate();
			auth.setSession(result.masterKey, result.accountId, result.credentialId);
			showReauth = false;
		} catch (err) {
			reauthError = isPasskeyCancelled(err)
				? 'Authentication was cancelled or timed out. Unlock your passkey manager and try again.'
				: err instanceof Error ? err.message : 'Authentication failed.';
		} finally {
			reauthLoading = false;
		}
	}
</script>

{#if showReauth}
	<!-- Re-auth overlay: shown when masterKey is gone (tab refresh) but credential exists -->
	<div class="fixed inset-0 bg-black/85 flex items-center justify-center z-[1000]">
		<div class="font-mono max-w-[400px] w-full p-8 bg-base border border-border rounded-lg mx-6">
			<h2 class="text-base text-text m-0 mb-2">Unlock your session</h2>
			<p class="text-subtle text-sm mb-6">
				Your session key is no longer in memory. Re-authenticate to continue.
			</p>

			{#if reauthError}
				<div class="text-error-muted text-sm mb-3">{reauthError}</div>
			{/if}

			<button
				onclick={handleReauth}
				disabled={reauthLoading}
				class="w-full py-3.5 text-white border-none rounded-md font-mono text-base
					{reauthLoading ? 'bg-muted cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
			>
				{reauthLoading ? 'Authenticating…' : 'Authenticate with passkey'}
			</button>

			<button
				onclick={() => goto('/login')}
				class="w-full py-2.5 mt-2 bg-transparent border-l-2 text-subtle border border-border rounded-md cursor-pointer font-mono text-sm hover:text-text transition-colors duration-100"
			>
				Sign out
			</button>
		</div>
	</div>
{/if}

<Sidebar />
<BottomNav />

<!-- Canvas wrapper: fills viewport, provides inset for the floating sheet -->
<div
	style="--sidebar-w: {sidebar.width}px;"
	class="app-main-content sm:[margin-left:var(--sidebar-w)] transition-[margin-left] duration-200 ease-linear h-screen overflow-hidden p-3 pb-[76px] sm:pb-3 box-border flex"
>
	<!-- Elevated sheet: floats above the canvas layer -->
	<div class="flex-1 min-h-0 bg-canvas rounded-xl shadow-[0_0_0_1px_var(--color-border)] overflow-auto flex flex-col">
		{@render children()}
	</div>
</div>
