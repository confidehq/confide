<script lang="ts">
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { renameWorkspace, WorkspaceError } from '$lib/workspaces';
	import { Settings, BarChart2, Building2, Mail } from '@lucide/svelte';

	type Tab = 'usage' | 'workspace' | 'smtp';
	let activeTab = $state<Tab>('usage');

	// ─── Workspace tab state ────────────────────────────────────────────────────
	let workspaceName = $state('');
	let workspaceSaving = $state(false);
	let workspaceSaved = $state(false);
	let workspaceSaveError = $state('');

	$effect(() => {
		if (workspacesStore.active) {
			workspaceName = workspacesStore.active.name;
		}
	});

	async function saveWorkspace() {
		const ws = workspacesStore.active;
		if (!ws) return;
		workspaceSaving = true;
		workspaceSaved = false;
		workspaceSaveError = '';
		try {
			await renameWorkspace(ws.id, workspaceName.trim());
			workspacesStore.update(ws.id, { name: workspaceName.trim() });
			workspaceSaved = true;
			setTimeout(() => (workspaceSaved = false), 2000);
		} catch (e) {
			workspaceSaveError = e instanceof WorkspaceError ? e.message : 'Failed to save';
		} finally {
			workspaceSaving = false;
		}
	}

	// ─── SMTP tab state ─────────────────────────────────────────────────────────
	let smtpHost = $state('');
	let smtpPort = $state('587');
	let smtpUser = $state('');
	let smtpPass = $state('');
	let smtpFrom = $state('');
	let smtpSaving = $state(false);
	let smtpSaved = $state(false);

	async function saveSMTP() {
		smtpSaving = true;
		smtpSaved = false;
		try {
			// TODO: wire to API
			await new Promise(r => setTimeout(r, 600));
			smtpSaved = true;
			setTimeout(() => (smtpSaved = false), 2000);
		} finally {
			smtpSaving = false;
		}
	}

	const tabs: { id: Tab; label: string; icon: typeof Settings; disabled?: boolean }[] = [
		{ id: 'usage',     label: 'Usage',     icon: BarChart2                },
		{ id: 'workspace', label: 'Workspace',  icon: Building2,							},
		{ id: 'smtp',      label: 'SMTP',       icon: Mail,      disabled: true },
	];

	const planBadge: Record<string, { label: string; color: string }> = {
		pro:  { label: 'Pro',  color: 'var(--color-warning-border)' },
		free: { label: 'Free', color: 'var(--color-muted-dim)' },
	};
</script>

<svelte:head>
	<title>Confide — Settings</title>
</svelte:head>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="mb-8">
		<h1 class="text-2xl m-0 mb-1 text-text-bright font-semibold">Settings</h1>
		{#if workspacesStore.active}
			<p class="m-0 text-sm text-muted-dim flex items-center gap-1.5">
				<span>{workspacesStore.active.name}</span>
				<span class="text-border-mid">·</span>
				<span class="capitalize
					{workspacesStore.active.plan === 'pro' && workspacesStore.active.planStatus === 'active'
						? 'text-success-text-dark'
						: 'text-muted-dim'}">
					{workspacesStore.active.plan}
				</span>
			</p>
		{:else if workspacesStore.loading}
			<p class="m-0 text-sm text-muted-mid">Loading…</p>
		{/if}
	</div>

	<!-- Tab bar -->
	<div class="flex border-b border-border-mid mb-8 gap-1">
		{#each tabs as tab}
			{@const active = activeTab === tab.id}
			<button
				onclick={() => !tab.disabled && (activeTab = tab.id)}
				disabled={tab.disabled}
				class="flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 -mb-px
					transition-[color,border-color] duration-100 bg-transparent font-mono
					{tab.disabled
						? 'border-transparent text-muted-mid cursor-not-allowed'
						: active
							? 'border-text-blue text-text-blue cursor-pointer'
							: 'border-transparent text-muted-dim hover:text-muted-blue hover:border-muted-mid cursor-pointer'}"
			>
				<svelte:component this={tab.icon} size={14} strokeWidth={1.75} />
				{tab.label}
			</button>
		{/each}
	</div>

	<!-- ─── Usage tab ─────────────────────────────────────────────────────────── -->
	{#if activeTab === 'usage'}
		<div>
			<div class="grid grid-cols-2 sm:grid-cols-3 gap-2 sm:gap-3 mb-10">
				{#each [
					{ label: 'Forms',     value: '—' },
					{ label: 'Responses', value: '—' },
					{ label: 'Members',   value: '—' },
				] as stat}
					<div class="px-4 py-4 sm:px-5 sm:py-5 border border-border-deep rounded-lg flex flex-col gap-2">
						<p class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">{stat.label}</p>
						<p class="m-0 text-4xl sm:text-5xl text-text-body leading-none tabular-nums">{stat.value}</p>
					</div>
				{/each}
			</div>

			<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Plan</h2>
			<div class="border border-border-deep rounded-lg overflow-hidden">
				{#if workspacesStore.active}
					{@const plan = planBadge[workspacesStore.active.plan] ?? planBadge.free}
					<div class="flex items-center justify-between px-4 py-4">
						<div>
							<p class="m-0 text-base text-text-body">
								Current plan: <span class="font-semibold" style="color: {plan.color};">{plan.label}</span>
							</p>
							<p class="m-0 mt-0.5 text-sm text-muted-dim capitalize">
								Status: {workspacesStore.active.planStatus}
							</p>
						</div>
						{#if workspacesStore.active.plan === 'free'}
							<button
								disabled
								class="px-4 py-2 bg-transparent text-muted-dim border border-border-subtle rounded cursor-not-allowed font-mono text-base"
							>
								Upgrade
							</button>
						{/if}
					</div>
				{:else}
					<div class="px-4 py-4">
						<p class="m-0 text-muted-dim text-base">Loading…</p>
					</div>
				{/if}
			</div>
		</div>

	<!-- ─── Workspace tab ─────────────────────────────────────────────────────── -->
	{:else if activeTab === 'workspace'}
		<div class="max-w-lg flex flex-col gap-6">

			<div class="flex flex-col gap-1.5">
				<label for="ws-name" class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
					Name
				</label>
				<input
					id="ws-name"
					type="text"
					bind:value={workspaceName}
					placeholder="Workspace name"
					class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
						placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
				/>
			</div>

			{#if workspacesStore.active}
				<div class="flex flex-col gap-1.5">
					<p class="m-0 text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">Slug</p>
					<p class="m-0 px-3 py-2.5 border border-border-mid rounded text-base text-muted-dim bg-surface-read select-all">
						{workspacesStore.active.slug}
					</p>
				</div>

				<div class="flex flex-col gap-1.5">
					<p class="m-0 text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">Workspace ID</p>
					<p class="m-0 px-3 py-2.5 border border-border-mid rounded text-base text-muted-dim bg-surface-read select-all break-all">
						{workspacesStore.active.id}
					</p>
				</div>
			{/if}

			<div class="flex items-center gap-3">
				<button
					onclick={saveWorkspace}
					disabled={workspaceSaving || !workspaceName.trim() || workspaceName.trim() === workspacesStore.active?.name}
					class="px-5 py-2.5 bg-primary text-white border-none rounded cursor-pointer font-mono text-base
						hover:bg-primary-hover transition-colors duration-100
						disabled:opacity-40 disabled:cursor-not-allowed"
				>
					{#if workspaceSaving}
						Saving…
					{:else if workspaceSaved}
						Saved
					{:else}
						Save changes
					{/if}
				</button>
				{#if workspaceSaveError}
					<span class="text-sm text-error-light">{workspaceSaveError}</span>
				{/if}
			</div>

			<!-- Danger zone -->
			<div class="mt-4">
				<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Danger zone</h2>
				<div class="border border-border-danger-deep rounded-lg px-4 py-4 flex items-center justify-between gap-4">
					<div>
						<p class="m-0 text-base text-text-body">Delete workspace</p>
						<p class="m-0 mt-0.5 text-sm text-muted-dim">Permanently delete this workspace and all its data.</p>
					</div>
					<button
						disabled
						class="shrink-0 px-4 py-2 bg-transparent text-error-light border border-border-danger-dark rounded
							cursor-not-allowed font-mono text-base opacity-50"
					>
						Delete
					</button>
				</div>
			</div>
		</div>

	<!-- ─── SMTP tab ──────────────────────────────────────────────────────────── -->
	{:else if activeTab === 'smtp'}
		<div class="max-w-lg flex flex-col gap-6">
			<p class="m-0 text-sm text-muted-dim">
				Configure a custom SMTP server to send notification emails from your own domain.
			</p>

			<div class="grid grid-cols-1 sm:grid-cols-[1fr_120px] gap-4">
				<div class="flex flex-col gap-1.5">
					<label for="smtp-host" class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
						Host
					</label>
					<input
						id="smtp-host"
						type="text"
						bind:value={smtpHost}
						placeholder="smtp.example.com"
						class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
							placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
					/>
				</div>

				<div class="flex flex-col gap-1.5">
					<label for="smtp-port" class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
						Port
					</label>
					<input
						id="smtp-port"
						type="text"
						bind:value={smtpPort}
						placeholder="587"
						class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
							placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="smtp-user" class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
					Username
				</label>
				<input
					id="smtp-user"
					type="text"
					bind:value={smtpUser}
					placeholder="user@example.com"
					class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
						placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="smtp-pass" class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
					Password
				</label>
				<input
					id="smtp-pass"
					type="password"
					bind:value={smtpPass}
					placeholder="••••••••"
					class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
						placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="smtp-from" class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">
					From address
				</label>
				<input
					id="smtp-from"
					type="text"
					bind:value={smtpFrom}
					placeholder="noreply@example.com"
					class="font-mono bg-surface-input border border-border-subtle rounded px-3 py-2.5 text-base text-text-body
						placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100"
				/>
			</div>

			<div class="flex items-center gap-3">
				<button
					onclick={saveSMTP}
					disabled={smtpSaving || !smtpHost.trim()}
					class="px-5 py-2.5 bg-primary text-white border-none rounded cursor-pointer font-mono text-base
						hover:bg-primary-hover transition-colors duration-100
						disabled:opacity-40 disabled:cursor-not-allowed"
				>
					{#if smtpSaving}
						Saving…
					{:else if smtpSaved}
						Saved
					{:else}
						Save SMTP
					{/if}
				</button>
				<button
					disabled
					class="px-4 py-2.5 bg-transparent text-muted-dim border border-border-subtle rounded
						cursor-not-allowed font-mono text-base opacity-50"
				>
					Send test email
				</button>
			</div>
		</div>
	{/if}

</div>
</div>
