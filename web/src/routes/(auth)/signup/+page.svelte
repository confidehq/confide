<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { detectPRFSupport } from "$lib/prf-detection";
	import { register } from "$lib/auth";
	import { auth } from "$lib/stores/auth.svelte";
	import {
		acceptInvitation,
		ensureIdentityKey,
		setupPersonalWorkspaceKey,
		listWorkspaces,
		renameWorkspace,
	} from "$lib/workspaces";
	import { workspacesStore } from "$lib/stores/workspaces.svelte";
	import { AtSign, X } from "@lucide/svelte";
	import faviconSvg from "$lib/assets/favicon.svg?raw";

	type Step =
		| "checking"
		| "username"
		| "passkey"
		| "recovery"
		| "workspace"
		| "success";

	let step = $state<Step>("checking");
	let prfError = $state<string | null>(null);
	let registerError = $state<string | null>(null);
	let workspaceError = $state<string | null>(null);
	let loading = $state(false);

	// Recovery code state
	let recoveryCode = $state("");
	let verifyInput = $state("");
	let verifyError = $state(false);
	let verifyPassed = $state(false);
	let codeCopied = $state(false);

	// Pending registration result (held until workspace step completes)
	let pendingMasterKey = $state<CryptoKey | null>(null);
	let pendingAccountId = $state<string | null>(null);
	let pendingCredentialId = $state<string | null>(null);

	// Form inputs
	let username = $state("");
	let workspaceName = $state("");
	let agreedToTerms = $state(false);

	// Username availability check
	type UsernameStatus = "idle" | "checking" | "available" | "taken";
	let usernameStatus = $state<UsernameStatus>("idle");

	$effect(() => {
		const value = username.trim();
		if (!value) {
			usernameStatus = "idle";
			return;
		}
		usernameStatus = "checking";
		const timer = setTimeout(async () => {
			try {
				const res = await fetch(
					`/api/auth/check-username?username=${encodeURIComponent(value)}`,
				);
				const data = (await res.json()) as { available: boolean };
				if (username.trim() === value) {
					usernameStatus = data.available ? "available" : "taken";
				}
			} catch {
				if (username.trim() === value) usernameStatus = "idle";
			}
		}, 400);
		return () => clearTimeout(timer);
	});

	// Visible step labels and their index mapping
	const stepLabels = ["Username", "Security", "Recovery", "Workspace"];
	const stepIndex: Record<Step, number> = {
		checking: -1,
		username: 0,
		passkey: 1,
		recovery: 2,
		workspace: 3,
		success: 4,
	};

	onMount(async () => {
		const result = await detectPRFSupport();
		if (!result.supported) {
			prfError = result.reason;
		} else {
			step = "username";
		}
	});

	function continueToPasskey() {
		if (!username.trim()) {
			registerError = "Please enter a username.";
			return;
		}
		if (usernameStatus === "taken") {
			registerError = null;
			return;
		}
		if (!agreedToTerms) {
			registerError = "Please agree to the Terms of Service and Privacy Policy.";
			return;
		}
		registerError = null;
		step = "passkey";
	}

	async function startRegistration() {
		loading = true;
		registerError = null;
		try {
			const result = await register(username.trim());
			recoveryCode = result.recoveryCode;
			pendingMasterKey = result.masterKey;
			pendingAccountId = result.accountId;
			pendingCredentialId = result.credentialId;
			step = "recovery";
		} catch (err) {
			registerError =
				err instanceof Error ? err.message : "Registration failed.";
		} finally {
			loading = false;
		}
	}

	function checkVerification() {
		const normalize = (s: string) => s.toUpperCase().replace(/\s/g, "");
		verifyError = normalize(verifyInput) !== normalize(recoveryCode);
		if (!verifyError && verifyInput.trim() !== "") {
			verifyPassed = true;
		}
	}

	function continueToWorkspace() {
		if (!verifyPassed) return;
		step = "workspace";
	}

	async function finishOnboarding() {
		if (!workspaceName.trim()) {
			workspaceError = "Please enter a workspace name.";
			return;
		}
		if (!pendingMasterKey || !pendingAccountId || !pendingCredentialId) return;

		loading = true;
		workspaceError = null;

		try {
			// Non-fatal: crypto key provisioning (can be healed on next login)
			try {
				await ensureIdentityKey(pendingMasterKey);
				await setupPersonalWorkspaceKey(pendingMasterKey, pendingAccountId);
			} catch {
				/* non-fatal */
			}

			// Required: rename the auto-created personal workspace
			const workspaces = await listWorkspaces();
			const personal = workspaces.find((w) => w.role === "owner");
			if (personal) {
				await renameWorkspace(personal.id, workspaceName.trim());
				workspacesStore.update(personal.id, { name: workspaceName.trim() });
			}

			// Pre-populate the store with the renamed workspace now. The store's
			// _loaded guard prevents the app layout from re-fetching on mount, so
			// the dashboard always reads the already-correct name.
			await workspacesStore.load();

			const inviteToken = page.url.searchParams.get("invite");
			if (inviteToken) {
				try {
					await acceptInvitation(inviteToken);
				} catch {
					/* non-fatal */
				}
			}

			step = "success";
			// Set session last — triggers the layout $effect which redirects to /dashboard
			setTimeout(() => {
				auth.setSession(pendingMasterKey!, pendingAccountId!, pendingCredentialId!);
			}, 1500);
		} catch (err) {
			workspaceError =
				err instanceof Error ? err.message : "Failed to set up workspace.";
		} finally {
			loading = false;
		}
	}

	function copyCode() {
		navigator.clipboard.writeText(recoveryCode);
		codeCopied = true;
		setTimeout(() => (codeCopied = false), 2000);
	}
</script>

<svelte:head>
	<title>Confide — Create Account</title>
</svelte:head>

<div
	class="min-h-screen flex flex-col items-center justify-center px-4 font-mono"
>
	{#if step === "checking"}
		<div class="w-full max-w-100">
			{#if prfError}
				<div
					class="p-5 border border-danger-text rounded-xl bg-danger-dark text-error-muted text-base"
				>
					<strong>Unsupported browser or device</strong>
					<p class="mt-2">{prfError}</p>
				</div>
			{:else}
				<p class="text-muted text-center text-base">
					Checking browser compatibility…
				</p>
			{/if}
		</div>
	{:else if step === "success"}
		<div class="w-full max-w-100">
			<div
				class="p-6 border border-success-text rounded-xl bg-success-bg-deep text-success-text-dark text-base text-center"
			>
				Account created. Redirecting…
			</div>
		</div>
	{:else}
		{@const current = stepIndex[step]}
		<div class="w-full max-w-100">
			<!-- Logo + heading -->
			<div class="flex flex-col items-center mb-8">
				<a href="https://useconfide.app" class="w-14 h-14 mb-1 [&>svg]:w-full [&>svg]:h-full block">
					{@html faviconSvg}
				</a>
				<h1 class="text-xl font-semibold text-text-body tracking-tight">
					Create your account
				</h1>
				<p class="text-base text-muted-dim mt-1.5">
					Set up Confide in just a few steps.
				</p>
			</div>

			<!-- Step indicator -->
			<div class="flex items-start mb-6 px-2">
				{#each stepLabels as label, i}
					{#if i > 0}
						<div
							class="flex-1 h-px mt-3.5 {i <= current
								? 'bg-primary'
								: 'bg-border'}"
						></div>
					{/if}
					<div class="flex flex-col items-center">
						<div
							class="w-7 h-7 rounded-full flex items-center justify-center text-sm font-semibold
							{i === current
								? 'bg-primary text-white'
								: i < current
									? 'bg-primary/20 text-primary'
									: 'bg-surface border border-border text-muted-dark'}"
						>
							{#if i < current}✓{:else}{i + 1}{/if}
						</div>
						<span
							class="text-[10px] mt-1 w-14 text-center leading-tight
							{i === current
								? 'text-text-body'
								: i < current
									? 'text-muted'
									: 'text-muted-dark'}"
						>
							{label}
						</span>
					</div>
				{/each}
			</div>

			<!-- Step card -->
			<div class="bg-surface border border-border rounded-xl p-6">
				{#if step === "username"}
					<h2 class="text-base font-medium text-text-body mb-1">
						Create your username
					</h2>
					<p class="text-sm text-muted-dim mb-5">
						This is your public name across Confide. You’ll use it to access any
						workspace you join or create.
					</p>

					<label class="block text-base text-muted mb-1.5" for="username"
						>Username</label
					>
					<div class="relative">
						<span
							class="absolute left-2 flex items-center justify-center gap-2 top-1/2 -translate-y-1/2 text-muted-mid pointer-events-none"
						>
							<AtSign size={18} strokeWidth={1.75} />
						</span>
						<input
							id="username"
							type="text"
							bind:value={username}
							placeholder="Username"
							onkeydown={(e) => e.key === "Enter" && continueToPasskey()}
							class="input-base w-full mb-1 text-base py-2.5 pl-4 px-3
							{usernameStatus === 'available' ? '!border-success-text' : ''}
							{usernameStatus === 'taken' ? '!border-danger-text' : ''}"
						/>
					</div>

					<div class="min-h-[1.25rem] mb-3">
						{#if usernameStatus === "available"}
							<span class="text-success-text text-sm">Looks good!</span>
						{:else if usernameStatus === "taken"}
							<span class="text-error text-sm">Username taken</span>
						{/if}
					</div>

					<label class="flex items-start gap-2.5 mb-4 cursor-pointer select-none">
						<input
							type="checkbox"
							bind:checked={agreedToTerms}
							class="mt-0.5 shrink-0 accent-primary w-4 h-4 cursor-pointer"
						/>
						<span class="text-sm text-muted-dim leading-snug">
							I agree to the
							<a
								href="https://useconfide.app/terms/"
								target="_blank"
								rel="noopener noreferrer"
								class="text-text-blue hover:underline"
							>Terms of Service</a>
							and
							<a
								href="https://useconfide.app/privacy/"
								target="_blank"
								rel="noopener noreferrer"
								class="text-text-blue hover:underline"
							>Privacy Policy</a>
						</span>
					</label>

					{#if registerError}
						<p class="text-error text-sm mb-3">{registerError}</p>
					{/if}

					<button
						onclick={continueToPasskey}
						class="w-full py-3 text-white border-none rounded-lg font-mono text-base font-medium
							bg-primary hover:bg-primary-hover cursor-pointer transition-colors duration-100"
					>
						Continue
					</button>
				{:else if step === "passkey"}
					<h2 class="text-base font-medium text-text-body mb-1">
						Your data stays yours
					</h2>
					<p class="text-sm text-muted-dim mb-5">
						Confide encrypts your forms and responses in your browser before
						they leave your device.
					</p>

					<p class="text-sm text-muted-dim mb-5">
						Your passkey (Face ID, Touch ID, Windows Hello) unlocks your data —
						we never see your encryption key and cannot access your data.
					</p>

					{#if registerError}
						<p class="text-error text-sm mb-3">{registerError}</p>
					{/if}

					<button
						onclick={startRegistration}
						disabled={loading}
						class="w-full py-3 text-white border-none rounded-lg font-mono text-base font-medium
							{loading
							? 'bg-muted-mid cursor-not-allowed'
							: 'bg-primary hover:bg-primary-hover cursor-pointer'}
							transition-colors duration-100"
					>
						{loading ? "Creating passkey…" : "Create passkey"}
					</button>

					<button
						onclick={() => {
							step = "username";
							registerError = null;
						}}
						class="w-full mt-2 py-2 text-muted-dark text-sm border-none bg-transparent cursor-pointer hover:text-muted transition-colors duration-100"
					>
						← Back
					</button>
				{:else if step === "recovery"}
					<h2 class="text-base font-medium text-text-body mb-1">
						Save your recovery code
					</h2>
					<p class="text-sm text-warning-border mb-4">
						If you lose access to your device, this code is the <strong
							>only way</strong
						> to restore your account.
					</p>
					<p class="text-sm text-warning-border mb-4">
						Store it somewhere safe, offline, and private. We cannot recover it
						for you.
					</p>

					<div
						class="p-4 bg-canvas border border-border rounded-lg text-sm text-text break-all tracking-[0.05em] mb-2 leading-relaxed"
					>
						{recoveryCode}
					</div>

					<button
						onclick={copyCode}
						class="px-3 py-1.5 border rounded cursor-pointer font-mono text-sm mb-5 transition-colors duration-100
							{codeCopied
							? 'bg-success-bg border-success-text text-success-text'
							: 'bg-surface text-muted border-border hover:text-text'}"
					>
						{codeCopied ? "✓ Copied" : "Copy code"}
					</button>

					<label class="block text-sm text-muted mb-1.5" for="verify">
						Paste your recovery code below to confirm you've saved it
					</label>
					<input
						id="verify"
						type="text"
						bind:value={verifyInput}
						oninput={checkVerification}
						placeholder="GHRK-XXXX-XXXX-…"
						class="input-base w-full mb-1 text-base py-2.5 px-3
							{verifyError ? '!border-danger-text' : ''}"
					/>
					{#if verifyError}
						<span class="text-error text-sm block mb-2"
							>Does not match — check what you pasted</span
						>
					{/if}

					<button
						onclick={continueToWorkspace}
						disabled={!verifyPassed}
						class="w-full py-3 mt-3 border-none rounded-lg font-mono text-base font-medium transition-colors duration-100
							{verifyPassed
							? 'bg-primary text-white cursor-pointer hover:bg-primary-hover'
							: 'bg-surface-active text-muted-dark cursor-not-allowed'}"
					>
						Continue
					</button>
				{:else if step === "workspace"}
					<h2 class="text-base font-medium text-text-body mb-1">
						Set up your workspace
					</h2>
					<p class="text-sm text-muted-dim mb-5">
						This is your hub for forms and team collaboration. You can customize
						the name at any time.
					</p>

					<label class="block text-base text-muted mb-1.5" for="workspace-name"
						>Workspace name</label
					>
					<input
						id="workspace-name"
						type="text"
						bind:value={workspaceName}
						placeholder="e.g. Acme Corp"
						disabled={loading}
						onkeydown={(e) => e.key === "Enter" && finishOnboarding()}
						class="input-base w-full mb-4 text-base py-2.5 px-3"
					/>

					{#if workspaceError}
						<p class="text-error text-sm mb-3">{workspaceError}</p>
					{/if}

					<button
						onclick={finishOnboarding}
						disabled={loading}
						class="w-full py-3 text-white border-none rounded-lg font-mono text-base font-medium
							{loading
							? 'bg-muted-mid cursor-not-allowed'
							: 'bg-primary hover:bg-primary-hover cursor-pointer'}
							transition-colors duration-100"
					>
						{loading ? "Setting up…" : "Create workspace"}
					</button>
				{/if}
			</div>

			<!-- Sign in link (username step only) -->
			{#if step === "username"}
				<div class="mt-6 pt-5 border-t border-border text-center">
					<p class="text-base text-muted-dim">
						Already have an account?
						<a href="/login" class="text-text-blue hover:underline font-medium"
							>Sign in</a
						>
					</p>
				</div>
			{/if}
		</div>
	{/if}
</div>
