<script lang="ts">
	import { onMount, setContext } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { createBuilderStore } from '$lib/stores/builder.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';
	import { getForm, setFormCustomDomain } from '$lib/forms';
	import { getCustomDomain, type CustomDomainInfo } from '$lib/workspaces';
	import { getAppConfig } from '$lib/config';
	import FieldCanvas from '$lib/components/builder/FieldCanvas.svelte';
	import FieldPalette from '$lib/components/builder/FieldPalette.svelte';
	import PropertiesPanel from '$lib/components/builder/PropertiesPanel.svelte';
	import FormSettingsPanel from '$lib/components/builder/FormSettingsPanel.svelte';
	import { ChevronDown, Settings, ScrollText, LayoutList, MessageCircle, Loader, CloudOff, Check } from '@lucide/svelte';
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

	// Custom domain state
	let useCustomDomain = $state(false);
	let workspaceDomain = $state<CustomDomainInfo | null>(null);
	let customDomainToggling = $state(false);
	let formsBaseUrl = $state('');

	onMount(async () => {
		if (!auth.masterKey || !store) {
			goto('/login');
			return;
		}
		try {
			await store.load();
			const { record } = await getForm(auth.masterKey, formId);
			useCustomDomain = record.useCustomDomain ?? false;
			if (record.workspaceId) {
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
		if (useCustomDomain && workspaceDomain?.verified && workspaceDomain.domain) {
			return `https://${workspaceDomain.domain}`;
		}
		return formsBaseUrl || undefined;
	}

	async function toggleCustomDomain() {
		customDomainToggling = true;
		try {
			await setFormCustomDomain(formId, !useCustomDomain);
			useCustomDomain = !useCustomDomain;
		} catch {
			// ignore toggle errors silently
		} finally {
			customDomainToggling = false;
		}
	}

	function handleAddLocale() {
		if (!store || !newLocaleInput.trim()) return;
		store.addLocale(newLocaleInput.trim().toLowerCase());
		newLocaleInput = '';
		showLocaleInput = false;
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
		<div class="flex items-center gap-3 px-5 h-9 border-b border-border-deep shrink-0 overflow-x-auto">
			<!-- Breadcrumb -->
			<Breadcrumb items={[
				{ label: 'Forms', href: '/forms' },
				{ label: store.schema.translations[store.schema.defaultLocale]?.formTitle || formsStore.formNames.get(formId) || formId.slice(0, 12) + '…', href: `/forms/${formId}` },
				{ label: 'Edit' }
			]} />

<!-- Layout selector — hidden for now -->
			<div class="hidden items-center gap-2 shrink-0">
				<div class="w-px h-[18px] bg-border-field"></div>
				<div class="relative">
					{#if layoutOpen}
						<div onclick={() => layoutOpen = false} class="fixed inset-0 z-10"></div>
					{/if}
					<button
						onclick={() => layoutOpen = !layoutOpen}
						style="background: {layoutOpen ? '#1f2937' : 'transparent'}; border-color: {layoutOpen ? 'var(--color-border)' : 'var(--color-border-field)'};"
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
						<div class="absolute top-[calc(100%+4px)] left-0 bg-surface border border-border-field rounded-lg p-1 min-w-[210px] z-20 shadow-[0_8px_24px_var(--color-overlay-light)]">
							{#each layoutModes as mode}
								{@const active = mode.value === store.schema.layout}
								<button
									onclick={() => { store!.setLayout(mode.value); layoutOpen = false; }}
									class="flex items-start gap-2.5 w-full px-2.5 py-2 border-none rounded-md cursor-pointer font-mono text-left transition-[background,color] duration-100
										{active ? 'bg-[#1f2d42] text-text' : 'bg-transparent text-muted hover:bg-[#1e2b3c] hover:text-text-dim'}"
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

			<!-- Locale switcher — hidden for now -->
			<div class="hidden items-center gap-1.5 shrink-0">
				<div class="w-px h-[18px] bg-border-field"></div>
				<div class="relative flex items-center">
					<select
						value={store.activeLocale}
						onchange={(e) => store!.setActiveLocale((e.target as HTMLSelectElement).value)}
						class="appearance-none pl-2.5 pr-7 h-7 bg-surface text-muted border border-border-field rounded-md cursor-pointer font-mono text-sm outline-none leading-none"
					>
						{#each store.schema.locales as locale}
							<option value={locale}>{locale}</option>
						{/each}
					</select>
					<span class="absolute right-1.5 top-1/2 -translate-y-1/2 pointer-events-none flex text-muted-dark">
						<ChevronDown size={12} strokeWidth={1.75} />
					</span>
				</div>
				{#if showLocaleInput}
					<input
						type="text"
						placeholder="e.g. fr"
						bind:value={newLocaleInput}
						onkeydown={(e) => { if (e.key === 'Enter') handleAddLocale(); if (e.key === 'Escape') showLocaleInput = false; }}
						class="w-14 px-2 h-7 bg-surface border border-border-field text-text-dim rounded-md font-mono text-sm outline-none box-border"
					/>
					<button
						onclick={handleAddLocale}
						class="px-2.5 h-7 bg-primary text-white border-none rounded-md cursor-pointer font-mono text-sm"
					>Add</button>
				{:else}
					<button
						onclick={() => (showLocaleInput = true)}
						class="px-2 h-7 bg-transparent text-muted-dark border border-dashed border-border-field rounded-md cursor-pointer font-mono text-sm transition-[color,border-color] duration-100 hover:text-muted-dark hover:border-border"
					>+ lang</button>
				{/if}
			</div>

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
				style="background: {store.showFormSettings ? '#1f2937' : 'transparent'}; color: {store.showFormSettings ? '#e5e7eb' : '#4b5563'}; border-color: {store.showFormSettings ? 'var(--color-border)' : 'transparent'};"
				class="shrink-0 px-1.5 h-7 flex items-center border rounded-md cursor-pointer transition-colors duration-100 hover:text-muted"
			><Settings size={15} strokeWidth={1.75} /></button>

			<div class="w-px h-[18px] bg-border-field shrink-0"></div>

			<!-- Preview toggle -->
			<button
				onclick={() => store!.setMode(store!.mode === 'edit' ? 'preview' : 'edit')}
				style="background: {store.mode === 'preview' ? '#1f2937' : 'transparent'}; color: {store.mode === 'preview' ? '#e5e7eb' : '#6b7280'}; border-color: {store.mode === 'preview' ? 'var(--color-border)' : 'var(--color-border-field)'};"
				class="shrink-0 px-3 h-7 border rounded-md cursor-pointer font-mono text-sm"
			>{store.mode === 'preview' ? 'Edit' : 'Preview'}</button>

			<!-- Publish button -->
			<button
				onclick={() => store!.setShowFormSettings(true)}
				class="shrink-0 px-3.5 h-7 text-white border-none rounded-md font-mono text-sm bg-primary hover:bg-primary-hover cursor-pointer transition-[background] duration-100"
			>Publish</button>
		</div>

		<!-- Body -->
		<div class="flex flex-1 overflow-hidden relative">
			{#if store.mode === 'edit'}
				<FieldPalette {store} />
			{/if}

			<FieldCanvas {store} />

			{#if store.mode === 'edit'}
				<PropertiesPanel {store} />
				<FormSettingsPanel
					{store}
					{formId}
					{workspaceDomain}
					{useCustomDomain}
					customDomainBase={customDomainBase}
					onToggleCustomDomain={toggleCustomDomain}
				/>
			{/if}
		</div>
	</div>
{/if}
