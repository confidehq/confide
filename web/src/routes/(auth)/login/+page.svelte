<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { login } from '$lib/auth';

	let error = $state<string | null>(null);
	let loading = $state(false);
	let username = $state('');

	const next = $derived(page.url.searchParams.get('next') ?? '/dashboard');

	async function handleLogin() {
		error = null;
		loading = true;
		try {
			// Prefer username for targeted login with correct PRF salt.
			// Fall back to stored credentialId if username is blank.
			const result = await login(auth.credentialId, username.trim() || undefined);
			auth.setSession(result.masterKey, result.accountId, result.credentialId);
			goto(next);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Confide — Sign In</title>
</svelte:head>

<div class="font-mono max-w-[480px] mx-auto mt-20 px-6">
	<h1 class="text-2xl mb-2">Confide</h1>
	<p class="text-muted text-sm mb-8">Sign in with your passkey.</p>

	<label class="block text-muted text-sm mb-1.5">Username</label>
	<input
		type="text"
		bind:value={username}
		placeholder="your username"
		disabled={loading}
		class="input-base mb-4 text-sm py-2.5 px-3"
	/>

	<button
		onclick={handleLogin}
		disabled={loading}
		class="w-full py-3.5 text-white border-none rounded-md font-mono text-base mb-4
			{loading ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
	>
		{loading ? 'Authenticating…' : 'Sign in with passkey'}
	</button>

	{#if error}
		<div class="text-error-muted text-sm mb-4">{error}</div>
	{/if}

	<p class="text-xs text-muted-dark mt-6">
		Lost your passkey?
		<a href="/recover" class="text-text-blue">Recover with recovery codes</a>
	</p>
</div>
