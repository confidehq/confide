<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { recover, rekey } from '$lib/auth';

	type Step = 'enter-code' | 'rekey' | 'success';

	let step = $state<Step>('enter-code');
	let error = $state<string | null>(null);
	let loading = $state(false);

	let accountId = $state('');
	let recoveryCode = $state('');

	// Held between steps
	let recoveredMasterKey = $state<CryptoKey | null>(null);
	let rekeyToken = $state('');

	async function handleRecover() {
		error = null;
		if (!accountId.trim()) {
			error = 'Account ID is required.';
			return;
		}
		if (!recoveryCode.trim()) {
			error = 'Recovery code is required.';
			return;
		}
		loading = true;
		try {
			const result = await recover(accountId.trim(), recoveryCode.trim());
			recoveredMasterKey = result.masterKey;
			rekeyToken = result.rekeyToken;
			step = 'rekey';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Recovery failed. Check your account ID and code.';
		} finally {
			loading = false;
		}
	}

	async function handleRekey() {
		if (!recoveredMasterKey) return;
		error = null;
		loading = true;
		try {
			const result = await rekey(recoveredMasterKey, rekeyToken);
			auth.setSession(recoveredMasterKey, accountId.trim(), result.credentialId);
			step = 'success';
			setTimeout(() => goto('/dashboard'), 1500);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Rekey failed.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Confide — Account Recovery</title>
</svelte:head>

<div style="font-family: monospace; max-width: 560px; margin: 60px auto; padding: 0 24px;">
	<h1 style="font-size: 1.4rem; margin-bottom: 8px;">Account Recovery</h1>

	{#if step === 'enter-code'}
		<p style="color: #888; font-size: 0.85rem; margin-bottom: 32px;">
			Enter your account ID and recovery code to regain access.
		</p>

		<div style="margin-bottom: 16px;">
			<label style="display: block; color: #9ca3af; font-size: 0.8rem; margin-bottom: 4px;">
				Account ID
			</label>
			<input
				type="text"
				bind:value={accountId}
				placeholder="Your account ID"
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
				"
			/>
		</div>

		<div style="margin-bottom: 20px;">
			<label style="display: block; color: #9ca3af; font-size: 0.8rem; margin-bottom: 4px;">
				Recovery code
			</label>
			<input
				type="text"
				bind:value={recoveryCode}
				placeholder="GHRK-XXXX-XXXX-XXXX-…"
				style="
					width: 100%;
					padding: 10px 12px;
					background: #111;
					border: 1px solid #374151;
					border-radius: 4px;
					color: #e5e7eb;
					font-family: monospace;
					font-size: 0.85rem;
					box-sizing: border-box;
				"
			/>
		</div>

		{#if error}
			<div style="color: #fca5a5; font-size: 0.85rem; margin-bottom: 12px;">{error}</div>
		{/if}

		<button
			onclick={handleRecover}
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
			"
		>
			{loading ? 'Verifying…' : 'Verify recovery code'}
		</button>

		<p style="font-size: 0.8rem; color: #6b7280; margin-top: 16px;">
			<a href="/login" style="color: #60a5fa;">Back to sign in</a>
		</p>

	{:else if step === 'rekey'}
		<div style="
			padding: 20px;
			border: 1px solid #166534;
			border-radius: 6px;
			background: #052e16;
			margin-bottom: 24px;
		">
			<p style="color: #bbf7d0; font-size: 0.9rem; margin: 0;">
				Recovery code verified. Now register a new passkey on this device.
			</p>
		</div>

		{#if error}
			<div style="color: #fca5a5; font-size: 0.85rem; margin-bottom: 12px;">{error}</div>
		{/if}

		<button
			onclick={handleRekey}
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
			"
		>
			{loading ? 'Registering passkey…' : 'Register new passkey'}
		</button>

	{:else if step === 'success'}
		<div style="
			padding: 24px;
			border: 1px solid #166534;
			border-radius: 6px;
			background: #052e16;
			color: #bbf7d0;
			font-size: 0.9rem;
			text-align: center;
		">
			New passkey registered. Redirecting to dashboard…
		</div>
	{/if}
</div>
