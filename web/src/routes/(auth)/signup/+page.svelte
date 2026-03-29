<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { detectPRFSupport } from '$lib/prf-detection';
	import { register } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';

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

	function completeSetup() {
		if (!verifyPassed || !pendingMasterKey || !pendingAccountId || !pendingCredentialId) return;
		auth.setSession(pendingMasterKey, pendingAccountId, pendingCredentialId);
		step = 'success';
		setTimeout(() => goto('/dashboard'), 1500);
	}

	function copyCode() {
		navigator.clipboard.writeText(recoveryCode);
	}
</script>

<svelte:head>
	<title>Confide — Create Account</title>
</svelte:head>

<div style="font-family: monospace; max-width: 560px; margin: 60px auto; padding: 0 24px;">
	<h1 style="font-size: 1.4rem; margin-bottom: 32px;">Create your Confide account</h1>

	<!-- Step: checking / PRF error -->
	{#if step === 'checking'}
		{#if prfError}
			<div style="
				padding: 20px;
				border: 1px solid #991b1b;
				border-radius: 6px;
				background: #1c0a0a;
				color: #fca5a5;
				font-size: 0.9rem;
			">
				<strong>Unsupported browser or device</strong>
				<p style="margin-top: 8px; color: #fca5a5;">{prfError}</p>
			</div>
		{:else}
			<p style="color: #888;">Checking browser compatibility…</p>
		{/if}

	<!-- Step: briefing (mandatory scroll-through) -->
	{:else if step === 'briefing'}
		<div
			bind:this={briefingRef}
			style="
				height: 360px;
				overflow-y: scroll;
				border: 1px solid #374151;
				border-radius: 6px;
				padding: 20px;
				margin-bottom: 20px;
				background: #0d0d0d;
			"
		>
			<h2 style="font-size: 1rem; margin-top: 0; color: #e5e7eb;">Before you continue</h2>
			<p style="color: #9ca3af; font-size: 0.85rem; line-height: 1.6;">
				Confide encrypts your data in your browser before it ever leaves your device.
				Your passkey (Touch ID, Face ID, or Windows Hello) is used to derive the encryption key —
				<strong>the server never sees your key.</strong>
			</p>
			<h3 style="font-size: 0.9rem; color: #e5e7eb; margin-top: 20px;">Your recovery code is your backup</h3>
			<p style="color: #9ca3af; font-size: 0.85rem; line-height: 1.6;">
				After signup, you will receive a recovery code. This code is the
				<strong>only way to recover your account</strong> if you lose your device.
			</p>
			<ul style="color: #9ca3af; font-size: 0.85rem; line-height: 1.8; padding-left: 20px;">
				<li>Store it somewhere safe (password manager, printed paper).</li>
				<li>Never share it — anyone with this code can access your account.</li>
				<li>You cannot recover your account without it.</li>
			</ul>
			<h3 style="font-size: 0.9rem; color: #e5e7eb; margin-top: 20px;">What Confide cannot do</h3>
			<p style="color: #9ca3af; font-size: 0.85rem; line-height: 1.6;">
				Because encryption happens entirely in your browser, Confide staff
				<strong>cannot read your data, reset your password, or recover your account</strong>
				for you. If you lose your passkey device and your recovery code, your data is unrecoverable.
			</p>
			<p style="color: #6b7280; font-size: 0.8rem; margin-top: 24px; font-style: italic;">
				Scroll to the bottom to continue.
			</p>
			<div bind:this={sentinelRef} style="height: 1px;"></div>
		</div>

		<button
			onclick={() => (step = 'creating')}
			disabled={!briefingScrolled}
			style="
				width: 100%;
				padding: 14px;
				background: {briefingScrolled ? '#2563eb' : '#1e3a5f'};
				color: {briefingScrolled ? 'white' : '#4b6583'};
				border: none;
				border-radius: 6px;
				cursor: {briefingScrolled ? 'pointer' : 'not-allowed'};
				font-family: monospace;
				font-size: 1rem;
			"
		>
			I understand — continue
		</button>

	<!-- Step: creating passkey -->
	{:else if step === 'creating'}
		<div style="
			padding: 24px;
			border: 1px solid #374151;
			border-radius: 6px;
			background: #0d0d0d;
			margin-bottom: 24px;
		">
			<p style="color: #9ca3af; font-size: 0.9rem; margin: 0 0 16px;">
				Choose a username, then your browser will prompt you to create a passkey.
			</p>
			<label style="display: block; color: #9ca3af; font-size: 0.85rem; margin-bottom: 6px;">
				Username
			</label>
			<input
				type="text"
				bind:value={username}
				placeholder="e.g. alice"
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
			{#if registerError}
				<div style="color: #fca5a5; font-size: 0.85rem; margin-bottom: 12px;">{registerError}</div>
			{/if}
			<button
				onclick={startRegistration}
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
				{loading ? 'Creating passkey…' : 'Create passkey'}
			</button>
		</div>

	<!-- Step: recovery code -->
	{:else if step === 'recovery'}
		<div style="margin-bottom: 24px;">
			<h2 style="font-size: 1rem; color: #e5e7eb; margin-bottom: 8px;">Save your recovery code</h2>
			<p style="color: #f59e0b; font-size: 0.85rem; margin-bottom: 20px;">
				This is the only way to recover your account. Save it now — you will not see it again.
			</p>

			<div style="
				padding: 16px 20px;
				background: #111;
				border: 1px solid #374151;
				border-radius: 6px;
				font-size: 0.9rem;
				color: #e5e7eb;
				word-break: break-all;
				letter-spacing: 0.05em;
				margin-bottom: 12px;
			">
				{recoveryCode}
			</div>

			<button
				onclick={copyCode}
				style="
					padding: 8px 16px;
					background: #1f2937;
					color: #9ca3af;
					border: 1px solid #374151;
					border-radius: 4px;
					cursor: pointer;
					font-family: monospace;
					font-size: 0.8rem;
					margin-bottom: 32px;
				"
			>
				Copy code
			</button>

			<h3 style="font-size: 0.9rem; color: #e5e7eb; margin-bottom: 8px;">Confirm you've saved it</h3>
			<p style="color: #9ca3af; font-size: 0.8rem; margin-bottom: 12px;">
				Paste your recovery code below to continue.
			</p>

			<input
				type="text"
				bind:value={verifyInput}
				oninput={checkVerification}
				placeholder="GHRK-XXXX-XXXX-…"
				style="
					width: 100%;
					padding: 10px 12px;
					background: #111;
					border: 1px solid {verifyError ? '#991b1b' : '#374151'};
					border-radius: 4px;
					color: #e5e7eb;
					font-family: monospace;
					font-size: 0.85rem;
					box-sizing: border-box;
					margin-bottom: 4px;
				"
			/>
			{#if verifyError}
				<span style="color: #fca5a5; font-size: 0.75rem; display: block; margin-bottom: 8px;">
					Does not match — check what you pasted
				</span>
			{/if}

			<button
				onclick={completeSetup}
				disabled={!verifyPassed}
				style="
					width: 100%;
					padding: 14px;
					margin-top: 12px;
					background: {verifyPassed ? '#166534' : '#1f2937'};
					color: {verifyPassed ? '#bbf7d0' : '#6b7280'};
					border: none;
					border-radius: 6px;
					cursor: {verifyPassed ? 'pointer' : 'not-allowed'};
					font-family: monospace;
					font-size: 1rem;
				"
			>
				Complete setup
			</button>
		</div>

	<!-- Step: success -->
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
			Account created. Redirecting…
		</div>
	{/if}
</div>
