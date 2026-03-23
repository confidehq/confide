<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { login } from '$lib/auth';
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
		if (!auth.credentialId) return;
		reauthError = null;
		reauthLoading = true;
		try {
			const result = await login(auth.credentialId);
			auth.setSession(result.masterKey, result.accountId, auth.credentialId);
			showReauth = false;
		} catch (err) {
			reauthError = err instanceof Error ? err.message : 'Authentication failed.';
		} finally {
			reauthLoading = false;
		}
	}
</script>

<svelte:head>
	<style>
		html, body { margin: 0; padding: 0; background: #111827; }
	</style>
</svelte:head>

{#if showReauth}
	<!-- Re-auth overlay: shown when masterKey is gone (tab refresh) but credential exists -->
	<div style="
		position: fixed;
		inset: 0;
		background: rgba(0,0,0,0.85);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
	">
		<div style="
			font-family: monospace;
			max-width: 400px;
			width: 100%;
			padding: 32px;
			background: #111;
			border: 1px solid #374151;
			border-radius: 8px;
			margin: 0 24px;
		">
			<h2 style="font-size: 1rem; color: #e5e7eb; margin: 0 0 8px;">Session expired</h2>
			<p style="color: #9ca3af; font-size: 0.85rem; margin-bottom: 24px;">
				Your session key is no longer in memory. Re-authenticate to continue.
			</p>

			{#if reauthError}
				<div style="color: #fca5a5; font-size: 0.85rem; margin-bottom: 12px;">{reauthError}</div>
			{/if}

			<button
				onclick={handleReauth}
				disabled={reauthLoading}
				style="
					width: 100%;
					padding: 14px;
					background: {reauthLoading ? '#555' : '#2563eb'};
					color: white;
					border: none;
					border-radius: 6px;
					cursor: {reauthLoading ? 'not-allowed' : 'pointer'};
					font-family: monospace;
					font-size: 1rem;
				"
			>
				{reauthLoading ? 'Authenticating…' : 'Authenticate with passkey'}
			</button>

			<button
				onclick={() => goto('/login')}
				style="
					width: 100%;
					padding: 10px;
					margin-top: 8px;
					background: transparent;
					color: #6b7280;
					border: 1px solid #374151;
					border-radius: 6px;
					cursor: pointer;
					font-family: monospace;
					font-size: 0.85rem;
				"
			>
				Sign out
			</button>
		</div>
	</div>
{/if}

<Sidebar />

<!-- Canvas wrapper: fills viewport, provides inset for the floating sheet -->
<div style="
	margin-left: {sidebar.width}px;
	transition: margin-left 200ms ease;
	height: 100vh;
	overflow: hidden;
	padding: 12px;
	box-sizing: border-box;
	display: flex;
">
	<!-- Elevated sheet: floats above the canvas layer -->
	<div style="
		flex: 1;
		min-height: 0;
		background: #1a2332;
		border-radius: 12px;
		box-shadow: 0 0 0 1px #2d3f55;
		overflow: auto;
		display: flex;
		flex-direction: column;
	">
		{@render children()}
	</div>
</div>
