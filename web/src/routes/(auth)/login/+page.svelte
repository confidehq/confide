<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte.ts';
	import { login } from '$lib/auth.ts';

	let error = $state<string | null>(null);
	let loading = $state(false);

	const hasCredential = auth.hasStoredCredential;

	async function handleLogin() {
		if (!auth.credentialId) return;
		error = null;
		loading = true;
		try {
			const result = await login(auth.credentialId);
			auth.setSession(result.masterKey, result.accountId, auth.credentialId!);
			goto('/dashboard');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>GhostForm — Sign In</title>
</svelte:head>

<div style="font-family: monospace; max-width: 480px; margin: 80px auto; padding: 0 24px;">
	<h1 style="font-size: 1.4rem; margin-bottom: 8px;">GhostForm</h1>
	<p style="color: #888; font-size: 0.85rem; margin-bottom: 40px;">Sign in with your passkey.</p>

	{#if hasCredential}
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
			Lost your device?
			<a href="/recover" style="color: #60a5fa;">Recover with recovery codes</a>
		</p>
	{:else}
		<div style="
			padding: 20px;
			border: 1px solid #374151;
			border-radius: 6px;
			color: #9ca3af;
			font-size: 0.9rem;
			margin-bottom: 24px;
		">
			No passkey registered on this device.
		</div>
		<a href="/signup" style="
			display: block;
			width: 100%;
			padding: 14px;
			background: #2563eb;
			color: white;
			border: none;
			border-radius: 6px;
			text-align: center;
			text-decoration: none;
			font-family: monospace;
			font-size: 1rem;
			box-sizing: border-box;
			margin-bottom: 16px;
		">Create an account</a>
		<p style="font-size: 0.8rem; color: #6b7280;">
			Have an account on another device?
			<a href="/recover" style="color: #60a5fa;">Recover with recovery codes</a>
		</p>
	{/if}
</div>
