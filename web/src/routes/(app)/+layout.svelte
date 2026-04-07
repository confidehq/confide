<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { reauthenticate } from '$lib/auth';
	import { sidebar } from '$lib/stores/sidebar.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let showReauth = $state(false);
	let reauthError = $state<string | null>(null);
	let reauthLoading = $state(false);

	$effect(() => {
		if (auth.masterKey === null && auth.credentialId !== null) {
			showReauth = true;
		} else if (auth.masterKey === null && auth.credentialId === null) {
			goto('/login');
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
			reauthError = err instanceof Error ? err.message : 'Authentication failed.';
		} finally {
			reauthLoading = false;
		}
	}
</script>

{#if showReauth}
	<!-- Re-auth overlay: shown when masterKey is gone (tab refresh) but credential exists -->
	<div class="fixed inset-0 bg-black/85 flex items-center justify-center z-[1000]">
		<div class="font-mono max-w-[400px] w-full p-8 bg-[#111] border border-border rounded-lg mx-6">
			<h2 class="text-base text-text m-0 mb-2">Unlock your session</h2>
			<p class="text-muted text-[0.975rem] mb-6">
				Your session key is no longer in memory. Re-authenticate to continue.
			</p>

			{#if reauthError}
				<div class="text-error-muted text-[0.975rem] mb-3">{reauthError}</div>
			{/if}

			<button
				onclick={handleReauth}
				disabled={reauthLoading}
				class="w-full py-3.5 text-white border-none rounded-md font-mono text-base
					{reauthLoading ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
			>
				{reauthLoading ? 'Authenticating…' : 'Authenticate with passkey'}
			</button>

			<button
				onclick={() => goto('/login')}
				class="w-full py-2.5 mt-2 bg-transparent text-muted-dark border border-border rounded-md cursor-pointer font-mono text-[0.975rem] hover:text-text transition-colors duration-100"
			>
				Sign out
			</button>
		</div>
	</div>
{/if}

<Sidebar />

<!-- Canvas wrapper: fills viewport, provides inset for the floating sheet -->
<div
	style="margin-left: {sidebar.width}px;"
	class="transition-[margin-left] duration-200 ease-linear h-screen overflow-hidden p-3 box-border flex"
>
	<!-- Elevated sheet: floats above the canvas layer -->
	<div class="flex-1 min-h-0 bg-surface rounded-xl shadow-[0_0_0_1px_var(--color-border-subtle)] overflow-auto flex flex-col">
		{@render children()}
	</div>
</div>
