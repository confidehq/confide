<script lang="ts">
	import { onMount, setContext } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { createBuilderStore } from '$lib/stores/builder.svelte';
	import { publishForm, rotateRenderKey } from '$lib/forms';
	import FieldCanvas from '$lib/components/builder/FieldCanvas.svelte';
	import FieldPalette from '$lib/components/builder/FieldPalette.svelte';
	import PropertiesPanel from '$lib/components/builder/PropertiesPanel.svelte';
	import { ChevronDown, Settings, ScrollText, LayoutList, MessageCircle, Loader, CloudOff, Check } from '@lucide/svelte';
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

	// Publish modal state
	let publishModalOpen = $state(false);
	let shareUrl = $state('');
	let publishing = $state(false);
	let publishError = $state('');

	// New locale input
	let newLocaleInput = $state('');
	let showLocaleInput = $state(false);

	onMount(async () => {
		if (!auth.masterKey || !store) {
			goto('/login');
			return;
		}
		try {
			await store.load();
		} catch {
			loadError = 'Form not found or could not be loaded.';
		} finally {
			loading = false;
		}
	});

	async function handlePublish() {
		if (!store || !auth.masterKey) return;
		publishing = true;
		publishError = '';
		try {
			await store.flushSave();
			const result = await publishForm(auth.masterKey, formId, store.schema, store.renderKeySalt, store.formKey ?? undefined);
			store.setRenderKeySalt(result.renderKeySalt);
			shareUrl = result.shareUrl;
			publishModalOpen = true;
		} catch (err) {
			publishError = err instanceof Error ? err.message : 'Publish failed';
		} finally {
			publishing = false;
		}
	}

	async function handleRotateKey() {
		if (!store || !auth.masterKey) return;
		publishing = true;
		publishError = '';
		try {
			const result = await rotateRenderKey(auth.masterKey, formId, store.schema, store.formKey ?? undefined);
			store.setRenderKeySalt(result.renderKeySalt);
			shareUrl = result.shareUrl;
		} catch (err) {
			publishError = err instanceof Error ? err.message : 'Key rotation failed';
		} finally {
			publishing = false;
		}
	}

	let copied = $state(false);
	let copiedTimer: ReturnType<typeof setTimeout> | null = null;

	function copyShareUrl() {
		navigator.clipboard.writeText(shareUrl);
		copied = true;
		if (copiedTimer) clearTimeout(copiedTimer);
		copiedTimer = setTimeout(() => { copied = false; }, 2000);
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
	<div class="font-mono flex items-center justify-center flex-1 bg-canvas text-muted">
		<p>Loading form…</p>
	</div>
{:else if loadError}
	<div class="font-mono flex flex-col items-center justify-center flex-1 bg-canvas text-error-light gap-4">
		<p>{loadError}</p>
		<a href="/forms" class="text-muted-dark text-sm no-underline">← Back to forms</a>
	</div>
{:else if store}
	<div class="flex flex-col flex-1 min-h-0 bg-canvas font-mono text-text-dim overflow-hidden">
		<!-- Toolbar -->
		<div class="flex items-center gap-2 px-3 h-11 bg-[#161d28] border-b border-border-field shrink-0 overflow-x-auto">
			<!-- Form name input -->
			<input
				type="text"
				placeholder="Untitled form"
				value={store.schema.name}
				oninput={(e) => store!.setName((e.target as HTMLInputElement).value)}
				class="bg-transparent border-none outline-none text-text font-mono text-base w-[140px] sm:w-[160px] min-w-0 shrink px-1.5 py-1 rounded transition-[background] duration-100 focus:bg-surface"
			/>

			<!-- Layout selector — hidden on mobile -->
			<div class="hidden sm:flex items-center gap-2 shrink-0">
				<div class="w-px h-[18px] bg-[#2a3341]"></div>
				<div class="relative">
					{#if layoutOpen}
						<div onclick={() => layoutOpen = false} class="fixed inset-0 z-10"></div>
					{/if}
					<button
						onclick={() => layoutOpen = !layoutOpen}
						style="background: {layoutOpen ? '#1f2937' : 'transparent'}; border-color: {layoutOpen ? '#374151' : '#2a3341'};"
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

			<!-- Locale switcher — hidden on mobile -->
			<div class="hidden sm:flex items-center gap-1.5 shrink-0">
				<div class="w-px h-[18px] bg-[#2a3341]"></div>
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
				style="background: {store.showFormSettings ? '#1f2937' : 'transparent'}; color: {store.showFormSettings ? '#e5e7eb' : '#4b5563'}; border-color: {store.showFormSettings ? '#374151' : 'transparent'};"
				class="shrink-0 px-1.5 h-7 flex items-center border rounded-md cursor-pointer transition-colors duration-100 hover:text-muted"
			><Settings size={15} strokeWidth={1.75} /></button>

			<div class="w-px h-[18px] bg-[#2a3341] shrink-0"></div>

			<!-- Preview toggle -->
			<button
				onclick={() => store!.setMode(store!.mode === 'edit' ? 'preview' : 'edit')}
				style="background: {store.mode === 'preview' ? '#1f2937' : 'transparent'}; color: {store.mode === 'preview' ? '#e5e7eb' : '#6b7280'}; border-color: {store.mode === 'preview' ? '#374151' : '#2a3341'};"
				class="shrink-0 px-3 h-7 border rounded-md cursor-pointer font-mono text-sm"
			>{store.mode === 'preview' ? 'Edit' : 'Preview'}</button>

			<!-- Publish button -->
			<button
				onclick={handlePublish}
				disabled={store.saving || publishing}
				class="shrink-0 px-3.5 h-7 text-white border-none rounded-md font-mono text-sm transition-[background,opacity] duration-100
					{store.saving || publishing ? 'bg-info-bg-dark cursor-not-allowed opacity-70' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
			>{publishing ? 'Publishing…' : 'Publish'}</button>

			{#if publishError}
				<span class="shrink-0 text-error-light text-xs">{publishError}</span>
			{/if}
		</div>

		<!-- Body -->
		<div class="flex flex-1 overflow-hidden relative">
			{#if store.mode === 'edit'}
				<FieldPalette {store} />
			{/if}

			<FieldCanvas {store} />

			{#if store.mode === 'edit'}
				<PropertiesPanel {store} />
			{/if}
		</div>
	</div>

	<!-- Publish modal -->
	{#if publishModalOpen}
		<div
			role="dialog"
			aria-modal="true"
			aria-label="Form published"
			class="fixed inset-0 bg-black/70 flex items-center justify-center z-[1000]"
			onclick={(e) => { if (e.target === e.currentTarget) publishModalOpen = false; }}
		>
			<div class="bg-surface border border-border rounded-lg p-8 max-w-[540px] w-[90%] font-mono">
				<h2 class="m-0 mb-2 text-xl text-text-bright">Your form is live.</h2>
				<p class="m-0 mb-5 text-sm text-muted">Share this link with respondents:</p>

				<div class="flex gap-2 mb-6">
					<input
						type="text"
						readonly
						value={shareUrl}
						class="flex-1 px-3 py-2 bg-canvas border border-border text-text-dim rounded font-mono text-sm outline-none"
					/>
					<button
						onclick={copyShareUrl}
						class="px-4 py-2 text-white border-none rounded font-mono text-sm transition-[background] duration-150
							{copied ? 'bg-[#16a34a]' : 'bg-primary-hover hover:bg-primary cursor-pointer'}"
					>
						{copied ? 'Copied!' : 'Copy'}
					</button>
				</div>

				<div class="flex justify-between items-center">
					<button
						onclick={handleRotateKey}
						disabled={publishing}
						class="px-3 py-1.5 bg-transparent text-muted border border-border rounded cursor-pointer font-mono text-sm
							{publishing ? 'cursor-not-allowed opacity-60' : 'hover:text-text transition-colors duration-100'}"
					>
						{publishing ? 'Rotating…' : 'Rotate key (invalidates old links)'}
					</button>
					<button
						onclick={() => (publishModalOpen = false)}
						class="px-3 py-1.5 bg-transparent text-muted-dark border-none cursor-pointer font-mono text-sm hover:text-muted transition-colors duration-100"
					>
						Close
					</button>
				</div>

				{#if publishError}
					<p class="mt-3 m-0 text-error-light text-sm">{publishError}</p>
				{/if}
			</div>
		</div>
	{/if}
{/if}
