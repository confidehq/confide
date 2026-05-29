<script lang="ts">
	import { onMount, setContext } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { createBuilderStore } from '$lib/stores/builder.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { getCustomDomain, type CustomDomainInfo } from '$lib/workspaces';
	import { getAppConfig } from '$lib/config';
	import FieldCanvas from '$lib/components/builder/FieldCanvas.svelte';
	import PropertiesPanel from '$lib/components/builder/PropertiesPanel.svelte';
	import FormSettingsPanel from '$lib/components/builder/FormSettingsPanel.svelte';
	import { ChevronDown, Settings, ScrollText, LayoutList, MessageCircle, Loader, CloudOff, Check, Languages } from '@lucide/svelte';
	import { publishForm } from '$lib/forms';
	import Breadcrumb from '$lib/components/Breadcrumb.svelte';
	import type { Component } from 'svelte';

	type LayoutMode = 'scroll' | 'steps' | 'convo';

	const layoutModes: Array<{ value: LayoutMode; label: string; help: string; icon: Component }> = [
		{ value: 'scroll', label: 'Scroll mode', help: 'All questions on a single page', icon: ScrollText },
		{ value: 'steps', label: 'Steps mode', help: 'One question per step', icon: LayoutList },
		{ value: 'convo', label: 'Convo mode', help: 'Chat-like conversational flow', icon: MessageCircle }
	];

	let layoutOpen = $state(false);

	const formId = $page.params.id ?? '';

	// Navigate to /forms if the workspace changes while editing this form
	const mountedWorkspaceId = workspacesStore.active?.id;
	$effect(() => {
		const id = workspacesStore.active?.id;
		if (id !== undefined && id !== mountedWorkspaceId) goto('/forms');
	});

	// Must be created synchronously at component init time so $effect/$derived
	// in the store have the correct Svelte component owner context.
	// If masterKey is null, we redirect to /login in onMount.
	const store = auth.masterKey ? createBuilderStore(auth.masterKey, formId) : null;
	if (store) setContext('builder', store);

	let loading = $state(true);
	let loadError = $state('');

	// New locale input
	let newLocaleInput = $state('');
	let showLocaleInput = $state(false);

	let workspaceDomain = $state<CustomDomainInfo | null>(null);
	let workspaceId = $state<string | undefined>(undefined);
	let formsBaseUrl = $state('');

	onMount(async () => {
		if (!auth.masterKey || !store) {
			goto('/login');
			return;
		}
		try {
			const record = await store.load();
			if (record.workspaceId) {
				workspaceId = record.workspaceId;
				getCustomDomain(record.workspaceId).then(d => { workspaceDomain = d; }).catch(() => {});
			}
			getAppConfig().then(c => { formsBaseUrl = c.formsDomain ? `https://${c.formsDomain}` : ''; }).catch(() => {});
		} catch {
			loadError = 'Form not found or could not be loaded.';
		} finally {
			loading = false;
		}
	});

	function customDomainBase(): string | undefined {
		if (workspaceDomain?.enabled && workspaceDomain.domain) {
			return `https://${workspaceDomain.domain}`;
		}
		return formsBaseUrl || undefined;
	}

	function handleAddLocale() {
		if (!store || !newLocaleInput.trim()) return;
		store.addLocale(newLocaleInput.trim().toLowerCase());
		newLocaleInput = '';
		showLocaleInput = false;
	}

	let publishing = $state(false);

	const publishButtonLabel = $derived(
		!store ? 'Publish'
		: publishing ? 'Publishing…'
		: store.formStatus === 'draft' ? 'Publish'
		: store.hasUnpublishedChanges ? 'Update'
		: 'Up to date'
	);
	const publishButtonDisabled = $derived(
		!store || store.saving || publishing || (store.formStatus !== 'draft' && !store.hasUnpublishedChanges)
	);

	async function handlePublish() {
		if (!auth.masterKey || !store) return;
		publishing = true;
		try {
			await store.flushSave();
			const isFirstPublish = store.formStatus === 'draft';
			const result = await publishForm(auth.masterKey, formId, store.schema, store.renderKeySalt, store.formKey ?? undefined, customDomainBase());
			store.setRenderKeySalt(result.renderKeySalt);
			store.markPublished();
			if (isFirstPublish) store.setShowFormSettings(true);
		} finally {
			publishing = false;
		}
	}
</script>

<svelte:head>
	<title>Form Builder</title>
</svelte:head>

{#if loading}
	<div class="font-mono flex items-center justify-center flex-1 bg-surface text-muted">
		<p>Loading form…</p>
	</div>
{:else if loadError}
	<div class="font-mono flex flex-col items-center justify-center flex-1 bg-surface text-error-light gap-4">
		<p>{loadError}</p>
		<a href="/forms" class="text-muted-dark text-sm no-underline">← Back to forms</a>
	</div>
{:else if store}
	<div class="flex flex-col flex-1 min-h-0 bg-surface font-mono text-text-dim overflow-hidden">
		<!-- Toolbar -->
		<div class="relative flex items-center gap-3 px-5 h-9 border-b border-border-deep shrink-0 overflow-x-auto">
			<!-- Breadcrumb -->
			<Breadcrumb items={[
				{ label: 'Forms', href: '/forms' },
				{ label: store.schema.translations[store.schema.defaultLocale]?.formTitle || formsStore.formNames.get(formId) || formId.slice(0, 12) + '…', href: `/forms/${formId}` },
				{ label: 'Edit' }
			]} />

<!-- Layout selector — hidden for now -->
			<div class="hidden items-center gap-2 shrink-0">
				<div class="w-px h-4 bg-border-field"></div>
				<div class="relative">
					{#if layoutOpen}
						<div onclick={() => layoutOpen = false} class="fixed inset-0 z-10"></div>
					{/if}
					<button
						onclick={() => layoutOpen = !layoutOpen}
						style="background: {layoutOpen ? 'var(--color-surface-toolbar)' : 'transparent'}; border-color: {layoutOpen ? 'var(--color-border)' : 'var(--color-border-field)'};"
						class="flex items-center gap-1.5 px-2 h-7 text-muted border rounded-md cursor-pointer font-mono text-sm transition-[background,border-color] duration-100"
					>
						{#each layoutModes as mode}
							{#if mode.value === store.schema.layout}
								<svelte:component this={mode.icon} size={13} strokeWidth={1.75} />
								<span>{mode.label}</span>
							{/if}
						{/each}
						<ChevronDown size={11} strokeWidth={1.75} class="text-muted-dark ml-0.5" />
					</button>

					{#if layoutOpen}
						<div class="absolute top-[calc(100%+4px)] left-0 bg-surface border border-border-field rounded-lg p-1 min-w-52 z-20 shadow-[0_8px_24px_var(--color-overlay-light)]">
							{#each layoutModes as mode}
								{@const active = mode.value === store.schema.layout}
								<button
									onclick={() => { store!.setLayout(mode.value); layoutOpen = false; }}
									class="flex items-start gap-2.5 w-full px-2.5 py-2 border-none rounded-md cursor-pointer font-mono text-left transition-[background,color] duration-100
										{active ? 'bg-surface-hover text-text' : 'bg-transparent text-muted hover:bg-surface-hover hover:text-text-dim'}"
								>
									<span class="mt-0.5 shrink-0 {active ? 'text-text-blue' : 'text-muted-dim'}">
										<svelte:component this={mode.icon} size={15} strokeWidth={1.75} />
									</span>
									<span>
										<span class="block text-sm">{mode.label}</span>
										<span class="block text-sm text-muted-dim mt-0.5">{mode.help}</span>
									</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			</div>

			<!-- Locale switcher — centered absolutely, desktop only (mobile: in FormSettingsPanel) -->
			{#if store.schema.locales.length > 1}
				<div class="hidden sm:flex absolute left-1/2 -translate-x-1/2 items-center pointer-events-none">
					<div class="relative flex items-center pointer-events-auto">
						<Languages size={13} strokeWidth={1.75} class="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none text-muted-dark" />
						<select
							value={store.activeLocale}
							onchange={(e) => store!.setActiveLocale((e.target as HTMLSelectElement).value)}
							class="appearance-none pl-7 pr-7 h-7 bg-surface text-muted border border-border-field rounded-md cursor-pointer font-mono text-sm outline-none leading-none"
						>
							{#each store.schema.locales as locale}
								<option value={locale}>
									{new Intl.DisplayNames([locale, 'en'], { type: 'language' }).of(locale) ?? locale}
								</option>
							{/each}
						</select>
						<span class="absolute right-1.5 top-1/2 -translate-y-1/2 pointer-events-none flex text-muted-dark">
							<ChevronDown size={12} strokeWidth={1.75} />
						</span>
					</div>
				</div>
			{/if}

			<!-- Spacer -->
			<div class="flex-1"></div>

			<!-- Save indicator -->
			{#if store.saving}
				<span title="Saving…" class="flex shrink-0 text-muted-dark"><Loader size={14} strokeWidth={2} /></span>
			{:else if store.dirty}
				<span title="Unsaved changes" class="flex shrink-0 text-muted-dark"><CloudOff size={14} strokeWidth={2} /></span>
			{:else if store.lastSaved}
				<span title="Saved" class="flex shrink-0 text-border"><Check size={14} strokeWidth={2} /></span>
			{/if}

			<!-- Form settings cog -->
			<button
				onclick={() => store!.setShowFormSettings(!store.showFormSettings)}
				title="Form settings"
				style="background: {store.showFormSettings ? 'var(--color-surface-toolbar)' : 'transparent'}; color: {store.showFormSettings ? 'var(--color-text)' : 'var(--color-text-subtle)'}; border-color: {store.showFormSettings ? 'var(--color-border)' : 'transparent'};"
				class="shrink-0 px-1.5 h-7 flex items-center border rounded-md cursor-pointer transition-colors duration-100 hover:text-muted"
			><Settings size={15} strokeWidth={1.75} /></button>

			<div class="w-px h-4 bg-border-field shrink-0"></div>

			<!-- Preview toggle -->
			<button
				onclick={() => store!.setMode(store!.mode === 'edit' ? 'preview' : 'edit')}
				style="background: {store.mode === 'preview' ? 'var(--color-surface-toolbar)' : 'transparent'}; color: {store.mode === 'preview' ? 'var(--color-text)' : 'var(--color-muted-dark)'}; border-color: {store.mode === 'preview' ? 'var(--color-border)' : 'var(--color-border-field)'};"
				class="shrink-0 px-3 h-7 border rounded-md cursor-pointer font-mono text-sm"
			>{store.mode === 'preview' ? 'Edit' : 'Preview'}</button>

			<!-- Publish button -->
			<button
				onclick={handlePublish}
				disabled={publishButtonDisabled}
				class="shrink-0 px-3.5 h-7 text-white border-none rounded-md font-mono text-sm bg-primary hover:bg-primary-hover cursor-pointer transition-[background] duration-100 disabled:opacity-50 disabled:cursor-not-allowed"
			>{publishButtonLabel}</button>
		</div>

		<!-- Body -->
		<div class="flex flex-1 overflow-hidden relative">
			<FieldCanvas {store} />

			{#if store.mode === 'edit'}
				<PropertiesPanel {store} />
				<FormSettingsPanel
					{store}
					{formId}
					{workspaceId}
					{workspaceDomain}
					customDomainBase={customDomainBase}
				/>
			{/if}
		</div>
	</div>
{/if}
