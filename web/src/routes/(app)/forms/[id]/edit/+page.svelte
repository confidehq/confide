<script lang="ts">
	import { onMount, setContext } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { createBuilderStore } from '$lib/stores/builder.svelte';
	import { publishForm } from '$lib/forms';
	import FieldPalette from '$lib/components/builder/FieldPalette.svelte';
	import FieldCanvas from '$lib/components/builder/FieldCanvas.svelte';
	import PropertiesPanel from '$lib/components/builder/PropertiesPanel.svelte';

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
	let publishedRenderKey: CryptoKey | null = null;
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
			const result = await publishForm(auth.masterKey, formId, store.schema);
			publishedRenderKey = result.renderKey;
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
			const result = await publishForm(auth.masterKey, formId, store.schema);
			publishedRenderKey = result.renderKey;
			shareUrl = result.shareUrl;
		} catch (err) {
			publishError = err instanceof Error ? err.message : 'Key rotation failed';
		} finally {
			publishing = false;
		}
	}

	function copyShareUrl() {
		navigator.clipboard.writeText(shareUrl);
	}

	function handleAddLocale() {
		if (!store || !newLocaleInput.trim()) return;
		store.addLocale(newLocaleInput.trim().toLowerCase());
		newLocaleInput = '';
		showLocaleInput = false;
	}

	function saveIndicatorText(s: ReturnType<typeof createBuilderStore>) {
		if (s.saving) return 'Saving…';
		if (s.dirty) return 'Unsaved changes';
		if (s.lastSaved) return 'Saved';
		return '';
	}
</script>

<svelte:head>
	<title>Form Builder</title>
</svelte:head>

{#if loading}
	<div style="
		font-family: monospace;
		display: flex; align-items: center; justify-content: center;
		flex: 1; background: #111827; color: #9ca3af;
	">
		<p>Loading form…</p>
	</div>
{:else if loadError}
	<div style="
		font-family: monospace;
		display: flex; flex-direction: column; align-items: center; justify-content: center;
		flex: 1; background: #111827; color: #f87171; gap: 16px;
	">
		<p>{loadError}</p>
		<a href="/forms" style="color: #6b7280; font-size: 0.85rem; text-decoration: none;">← Back to forms</a>
	</div>
{:else if store}
	<div style="
		display: flex; flex-direction: column;
		flex: 1; min-height: 0; background: #111827;
		font-family: monospace; color: #d1d5db;
		overflow: hidden;
	">
		<!-- Toolbar -->
		<div style="
			display: flex; align-items: center; gap: 12px;
			padding: 8px 16px;
			background: #1f2937;
			border-bottom: 1px solid #374151;
			flex-shrink: 0;
			flex-wrap: wrap;
		">
			<!-- Form title input -->
			<input
				type="text"
				placeholder="Form title…"
				value={store.activeTranslation?.formTitle ?? ''}
				oninput={(e) => store!.updateTranslation(null, 'formTitle', (e.target as HTMLInputElement).value)}
				style="
					background: transparent; border: none; outline: none;
					color: #f9fafb; font-family: monospace; font-size: 0.95rem;
					width: 220px;
				"
			/>

			<div style="width: 1px; height: 20px; background: #374151;"></div>

			<!-- Layout selector -->
			<div style="display: flex; gap: 2px;">
				{#each ['scroll', 'steps', 'convo'] as layout}
					<button
						onclick={() => store!.setLayout(layout as 'scroll' | 'steps' | 'convo')}
						style="
							padding: 4px 10px;
							background: {store.schema.layout === layout ? '#1d4ed8' : 'transparent'};
							color: {store.schema.layout === layout ? '#fff' : '#9ca3af'};
							border: 1px solid {store.schema.layout === layout ? '#1d4ed8' : '#374151'};
							border-radius: 4px;
							cursor: pointer;
							font-family: monospace;
							font-size: 0.75rem;
						"
					>
						{layout}
					</button>
				{/each}
			</div>

			<div style="width: 1px; height: 20px; background: #374151;"></div>

			<!-- Locale switcher -->
			<div style="display: flex; align-items: center; gap: 6px;">
				{#each store.schema.locales as locale}
					<button
						onclick={() => store!.setActiveLocale(locale)}
						style="
							padding: 4px 10px;
							background: {store.activeLocale === locale ? '#374151' : 'transparent'};
							color: {store.activeLocale === locale ? '#f9fafb' : '#9ca3af'};
							border: 1px solid #374151;
							border-radius: 4px;
							cursor: pointer;
							font-family: monospace;
							font-size: 0.75rem;
						"
					>
						{locale}
					</button>
				{/each}
				{#if showLocaleInput}
					<input
						type="text"
						placeholder="e.g. fr"
						bind:value={newLocaleInput}
						onkeydown={(e) => { if (e.key === 'Enter') handleAddLocale(); if (e.key === 'Escape') showLocaleInput = false; }}
						style="
							width: 60px; padding: 4px 6px;
							background: #111827; border: 1px solid #374151;
							color: #d1d5db; border-radius: 4px;
							font-family: monospace; font-size: 0.75rem; outline: none;
						"
					/>
					<button
						onclick={handleAddLocale}
						style="padding: 4px 8px; background: #1d4ed8; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.75rem;"
					>
						Add
					</button>
				{:else}
					<button
						onclick={() => (showLocaleInput = true)}
						style="
							padding: 4px 8px;
							background: transparent; color: #6b7280;
							border: 1px dashed #374151;
							border-radius: 4px; cursor: pointer;
							font-family: monospace; font-size: 0.75rem;
						"
					>
						+ Language
					</button>
				{/if}
			</div>

			<!-- Spacer -->
			<div style="flex: 1;"></div>

			<!-- Save indicator -->
			<span style="font-size: 0.75rem; color: #6b7280; min-width: 120px; text-align: right;">
				{saveIndicatorText(store)}
			</span>

			<div style="width: 1px; height: 20px; background: #374151;"></div>

			<!-- Preview toggle -->
			<button
				onclick={() => store!.setMode(store!.mode === 'edit' ? 'preview' : 'edit')}
				style="
					padding: 6px 12px;
					background: {store.mode === 'preview' ? '#374151' : 'transparent'};
					color: {store.mode === 'preview' ? '#f9fafb' : '#9ca3af'};
					border: 1px solid #374151;
					border-radius: 4px; cursor: pointer;
					font-family: monospace; font-size: 0.8rem;
				"
			>
				{store.mode === 'preview' ? 'Edit' : 'Preview'}
			</button>

			<!-- Publish button -->
			<button
				onclick={handlePublish}
				disabled={store.saving || publishing}
				style="
					padding: 6px 16px;
					background: {store.saving || publishing ? '#1e3a8a' : '#1d4ed8'};
					color: #fff;
					border: none; border-radius: 4px;
					cursor: {store.saving || publishing ? 'not-allowed' : 'pointer'};
					font-family: monospace; font-size: 0.8rem;
				"
			>
				{publishing ? 'Publishing…' : 'Publish'}
			</button>

			{#if publishError}
				<span style="color: #f87171; font-size: 0.75rem;">{publishError}</span>
			{/if}
		</div>

		<!-- Three-panel body -->
		<div style="
			display: grid;
			grid-template-columns: {store.mode === 'preview' ? '0' : '240px'} 1fr {store.mode === 'preview' ? '0' : '320px'};
			flex: 1;
			overflow: hidden;
		">
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
			style="
				position: fixed; inset: 0;
				background: rgba(0,0,0,0.7);
				display: flex; align-items: center; justify-content: center;
				z-index: 1000;
			"
			onclick={(e) => { if (e.target === e.currentTarget) publishModalOpen = false; }}
		>
			<div style="
				background: #1f2937;
				border: 1px solid #374151;
				border-radius: 8px;
				padding: 32px;
				max-width: 540px;
				width: 90%;
				font-family: monospace;
			">
				<h2 style="margin: 0 0 8px; font-size: 1.1rem; color: #f9fafb;">Your form is live.</h2>
				<p style="margin: 0 0 20px; font-size: 0.85rem; color: #9ca3af;">Share this link with respondents:</p>

				<div style="display: flex; gap: 8px; margin-bottom: 24px;">
					<input
						type="text"
						readonly
						value={shareUrl}
						style="
							flex: 1; padding: 8px 12px;
							background: #111827; border: 1px solid #374151;
							color: #d1d5db; border-radius: 4px;
							font-family: monospace; font-size: 0.8rem; outline: none;
						"
					/>
					<button
						onclick={copyShareUrl}
						style="
							padding: 8px 16px;
							background: #1d4ed8; color: #fff;
							border: none; border-radius: 4px;
							cursor: pointer; font-family: monospace; font-size: 0.8rem;
						"
					>
						Copy
					</button>
				</div>

				<div style="display: flex; justify-content: space-between; align-items: center;">
					<button
						onclick={handleRotateKey}
						disabled={publishing}
						style="
							padding: 6px 12px;
							background: transparent; color: #9ca3af;
							border: 1px solid #374151; border-radius: 4px;
							cursor: {publishing ? 'not-allowed' : 'pointer'};
							font-family: monospace; font-size: 0.75rem;
						"
					>
						{publishing ? 'Rotating…' : 'Rotate key (invalidates old links)'}
					</button>
					<button
						onclick={() => (publishModalOpen = false)}
						style="
							padding: 6px 12px;
							background: transparent; color: #6b7280;
							border: none; cursor: pointer;
							font-family: monospace; font-size: 0.75rem;
						"
					>
						Close
					</button>
				</div>

				{#if publishError}
					<p style="margin: 12px 0 0; color: #f87171; font-size: 0.8rem;">{publishError}</p>
				{/if}
			</div>
		</div>
	{/if}
{/if}
