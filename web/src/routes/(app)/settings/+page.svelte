<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { get } from 'svelte/store';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		renameWorkspace,
		deleteWorkspace,
		leaveWorkspace,
		WorkspaceError,
		getBillingInfo,
		subscribe,
		openBillingPortal,
		getCustomDomain,
		setCustomDomain,
		clearCustomDomain,
		verifyCustomDomain,
		type BillingInfo,
		type CustomDomainInfo
	} from '$lib/workspaces';
	import { Settings, BarChart2, Building2, Mail, CreditCard, Check, ExternalLink, AlertTriangle } from '@lucide/svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	// ─── Tab init from URL ────────────────────────────────────────────────────────

	type Tab = 'usage' | 'billing' | 'workspace' | 'smtp';
	const _urlTab = get(page).url.searchParams.get('tab') as Tab | null;
	const _validTabs: Tab[] = ['usage', 'billing', 'workspace', 'smtp'];
	let activeTab = $state<Tab>(_urlTab && _validTabs.includes(_urlTab) ? _urlTab : 'usage');

	// Show success banner when returning from Stripe checkout
	const _upgraded = get(page).url.searchParams.get('upgraded') === 'true';
	let showUpgradedBanner = $state(_upgraded);
	if (_upgraded) {
		goto('/settings?tab=billing', { replaceState: true });
		setTimeout(() => { showUpgradedBanner = false; }, 6000);
	}

	// ─── Billing info (shared by Usage and Billing tabs) ─────────────────────────

	let billingInfo = $state<BillingInfo | null>(null);
	let billingLoading = $state(false);
	let billingError = $state('');
	let billingLoaded = $state(false);

	async function loadBillingInfo() {
		const ws = workspacesStore.active;
		if (!ws || ws.role !== 'owner' || billingLoaded || billingLoading) return;
		billingLoading = true;
		billingError = '';
		try {
			billingInfo = await getBillingInfo(ws.id);
			billingLoaded = true;
		} catch (e) {
			billingError = e instanceof WorkspaceError ? e.message : 'Failed to load billing info';
		} finally {
			billingLoading = false;
		}
	}

	// Reset billing cache when workspace changes
	let _lastWsId = $state<string | null>(null);
	$effect(() => {
		const wsId = workspacesStore.active?.id ?? null;
		if (wsId !== _lastWsId) {
			_lastWsId = wsId;
			billingLoaded = false;
			billingInfo = null;
			billingError = '';
			customDomain = null;
			domainInput = '';
			domainError = '';
		}
	});

	$effect(() => {
		if (activeTab === 'usage' || activeTab === 'billing') {
			loadBillingInfo();
		}
	});

	// ─── Custom domain state ──────────────────────────────────────────────────────

	let customDomain = $state<CustomDomainInfo | null>(null);
	let domainInput = $state('');
	let domainSaving = $state(false);
	let domainError = $state('');
	let domainRemoving = $state(false);

	function isAdmin() {
		const role = workspacesStore.active?.role;
		return role === 'admin' || role === 'owner';
	}

	async function loadCustomDomain() {
		const ws = workspacesStore.active;
		if (!ws || !isAdmin()) return;
		try {
			customDomain = await getCustomDomain(ws.id);
		} catch {
			// non-critical
		}
	}

	let domainChecking = $state(false);

	async function saveDomain() {
		const ws = workspacesStore.active;
		if (!ws || !domainInput.trim()) return;
		domainSaving = true;
		domainError = '';
		try {
			customDomain = await setCustomDomain(ws.id, domainInput.trim());
			domainInput = '';
		} catch (e) {
			domainError = e instanceof Error ? e.message : 'Failed to save domain';
		} finally {
			domainSaving = false;
		}
	}

	async function removeDomain() {
		const ws = workspacesStore.active;
		if (!ws) return;
		domainRemoving = true;
		domainError = '';
		try {
			await clearCustomDomain(ws.id);
			customDomain = await getCustomDomain(ws.id);
		} catch (e) {
			domainError = e instanceof Error ? e.message : 'Failed to remove domain';
		} finally {
			domainRemoving = false;
		}
	}

	async function checkDomainNow() {
		const ws = workspacesStore.active;
		if (!ws) return;
		domainChecking = true;
		domainError = '';
		try {
			customDomain = await verifyCustomDomain(ws.id);
		} catch (e) {
			domainError = e instanceof Error ? e.message : 'Verification check failed';
		} finally {
			domainChecking = false;
		}
	}

	$effect(() => {
		if (activeTab === 'workspace') {
			loadCustomDomain();
		}
	});

	// ─── Workspace tab state ──────────────────────────────────────────────────────

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

	// ─── Delete workspace state ───────────────────────────────────────────────────

	let showDeleteConfirm = $state(false);
	let deleting = $state(false);
	let deleteError = $state('');

	// ─── Leave workspace state ────────────────────────────────────────────────────

	let showLeaveConfirm = $state(false);
	let leaving = $state(false);
	let leaveError = $state('');

	async function handleLeave() {
		const ws = workspacesStore.active;
		if (!ws || !auth.accountId) return;
		leaving = true;
		leaveError = '';
		try {
			await leaveWorkspace(ws.id, auth.accountId);
			workspacesStore.remove(ws.id);
			await goto('/workspaces');
		} catch (e) {
			leaveError = e instanceof WorkspaceError ? e.message : 'Failed to leave workspace.';
		} finally {
			leaving = false;
		}
	}

	async function handleDelete() {
		const ws = workspacesStore.active;
		if (!ws) return;
		deleting = true;
		deleteError = '';
		try {
			await deleteWorkspace(ws.id);
			workspacesStore.remove(ws.id);
			await goto('/workspaces');
		} catch (e) {
			deleteError = e instanceof WorkspaceError ? e.message : 'Failed to delete workspace.';
		} finally {
			deleting = false;
		}
	}

	// ─── SMTP tab state ───────────────────────────────────────────────────────────

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
			await new Promise(r => setTimeout(r, 600));
			smtpSaved = true;
			setTimeout(() => (smtpSaved = false), 2000);
		} finally {
			smtpSaving = false;
		}
	}

	// ─── Billing tab actions ──────────────────────────────────────────────────────

	let upgrading = $state(false);
	let upgradeError = $state('');
	let portalLoading = $state(false);
	let showDowngradeModal = $state(false);

	async function handleUpgrade() {
		const ws = workspacesStore.active;
		if (!ws) return;
		upgrading = true;
		upgradeError = '';
		try {
			const checkoutUrl = await subscribe(
				ws.id,
				'pro',
				`${window.location.origin}/settings?tab=billing&upgraded=true`,
				`${window.location.origin}/settings?tab=billing`
			);
			window.location.href = checkoutUrl;
		} catch (e) {
			upgradeError = e instanceof WorkspaceError ? e.message : 'Failed to start checkout';
			upgrading = false;
		}
	}

	async function handleOpenPortal() {
		const ws = workspacesStore.active;
		if (!ws) return;
		portalLoading = true;
		try {
			const portalUrl = await openBillingPortal(ws.id, `${window.location.origin}/settings?tab=billing`);
			window.location.href = portalUrl;
		} catch (e) {
			portalLoading = false;
		}
	}

	function handleDowngradeClick() {
		const hasExtraMembers = billingInfo && billingInfo.memberCount > 1;
		const hasExtraWorkspaces = workspacesStore.workspaces.length > 1;
		if (hasExtraMembers || hasExtraWorkspaces) {
			showDowngradeModal = true;
		} else {
			handleOpenPortal();
		}
	}

	// ─── Helpers ──────────────────────────────────────────────────────────────────

	const tabs: { id: Tab; label: string; icon: typeof Settings; disabled?: boolean; ownerOnly?: boolean }[] = [
		{ id: 'usage',     label: 'Usage',     icon: BarChart2                              },
		{ id: 'billing',   label: 'Billing',   icon: CreditCard, ownerOnly: true            },
		{ id: 'workspace', label: 'Workspace',  icon: Building2                              },
		{ id: 'smtp',      label: 'SMTP',       icon: Mail,       disabled: true             },
	];

	const planBadge: Record<string, { label: string; color: string }> = {
		pro:  { label: 'Pro',  color: 'var(--color-warning-border)' },
		org:  { label: 'Org',  color: 'var(--color-text-blue)'      },
		free: { label: 'Free', color: 'var(--color-muted-dim)'      },
	};

	function formatDate(iso: string): string {
		return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
	}

	const proFeatures: { label: string; enabled: boolean }[] = [
		{ label: 'Unlimited workspaces',       enabled: true  },
		{ label: 'Up to 10 members',           enabled: true  },
		{ label: '10,000 responses / month',   enabled: true  },
		{ label: '100,000 stored responses',   enabled: true  },
		{ label: 'Custom domain',              enabled: true  },
		{ label: '5 GB file storage',          enabled: false },
		{ label: 'Remove branding',            enabled: false },
		{ label: 'Form customization',         enabled: false },
		{ label: 'CSV export',                 enabled: false },
	];

	const freeFeatures: { label: string; enabled: boolean }[] = [
		{ label: 'Up to 2 members',          enabled: true },
		{ label: 'Unlimited forms',          enabled: true },
		{ label: '250 responses / month',    enabled: true },
		{ label: '2,000 stored responses',   enabled: true },
		{ label: '100 MB file storage',      enabled: true },
	];
</script>

<svelte:head>
	<title>Confide — Settings</title>
</svelte:head>

{#if workspacesStore.active}
<ConfirmDialog
	open={showDeleteConfirm}
	title="Delete workspace?"
	description={`This will permanently delete "${workspacesStore.active.name}" and all its forms and responses. This cannot be undone.`}
	loading={deleting}
	error={deleteError}
	onconfirm={handleDelete}
	oncancel={() => { showDeleteConfirm = false; deleteError = ''; }}
/>
<ConfirmDialog
	open={showLeaveConfirm}
	title="Leave workspace?"
	description={`You will lose access to "${workspacesStore.active.name}" and all its forms. You can only rejoin if invited again.`}
	loading={leaving}
	error={leaveError}
	onconfirm={handleLeave}
	oncancel={() => { showLeaveConfirm = false; leaveError = ''; }}
/>
{/if}

<!-- Downgrade warning modal -->
{#if showDowngradeModal && billingInfo}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background: var(--color-overlay); backdrop-filter: blur(2px);"
		onclick={(e) => { if (e.target === e.currentTarget) showDowngradeModal = false; }}
		onkeydown={(e) => { if (e.key === 'Escape') showDowngradeModal = false; }}
		role="presentation"
	>
		<div
			class="font-mono w-full max-w-sm flex flex-col gap-5"
			style="background: var(--color-surface-subtle); border: 1px solid var(--color-border-deep); border-radius: 10px; padding: 1.25rem; box-shadow: 0 24px 48px -12px rgba(0,0,0,0.7);"
			role="dialog"
			aria-modal="true"
		>
			<div class="flex items-center gap-2.5">
				<span class="shrink-0 flex items-center justify-center w-7 h-7 rounded-md bg-surface-deep border border-border">
					<AlertTriangle size={14} strokeWidth={1.75} class="text-warning-text" />
				</span>
				<h2 class="m-0 text-base font-semibold text-text-bright">Before you downgrade</h2>
			</div>

			<div class="flex flex-col gap-2 text-sm text-muted-dim leading-relaxed">
				{#if billingInfo.memberCount > 1}
					<p class="m-0">
						You have <span class="text-text-body font-medium">{billingInfo.memberCount} members</span>.
						The Free plan is limited to 2 members — others will lose access when your subscription ends.
					</p>
				{/if}
				{#if workspacesStore.workspaces.length > 1}
					<p class="m-0">
						You have <span class="text-text-body font-medium">{workspacesStore.workspaces.length} workspaces</span>.
						The Free plan is limited to 1 workspace — additional workspaces will become inaccessible.
					</p>
				{/if}
				{#if billingInfo.planPeriodEnd}
					<p class="m-0 text-muted-mid">
						Your Pro access continues until <span class="text-text-body">{formatDate(billingInfo.planPeriodEnd)}</span>.
					</p>
				{/if}
			</div>

			<div class="h-px bg-border-deep"></div>

			<div class="flex gap-2 justify-end">
				<button
					onclick={() => { showDowngradeModal = false; }}
					class="px-4 py-2 bg-transparent text-muted-dim border border-border-deep rounded cursor-pointer
						font-mono text-sm hover:text-text-body hover:border-muted-mid transition-colors duration-100"
				>Cancel</button>
				<button
					onclick={() => { showDowngradeModal = false; handleOpenPortal(); }}
					disabled={portalLoading}
					class="px-4 py-2 border border-border-deep rounded cursor-pointer font-mono text-sm flex items-center gap-1.5
						text-muted-dim bg-transparent hover:text-text-body hover:border-border-subtle transition-colors duration-100
						disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{portalLoading ? 'Opening…' : 'Continue to billing portal'}
					{#if !portalLoading}<ExternalLink size={12} strokeWidth={1.75} />{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

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
			{@const hidden = tab.ownerOnly && workspacesStore.active?.role !== 'owner'}
			{#if !hidden}
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
			{/if}
		{/each}
	</div>

	<!-- ─── Usage tab ─────────────────────────────────────────────────────────── -->
	{#if activeTab === 'usage'}
		<div>
			<div class="grid grid-cols-2 sm:grid-cols-3 gap-2 sm:gap-3 mb-10">
				{#each [
					{ label: 'Forms',     value: billingLoading ? '…' : billingInfo ? String(billingInfo.formCount) : '—' },
					{ label: 'Responses', value: billingLoading ? '…' : billingInfo ? String(billingInfo.monthlyResponseCount) : '—' },
					{ label: 'Members',   value: billingLoading ? '…' : billingInfo ? String(billingInfo.memberCount) : '—' },
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
						{#if workspacesStore.active.plan === 'free' && workspacesStore.active.role === 'owner'}
							<button
								onclick={() => { activeTab = 'billing'; }}
								class="px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base
									hover:bg-primary-hover transition-colors duration-100"
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

	<!-- ─── Billing tab ───────────────────────────────────────────────────────── -->
	{:else if activeTab === 'billing'}
		{#if workspacesStore.active?.role !== 'owner'}
			<p class="text-sm text-muted-dim">Only workspace owners can manage billing.</p>
		{:else}
			<!-- Upgraded success banner -->
			{#if showUpgradedBanner}
				<div class="mb-6 px-4 py-3 rounded-lg border border-success-border bg-success-bg flex items-center justify-between gap-3">
					<div class="flex items-center gap-2">
						<Check size={14} strokeWidth={2} class="text-success-text-dark shrink-0" />
						<p class="m-0 text-sm text-success-text-dark font-medium">You're now on Pro — welcome!</p>
					</div>
					<button
						onclick={() => { showUpgradedBanner = false; }}
						class="text-success-text-dark opacity-60 hover:opacity-100 bg-transparent border-none cursor-pointer p-0 leading-none"
						aria-label="Dismiss"
					>×</button>
				</div>
			{/if}

			<!-- Past-due banner -->
			{#if billingInfo?.planStatus === 'past_due'}
				<div class="mb-6 px-4 py-3 rounded-lg border border-warning-border bg-warning-bg flex items-center justify-between gap-3">
					<div class="flex items-center gap-2">
						<AlertTriangle size={14} strokeWidth={1.75} class="text-warning-text shrink-0" />
						<p class="m-0 text-sm text-warning-text">Payment failed — update your payment method to keep Pro access.</p>
					</div>
					<button
						onclick={handleOpenPortal}
						disabled={portalLoading}
						class="shrink-0 px-3 py-1.5 text-sm font-medium text-warning-text border border-warning-border rounded
							cursor-pointer bg-transparent hover:bg-warning-bg-dark transition-colors duration-100 font-mono
							disabled:opacity-50 disabled:cursor-not-allowed"
					>{portalLoading ? 'Opening…' : 'Update payment →'}</button>
				</div>
			{/if}

			<!-- Plan cards -->
			<h2 class="m-0 mb-4 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Plan</h2>

			{#if billingLoading && !billingInfo}
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-8">
					{#each [0, 1] as _}
						<div class="border border-border-deep rounded-lg p-5 h-64 animate-pulse bg-surface-deep"></div>
					{/each}
				</div>
			{:else}
				{@const currentPlan = billingInfo?.plan ?? workspacesStore.active?.plan ?? 'free'}
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-8">

					<!-- Free card -->
					<div class="border rounded-lg p-5 flex flex-col gap-4
						{currentPlan === 'free' ? 'border-border-subtle bg-surface-hover' : 'border-border-deep'}">
						<div class="flex items-start justify-between gap-2">
							<div>
								<p class="m-0 text-base font-semibold text-text-bright">Free</p>
								<p class="m-0 text-2xl font-semibold text-text-bright mt-1">$0<span class="text-sm font-normal text-muted-dim">/mo</span></p>
							</div>
							{#if currentPlan === 'free'}
								<span class="px-2 py-0.5 text-xs font-medium rounded border border-border-subtle text-muted-dim bg-surface-deep">
									Current plan
								</span>
							{/if}
						</div>

						<ul class="m-0 p-0 list-none flex flex-col gap-1.5 flex-1">
							{#each freeFeatures as f}
								<li class="flex items-center gap-2 text-sm text-muted-dim">
									<Check size={12} strokeWidth={2.5} class="shrink-0 text-success-text-dark" />
									{f.label}
								</li>
							{/each}
						</ul>

						{#if currentPlan !== 'free'}
							<button
								onclick={handleDowngradeClick}
								disabled={portalLoading}
								class="w-full py-2 text-sm font-medium rounded border border-border-deep text-muted-dim
									bg-transparent cursor-pointer hover:border-border-subtle hover:text-text-body
									transition-colors duration-100 font-mono disabled:opacity-50 disabled:cursor-not-allowed"
							>{portalLoading ? 'Opening…' : 'Downgrade'}</button>
						{/if}
					</div>

					<!-- Pro card -->
					<div class="border rounded-lg p-5 flex flex-col gap-4
						{currentPlan === 'pro' ? 'border-text-blue bg-surface-hover' : 'border-border-deep'}">
						<div class="flex items-start justify-between gap-2">
							<div>
								<p class="m-0 text-base font-semibold text-text-bright">Pro</p>
								<p class="m-0 text-2xl font-semibold text-text-bright mt-1">$20<span class="text-sm font-normal text-muted-dim">/mo</span></p>
							</div>
							{#if currentPlan === 'pro'}
								<span class="px-2 py-0.5 text-xs font-medium rounded border border-text-blue text-text-blue">
									Current plan
								</span>
							{/if}
						</div>

						<ul class="m-0 p-0 list-none flex flex-col gap-1.5 flex-1">
							{#each proFeatures as f}
								<li class="flex items-center gap-2 text-sm {f.enabled ? 'text-muted-dim' : 'text-muted-mid'}">
									{#if f.enabled}
										<Check size={12} strokeWidth={2.5} class="shrink-0 text-success-text-dark" />
									{:else}
										<span class="shrink-0 w-3 h-3 rounded-full border border-border-mid flex items-center justify-center"></span>
									{/if}
									{f.label}
									{#if !f.enabled}
										<span class="ml-auto text-xs text-muted-mid border border-border-mid rounded px-1.5 py-0.5 shrink-0">soon</span>
									{/if}
								</li>
							{/each}
						</ul>

						{#if currentPlan !== 'pro'}
							<button
								onclick={handleUpgrade}
								disabled={upgrading}
								class="w-full py-2 text-sm font-medium rounded border-none text-white
									bg-primary cursor-pointer hover:bg-primary-hover
									transition-colors duration-100 font-mono disabled:opacity-50 disabled:cursor-not-allowed
									flex items-center justify-center gap-1.5"
							>
								{#if upgrading}
									Redirecting…
								{:else}
									Upgrade to Pro
									<ExternalLink size={12} strokeWidth={1.75} />
								{/if}
							</button>
							{#if upgradeError}
								<p class="m-0 text-xs text-error-light">{upgradeError}</p>
							{/if}
						{/if}
					</div>
				</div>
			{/if}

			<!-- Usage meters (only shown once loaded) -->
			{#if billingInfo}
				{@const memberLimit = billingInfo.plan === 'free' ? 2 : billingInfo.plan === 'pro' ? 10 : -1}
				{@const responseLimit = billingInfo.plan === 'free' ? 250 : billingInfo.plan === 'pro' ? 10_000 : billingInfo.plan === 'org' ? 100_000 : -1}
				<h2 class="m-0 mb-4 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Usage</h2>
				<div class="border border-border-deep rounded-lg divide-y divide-border-deep mb-8">
					<!-- Members row -->
					<div class="px-4 py-3 flex items-center gap-4">
						<p class="m-0 text-sm text-muted-mid w-28 shrink-0">Members</p>
						<div class="flex-1 h-1.5 bg-surface-deep rounded-full overflow-hidden">
							{#if memberLimit > 0}
								{@const pct = Math.min(100, (billingInfo.memberCount / memberLimit) * 100)}
								<div
									class="h-full rounded-full transition-all duration-300
										{pct >= 100 ? 'bg-error-light' : pct >= 80 ? 'bg-warning-text' : 'bg-text-blue'}"
									style="width: {pct}%"
								></div>
							{:else}
								<div class="h-full rounded-full bg-text-blue" style="width: 30%"></div>
							{/if}
						</div>
						<p class="m-0 text-sm text-muted-dim tabular-nums shrink-0 text-right w-20">
							{billingInfo.memberCount}{memberLimit > 0 ? ` / ${memberLimit}` : ''}
						</p>
					</div>

					<!-- Forms row -->
					<div class="px-4 py-3 flex items-center gap-4">
						<p class="m-0 text-sm text-muted-mid w-28 shrink-0">Forms</p>
						<div class="flex-1 h-1.5 bg-surface-deep rounded-full overflow-hidden">
							<div class="h-full rounded-full bg-text-blue" style="width: 30%"></div>
						</div>
						<p class="m-0 text-sm text-muted-dim tabular-nums shrink-0 text-right w-20">
							{billingInfo.formCount} <span class="text-muted-mid">∞</span>
						</p>
					</div>

					<!-- Responses/month row -->
					<div class="px-4 py-3 flex items-center gap-4">
						<p class="m-0 text-sm text-muted-mid w-28 shrink-0">Responses/mo</p>
						<div class="flex-1 h-1.5 bg-surface-deep rounded-full overflow-hidden">
							{#if responseLimit > 0}
								{@const pct = Math.min(100, (billingInfo.monthlyResponseCount / responseLimit) * 100)}
								<div
									class="h-full rounded-full transition-all duration-300
										{pct >= 100 ? 'bg-error-light' : pct >= 80 ? 'bg-warning-text' : 'bg-text-blue'}"
									style="width: {pct}%"
								></div>
							{:else}
								<div class="h-full rounded-full bg-text-blue" style="width: 30%"></div>
							{/if}
						</div>
						<p class="m-0 text-sm text-muted-dim tabular-nums shrink-0 text-right w-20">
							{billingInfo.monthlyResponseCount}{responseLimit > 0 ? ` / ${responseLimit.toLocaleString()}` : ' ∞'}
						</p>
					</div>
				</div>
			{/if}

			<!-- Billing management (shown once a Stripe customer exists) -->
			{#if billingInfo?.hasStripeCustomer}
				<h2 class="m-0 mb-4 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Billing</h2>
				<div class="border border-border-deep rounded-lg px-4 py-4 flex items-center justify-between gap-4">
					<div>
						<p class="m-0 text-base text-text-body">Manage payment method and invoices</p>
						{#if billingInfo.planPeriodEnd}
							<p class="m-0 mt-0.5 text-sm text-muted-dim">
								Current period ends {formatDate(billingInfo.planPeriodEnd)}
							</p>
						{/if}
					</div>
					<button
						onclick={handleOpenPortal}
						disabled={portalLoading}
						class="shrink-0 px-4 py-2 bg-transparent text-muted-dim border border-border-deep rounded
							cursor-pointer font-mono text-sm hover:text-text-body hover:border-border-subtle
							transition-colors duration-100 flex items-center gap-1.5
							disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{portalLoading ? 'Opening…' : 'Open billing portal'}
						{#if !portalLoading}<ExternalLink size={12} strokeWidth={1.75} />{/if}
					</button>
				</div>
			{/if}

			{#if billingError}
				<p class="mt-4 text-sm text-error-light">{billingError}</p>
			{/if}
		{/if}

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

			<!-- Custom domain -->
			{#if isAdmin()}
			<div class="mt-4 pt-6 border-t border-border-deep flex flex-col gap-3">
				<div class="flex items-center gap-3">
					<p class="m-0 text-sm font-semibold tracking-[0.08em] uppercase text-muted-mid">Custom domain</p>
					{#if workspacesStore.active?.plan !== 'pro'}
						<span class="px-2 py-0.5 rounded text-xs border border-border-deep text-muted-dim">Pro</span>
					{/if}
				</div>

				{#if workspacesStore.active?.plan !== 'pro'}
					<p class="m-0 text-muted-dim text-base">Upgrade to Pro to serve forms on your own domain.</p>
				{:else if customDomain === null}
					<p class="m-0 text-muted-dim text-base">Loading…</p>
				{:else if customDomain.domain}
					<!-- Domain set — show status + DNS records -->
					<div class="flex items-center gap-3 flex-wrap">
						<span class="text-text-body text-base font-mono">{customDomain.domain}</span>
						{#if customDomain.enabled}
							<span class="px-2 py-0.5 rounded-full text-xs bg-open-bg text-open-text border border-open-border">Active</span>
						{:else}
							<span class="px-2 py-0.5 rounded-full text-xs bg-closed-bg text-closed-text border border-closed-border">Pending</span>
						{/if}
						<button
							onclick={removeDomain}
							disabled={domainRemoving}
							class="px-3 py-1 bg-transparent text-error-light border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border-danger-dark transition-colors duration-100 disabled:opacity-50"
						>{domainRemoving ? 'Removing…' : 'Remove'}</button>
					</div>

					{#if !customDomain.enabled}
						<div class="p-4 bg-surface-mid border border-border-deep rounded flex flex-col gap-3">
							<p class="m-0 text-base text-text-body">Add these DNS records, then click Check:</p>

							<!-- CNAME record -->
							<div class="flex flex-col gap-1">
								<div class="flex items-center gap-2">
									<span class="text-xs font-mono text-muted-dim uppercase tracking-wide w-10">CNAME</span>
									<div class="flex-1 flex items-center gap-2 min-w-0">
										<code class="flex-1 px-2 py-1 bg-surface border border-border-deep rounded font-mono text-sm text-text-body truncate">{customDomain.cnameRecord?.name ?? customDomain.domain}</code>
										<span class="text-muted-dark shrink-0">→</span>
										<code class="flex-1 px-2 py-1 bg-surface border border-border-deep rounded font-mono text-sm text-text-body truncate">{customDomain.cnameRecord?.value ?? customDomain.cnameTarget}</code>
									</div>
									{#if customDomain.cnameOK}
										<span class="shrink-0 text-xs text-open-text font-mono">✓</span>
									{:else}
										<span class="shrink-0 text-xs text-muted-dark font-mono">✗</span>
									{/if}
								</div>
							</div>

							<!-- TXT record -->
							<div class="flex flex-col gap-1">
								<div class="flex items-center gap-2">
									<span class="text-xs font-mono text-muted-dim uppercase tracking-wide w-10">TXT</span>
									<div class="flex-1 flex flex-col gap-1 min-w-0">
										<code class="px-2 py-1 bg-surface border border-border-deep rounded font-mono text-sm text-text-body truncate">{customDomain.txtRecord?.name ?? `_confide-verify.${customDomain.domain}`}</code>
										<code class="px-2 py-1 bg-surface border border-border-deep rounded font-mono text-sm text-text-body truncate">{customDomain.txtRecord?.value ?? '—'}</code>
									</div>
									{#if customDomain.txtOK}
										<span class="shrink-0 text-xs text-open-text font-mono self-start mt-1">✓</span>
									{:else}
										<span class="shrink-0 text-xs text-muted-dark font-mono self-start mt-1">✗</span>
									{/if}
								</div>
							</div>

							<button
								onclick={checkDomainNow}
								disabled={domainChecking}
								class="self-end px-4 py-1.5 bg-transparent text-muted-blue border border-border-subtle rounded cursor-pointer font-mono text-base hover:border-border transition-colors duration-100 disabled:opacity-50"
							>{domainChecking ? 'Checking…' : 'Check now'}</button>
						</div>
					{/if}
				{:else}
					<!-- No domain set -->
					<div class="flex gap-2 flex-wrap items-center">
						<input
							bind:value={domainInput}
							placeholder="forms.yourdomain.com"
							class="px-3 py-2 bg-surface-input border border-border-subtle rounded font-mono text-base text-text-body placeholder-muted-dim focus:outline-none focus:border-border-focus transition-colors duration-100 w-64"
						/>
						<button
							onclick={saveDomain}
							disabled={domainSaving || !domainInput.trim()}
							class="px-5 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-base hover:bg-primary-hover transition-colors duration-100 disabled:opacity-50"
						>{domainSaving ? 'Saving…' : 'Save'}</button>
					</div>
					<p class="m-0 text-muted-dim text-base">
						Enter the hostname you want to use (e.g. <span class="font-mono text-muted-mid">forms.yourdomain.com</span>).
						You'll need to add a CNAME and a TXT record at your DNS provider.
					</p>
				{/if}

				{#if domainError}
					<p class="m-0 text-error-light text-base">{domainError}</p>
				{/if}
			</div>
			{/if}

			<!-- Danger zone -->
			{#if workspacesStore.active?.role === 'owner'}
			<div class="mt-4">
				<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Danger zone</h2>
				<div class="border border-border-danger-deep rounded-lg px-4 py-4 flex items-center justify-between gap-4">
					<div>
						<p class="m-0 text-base text-text-body">Delete workspace</p>
						<p class="m-0 mt-0.5 text-sm text-muted-dim">Permanently delete this workspace and all its data.</p>
					</div>
					<button
						onclick={() => { showDeleteConfirm = true; deleteError = ''; }}
						class="shrink-0 px-4 py-2 bg-transparent text-error-light border border-border-danger-dark rounded
							cursor-pointer font-mono text-base hover:bg-danger-bg-dark transition-colors duration-100"
					>
						Delete
					</button>
				</div>
			</div>
			{:else}
			<div class="mt-4">
				<h2 class="m-0 mb-3 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Danger zone</h2>
				<div class="border border-border-danger-deep rounded-lg px-4 py-4 flex items-center justify-between gap-4">
					<div>
						<p class="m-0 text-base text-text-body">Leave workspace</p>
						<p class="m-0 mt-0.5 text-sm text-muted-dim">Remove yourself from this workspace. You'll need an invitation to rejoin.</p>
					</div>
					<button
						onclick={() => { showLeaveConfirm = true; leaveError = ''; }}
						class="shrink-0 px-4 py-2 bg-transparent text-error-light border border-border-danger-dark rounded
							cursor-pointer font-mono text-base hover:bg-danger-bg-dark transition-colors duration-100"
					>
						Leave
					</button>
				</div>
			</div>
			{/if}
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
