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

<div class="font-mono max-w-[560px] mx-auto mt-[60px] px-6">
	<h1 class="text-[1.4rem] mb-2">Account Recovery</h1>

	{#if step === 'enter-code'}
		<p class="text-[#888] text-[0.85rem] mb-8">
			Enter your account ID and recovery code to regain access.
		</p>

		<div class="mb-4">
			<label class="block text-muted text-[0.8rem] mb-1">Account ID</label>
			<input
				type="text"
				bind:value={accountId}
				placeholder="Your account ID"
				class="input-base py-2.5 px-3 text-[0.9rem]"
			/>
		</div>

		<div class="mb-5">
			<label class="block text-muted text-[0.8rem] mb-1">Recovery code</label>
			<input
				type="text"
				bind:value={recoveryCode}
				placeholder="GHRK-XXXX-XXXX-XXXX-…"
				class="input-base py-2.5 px-3 text-[0.85rem]"
			/>
		</div>

		{#if error}
			<div class="text-error-muted text-[0.85rem] mb-3">{error}</div>
		{/if}

		<button
			onclick={handleRecover}
			disabled={loading}
			class="w-full py-3.5 text-white border-none rounded-md font-mono text-base
				{loading ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
		>
			{loading ? 'Verifying…' : 'Verify recovery code'}
		</button>

		<p class="text-[0.8rem] text-muted-dark mt-4">
			<a href="/login" class="text-[#60a5fa]">Back to sign in</a>
		</p>

	{:else if step === 'rekey'}
		<div class="p-5 border border-[#166534] rounded-md bg-[#052e16] mb-6">
			<p class="text-[#bbf7d0] text-[0.9rem] m-0">
				Recovery code verified. Now register a new passkey on this device.
			</p>
		</div>

		{#if error}
			<div class="text-error-muted text-[0.85rem] mb-3">{error}</div>
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
		<div class="p-6 border border-[#166534] rounded-md bg-[#052e16] text-[#bbf7d0] text-[0.9rem] text-center">
			New passkey registered. Redirecting to dashboard…
		</div>
	{/if}
</div>
