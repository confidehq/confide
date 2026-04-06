<script lang="ts">
	import { onMount, setContext } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { createBuilderStore } from '$lib/stores/builder.svelte';
	import { publishForm, rotateRenderKey } from '$lib/forms';
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
			// Pass existing salt so the share URL stays stable across publishes.
			// On first publish renderKeySalt is null and a new salt is generated.
			const result = await publishForm(auth.masterKey, formId, store.schema, store.renderKeySalt);
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
			const result = await rotateRenderKey(auth.masterKey, formId, store.schema);
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
			display: flex; align-items: center; gap: 8px;
			padding: 0 12px;
			height: 44px;
			background: #161d28;
			border-bottom: 1px solid #2a3341;
			flex-shrink: 0;
		">
			<!-- Form name input -->
			<input
				type="text"
				placeholder="Untitled form"
				value={store.schema.name}
				oninput={(e) => store!.setName((e.target as HTMLInputElement).value)}
				style="
					background: transparent; border: none; outline: none;
					color: #e5e7eb; font-family: monospace; font-size: 0.875rem;
					width: 200px; min-width: 0;
					padding: 4px 6px;
					border-radius: 4px;
					transition: background 0.1s;
				"
				onfocus={(e) => { (e.target as HTMLInputElement).style.background = '#1f2937'; }}
				onblur={(e) => { (e.target as HTMLInputElement).style.background = 'transparent'; }}
			/>

			<div style="width: 1px; height: 18px; background: #2a3341; flex-shrink: 0;"></div>

			<!-- Layout selector -->
			<div style="position: relative; display: flex; align-items: center;">
				<select
					value={store.schema.layout}
					onchange={(e) => store!.setLayout((e.target as HTMLSelectElement).value as 'scroll' | 'steps' | 'convo')}
					style="
						appearance: none; -webkit-appearance: none;
						padding: 0 28px 0 10px;
						height: 28px;
						background: #1f2937;
						color: #9ca3af;
						border: 1px solid #2a3341;
						border-radius: 5px;
						cursor: pointer;
						font-family: monospace;
						font-size: 0.75rem;
						outline: none;
						line-height: 1;
					"
				>
					<option value="scroll">Scroll</option>
					<option value="steps">Steps</option>
					<option value="convo">Convo</option>
				</select>
				<svg style="position: absolute; right: 7px; top: 50%; transform: translateY(-50%); pointer-events: none;" width="10" height="10" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M2 3.5L5 6.5L8 3.5" stroke="#4b5563" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
			</div>

			<div style="width: 1px; height: 18px; background: #2a3341; flex-shrink: 0;"></div>

			<!-- Locale switcher -->
			<div style="display: flex; align-items: center; gap: 6px;">
				<div style="position: relative; display: flex; align-items: center;">
					<select
						value={store.activeLocale}
						onchange={(e) => store!.setActiveLocale((e.target as HTMLSelectElement).value)}
						style="
							appearance: none; -webkit-appearance: none;
							padding: 0 28px 0 10px;
							height: 28px;
							background: #1f2937;
							color: #9ca3af;
							border: 1px solid #2a3341;
							border-radius: 5px;
							cursor: pointer;
							font-family: monospace;
							font-size: 0.75rem;
							outline: none;
							line-height: 1;
						"
					>
						{#each store.schema.locales as locale}
							<option value={locale}>{locale}</option>
						{/each}
					</select>
					<svg style="position: absolute; right: 7px; top: 50%; transform: translateY(-50%); pointer-events: none;" width="10" height="10" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M2 3.5L5 6.5L8 3.5" stroke="#4b5563" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
				</div>
				{#if showLocaleInput}
					<input
						type="text"
						placeholder="e.g. fr"
						bind:value={newLocaleInput}
						onkeydown={(e) => { if (e.key === 'Enter') handleAddLocale(); if (e.key === 'Escape') showLocaleInput = false; }}
						style="
							width: 56px; padding: 0 8px; height: 28px;
							background: #1f2937; border: 1px solid #2a3341;
							color: #d1d5db; border-radius: 5px;
							font-family: monospace; font-size: 0.75rem; outline: none;
							box-sizing: border-box;
						"
					/>
					<button
						onclick={handleAddLocale}
						style="
							padding: 0 10px; height: 28px;
							background: #1d4ed8; color: #fff;
							border: none; border-radius: 5px;
							cursor: pointer; font-family: monospace; font-size: 0.75rem;
						"
					>Add</button>
				{:else}
					<button
						onclick={() => (showLocaleInput = true)}
						style="
							padding: 0 8px; height: 28px;
							background: transparent; color: #4b5563;
							border: 1px dashed #2a3341;
							border-radius: 5px; cursor: pointer;
							font-family: monospace; font-size: 0.75rem;
							transition: color 0.1s, border-color 0.1s;
						"
						onmouseenter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#6b7280'; (e.currentTarget as HTMLButtonElement).style.borderColor = '#374151'; }}
						onmouseleave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#4b5563'; (e.currentTarget as HTMLButtonElement).style.borderColor = '#2a3341'; }}
					>+ lang</button>
				{/if}
			</div>

			<!-- Spacer -->
			<div style="flex: 1;"></div>

			<!-- Save indicator -->
			<span style="font-size: 0.7rem; color: #374151; letter-spacing: 0.02em;">
				{saveIndicatorText(store)}
			</span>

			<!-- Form settings cog -->
			<button
				onclick={() => store!.setShowFormSettings(!store.showFormSettings)}
				title="Form settings"
				style="
					padding: 0 8px; height: 28px;
					background: {store.showFormSettings ? '#1f2937' : 'transparent'};
					color: {store.showFormSettings ? '#e5e7eb' : '#4b5563'};
					border: 1px solid {store.showFormSettings ? '#374151' : 'transparent'};
					border-radius: 5px; cursor: pointer;
					font-size: 0.9rem; line-height: 1;
					transition: color 0.1s;
				"
				onmouseenter={(e) => { if (!store.showFormSettings) (e.currentTarget as HTMLButtonElement).style.color = '#9ca3af'; }}
				onmouseleave={(e) => { if (!store.showFormSettings) (e.currentTarget as HTMLButtonElement).style.color = '#4b5563'; }}
			>⚙</button>

			<div style="width: 1px; height: 18px; background: #2a3341; flex-shrink: 0;"></div>

			<!-- Preview toggle -->
			<button
				onclick={() => store!.setMode(store!.mode === 'edit' ? 'preview' : 'edit')}
				style="
					padding: 0 12px; height: 28px;
					background: {store.mode === 'preview' ? '#1f2937' : 'transparent'};
					color: {store.mode === 'preview' ? '#e5e7eb' : '#6b7280'};
					border: 1px solid {store.mode === 'preview' ? '#374151' : '#2a3341'};
					border-radius: 5px; cursor: pointer;
					font-family: monospace; font-size: 0.75rem;
				"
			>{store.mode === 'preview' ? 'Edit' : 'Preview'}</button>

			<!-- Publish button -->
			<button
				onclick={handlePublish}
				disabled={store.saving || publishing}
				style="
					padding: 0 14px; height: 28px;
					background: {store.saving || publishing ? '#1e3a8a' : '#2563eb'};
					color: #fff;
					border: none; border-radius: 5px;
					cursor: {store.saving || publishing ? 'not-allowed' : 'pointer'};
					font-family: monospace; font-size: 0.75rem;
					opacity: {store.saving || publishing ? '0.7' : '1'};
				"
			>{publishing ? 'Publishing…' : 'Publish'}</button>

			{#if publishError}
				<span style="color: #f87171; font-size: 0.7rem;">{publishError}</span>
			{/if}
		</div>

		<!-- Body -->
		<div style="
			display: grid;
			grid-template-columns: {store.mode === 'preview' ? '1fr' : '240px 1fr'};
			flex: 1;
			overflow: hidden;
			position: relative;
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
							background: {copied ? '#16a34a' : '#1d4ed8'}; color: #fff;
							border: none; border-radius: 4px;
							cursor: pointer; font-family: monospace; font-size: 0.8rem;
							transition: background 0.15s;
						"
					>
						{copied ? 'Copied!' : 'Copy'}
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
