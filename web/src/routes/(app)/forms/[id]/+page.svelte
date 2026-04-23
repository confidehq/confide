<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		getForm,
		updateFormStatus,
		updateFormExpiration,
		deleteForm,
		publishForm,
		listResponses,
		decryptResponseRecord,
		deleteResponse,
		getSchemaVersion,
		type FormRecord,
		type EncryptedResponseRecord
	} from '$lib/forms';
	import { getAppConfig } from '$lib/config';
	import type { BuilderSchema, BuilderField, MultipleChoiceConfig, CheckboxesConfig, DropdownConfig, RatingConfig } from '$lib/types/builder';
	import { RefreshCw, Copy, Check, ExternalLink, Pencil } from '@lucide/svelte';
	import Breadcrumb from '$lib/components/Breadcrumb.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	type AnswerValue = string | string[] | number | null | undefined;

	interface DecryptedResponse {
		submittedAt: string;
		locale: string;
		answers: Record<string, AnswerValue>;
		schema: BuilderSchema;
	}

	const formId = $page.params.id ?? '';

	// ── Form ──────────────────────────────────────────────────────────────────
	let record = $state<FormRecord | null>(null);
	let resolvedFormKey = $state<CryptoKey | null>(null);
	let formName = $state('');
	let loading = $state(true);
	let loadError = $state('');

	let statusSaving = $state(false);

	let expiresAt = $state('');
	let responseLimit = $state('');
	let responseTtlDays = $state('');
	let burnAfterReading = $state(false);
	let settingsSaving = $state(false);
	let settingsSaved = $state(false);
	let settingsError = $state('');

	let shareUrl = $state('');
	let publishing = $state(false);
	let publishError = $state('');
	let copied = $state(false);

	let pendingDeleteForm = $state(false);
	let deleteFormLoading = $state(false);
	let deleteFormError = $state('');

	// ── Responses ─────────────────────────────────────────────────────────────
	let responses = $state<EncryptedResponseRecord[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let hasMore = $state(false);
	let responsesLoading = $state(true);
	let loadingMore = $state(false);
	let responsesError = $state('');

	// null means "details view"; a string means a response ID is selected
	let selectedId = $state<string | null>(null);
	let decrypted = $state<Map<string, DecryptedResponse>>(new Map());
	let decrypting = $state<Set<string>>(new Set());
	let decryptErrors = $state<Map<string, string>>(new Map());

	let schemaCache = $state<Map<number, BuilderSchema>>(new Map());

	let confirmDeleteResponse = $state<string | null>(null);
	let deletingResponses = $state<Set<string>>(new Set());

	// ── Init ─────────────────────────────────────────────────────────────────
	onMount(async () => {
		if (!auth.masterKey) { goto('/login'); return; }
		try {
			const { schema, formKey } = await getForm(auth.masterKey, formId);
			formName = schema.translations[schema.defaultLocale]?.formTitle ?? null;
			resolvedFormKey = formKey;
		} catch {
			// Will surface per-response when handleDecrypt runs
		}
		await Promise.all([loadForm(), loadResponses()]);
	});

	// ── Form functions ────────────────────────────────────────────────────────
	async function loadForm() {
		if (!auth.masterKey) return;
		loading = true;
		loadError = '';
		try {
			const { schema, record: r, formKey } = await getForm(auth.masterKey, formId);
			record = r;
			resolvedFormKey = formKey;
			formName = schema.translations[schema.defaultLocale]?.formTitle ?? '';
			expiresAt = r.expiresAt ?? '';
			responseLimit = r.responseLimit != null ? String(r.responseLimit) : '';
			responseTtlDays = r.responseTtlDays != null ? String(r.responseTtlDays) : '';
			burnAfterReading = r.burnAfterReading ?? false;
		} catch (e) {
			loadError = e instanceof Error ? e.message : 'Failed to load form';
		} finally {
			loading = false;
		}
	}

	async function toggleStatus() {
		if (!record) return;
		statusSaving = true;
		const next = record.status === 'open' ? 'closed' : 'open';
		try {
			await updateFormStatus(formId, next);
			record = { ...record, status: next };
		} finally {
			statusSaving = false;
		}
	}

	async function saveSettings() {
		settingsSaving = true;
		settingsError = '';
		settingsSaved = false;
		try {
			await updateFormExpiration(
				formId,
				expiresAt || null,
				responseLimit ? parseInt(responseLimit) : null,
				responseTtlDays ? parseInt(responseTtlDays) : null,
				burnAfterReading
			);
			if (record) {
				record = {
					...record,
					expiresAt: expiresAt || null,
					responseLimit: responseLimit ? parseInt(responseLimit) : null,
					responseTtlDays: responseTtlDays ? parseInt(responseTtlDays) : null,
					burnAfterReading
				};
			}
			settingsSaved = true;
			setTimeout(() => { settingsSaved = false; }, 2500);
		} catch (e) {
			settingsError = e instanceof Error ? e.message : 'Failed to save settings';
		} finally {
			settingsSaving = false;
		}
	}

	async function handlePublish() {
		if (!auth.masterKey || !record) return;
		publishing = true;
		publishError = '';
		try {
			const salt = record.renderKeySalt
				? Uint8Array.from(atob(record.renderKeySalt), c => c.charCodeAt(0))
				: null;
			const { schema, formKey } = await getForm(auth.masterKey, formId, undefined);
			const config = await getAppConfig();
			const base = config.formsDomain ? `https://${config.formsDomain}` : undefined;
			const result = await publishForm(auth.masterKey, formId, schema as any, salt, formKey, base);
			shareUrl = result.shareUrl;
		} catch (e) {
			publishError = e instanceof Error ? e.message : 'Publish failed';
		} finally {
			publishing = false;
		}
	}

	async function copyShareUrl() {
		if (!shareUrl) return;
		await navigator.clipboard.writeText(shareUrl);
		copied = true;
		setTimeout(() => { copied = false; }, 2000);
	}

	async function confirmDeleteForm() {
		deleteFormLoading = true;
		deleteFormError = '';
		try {
			await deleteForm(formId);
			goto('/forms');
		} catch (e) {
			deleteFormError = e instanceof Error ? e.message : 'Failed to delete form';
		} finally {
			deleteFormLoading = false;
		}
	}

	// ── Response functions ────────────────────────────────────────────────────
	async function loadResponses(cursor?: string) {
		if (!auth.masterKey) return;
		if (cursor) loadingMore = true;
		else responsesLoading = true;
		responsesError = '';
		try {
			const result = await listResponses(formId, cursor, 25);
			responses = cursor ? [...responses, ...result.responses] : result.responses;
			nextCursor = result.nextCursor;
			hasMore = !!result.nextCursor;
		} catch (err) {
			responsesError = err instanceof Error ? err.message : 'Failed to load responses';
		} finally {
			responsesLoading = false;
			loadingMore = false;
		}
	}

	async function selectResponse(id: string) {
		selectedId = id;
		const rec = responses.find(r => r.id === id);
		if (rec && !decrypted.has(id) && !decrypting.has(id)) {
			await handleDecrypt(rec);
		}
	}

	async function handleDecrypt(rec: EncryptedResponseRecord) {
		if (!auth.masterKey || decrypted.has(rec.id)) return;
		decrypting = new Set([...decrypting, rec.id]);
		const errs = new Map(decryptErrors);
		errs.delete(rec.id);
		decryptErrors = errs;
		try {
			let schema = schemaCache.get(rec.schemaVersion);
			if (!schema) {
				schema = await getSchemaVersion(auth.masterKey, formId, rec.schemaVersion, resolvedFormKey ?? undefined);
				schemaCache = new Map([...schemaCache, [rec.schemaVersion, schema]]);
			}
			const payload = await decryptResponseRecord(auth.masterKey, formId, rec, resolvedFormKey ?? undefined);
			decrypted = new Map([...decrypted, [rec.id, {
				submittedAt: payload.submittedAt,
				locale: payload.locale,
				answers: payload.answers as Record<string, AnswerValue>,
				schema
			}]]);
		} catch (err) {
			decryptErrors = new Map([...decryptErrors, [rec.id, err instanceof Error ? err.message : 'Decryption failed']]);
		} finally {
			const d = new Set(decrypting);
			d.delete(rec.id);
			decrypting = d;
		}
	}

	async function handleDeleteResponse(responseId: string) {
		deletingResponses = new Set([...deletingResponses, responseId]);
		try {
			await deleteResponse(formId, responseId);
			responses = responses.filter(r => r.id !== responseId);
			const nd = new Map(decrypted);
			nd.delete(responseId);
			decrypted = nd;
			confirmDeleteResponse = null;
			if (selectedId === responseId) {
				selectedId = null;
			}
			// Update response count on the record
			if (record) record = { ...record, responseCount: Math.max(0, record.responseCount - 1) };
		} catch {
			// keep confirm open
		} finally {
			const d = new Set(deletingResponses);
			d.delete(responseId);
			deletingResponses = d;
		}
	}

	// ── Helpers ───────────────────────────────────────────────────────────────
	function renderAnswer(field: BuilderField, d: DecryptedResponse): string {
		const value = d.answers[field.id];
		const t = (d.schema.translations[d.locale] ?? d.schema.translations[d.schema.defaultLocale])?.fields[field.id];
		if (value === null || value === undefined) return '—';
		switch (field.type) {
			case 'short_text':
			case 'long_text':
				return String(value);
			case 'multiple_choice': {
				const str = String(value);
				if (str.startsWith('other:')) return `Other: ${str.slice(6)}`;
				const cfg = field.config as MultipleChoiceConfig;
				const idx = cfg.options.findIndex(o => o.id === str);
				return t?.options?.[idx] ?? str;
			}
			case 'checkboxes': {
				const arr = value as string[];
				const cfg = field.config as CheckboxesConfig;
				return arr.map(id => {
					const idx = cfg.options.findIndex(o => o.id === id);
					return t?.options?.[idx] ?? id;
				}).join(', ');
			}
			case 'dropdown': {
				const cfg = field.config as DropdownConfig;
				const idx = cfg.options.findIndex(o => o.id === String(value));
				return t?.options?.[idx] ?? String(value);
			}
			case 'date_time':
				return String(value);
			case 'rating': {
				const cfg = field.config as RatingConfig;
				return `${value} / ${cfg.scale}`;
			}
			default:
				return String(value);
		}
	}

	function formatDate(iso: string): string {
		try {
			return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
		} catch { return iso; }
	}

	function formatDateShort(iso: string): string {
		try {
			return new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
		} catch { return iso; }
	}

	function formatDateLong(iso: string): string {
		try {
			return new Date(iso).toLocaleString(undefined, { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
		} catch { return iso; }
	}

	// ── Derived ───────────────────────────────────────────────────────────────
	const statusColor = $derived(record?.status === 'open' ? 'bg-success-text-dark' : 'bg-muted-mid');
	const selectedRecord = $derived(responses.find(r => r.id === selectedId));
	const selectedDecrypted = $derived(selectedId ? decrypted.get(selectedId) : undefined);
	const isDecryptingSelected = $derived(selectedId ? decrypting.has(selectedId) : false);
	const selectedDecryptError = $derived(selectedId ? decryptErrors.get(selectedId) : undefined);
</script>

<svelte:head>
	<title>Confide — {formName || 'Form'}</title>
</svelte:head>

<style>
	@keyframes spin { to { transform: rotate(360deg); } }
	.spinner { animation: spin 0.7s linear infinite; }

	input[type="date"]::-webkit-calendar-picker-indicator {
		filter: invert(0.4);
		cursor: pointer;
	}
</style>

<!-- Form delete confirm -->
<ConfirmDialog
	open={pendingDeleteForm}
	title="Delete form?"
	description="This will permanently delete the form and all its responses. This cannot be undone."
	loading={deleteFormLoading}
	error={deleteFormError}
	onconfirm={confirmDeleteForm}
	oncancel={() => { pendingDeleteForm = false; deleteFormError = ''; }}
/>

<!-- Response delete confirm -->
<ConfirmDialog
	open={!!confirmDeleteResponse}
	title="Delete response?"
	description="This will permanently delete this response. This cannot be undone."
	loading={confirmDeleteResponse ? deletingResponses.has(confirmDeleteResponse) : false}
	onconfirm={() => confirmDeleteResponse && handleDeleteResponse(confirmDeleteResponse)}
	oncancel={() => (confirmDeleteResponse = null)}
/>

<!-- Root -->
<div class="flex flex-col flex-1 min-h-0 h-full font-mono">

	<!-- Top bar -->
	<div class="flex items-center gap-3 px-5 h-9 border-b border-border-deep shrink-0 overflow-hidden">
		<Breadcrumb items={[
			{ label: 'Forms', href: '/forms' },
			{ label: formName || formId.slice(0, 12) + '…', onclick: selectedId ? () => { selectedId = null; } : undefined },
			...(selectedRecord ? [{ label: selectedRecord.id }] : [])
		]} />
		<div class="flex-1 shrink-0"></div>
		<a
			href="/forms/{formId}/edit"
			class="shrink-0 font-mono text-sm text-muted-dim no-underline px-2.5 py-0.5 border border-border-deep rounded-sm whitespace-nowrap
				hover:text-text-body hover:border-border-subtle transition-colors duration-100 flex items-center gap-1.5"
		>
			<Pencil size={12} strokeWidth={1.75} />
			Edit form
		</a>
	</div>

	<!-- Shell -->
	<div class="flex flex-1 min-h-0 font-mono">

		<!-- Left panel: response list -->
		<div class="w-[280px] shrink-0 flex flex-col border-r border-surface-card min-h-0">

			<!-- List header -->
			<div class="px-4 py-3 border-b border-surface-card shrink-0 flex items-center justify-between gap-2">
				<p class="text-xs font-semibold tracking-[0.1em] uppercase text-muted-dim m-0 flex-1">Responses</p>
				<button
					title="Refresh"
					disabled={responsesLoading || loadingMore}
					onclick={() => loadResponses()}
					class="flex items-center justify-center w-7 h-7 bg-transparent border-none rounded cursor-pointer text-muted-dim transition-[color,background] duration-100 hover:text-muted-blue hover:bg-border-deep disabled:opacity-30 disabled:cursor-not-allowed"
				>
					<RefreshCw size={15} strokeWidth={2} />
				</button>
			</div>

			{#if responsesLoading}
				<div class="flex-1 flex items-center justify-center text-muted-dim text-sm gap-2.5">
					<div class="spinner w-3.5 h-3.5 border-2 border-surface-card border-t-[#3b82f6] rounded-full"></div>
					Loading…
				</div>
			{:else if responsesError}
				<div class="flex-1 flex items-center justify-center text-error-light text-sm p-8 text-center">{responsesError}</div>
			{:else if responses.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
					<div class="text-4xl mb-3 opacity-40">○</div>
					<p class="text-sm m-0">No responses yet</p>
				</div>
			{:else}
				<div class="flex-1 overflow-y-auto overflow-x-hidden">
					{#each responses as resp, i (resp.id)}
						<button
							onclick={() => selectResponse(resp.id)}
							class="block w-full px-4 py-[11px] text-left bg-transparent border-none border-b border-border-deep cursor-pointer transition-[background] duration-100 font-mono hover:bg-surface-3
								{selectedId === resp.id ? 'bg-[#172030] border-l-2 border-l-[#3b82f6] pl-[14px]' : ''}"
						>
							<div class="flex items-center justify-between gap-2">
								<span class="text-sm overflow-hidden text-ellipsis whitespace-nowrap flex-1
									{selectedId === resp.id ? 'text-text-blue' : 'text-muted-blue'}">
									#{i + 1} · {resp.id.slice(0, 12)}…
								</span>
								{#if decrypted.has(resp.id)}
									<span class="w-[5px] h-[5px] rounded-full bg-success-border shrink-0" title="Decrypted"></span>
								{/if}
								<span class="text-xs text-muted-dim bg-canvas border border-surface-card rounded-sm px-1.5 py-px shrink-0">v{resp.schemaVersion}</span>
							</div>
							<div class="text-xs text-muted-dim mt-0.5">{formatDateShort(resp.receivedAt)}</div>
						</button>
					{/each}
				</div>

				{#if hasMore}
					<div class="px-4 py-3 border-t border-border-deep shrink-0">
						<button
							onclick={() => loadResponses(nextCursor)}
							disabled={loadingMore}
							class="w-full px-3 py-1.5 bg-transparent text-muted-dim border border-surface-card rounded cursor-pointer font-mono text-sm transition-[color,border-color] duration-100 hover:not-disabled:text-muted-blue hover:not-disabled:border-border disabled:opacity-40 disabled:cursor-not-allowed"
						>
							{loadingMore ? 'Loading…' : 'Load more'}
						</button>
					</div>
				{/if}
			{/if}
		</div>

		<!-- Right panel -->
		<div class="flex-1 min-w-0 flex flex-col min-h-0">

			{#if selectedId === null}
				<!-- ── Details / Settings view ──────────────────────────────────── -->
				<div class="flex-1 overflow-y-auto">
					{#if loading}
						<div class="flex items-center justify-center gap-2.5 text-muted-dim text-base py-16">
							<div class="spinner w-3.5 h-3.5 border-2 border-surface-card border-t-[#3b82f6] rounded-full"></div>
							Loading…
						</div>
					{:else if loadError}
						<div class="p-8 text-base text-error-light">{loadError}</div>
					{:else if record}
						<div class="max-w-3xl px-8 py-8 flex flex-col gap-8">

							<!-- Details -->
							<section class="flex flex-col gap-4">
								<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Details</h2>

								<div class="border border-border-deep rounded-lg overflow-hidden">
									<div class="flex items-center gap-4 px-4 py-3.5 border-b border-border-deep">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Name</span>
										<span class="text-base text-text-body flex-1 min-w-0 truncate">{formName || '—'}</span>
									</div>
									<div class="flex items-center gap-4 px-4 py-3.5 border-b border-border-deep">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Status</span>
										<div class="flex items-center gap-2.5 flex-1">
											<span class="w-2 h-2 rounded-full shrink-0 {statusColor}"></span>
											<span class="text-base text-text-body capitalize">{record.status}</span>
											<button
												onclick={toggleStatus}
												disabled={statusSaving}
												class="ml-auto px-3 py-1.5 text-sm font-mono border rounded cursor-pointer transition-colors duration-100
													{statusSaving
														? 'bg-transparent text-muted-mid border-border-deep cursor-not-allowed'
														: record.status === 'open'
															? 'bg-transparent text-error-light border-border-danger-muted hover:bg-danger-hover hover:border-border-danger-dark'
															: 'bg-transparent text-success-text-dark border-[#1e3a20] hover:bg-[#0e1a0e] hover:border-success-text'}"
											>
												{statusSaving ? '…' : record.status === 'open' ? 'Close' : 'Open'}
											</button>
										</div>
									</div>
									<div class="flex items-center gap-4 px-4 py-3.5 border-b border-border-deep">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Form ID</span>
										<span class="text-base text-muted-dim font-mono flex-1 min-w-0 truncate">{formId}</span>
									</div>
									<div class="flex items-center gap-4 px-4 py-3.5 border-b border-border-deep">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Responses</span>
										<span class="text-base text-text-body tabular-nums">{record.responseCount}</span>
									</div>
									<div class="flex items-center gap-4 px-4 py-3.5 border-b border-border-deep">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Version</span>
										<span class="text-base text-muted-dim tabular-nums">v{record.schemaVersion}</span>
									</div>
									<div class="flex items-center gap-4 px-4 py-3.5 border-b border-border-deep">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Created</span>
										<span class="text-base text-muted-dim">{formatDate(record.createdAt)}</span>
									</div>
									<div class="flex items-center gap-4 px-4 py-3.5">
										<span class="w-32 shrink-0 text-sm text-muted-dim">Updated</span>
										<span class="text-base text-muted-dim">{formatDate(record.updatedAt)}</span>
									</div>
								</div>
							</section>

							<!-- Share link -->
							<section class="flex flex-col gap-4">
								<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Share link</h2>

								{#if shareUrl}
									<div class="flex items-center gap-2">
										<input
											readonly
											value={shareUrl}
											class="flex-1 min-w-0 text-sm text-muted-dim bg-canvas border border-border-deep rounded px-3 py-2 font-mono outline-none"
										/>
										<button
											onclick={copyShareUrl}
											class="shrink-0 flex items-center gap-1.5 px-4 py-2 border rounded font-mono text-sm cursor-pointer transition-colors duration-100
												{copied
													? 'bg-[#0e1a0e] text-success-text-dark border-success-text'
													: 'bg-transparent text-muted-dim border-border-deep hover:text-text-body hover:border-border-subtle'}"
										>
											{#if copied}
												<Check size={13} strokeWidth={2} />
												Copied
											{:else}
												<Copy size={13} strokeWidth={1.75} />
												Copy
											{/if}
										</button>
									</div>
									<p class="m-0 text-sm text-muted-mid">Anyone with this link can submit a response. Rotate the key on the edit page to invalidate old links.</p>
								{:else}
									<div class="flex flex-col gap-2">
										{#if publishError}
											<p class="m-0 text-sm text-error-light">{publishError}</p>
										{/if}
										<div>
											<button
												onclick={handlePublish}
												disabled={publishing}
												class="flex items-center gap-2 px-4 py-2 border rounded font-mono text-base cursor-pointer transition-colors duration-100
													{publishing
														? 'bg-transparent text-muted-mid border-border-deep cursor-not-allowed'
														: 'bg-transparent text-text-blue border-[#1e3a5c] hover:bg-[#0e1a30] hover:border-info-border'}"
											>
												{#if publishing}
													<div class="spinner w-3.5 h-3.5 border-2 border-surface-card border-t-[#3b82f6] rounded-full"></div>
													Generating…
												{:else}
													<ExternalLink size={14} strokeWidth={1.75} />
													{record.renderKeySalt ? 'Get share link' : 'Publish form'}
												{/if}
											</button>
											{#if !record.renderKeySalt}
												<p class="mt-2 m-0 text-sm text-muted-mid">Publishing generates an encrypted share link for respondents.</p>
											{/if}
										</div>
									</div>
								{/if}
							</section>

							<!-- Settings -->
							<section class="flex flex-col gap-4">
								<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Settings</h2>

								<div class="border border-border-deep rounded-lg overflow-hidden">
									<div class="flex items-start gap-4 px-4 py-4 border-b border-border-deep">
										<div class="w-48 shrink-0 pt-px">
											<p class="m-0 text-sm text-muted-blue">Expiration date</p>
											<p class="m-0 mt-0.5 text-xs text-muted-mid leading-snug">Close form automatically on this date</p>
										</div>
										<input
											type="date"
											bind:value={expiresAt}
											class="flex-1 min-w-0 text-base text-text-body bg-canvas border border-border-deep rounded px-3 py-2 font-mono outline-none
												focus:border-info-border transition-colors duration-100"
										/>
									</div>
									<div class="flex items-start gap-4 px-4 py-4 border-b border-border-deep">
										<div class="w-48 shrink-0 pt-px">
											<p class="m-0 text-sm text-muted-blue">Response limit</p>
											<p class="m-0 mt-0.5 text-xs text-muted-mid leading-snug">Close after this many responses</p>
										</div>
										<input
											type="number"
											min="1"
											placeholder="No limit"
											bind:value={responseLimit}
											class="flex-1 min-w-0 text-base text-text-body bg-canvas border border-border-deep rounded px-3 py-2 font-mono outline-none
												focus:border-info-border transition-colors duration-100 placeholder:text-muted-mid"
										/>
									</div>
									<div class="flex items-start gap-4 px-4 py-4 border-b border-border-deep">
										<div class="w-48 shrink-0 pt-px">
											<p class="m-0 text-sm text-muted-blue">Response TTL (days)</p>
											<p class="m-0 mt-0.5 text-xs text-muted-mid leading-snug">Auto-delete responses after N days</p>
										</div>
										<input
											type="number"
											min="1"
											placeholder="Never"
											bind:value={responseTtlDays}
											class="flex-1 min-w-0 text-base text-text-body bg-canvas border border-border-deep rounded px-3 py-2 font-mono outline-none
												focus:border-info-border transition-colors duration-100 placeholder:text-muted-mid"
										/>
									</div>
									<div class="flex items-center gap-4 px-4 py-4">
										<div class="w-48 shrink-0">
											<p class="m-0 text-sm text-muted-blue">Burn after reading</p>
											<p class="m-0 mt-0.5 text-xs text-muted-mid leading-snug">Delete each response once viewed</p>
										</div>
										<button
											onclick={() => { burnAfterReading = !burnAfterReading; }}
											role="switch"
											aria-checked={burnAfterReading}
											class="relative shrink-0 w-9 h-5 rounded-full border transition-colors duration-150 cursor-pointer outline-none
												{burnAfterReading
													? 'bg-primary border-info-border'
													: 'bg-canvas border-border-deep hover:border-muted-mid'}"
										>
											<span
												class="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-[left] duration-150
													{burnAfterReading ? 'left-[18px]' : 'left-0.5'}"
											></span>
										</button>
									</div>
								</div>

								<div class="flex items-center gap-3">
									<button
										onclick={saveSettings}
										disabled={settingsSaving}
										class="px-4 py-2 border rounded font-mono text-base cursor-pointer transition-colors duration-100
											{settingsSaving
												? 'bg-transparent text-muted-mid border-border-deep cursor-not-allowed'
												: 'bg-transparent text-text-blue border-[#1e3a5c] hover:bg-[#0e1a30] hover:border-info-border'}"
									>
										{settingsSaving ? 'Saving…' : 'Save settings'}
									</button>
									{#if settingsSaved}
										<span class="text-sm text-success-text-dark flex items-center gap-1">
											<Check size={13} strokeWidth={2} />
											Saved
										</span>
									{/if}
									{#if settingsError}
										<span class="text-sm text-error-light">{settingsError}</span>
									{/if}
								</div>
							</section>

							<!-- Danger zone -->
							<section class="flex flex-col gap-4">
								<h2 class="m-0 text-base font-semibold tracking-[0.08em] uppercase text-muted-mid">Danger zone</h2>

								<div class="border border-border-danger-muted rounded-lg px-4 py-4 flex items-center justify-between gap-4">
									<div class="min-w-0">
										<p class="m-0 text-base text-text-body">Delete this form</p>
										<p class="m-0 mt-0.5 text-sm text-muted-dim">
											Permanently deletes the form and all {record.responseCount} response{record.responseCount === 1 ? '' : 's'}. Cannot be undone.
										</p>
									</div>
									<button
										onclick={() => { pendingDeleteForm = true; }}
										class="shrink-0 px-4 py-2 bg-transparent text-error-light border border-border-danger-muted rounded cursor-pointer font-mono text-base
											hover:bg-danger-hover hover:border-border-danger-dark transition-colors duration-100"
									>Delete</button>
								</div>
							</section>

						</div>
					{/if}
				</div>

			{:else}
				<!-- ── Response detail view ───────────────────────────────────── -->
				{#if !selectedRecord}
					<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
						<p class="text-sm m-0">Response not found</p>
					</div>
				{:else}
					<!-- Detail header -->
					<div class="px-6 pt-5 pb-4 border-b border-surface-card shrink-0 flex items-start justify-between gap-4">
						<div class="min-w-0">
							<p class="text-base text-muted-blue m-0 mb-1 overflow-hidden text-ellipsis whitespace-nowrap">{selectedRecord.id}</p>
							<p class="text-sm text-muted-dim m-0">
								Received {formatDateLong(selectedRecord.receivedAt)}
								{#if selectedDecrypted}
									<span class="inline-block text-xs text-muted-dim bg-canvas border border-surface-card rounded-sm px-1.5 py-px ml-2 align-middle">{selectedDecrypted.locale}</span>
								{/if}
							</p>
						</div>
						<div class="flex items-center gap-2 shrink-0">
							<button
								onclick={() => (confirmDeleteResponse = selectedRecord.id)}
								class="px-4 py-2 bg-transparent text-error-light border border-border-deep rounded cursor-pointer font-mono text-base transition-colors duration-100 hover:bg-danger-hover hover:border-border-danger-dark"
							>
								Delete
							</button>
						</div>
					</div>

					<!-- Detail content -->
					<div class="flex-1 overflow-y-auto p-6">
						{#if isDecryptingSelected}
							<div class="flex items-center gap-2.5 text-muted-dim text-base py-8">
								<div class="spinner w-3.5 h-3.5 border-2 border-surface-card border-t-[#3b82f6] rounded-full"></div>
								Decrypting…
							</div>
						{:else if selectedDecryptError}
							<p class="text-error-light text-base py-3 m-0">{selectedDecryptError}</p>
						{:else if selectedDecrypted}
							<div class="flex flex-col gap-6">
								{#each selectedDecrypted.schema.fields as field (field.id)}
									{#if field.type !== 'section_break'}
										{@const fieldT = (selectedDecrypted.schema.translations[selectedDecrypted.locale] ?? selectedDecrypted.schema.translations[selectedDecrypted.schema.defaultLocale])?.fields[field.id]}
										{@const answer = renderAnswer(field, selectedDecrypted)}
										<div class="border-b border-border-deep pb-6 last:border-b-0 last:pb-0">
											<p class="text-sm font-semibold tracking-[0.08em] uppercase text-muted-dim m-0 mb-2">
												{fieldT?.label ?? field.id}{#if field.required}<span class="text-error-light ml-0.5">*</span>{/if}
											</p>
											<p class="text-base text-text-body m-0 leading-relaxed whitespace-pre-wrap break-words
												{answer === '—' ? 'text-[#3a4f63] italic' : ''}">
												{answer}
											</p>
										</div>
									{/if}
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			{/if}

		</div>
	</div>
</div>
