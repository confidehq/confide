<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { detectPRFSupport } from '$lib/prf-detection';
	import { register } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { acceptInvitation, ensureIdentityKey } from '$lib/workspaces';

	type Step = 'checking' | 'briefing' | 'creating' | 'recovery' | 'success';

	let step = $state<Step>('checking');
	let prfError = $state<string | null>(null);
	let registerError = $state<string | null>(null);
	let loading = $state(false);

	// Briefing scroll gate
	let briefingScrolled = $state(false);
	let briefingRef = $state<HTMLDivElement | undefined>(undefined);
	let sentinelRef = $state<HTMLDivElement | undefined>(undefined);

	// Recovery code state — single GHRK-... string
	let recoveryCode = $state('');
	let verifyInput = $state('');
	let verifyError = $state(false);
	let verifyPassed = $state(false);

	// Generated registration result (held for setSession after verify)
	let pendingMasterKey = $state<CryptoKey | null>(null);
	let pendingAccountId = $state<string | null>(null);
	let pendingCredentialId = $state<string | null>(null);

	// Username input
	let username = $state('');

	// PRF check on mount
	onMount(async () => {
		const result = await detectPRFSupport();
		if (!result.supported) {
			prfError = result.reason;
			step = 'checking';
		} else {
			step = 'briefing';
		}
	});

	// IntersectionObserver for briefing scroll gate
	$effect(() => {
		if (step !== 'briefing' || !sentinelRef) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) briefingScrolled = true;
			},
			{ threshold: 1.0 }
		);
		observer.observe(sentinelRef);
		return () => observer.disconnect();
	});

	async function startRegistration() {
		if (!username.trim()) {
			registerError = 'Please enter a username.';
			return;
		}
		loading = true;
		registerError = null;
		try {
			const result = await register(username.trim());
			recoveryCode = result.recoveryCode;
			pendingMasterKey = result.masterKey;
			pendingAccountId = result.accountId;
			pendingCredentialId = result.credentialId;
			step = 'recovery';
		} catch (err) {
			registerError = err instanceof Error ? err.message : 'Registration failed.';
		} finally {
			loading = false;
		}
	}

	function checkVerification() {
		const normalize = (s: string) => s.toUpperCase().replace(/\s/g, '');
		verifyError = normalize(verifyInput) !== normalize(recoveryCode);
		if (!verifyError && verifyInput.trim() !== '') {
			verifyPassed = true;
		}
	}

	async function completeSetup() {
		if (!verifyPassed || !pendingMasterKey || !pendingAccountId || !pendingCredentialId) return;
		auth.setSession(pendingMasterKey, pendingAccountId, pendingCredentialId);
		step = 'success';

		// Always create the identity keypair at registration so admins can grant
		// workspace keys immediately without waiting for the user to create a workspace.
		try {
			await ensureIdentityKey(pendingMasterKey);
		} catch { /* non-fatal */ }

		const inviteToken = page.url.searchParams.get('invite');
		if (inviteToken) {
			try {
				await acceptInvitation(inviteToken);
			} catch { /* non-fatal */ }
			setTimeout(() => goto('/dashboard'), 1500);
		} else {
			setTimeout(() => goto('/dashboard'), 1500);
		}
	}

	function copyCode() {
		navigator.clipboard.writeText(recoveryCode);
	}
</script>

<svelte:head>
	<title>Confide — Create Account</title>
</svelte:head>

<div class="font-mono max-w-[560px] mx-auto mt-[60px] px-6">
	<h1 class="text-2xl mb-8">Create your Confide account</h1>

	<!-- Step: checking / PRF error -->
	{#if step === 'checking'}
		{#if prfError}
			<div class="p-5 border border-[#991b1b] rounded-md bg-[#1c0a0a] text-error-muted text-sm">
				<strong>Unsupported browser or device</strong>
				<p class="mt-2 text-error-muted">{prfError}</p>
			</div>
		{:else}
			<p class="text-[#888]">Checking browser compatibility…</p>
		{/if}

	<!-- Step: briefing (mandatory scroll-through) -->
	{:else if step === 'briefing'}
		<div
			bind:this={briefingRef}
			class="h-[360px] overflow-y-scroll border border-border rounded-md p-5 mb-5 bg-[#0d0d0d]"
		>
			<h2 class="text-base mt-0 text-text">Before you continue</h2>
			<p class="text-muted text-sm leading-relaxed">
				Confide encrypts your data in your browser before it ever leaves your device.
				Your passkey (Touch ID, Face ID, or Windows Hello) is used to derive the encryption key —
				<strong>the server never sees your key.</strong>
			</p>
			<h3 class="text-sm text-text mt-5">Your recovery code is your backup</h3>
			<p class="text-muted text-sm leading-relaxed">
				After signup, you will receive a recovery code. This code is the
				<strong>only way to recover your account</strong> if you lose your device.
			</p>
			<ul class="text-muted text-sm leading-[1.8] pl-5">
				<li>Store it somewhere safe (password manager, printed paper).</li>
				<li>Never share it — anyone with this code can access your account.</li>
				<li>You cannot recover your account without it.</li>
			</ul>
			<h3 class="text-sm text-text mt-5">What Confide cannot do</h3>
			<p class="text-muted text-sm leading-relaxed">
				Because encryption happens entirely in your browser, Confide staff
				<strong>cannot read your data, reset your password, or recover your account</strong>
				for you. If you lose your passkey device and your recovery code, your data is unrecoverable.
			</p>
			<p class="text-muted-dark text-xs mt-6 italic">Scroll to the bottom to continue.</p>
			<div bind:this={sentinelRef} class="h-px"></div>
		</div>

		<button
			onclick={() => (step = 'creating')}
			disabled={!briefingScrolled}
			class="w-full py-3.5 border-none rounded-md font-mono text-base
				{briefingScrolled
					? 'bg-primary text-white cursor-pointer hover:bg-primary-hover'
					: 'bg-[#1e3a5f] text-[#4b6583] cursor-not-allowed'}"
		>
			I understand — continue
		</button>

	<!-- Step: creating passkey -->
	{:else if step === 'creating'}
		<div class="p-6 border border-border rounded-md bg-[#0d0d0d] mb-6">
			<p class="text-muted text-sm m-0 mb-4">
				Choose a username, then your browser will prompt you to create a passkey.
			</p>
			<label class="block text-muted text-sm mb-1.5">Username</label>
			<input
				type="text"
				bind:value={username}
				placeholder="e.g. alice"
				disabled={loading}
				class="input-base mb-4 text-sm py-2.5 px-3"
			/>
			{#if registerError}
				<div class="text-error-muted text-sm mb-3">{registerError}</div>
			{/if}
			<button
				onclick={startRegistration}
				disabled={loading}
				class="w-full py-3.5 text-white border-none rounded-md font-mono text-base
					{loading ? 'bg-[#555] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
			>
				{loading ? 'Creating passkey…' : 'Create passkey'}
			</button>
		</div>

	<!-- Step: recovery code -->
	{:else if step === 'recovery'}
		<div class="mb-6">
			<h2 class="text-base text-text mb-2">Save your recovery code</h2>
			<p class="text-[#f59e0b] text-sm mb-5">
				This is the only way to recover your account. Save it now — you will not see it again.
			</p>

			<div class="p-4 px-5 bg-[#111] border border-border rounded-md text-sm text-text break-all tracking-[0.05em] mb-3">
				{recoveryCode}
			</div>

			<button
				onclick={copyCode}
				class="px-4 py-2 bg-surface-2 text-muted border border-border rounded cursor-pointer font-mono text-xs mb-8 hover:text-text transition-colors duration-100"
			>
				Copy code
			</button>

			<h3 class="text-sm text-text mb-2">Confirm you've saved it</h3>
			<p class="text-muted text-xs mb-3">Paste your recovery code below to continue.</p>

			<input
				type="text"
				bind:value={verifyInput}
				oninput={checkVerification}
				placeholder="GHRK-XXXX-XXXX-…"
				class="input-base mb-1 text-sm py-2.5 px-3
					{verifyError ? '!border-[#991b1b]' : ''}"
			/>
			{#if verifyError}
				<span class="text-error-muted text-xs block mb-2">
					Does not match — check what you pasted
				</span>
			{/if}

			<button
				onclick={completeSetup}
				disabled={!verifyPassed}
				class="w-full py-3.5 mt-3 border-none rounded-md font-mono text-base
					{verifyPassed
						? 'bg-[#166534] text-[#bbf7d0] cursor-pointer'
						: 'bg-surface-2 text-muted-dark cursor-not-allowed'}"
			>
				Complete setup
			</button>
		</div>

	<!-- Step: success -->
	{:else if step === 'success'}
		<div class="p-6 border border-[#166534] rounded-md bg-[#052e16] text-[#bbf7d0] text-sm text-center">
			Account created. Redirecting…
		</div>
	{/if}
</div>
