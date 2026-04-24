<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { recover, rekey } from '$lib/auth';
	import faviconSvg from '$lib/assets/favicon.svg?raw';

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

<div class="min-h-screen flex flex-col items-center justify-center px-4 font-mono">
	<div class="w-full max-w-[360px]">

		<!-- Logo + heading -->
		<div class="flex flex-col items-center mb-8">
			<a href="https://useconfide.app" class="w-14 h-14 mb-1 [&>svg]:w-full [&>svg]:h-full block">{@html faviconSvg}</a>
			<h1 class="text-xl font-semibold text-text-body tracking-tight">Account Recovery</h1>
			<p class="text-sm text-muted-dim mt-1.5">Regain access using your recovery code.</p>
		</div>

		{#if step === 'enter-code'}
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

				<label class="block text-sm text-muted mb-1.5" for="recovery-code">Recovery code</label>
				<input
					id="recovery-code"
					type="text"
					bind:value={recoveryCode}
					placeholder="GHRK-XXXX-XXXX-XXXX-…"
					disabled={loading}
					class="input-base w-full mb-4 text-sm py-2.5 px-3"
				/>

				<button
					onclick={handleRecover}
					disabled={loading}
					class="w-full py-3 text-white border-none rounded-lg font-mono text-sm font-medium
						{loading ? 'bg-muted-mid cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}
						transition-colors duration-100"
				>
					{loading ? 'Verifying…' : 'Verify recovery code'}
				</button>

				{#if error}
					<p class="text-error text-xs mt-3 text-center">{error}</p>
				{/if}
			</div>

			<p class="text-xs text-muted-dark text-center mt-4">
				<a href="/login" class="text-text-blue hover:underline">Back to sign in</a>
			</p>

		{:else if step === 'rekey'}
			<!-- Form card -->
			<div class="bg-surface border border-border rounded-xl p-6">
				<div class="p-4 border border-success-text rounded-lg bg-success-bg-deep mb-5">
					<p class="text-success-text-dark text-xs m-0">
						Recovery code verified. Now register a new passkey on this device.
					</p>
				</div>

				<button
					onclick={handleRekey}
					disabled={loading}
					class="w-full py-3 text-white border-none rounded-lg font-mono text-sm font-medium
						{loading ? 'bg-muted-mid cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}
						transition-colors duration-100"
				>
					{loading ? 'Registering passkey…' : 'Register new passkey'}
				</button>

				{#if error}
					<p class="text-error text-xs mt-3 text-center">{error}</p>
				{/if}
			</div>

		{:else if step === 'success'}
			<div class="bg-surface border border-border rounded-xl p-6">
				<div class="p-4 border border-success-text rounded-lg bg-success-bg-deep text-success-text-dark text-xs text-center">
					New passkey registered. Redirecting to dashboard…
				</div>
			</div>
		{/if}

	</div>
</div>
