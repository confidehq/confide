<script lang="ts">
import {
	Check,
	ChevronDown,
	Download,
	Languages,
	Link,
	QrCode,
	X,
} from "@lucide/svelte";
import QRCode from "qrcode";
import { untrack } from "svelte";
import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
import { deriveShareUrl, publishForm, rotateRenderKey } from "$lib/forms";
import { access } from "$lib/stores/access.svelte";
import { auth } from "$lib/stores/auth.svelte";
import type { createBuilderStore } from "$lib/stores/builder.svelte";
import type { CustomDomainInfo } from "$lib/workspaces";

const LANGUAGES: { code: string; name: string }[] = [
	{ code: "en", name: "English" },
	{ code: "es", name: "Spanish" },
	{ code: "fr", name: "French" },
	{ code: "pt", name: "Portuguese" },
];

function languageName(code: string): string {
	return LANGUAGES.find((l) => l.code === code)?.name ?? code;
}

interface Props {
	store: ReturnType<typeof createBuilderStore>;
	formId: string;
	workspaceId?: string;
	workspaceDomain: CustomDomainInfo | null;
	customDomainBase: () => string | undefined;
}

const { store, formId, workspaceId, workspaceDomain, customDomainBase }: Props =
	$props();

let closeOnDatePending = $state(false);
let limitResponsesPending = $state(false);
let autoDeletePending = $state(false);
let completionMessagePending = $state(false);
const closeOnDateOpen = $derived(!!store.expiresAt || closeOnDatePending);
const limitResponsesOpen = $derived(
	!!store.responseLimit || limitResponsesPending,
);
const completionMessageOpen = $derived(
	!!store.activeTranslation?.convoCompletionMessage || completionMessagePending,
);

let shareUrl = $state("");
let shareUrlLoading = $state(false);
let publishing = $state(false);
let publishError = $state("");
let copied = $state(false);
let copiedTimer: ReturnType<typeof setTimeout> | null = null;
let confirmRotate = $state(false);
let confirmRemoveLocale = $state<string | null>(null);

const isConvo = $derived(store.schema.layout === "convo");

let expirationSaving = $state(false);
let expirationError = $state<string | null>(null);

// Re-derive share URL whenever the custom domain becomes available or changes.
// Using $effect instead of onMount so it re-runs after the parent's async
// getCustomDomain() fetch resolves and updates customDomainBase().
$effect(() => {
	const base = customDomainBase();
	const { formStatus, renderKeySalt, formKey } = store;
	if (formStatus === "draft" || !renderKeySalt || !formKey) return;
	const saltBase64 = btoa(String.fromCharCode(...renderKeySalt));
	shareUrlLoading = true;
	deriveShareUrl(formId, saltBase64, formKey, base)
		.then((url) => {
			untrack(() => {
				shareUrl = url;
				shareUrlLoading = false;
			});
		})
		.catch(() => {
			untrack(() => {
				shareUrlLoading = false;
			});
		});
});

const isFirstPublish = $derived(store.formStatus === "draft");
const publishButtonLabel = $derived(
	publishing
		? "Publishing…"
		: isFirstPublish
			? "Publish"
			: store.hasUnpublishedChanges
				? "Update"
				: "Up to date",
);
const publishButtonDisabled = $derived(
	store.saving ||
		publishing ||
		(!isFirstPublish && !store.hasUnpublishedChanges),
);

async function handlePublish() {
	if (!auth.masterKey) return;
	publishing = true;
	publishError = "";
	try {
		await store.flushSave();
		const result = await publishForm(
			auth.masterKey,
			formId,
			store.schema,
			store.renderKeySalt,
			store.formKey ?? undefined,
			customDomainBase(),
		);
		store.setRenderKeySalt(result.renderKeySalt);
		store.markPublished();
		shareUrl = result.shareUrl;
	} catch (err) {
		publishError = err instanceof Error ? err.message : "Publish failed";
	} finally {
		publishing = false;
	}
}

async function handleRotateKey() {
	if (!auth.masterKey) return;
	publishing = true;
	publishError = "";
	try {
		const result = await rotateRenderKey(
			auth.masterKey,
			formId,
			store.schema,
			store.formKey ?? undefined,
			customDomainBase(),
		);
		store.setRenderKeySalt(result.renderKeySalt);
		store.markPublished();
		shareUrl = result.shareUrl;
	} catch (err) {
		publishError = err instanceof Error ? err.message : "Key rotation failed";
	} finally {
		publishing = false;
	}
}

function copyShareUrl() {
	navigator.clipboard.writeText(shareUrl);
	copied = true;
	if (copiedTimer) clearTimeout(copiedTimer);
	copiedTimer = setTimeout(() => {
		copied = false;
	}, 2000);
}

async function applyExpiration(
	newExpiresAt: string | null,
	newResponseLimit: number | null,
	newTtlDays: number | null,
	newBurnAfterReading: boolean,
) {
	if (newExpiresAt && newExpiresAt < nowLocal()) {
		expirationError = "Close date must be in the future.";
		return;
	}
	expirationSaving = true;
	expirationError = null;
	try {
		await store.setExpiration(
			newExpiresAt,
			newResponseLimit,
			newTtlDays,
			newBurnAfterReading,
		);
	} catch {
		expirationError = "Failed to save — please try again.";
	} finally {
		expirationSaving = false;
	}
}

type ResponseLifetimePolicy = "none" | "burn" | "ttl";

const responseLifetimePolicy = $derived<ResponseLifetimePolicy>(
	store.burnAfterReading ? "burn" : store.responseTtlDays ? "ttl" : "none",
);

const autoDeleteOpen = $derived(
	responseLifetimePolicy !== "none" || autoDeletePending,
);

function applyResponseLifetime(
	policy: ResponseLifetimePolicy,
	ttlDays: number | null,
) {
	const burn = policy === "burn";
	const days = policy === "ttl" ? ttlDays : null;
	applyExpiration(store.expiresAt, store.responseLimit, days, burn);
}

function toDatetimeLocal(value: string | null): string {
	if (!value) return '';
	return value.slice(0, 16);
}

function nowLocal(): string {
	const now = new Date();
	return new Date(now.getTime() - now.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}

function handleAddLanguage(e: Event) {
	const code = (e.target as HTMLSelectElement).value;
	if (!code) return;
	store.addLocale(code);
	(e.target as HTMLSelectElement).value = "";
}

const availableLanguages = $derived(
	LANGUAGES.filter((l) => !store.schema.locales.includes(l.code)),
);

let qrCanvas = $state<HTMLCanvasElement | null>(null);
let qrVisible = $state(false);
let qrError = $state("");

async function showQRCode() {
	if (!shareUrl) return;
	qrVisible = true;
	qrError = "";
	// Render after DOM update
	await new Promise((r) => setTimeout(r, 0));
	try {
		if (qrCanvas) {
			await QRCode.toCanvas(qrCanvas, shareUrl, { width: 240, margin: 2 });
		}
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
</script>

<aside
	class="form-settings-panel {store.showFormSettings ? 'is-open' : ''}
		fixed bottom-0 left-0 right-0 max-h-[65vh] rounded-t-xl
		sm:absolute sm:top-2 sm:bottom-2 sm:left-auto sm:right-2 sm:w-96 sm:max-h-none sm:rounded-xl
		bg-base border border-border-canvas overflow-y-auto z-20 flex flex-col"
>
	<!-- Mobile drag handle -->
	<div class="sm:hidden flex justify-center pt-2.5 pb-1 shrink-0 sticky top-0 bg-base">
		<div class="w-8 h-1 bg-border rounded-full"></div>
	</div>

	<!-- Header -->
	<div class="flex items-center px-5 h-9 shrink-0 border-b border-border sticky top-0 bg-base z-10">
		<p class="m-0 text-sm text-subtle uppercase tracking-[0.05em]">Settings</p>
	</div>

	<!-- Scrollable content -->
	<div class="flex-1 overflow-y-auto">

	<!-- Publish section -->
	<div class="p-5">
		<div class="flex flex-col gap-3">
				{#if store.formStatus === 'draft'}
					<div class="py-4 flex flex-col items-center gap-2 text-center">
						<p class="m-0 text-sm text-text">This form is unpublished</p>
						<p class="m-0 text-xs text-subtle">Publish to make it accessible and get a share link.</p>
					</div>
				{:else if shareUrlLoading || !shareUrl}
					<div class="py-4 flex flex-col items-center gap-2 text-center">
						<p class="m-0 text-xs text-subtle">Loading link…</p>
					</div>
				{:else}
					<div class="flex gap-1.5">
						<input
							type="text"
							readonly
							value={shareUrl}
							class="flex-1 px-3 py-2 bg-canvas border border-border-canvas text-text rounded-md font-mono text-sm outline-none min-w-0"
						/>
						<button
							onclick={copyShareUrl}
							class="shrink-0 px-3 py-2 border-none rounded-md font-mono text-sm transition-[background,color] duration-150 grid items-center
								{copied ? 'bg-success-light text-success cursor-default' : 'bg-primary text-white hover:bg-primary-hover cursor-pointer'}"
						>
							<!-- Both labels share the same grid cell so the button width never changes -->
							<span class="col-start-1 row-start-1 flex items-center justify-center gap-1.5 {copied ? '' : 'invisible'}">
								<Check size={13} strokeWidth={2} />Copied
							</span>
							<span class="col-start-1 row-start-1 flex items-center justify-center gap-1.5 {copied ? 'invisible' : ''}">
								<Link size={13} strokeWidth={1.75} />Copy secure link
							</span>
						</button>
					</div>
					{#if store.formStatus === 'closed'}
						<p class="m-0 text-xs text-closed-text">This form is closed — the link is active but not accepting responses.</p>
					{:else}
						<p class="m-0 text-xs text-subtle">Anyone with the link can access this form.</p>
					{/if}

					{#if workspaceDomain?.enabled && workspaceDomain.domain}
						<p class="m-0 text-xs text-subtle font-mono">
							Served on <span class="text-text">{workspaceDomain.domain}</span>
						</p>
					{/if}

					<!-- QR Code section -->
					<div class="border-t border-border pt-3 flex flex-col gap-2">
						{#if !qrVisible}
							<button
								onclick={showQRCode}
								class="px-3 py-2 bg-transparent text-subtle border border-border rounded-md cursor-pointer font-mono text-sm flex items-center gap-1.5 hover:text-text hover:border-border transition-colors duration-100"
							>
								<QrCode size={13} strokeWidth={1.75} />Get QR code
							</button>
						{:else}
							<div class="flex flex-col items-center gap-2">
								<canvas bind:this={qrCanvas} class="rounded-md"></canvas>
								<button
									onclick={downloadQR}
									class="px-3 py-2 bg-transparent text-subtle border border-border rounded-md cursor-pointer font-mono text-sm flex items-center gap-1.5 hover:text-text hover:border-border transition-colors duration-100 w-full justify-center"
								>
									<Download size={13} strokeWidth={1.75} />Download PNG
								</button>
								<button
									onclick={() => { qrVisible = false; }}
									class="text-xs text-subtle hover:text-subtle cursor-pointer bg-transparent border-none"
								>Hide</button>
							</div>
						{/if}
						{#if qrError}
							<p class="m-0 text-xs text-danger">{qrError}</p>
						{/if}
						<p class="m-0 text-xs text-subtle">QR code stays valid when you edit your form. Rotating your link will require a new QR code.</p>
					</div>

					<div class="h-px bg-border-deep"></div>

					<button
						onclick={() => { confirmRotate = true; }}
						disabled={publishing}
						class="px-3 py-2 bg-transparent text-subtle border border-border rounded-md cursor-pointer font-mono text-sm
							{publishing ? 'cursor-not-allowed opacity-60' : 'hover:text-text hover:border-border transition-colors duration-100'}"
					>Generate new link</button>
				{/if}
			</div>
	</div>

	<div class="h-px bg-border-deep"></div>

	<div class="p-5 flex flex-col gap-3.5">
		<!-- Locale switcher — mobile only (desktop: in toolbar) -->
		{#if store.schema.locales.length > 1}
			<div class="sm:hidden">
				<label class="block text-xs text-subtle mb-1.5 uppercase tracking-wider">Language</label>
				<div class="relative">
					<Languages size={13} strokeWidth={1.75} class="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none text-subtle" />
					<select
						value={store.activeLocale}
						onchange={(e) => store.setActiveLocale((e.target as HTMLSelectElement).value)}
						class="input-base w-full pl-8 pr-7 appearance-none cursor-pointer"
					>
						{#each store.schema.locales as locale}
							<option value={locale}>
								{new Intl.DisplayNames([locale, 'en'], { type: 'language' }).of(locale) ?? locale}
							</option>
						{/each}
					</select>
					<span class="absolute right-2 top-1/2 -translate-y-1/2 pointer-events-none flex text-subtle">
						<ChevronDown size={12} strokeWidth={1.75} />
					</span>
				</div>
			</div>
		{/if}

		<div class="flex items-center justify-between gap-3">
			<div>
				<p class="m-0 text-sm text-text">Custom completion message</p>
				<p class="m-0 text-xs text-subtle mt-0.5">Show a message after respondents submit.</p>
			</div>
			<button
				role="switch"
				aria-checked={completionMessageOpen}
				onclick={() => {
					if (completionMessageOpen) {
						completionMessagePending = false;
						store.updateTranslation(null, 'convoCompletionMessage', '');
					} else {
						completionMessagePending = true;
					}
				}}
				class="relative shrink-0 w-8 h-[18px] rounded-full transition-colors duration-150 border-none cursor-pointer
					{completionMessageOpen ? 'bg-primary' : 'bg-border-deep'}"
			>
				<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
					{completionMessageOpen ? 'translate-x-3.5' : 'translate-x-0'}"></span>
			</button>
		</div>
		{#if completionMessageOpen}
			<textarea
				value={store.activeTranslation?.convoCompletionMessage ?? ''}
				oninput={(e) => store.updateTranslation(null, 'convoCompletionMessage', (e.target as HTMLTextAreaElement).value)}
				placeholder="Your response has been submitted."
				rows={2}
				class="input-base"
			></textarea>
		{/if}

		{#if isConvo}
			<div class="flex items-center justify-between">
				<label class="text-sm text-text">Allow edit after submit</label>
				<input
					type="checkbox"
					checked={store.schema.convoAllowEdit ?? false}
					onchange={(e) => store.setConvoAllowEdit((e.target as HTMLInputElement).checked)}
				/>
			</div>
		{/if}

		<!-- Branding -->
		<div class="border-t border-border pt-4">
			<div class="flex items-center justify-between gap-3">
				<div>
					<p class="m-0 text-sm text-text">Show Confide watermark</p>
					<p class="m-0 text-xs text-subtle mt-0.5">
						{#if access.can('whitelabel')}
							Display the Confide logo at the bottom of the form.
						{:else}
							Upgrade to Pro to hide the Confide watermark.
						{/if}
					</p>
				</div>
				<button
					role="switch"
					aria-checked={store.schema.showWatermark !== false}
					onclick={() => access.can('whitelabel') && store.setShowWatermark(store.schema.showWatermark === false)}
					disabled={!access.can('whitelabel')}
					class="relative shrink-0 w-8 h-[18px] rounded-full transition-colors duration-150 border-none
						{access.can('whitelabel') ? 'cursor-pointer' : 'cursor-not-allowed opacity-40'}
						{store.schema.showWatermark !== false ? 'bg-primary' : 'bg-border-deep'}"
				>
					<span class="absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full transition-transform duration-150
						{store.schema.showWatermark !== false ? 'translate-x-3.5' : 'translate-x-0'}"></span>
				</button>
			</div>
		</div>

		<!-- Scheduling options -->
		<div class="border-t border-border pt-4 flex flex-col divide-y divide-border-deep">

			<!-- Close on date -->
			<div class="py-3 first:pt-0">
				<div class="flex items-center justify-between gap-3">
					<div>
						<p class="m-0 text-sm text-text">Close on date</p>
						<p class="m-0 text-xs text-subtle mt-0.5">Stop accepting responses after a date.</p>
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
							{closeOnDateOpen ? 'translate-x-3.5' : 'translate-x-0'}"></span>
					</button>
				</div>
				{#if closeOnDateOpen}
					<div class="mt-2.5">
						<input
							type="datetime-local"
							min={nowLocal()}
							value={toDatetimeLocal(store.expiresAt)}
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
						<p class="m-0 text-sm text-text">Limit total responses</p>
						<p class="m-0 text-xs text-subtle mt-0.5">Close after a set number of submissions.</p>
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
							{limitResponsesOpen ? 'translate-x-3.5' : 'translate-x-0'}"></span>
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
						<p class="m-0 text-sm text-text">Auto delete responses</p>
						<p class="m-0 text-xs text-subtle mt-0.5">Remove responses from our servers after a set period.</p>
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
							{autoDeleteOpen ? 'translate-x-3.5' : 'translate-x-0'}"></span>
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
								<span class="text-sm text-subtle shrink-0">days</span>
							</div>
						{:else if responseLifetimePolicy === 'burn'}
							<p class="m-0 text-xs text-subtle leading-relaxed">Responses are scheduled for deletion once you view them. They remain visible until the next cleanup pass.</p>
						{/if}
					</div>
				{/if}
			</div>

		</div>

		<!-- Languages -->
		<div class="border-t border-border pt-4 flex flex-col gap-3">
			<div>
				<p class="m-0 text-sm text-text">Support languages</p>
				<p class="m-0 text-xs text-subtle mt-0.5">Add languages to provide translated versions of this form.</p>
			</div>

			<!-- Added locales -->
			<div class="flex flex-col gap-1.5">
				{#each store.schema.locales as locale (locale)}
					<div class="flex items-center justify-between gap-2 px-3 py-2 bg-canvas border border-border-canvas rounded-md">
						<span class="text-sm text-text font-mono">{languageName(locale)}</span>
						{#if locale === store.schema.defaultLocale}
							<span class="text-xs text-subtle">default</span>
						{:else}
							<button
								onclick={() => { confirmRemoveLocale = locale; }}
								class="flex items-center justify-center w-5 h-5 rounded bg-transparent border-none cursor-pointer
									text-subtle hover:text-danger hover:bg-danger-dark transition-colors duration-100"
								aria-label="Remove {languageName(locale)}"
							>
								<X size={12} strokeWidth={2} />
							</button>
						{/if}
					</div>
				{/each}
			</div>

			<!-- Add language dropdown -->
			{#if availableLanguages.length > 0}
				<select
					onchange={handleAddLanguage}
					class="input-base text-subtle"
				>
					<option value="">Add a language…</option>
					{#each availableLanguages as lang (lang.code)}
						<option value={lang.code}>{lang.name}</option>
					{/each}
				</select>
			{/if}
		</div>

		{#if expirationSaving}
			<p class="m-0 text-xs text-subtle">Saving…</p>
		{:else if expirationError}
			<p class="m-0 text-xs text-danger">{expirationError}</p>
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

	<ConfirmDialog
		open={!!confirmRemoveLocale}
		title="Remove language?"
		description="This will permanently delete the {confirmRemoveLocale ? languageName(confirmRemoveLocale) : ''} translation and all its content. This cannot be undone."
		confirmLabel="Remove language"
		onconfirm={() => { if (confirmRemoveLocale) store.removeLocale(confirmRemoveLocale); confirmRemoveLocale = null; }}
		oncancel={() => { confirmRemoveLocale = null; }}
	/>

	<!-- Sticky publish button -->
	<div class="shrink-0 p-3 border-t border-border bg-base">
		{#if publishError}
			<p class="m-0 mb-2 text-xs text-danger">{publishError}</p>
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
