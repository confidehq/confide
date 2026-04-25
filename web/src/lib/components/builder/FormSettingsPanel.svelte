<script lang="ts">
	import { onMount } from 'svelte';
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type { CustomDomainInfo } from '$lib/workspaces';
	import { auth } from '$lib/stores/auth.svelte';
	import { publishForm, rotateRenderKey, deriveShareUrl } from '$lib/forms';
	import { Copy, Check } from '@lucide/svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	interface Props {
		store: ReturnType<typeof createBuilderStore>;
		formId: string;
		workspaceDomain: CustomDomainInfo | null;
		useCustomDomain: boolean;
		customDomainBase: () => string | undefined;
		onToggleCustomDomain: () => Promise<void>;
	}

	const { store, formId, workspaceDomain, useCustomDomain, customDomainBase, onToggleCustomDomain }: Props = $props();

	let closeOnDatePending = $state(false);
	let limitResponsesPending = $state(false);
	let autoDeletePending = $state(false);
	const closeOnDateOpen = $derived(!!store.expiresAt || closeOnDatePending);
	const limitResponsesOpen = $derived(!!store.responseLimit || limitResponsesPending);

	let shareUrl = $state('');
	let publishing = $state(false);
	let publishError = $state('');
	let copied = $state(false);
	let copiedTimer: ReturnType<typeof setTimeout> | null = null;
	let confirmRotate = $state(false);

	const isConvo = $derived(store.schema.layout === 'convo');

	let expirationSaving = $state(false);
	let expirationError = $state<string | null>(null);

	// Derive share URL on mount if form was previously published
	onMount(async () => {
		if (store.formStatus !== 'draft' && store.renderKeySalt && store.formKey) {
			try {
				const saltBase64 = btoa(String.fromCharCode(...store.renderKeySalt));
				shareUrl = await deriveShareUrl(formId, saltBase64, store.formKey, customDomainBase());
			} catch {
				// non-critical: share URL will be shown after next publish
			}
		}
	});

	const isFirstPublish = $derived(store.formStatus === 'draft');
	const publishButtonLabel = $derived(
		publishing ? 'Publishing…'
		: isFirstPublish ? 'Publish'
		: store.hasUnpublishedChanges ? 'Update'
		: 'Up to date'
	);
	const publishButtonDisabled = $derived(
		store.saving || publishing || (!isFirstPublish && !store.hasUnpublishedChanges)
	);

	async function handlePublish() {
		if (!auth.masterKey) return;
		publishing = true;
		publishError = '';
		try {
			await store.flushSave();
			const result = await publishForm(auth.masterKey, formId, store.schema, store.renderKeySalt, store.formKey ?? undefined, customDomainBase());
			store.setRenderKeySalt(result.renderKeySalt);
			store.markPublished();
			shareUrl = result.shareUrl;
		} catch (err) {
			publishError = err instanceof Error ? err.message : 'Publish failed';
		} finally {
			publishing = false;
		}
	}

	async function handleRotateKey() {
		if (!auth.masterKey) return;
		publishing = true;
		publishError = '';
		try {
			const result = await rotateRenderKey(auth.masterKey, formId, store.schema, store.formKey ?? undefined, customDomainBase());
			store.setRenderKeySalt(result.renderKeySalt);
			store.markPublished();
			shareUrl = result.shareUrl;
		} catch (err) {
			publishError = err instanceof Error ? err.message : 'Key rotation failed';
		} finally {
			publishing = false;
		}
	}

	function copyShareUrl() {
		navigator.clipboard.writeText(shareUrl);
		copied = true;
		if (copiedTimer) clearTimeout(copiedTimer);
		copiedTimer = setTimeout(() => { copied = false; }, 2000);
	}

	async function applyExpiration(newExpiresAt: string | null, newResponseLimit: number | null, newTtlDays: number | null, newBurnAfterReading: boolean) {
		expirationSaving = true;
		expirationError = null;
		try {
			await store.setExpiration(newExpiresAt, newResponseLimit, newTtlDays, newBurnAfterReading);
		} catch {
			expirationError = 'Failed to save — please try again.';
		} finally {
			expirationSaving = false;
		}
	}

	type ResponseLifetimePolicy = 'none' | 'burn' | 'ttl';

	const responseLifetimePolicy = $derived<ResponseLifetimePolicy>(
		store.burnAfterReading ? 'burn' : store.responseTtlDays ? 'ttl' : 'none'
	);

	const autoDeleteOpen = $derived(responseLifetimePolicy !== 'none' || autoDeletePending);

	function applyResponseLifetime(policy: ResponseLifetimePolicy, ttlDays: number | null) {
		const burn = policy === 'burn';
		const days = policy === 'ttl' ? ttlDays : null;
		applyExpiration(store.expiresAt, store.responseLimit, days, burn);
	}
</script>

<aside
	class="form-settings-panel {store.showFormSettings ? 'is-open' : ''}
		fixed bottom-0 left-0 right-0 max-h-[65vh] rounded-t-xl
		sm:absolute sm:top-2 sm:bottom-2 sm:left-auto sm:right-2 sm:w-[380px] sm:max-h-none sm:rounded-xl
		bg-canvas border border-border-deep overflow-y-auto z-20 flex flex-col"
>
	<!-- Mobile drag handle -->
	<div class="sm:hidden flex justify-center pt-2.5 pb-1 shrink-0 sticky top-0 bg-canvas">
		<div class="w-8 h-1 bg-border rounded-full"></div>
	</div>

	<!-- Header -->
	<div class="flex items-center px-5 h-9 shrink-0 border-b border-border-deep sticky top-0 bg-canvas z-10">
		<p class="m-0 text-sm text-muted-dark uppercase tracking-[0.05em]">Settings</p>
	</div>

	<!-- Scrollable content -->
	<div class="flex-1 overflow-y-auto">

	<!-- Publish section -->
	<div class="p-5">
		<div class="flex flex-col gap-3">
				{#if shareUrl}
					<div class="flex gap-1.5">
						<input
							type="text"
							readonly
							value={shareUrl}
							class="flex-1 px-3 py-2 bg-surface border border-border-deep text-text-dim rounded-md font-mono text-sm outline-none min-w-0"
						/>
						<button
							onclick={copyShareUrl}
							class="shrink-0 px-3 py-2 border-none rounded-md font-mono text-sm transition-[background] duration-150 flex items-center gap-1.5
								{copied ? 'bg-success-muted text-success cursor-default' : 'bg-primary text-white hover:bg-primary-hover cursor-pointer'}"
						>
							{#if copied}
								<Check size={13} strokeWidth={2} />Copied
							{:else}
								<Copy size={13} strokeWidth={1.75} />Copy link
							{/if}
						</button>
					</div>
					<p class="m-0 text-xs text-muted-dark">Anyone with the link can access this form.</p>

					{#if workspaceDomain?.verified && workspaceDomain.domain}
						<div class="flex items-center gap-2.5">
							<button
								role="switch"
								aria-checked={useCustomDomain}
								onclick={onToggleCustomDomain}
								class="relative w-8 h-[18px] rounded-full transition-colors duration-150 border-none cursor-pointer shrink-0
									{useCustomDomain ? 'bg-primary' : 'bg-border-deep'}"
							>
								<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
									{useCustomDomain ? 'translate-x-[14px]' : 'translate-x-0'}"></span>
							</button>
							<span class="text-sm text-muted font-mono">
								Serve on <span class="text-text-dim">{workspaceDomain.domain}</span>
							</span>
						</div>
					{/if}

					<button
						onclick={() => { confirmRotate = true; }}
						disabled={publishing}
						class="px-3 py-2 bg-transparent text-muted border border-border-deep rounded-md cursor-pointer font-mono text-sm
							{publishing ? 'cursor-not-allowed opacity-60' : 'hover:text-text-dim hover:border-border transition-colors duration-100'}"
					>Generate new link</button>
				{:else}
					<div class="py-4 flex flex-col items-center gap-2 text-center">
						<p class="m-0 text-sm text-text-dim">This form is unpublished</p>
						<p class="m-0 text-xs text-muted-dark">Publish to make it accessible and get a share link.</p>
					</div>
				{/if}
			</div>
	</div>

	<div class="h-px bg-border-deep"></div>

	<div class="p-5 flex flex-col gap-3.5">
		<!-- Form name -->
		<div>
			<label class="block text-sm text-muted mb-1">Form name</label>
			<input
				type="text"
				placeholder="Internal name…"
				value={store.schema.name}
				oninput={(e) => store.setName((e.target as HTMLInputElement).value)}
				class="input-base"
			/>
			<p class="mt-1 m-0 text-xs text-muted-dark">Used in your dashboard only.</p>
		</div>

		{#if isConvo}
			<div>
				<label class="block text-sm text-muted mb-1">Completion message</label>
				<textarea
					value={store.activeTranslation?.convoCompletionMessage ?? ''}
					oninput={(e) => store.updateTranslation(null, 'convoCompletionMessage', (e.target as HTMLTextAreaElement).value)}
					rows={2}
					class="input-base"
				></textarea>
			</div>
			<div class="flex items-center justify-between">
				<label class="text-sm text-text-dim">Allow edit after submit</label>
				<input
					type="checkbox"
					checked={store.schema.convoAllowEdit ?? false}
					onchange={(e) => store.setConvoAllowEdit((e.target as HTMLInputElement).checked)}
				/>
			</div>
		{/if}

		<!-- Scheduling options -->
		<div class="border-t border-border-deep pt-4 flex flex-col divide-y divide-border-deep">

			<!-- Close on date -->
			<div class="py-3 first:pt-0">
				<div class="flex items-center justify-between gap-3">
					<div>
						<p class="m-0 text-sm text-text-dim">Close on date</p>
						<p class="m-0 text-xs text-muted-dark mt-0.5">Stop accepting responses after a date.</p>
					</div>
					<button
						role="switch"
						aria-checked={closeOnDateOpen}
						onclick={() => {
							if (closeOnDateOpen) {
								closeOnDatePending = false;
								applyExpiration(null, store.responseLimit, store.responseTtlDays, store.burnAfterReading);
							} else {
								closeOnDatePending = true;
							}
						}}
						class="relative shrink-0 w-8 h-[18px] rounded-full transition-colors duration-150 border-none cursor-pointer
							{closeOnDateOpen ? 'bg-primary' : 'bg-border-deep'}"
					>
						<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
							{closeOnDateOpen ? 'translate-x-[14px]' : 'translate-x-0'}"></span>
					</button>
				</div>
				{#if closeOnDateOpen}
					<div class="mt-2.5">
						<input
							type="date"
							value={store.expiresAt ?? ''}
							onchange={(e) => {
								const v = (e.target as HTMLInputElement).value;
								applyExpiration(v || null, store.responseLimit, store.responseTtlDays, store.burnAfterReading);
							}}
							class="input-base"
						/>
					</div>
				{/if}
			</div>

			<!-- Limit responses -->
			<div class="py-3">
				<div class="flex items-center justify-between gap-3">
					<div>
						<p class="m-0 text-sm text-text-dim">Limit total responses</p>
						<p class="m-0 text-xs text-muted-dark mt-0.5">Close after a set number of submissions.</p>
					</div>
					<button
						role="switch"
						aria-checked={limitResponsesOpen}
						onclick={() => {
							if (limitResponsesOpen) {
								limitResponsesPending = false;
								applyExpiration(store.expiresAt, null, store.responseTtlDays, store.burnAfterReading);
							} else {
								limitResponsesPending = true;
							}
						}}
						class="relative shrink-0 w-8 h-[18px] rounded-full transition-colors duration-150 border-none cursor-pointer
							{limitResponsesOpen ? 'bg-primary' : 'bg-border-deep'}"
					>
						<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
							{limitResponsesOpen ? 'translate-x-[14px]' : 'translate-x-0'}"></span>
					</button>
				</div>
				{#if limitResponsesOpen}
					<div class="mt-2.5">
						<input
							type="number"
							min="1"
							placeholder="e.g. 100"
							value={store.responseLimit ?? ''}
							onchange={(e) => {
								const v = parseInt((e.target as HTMLInputElement).value);
								applyExpiration(store.expiresAt, v > 0 ? v : null, store.responseTtlDays, store.burnAfterReading);
							}}
							class="input-base"
						/>
					</div>
				{/if}
			</div>

			<!-- Auto delete -->
			<div class="py-3">
				<div class="flex items-center justify-between gap-3">
					<div>
						<p class="m-0 text-sm text-text-dim">Auto delete responses</p>
						<p class="m-0 text-xs text-muted-dark mt-0.5">Remove responses from our servers after a set period.</p>
					</div>
					<button
						role="switch"
						aria-checked={autoDeleteOpen}
						onclick={() => {
							if (autoDeleteOpen) {
								autoDeletePending = false;
								applyResponseLifetime('none', null);
							} else {
								autoDeletePending = true;
							}
						}}
						class="relative shrink-0 w-8 h-[18px] rounded-full transition-colors duration-150 border-none cursor-pointer
							{autoDeleteOpen ? 'bg-primary' : 'bg-border-deep'}"
					>
						<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
							{autoDeleteOpen ? 'translate-x-[14px]' : 'translate-x-0'}"></span>
					</button>
				</div>
				{#if autoDeleteOpen}
					<div class="mt-2.5 flex flex-col gap-2.5">
						<select
							value={responseLifetimePolicy === 'none' ? 'burn' : responseLifetimePolicy}
							onchange={(e) => {
								const policy = (e.target as HTMLSelectElement).value as ResponseLifetimePolicy;
								applyResponseLifetime(policy, policy === 'ttl' ? (store.responseTtlDays ?? 30) : null);
							}}
							class="input-base"
						>
							<option value="burn">Burn after reading</option>
							<option value="ttl">Delete after a set period</option>
						</select>
						{#if responseLifetimePolicy === 'ttl'}
							<div class="flex gap-1.5 items-center">
								<input
									type="number"
									min="1"
									placeholder="Days"
									value={store.responseTtlDays ?? ''}
									onchange={(e) => {
										const v = parseInt((e.target as HTMLInputElement).value);
										applyResponseLifetime('ttl', v > 0 ? v : null);
									}}
									class="input-base"
								/>
								<span class="text-sm text-muted shrink-0">days</span>
							</div>
						{:else if responseLifetimePolicy === 'burn'}
							<p class="m-0 text-xs text-muted-dark leading-relaxed">Responses are scheduled for deletion once you view them. They remain visible until the next cleanup pass.</p>
						{/if}
					</div>
				{/if}
			</div>

		</div>

		{#if expirationSaving}
			<p class="m-0 text-xs text-muted-dark">Saving…</p>
		{:else if expirationError}
			<p class="m-0 text-xs text-error">{expirationError}</p>
		{/if}
	</div>

	</div>

	<ConfirmDialog
		open={confirmRotate}
		title="Generate new link?"
		description="This will replace the current share link. Anyone using the old link will no longer be able to access this form."
		confirmLabel="Generate new link"
		onconfirm={() => { confirmRotate = false; handleRotateKey(); }}
		oncancel={() => { confirmRotate = false; }}
	/>

	<!-- Sticky publish button -->
	<div class="shrink-0 p-3 border-t border-border-deep bg-canvas">
		{#if publishError}
			<p class="m-0 mb-2 text-xs text-error-light">{publishError}</p>
		{/if}
		<button
			onclick={handlePublish}
			disabled={publishButtonDisabled}
			class="w-full py-2 text-white border-none rounded-md font-mono text-sm transition-[background,opacity] duration-100
				{publishButtonDisabled ? 'bg-info-bg-dark cursor-not-allowed opacity-70' : 'bg-primary hover:bg-primary-hover cursor-pointer'}"
		>{publishButtonLabel}</button>
	</div>
</aside>

<style>
	.form-settings-panel {
		transform: translateY(100%);
		transition: transform 0.2s ease;
	}
	.form-settings-panel.is-open {
		transform: translateY(0);
	}
	@media (min-width: 640px) {
		.form-settings-panel {
			transform: translateX(calc(100% + 8px));
		}
		.form-settings-panel.is-open {
			transform: translateX(0);
		}
	}
</style>
