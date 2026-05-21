<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { login, isPasskeyCancelled } from '$lib/auth';
	import { ensureIdentityKey, setupPersonalWorkspaceKey } from '$lib/workspaces';
	import { getAppConfig } from '$lib/config';
	import faviconSvg from '$lib/assets/favicon.svg?raw';

	let error = $state<string | null>(null);
	let loading = $state(false);
	let username = $state('');
	let registrationOpen = $state(true);

	$effect(() => {
		getAppConfig().then(c => { registrationOpen = c.registrationOpen; }).catch(() => {});
	});

	const next = $derived(page.url.searchParams.get('next') ?? '/dashboard');

	async function handleLogin() {
		error = null;
		loading = true;
		try {
			const result = await login(auth.credentialId, username.trim() || undefined);
			auth.setSession(result.masterKey, result.accountId, result.credentialId);
			// Heal any account that never got a personal workspace key (e.g. signup
			// interrupted before key provisioning completed). No-op if already set up.
			ensureIdentityKey(result.masterKey)
				.then(() => setupPersonalWorkspaceKey(result.masterKey, result.accountId))
				.catch(() => {});
			goto(next);
		} catch (err) {
			if (!isPasskeyCancelled(err)) {
				error = err instanceof Error ? err.message : 'Login failed.';
			}
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Confide — Sign In</title>
</svelte:head>

<div class="min-h-screen flex flex-col items-center justify-center px-4 font-mono">
	<div class="w-full max-w-[360px]">

		<!-- Logo + heading -->
		<div class="flex flex-col items-center mb-8">
			<a href="https://useconfide.app" class="w-14 h-14 mb-1 [&>svg]:w-full [&>svg]:h-full block">{@html faviconSvg}</a>
			<h1 class="text-xl font-semibold text-text-body tracking-tight">Sign in to Confide</h1>
			<p class="text-sm text-muted-dim mt-1.5">Use your passkey to continue.</p>
		</div>

		<!-- Form card -->
		<div class="bg-surface border border-border rounded-xl p-6">
			<label class="block text-sm text-muted mb-1.5" for="username">Username</label>
			<input
				id="username"
				type="text"
				bind:value={username}
				placeholder="your username"
				disabled={loading}
				class="input-base w-full mb-4 text-sm py-2.5 px-3"
			/>

			<button
				onclick={handleLogin}
				disabled={loading}
				class="w-full py-3 text-white border-none rounded-lg font-mono text-sm font-medium
					{loading ? 'bg-muted-mid cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}
					transition-colors duration-100"
			>
				{loading ? 'Authenticating…' : 'Sign in with passkey'}
			</button>

			{#if error}
				<p class="text-error text-xs mt-3 text-center">{error}</p>
			{/if}
		</div>

		<!-- Recovery link -->
		<p class="text-xs text-muted-dark text-center mt-4">
			Lost your passkey?
			<a href="/recover" class="text-text-blue hover:underline">Recover your account</a>
		</p>

		<!-- Sign up -->
		{#if registrationOpen}
		<div class="mt-6 pt-5 border-t border-border text-center">
			<p class="text-sm text-muted-dim">
				Don't have an account?
				<a href="/signup" class="text-text-blue hover:underline font-medium">Sign up</a>
			</p>
		</div>
		{/if}

	</div>
</div>
