<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		listResponses,
		decryptResponseRecord,
		deleteResponse,
		getSchemaVersion,
		getForm,
		type EncryptedResponseRecord
	} from '$lib/forms';
	import type { BuilderSchema, BuilderField, MultipleChoiceConfig, CheckboxesConfig, DropdownConfig, RatingConfig } from '$lib/types/builder';

	type AnswerValue = string | string[] | number | null | undefined;

	interface DecryptedResponse {
		submittedAt: string;
		locale: string;
		answers: Record<string, AnswerValue>;
		schema: BuilderSchema;
	}

	const formId = $page.params.id ?? '';

	let responses = $state<EncryptedResponseRecord[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let hasMore = $state(false);
	let loading = $state(true);
	let loadMore = $state(false);
	let loadError = $state('');

	let selectedId = $state<string | null>(null);
	let decrypted = $state<Map<string, DecryptedResponse>>(new Map());
	let decrypting = $state<Set<string>>(new Set());
	let decryptErrors = $state<Map<string, string>>(new Map());
	let confirmDelete = $state<string | null>(null);
	let deleting = $state<Set<string>>(new Set());

	let schemaCache = $state<Map<number, BuilderSchema>>(new Map());
	let formName = $state<string | null>(null);

	onMount(async () => {
		if (!auth.masterKey) {
			goto('/login');
			return;
		}
		await loadResponses();
		getForm(auth.masterKey, formId).then(({ schema }) => {
			formName = schema.translations[schema.defaultLocale]?.formTitle ?? null;
		}).catch(() => {});
	});

	async function loadResponses(cursor?: string) {
		if (!auth.masterKey) return;
		if (cursor) loadMore = true;
		else loading = true;
		loadError = '';
		try {
			const result = await listResponses(formId, cursor, 25);
			responses = cursor ? [...responses, ...result.responses] : result.responses;
			nextCursor = result.nextCursor;
			hasMore = !!result.nextCursor;

			// Auto-select first if none selected
			if (!selectedId && result.responses.length > 0) {
				await selectResponse(result.responses[0]);
			}
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load responses';
		} finally {
			loading = false;
			loadMore = false;
		}
	}

	async function selectResponse(record: EncryptedResponseRecord) {
		selectedId = record.id;
		if (!decrypted.has(record.id) && !decrypting.has(record.id)) {
			await handleDecrypt(record);
		}
	}

	async function handleDecrypt(record: EncryptedResponseRecord) {
		if (!auth.masterKey || decrypted.has(record.id)) return;

		decrypting = new Set([...decrypting, record.id]);
		const errs = new Map(decryptErrors);
		errs.delete(record.id);
		decryptErrors = errs;

		try {
			let schema = schemaCache.get(record.schemaVersion);
			if (!schema) {
				schema = await getSchemaVersion(auth.masterKey, formId, record.schemaVersion);
				schemaCache = new Map([...schemaCache, [record.schemaVersion, schema]]);
			}

			const payload = await decryptResponseRecord(auth.masterKey, formId, record);
			decrypted = new Map([...decrypted, [record.id, {
				submittedAt: payload.submittedAt,
				locale: payload.locale,
				answers: payload.answers as Record<string, AnswerValue>,
				schema
			}]]);
		} catch (err) {
			decryptErrors = new Map([...decryptErrors, [record.id, err instanceof Error ? err.message : 'Decryption failed']]);
		} finally {
			const d = new Set(decrypting);
			d.delete(record.id);
			decrypting = d;
		}
	}

	async function handleDelete(responseId: string) {
		deleting = new Set([...deleting, responseId]);
		try {
			await deleteResponse(formId, responseId);
			responses = responses.filter((r) => r.id !== responseId);
			const nd = new Map(decrypted);
			nd.delete(responseId);
			decrypted = nd;
			confirmDelete = null;
			// Select next response
			if (selectedId === responseId) {
				const next = responses[0];
				if (next) await selectResponse(next);
				else selectedId = null;
			}
		} catch {
			// keep confirm open
		} finally {
			const d = new Set(deleting);
			d.delete(responseId);
			deleting = d;
		}
	}

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
				const idx = cfg.options.findIndex((o) => o.id === str);
				return t?.options?.[idx] ?? str;
			}
			case 'checkboxes': {
				const arr = value as string[];
				const cfg = field.config as CheckboxesConfig;
				return arr.map((id) => {
					const idx = cfg.options.findIndex((o) => o.id === id);
					return t?.options?.[idx] ?? id;
				}).join(', ');
			}
			case 'dropdown': {
				const cfg = field.config as DropdownConfig;
				const idx = cfg.options.findIndex((o) => o.id === String(value));
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
			return new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
		} catch { return iso; }
	}

	function formatDateLong(iso: string): string {
		try {
			return new Date(iso).toLocaleString(undefined, { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
		} catch { return iso; }
	}

	const selectedRecord = $derived(responses.find(r => r.id === selectedId));
	const selectedDecrypted = $derived(selectedId ? decrypted.get(selectedId) : undefined);
	const isDecryptingSelected = $derived(selectedId ? decrypting.has(selectedId) : false);
	const selectedDecryptError = $derived(selectedId ? decryptErrors.get(selectedId) : undefined);
</script>

<svelte:head>
	<title>Confide — Responses</title>
</svelte:head>

<style>
	.responses-root {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
		height: 100%;
		font-family: monospace;
	}

	.top-bar {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 0 20px;
		height: 36px;
		border-bottom: 1px solid #1e2d3e;
		flex-shrink: 0;
	}

	.top-bar-name {
		font-size: 0.8rem;
		color: #c5d3e0;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.top-bar-divider {
		width: 1px;
		height: 12px;
		background: #2d3f55;
		flex-shrink: 0;
	}

	.top-bar-id {
		font-size: 0.72rem;
		color: #4b6280;
		white-space: nowrap;
	}

	.responses-shell {
		display: flex;
		flex: 1;
		min-height: 0;
		font-family: monospace;
	}

	/* ── Left panel ── */
	.list-panel {
		width: 280px;
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		border-right: 1px solid #243347;
		min-height: 0;
	}

	.list-header {
		padding: 12px 16px;
		border-bottom: 1px solid #243347;
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.list-title {
		font-size: 0.7rem;
		font-weight: 600;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: #4b6280;
		margin: 0;
		flex: 1;
	}

	.list-header-actions {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.icon-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: transparent;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		color: #4b6280;
		transition: color 120ms, background 120ms;
	}
	.icon-btn:hover { color: #8899aa; background: #1e2d3e; }
	.icon-btn:disabled { opacity: 0.3; cursor: not-allowed; }

	.list-scroll {
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden;
	}

	.list-item {
		display: block;
		width: 100%;
		padding: 11px 16px;
		text-align: left;
		background: transparent;
		border: none;
		border-bottom: 1px solid #1e2d3e;
		cursor: pointer;
		transition: background 100ms;
		font-family: monospace;
	}
	.list-item:hover { background: #1e2c3d; }
	.list-item.selected { background: #172030; border-left: 2px solid #3b82f6; padding-left: 14px; }
	.list-item.selected .item-id { color: #93c5fd; }

	.item-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.item-id {
		font-size: 0.75rem;
		color: #8899aa;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
	}

	.item-version {
		font-size: 0.65rem;
		color: #4b6280;
		background: #111e2d;
		border: 1px solid #243347;
		border-radius: 3px;
		padding: 1px 5px;
		flex-shrink: 0;
	}

	.item-date {
		font-size: 0.7rem;
		color: #4b6280;
		margin-top: 3px;
	}

	.item-decrypted-dot {
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: #22c55e;
		flex-shrink: 0;
	}

	.list-load-more {
		padding: 12px 16px;
		border-top: 1px solid #1e2d3e;
		flex-shrink: 0;
	}

	.btn-ghost {
		width: 100%;
		padding: 7px 12px;
		background: transparent;
		color: #4b6280;
		border: 1px solid #243347;
		border-radius: 4px;
		cursor: pointer;
		font-family: monospace;
		font-size: 0.75rem;
		transition: color 120ms, border-color 120ms;
	}
	.btn-ghost:hover:not(:disabled) { color: #8899aa; border-color: #374151; }
	.btn-ghost:disabled { opacity: 0.4; cursor: not-allowed; }

	/* ── Right panel ── */
	.detail-panel {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.detail-header {
		padding: 18px 24px 14px;
		border-bottom: 1px solid #243347;
		flex-shrink: 0;
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}

	.detail-header-meta {
		min-width: 0;
	}

	.detail-id {
		font-size: 0.8rem;
		color: #8899aa;
		margin: 0 0 4px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.detail-submitted {
		font-size: 0.72rem;
		color: #4b6280;
		margin: 0;
	}

	.detail-header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
	}

	.detail-scroll {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
	}

	.detail-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		color: #2d3f55;
		text-align: center;
		padding: 48px;
	}

	.detail-empty-icon {
		font-size: 2rem;
		margin-bottom: 12px;
		opacity: 0.4;
	}

	.detail-empty-text {
		font-size: 0.8rem;
		margin: 0;
	}

	.detail-fields {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.field-block {
		border-bottom: 1px solid #1e2d3e;
		padding-bottom: 20px;
	}
	.field-block:last-child {
		border-bottom: none;
		padding-bottom: 0;
	}

	.field-label {
		font-size: 0.7rem;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: #4b6280;
		margin: 0 0 6px;
	}

	.field-required {
		color: #f87171;
		margin-left: 2px;
	}

	.field-value {
		font-size: 0.875rem;
		color: #c5d3e0;
		margin: 0;
		line-height: 1.6;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.field-value.empty {
		color: #3a4f63;
		font-style: italic;
	}

	.decrypting-state {
		display: flex;
		align-items: center;
		gap: 10px;
		color: #4b6280;
		font-size: 0.8rem;
		padding: 32px 0;
	}

	.spinner {
		width: 14px;
		height: 14px;
		border: 2px solid #243347;
		border-top-color: #3b82f6;
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin { to { transform: rotate(360deg); } }

	.error-msg {
		color: #f87171;
		font-size: 0.8rem;
		padding: 12px 0;
		margin: 0;
	}

	.btn-delete {
		padding: 5px 12px;
		background: transparent;
		color: #f87171;
		border: 1px solid #374151;
		border-radius: 4px;
		cursor: pointer;
		font-family: monospace;
		font-size: 0.75rem;
		transition: background 120ms, border-color 120ms;
	}
	.btn-delete:hover { background: #1a0e0e; border-color: #7f1d1d; }

	.btn-confirm-delete {
		padding: 5px 12px;
		background: #7f1d1d;
		color: #fca5a5;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-family: monospace;
		font-size: 0.75rem;
	}

	.btn-cancel {
		padding: 5px 12px;
		background: transparent;
		color: #6b7280;
		border: 1px solid #374151;
		border-radius: 4px;
		cursor: pointer;
		font-family: monospace;
		font-size: 0.75rem;
	}

	.confirm-label {
		font-size: 0.72rem;
		color: #f87171;
	}

	.locale-badge {
		display: inline-block;
		font-size: 0.65rem;
		color: #4b6280;
		background: #111e2d;
		border: 1px solid #243347;
		border-radius: 3px;
		padding: 1px 6px;
		margin-left: 8px;
		vertical-align: middle;
	}

	.loading-full {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #4b6280;
		font-size: 0.8rem;
		gap: 10px;
	}

	.error-full {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #f87171;
		font-size: 0.8rem;
		padding: 32px;
		text-align: center;
	}
</style>

<div class="responses-root">
<div class="top-bar">
	<span class="top-bar-name">{formName ?? '—'}</span>
	<div class="top-bar-divider"></div>
	<span class="top-bar-id">{formId.slice(0, 16)}…</span>
	<div style="flex: 1;"></div>
	<a href="/forms/{formId}/edit" style="font-family: monospace; font-size: 0.72rem; color: #93c5fd; text-decoration: none; padding: 2px 10px; border: 1px solid #2d3f55; border-radius: 3px; white-space: nowrap;">→ Edit form</a>
</div>
<div class="responses-shell">
	<!-- ── Left: response list ── -->
	<div class="list-panel">
		<div class="list-header">
			<p class="list-title">Responses</p>
			<div class="list-header-actions">
				<!-- Mark all as read -->
				<button class="icon-btn" title="Mark all as read" onclick={() => {}}>
					<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M20 6L9 17l-5-5"/>
						<path d="M20 12L9 23l-5-5" opacity="0.4"/>
					</svg>
				</button>
				<!-- Refresh -->
				<button class="icon-btn" title="Refresh" disabled={loading || loadMore} onclick={() => loadResponses()}>
					<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/>
						<path d="M21 3v5h-5"/>
						<path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/>
						<path d="M8 16H3v5"/>
					</svg>
				</button>
			</div>
		</div>

		{#if loading}
			<div class="loading-full">
				<div class="spinner"></div>
				Loading…
			</div>
		{:else if loadError}
			<div class="error-full">{loadError}</div>
		{:else if responses.length === 0}
			<div class="detail-empty" style="flex: 1;">
				<div class="detail-empty-icon">○</div>
				<p class="detail-empty-text">No responses yet</p>
			</div>
		{:else}
			<div class="list-scroll">
				{#each responses as record, i (record.id)}
					<button
						class="list-item"
						class:selected={selectedId === record.id}
						onclick={() => selectResponse(record)}
					>
						<div class="item-row">
							<span class="item-id">#{i + 1} · {record.id.slice(0, 12)}…</span>
							{#if decrypted.has(record.id)}
								<span class="item-decrypted-dot" title="Decrypted"></span>
							{/if}
							<span class="item-version">v{record.schemaVersion}</span>
						</div>
						<div class="item-date">{formatDate(record.receivedAt)}</div>
					</button>
				{/each}
			</div>

			{#if hasMore}
				<div class="list-load-more">
					<button class="btn-ghost" onclick={() => loadResponses(nextCursor)} disabled={loadMore}>
						{loadMore ? 'Loading…' : 'Load more'}
					</button>
				</div>
			{/if}
		{/if}
	</div>

	<!-- ── Right: detail panel ── -->
	<div class="detail-panel">
		{#if !selectedRecord}
			<div class="detail-empty">
				<div class="detail-empty-icon">⊡</div>
				<p class="detail-empty-text">Select a response to view its contents</p>
			</div>
		{:else}
			<!-- Detail header -->
			<div class="detail-header">
				<div class="detail-header-meta">
					<p class="detail-id">{selectedRecord.id}</p>
					<p class="detail-submitted">
						Received {formatDateLong(selectedRecord.receivedAt)}
						{#if selectedDecrypted}
							<span class="locale-badge">{selectedDecrypted.locale}</span>
						{/if}
					</p>
				</div>

				<div class="detail-header-actions">
					{#if confirmDelete === selectedRecord.id}
						<span class="confirm-label">Delete?</span>
						<button
							class="btn-confirm-delete"
							onclick={() => handleDelete(selectedRecord.id)}
							disabled={deleting.has(selectedRecord.id)}
						>
							{deleting.has(selectedRecord.id) ? '…' : 'Confirm'}
						</button>
						<button class="btn-cancel" onclick={() => (confirmDelete = null)}>Cancel</button>
					{:else}
						<button class="btn-delete" onclick={() => (confirmDelete = selectedRecord.id)}>
							Delete
						</button>
					{/if}
				</div>
			</div>

			<!-- Detail content -->
			<div class="detail-scroll">
				{#if isDecryptingSelected}
					<div class="decrypting-state">
						<div class="spinner"></div>
						Decrypting…
					</div>
				{:else if selectedDecryptError}
					<p class="error-msg">{selectedDecryptError}</p>
				{:else if selectedDecrypted}
					<div class="detail-fields">
						{#each selectedDecrypted.schema.fields as field (field.id)}
							{#if field.type !== 'section_break'}
								{@const fieldT = (selectedDecrypted.schema.translations[selectedDecrypted.locale] ?? selectedDecrypted.schema.translations[selectedDecrypted.schema.defaultLocale])?.fields[field.id]}
								{@const answer = renderAnswer(field, selectedDecrypted)}
								<div class="field-block">
									<p class="field-label">
										{fieldT?.label ?? field.id}{#if field.required}<span class="field-required">*</span>{/if}
									</p>
									<p class="field-value" class:empty={answer === '—'}>{answer}</p>
								</div>
							{/if}
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	</div>
</div>
</div>
