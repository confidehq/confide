<script lang="ts">
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { Settings, BarChart2, Building2, Mail } from '@lucide/svelte';

	type Tab = 'usage' | 'workspace' | 'smtp';
	let activeTab = $state<Tab>('usage');

	// ─── Workspace tab state ────────────────────────────────────────────────────
	let workspaceName = $state('');
	let workspaceSaving = $state(false);
	let workspaceSaved = $state(false);

	$effect(() => {
		if (workspacesStore.active) {
			workspaceName = workspacesStore.active.name;
		}
	});

	async function saveWorkspace() {
		workspaceSaving = true;
		workspaceSaved = false;
		try {
			// TODO: wire to API
			await new Promise(r => setTimeout(r, 600));
			workspaceSaved = true;
			setTimeout(() => (workspaceSaved = false), 2000);
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
		{ id: 'workspace', label: 'Workspace',  icon: Building2, disabled: true },
		{ id: 'smtp',      label: 'SMTP',       icon: Mail,      disabled: true },
	];

	const planBadge: Record<string, { label: string; color: string }> = {
		pro:  { label: 'Pro',  color: '#f59e0b' },
		free: { label: 'Free', color: '#4b6280' },
	};
</script>

<svelte:head>
	<title>Confide — Settings</title>
</svelte:head>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="mb-8">
		<h1 class="text-2xl m-0 mb-1 text-[#e2e8f0] font-semibold">Settings</h1>
		{#if workspacesStore.active}
			<p class="m-0 text-sm text-[#4b6280] flex items-center gap-1.5">
				<span>{workspacesStore.active.name}</span>
				<span class="text-[#1e3347]">·</span>
				<span class="capitalize
					{workspacesStore.active.plan === 'pro' && workspacesStore.active.planStatus === 'active'
						? 'text-[#4ade80]'
						: 'text-[#4b6280]'}">
					{workspacesStore.active.plan}
				</span>
			</p>
		{:else if workspacesStore.loading}
			<p class="m-0 text-sm text-[#374d63]">Loading…</p>
		{/if}
	</div>

	<!-- Tab bar -->
	<div class="flex border-b border-[#1e3347] mb-8 gap-1">
		{#each tabs as tab}
			{@const active = activeTab === tab.id}
			<button
				onclick={() => !tab.disabled && (activeTab = tab.id)}
				disabled={tab.disabled}
				class="flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 -mb-px
					transition-[color,border-color] duration-100 bg-transparent font-mono
					{tab.disabled
						? 'border-transparent text-[#2a3a4a] cursor-not-allowed'
						: active
							? 'border-[#93c5fd] text-[#93c5fd] cursor-pointer'
							: 'border-transparent text-[#4b6280] hover:text-[#8899aa] hover:border-[#2a3a4a] cursor-pointer'}"
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
						<p class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-[#374d63]">{stat.label}</p>
						<p class="m-0 text-4xl sm:text-5xl text-[#c5d3e0] leading-none tabular-nums">{stat.value}</p>
					</div>
				{/each}
			</div>

			<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-[#374d63]">Plan</h2>
			<div class="border border-border-deep rounded-lg overflow-hidden">
				{#if workspacesStore.active}
					{@const plan = planBadge[workspacesStore.active.plan] ?? planBadge.free}
					<div class="flex items-center justify-between px-4 py-4">
						<div>
							<p class="m-0 text-base text-[#c5d3e0]">
								Current plan: <span class="font-semibold" style="color: {plan.color};">{plan.label}</span>
							</p>
							<p class="m-0 mt-0.5 text-sm text-[#4b6280] capitalize">
								Status: {workspacesStore.active.planStatus}
							</p>
						</div>
						{#if workspacesStore.active.plan === 'free'}
							<button
								disabled
								class="px-4 py-2 bg-transparent text-[#4b6280] border border-border-subtle rounded cursor-not-allowed font-mono text-base"
							>
								Upgrade
							</button>
						{/if}
					</div>
				{:else}
					<div class="px-4 py-4">
						<p class="m-0 text-[#4b6280] text-base">Loading…</p>
					</div>
				{/if}
			</div>
		</div>

	<!-- ─── Workspace tab ─────────────────────────────────────────────────────── -->
	{:else if activeTab === 'workspace'}
		<div class="max-w-lg flex flex-col gap-6">

			<div class="flex flex-col gap-1.5">
				<label for="ws-name" class="text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">
					Name
				</label>
				<input
					id="ws-name"
					type="text"
					bind:value={workspaceName}
					placeholder="Workspace name"
					class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-2.5 text-base text-[#c5d3e0]
						placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070] transition-colors duration-100"
				/>
			</div>

			{#if workspacesStore.active}
				<div class="flex flex-col gap-1.5">
					<p class="m-0 text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">Slug</p>
					<p class="m-0 px-3 py-2.5 border border-[#1e3347] rounded text-base text-[#4b6280] bg-[#080f18] select-all">
						{workspacesStore.active.slug}
					</p>
				</div>

				<div class="flex flex-col gap-1.5">
					<p class="m-0 text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">Workspace ID</p>
					<p class="m-0 px-3 py-2.5 border border-[#1e3347] rounded text-base text-[#4b6280] bg-[#080f18] select-all break-all">
						{workspacesStore.active.id}
					</p>
				</div>
			{/if}

			<div>
				<button
					onclick={saveWorkspace}
					disabled={workspaceSaving || !workspaceName.trim()}
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
			</div>

			<!-- Danger zone -->
			<div class="mt-4">
				<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-[#374d63]">Danger zone</h2>
				<div class="border border-[#3d1414] rounded-lg px-4 py-4 flex items-center justify-between gap-4">
					<div>
						<p class="m-0 text-base text-[#c5d3e0]">Delete workspace</p>
						<p class="m-0 mt-0.5 text-sm text-[#4b6280]">Permanently delete this workspace and all its data.</p>
					</div>
					<button
						disabled
						class="shrink-0 px-4 py-2 bg-transparent text-[#f87171] border border-[#7f1d1d] rounded
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
			<p class="m-0 text-sm text-[#4b6280]">
				Configure a custom SMTP server to send notification emails from your own domain.
			</p>

			<div class="grid grid-cols-1 sm:grid-cols-[1fr_120px] gap-4">
				<div class="flex flex-col gap-1.5">
					<label for="smtp-host" class="text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">
						Host
					</label>
					<input
						id="smtp-host"
						type="text"
						bind:value={smtpHost}
						placeholder="smtp.example.com"
						class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-2.5 text-base text-[#c5d3e0]
							placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070] transition-colors duration-100"
					/>
				</div>

				<div class="flex flex-col gap-1.5">
					<label for="smtp-port" class="text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">
						Port
					</label>
					<input
						id="smtp-port"
						type="text"
						bind:value={smtpPort}
						placeholder="587"
						class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-2.5 text-base text-[#c5d3e0]
							placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070] transition-colors duration-100"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="smtp-user" class="text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">
					Username
				</label>
				<input
					id="smtp-user"
					type="text"
					bind:value={smtpUser}
					placeholder="user@example.com"
					class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-2.5 text-base text-[#c5d3e0]
						placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070] transition-colors duration-100"
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="smtp-pass" class="text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">
					Password
				</label>
				<input
					id="smtp-pass"
					type="password"
					bind:value={smtpPass}
					placeholder="••••••••"
					class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-2.5 text-base text-[#c5d3e0]
						placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070] transition-colors duration-100"
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="smtp-from" class="text-sm font-semibold tracking-[0.08em] uppercase text-[#374d63]">
					From address
				</label>
				<input
					id="smtp-from"
					type="text"
					bind:value={smtpFrom}
					placeholder="noreply@example.com"
					class="font-mono bg-[#0d1520] border border-border-subtle rounded px-3 py-2.5 text-base text-[#c5d3e0]
						placeholder-[#4b6280] focus:outline-none focus:border-[#3a5070] transition-colors duration-100"
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
					class="px-4 py-2.5 bg-transparent text-[#4b6280] border border-border-subtle rounded
						cursor-not-allowed font-mono text-base opacity-50"
				>
					Send test email
				</button>
			</div>
		</div>
	{/if}

</div>
</div>
