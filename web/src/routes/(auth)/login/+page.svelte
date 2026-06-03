<script lang="ts">
import { goto } from "$app/navigation";
import { page } from "$app/state";
import { untrack } from "svelte";
import { Fingerprint } from "@lucide/svelte";
import faviconSvg from "$lib/assets/favicon.svg?raw";
import { isPasskeyCancelled, login, PrfNotSupportedError } from "$lib/auth";
import { getAppConfig } from "$lib/config";
import { auth } from "$lib/stores/auth.svelte";
import { ensureIdentityKey, setupPersonalWorkspaceKey } from "$lib/workspaces";
import type { LoginResult } from "$lib/auth";

let error = $state<string | null>(null);
let loading = $state(false);
let registrationOpen = $state(true);

$effect(() => {
	getAppConfig()
		.then((c) => {
			registrationOpen = c.registrationOpen;
		})
		.catch(() => {});
});

const next = $derived(page.url.searchParams.get("next") ?? "/dashboard");

let settled = false;

async function handleLoginResult(result: LoginResult) {
	if (settled) return;
	settled = true;
	auth.setSession(result.masterKey, result.accountId, result.credentialId);
	try {
		await ensureIdentityKey(result.masterKey);
		await setupPersonalWorkspaceKey(result.masterKey, result.accountId);
	} catch (err) {
		console.error("Post-login setup failed:", err);
		error = "Signed in, but workspace setup encountered an error. Try refreshing if things look wrong.";
	}
	goto(next);
}

function handleLoginError(err: unknown) {
	if (settled) return;
	if (!isPasskeyCancelled(err)) {
		if (err instanceof PrfNotSupportedError) {
			error = "Your browser or device doesn't support passkeys. Try Chrome, Edge, or Safari, or recover your account below.";
		} else {
			error = err instanceof Error ? err.message : "Login failed.";
		}
	}
}

$effect(() => {
	settled = false;

	const credentialId = untrack(() => auth.credentialId);
	if (credentialId) {
		loading = true;
		login(credentialId)
			.then(handleLoginResult)
			.catch((err) => {
				handleLoginError(err);
				loading = false;
			});
	}

	return () => {
		settled = true;
	};
});

async function handleLogin() {
	settled = true;
	error = null;
	loading = true;
	try {
		const result = await login(auth.credentialId);
		settled = false;
		await handleLoginResult(result);
	} catch (err) {
		settled = false;
		handleLoginError(err);
	} finally {
		loading = false;
	}
}
</script>

<svelte:head>
	<title>Confide — Sign In</title>
</svelte:head>

<div class="min-h-screen flex flex-col items-center justify-center px-4 font-mono">
	<div class="w-full max-w-[320px] flex flex-col items-center">

		<!-- Logo + heading -->
		<a href="https://useconfide.app" class="w-12 h-12 mb-6 [&>svg]:w-full [&>svg]:h-full block">{@html faviconSvg}</a>
		<h1 class="text-xl font-semibold text-text tracking-tight mb-1">Sign in to Confide</h1>
		<p class="text-sm text-subtle mb-10">Touch your passkey to continue.</p>

		<!-- Required by WebAuthn conditional UI spec for passkey autofill discovery -->
		<input type="text" autocomplete="username webauthn" aria-hidden="true" tabindex="-1" class="sr-only" />

		<!-- Fingerprint button -->
		<button
			onclick={handleLogin}
			disabled={loading}
			aria-label="Sign in with passkey"
			class="group relative flex items-center justify-center w-24 h-24 rounded-full border-2 mb-8
				{loading
					? 'border-border bg-surface cursor-not-allowed'
					: 'border-primary bg-primary/5 hover:bg-primary/10 cursor-pointer'}
				transition-colors duration-150"
		>
			{#if loading}
				<svg class="w-8 h-8 text-subtle animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
				</svg>
			{:else}
				<Fingerprint
					class="w-12 h-12 text-primary transition-transform duration-150 group-hover:scale-105"
					strokeWidth={1.25}
				/>
			{/if}
		</button>

		<p class="text-xs text-subtle mb-1">
			{loading ? 'Authenticating…' : 'Tap to authenticate'}
		</p>

		{#if error}
			<p class="text-danger text-xs mt-3 text-center max-w-[260px]">{error}</p>
		{/if}

		<!-- Recovery link -->
		<p class="text-xs text-subtle text-center mt-8">
			Lost your passkey?
			<a href="/recover" class="text-text hover:underline">Recover your account</a>
		</p>

		<!-- Sign up -->
		{#if registrationOpen}
		<div class="mt-6 pt-5 border-t border-border w-full text-center">
			<p class="text-sm text-subtle">
				Don't have an account?
				<a href="/signup" class="text-text hover:underline font-medium">Sign up</a>
			</p>
		</div>
		{/if}

	</div>
</div>
