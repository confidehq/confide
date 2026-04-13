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
	import { CheckCheck, RefreshCw } from '@lucide/svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import Breadcrumb from '$lib/components/Breadcrumb.svelte';

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
	let resolvedFormKey = $state<CryptoKey | null>(null);

	onMount(async () => {
		if (!auth.masterKey) {
			goto('/login');
			return;
		}
		await loadResponses();
		getForm(auth.masterKey, formId).then(({ schema, formKey }) => {
			formName = schema.translations[schema.defaultLocale]?.formTitle ?? null;
			resolvedFormKey = formKey;
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

			const payload = await decryptResponseRecord(auth.masterKey, formId, record, resolvedFormKey ?? undefined);
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
	@keyframes spin { to { transform: rotate(360deg); } }
	.spinner { animation: spin 0.7s linear infinite; }
</style>

<ConfirmDialog
	open={!!confirmDelete}
	title="Delete response?"
	description="This will permanently delete this response. This cannot be undone."
	loading={confirmDelete ? deleting.has(confirmDelete) : false}
	onconfirm={() => confirmDelete && handleDelete(confirmDelete)}
	oncancel={() => (confirmDelete = null)}
/>

<!-- Root -->
<div class="flex flex-col flex-1 min-h-0 h-full font-mono">

	<!-- Top bar -->
	<div class="flex items-center gap-3 px-5 h-9 border-b border-border-deep shrink-0 overflow-hidden">
		<Breadcrumb items={[
			{ label: 'Forms', href: '/forms' },
			{ label: formName || formId.slice(0, 12) + '…', href: `/forms/${formId}` },
			...(selectedRecord ? [{ label: selectedRecord.id }] : [])
		]} />
		<div class="flex-1 shrink-0"></div>
	</div>

	<!-- Shell -->
	<div class="flex flex-1 min-h-0 font-mono">

		<!-- Left panel: response list -->
		<div class="w-[280px] shrink-0 flex flex-col border-r border-[#243347] min-h-0">

			<!-- List header -->
			<div class="px-4 py-3 border-b border-[#243347] shrink-0 flex items-center justify-between gap-2">
				<p class="text-xs font-semibold tracking-[0.1em] uppercase text-[#4b6280] m-0 flex-1">Responses</p>
				<div class="flex items-center gap-1">
					<button
						title="Mark all as read"
						onclick={() => {}}
						class="flex items-center justify-center w-7 h-7 bg-transparent border-none rounded cursor-pointer text-[#4b6280] transition-[color,background] duration-100 hover:text-[#8899aa] hover:bg-border-deep"
					>
						<CheckCheck size={15} strokeWidth={2} />
					</button>
					<button
						title="Refresh"
						disabled={loading || loadMore}
						onclick={() => loadResponses()}
						class="flex items-center justify-center w-7 h-7 bg-transparent border-none rounded cursor-pointer text-[#4b6280] transition-[color,background] duration-100 hover:text-[#8899aa] hover:bg-border-deep disabled:opacity-30 disabled:cursor-not-allowed"
					>
						<RefreshCw size={15} strokeWidth={2} />
					</button>
				</div>
			</div>

			{#if loading}
				<div class="flex-1 flex items-center justify-center text-[#4b6280] text-sm gap-2.5">
					<div class="spinner w-3.5 h-3.5 border-2 border-[#243347] border-t-[#3b82f6] rounded-full"></div>
					Loading…
				</div>
			{:else if loadError}
				<div class="flex-1 flex items-center justify-center text-error-light text-sm p-8 text-center">{loadError}</div>
			{:else if responses.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
					<div class="text-4xl mb-3 opacity-40">○</div>
					<p class="text-sm m-0">No responses yet</p>
				</div>
			{:else}
				<div class="flex-1 overflow-y-auto overflow-x-hidden">
					{#each responses as record, i (record.id)}
						<button
							onclick={() => selectResponse(record)}
							class="block w-full px-4 py-[11px] text-left bg-transparent border-none border-b border-border-deep cursor-pointer transition-[background] duration-100 font-mono hover:bg-[#1e2c3d]
								{selectedId === record.id ? 'bg-[#172030] border-l-2 border-l-[#3b82f6] pl-[14px]' : ''}"
						>
							<div class="flex items-center justify-between gap-2">
								<span class="text-sm overflow-hidden text-ellipsis whitespace-nowrap flex-1
									{selectedId === record.id ? 'text-[#93c5fd]' : 'text-[#8899aa]'}">
									#{i + 1} · {record.id.slice(0, 12)}…
								</span>
								{#if decrypted.has(record.id)}
									<span class="w-[5px] h-[5px] rounded-full bg-[#22c55e] shrink-0" title="Decrypted"></span>
								{/if}
								<span class="text-xs text-[#4b6280] bg-[#111e2d] border border-[#243347] rounded-sm px-1.5 py-px shrink-0">v{record.schemaVersion}</span>
							</div>
							<div class="text-xs text-[#4b6280] mt-0.5">{formatDate(record.receivedAt)}</div>
						</button>
					{/each}
				</div>

				{#if hasMore}
					<div class="px-4 py-3 border-t border-border-deep shrink-0">
						<button
							onclick={() => loadResponses(nextCursor)}
							disabled={loadMore}
							class="w-full px-3 py-1.5 bg-transparent text-[#4b6280] border border-[#243347] rounded cursor-pointer font-mono text-sm transition-[color,border-color] duration-100 hover:not-disabled:text-[#8899aa] hover:not-disabled:border-border disabled:opacity-40 disabled:cursor-not-allowed"
						>
							{loadMore ? 'Loading…' : 'Load more'}
						</button>
					</div>
				{/if}
			{/if}
		</div>

		<!-- Right panel: detail -->
		<div class="flex-1 min-w-0 flex flex-col min-h-0">
			{#if !selectedRecord}
				<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
					<div class="text-4xl mb-3 opacity-40">⊡</div>
					<p class="text-sm m-0">Select a response to view its contents</p>
				</div>
			{:else}
				<!-- Detail header -->
				<div class="px-6 pt-5 pb-4 border-b border-[#243347] shrink-0 flex items-start justify-between gap-4">
					<div class="min-w-0">
						<p class="text-base text-[#8899aa] m-0 mb-1 overflow-hidden text-ellipsis whitespace-nowrap">{selectedRecord.id}</p>
						<p class="text-sm text-[#4b6280] m-0">
							Received {formatDateLong(selectedRecord.receivedAt)}
							{#if selectedDecrypted}
								<span class="inline-block text-xs text-[#4b6280] bg-[#111e2d] border border-[#243347] rounded-sm px-1.5 py-px ml-2 align-middle">{selectedDecrypted.locale}</span>
							{/if}
						</p>
					</div>

					<div class="flex items-center gap-2 shrink-0">
						<button
							onclick={() => (confirmDelete = selectedRecord.id)}
							class="px-4 py-2 bg-transparent text-[#f87171] border border-[#1e3048] rounded cursor-pointer font-mono text-base transition-colors duration-100 hover:bg-[#1a0e0e] hover:border-[#7f1d1d]"
						>
							Delete
						</button>
					</div>
				</div>

				<!-- Detail content -->
				<div class="flex-1 overflow-y-auto p-6">
					{#if isDecryptingSelected}
						<div class="flex items-center gap-2.5 text-[#4b6280] text-base py-8">
							<div class="spinner w-3.5 h-3.5 border-2 border-[#243347] border-t-[#3b82f6] rounded-full"></div>
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
										<p class="text-sm font-semibold tracking-[0.08em] uppercase text-[#4b6280] m-0 mb-2">
											{fieldT?.label ?? field.id}{#if field.required}<span class="text-error-light ml-0.5">*</span>{/if}
										</p>
										<p class="text-base text-[#c5d3e0] m-0 leading-relaxed whitespace-pre-wrap break-words
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
		</div>
	</div>
</div>
