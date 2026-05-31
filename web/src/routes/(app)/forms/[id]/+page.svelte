<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import {
		getForm,
		updateFormStatus,
		updateFormExpiration,
		updateFormPGPNotification,
		validatePGPKey,
		deleteForm,
		publishForm,
		rotateRenderKey,
		deriveShareUrl,
		listResponses,
		decryptResponseRecord,
		deleteResponse,
		getSchemaVersion,
		type FormRecord,
		type EncryptedResponseRecord
	} from '$lib/forms';
	import { getAppConfig } from '$lib/config';
	import { getCustomDomain, type CustomDomainInfo } from '$lib/workspaces';
	import type { BuilderSchema, BuilderField, MultipleChoiceConfig, CheckboxesConfig, DropdownConfig, RatingConfig } from '$lib/types/builder';
	import { RefreshCw, Copy, Check, ExternalLink, Pencil, Link, QrCode, Download } from '@lucide/svelte';
	import Breadcrumb from '$lib/components/Breadcrumb.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import QRCode from 'qrcode';

	type AnswerValue = string | string[] | number | null | undefined;

	interface DecryptedResponse {
		submittedAt: string;
		locale: string;
		answers: Record<string, AnswerValue>;
		schema: BuilderSchema;
	}

	const formId = $page.params.id ?? '';

	// Navigate to /forms if the workspace changes while viewing this form
	const mountedWorkspaceId = workspacesStore.active?.id;
	$effect(() => {
		const id = workspacesStore.active?.id;
		if (id !== undefined && id !== mountedWorkspaceId) goto('/forms');
	});

	// ── Form ──────────────────────────────────────────────────────────────────
	let record = $state<FormRecord | null>(null);
	let resolvedFormKey = $state<CryptoKey | null>(null);
	const formName = $derived(formsStore.formNames.get(formId) ?? '');
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

	let notificationEmail = $state('');
	let pgpPublicKey = $state('');
	let notificationFrom = $state('');
	let notificationSubject = $state('');
	let pgpPending = $state(false);
	const pgpOpen = $derived(!!notificationEmail || pgpPending);
	let emailEnabled = $state(false);
	let smtpSender = $state('');
	let pgpKeyFingerprint = $state('');
	let pgpKeyError = $state('');

	async function handlePGPKeyInput(value: string) {
		pgpPublicKey = value;
		pgpKeyFingerprint = '';
		pgpKeyError = '';
		if (!value.trim()) return;
		try {
			pgpKeyFingerprint = await validatePGPKey(value);
		} catch (e) {
			pgpKeyError = e instanceof Error ? e.message : 'Invalid PGP key';
		}
	}

	let closeOnDatePending = $state(false);
	let limitResponsesPending = $state(false);
	let autoDeletePending = $state(false);
	const closeOnDateOpen = $derived(!!expiresAt || closeOnDatePending);
	const limitResponsesOpen = $derived(!!responseLimit || limitResponsesPending);
	const settingsLifetimePolicy = $derived<'none' | 'burn' | 'ttl'>(
		burnAfterReading ? 'burn' : responseTtlDays ? 'ttl' : 'none'
	);
	const autoDeleteOpen = $derived(settingsLifetimePolicy !== 'none' || autoDeletePending);

	let shareUrl = $state('');
	let shareUrlLoading = $state(false);
	let publishing = $state(false);
	let publishError = $state('');
	let copied = $state(false);
	let confirmRotate = $state(false);
	let customDomainInfo = $state<CustomDomainInfo | null>(null);
	let qrCanvas = $state<HTMLCanvasElement | null>(null);
	let qrVisible = $state(false);
	let qrError = $state('');

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
	let activeTab = $state<'details' | 'share' | 'settings' | 'responses'>('details');
	let decrypted = $state<Map<string, DecryptedResponse>>(new Map());
	let decrypting = $state<Set<string>>(new Set());
	let decryptErrors = $state<Map<string, string>>(new Map());

	let schemaCache = $state<Map<number, BuilderSchema>>(new Map());

	let confirmDeleteResponse = $state<string | null>(null);
	let deletingResponses = $state<Set<string>>(new Set());

	// ── Init ─────────────────────────────────────────────────────────────────
	onMount(async () => {
		if (!auth.masterKey) { goto('/login'); return; }
		getAppConfig().then(c => { emailEnabled = c.emailEnabled; smtpSender = c.smtpSender; }).catch(() => {});
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
			const title = schema.translations[schema.defaultLocale]?.formTitle;
			if (title) formsStore.updateName(formId, title);
			if (r.renderKeySalt && r.status !== 'draft') {
				const cached = formsStore.shareUrls.get(formId);
				if (cached) {
					shareUrl = cached;
				} else {
					shareUrlLoading = true;
					if (r.workspaceId) {
						getCustomDomain(r.workspaceId).then(async d => {
							customDomainInfo = d;
							const base = d?.enabled && d.domain ? `https://${d.domain}` : undefined;
							shareUrl = await deriveShareUrl(formId, r.renderKeySalt!, formKey, base);
						}).catch(() => {}).finally(() => { shareUrlLoading = false; });
					} else {
						deriveShareUrl(formId, r.renderKeySalt, formKey).then(u => { shareUrl = u; }).catch(() => {}).finally(() => { shareUrlLoading = false; });
					}
				}
			}
			expiresAt = r.expiresAt ?? '';
			responseLimit = r.responseLimit != null ? String(r.responseLimit) : '';
			responseTtlDays = r.responseTtlDays != null ? String(r.responseTtlDays) : '';
			burnAfterReading = r.burnAfterReading ?? false;
			notificationEmail = r.notificationEmail ?? '';
			pgpPublicKey = r.pgpPublicKey ?? '';
			notificationFrom = r.notificationFrom ?? '';
			notificationSubject = r.notificationSubject ?? '';
			pgpPending = false;
			pgpKeyFingerprint = '';
			pgpKeyError = '';
			if (pgpPublicKey) {
				try { pgpKeyFingerprint = await validatePGPKey(pgpPublicKey); } catch { /* stored key shown as-is */ }
			}
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
		if (pgpOpen && !notificationEmail.trim()) {
			settingsError = 'A recipient email address is required for email forwarding.';
			return;
		}
		if (pgpOpen && !pgpPublicKey.trim()) {
			settingsError = 'A PGP public key is required for email forwarding.';
			return;
		}
		if (pgpPublicKey && pgpKeyError) {
			settingsError = pgpKeyError;
			return;
		}
		settingsSaving = true;
		settingsError = '';
		settingsSaved = false;
		try {
			await Promise.all([
				updateFormExpiration(
					formId,
					expiresAt || null,
					responseLimit ? parseInt(responseLimit) : null,
					responseTtlDays ? parseInt(responseTtlDays) : null,
					burnAfterReading
				),
				updateFormPGPNotification(formId, notificationEmail, pgpPublicKey, notificationFrom, notificationSubject)
			]);
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
			const base = customDomainBase() ?? (await getAppConfig().then(c => c.formsDomain ? `https://${c.formsDomain}` : undefined));
			const result = await publishForm(auth.masterKey, formId, schema as any, salt, formKey, base);
			shareUrl = result.shareUrl;
			// publishForm atomically sets status='open' on the server
			record = { ...record, status: 'open', hasUnpublishedChanges: false };
			formsStore.updateStatus(formId, 'open');
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

	function customDomainBase(): string | undefined {
		if (customDomainInfo?.enabled && customDomainInfo.domain) return `https://${customDomainInfo.domain}`;
		return undefined;
	}

	async function handleRotateKey() {
		if (!auth.masterKey || !record) return;
		publishing = true;
		publishError = '';
		confirmRotate = false;
		try {
			const { schema, formKey } = await getForm(auth.masterKey, formId, undefined);
			const result = await rotateRenderKey(auth.masterKey, formId, schema as any, formKey, customDomainBase());
			shareUrl = result.shareUrl;
			record = { ...record, renderKeySalt: btoa(String.fromCharCode(...result.renderKeySalt)) };
			qrVisible = false;
		} catch (e) {
			publishError = e instanceof Error ? e.message : 'Key rotation failed';
		} finally {
			publishing = false;
		}
	}

	async function showQRCode() {
		if (!shareUrl) return;
		qrVisible = true;
		qrError = '';
		await new Promise(r => setTimeout(r, 0));
		try {
			if (qrCanvas) await QRCode.toCanvas(qrCanvas, shareUrl, { width: 240, margin: 2 });
		} catch {
			qrError = 'Failed to generate QR code';
		}
	}

	function downloadQR() {
		if (!qrCanvas) return;
		const a = document.createElement('a');
		a.href = qrCanvas.toDataURL('image/png');
		a.download = `form-qr-${formId}.png`;
		a.click();
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
		if (activeTab === 'responses') activeTab = 'details';
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
	const statusColor = $derived(
		record?.status === 'open' ? 'bg-success'
		: record?.status === 'draft' ? 'bg-warn-dim'
		: 'bg-muted'
	);
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

<!-- Rotate key confirm -->
<ConfirmDialog
	open={confirmRotate}
	title="Generate new link?"
	description="This will invalidate the current share link and QR code. Anyone using the old link will no longer be able to access the form."
	onconfirm={handleRotateKey}
	oncancel={() => { confirmRotate = false; }}
/>

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
	<div class="flex items-center gap-2 sm:gap-3 px-4 sm:px-5 h-9 border-b border-border-canvas shrink-0 overflow-hidden">
		<Breadcrumb items={[
			{ label: 'Forms', href: '/forms' },
			{ label: formName || formId.slice(0, 12) + '…', onclick: selectedId ? () => { selectedId = null; } : undefined },
			...(selectedRecord ? [{ label: selectedRecord.id.slice(0, 8) + '…' }] : [])
		]} />
		<div class="flex-1 min-w-0"></div>
		<a
			href="/forms/{formId}/edit"
			class="shrink-0 font-mono text-base text-subtle no-underline px-2.5 py-0.5 border border-border-canvas rounded-sm whitespace-nowrap
				hover:text-text hover:border-border-canvas transition-colors duration-100 flex items-center gap-1.5"
		>
			<Pencil size={12} strokeWidth={1.75} />
			<span class="hidden sm:inline">Edit form</span>
			<span class="sm:hidden">Edit</span>
		</a>
	</div>

	<!-- Shell -->
	<div class="flex flex-col sm:flex-row flex-1 min-h-0 font-mono">

		<!-- Left panel: response list (desktop only) -->
		<div class="hidden sm:flex sm:w-[280px] shrink-0 flex-col border-r border-surface min-h-0">

			<!-- List header -->
			<div class="px-4 py-3 border-b border-surface shrink-0 flex items-center justify-between gap-2">
				<p class="text-base font-semibold tracking-[0.1em] uppercase text-subtle m-0 flex-1">Responses</p>
				<button
					title="Refresh"
					disabled={responsesLoading || loadingMore}
					onclick={() => loadResponses()}
					class="flex items-center justify-center w-7 h-7 bg-transparent border-none rounded cursor-pointer text-subtle transition-[color,background] duration-100 hover:text-subtle hover:bg-border-deep disabled:opacity-30 disabled:cursor-not-allowed"
				>
					<RefreshCw size={15} strokeWidth={2} />
				</button>
			</div>

			{#if responsesLoading}
				<div class="flex-1 flex items-center justify-center text-subtle text-base gap-2.5">
					<div class="spinner w-3.5 h-3.5 border-2 border-surface border-t-info-border rounded-full"></div>
					Loading…
				</div>
			{:else if responsesError}
				<div class="flex-1 flex items-center justify-center text-error-light text-base p-8 text-center">{responsesError}</div>
			{:else if responses.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
					<div class="text-4xl text-subtle mb-3 opacity-40">○</div>
					<p class="text-subtle m-0">No responses yet</p>
				</div>
			{:else}
				<div class="flex-1 overflow-y-auto overflow-x-hidden">
					{#each responses as resp, i (resp.id)}
						<button
							onclick={() => selectResponse(resp.id)}
							class="block w-full px-4 py-[11px] text-left bg-transparent border-none border-b border-border-canvas cursor-pointer transition-[background] duration-100 font-mono hover:bg-surface
								{selectedId === resp.id ? 'bg-canvas border-l-2 border-l-info-border pl-[14px]' : ''}"
						>
							<div class="flex items-center justify-between gap-2">
								<span class="text-base overflow-hidden text-ellipsis whitespace-nowrap flex-1
									{selectedId === resp.id ? 'text-text' : 'text-subtle'}">
									#{i + 1} · {resp.id.slice(0, 12)}…
								</span>
								{#if decrypted.has(resp.id)}
									<span class="w-[5px] h-[5px] rounded-full bg-success-dim shrink-0" title="Decrypted"></span>
								{/if}
								<span class="text-base text-subtle bg-base border border-surface rounded-sm px-1.5 py-px shrink-0">v{resp.schemaVersion}</span>
							</div>
							<div class="text-base text-subtle mt-0.5">{formatDateShort(resp.receivedAt)}</div>
						</button>
					{/each}
				</div>

				{#if hasMore}
					<div class="px-4 py-3 border-t border-border-canvas shrink-0">
						<button
							onclick={() => loadResponses(nextCursor)}
							disabled={loadingMore}
							class="w-full px-3 py-1.5 bg-transparent text-subtle border border-surface rounded cursor-pointer font-mono text-base transition-[color,border-color] duration-100 hover:not-disabled:text-subtle hover:not-disabled:border-border-canvas disabled:opacity-40 disabled:cursor-not-allowed"
						>
							{loadingMore ? 'Loading…' : 'Load more'}
						</button>
					</div>
				{/if}
			{/if}
		</div>

		<!-- Right panel -->
		<div class="flex flex-1 min-w-0 flex-col min-h-0">

			{#if selectedId === null}
				<!-- ── Details / Settings view ──────────────────────────────────── -->
				<div class="flex-1 flex flex-col min-h-0">

					<!-- Form name heading -->
					{#if formName}
						<h1 class="m-0 px-4 sm:px-5 pt-4 sm:pt-5 pb-2 sm:pb-3 text-lg sm:text-xl text-text font-mono font-normal shrink-0">{formName}</h1>
					{/if}

					<!-- Tab bar -->
					<div class="flex items-end h-9 border-b border-border-canvas shrink-0 px-4 gap-1">
						<button
							onclick={() => { activeTab = 'responses'; }}
							class="sm:hidden h-full px-3 text-base font-mono bg-transparent border-0 border-b-2 -mb-px cursor-pointer transition-colors duration-100 whitespace-nowrap
								{activeTab === 'responses'
									? 'text-text border-b-info-border'
									: 'text-subtle border-b-transparent hover:text-subtle'}"
						>Responses{record ? ` (${record.responseCount})` : ''}</button>
						{#each [['details', 'Details'], ['share', 'Share'], ['settings', 'Settings']] as [id, label]}
							<button
								onclick={() => { activeTab = id as typeof activeTab; }}
								class="h-full px-3 text-base font-mono bg-transparent border-0 border-b-2 -mb-px cursor-pointer transition-colors duration-100 whitespace-nowrap
									{activeTab === id
										? 'text-text border-b-info-border'
										: 'text-subtle border-b-transparent hover:text-subtle'}"
							>{label}</button>
						{/each}
					</div>

					<!-- Tab content -->
					<div class="flex-1 overflow-y-auto">
						{#if loading}
							<div class="flex items-center justify-center gap-2.5 text-subtle text-lg py-16">
								<div class="spinner w-3.5 h-3.5 border-2 border-surface border-t-info-border rounded-full"></div>
								Loading…
							</div>
						{:else if loadError}
							<div class="p-8 text-lg text-error-light">{loadError}</div>
						{:else if record}
							<div class="max-w-3xl px-4 py-4 sm:px-8 sm:py-8">

								{#if activeTab === 'responses'}
									<!-- Responses (mobile inline list) -->
									<div class="sm:hidden flex flex-col min-h-0 -mx-4 -mt-4">
										{#if responsesLoading}
											<div class="flex items-center justify-center text-subtle text-base gap-2.5 py-12">
												<div class="spinner w-3.5 h-3.5 border-2 border-surface border-t-info-border rounded-full"></div>
												Loading…
											</div>
										{:else if responsesError}
											<div class="flex items-center justify-center text-error-light text-base p-8 text-center">{responsesError}</div>
										{:else if responses.length === 0}
											<div class="flex flex-col items-center justify-center text-border-subtle text-center py-16 px-8">
												<div class="text-4xl mb-3 opacity-40">○</div>
												<p class="text-base m-0">No responses yet</p>
											</div>
										{:else}
											<div>
												{#each responses as resp, i (resp.id)}
													<button
														onclick={() => selectResponse(resp.id)}
														class="block w-full px-4 py-[11px] text-left bg-transparent border-none border-b border-border-canvas cursor-pointer transition-[background] duration-100 font-mono hover:bg-surface"
													>
														<div class="flex items-center justify-between gap-2">
															<span class="text-base overflow-hidden text-ellipsis whitespace-nowrap flex-1 text-subtle">
																#{i + 1} · {resp.id.slice(0, 12)}…
															</span>
															{#if decrypted.has(resp.id)}
																<span class="w-[5px] h-[5px] rounded-full bg-success-dim shrink-0" title="Decrypted"></span>
															{/if}
															<span class="text-base text-subtle bg-base border border-surface rounded-sm px-1.5 py-px shrink-0">v{resp.schemaVersion}</span>
														</div>
														<div class="text-base text-subtle mt-0.5">{formatDateShort(resp.receivedAt)}</div>
													</button>
												{/each}
											</div>
											{#if hasMore}
												<div class="px-4 py-3 border-t border-border-canvas">
													<button
														onclick={() => loadResponses(nextCursor)}
														disabled={loadingMore}
														class="w-full px-3 py-1.5 bg-transparent text-subtle border border-surface rounded cursor-pointer font-mono text-base transition-[color,border-color] duration-100 hover:not-disabled:text-subtle hover:not-disabled:border-border-canvas disabled:opacity-40 disabled:cursor-not-allowed"
													>
														{loadingMore ? 'Loading…' : 'Load more'}
													</button>
												</div>
											{/if}
										{/if}
									</div>

								{:else if activeTab === 'details'}
									<!-- Details -->
									<section class="flex flex-col gap-4">
										<div class="border border-border-canvas rounded-lg overflow-hidden">
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5 border-b border-border-canvas">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Name</span>
												<span class="text-base sm:text-lg text-text flex-1 min-w-0 truncate">{formName || '—'}</span>
											</div>
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5 border-b border-border-canvas">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Status</span>
												<div class="flex items-center gap-2 sm:gap-2.5 flex-1 min-w-0">
													<span class="w-2 h-2 rounded-full shrink-0 {statusColor}"></span>
													<span class="text-base sm:text-lg text-text capitalize">{record.status}</span>
													{#if record.status === 'draft'}
														<a
															href="/forms/{formId}/edit"
															class="ml-auto px-2.5 sm:px-3 py-1 sm:py-1.5 text-base font-mono border rounded cursor-pointer transition-colors duration-100 no-underline
																bg-transparent text-info border-info-dim hover:bg-info-dark hover:border-info-dim"
														>Publish</a>
													{:else}
														<button
															onclick={toggleStatus}
															disabled={statusSaving}
															class="ml-auto px-2.5 sm:px-3 py-1 sm:py-1.5 text-base font-mono border rounded cursor-pointer transition-colors duration-100
																{statusSaving
																	? 'bg-transparent text-subtle border-border-canvas cursor-not-allowed'
																	: record.status === 'open'
																		? 'bg-transparent text-danger border-danger-dim hover:bg-danger-dark hover:border-danger-dim'
																		: 'bg-transparent text-success border-success-dim hover:bg-success-dark hover:border-success-dim'}"
														>
															{statusSaving ? '…' : record.status === 'open' ? 'Close' : 'Open'}
														</button>
													{/if}
												</div>
											</div>
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5 border-b border-border-canvas">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Form ID</span>
												<span class="text-base sm:text-lg text-subtle font-mono flex-1 min-w-0 truncate">{formId}</span>
											</div>
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5 border-b border-border-canvas">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Responses</span>
												<span class="text-base sm:text-lg text-text tabular-nums">{record.responseCount}</span>
											</div>
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5 border-b border-border-canvas">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Version</span>
												<span class="text-base sm:text-lg text-subtle tabular-nums">v{record.schemaVersion}</span>
											</div>
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5 border-b border-border-canvas">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Created</span>
												<span class="text-base sm:text-lg text-subtle">{formatDate(record.createdAt)}</span>
											</div>
											<div class="flex items-center gap-4 px-4 py-3 sm:py-3.5">
												<span class="w-24 sm:w-32 shrink-0 text-base text-subtle">Updated</span>
												<span class="text-base sm:text-lg text-subtle">{formatDate(record.updatedAt)}</span>
											</div>
										</div>
									</section>

								{:else if activeTab === 'share'}
									<!-- Share link -->
									<section class="flex flex-col gap-3">
										{#if record.status === 'draft'}
											<div class="py-4 flex flex-col items-center gap-3 text-center">
												<div>
													<p class="m-0 text-sm text-text">This form is unpublished</p>
													<p class="m-0 text-xs text-subtle mt-1">Publish to make it accessible and get a share link.</p>
												</div>
												{#if publishError}
													<p class="m-0 text-sm text-error-light">{publishError}</p>
												{/if}
												<button
													onclick={handlePublish}
													disabled={publishing}
													class="flex items-center gap-2 px-4 py-2 bg-primary text-white border-none rounded-md font-mono text-sm cursor-pointer transition-[background] duration-100 hover:bg-primary-hover disabled:opacity-60 disabled:cursor-not-allowed"
												>
													{#if publishing}
														<div class="spinner w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full"></div>
														Publishing…
													{:else}
														Publish form
													{/if}
												</button>
											</div>
										{:else if shareUrlLoading || !shareUrl}
											<div class="py-4 flex flex-col items-center gap-2 text-center">
												<p class="m-0 text-xs text-subtle">Loading link…</p>
											</div>
										{:else}
											<div class="flex flex-col sm:flex-row gap-1.5">
												<input
													type="text"
													readonly
													value={shareUrl}
													class="flex-1 px-3 py-2 bg-base border border-border-canvas text-subtle rounded-md font-mono text-sm outline-none min-w-0"
												/>
												<button
													onclick={copyShareUrl}
													class="sm:shrink-0 px-3 py-2 border-none rounded-md font-mono text-sm transition-[background] duration-150 grid items-center
														{copied ? 'bg-success-muted text-success cursor-default' : 'bg-primary text-white hover:bg-primary-hover cursor-pointer'}"
												>
													<span class="col-start-1 row-start-1 flex items-center justify-center gap-1.5 {copied ? '' : 'invisible'}">
														<Check size={13} strokeWidth={2} />Copied
													</span>
													<span class="col-start-1 row-start-1 flex items-center justify-center gap-1.5 {copied ? 'invisible' : ''}">
														<Link size={13} strokeWidth={1.75} />Copy secure link
													</span>
												</button>
											</div>

											{#if record.status === 'closed'}
												<p class="m-0 text-sm text-closed-text">This form is closed — the link is active but not accepting responses.</p>
											{:else if record.hasUnpublishedChanges}
												<p class="m-0 text-sm text-warn-dim">This link reflects the last published version. <a href="/forms/{formId}/edit" class="text-text underline">Update</a> to publish your latest changes.</p>
											{:else}
												<p class="m-0 text-sm text-subtle">Anyone with the link can access this form.</p>
											{/if}

											{#if customDomainInfo?.enabled && customDomainInfo.domain}
												<p class="m-0 text-sm text-subtle font-mono">
													Served on <span class="text-text">{customDomainInfo.domain}</span>
												</p>
											{/if}

											<!-- QR Code -->
											<div class="border-t border-border-canvas pt-3 flex flex-col gap-2">
												{#if !qrVisible}
													<button
														onclick={showQRCode}
														class="px-3 py-2 bg-transparent text-subtle border border-border-canvas rounded-md cursor-pointer font-mono text-sm flex items-center gap-1.5 hover:text-text hover:border-border-canvas transition-colors duration-100"
													><QrCode size={13} strokeWidth={1.75} />Get QR code</button>
												{:else}
													<div class="flex flex-col items-center gap-2">
														<canvas bind:this={qrCanvas} class="rounded-md"></canvas>
														<button
															onclick={downloadQR}
															class="px-3 py-2 bg-transparent text-subtle border border-border-canvas rounded-md cursor-pointer font-mono text-sm flex items-center gap-1.5 hover:text-text hover:border-border-canvas transition-colors duration-100 w-full justify-center"
														><Download size={13} strokeWidth={1.75} />Download PNG</button>
														<button
															onclick={() => { qrVisible = false; }}
															class="text-xs text-subtle hover:text-subtle cursor-pointer bg-transparent border-none"
														>Hide</button>
													</div>
												{/if}
												{#if qrError}<p class="m-0 text-sm text-error">{qrError}</p>{/if}
												<p class="m-0 text-xs text-subtle">QR code stays valid when you edit your form. Rotating your link will require a new QR code.</p>
											</div>

											<div class="h-px bg-border-deep"></div>

											{#if publishError}
												<p class="m-0 text-sm text-error-light">{publishError}</p>
											{/if}
											<button
												onclick={() => { confirmRotate = true; }}
												disabled={publishing}
												class="px-3 py-2 bg-transparent text-subtle border border-border-canvas rounded-md cursor-pointer font-mono text-sm
													{publishing ? 'cursor-not-allowed opacity-60' : 'hover:text-text hover:border-border-canvas transition-colors duration-100'}"
											>Generate new link</button>
										{/if}
									</section>

								{:else if activeTab === 'settings'}
									<!-- Settings -->
									<section class="flex flex-col gap-4">
										<div class="flex flex-col divide-y border-surface">

											<!-- Close on date -->
											<div class="py-3 first:pt-0">
												<div class="flex items-center justify-between gap-3">
													<div>
														<p class="m-0 text-text">Close on date</p>
														<p class="m-0 text-sm text-subtle mt-0.5">Stop accepting responses after a date.</p>
													</div>
													<button
														role="switch"
														aria-checked={closeOnDateOpen}
														onclick={() => {
															if (closeOnDateOpen) { closeOnDatePending = false; expiresAt = ''; }
															else { closeOnDatePending = true; }
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
														<input type="date" bind:value={expiresAt} class="input-base" />
													</div>
												{/if}
											</div>

											<!-- Limit responses -->
											<div class="py-3">
												<div class="flex items-center justify-between gap-3">
													<div>
														<p class="m-0 text-text">Limit total responses</p>
														<p class="m-0 text-sm text-subtle mt-0.5">Close after a set number of submissions.</p>
													</div>
													<button
														role="switch"
														aria-checked={limitResponsesOpen}
														onclick={() => {
															if (limitResponsesOpen) { limitResponsesPending = false; responseLimit = ''; }
															else { limitResponsesPending = true; }
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
														<input type="number" min="1" placeholder="e.g. 100" bind:value={responseLimit} class="input-base" />
													</div>
												{/if}
											</div>

											<!-- Auto delete -->
											<div class="py-3">
												<div class="flex items-center justify-between gap-3">
													<div>
														<p class="m-0 text-text">Auto delete responses</p>
														<p class="m-0 text-sm text-subtle mt-0.5">Remove responses from our servers after a set period.</p>
													</div>
													<button
														role="switch"
														aria-checked={autoDeleteOpen}
														onclick={() => {
															if (autoDeleteOpen) {
																autoDeletePending = false;
																burnAfterReading = false;
																responseTtlDays = '';
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
															value={settingsLifetimePolicy === 'none' ? 'burn' : settingsLifetimePolicy}
															onchange={(e) => {
																const p = (e.target as HTMLSelectElement).value;
																burnAfterReading = p === 'burn';
																responseTtlDays = p === 'ttl' ? (responseTtlDays || '30') : '';
															}}
															class="input-base"
														>
															<option value="burn">Burn after reading</option>
															<option value="ttl">Delete after a set period</option>
														</select>
														{#if settingsLifetimePolicy === 'ttl'}
															<div class="flex gap-1.5 items-center">
																<input type="number" min="1" placeholder="Days" bind:value={responseTtlDays} class="input-base" />
																<span class="text-sm text-subtle shrink-0">days</span>
															</div>
														{:else if settingsLifetimePolicy === 'burn'}
															<p class="m-0 text-xs text-subtle leading-relaxed">Responses are scheduled for deletion once you view them. They remain visible until the next cleanup pass.</p>
														{/if}
													</div>
												{/if}
											</div>

										<!-- Email notifications -->
										<div class="py-3">
											<div class="flex items-center justify-between gap-3">
												<div>
													<p class="m-0 text-base text-text">Email forwarding</p>
													<p class="m-0 text-sm text-subtle mt-0.5">Forward encrypted responses to an email via PGP.</p>
													{#if !emailEnabled}
														<p class="m-0 text-xs text-warning-text mt-1">Email is not configured on this server.</p>
													{/if}
												</div>
												<button
													role="switch"
													aria-checked={pgpOpen}
													disabled={!emailEnabled}
													onclick={() => {
														if (!emailEnabled) return;
														if (pgpOpen) { pgpPending = false; notificationEmail = ''; pgpPublicKey = ''; }
														else { pgpPending = true; }
													}}
													class="relative shrink-0 w-8 h-[18px] rounded-full transition-colors duration-150 border-none
														{emailEnabled ? 'cursor-pointer' : 'cursor-not-allowed opacity-40'}
														{pgpOpen ? 'bg-primary' : 'bg-border-deep'}"
												>
													<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
														{pgpOpen ? 'translate-x-[14px]' : 'translate-x-0'}"></span>
												</button>
											</div>
											{#if pgpOpen || notificationEmail}
												<div class="mt-2.5 flex flex-col gap-5">

													<!-- Email headers -->
													<div>
														<div class="border border-border-canvas rounded-md overflow-hidden">
															<div class="flex items-center border-b border-border-canvas">
																<span class="w-20 shrink-0 px-3 py-2 text-sm text-subtle border-r border-border-canvas">To</span>
																<input
																	type="email"
																	placeholder="recipient@example.com"
																	bind:value={notificationEmail}
																	class="flex-1 min-w-0 px-3 py-2 bg-transparent border-none outline-none text-sm text-text placeholder:text-subtle font-mono"
																/>
															</div>
															<div class="flex items-center border-b border-border-canvas">
																<span class="w-20 shrink-0 px-3 py-2 text-sm text-subtle border-r border-border-canvas">From</span>
																<span class="flex-1 min-w-0 px-3 py-2 text-sm text-subtle font-mono truncate">
																	{smtpSender || 'Confide Forms <notifications@example.com>'}
																</span>
															</div>
															<div class="flex items-center">
																<span class="w-20 shrink-0 px-3 py-2 text-sm text-subtle border-r border-border-canvas">Subject</span>
																<input
																	type="text"
																	placeholder="New Confide Form submission"
																	bind:value={notificationSubject}
																	class="flex-1 min-w-0 px-3 py-2 bg-transparent border-none outline-none text-sm text-text placeholder:text-subtle font-mono"
																/>
															</div>
														</div>
														<p class="m-0 mt-1.5 text-xs text-subtle">To, From, and Subject are stored unencrypted on our servers.</p>
													</div>

													<!-- PGP key -->
													<div>
														<p class="m-0 mb-1.5 text-sm text-text">PGP Public Key</p>
														<div class="border border-border-canvas rounded-md overflow-hidden {pgpKeyError ? 'border-border-canvas-danger-muted' : ''}">
															<textarea
																placeholder="Begins with '-----BEGIN PGP PUBLIC KEY BLOCK----'"
																value={pgpPublicKey}
																oninput={(e) => handlePGPKeyInput((e.target as HTMLTextAreaElement).value)}
																rows={5}
																class="w-full px-3 py-2.5 bg-transparent border-none outline-none text-xs text-text placeholder:text-subtle font-mono resize-y block"
															></textarea>
														</div>
														<p class="m-0 mt-1.5 text-xs {pgpKeyError ? 'text-error-light' : pgpKeyFingerprint ? 'text-success font-mono tracking-wide' : 'text-subtle'}">
															{#if pgpKeyError}
																{pgpKeyError}
															{:else if pgpKeyFingerprint}
																✓ {pgpKeyFingerprint.match(/.{1,4}/g)?.join(' ')}
															{:else}
																Paste your PGP public key block. In Proton Mail: Settings → Encryption & keys → Export public key.
															{/if}
														</p>
													</div>

												</div>
											{/if}
										</div>

									</div>

									<div class="border-t border-border-canvas pt-4 flex items-center gap-3">
										<button
											onclick={saveSettings}
											disabled={settingsSaving || !!pgpKeyError}
											class="px-4 py-2 border rounded font-mono text-base cursor-pointer transition-colors duration-100
												{settingsSaving || pgpKeyError
													? 'bg-transparent text-subtle border-border-canvas cursor-not-allowed'
													: 'bg-transparent text-text border-info-action-border hover:bg-info-action-bg hover:border-info-border'}"
										>
											{settingsSaving ? 'Saving…' : 'Save settings'}
										</button>
										{#if settingsSaved}
											<span class="text-base text-success flex items-center gap-1">
												<Check size={13} strokeWidth={2} />
												Saved
											</span>
										{/if}
										{#if settingsError}
											<span class="text-base text-error-light">{settingsError}</span>
										{/if}
									</div>
									</section>

									<!-- Danger zone -->
									<section class="flex flex-col gap-4 mt-8">
										<h2 class="m-0 font-semibold tracking-[0.08em] uppercase text-subtle">Danger zone</h2>
										<div class="border border-danger rounded-lg px-4 py-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-4">
											<div class="min-w-0">
												<p class="m-0 sm:text-lg text-text">Delete this form</p>
												<p class="m-0 mt-0.5 text-subtle">
													Permanently deletes the form and all {record.responseCount} response{record.responseCount === 1 ? '' : 's'}. Cannot be undone.
												</p>
											</div>
											<button
												onclick={() => { pendingDeleteForm = true; }}
												class="sm:shrink-0 px-4 py-2 bg-transparent text-error-light border border-danger rounded cursor-pointer font-mono text-danger sm:text-lg
													hover:bg-danger-hover hover:border-border-canvas-danger-dark transition-colors duration-100"
											>Delete</button>
										</div>
									</section>
								{/if}

							</div>
						{/if}
					</div>
				</div>

			{:else}
				<!-- ── Response detail view ───────────────────────────────────── -->
				{#if !selectedRecord}
					<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
						<p class="text-base m-0">Response not found</p>
					</div>
				{:else}
					<!-- Detail header -->
					<div class="px-4 sm:px-6 pt-4 sm:pt-5 pb-3 sm:pb-4 border-b border-surface shrink-0">
						<div class="flex items-start justify-between gap-3 sm:gap-4">
							<div class="min-w-0">
								<p class="text-base sm:text-lg text-subtle m-0 mb-1 overflow-hidden text-ellipsis whitespace-nowrap">{selectedRecord.id}</p>
								<p class="text-sm sm:text-base text-subtle m-0">
									Received {formatDateLong(selectedRecord.receivedAt)}
									{#if selectedDecrypted}
										<span class="inline-block text-base text-subtle bg-base border border-surface rounded-sm px-1.5 py-px ml-2 align-middle">{selectedDecrypted.locale}</span>
									{/if}
								</p>
							</div>
							<div class="flex items-center gap-2 shrink-0">
								<button
									onclick={() => (confirmDeleteResponse = selectedRecord.id)}
									class="px-3 sm:px-4 py-1.5 sm:py-2 bg-transparent text-error-light border border-border-canvas rounded cursor-pointer font-mono text-base sm:text-lg transition-colors duration-100 hover:bg-danger-hover hover:border-border-canvas-danger-dark"
								>
									Delete
								</button>
							</div>
						</div>
						<button
							class="sm:hidden mt-2 text-sm text-subtle hover:text-subtle bg-transparent border-none cursor-pointer p-0 font-mono"
							onclick={() => { selectedId = null; activeTab = 'responses'; }}
						>← All responses</button>
					</div>

					<!-- Detail content -->
					<div class="flex-1 overflow-y-auto p-4 sm:p-6">
						{#if isDecryptingSelected}
							<div class="flex items-center gap-2.5 text-subtle text-lg py-8">
								<div class="spinner w-3.5 h-3.5 border-2 border-surface border-t-info-border rounded-full"></div>
								Decrypting…
							</div>
						{:else if selectedDecryptError}
							<p class="text-error-light text-lg py-3 m-0">{selectedDecryptError}</p>
						{:else if selectedDecrypted}
							<div class="flex flex-col gap-6">
								{#each selectedDecrypted.schema.fields as field (field.id)}
									{#if field.type !== 'section_break'}
										{@const fieldT = (selectedDecrypted.schema.translations[selectedDecrypted.locale] ?? selectedDecrypted.schema.translations[selectedDecrypted.schema.defaultLocale])?.fields[field.id]}
										{@const answer = renderAnswer(field, selectedDecrypted)}
										<div class="border-b border-border-canvas pb-6 last:border-b-0 last:pb-0">
											<p class="text-base font-semibold tracking-[0.08em] uppercase text-subtle m-0 mb-2">
												{fieldT?.label ?? field.id}{#if field.required}<span class="text-error-light ml-0.5">*</span>{/if}
											</p>
											<p class="text-lg text-text m-0 leading-relaxed whitespace-pre-wrap break-words
												{answer === '—' ? 'text-subtle italic' : ''}">
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
