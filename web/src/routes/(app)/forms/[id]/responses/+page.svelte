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
		ApiError,
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
	let loadError = $state('');
	let totalResponseCount = $state(0);

	// Per-response state
	let decrypted = $state<Map<string, DecryptedResponse>>(new Map());
	let decrypting = $state<Set<string>>(new Set());
	let decryptErrors = $state<Map<string, string>>(new Map());
	let confirmDelete = $state<string | null>(null); // responseId pending deletion
	let deleting = $state<Set<string>>(new Set());

	// Schema version cache: version number → decrypted schema
	let schemaCache = $state<Map<number, BuilderSchema>>(new Map());

	onMount(async () => {
		if (!auth.masterKey) {
			goto('/login');
			return;
		}
		await loadResponses();
	});

	async function loadResponses(cursor?: string) {
		if (!auth.masterKey) return;
		loading = true;
		loadError = '';
		try {
			const result = await listResponses(formId, cursor, 25);
			if (cursor) {
				responses = [...responses, ...result.responses];
			} else {
				responses = result.responses;
				totalResponseCount = result.responses.length; // seed; bumped on delete
			}
			nextCursor = result.nextCursor;
			hasMore = !!result.nextCursor;
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load responses';
		} finally {
			loading = false;
		}
	}

	async function handleDecrypt(record: EncryptedResponseRecord) {
		if (!auth.masterKey || decrypted.has(record.id)) return;

		const nextDecrypting = new Set(decrypting);
		nextDecrypting.add(record.id);
		decrypting = nextDecrypting;

		const nextErrors = new Map(decryptErrors);
		nextErrors.delete(record.id);
		decryptErrors = nextErrors;

		try {
			// Fetch schema version if not cached
			let schema = schemaCache.get(record.schemaVersion);
			if (!schema) {
				schema = await getSchemaVersion(auth.masterKey, formId, record.schemaVersion);
				const nextCache = new Map(schemaCache);
				nextCache.set(record.schemaVersion, schema);
				schemaCache = nextCache;
			}

			const payload = await decryptResponseRecord(auth.masterKey, formId, record);

			const nextDecrypted = new Map(decrypted);
			nextDecrypted.set(record.id, {
				submittedAt: payload.submittedAt,
				locale: payload.locale,
				answers: payload.answers as Record<string, AnswerValue>,
				schema
			});
			decrypted = nextDecrypted;
		} catch (err) {
			const nextErrors2 = new Map(decryptErrors);
			nextErrors2.set(record.id, err instanceof Error ? err.message : 'Decryption failed');
			decryptErrors = nextErrors2;
		} finally {
			const nextDecrypting2 = new Set(decrypting);
			nextDecrypting2.delete(record.id);
			decrypting = nextDecrypting2;
		}
	}

	async function handleDelete(responseId: string) {
		const nextDeleting = new Set(deleting);
		nextDeleting.add(responseId);
		deleting = nextDeleting;
		try {
			await deleteResponse(formId, responseId);
			responses = responses.filter((r) => r.id !== responseId);
			const nextDecrypted = new Map(decrypted);
			nextDecrypted.delete(responseId);
			decrypted = nextDecrypted;
			confirmDelete = null;
		} catch {
			// keep confirm open; user can retry
		} finally {
			const nextDeleting2 = new Set(deleting);
			nextDeleting2.delete(responseId);
			deleting = nextDeleting2;
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
				return arr
					.map((id) => {
						const idx = cfg.options.findIndex((o) => o.id === id);
						return t?.options?.[idx] ?? id;
					})
					.join(', ');
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
			return new Date(iso).toLocaleString();
		} catch {
			return iso;
		}
	}
</script>

<svelte:head>
	<title>GhostForm — Responses</title>
</svelte:head>

<div style="font-family: monospace; max-width: 900px; margin: 60px auto; padding: 0 24px;">
	<!-- Header -->
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 32px;">
		<div>
			<a href="/forms" style="color: #9ca3af; font-size: 0.85rem; text-decoration: none;">← Forms</a>
			<h1 style="font-size: 1.2rem; margin: 8px 0 0;">Responses</h1>
			<p style="margin: 4px 0 0; font-size: 0.8rem; color: #6b7280;">
				{formId.slice(0, 16)}… · {responses.length} loaded
			</p>
		</div>
		<a
			href="/forms/{formId}/edit"
			style="padding: 6px 14px; background: transparent; color: #93c5fd; border: 1px solid #374151; border-radius: 4px; font-family: monospace; font-size: 0.8rem; text-decoration: none;"
		>
			Edit form
		</a>
	</div>

	{#if loading && responses.length === 0}
		<p style="color: #6b7280; font-size: 0.9rem;">Loading…</p>
	{:else if loadError}
		<p style="color: #f87171; font-size: 0.9rem;">{loadError}</p>
	{:else if responses.length === 0}
		<div style="padding: 48px 32px; border: 1px dashed #374151; border-radius: 8px; text-align: center; color: #6b7280;">
			<p style="margin: 0 0 8px; font-size: 0.95rem;">No responses yet</p>
			<p style="margin: 0; font-size: 0.8rem;">Responses appear here after submission (up to 60s delay)</p>
		</div>
	{:else}
		<div style="display: flex; flex-direction: column; gap: 8px;">
			{#each responses as record (record.id)}
				{@const d = decrypted.get(record.id)}
				{@const isDecrypting = decrypting.has(record.id)}
				{@const decryptErr = decryptErrors.get(record.id)}
				{@const isConfirmDelete = confirmDelete === record.id}
				{@const isDeleting = deleting.has(record.id)}

				<div style="border: 1px solid #374151; border-radius: 6px; background: #1f2937; overflow: hidden;">
					<!-- Row header -->
					<div style="display: flex; align-items: center; gap: 12px; padding: 12px 16px; font-size: 0.8rem;">
						<span style="color: #9ca3af; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
							{record.id.slice(0, 16)}…
						</span>
						<span style="color: #6b7280; flex-shrink: 0;">{formatDate(record.receivedAt)}</span>
						<span style="
							padding: 2px 8px;
							background: #111827;
							color: #6b7280;
							border-radius: 9999px;
							font-size: 0.7rem;
							flex-shrink: 0;
						">v{record.schemaVersion}</span>

						<!-- Actions -->
						{#if isConfirmDelete}
							<span style="color: #f87171; font-size: 0.75rem; flex-shrink: 0;">Delete?</span>
							<button
								onclick={() => handleDelete(record.id)}
								disabled={isDeleting}
								style="padding: 3px 10px; background: #7f1d1d; color: #fca5a5; border: none; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.75rem;"
							>
								{isDeleting ? '…' : 'Confirm'}
							</button>
							<button
								onclick={() => (confirmDelete = null)}
								style="padding: 3px 10px; background: transparent; color: #9ca3af; border: 1px solid #374151; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.75rem;"
							>
								Cancel
							</button>
						{:else}
							{#if !d}
								<button
									onclick={() => handleDecrypt(record)}
									disabled={isDecrypting}
									style="padding: 3px 10px; background: transparent; color: #93c5fd; border: 1px solid #374151; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.75rem;"
								>
									{isDecrypting ? '…' : 'Decrypt'}
								</button>
							{/if}
							<button
								onclick={() => (confirmDelete = record.id)}
								style="padding: 3px 10px; background: transparent; color: #f87171; border: 1px solid #374151; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.75rem;"
							>
								×
							</button>
						{/if}
					</div>

					<!-- Decrypt error -->
					{#if decryptErr}
						<div style="padding: 8px 16px; border-top: 1px solid #374151; background: #1a0e0e;">
							<p style="margin: 0; font-size: 0.75rem; color: #f87171;">{decryptErr}</p>
						</div>
					{/if}

					<!-- Decrypted content -->
					{#if d}
						<div style="border-top: 1px solid #374151; padding: 16px;">
							<p style="margin: 0 0 12px; font-size: 0.75rem; color: #6b7280;">
								Submitted {formatDate(d.submittedAt)} · Locale: {d.locale}
							</p>
							<div style="display: flex; flex-direction: column; gap: 12px;">
								{#each d.schema.fields as field (field.id)}
									{#if field.type !== 'section_break'}
										{@const fieldT = (d.schema.translations[d.locale] ?? d.schema.translations[d.schema.defaultLocale])?.fields[field.id]}
										<div>
											<p style="margin: 0 0 4px; font-size: 0.75rem; color: #9ca3af;">
												{fieldT?.label ?? field.id}{field.required ? ' *' : ''}
											</p>
											<p style="margin: 0; font-size: 0.85rem; color: #d1d5db; white-space: pre-wrap; word-break: break-word;">
												{renderAnswer(field, d)}
											</p>
										</div>
									{/if}
								{/each}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		{#if hasMore}
			<button
				onclick={() => loadResponses(nextCursor)}
				disabled={loading}
				style="margin-top: 16px; padding: 8px 20px; background: transparent; color: #9ca3af; border: 1px solid #374151; border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 0.8rem;"
			>
				{loading ? 'Loading…' : 'Load more'}
			</button>
		{/if}
	{/if}
</div>
