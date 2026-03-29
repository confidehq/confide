<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { login } from '$lib/auth';

	let error = $state<string | null>(null);
	let loading = $state(false);
	let username = $state('');

	async function handleLogin() {
		error = null;
		loading = true;
		try {
			// Prefer username for targeted login with correct PRF salt.
			// Fall back to stored credentialId if username is blank.
			const result = await login(auth.credentialId, username.trim() || undefined);
			auth.setSession(result.masterKey, result.accountId, result.credentialId);
			goto('/dashboard');
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

<div style="font-family: monospace; max-width: 480px; margin: 80px auto; padding: 0 24px;">
	<h1 style="font-size: 1.4rem; margin-bottom: 8px;">Confide</h1>
	<p style="color: #888; font-size: 0.85rem; margin-bottom: 32px;">Sign in with your passkey.</p>

	<label style="display: block; color: #9ca3af; font-size: 0.85rem; margin-bottom: 6px;">
		Username
	</label>
	<input
		type="text"
		bind:value={username}
		placeholder="your username"
		disabled={loading}
		style="
			width: 100%;
			padding: 10px 12px;
			background: #111;
			border: 1px solid #374151;
			border-radius: 4px;
			color: #e5e7eb;
			font-family: monospace;
			font-size: 0.9rem;
			box-sizing: border-box;
			margin-bottom: 16px;
		"
	/>

	<button
		onclick={handleLogin}
		disabled={loading}
		style="
			width: 100%;
			padding: 14px;
			background: {loading ? '#555' : '#2563eb'};
			color: white;
			border: none;
			border-radius: 6px;
			cursor: {loading ? 'not-allowed' : 'pointer'};
			font-family: monospace;
			font-size: 1rem;
			margin-bottom: 16px;
		"
	>
		{loading ? 'Authenticating…' : 'Sign in with passkey'}
	</button>

	{#if error}
		<div style="color: #fca5a5; font-size: 0.85rem; margin-bottom: 16px;">{error}</div>
	{/if}

	<p style="font-size: 0.8rem; color: #6b7280; margin-top: 24px;">
		Lost your passkey?
		<a href="/recover" style="color: #60a5fa;">Recover with recovery codes</a>
	</p>
</div>
