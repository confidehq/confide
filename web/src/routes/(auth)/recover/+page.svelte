<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { recover, rekey } from '$lib/auth';

	type Step = 'enter-code' | 'rekey' | 'success';

	let step = $state<Step>('enter-code');
	let error = $state<string | null>(null);
	let loading = $state(false);

	let username = $state('');
	let recoveryCode = $state('');

	// Held between steps
	let recoveredMasterKey = $state<CryptoKey | null>(null);
	let recoveredAccountId = $state('');
	let rekeyToken = $state('');

	async function handleRecover() {
		error = null;
		if (!username.trim()) {
			error = 'Username is required.';
			return;
		}
		if (!recoveryCode.trim()) {
			error = 'Recovery code is required.';
			return;
		}
		loading = true;
		try {
			const result = await recover(username.trim(), recoveryCode.trim());
			recoveredMasterKey = result.masterKey;
			recoveredAccountId = result.accountId;
			rekeyToken = result.rekeyToken;
			step = 'rekey';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Recovery failed. Check your username and code.';
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
			workspacesStore.clear();
			auth.setSession(recoveredMasterKey, recoveredAccountId, result.credentialId);
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

<div class="font-mono max-w-[560px] mx-auto mt-[60px] px-6">
	<h1 class="text-2xl mb-2">Account Recovery</h1>

	{#if step === 'enter-code'}
		<p class="text-muted text-sm mb-8">
			Enter your username and recovery code to regain access.
		</p>

		<div class="mb-4">
			<label class="block text-muted text-xs mb-1">Username</label>
			<input
				type="text"
				bind:value={username}
				placeholder="Your username"
				class="input-base py-2.5 px-3 text-sm"
			/>
		</div>

		<div class="mb-5">
			<label class="block text-muted text-xs mb-1">Recovery code</label>
			<input
				type="text"
				bind:value={recoveryCode}
				placeholder="GHRK-XXXX-XXXX-XXXX-…"
				class="input-base py-2.5 px-3 text-sm"
			/>
		</div>

		{#if error}
			<div class="text-error-muted text-sm mb-3">{error}</div>
		{/if}

		<button
			onclick={handleRecover}
			disabled={loading}
			class="w-full py-3.5 text-white border-none rounded-md font-mono text-base
				{loading ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
		>
			{loading ? 'Verifying…' : 'Verify recovery code'}
		</button>

		<p class="text-xs text-muted-dark mt-4">
			<a href="/login" class="text-text-blue">Back to sign in</a>
		</p>

	{:else if step === 'rekey'}
		<div class="p-5 border border-success-text rounded-md bg-success-bg-deep mb-6">
			<p class="text-success-text-dark text-sm m-0">
				Recovery code verified. Now register a new passkey on this device.
			</p>
		</div>

		{#if error}
			<div class="text-error-muted text-sm mb-3">{error}</div>
		{/if}

		<button
			onclick={handleRekey}
			disabled={loading}
			class="w-full py-3.5 text-white border-none rounded-md font-mono text-base
				{loading ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
		>
			{loading ? 'Registering passkey…' : 'Register new passkey'}
		</button>

	{:else if step === 'success'}
		<div class="p-6 border border-success-text rounded-md bg-success-bg-deep text-success-text-dark text-sm text-center">
			New passkey registered. Redirecting to dashboard…
		</div>
	{/if}
</div>
