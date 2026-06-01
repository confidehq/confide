<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		lookupPairingByCode,
		requestPairing,
		pollPairing,
		completePairing,
		type PairingRequestResult
	} from '$lib/auth';
	import { pairingFingerprint } from '$lib/crypto';
	import { base64ToBytes } from '$lib/encoding';

	type Stage =
		| 'entry'        // waiting for QR scan or code entry
		| 'fingerprint'  // show fingerprint, waiting for existing device to confirm
		| 'waiting'      // state = requested, waiting for fulfill
		| 'registering'  // calling startRegistration
		| 'done'
		| 'error';

	let stage = $state<Stage>('entry');
	let errorMsg = $state('');
	let codeInput = $state('');
	let fingerprint = $state('');

	// Held in memory between requestPairing and completePairing
	let pairingReq = $state<PairingRequestResult | null>(null);
	let pairingToken = $state('');
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	// On mount, check URL params for a pre-scanned token
	$effect(() => {
		const t = page.url.searchParams.get('t');
		if (t) {
			pairingToken = t;
			startRequest(t);
		}
	});

	function stopPolling() {
		if (pollTimer !== null) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function startRequest(token: string) {
		try {
			const req = await requestPairing(token);
			pairingReq = req;
			// Compute and display fingerprint from our own public key
			fingerprint = await pairingFingerprint(req.publicKeyBytes);
			stage = 'fingerprint';
			startPolling(token);
		} catch (err: unknown) {
			const e = err as { code?: string; message?: string };
			if (e.code === 'pairing_claimed') {
				errorMsg = 'This pairing request was already accepted by another device.';
			} else if (e.code === 'pairing_not_found') {
				errorMsg = 'Pairing expired. Please start over on your other device.';
			} else {
				errorMsg = e.message ?? 'Failed to start pairing.';
			}
			stage = 'error';
		}
	}

	function startPolling(token: string) {
		stopPolling();
		pollTimer = setInterval(async () => {
			try {
				const status = await pollPairing(token);
				if (status.state === 'expired') {
					stopPolling();
					errorMsg = 'Pairing expired. Please start over on your other device.';
					stage = 'error';
					return;
				}
				if (status.state === 'fulfilled' && status.wrappedMasterKey && pairingReq) {
					stopPolling();
					stage = 'registering';
					try {
						const { masterKey, accountId, credentialId } = await completePairing(
							token,
							pairingReq,
							status.wrappedMasterKey
						);
						auth.setSession(masterKey, accountId, credentialId);
						stage = 'done';
						setTimeout(() => goto('/dashboard'), 800);
					} catch (err: unknown) {
						errorMsg = err instanceof Error ? err.message : 'Failed to complete pairing.';
						stage = 'error';
					}
				}
			} catch {
				// transient network errors — keep polling
			}
		}, 2000);
	}

	async function handleCodeSubmit() {
		const code = codeInput.trim().toUpperCase().replace(/[^A-Z0-9]/g, '');
		if (code.length < 6) {
			errorMsg = 'Please enter the full code shown on your other device.';
			return;
		}
		errorMsg = '';
		try {
			const token = await lookupPairingByCode(code);
			pairingToken = token;
			await startRequest(token);
		} catch (err: unknown) {
			errorMsg = err instanceof Error ? err.message : 'Code not found or expired.';
			stage = 'error';
		}
	}

	function reset() {
		stopPolling();
		stage = 'entry';
		errorMsg = '';
		codeInput = '';
		pairingReq = null;
		pairingToken = '';
		fingerprint = '';
	}
</script>

<svelte:head>
	<title>Confide — Add New Device</title>
</svelte:head>

<div class="font-mono min-h-screen flex items-center justify-center bg-canvas-base px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8">
			<h1 class="text-2xl text-text m-0 mb-1">Add this device</h1>
			<p class="text-subtle text-sm m-0">Sign in by pairing with a device you're already signed in on.</p>
		</div>

		{#if stage === 'entry'}
			<div class="flex flex-col gap-4">
				<div class="flex flex-col gap-2">
					<label class="text-text text-sm" for="pair-code">
						Enter the code shown on your signed-in device
					</label>
					<input
						id="pair-code"
						type="text"
						bind:value={codeInput}
						placeholder="e.g. AB3K7M2P"
						maxlength="12"
						class="font-mono bg-canvas border border-border rounded px-3 py-2 text-sm text-text placeholder-subtle focus:outline-none focus:border-border uppercase tracking-widest"
						onkeydown={(e) => { if (e.key === 'Enter') handleCodeSubmit(); }}
					/>
				</div>
				{#if errorMsg}
					<p class="text-danger text-xs m-0">{errorMsg}</p>
				{/if}
				<button
					onclick={handleCodeSubmit}
					class="px-4 py-2 bg-info-light border border-canvas rounded text-sm text-text hover:bg-info transition-colors cursor-pointer font-mono"
				>
					Continue
				</button>
				<p class="text-subtle text-xs m-0 text-center">
					Or scan the QR code on your other device's screen with your camera app.
				</p>
			</div>

		{:else if stage === 'fingerprint'}
			<div class="flex flex-col gap-5">
				<div class="p-4 border border-border rounded-md bg-canvas">
					<p class="text-text text-sm m-0 mb-3">
						On your <strong class="text-text">signed-in device</strong>, confirm these words before tapping Yes:
					</p>
					<div class="flex gap-2 flex-wrap">
						{#each fingerprint.split('-') as word}
							<span class="px-2.5 py-1 bg-info-code-bg border border-border rounded text-sm text-info-code-text font-mono tracking-wide">
								{word}
							</span>
						{/each}
					</div>
				</div>
				<p class="text-subtle text-xs m-0">
					Waiting for approval on your other device…
				</p>
				<div class="flex items-center gap-2">
					<span class="inline-block w-2 h-2 bg-status-waiting-green rounded-full animate-pulse"></span>
					<span class="text-subtle text-xs">Connected</span>
				</div>
				<button
					onclick={reset}
					class="text-xs text-subtle hover:text-subtle cursor-pointer bg-transparent border-none font-mono p-0 self-start"
				>
					Cancel
				</button>
			</div>

		{:else if stage === 'waiting'}
			<div class="flex flex-col gap-4">
				<p class="text-subtle text-sm animate-pulse m-0">Waiting for approval on your other device…</p>
				<button onclick={reset} class="text-xs text-subtle hover:text-subtle cursor-pointer bg-transparent border-none font-mono p-0 self-start">
					Cancel
				</button>
			</div>

		{:else if stage === 'registering'}
			<p class="text-subtle text-sm animate-pulse m-0">Creating your passkey…</p>

		{:else if stage === 'done'}
			<div class="flex flex-col gap-3">
				<p class="text-success text-sm m-0">Device added. Redirecting…</p>
			</div>

		{:else if stage === 'error'}
			<div class="flex flex-col gap-4">
				<p class="text-danger text-sm m-0">{errorMsg}</p>
				<button
					onclick={reset}
					class="px-4 py-2 bg-transparent border border-border rounded text-sm text-subtle hover:text-subtle transition-colors cursor-pointer font-mono self-start"
				>
					Try again
				</button>
			</div>
		{/if}
	</div>
</div>
