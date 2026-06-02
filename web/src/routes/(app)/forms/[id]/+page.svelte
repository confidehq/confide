<script lang="ts">
import {
	Check,
	Clock,
	Copy,
	Download,
	ExternalLink,
	Link,
	Lock,
	Pencil,
	QrCode,
	RefreshCw,
	Search,
} from "@lucide/svelte";
import QRCode from "qrcode";
import { onMount } from "svelte";
import { goto } from "$app/navigation";
import { page } from "$app/stores";
import Breadcrumb from "$lib/components/Breadcrumb.svelte";
import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
import { getAppConfig } from "$lib/config";
import {
	decryptResponseRecord,
	deleteForm,
	deleteResponse,
	deriveShareUrl,
	type EncryptedResponseRecord,
	type FormRecord,
	getForm,
	getSchemaVersion,
	listResponses,
	publishForm,
	rotateRenderKey,
	updateFormExpiration,
	updateFormPGPNotification,
	updateFormStatus,
	validatePGPKey,
} from "$lib/forms";
import { auth } from "$lib/stores/auth.svelte";
import { formsStore } from "$lib/stores/forms.svelte";
import { workspacesStore } from "$lib/stores/workspaces.svelte";
import type {
	BuilderField,
	BuilderSchema,
	CheckboxesConfig,
	DropdownConfig,
	MultipleChoiceConfig,
	RatingConfig,
} from "$lib/types/builder";
import { type CustomDomainInfo, getCustomDomain } from "$lib/workspaces";

type AnswerValue = string | string[] | number | null | undefined;

interface DecryptedResponse {
	submittedAt: string;
	locale: string;
	answers: Record<string, AnswerValue>;
	schema: BuilderSchema;
}

const formId = $page.params.id ?? "";

// Navigate to /forms if the workspace changes while viewing this form
const mountedWorkspaceId = workspacesStore.active?.id;
$effect(() => {
	const id = workspacesStore.active?.id;
	if (id !== undefined && id !== mountedWorkspaceId) goto("/forms");
});

// ── Form ──────────────────────────────────────────────────────────────────
let record = $state<FormRecord | null>(null);
let resolvedFormKey = $state<CryptoKey | null>(null);
const formName = $derived(formsStore.formNames.get(formId) ?? "");
let formDescription = $state(formsStore.formDescriptions.get(formId) ?? "");
let loading = $state(true);
let loadError = $state("");

let statusSaving = $state(false);

let expiresAt = $state("");
let responseLimit = $state("");
let responseTtlDays = $state("");
let burnAfterReading = $state(false);
let settingsSaving = $state(false);
let settingsSaved = $state(false);
let settingsError = $state("");

let notificationEmail = $state("");
let pgpPublicKey = $state("");
let notificationFrom = $state("");
let notificationSubject = $state("");
let pgpPending = $state(false);
const pgpOpen = $derived(!!notificationEmail || pgpPending);
let emailEnabled = $state(false);
let smtpSender = $state("");
let pgpKeyFingerprint = $state("");
let pgpKeyError = $state("");

async function handlePGPKeyInput(value: string) {
	pgpPublicKey = value;
	pgpKeyFingerprint = "";
	pgpKeyError = "";
	if (!value.trim()) return;
	try {
		pgpKeyFingerprint = await validatePGPKey(value);
	} catch (e) {
		pgpKeyError = e instanceof Error ? e.message : "Invalid PGP key";
	}
}

let closeOnDatePending = $state(false);
let limitResponsesPending = $state(false);
let autoDeletePending = $state(false);
const closeOnDateOpen = $derived(!!expiresAt || closeOnDatePending);
const limitResponsesOpen = $derived(!!responseLimit || limitResponsesPending);
const settingsLifetimePolicy = $derived<"none" | "burn" | "ttl">(
	burnAfterReading ? "burn" : responseTtlDays ? "ttl" : "none",
);
const autoDeleteOpen = $derived(
	settingsLifetimePolicy !== "none" || autoDeletePending,
);

let shareUrl = $state("");
let shareUrlLoading = $state(false);
let publishing = $state(false);
let publishError = $state("");
let copied = $state(false);
let confirmRotate = $state(false);
let customDomainInfo = $state<CustomDomainInfo | null>(null);
let qrCanvas = $state<HTMLCanvasElement | null>(null);
let qrVisible = $state(false);
let qrError = $state("");

let pendingDeleteForm = $state(false);
let deleteFormLoading = $state(false);
let deleteFormError = $state("");

// ── Responses ─────────────────────────────────────────────────────────────
let responses = $state<EncryptedResponseRecord[]>([]);
let nextCursor = $state<string | undefined>(undefined);
let hasMore = $state(false);
let responsesLoading = $state(true);
let loadingMore = $state(false);
let responsesError = $state("");

// null means "details view"; a string means a response ID is selected
let selectedId = $state<string | null>(null);
let decrypted = $state<Map<string, DecryptedResponse>>(new Map());
const unreadCount = $derived(
	record ? Math.max(0, record.responseCount - decrypted.size) : 0,
);
let decrypting = $state<Set<string>>(new Set());
let decryptErrors = $state<Map<string, string>>(new Map());

let schemaCache = $state<Map<number, BuilderSchema>>(new Map());

let confirmDeleteResponse = $state<string | null>(null);
let deletingResponses = $state<Set<string>>(new Set());

// ── Init ─────────────────────────────────────────────────────────────────
onMount(async () => {
	if (!auth.masterKey) {
		goto("/login");
		return;
	}
	getAppConfig()
		.then((c) => {
			emailEnabled = c.emailEnabled;
			smtpSender = c.smtpSender;
		})
		.catch(() => {});
	await Promise.all([loadForm(), loadResponses()]);
});

// ── Form functions ────────────────────────────────────────────────────────
async function loadForm() {
	if (!auth.masterKey) return;
	loading = true;
	loadError = "";
	try {
		const {
			schema,
			record: r,
			formKey,
		} = await getForm(auth.masterKey, formId);
		record = r;
		resolvedFormKey = formKey;
		const title = schema.translations[schema.defaultLocale]?.formTitle;
		if (title) formsStore.updateName(formId, title);
		const desc = schema.translations[schema.defaultLocale]?.formDescription;
		if (desc) formDescription = desc;
		if (r.renderKeySalt && r.status !== "draft") {
			const cached = formsStore.shareUrls.get(formId);
			if (cached) {
				shareUrl = cached;
			} else {
				shareUrlLoading = true;
				if (r.workspaceId) {
					getCustomDomain(r.workspaceId)
						.then(async (d) => {
							customDomainInfo = d;
							const base =
								d?.enabled && d.domain ? `https://${d.domain}` : undefined;
							shareUrl = await deriveShareUrl(
								formId,
								r.renderKeySalt!,
								formKey,
								base,
							);
						})
						.catch(() => {})
						.finally(() => {
							shareUrlLoading = false;
						});
				} else {
					deriveShareUrl(formId, r.renderKeySalt, formKey)
						.then((u) => {
							shareUrl = u;
						})
						.catch(() => {})
						.finally(() => {
							shareUrlLoading = false;
						});
				}
			}
		}
		expiresAt = r.expiresAt ?? "";
		responseLimit = r.responseLimit != null ? String(r.responseLimit) : "";
		responseTtlDays =
			r.responseTtlDays != null ? String(r.responseTtlDays) : "";
		burnAfterReading = r.burnAfterReading ?? false;
		notificationEmail = r.notificationEmail ?? "";
		pgpPublicKey = r.pgpPublicKey ?? "";
		notificationFrom = r.notificationFrom ?? "";
		notificationSubject = r.notificationSubject ?? "";
		pgpPending = false;
		pgpKeyFingerprint = "";
		pgpKeyError = "";
		if (pgpPublicKey) {
			try {
				pgpKeyFingerprint = await validatePGPKey(pgpPublicKey);
			} catch {
				/* stored key shown as-is */
			}
		}
	} catch (e) {
		loadError = e instanceof Error ? e.message : "Failed to load form";
	} finally {
		loading = false;
	}
}

async function toggleStatus() {
	if (!record) return;
	statusSaving = true;
	const next = record.status === "open" ? "closed" : "open";
	try {
		await updateFormStatus(formId, next);
		record = { ...record, status: next };
	} finally {
		statusSaving = false;
	}
}

async function saveSettings() {
	if (pgpOpen && !notificationEmail.trim()) {
		settingsError =
			"A recipient email address is required for email forwarding.";
		return;
	}
	if (pgpOpen && !pgpPublicKey.trim()) {
		settingsError = "A PGP public key is required for email forwarding.";
		return;
	}
	if (pgpPublicKey && pgpKeyError) {
		settingsError = pgpKeyError;
		return;
	}
	settingsSaving = true;
	settingsError = "";
	settingsSaved = false;
	try {
		await Promise.all([
			updateFormExpiration(
				formId,
				expiresAt || null,
				responseLimit ? parseInt(responseLimit) : null,
				responseTtlDays ? parseInt(responseTtlDays) : null,
				burnAfterReading,
			),
			updateFormPGPNotification(
				formId,
				notificationEmail,
				pgpPublicKey,
				notificationFrom,
				notificationSubject,
			),
		]);
		if (record) {
			record = {
				...record,
				expiresAt: expiresAt || null,
				responseLimit: responseLimit ? parseInt(responseLimit) : null,
				responseTtlDays: responseTtlDays ? parseInt(responseTtlDays) : null,
				burnAfterReading,
			};
		}
		settingsSaved = true;
		setTimeout(() => {
			settingsSaved = false;
		}, 2500);
	} catch (e) {
		settingsError = e instanceof Error ? e.message : "Failed to save settings";
	} finally {
		settingsSaving = false;
	}
}

async function handlePublish() {
	if (!auth.masterKey || !record) return;
	publishing = true;
	publishError = "";
	try {
		const salt = record.renderKeySalt
			? Uint8Array.from(atob(record.renderKeySalt), (c) => c.charCodeAt(0))
			: null;
		const { schema, formKey } = await getForm(
			auth.masterKey,
			formId,
			undefined,
		);
		const base =
			customDomainBase() ??
			(await getAppConfig().then((c) =>
				c.formsDomain ? `https://${c.formsDomain}` : undefined,
			));
		const result = await publishForm(
			auth.masterKey,
			formId,
			schema as any,
			salt,
			formKey,
			base,
		);
		shareUrl = result.shareUrl;
		// publishForm atomically sets status='open' on the server
		record = { ...record, status: "open", hasUnpublishedChanges: false };
		formsStore.updateStatus(formId, "open");
	} catch (e) {
		publishError = e instanceof Error ? e.message : "Publish failed";
	} finally {
		publishing = false;
	}
}

async function copyShareUrl() {
	if (!shareUrl) return;
	await navigator.clipboard.writeText(shareUrl);
	copied = true;
	setTimeout(() => {
		copied = false;
	}, 2000);
}

function customDomainBase(): string | undefined {
	if (customDomainInfo?.enabled && customDomainInfo.domain)
		return `https://${customDomainInfo.domain}`;
	return undefined;
}

async function handleRotateKey() {
	if (!auth.masterKey || !record) return;
	publishing = true;
	publishError = "";
	confirmRotate = false;
	try {
		const { schema, formKey } = await getForm(
			auth.masterKey,
			formId,
			undefined,
		);
		const result = await rotateRenderKey(
			auth.masterKey,
			formId,
			schema as any,
			formKey,
			customDomainBase(),
		);
		shareUrl = result.shareUrl;
		record = {
			...record,
			renderKeySalt: btoa(String.fromCharCode(...result.renderKeySalt)),
		};
		qrVisible = false;
	} catch (e) {
		publishError = e instanceof Error ? e.message : "Key rotation failed";
	} finally {
		publishing = false;
	}
}

async function showQRCode() {
	if (!shareUrl) return;
	qrVisible = true;
	qrError = "";
	await new Promise((r) => setTimeout(r, 0));
	try {
		if (qrCanvas)
			await QRCode.toCanvas(qrCanvas, shareUrl, { width: 240, margin: 2 });
	} catch {
		qrError = "Failed to generate QR code";
	}
}

function downloadQR() {
	if (!qrCanvas) return;
	const a = document.createElement("a");
	a.href = qrCanvas.toDataURL("image/png");
	a.download = `form-qr-${formId}.png`;
	a.click();
}

async function confirmDeleteForm() {
	deleteFormLoading = true;
	deleteFormError = "";
	try {
		await deleteForm(formId);
		goto("/forms");
	} catch (e) {
		deleteFormError = e instanceof Error ? e.message : "Failed to delete form";
	} finally {
		deleteFormLoading = false;
	}
}

// ── Response functions ────────────────────────────────────────────────────
async function loadResponses(cursor?: string) {
	if (!auth.masterKey) return;
	if (cursor) loadingMore = true;
	else responsesLoading = true;
	responsesError = "";
	try {
		const result = await listResponses(formId, cursor, 25);
		responses = cursor ? [...responses, ...result.responses] : result.responses;
		nextCursor = result.nextCursor;
		hasMore = !!result.nextCursor;
	} catch (err) {
		responsesError =
			err instanceof Error ? err.message : "Failed to load responses";
	} finally {
		responsesLoading = false;
		loadingMore = false;
	}
}

async function selectResponse(id: string) {
	selectedId = id;
	const rec = responses.find((r) => r.id === id);
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
			schema = await getSchemaVersion(
				auth.masterKey,
				formId,
				rec.schemaVersion,
				resolvedFormKey ?? undefined,
			);
			schemaCache = new Map([...schemaCache, [rec.schemaVersion, schema]]);
		}
		const payload = await decryptResponseRecord(
			auth.masterKey,
			formId,
			rec,
			resolvedFormKey ?? undefined,
		);
		decrypted = new Map([
			...decrypted,
			[
				rec.id,
				{
					submittedAt: payload.submittedAt,
					locale: payload.locale,
					answers: payload.answers as Record<string, AnswerValue>,
					schema,
				},
			],
		]);
	} catch (err) {
		decryptErrors = new Map([
			...decryptErrors,
			[rec.id, err instanceof Error ? err.message : "Decryption failed"],
		]);
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
		responses = responses.filter((r) => r.id !== responseId);
		const nd = new Map(decrypted);
		nd.delete(responseId);
		decrypted = nd;
		confirmDeleteResponse = null;
		if (selectedId === responseId) {
			selectedId = null;
		}
		// Update response count on the record
		if (record)
			record = {
				...record,
				responseCount: Math.max(0, record.responseCount - 1),
			};
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
	const t = (
		d.schema.translations[d.locale] ??
		d.schema.translations[d.schema.defaultLocale]
	)?.fields[field.id];
	if (value === null || value === undefined) return "—";
	switch (field.type) {
		case "short_text":
		case "long_text":
			return String(value);
		case "multiple_choice": {
			const str = String(value);
			if (str.startsWith("other:")) return `Other: ${str.slice(6)}`;
			const cfg = field.config as MultipleChoiceConfig;
			const idx = cfg.options.findIndex((o) => o.id === str);
			return t?.options?.[idx] ?? str;
		}
		case "checkboxes": {
			const arr = value as string[];
			const cfg = field.config as CheckboxesConfig;
			return arr
				.map((id) => {
					const idx = cfg.options.findIndex((o) => o.id === id);
					return t?.options?.[idx] ?? id;
				})
				.join(", ");
		}
		case "dropdown": {
			const cfg = field.config as DropdownConfig;
			const idx = cfg.options.findIndex((o) => o.id === String(value));
			return t?.options?.[idx] ?? String(value);
		}
		case "date_time":
			return String(value);
		case "rating": {
			const cfg = field.config as RatingConfig;
			return `${value} / ${cfg.scale}`;
		}
		default:
			return String(value);
	}
}

function formatDate(iso: string): string {
	try {
		return new Date(iso).toLocaleDateString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
		});
	} catch {
		return iso;
	}
}

function formatDateShort(iso: string): string {
	try {
		return new Date(iso).toLocaleString(undefined, {
			month: "short",
			day: "numeric",
			hour: "2-digit",
			minute: "2-digit",
		});
	} catch {
		return iso;
	}
}

function formatDateLong(iso: string): string {
	try {
		return new Date(iso).toLocaleString(undefined, {
			year: "numeric",
			month: "long",
			day: "numeric",
			hour: "2-digit",
			minute: "2-digit",
			second: "2-digit",
		});
	} catch {
		return iso;
	}
}

// ── Derived ───────────────────────────────────────────────────────────────
const statusColor = $derived(
	record?.status === "open"
		? "bg-success"
		: record?.status === "draft"
			? "bg-warn-light"
			: "bg-muted",
);
const selectedRecord = $derived(responses.find((r) => r.id === selectedId));
const selectedDecrypted = $derived(
	selectedId ? decrypted.get(selectedId) : undefined,
);
const isDecryptingSelected = $derived(
	selectedId ? decrypting.has(selectedId) : false,
);
const selectedDecryptError = $derived(
	selectedId ? decryptErrors.get(selectedId) : undefined,
);

// ── Search / filter ───────────────────────────────────────────────────────
let searchQuery = $state("");
let activeTab = $state<"All" | "Unread">("All");

const filteredResponses = $derived(
	responses.filter((resp) => {
		if (activeTab === "Unread" && decrypted.has(resp.id)) return false;
		const q = searchQuery.trim().toLowerCase();
		if (!q) return true;
		if (resp.id.toLowerCase().includes(q)) return true;
		const d = decrypted.get(resp.id);
		if (d) {
			for (const v of Object.values(d.answers)) {
				if (
					String(v ?? "")
						.toLowerCase()
						.includes(q)
				)
					return true;
			}
		}
		return false;
	}),
);

// ── Avatar helpers ────────────────────────────────────────────────────────
const AVATAR_COLORS = [
	{ bg: "#1D2739", color: "#7191CA" },
	{ bg: "#1D391E", color: "#58AE5B" },
	{ bg: "#39341D", color: "#B7A449" },
	{ bg: "#391D1D", color: "#C37D7D" },
	{ bg: "#2D1F3D", color: "#A78BFA" },
	{ bg: "#1D2D39", color: "#5AAFCA" },
];

function getInitials(d: DecryptedResponse | undefined): string {
	if (!d) return "?";
	const firstShort = d.schema.fields.find((f) => f.type === "short_text");
	if (firstShort) {
		const val = String(d.answers[firstShort.id] ?? "").trim();
		const parts = val.split(/\s+/);
		if (parts.length >= 2 && parts[0] && parts[1])
			return (parts[0][0] + parts[1][0]).toUpperCase();
		if (parts[0]?.length >= 2) return parts[0].slice(0, 2).toUpperCase();
	}
	return "?";
}

function getDisplayName(d: DecryptedResponse | undefined, idx: number): string {
	if (!d) return `Response #${idx + 1}`;
	const firstShort = d.schema.fields.find((f) => f.type === "short_text");
	if (firstShort) {
		const val = String(d.answers[firstShort.id] ?? "").trim();
		if (val) return val;
	}
	return `Response #${idx + 1}`;
}

function getPreviewText(d: DecryptedResponse | undefined): string {
	if (!d) return "";
	const longField = d.schema.fields.find((f) => f.type === "long_text");
	if (longField) {
		const val = String(d.answers[longField.id] ?? "").trim();
		if (val) return val.slice(0, 90);
	}
	for (const f of d.schema.fields
		.filter((f) => f.type === "short_text")
		.slice(1)) {
		const val = String(d.answers[f.id] ?? "").trim();
		if (val) return val;
	}
	return "";
}

function avatarColorForIdx(idx: number) {
	return AVATAR_COLORS[idx % AVATAR_COLORS.length];
}

function responseIndexInFull(id: string): number {
	return responses.findIndex((r) => r.id === id);
}
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
	<div class="flex items-center gap-2 sm:gap-3 px-4 sm:px-6 h-10 border-b border-border-canvas shrink-0 overflow-hidden bg-canvas">
		<Breadcrumb items={[
			{ label: 'Forms', href: '/forms' },
			{ label: formName || formId.slice(0, 12) + '…', onclick: selectedId ? () => { selectedId = null; } : undefined },
			...(selectedRecord ? [{ label: selectedRecord.id.slice(0, 8) + '…' }] : [])
		]} />
		<div class="flex-1 min-w-0"></div>
		<a
			href="/forms/{formId}/edit"
			class="shrink-0 font-mono text-sm text-subtle no-underline px-3 py-1.5 border border-border-canvas rounded whitespace-nowrap
				hover:text-text hover:border-border transition-colors duration-100 flex items-center gap-1.5"
		>
			<Pencil size={11} strokeWidth={2} />
			<span class="hidden sm:inline">Edit form</span>
			<span class="sm:hidden">Edit</span>
		</a>
	</div>

	<!-- Shell -->
	<div class="flex flex-col sm:flex-row flex-1 min-h-0">

		<!-- Left panel: response list (desktop only) -->
		<div class="hidden sm:flex sm:w-96 shrink-0 flex-col border-r border-border-canvas min-h-0 bg-canvas">

			<!-- List header -->
			<div class="px-3 pt-3 pb-2 shrink-0">
				<div class="flex items-center justify-between gap-2 mb-2.5">
					<div class="flex items-center gap-2">
						<span class="text-sm font-bold tracking-[0.12em] uppercase text-muted">Responses</span>
						{#if record}
							<span class="bg-surface text-subtle text-sm font-bold px-1.5 py-0.5 rounded-full border border-border-canvas">{record.responseCount}</span>
						{/if}
					</div>
					<button
						title="Refresh"
						disabled={responsesLoading || loadingMore}
						onclick={() => loadResponses()}
						class="flex items-center justify-center w-6 h-6 bg-transparent border-none rounded cursor-pointer text-muted transition-[color,background] duration-100 hover:text-subtle hover:bg-surface disabled:opacity-30 disabled:cursor-not-allowed"
					>
						<RefreshCw size={12} strokeWidth={2} />
					</button>
				</div>
				<!-- Search -->
				<div class="flex items-center gap-2 bg-surface border border-border-canvas rounded px-2.5 py-1.5 mb-2">
					<Search size={11} class="text-muted shrink-0" strokeWidth={2} />
					<input
						type="text"
						placeholder="Search…"
						bind:value={searchQuery}
						class="flex-1 bg-transparent border-none outline-none text-xs text-text placeholder:text-muted font-mono"
					/>
				</div>
				<!-- Filter tabs -->
				<div class="flex gap-0.5">
					{#each (['All', 'Unread'] as const) as tab}
						<button
							onclick={() => { activeTab = tab; }}
							class="px-2.5 py-1 rounded text-sm font-medium cursor-pointer border-none transition-all duration-100
								{activeTab === tab ? 'bg-surface text-text' : 'bg-transparent text-muted hover:text-subtle'}"
						>{tab}</button>
					{/each}
				</div>
			</div>

			{#if responsesLoading}
				<div class="flex-1 flex items-center justify-center text-muted text-xs gap-2">
					<div class="spinner w-3 h-3 border-2 border-surface border-t-info-border rounded-full"></div>
					Loading…
				</div>
			{:else if responsesError}
				<div class="flex-1 flex items-center justify-center text-danger text-xs p-6 text-center">{responsesError}</div>
			{:else if responses.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center text-border-subtle text-center p-12">
					<div class="text-4xl text-subtle mb-3 opacity-40">○</div>
					<p class="text-subtle m-0">No responses yet</p>
				</div>
			{:else}
				<div class="flex-1 overflow-y-auto overflow-x-hidden">
					{#each filteredResponses as resp (resp.id)}
						{@const globalIdx = responseIndexInFull(resp.id)}
						{@const isDecrypted = decrypted.has(resp.id)}
						{@const d = decrypted.get(resp.id)}
						{@const ac = avatarColorForIdx(globalIdx)}
						{@const preview = getPreviewText(d)}
						{@const displayName = getDisplayName(d, globalIdx)}
						<button
							onclick={() => selectResponse(resp.id)}
							class="block w-full px-3 py-2.5 text-left bg-transparent border-none border-b border-border-canvas cursor-pointer transition-[background] duration-100 hover:bg-surface
								{selectedId === resp.id ? 'bg-highlight-low border-l-2 border-l-info-light !pl-2.5' : ''}"
						>
							<div class="flex items-start gap-2.5">
								<div
									class="w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold shrink-0 mt-0.5"
									style="background:{ac.bg};color:{ac.color}"
								>
									{isDecrypted ? getInitials(d) : String(globalIdx + 1)}
								</div>
								<div class="flex-1 min-w-0">
									<div class="flex items-center justify-between gap-1.5">
										<span class="text-xs font-semibold truncate {selectedId === resp.id ? 'text-text' : 'text-subtle'}">
											{#if !isDecrypted}<span class="inline-block w-1.5 h-1.5 rounded-full bg-info-light align-middle mr-1 -mt-px"></span>{/if}{displayName}
										</span>
										<span class="text-sm text-muted shrink-0">{formatDateShort(resp.receivedAt)}</span>
									</div>
									{#if preview}
										<p class="m-0 mt-0.5 text-sm text-muted leading-snug overflow-hidden" style="display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical">{preview}</p>
									{:else}
										<p class="m-0 mt-0.5 text-sm text-muted font-mono truncate">{resp.id.slice(0, 14)}…</p>
									{/if}
								</div>
							</div>
						</button>
					{/each}

					{#if filteredResponses.length === 0 && (searchQuery || activeTab !== 'All')}
						<div class="px-4 py-8 text-center text-xs text-muted">No responses match your filter.</div>
					{/if}
				</div>

				{#if hasMore}
					<div class="px-3 py-2.5 border-t border-border-canvas shrink-0">
						<button
							onclick={() => loadResponses(nextCursor)}
							disabled={loadingMore}
							class="w-full px-3 py-1.5 bg-transparent text-muted border border-border-canvas rounded text-xs cursor-pointer font-mono transition-[color,border-color] duration-100 hover:not-disabled:text-subtle hover:not-disabled:border-border disabled:opacity-40 disabled:cursor-not-allowed"
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
				<div class="flex-1 overflow-y-auto">
					{#if loading}
						<div class="flex items-center justify-center gap-2 text-muted text-sm py-16">
							<div class="spinner w-3 h-3 border-2 border-surface border-t-info-border rounded-full"></div>
							Loading…
						</div>
					{:else if loadError}
						<div class="p-8 text-sm text-danger">{loadError}</div>
					{:else if record}
						<!-- Form hero -->
						<div class="px-8 pt-8 pb-7 border-b border-border-canvas">
							<div class="max-w-4xl">
								<div class="flex items-start justify-between gap-4 mb-5">
									<div class="min-w-0">
										<h1 class="m-0 text-2xl text-text font-semibold leading-tight truncate">{formName || formId.slice(0, 12) + '…'}</h1>
										{#if formDescription}
											<p class="m-0 mt-2 text-sm text-muted leading-relaxed max-w-lg">{formDescription}</p>
										{/if}
									</div>
									<!-- Status pill -->
									<div class="flex items-center gap-1.5 px-3 py-1 rounded-full shrink-0 border
										{record.status === 'open'
											? 'bg-success-dark border-success-light/30'
											: record.status === 'closed'
												? 'bg-info-dark border-info-light/30'
												: 'bg-highlight-low border-border-canvas'}">
										<span class="w-1.5 h-1.5 rounded-full shrink-0
											{record.status === 'open' ? 'bg-success-light animate-pulse' : record.status === 'closed' ? 'bg-info-light' : record.status === 'draft' ? 'bg-warn-light' : 'bg-muted'}"></span>
										<span class="text-sm font-bold uppercase tracking-wider
											{record.status === 'open' ? 'text-success-light' : record.status === 'closed' ? 'text-info-light' : 'text-muted'}">{record.status}</span>
									</div>
								</div>

								<!-- Stats row -->
								<div class="flex gap-0">
									<div class="flex flex-col gap-0.5 pr-6">
										<span class="text-2xl font-semibold tabular-nums text-text">{record.responseCount}</span>
										<span class="text-sm font-bold uppercase tracking-wider text-muted">Total</span>
									</div>
									{#if unreadCount > 0}
										<div class="flex flex-col gap-0.5 px-6 border-l border-border-canvas">
											<span class="text-2xl font-semibold tabular-nums text-info-light">{unreadCount}</span>
											<span class="text-sm font-bold uppercase tracking-wider text-muted">Unread</span>
										</div>
									{/if}
								</div>

								<!-- Status actions -->
								<div class="flex items-center gap-2 mt-5">
									{#if record.status === 'draft'}
										<a
											href="/forms/{formId}/edit"
											class="px-3 py-1.5 text-sm font-mono border rounded no-underline transition-colors duration-100
												bg-transparent text-info-light border-info-light/50 hover:bg-info-dark hover:border-info-light"
										>Publish</a>
									{:else}
										<button
											onclick={toggleStatus}
											disabled={statusSaving}
											class="px-3 py-1.5 text-sm font-mono border rounded cursor-pointer transition-colors duration-100
												{statusSaving
													? 'bg-transparent text-muted border-border-canvas cursor-not-allowed'
													: record.status === 'open'
														? 'bg-transparent text-danger border-danger-light hover:bg-danger-light hover:border-danger-dark hover:text-white'
														: 'bg-transparent text-success border-success-light hover:bg-success-light hover:border-success-dark'}"
										>{statusSaving ? '…' : record.status === 'open' ? 'Close form' : 'Reopen form'}</button>
									{/if}
								</div>
							</div>
						</div>

						<!-- Mobile response list -->
						<div class="sm:hidden border-b border-border-canvas">
							{#if responsesLoading}
								<div class="flex items-center justify-center text-muted text-sm gap-2 py-8">
									<div class="spinner w-3 h-3 border-2 border-surface border-t-info-border rounded-full"></div>
									Loading…
								</div>
							{:else if responsesError}
								<div class="flex items-center justify-center text-danger text-sm p-6 text-center">{responsesError}</div>
							{:else if responses.length === 0}
								<div class="flex flex-col items-center justify-center text-center py-8 px-6">
									<p class="text-sm text-muted m-0">No responses yet</p>
								</div>
							{:else}
								<div class="px-6 py-2 border-b border-border-canvas flex items-center justify-between">
									<span class="text-sm text-muted uppercase tracking-[0.12em] font-bold">Responses</span>
								</div>
								<div>
									{#each responses as resp, i (resp.id)}
										{@const isDecrypted = decrypted.has(resp.id)}
										{@const d = decrypted.get(resp.id)}
										{@const ac = avatarColorForIdx(i)}
										{@const preview = getPreviewText(d)}
										{@const displayName = getDisplayName(d, i)}
										<button
											onclick={() => selectResponse(resp.id)}
											class="block w-full px-4 py-3 text-left bg-transparent border-none border-b border-border-canvas cursor-pointer transition-[background] duration-100 hover:bg-surface"
										>
											<div class="flex items-start gap-3">
												<div class="w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold shrink-0 mt-0.5" style="background:{ac.bg};color:{ac.color}">
													{isDecrypted ? getInitials(d) : String(i + 1)}
												</div>
												<div class="flex-1 min-w-0">
													<div class="flex items-center justify-between gap-2">
														<span class="text-sm font-semibold text-subtle truncate">{displayName}</span>
														<span class="text-xs text-muted shrink-0">{formatDateShort(resp.receivedAt)}</span>
													</div>
													{#if preview}
														<p class="m-0 mt-0.5 text-xs text-muted truncate">{preview}</p>
													{/if}
												</div>
											</div>
										</button>
									{/each}
								</div>
								{#if hasMore}
									<div class="px-6 py-3">
										<button
											onclick={() => loadResponses(nextCursor)}
											disabled={loadingMore}
											class="w-full px-3 py-1.5 bg-transparent text-muted border border-border-canvas rounded text-sm cursor-pointer font-mono transition-[color,border-color] duration-100 hover:not-disabled:text-subtle hover:not-disabled:border-border disabled:opacity-40 disabled:cursor-not-allowed"
										>
											{loadingMore ? 'Loading…' : 'Load more'}
										</button>
									</div>
								{/if}
							{/if}
						</div>

						<!-- Settings body -->
						<div class="px-8 py-7 max-w-4xl flex flex-col gap-9">

							<!-- Share section -->
							<section class="flex flex-col gap-3">
								<h2 class="m-0 font-bold tracking-[0.12em] uppercase text-muted mb-1">Share</h2>
								{#if record.status === 'draft'}
									<div class="rounded-lg border border-border-canvas bg-canvas px-5 py-5 flex flex-col gap-3">
										<div>
											<p class="m-0 text-text font-medium">This form is unpublished</p>
											<p class="m-0 text-sm text-muted mt-1.5">Publish to make it accessible and generate a share link.</p>
										</div>
										{#if publishError}
											<p class="m-0 text-sm text-danger">{publishError}</p>
										{/if}
										<div>
											<button
												onclick={handlePublish}
												disabled={publishing}
												class="flex items-center gap-2 px-4 py-2 bg-primary text-white border-none rounded font-mono text-sm cursor-pointer transition-[background] duration-100 hover:bg-primary-hover disabled:opacity-60 disabled:cursor-not-allowed"
											>
												{#if publishing}
													<div class="spinner w-3 h-3 border-2 border-white/30 border-t-white rounded-full"></div>
													Publishing…
												{:else}
													Publish form
												{/if}
											</button>
										</div>
									</div>
								{:else if shareUrlLoading || !shareUrl}
									<p class="m-0 text-sm text-muted">Loading link…</p>
								{:else}
									<div class="rounded-lg border border-border-canvas overflow-hidden">
										<!-- Link row -->
										<div class="flex items-center gap-3 px-4 py-3">
											<div class="w-7 h-7 rounded-md bg-surface border border-border-canvas flex items-center justify-center shrink-0">
												<Link size={12} strokeWidth={2} class="text-subtle" />
											</div>
											<div class="flex-1 min-w-0">
												<p class="m-0 font-semibold text-text mb-0.5">Direct link</p>
												<p class="m-0 text-sm text-muted font-mono truncate">{shareUrl}</p>
											</div>
											<button
												onclick={copyShareUrl}
												class="shrink-0 px-2.5 py-1.5 rounded font-mono transition-[background,color] duration-150 grid items-center
													{copied
														? 'bg-success-light text-success cursor-default'
														: 'bg-primary text-white hover:bg-primary-hover cursor-pointer'}"
											>
												<!-- Both labels in same grid cell — width stays fixed -->
												<span class="col-start-1 row-start-1 flex items-center justify-center gap-1 {copied ? '' : 'invisible'}">
													<Check size={10} strokeWidth={2.5} />Copied
												</span>
												<span class="col-start-1 row-start-1 flex items-center justify-center {copied ? 'invisible' : ''}">
													Copy secure link
												</span>
											</button>
										</div>

										<!-- Status notice -->
										{#if record.status === 'closed' || record.hasUnpublishedChanges || (customDomainInfo?.enabled && customDomainInfo.domain)}
											<div class="px-4 py-2.5 border-t border-border-canvas bg-surface/50">
												{#if record.status === 'closed'}
													<p class="m-0 text-sm text-info-light">Form is closed — link is active but not accepting responses.</p>
												{:else if record.hasUnpublishedChanges}
													<p class="m-0 text-sm text-warn">Showing last published version. <a href="/forms/{formId}/edit" class="text-text underline">Edit</a> to publish latest changes.</p>
												{/if}
												{#if customDomainInfo?.enabled && customDomainInfo.domain}
													<p class="m-0 text-sm text-muted">Served on <span class="text-subtle">{customDomainInfo.domain}</span></p>
												{/if}
											</div>
										{/if}

										<!-- QR row -->
										<div class="px-4 py-3 border-t border-border-canvas">
											{#if !qrVisible}
												<button
													onclick={showQRCode}
													class="flex items-center gap-1.5 px-3 py-1.5 text-sm text-muted font-mono border border-border-canvas rounded bg-transparent cursor-pointer transition-colors duration-100 hover:text-text hover:border-border"
												><QrCode size={11} strokeWidth={2} />Get QR code</button>
											{:else}
												<div class="flex flex-col items-center gap-2">
													<canvas bind:this={qrCanvas} class="rounded border border-border-canvas"></canvas>
													<div class="flex items-center gap-2">
														<button
															onclick={downloadQR}
															class="flex items-center gap-1.5 text-sm text-muted hover:text-subtle cursor-pointer bg-transparent border-none p-0 font-mono transition-colors duration-100"
														><Download size={11} strokeWidth={2} />Download PNG</button>
														<span class="text-muted text-sm">·</span>
														<button
															onclick={() => { qrVisible = false; }}
															class="text-sm text-muted hover:text-subtle cursor-pointer bg-transparent border-none p-0 font-mono"
														>Hide</button>
													</div>
												</div>
											{/if}
											{#if qrError}<p class="m-0 mt-1 text-xs text-danger">{qrError}</p>{/if}
										</div>

										<!-- Rotate row -->
										<div class="px-4 py-3 border-t border-border-canvas">
											{#if publishError}
												<p class="m-0 mb-2 text-xs text-danger">{publishError}</p>
											{/if}
											<button
												onclick={() => { confirmRotate = true; }}
												disabled={publishing}
												class="px-3 py-1.5 text-sm font-mono border rounded transition-colors duration-100
													{publishing ? 'cursor-not-allowed opacity-50 text-muted border-border-canvas bg-transparent' : 'cursor-pointer text-muted border-border-canvas bg-transparent hover:text-text hover:border-border'}"
											>Generate new link</button>
											<p class="m-0 mt-2 text-xs text-muted leading-relaxed">QR code stays valid when you edit your form. Rotating your link will require a new QR code.</p>
										</div>
									</div>
								{/if}
							</section>

							<!-- Form settings section -->
							<section class="flex flex-col gap-3">
								<h2 class="m-0 font-bold tracking-[0.12em] uppercase text-muted mb-1">Form settings</h2>
								<div class="rounded-lg border border-border-canvas overflow-hidden">

									<!-- Close on date -->
									<div class="px-4 py-3.5 border-b border-border-canvas last:border-b-0">
										<div class="flex items-center justify-between gap-4">
											<div class="min-w-0">
												<p class="m-0 text-text font-medium">Close on date</p>
												<p class="m-0 text-sm text-muted mt-0.5">Stop accepting responses after a specific date.</p>
											</div>
											<button
												role="switch"
												aria-label="Close on date"
												aria-checked={closeOnDateOpen}
												onclick={() => {
													if (closeOnDateOpen) { closeOnDatePending = false; expiresAt = ''; }
													else { closeOnDatePending = true; }
												}}
												class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
													{closeOnDateOpen ? 'bg-primary' : 'bg-surface'}"
											>
												<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
													{closeOnDateOpen ? 'translate-x-5' : 'translate-x-0'}"></span>
											</button>
										</div>
										{#if closeOnDateOpen}
											<div class="mt-3">
												<input type="date" bind:value={expiresAt} class="input-base" />
											</div>
										{/if}
									</div>

									<!-- Limit responses -->
									<div class="px-4 py-3.5 border-b border-border-canvas">
										<div class="flex items-center justify-between gap-4">
											<div class="min-w-0">
												<p class="m-0 text-text font-medium">Limit total responses</p>
												<p class="m-0 text-sm text-muted mt-0.5">Automatically close after a set number of submissions.</p>
											</div>
											<button
												role="switch"
												aria-label="Limit total responses"
												aria-checked={limitResponsesOpen}
												onclick={() => {
													if (limitResponsesOpen) { limitResponsesPending = false; responseLimit = ''; }
													else { limitResponsesPending = true; }
												}}
												class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
													{limitResponsesOpen ? 'bg-primary' : 'bg-surface'}"
											>
												<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
													{limitResponsesOpen ? 'translate-x-5' : 'translate-x-0'}"></span>
											</button>
										</div>
										{#if limitResponsesOpen}
											<div class="mt-3">
												<input type="number" min="1" placeholder="e.g. 100" bind:value={responseLimit} class="input-base" />
											</div>
										{/if}
									</div>

									<!-- Auto delete -->
									<div class="px-4 py-3.5 border-b border-border-canvas">
										<div class="flex items-center justify-between gap-4">
											<div class="min-w-0">
												<p class="m-0 text-text font-medium">Auto-delete responses</p>
												<p class="m-0 text-sm text-muted mt-0.5">Remove responses from our servers after a set period.</p>
											</div>
											<button
												role="switch"
												aria-label="Auto-delete responses"
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
												class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
													{autoDeleteOpen ? 'bg-primary' : 'bg-surface'}"
											>
												<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
													{autoDeleteOpen ? 'translate-x-5' : 'translate-x-0'}"></span>
											</button>
										</div>
										{#if autoDeleteOpen}
											<div class="mt-3 flex flex-col gap-2.5">
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
													<div class="flex gap-2 items-center">
														<input type="number" min="1" placeholder="Days" bind:value={responseTtlDays} class="input-base" />
														<span class="text-sm text-muted shrink-0">days</span>
													</div>
												{:else if settingsLifetimePolicy === 'burn'}
													<p class="m-0 text-sm text-muted leading-relaxed">Responses are scheduled for deletion once you view them. They remain visible until the next cleanup pass.</p>
												{/if}
											</div>
										{/if}
									</div>

									<!-- Email forwarding -->
									<div class="px-4 py-3.5">
										<div class="flex items-center justify-between gap-4">
											<div class="min-w-0">
												<p class="m-0 text-text font-medium">Email forwarding</p>
												<p class="m-0 text-sm text-muted mt-0.5">Forward encrypted responses to an email address via PGP.</p>
												{#if !emailEnabled}
													<p class="m-0 text-sm text-warn-light mt-1">Email is not configured on this server.</p>
												{/if}
											</div>
											<button
												role="switch"
												aria-label="Email forwarding"
												aria-checked={pgpOpen}
												disabled={!emailEnabled}
												onclick={() => {
													if (!emailEnabled) return;
													if (pgpOpen) { pgpPending = false; notificationEmail = ''; pgpPublicKey = ''; }
													else { pgpPending = true; }
												}}
												class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none
													{emailEnabled ? 'cursor-pointer' : 'cursor-not-allowed opacity-40'}
													{pgpOpen ? 'bg-primary' : 'bg-surface'}"
											>
												<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
													{pgpOpen ? 'translate-x-5' : 'translate-x-0'}"></span>
											</button>
										</div>
										{#if pgpOpen || notificationEmail}
											<div class="mt-3 flex flex-col gap-4">
												<!-- Email headers -->
												<div>
													<div class="border border-border-canvas rounded overflow-hidden">
														<div class="flex items-center border-b border-border-canvas">
															<span class="w-20 shrink-0 px-3 py-2 text-sm text-muted border-r border-border-canvas">To</span>
															<input
																type="email"
																placeholder="recipient@example.com"
																bind:value={notificationEmail}
																class="flex-1 min-w-0 px-3 py-2 bg-transparent border-none outline-none text-sm text-text placeholder:text-muted font-mono"
															/>
														</div>
														<div class="flex items-center border-b border-border-canvas">
															<span class="w-20 shrink-0 px-3 py-2 text-xs text-muted border-r border-border-canvas">From</span>
															<span class="flex-1 min-w-0 px-3 py-2 text-xs text-muted font-mono truncate">
																{smtpSender || 'Confide Forms <notifications@example.com>'}
															</span>
														</div>
														<div class="flex items-center">
															<span class="w-20 shrink-0 px-3 py-2 text-xs text-muted border-r border-border-canvas">Subject</span>
															<input
																type="text"
																placeholder="New Confide Form submission"
																bind:value={notificationSubject}
																class="flex-1 min-w-0 px-3 py-2 bg-transparent border-none outline-none text-text placeholder:text-muted font-mono"
															/>
														</div>
													</div>
													<p class="m-0 mt-1.5 text-xs text-muted">To, From, and Subject are stored unencrypted on our servers.</p>
												</div>

												<!-- PGP key -->
												<div>
													<p class="m-0 mb-1.5 text-xs text-subtle uppercase tracking-[0.08em]">PGP Public Key</p>
													<div class="border rounded overflow-hidden {pgpKeyError ? 'border-danger-light' : 'border-border-canvas'}">
														<textarea
															placeholder="-----BEGIN PGP PUBLIC KEY BLOCK-----"
															value={pgpPublicKey}
															oninput={(e) => handlePGPKeyInput((e.target as HTMLTextAreaElement).value)}
															rows={5}
															class="w-full px-3 py-2.5 bg-transparent border-none outline-none text-xs text-text placeholder:text-muted font-mono resize-y block"
														></textarea>
													</div>
													<p class="m-0 mt-1.5 text-xs {pgpKeyError ? 'text-danger' : pgpKeyFingerprint ? 'text-success-light font-mono tracking-wide' : 'text-muted'}">
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

								<!-- Save button -->
								<div class="flex items-center gap-3 mt-1">
									<button
										onclick={saveSettings}
										disabled={settingsSaving || !!pgpKeyError}
										class="px-4 py-2 border rounded font-mono cursor-pointer transition-colors duration-100
											{settingsSaving || pgpKeyError
												? 'bg-transparent text-muted border-border-canvas cursor-not-allowed'
												: 'bg-transparent text-text border-border hover:bg-surface hover:border-border'}"
									>
										{settingsSaving ? 'Saving…' : 'Save settings'}
									</button>
									{#if settingsSaved}
										<span class="text-success-light flex items-center gap-1.5">
											<Check size={11} strokeWidth={2.5} />
											Saved
										</span>
									{/if}
									{#if settingsError}
										<span class="text-sm text-danger">{settingsError}</span>
									{/if}
								</div>
							</section>

							<!-- Danger zone -->
							<section class="flex flex-col gap-3">
								<h2 class="m-0 font-bold tracking-[0.12em] uppercase text-muted mb-1">Danger zone</h2>
								<div class="border border-danger-light/30 rounded-lg px-5 py-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-6">
									<div class="min-w-0">
										<p class="m-0 text-text font-medium">Delete this form</p>
										<p class="m-0 mt-1 text-sm text-muted">
											Permanently deletes the form and all {record.responseCount} response{record.responseCount === 1 ? '' : 's'}. Cannot be undone.
										</p>
									</div>
									<button
										onclick={() => { pendingDeleteForm = true; }}
										class="sm:shrink-0 px-4 py-2 bg-transparent text-danger-light border border-danger-light/50 rounded cursor-pointer font-mono text-sm
											hover:bg-danger-dark hover:border-danger-light transition-colors duration-100"
									>Delete form</button>
								</div>
							</section>

						</div>
					{/if}
				</div>

			{:else}
				<!-- ── Response detail view ───────────────────────────────────── -->
				{#if !selectedRecord}
					<div class="flex-1 flex flex-col items-center justify-center text-center p-12">
						<p class="text-muted m-0">Response not found</p>
					</div>
				{:else}
					{@const detailIdx = responseIndexInFull(selectedRecord.id)}
					{@const detailAc = avatarColorForIdx(detailIdx)}
					{@const detailInitials = selectedDecrypted ? getInitials(selectedDecrypted) : String(detailIdx + 1)}
					{@const detailName = selectedDecrypted ? getDisplayName(selectedDecrypted, detailIdx) : `Response #${detailIdx + 1}`}

					<!-- Detail header -->
					<div class="px-6 py-5 border-b border-border-canvas shrink-0 bg-canvas">
						<div class="flex items-start gap-3.5">
							<!-- Avatar -->
							<div
								class="w-10 h-10 rounded-full flex items-center justify-center font-bold shrink-0"
								style="background:{detailAc.bg};color:{detailAc.color}"
							>{detailInitials}</div>
							<!-- Name + ID -->
							<div class="flex-1 min-w-0">
								<p class="m-0 font-semibold text-text truncate">{detailName}</p>
								<p class="m-0 text-muted font-mono mt-0.5 truncate">{selectedRecord.id}</p>
							</div>
							<!-- Actions -->
							<div class="flex items-center gap-2 shrink-0">
								<button
									onclick={() => (confirmDeleteResponse = selectedRecord.id)}
									class="px-3 py-1.5 bg-transparent text-danger-light border border-danger-light/50 rounded cursor-pointer font-mono text-xs transition-colors duration-100 hover:bg-danger-dark hover:border-danger-light"
								>Delete</button>
							</div>
						</div>

						<!-- Meta row -->
						<div class="flex gap-4 mt-4 flex-wrap items-center">
							<div class="flex items-center gap-1.5 text-muted">
								<Clock size={11} strokeWidth={2} />
								{formatDateLong(selectedRecord.receivedAt)}
							</div>
							<div class="flex items-center gap-1.5 text-success-light">
								<Lock size={11} strokeWidth={2} />
								End-to-end encrypted
							</div>
							{#if selectedDecrypted}
								<span class="text-muted bg-surface border border-border-canvas rounded px-1.5 py-0.5 font-mono">{selectedDecrypted.locale}</span>
							{/if}
						</div>

						<button
							class="sm:hidden mt-3 text-xs text-muted hover:text-subtle bg-transparent border-none cursor-pointer p-0 font-mono transition-colors duration-100"
							onclick={() => { selectedId = null; }}
						>← All responses</button>
					</div>

					<!-- Detail content -->
					<div class="flex-1 overflow-y-auto px-6 py-6">
						{#if isDecryptingSelected}
							<div class="flex items-center gap-2 text-muted py-8">
								<div class="spinner w-3 h-3 border-2 border-surface border-t-info-border rounded-full"></div>
								Decrypting…
							</div>
						{:else if selectedDecryptError}
							<p class="text-danger py-3 m-0">{selectedDecryptError}</p>
						{:else if selectedDecrypted}
							<div class="flex flex-col gap-5 max-w-2xl">
								{#each selectedDecrypted.schema.fields as field (field.id)}
									{#if field.type !== 'section_break'}
										{@const fieldT = (selectedDecrypted.schema.translations[selectedDecrypted.locale] ?? selectedDecrypted.schema.translations[selectedDecrypted.schema.defaultLocale])?.fields[field.id]}
										{@const answer = renderAnswer(field, selectedDecrypted)}
										<div>
											<p class="font-bold tracking-[0.1em] uppercase text-muted m-0 mb-2">
												{fieldT?.label ?? field.id}{#if field.required}<span class="text-danger ml-0.5">*</span>{/if}
											</p>
											<div class="bg-canvas border border-border-canvas rounded-lg px-4 py-3">
												<p class="text-sm text-text m-0 leading-relaxed whitespace-pre-wrap break-words
													{answer === '—' ? 'text-muted italic' : ''}">
													{answer}
												</p>
											</div>
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
