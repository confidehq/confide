<script lang="ts">
import { goto } from "$app/navigation";
import { page } from "$app/state";
import { untrack } from "svelte";
import faviconSvg from "$lib/assets/favicon.svg?raw";
import { isPasskeyCancelled, login, loginWithAutofill, PrfNotSupportedError } from "$lib/auth";
import { getAppConfig } from "$lib/config";
import { auth } from "$lib/stores/auth.svelte";
import { ensureIdentityKey, setupPersonalWorkspaceKey } from "$lib/workspaces";
import type { LoginResult } from "$lib/auth";

let error = $state<string | null>(null);
let loading = $state(false);
let username = $state("");
let registrationOpen = $state(true);

$effect(() => {
	getAppConfig()
		.then((c) => {
			registrationOpen = c.registrationOpen;
		})
		.catch(() => {});
});

const next = $derived(page.url.searchParams.get("next") ?? "/dashboard");

// Track whether a login has already completed so parallel paths don't race.
let settled = false;
let autofillStarted = false;

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
	autofillStarted = false;

	// Read credentialId outside reactive tracking so this effect doesn't
	// re-run when auth.setSession() updates it after a successful login.
	const credentialId = untrack(() => auth.credentialId);
	if (credentialId) {
		// Returning user on a known device — auto-trigger the passkey prompt
		// immediately so they don't have to click anything.
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

// For new devices (no stored credentialId), start conditional UI on first
// focus of the username field. Waiting for a user gesture prevents 1Password
// from auto-triggering the ceremony before the user has interacted.
function startAutofill() {
	if (autofillStarted || untrack(() => auth.credentialId)) return;
	autofillStarted = true;
	loginWithAutofill()
		.then(handleLoginResult)
		.catch((err) => {
			if (settled) return;
			handleLoginError(err);
		});
}

async function handleLogin() {
	settled = true; // suppress any concurrent autofill result
	error = null;
	loading = true;
	try {
		const result = await login(auth.credentialId, username.trim() || undefined);
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
	<div class="w-full max-w-[360px]">

		<!-- Logo + heading -->
		<div class="flex flex-col items-center mb-8">
			<a href="https://useconfide.app" class="w-14 h-14 mb-1 [&>svg]:w-full [&>svg]:h-full block">{@html faviconSvg}</a>
			<h1 class="text-xl font-semibold text-text tracking-tight">Sign in to Confide</h1>
			<p class="text-sm text-subtle mt-1.5">Use your passkey to continue.</p>
		</div>

		<!-- Form card -->
		<div class="bg-canvas border border-border rounded-xl p-6">
			<label class="block text-sm text-subtle mb-1.5" for="username">Username</label>
			<input
				id="username"
				type="text"
				bind:value={username}
				placeholder="your username"
				disabled={loading}
				autocomplete="username webauthn"
				onfocus={startAutofill}
				class="input-base w-full mb-4 text-sm py-2.5 px-3"
			/>

			<button
				onclick={handleLogin}
				disabled={loading}
				class="w-full py-3 text-white border-none rounded-lg font-mono text-sm font-medium
					{loading ? 'bg-muted cursor-not-allowed' : 'bg-primary hover:bg-primary-hover cursor-pointer'}
					transition-colors duration-100"
			>
				{loading ? 'Authenticating…' : 'Sign in with passkey'}
			</button>

			{#if error}
				<p class="text-danger text-xs mt-3 text-center">{error}</p>
			{/if}
		</div>

		<!-- Recovery link -->
		<p class="text-xs text-subtle text-center mt-4">
			Lost your passkey?
			<a href="/recover" class="text-text hover:underline">Recover your account</a>
		</p>

		<!-- Sign up -->
		{#if registrationOpen}
		<div class="mt-6 pt-5 border-t border-border text-center">
			<p class="text-sm text-subtle">
				Don't have an account?
				<a href="/signup" class="text-text hover:underline font-medium">Sign up</a>
			</p>
		</div>
		{/if}

	</div>
</div>
