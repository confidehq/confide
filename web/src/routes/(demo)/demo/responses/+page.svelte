<script lang="ts">
import {
	Check,
	Clock,
	Download,
	Link,
	Lock,
	Pencil,
	QrCode,
	RefreshCw,
	Search,
	ShieldCheck,
	Trash2,
} from "@lucide/svelte";
import {
	DEMO_FORM_NAMES,
	DEMO_FORMS,
	DEMO_FORM_SCHEMA,
	DEMO_RESPONSES,
	type DemoResponse,
} from "$lib/demo/data";
import type {
	CheckboxesConfig,
	DropdownConfig,
	MultipleChoiceConfig,
} from "$lib/types/builder";

const schema = DEMO_FORM_SCHEMA;
const translation = schema.translations["en"];
const form = DEMO_FORMS[0];
const formName = DEMO_FORM_NAMES["demo-form-1"];
const FAKE_SHARE_URL =
	"https://confide.app/f/demo-a1b2c3d4#rk=xK9mP2nQvTsLhFjR";

const answerFields = schema.fields.filter(
	(f) => !["accent", "section_break", "heading", "accordion"].includes(f.type),
);

// ── Response list state ───────────────────────────────────────────────────
let selectedId = $state<string | null>(null);
let searchQuery = $state("");
let activeTab = $state<"All" | "Unread">("All");
let viewedIds = $state(new Set<string>());

const selected = $derived(
	selectedId ? DEMO_RESPONSES.find((r) => r.id === selectedId) ?? null : null,
);

const selectedIdx = $derived(
	selectedId ? DEMO_RESPONSES.findIndex((r) => r.id === selectedId) : -1,
);

const filteredResponses = $derived(
	DEMO_RESPONSES.filter((r) => {
		if (activeTab === "Unread" && viewedIds.has(r.id)) return false;
		const q = searchQuery.trim().toLowerCase();
		if (!q) return true;
		return Object.values(r.answers).some((v) =>
			String(Array.isArray(v) ? v.join(" ") : v)
				.toLowerCase()
				.includes(q),
		);
	}),
);

function selectResponse(id: string) {
	selectedId = id;
	viewedIds = new Set([...viewedIds, id]);
}

// ── Settings state ────────────────────────────────────────────────────────
let closeOnDate = $state(false);
let limitResponses = $state(false);
let autoDelete = $state(false);
let emailForwarding = $state(false);
let showWatermark = $state(false);
let qrVisible = $state(false);
let copied = $state(false);
let settingsSaved = $state(false);

function copyLink() {
	navigator.clipboard?.writeText(FAKE_SHARE_URL).catch(() => {});
	copied = true;
	setTimeout(() => {
		copied = false;
	}, 2000);
}

function saveSettings() {
	settingsSaved = true;
	setTimeout(() => {
		settingsSaved = false;
	}, 2500);
}

// ── Avatar helpers ────────────────────────────────────────────────────────
const AVATAR_COLORS = [
	{ bg: "#1D2739", color: "#7191CA" },
	{ bg: "#1D391E", color: "#58AE5B" },
	{ bg: "#39341D", color: "#B7A449" },
	{ bg: "#391D1D", color: "#C37D7D" },
	{ bg: "#2D1F3D", color: "#A78BFA" },
];

function avatarColor(idx: number) {
	return AVATAR_COLORS[idx % AVATAR_COLORS.length];
}

function getPreview(r: DemoResponse): string {
	const desc = r.answers["description"];
	if (desc) return String(desc).slice(0, 80);
	return "";
}

function formatShort(iso: string): string {
	return new Date(iso).toLocaleString(undefined, {
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
}

function formatLong(iso: string): string {
	return new Date(iso).toLocaleString(undefined, {
		year: "numeric",
		month: "long",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
		second: "2-digit",
	});
}

function renderAnswer(
	fieldId: string,
	value: string | string[],
): string {
	const field = schema.fields.find((f) => f.id === fieldId);
	if (!field || value === null || value === undefined || value === "")
		return "—";

	const t = translation.fields[fieldId];

	if (field.type === "multiple_choice") {
		const cfg = field.config as MultipleChoiceConfig;
		const idx = cfg.options.findIndex((o) => o.id === String(value));
		return t?.options?.[idx] ?? String(value);
	}
	if (field.type === "checkboxes") {
		const cfg = field.config as CheckboxesConfig;
		return (value as string[])
			.map((id) => {
				const idx = cfg.options.findIndex((o) => o.id === id);
				return t?.options?.[idx] ?? id;
			})
			.join(", ");
	}
	if (field.type === "dropdown") {
		const cfg = field.config as DropdownConfig;
		const idx = cfg.options.findIndex((o) => o.id === String(value));
		return t?.options?.[idx] ?? String(value);
	}
	return Array.isArray(value) ? value.join(", ") : String(value);
}
</script>

<svelte:head>
	<title>Anonymous Incident Report — Responses Demo</title>
</svelte:head>

<div class="flex flex-col font-mono h-full min-h-0 overflow-hidden">

	<!-- Top bar -->
	<div class="flex items-center gap-2 sm:gap-3 px-4 sm:px-6 h-10 border-b border-border-canvas shrink-0 overflow-hidden bg-canvas">
		<nav class="flex items-center gap-1.5 min-w-0 overflow-hidden text-sm">
			<a href="/demo/workspace" class="text-subtle hover:text-text transition-colors duration-100 no-underline whitespace-nowrap">Workspace</a>
			<span class="text-muted">/</span>
			<button
				class="text-subtle hover:text-text transition-colors duration-100 bg-transparent border-none cursor-pointer font-mono text-sm p-0 whitespace-nowrap truncate"
				onclick={() => { selectedId = null; }}
			>{formName}</button>
			{#if selected}
				<span class="text-muted">/</span>
				<span class="text-subtle text-sm truncate font-mono">{selected.id.slice(0, 8)}…</span>
			{/if}
		</nav>
		<div class="flex-1 min-w-0"></div>
		<a
			href="/demo/dashboard"
			class="shrink-0 font-mono text-sm text-subtle no-underline px-3 py-1.5 border border-border-canvas rounded whitespace-nowrap
				hover:text-text hover:border-border transition-colors duration-100 flex items-center gap-1.5"
		>
			<Pencil size={11} strokeWidth={2} />
			<span class="hidden sm:inline">View Dashboard</span>
			<span class="sm:hidden">Dashboard</span>
		</a>
	</div>

	<!-- Two-panel shell -->
	<div class="flex flex-1 min-h-0">

		<!-- Left panel: response list -->
		<div class="hidden sm:flex sm:w-96 shrink-0 flex-col border-r border-border-canvas min-h-0 bg-canvas">

			<!-- List header -->
			<div class="px-3 pt-3 pb-2 shrink-0">
				<div class="flex items-center justify-between gap-2 mb-2.5">
					<div class="flex items-center gap-2">
						<span class="text-sm font-bold tracking-[0.12em] uppercase text-muted">Responses</span>
						<span class="bg-surface text-subtle text-sm font-bold px-1.5 py-0.5 rounded-full border border-border-canvas">{form.responseCount}</span>
					</div>
					<button
						title="Refresh"
						class="flex items-center justify-center w-6 h-6 bg-transparent border-none rounded cursor-not-allowed text-muted opacity-40"
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

			<!-- Response items -->
			<div class="flex-1 overflow-y-auto overflow-x-hidden">
				{#each filteredResponses as resp, i (resp.id)}
					{@const globalIdx = DEMO_RESPONSES.findIndex(r => r.id === resp.id)}
					{@const ac = avatarColor(globalIdx)}
					{@const preview = getPreview(resp)}
					<button
						onclick={() => selectResponse(resp.id)}
						class="block w-full px-3 py-2.5 text-left bg-transparent border-none border-b border-border-canvas cursor-pointer transition-[background] duration-100 hover:bg-surface
							{selectedId === resp.id ? 'bg-highlight-low border-l-2 border-l-info-light !pl-2.5' : ''}"
					>
						<div class="flex items-start gap-2.5">
							<div
								class="w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold shrink-0 mt-0.5"
								style="background:{ac.bg};color:{ac.color}"
							>{globalIdx + 1}</div>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-1.5">
									<span class="text-xs font-semibold truncate {selectedId === resp.id ? 'text-text' : 'text-subtle'}">
										{#if !viewedIds.has(resp.id)}<span class="inline-block w-1.5 h-1.5 rounded-full bg-info-light align-middle mr-1 -mt-px"></span>{/if}Response #{globalIdx + 1}
									</span>
									<span class="text-sm text-muted shrink-0">{formatShort(resp.receivedAt)}</span>
								</div>
								{#if preview}
									<p class="m-0 mt-0.5 text-sm text-muted leading-snug overflow-hidden" style="display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical">{preview}…</p>
								{/if}
							</div>
						</div>
					</button>
				{/each}

				{#if filteredResponses.length === 0}
					<div class="px-4 py-8 text-center text-xs text-muted">No responses match your filter.</div>
				{/if}
			</div>
		</div>

		<!-- Right panel -->
		<div class="flex flex-1 min-w-0 flex-col min-h-0">

			{#if selectedId === null}
				<!-- ── Overview / Settings ──────────────────────────────────── -->
				<div class="flex-1 overflow-y-auto">

					<!-- Form hero -->
					<div class="px-8 pt-8 pb-7 border-b border-border-canvas">
						<div class="max-w-4xl">
							<div class="flex items-start justify-between gap-4 mb-5">
								<h1 class="m-0 text-2xl text-text font-semibold leading-tight">{formName}</h1>
								<!-- Status pill -->
								<div class="flex items-center gap-1.5 px-3 py-1 rounded-full shrink-0 border bg-success-dark border-success-light/30">
									<span class="w-1.5 h-1.5 rounded-full shrink-0 bg-success-light animate-pulse"></span>
									<span class="text-sm font-bold uppercase tracking-wider text-success-light">open</span>
								</div>
							</div>

							<!-- Stats row -->
							<div class="flex gap-0">
								<div class="flex flex-col gap-0.5 pr-6">
									<span class="text-2xl font-semibold tabular-nums text-text">{form.responseCount}</span>
									<span class="text-sm font-bold uppercase tracking-wider text-muted">Total</span>
								</div>
								<div class="flex flex-col gap-0.5 px-6 border-l border-border-canvas">
									<span class="text-2xl font-semibold tabular-nums text-info-light">5</span>
									<span class="text-sm font-bold uppercase tracking-wider text-muted">Unread</span>
								</div>
								<div class="flex flex-col gap-0.5 px-6 border-l border-border-canvas">
									<span class="text-2xl font-semibold tabular-nums text-text">{answerFields.length}</span>
									<span class="text-sm font-bold uppercase tracking-wider text-muted">Questions</span>
								</div>
							</div>

							<!-- Actions -->
							<div class="flex items-center gap-2 mt-5">
								<button
									class="px-3 py-1.5 text-sm font-mono border rounded cursor-pointer transition-colors duration-100
										bg-transparent text-danger border-danger-light hover:bg-danger-dark hover:border-danger-dark hover:text-white"
								>Close form</button>
							</div>
						</div>
					</div>

					<!-- Settings body -->
					<div class="px-8 py-7 max-w-4xl flex flex-col gap-9">

						<!-- Share section -->
						<section class="flex flex-col gap-3">
							<h2 class="m-0 font-bold tracking-[0.12em] uppercase text-muted mb-1">Share</h2>

							<div class="rounded-lg border border-border-canvas overflow-hidden">
								<!-- Link row -->
								<div class="flex items-center gap-3 px-4 py-3">
									<div class="w-7 h-7 rounded-md bg-surface border border-border-canvas flex items-center justify-center shrink-0">
										<Link size={12} strokeWidth={2} class="text-subtle" />
									</div>
									<div class="flex-1 min-w-0">
										<p class="m-0 font-semibold text-text mb-0.5">Direct link</p>
										<p class="m-0 text-sm text-muted font-mono truncate">{FAKE_SHARE_URL}</p>
									</div>
									<button
										onclick={copyLink}
										class="shrink-0 px-2.5 py-1.5 rounded font-mono text-sm transition-[background,color] duration-150 grid items-center
											{copied ? 'bg-success-light text-success cursor-default' : 'bg-primary text-white hover:bg-primary-hover cursor-pointer'}"
									>
										<span class="col-start-1 row-start-1 flex items-center justify-center gap-1 {copied ? '' : 'invisible'}">
											<Check size={10} strokeWidth={2.5} />Copied
										</span>
										<span class="col-start-1 row-start-1 flex items-center justify-center {copied ? 'invisible' : ''}">
											Copy secure link
										</span>
									</button>
								</div>

								<!-- QR row -->
								<div class="px-4 py-3 border-t border-border-canvas">
									{#if !qrVisible}
										<button
											onclick={() => { qrVisible = true; }}
											class="flex items-center gap-1.5 px-3 py-1.5 text-sm text-muted font-mono border border-border-canvas rounded bg-transparent cursor-pointer transition-colors duration-100 hover:text-text hover:border-border"
										><QrCode size={11} strokeWidth={2} />Get QR code</button>
									{:else}
										<div class="flex flex-col items-start gap-3">
											<div class="w-32 h-32 border border-border-canvas rounded flex items-center justify-center bg-surface">
												<QrCode size={72} strokeWidth={1} class="text-subtle opacity-40" />
											</div>
											<div class="flex items-center gap-2">
												<button
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
								</div>

								<!-- Rotate row -->
								<div class="px-4 py-3 border-t border-border-canvas">
									<button
										class="px-3 py-1.5 text-sm font-mono border rounded cursor-pointer text-muted border-border-canvas bg-transparent hover:text-text hover:border-border transition-colors duration-100"
									>Generate new link</button>
									<p class="m-0 mt-2 text-xs text-muted leading-relaxed">QR code stays valid when you edit your form. Rotating your link will require a new QR code.</p>
								</div>
							</div>
						</section>

						<!-- Form settings section -->
						<section class="flex flex-col gap-3">
							<h2 class="m-0 font-bold tracking-[0.12em] uppercase text-muted mb-1">Form settings</h2>
							<div class="rounded-lg border border-border-canvas overflow-hidden">

								<!-- Close on date -->
								<div class="px-4 py-3.5 border-b border-border-canvas">
									<div class="flex items-center justify-between gap-4">
										<div class="min-w-0">
											<p class="m-0 text-text font-medium">Close on date</p>
											<p class="m-0 text-sm text-muted mt-0.5">Stop accepting responses after a specific date.</p>
										</div>
										<button
											role="switch"
											aria-label="Close on date"
											aria-checked={closeOnDate}
											onclick={() => { closeOnDate = !closeOnDate; }}
											class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
												{closeOnDate ? 'bg-primary' : 'bg-surface'}"
										>
											<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
												{closeOnDate ? 'translate-x-5' : 'translate-x-0'}"></span>
										</button>
									</div>
									{#if closeOnDate}
										<div class="mt-3">
											<input type="datetime-local" class="input-base" />
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
											aria-checked={limitResponses}
											onclick={() => { limitResponses = !limitResponses; }}
											class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
												{limitResponses ? 'bg-primary' : 'bg-surface'}"
										>
											<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
												{limitResponses ? 'translate-x-5' : 'translate-x-0'}"></span>
										</button>
									</div>
									{#if limitResponses}
										<div class="mt-3">
											<input type="number" min="1" placeholder="e.g. 100" class="input-base" />
										</div>
									{/if}
								</div>

								<!-- Auto-delete -->
								<div class="px-4 py-3.5 border-b border-border-canvas">
									<div class="flex items-center justify-between gap-4">
										<div class="min-w-0">
											<p class="m-0 text-text font-medium">Auto-delete responses</p>
											<p class="m-0 text-sm text-muted mt-0.5">Remove responses from our servers after a set period.</p>
										</div>
										<button
											role="switch"
											aria-label="Auto-delete responses"
											aria-checked={autoDelete}
											onclick={() => { autoDelete = !autoDelete; }}
											class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
												{autoDelete ? 'bg-primary' : 'bg-surface'}"
										>
											<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
												{autoDelete ? 'translate-x-5' : 'translate-x-0'}"></span>
										</button>
									</div>
									{#if autoDelete}
										<div class="mt-3">
											<select class="input-base">
												<option value="burn">Burn after reading</option>
												<option value="ttl">Delete after a set period</option>
											</select>
										</div>
									{/if}
								</div>

								<!-- Email forwarding -->
								<div class="px-4 py-3.5 border-b border-border-canvas">
									<div class="flex items-center justify-between gap-4">
										<div class="min-w-0">
											<p class="m-0 text-text font-medium">Email forwarding</p>
											<p class="m-0 text-sm text-muted mt-0.5">Forward encrypted responses to an email address via PGP.</p>
										</div>
										<button
											role="switch"
											aria-label="Email forwarding"
											aria-checked={emailForwarding}
											onclick={() => { emailForwarding = !emailForwarding; }}
											class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
												{emailForwarding ? 'bg-primary' : 'bg-surface'}"
										>
											<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
												{emailForwarding ? 'translate-x-5' : 'translate-x-0'}"></span>
										</button>
									</div>
									{#if emailForwarding}
										<div class="mt-3">
											<div class="border border-border-canvas rounded overflow-hidden">
												<div class="flex items-center border-b border-border-canvas">
													<span class="w-20 shrink-0 px-3 py-2.5 text-sm text-muted border-r border-border-canvas">To</span>
													<input type="email" placeholder="recipient@example.com"
														class="flex-1 min-w-0 px-3 py-2.5 bg-transparent border-none outline-none text-sm text-text placeholder:text-muted font-mono" />
												</div>
												<div class="flex items-center border-b border-border-canvas">
													<span class="w-20 shrink-0 px-3 py-2.5 text-sm text-muted border-r border-border-canvas">From</span>
													<span class="flex-1 min-w-0 px-3 py-2.5 text-sm text-muted font-mono truncate">Confide Forms &lt;notifications@confide.app&gt;</span>
												</div>
												<div class="flex items-center">
													<span class="w-20 shrink-0 px-3 py-2.5 text-sm text-muted border-r border-border-canvas">Subject</span>
													<input type="text" placeholder="New Confide Form submission"
														class="flex-1 min-w-0 px-3 py-2.5 bg-transparent border-none outline-none text-sm text-text placeholder:text-muted font-mono" />
												</div>
											</div>
											<p class="m-0 mt-1.5 text-xs text-muted">To, From, and Subject are stored unencrypted on our servers.</p>
											<div class="mt-3">
												<p class="m-0 mb-1.5 text-xs text-subtle uppercase tracking-[0.08em]">PGP Public Key</p>
												<div class="border border-border-canvas rounded overflow-hidden">
													<textarea
														placeholder="-----BEGIN PGP PUBLIC KEY BLOCK-----"
														rows={4}
														class="w-full px-3 py-2.5 bg-transparent border-none outline-none text-xs text-text placeholder:text-muted font-mono resize-y block"
													></textarea>
												</div>
												<p class="m-0 mt-1.5 text-xs text-muted">Paste your PGP public key block. In Proton Mail: Settings → Encryption &amp; keys → Export public key.</p>
											</div>
										</div>
									{/if}
								</div>

								<!-- Watermark -->
								<div class="px-4 py-3.5">
									<div class="flex items-center justify-between gap-4">
										<div class="min-w-0">
											<p class="m-0 text-text font-medium">Show Confide watermark</p>
											<p class="m-0 text-sm text-muted mt-0.5">Display the Confide logo at the bottom of the form.</p>
										</div>
										<button
											role="switch"
											aria-label="Show Confide watermark"
											aria-checked={showWatermark}
											onclick={() => { showWatermark = !showWatermark; }}
											class="relative shrink-0 w-11 h-6 rounded-full transition-colors duration-150 border-none cursor-pointer
												{showWatermark ? 'bg-primary' : 'bg-surface'}"
										>
											<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform duration-150 shadow-sm
												{showWatermark ? 'translate-x-5' : 'translate-x-0'}"></span>
										</button>
									</div>
								</div>
							</div>

							<!-- Save button -->
							<div class="flex items-center gap-3 mt-1">
								<button
									onclick={saveSettings}
									class="px-4 py-2 border rounded font-mono cursor-pointer transition-colors duration-100
										bg-transparent text-text border-border hover:bg-surface hover:border-border"
								>Save settings</button>
								{#if settingsSaved}
									<span class="text-success flex items-center gap-1.5 text-sm">
										<Check size={11} strokeWidth={2.5} />Saved
									</span>
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
										Permanently deletes the form and all {form.responseCount} responses. Cannot be undone.
									</p>
								</div>
								<button
									class="sm:shrink-0 flex items-center gap-1.5 px-4 py-2 bg-transparent text-danger-light border border-danger-light/50 rounded cursor-pointer font-mono text-sm
										hover:bg-danger-dark hover:border-danger-light transition-colors duration-100"
								>
									<Trash2 size={13} strokeWidth={1.75} />Delete form
								</button>
							</div>
						</section>

					</div>
				</div>

			{:else}
				<!-- ── Response detail ──────────────────────────────────────── -->
				{#if selected}
					{@const ac = avatarColor(selectedIdx)}

					<!-- Detail header -->
					<div class="px-6 py-5 border-b border-border-canvas shrink-0 bg-canvas">
						<div class="flex items-start gap-3.5">
							<!-- Avatar -->
							<div
								class="w-10 h-10 rounded-full flex items-center justify-center font-bold text-base shrink-0"
								style="background:{ac.bg};color:{ac.color}"
							>{selectedIdx + 1}</div>
							<!-- Name + ID -->
							<div class="flex-1 min-w-0">
								<p class="m-0 font-semibold text-text">Response #{selectedIdx + 1}</p>
								<p class="m-0 text-muted font-mono text-sm mt-0.5 truncate">{selected.id}</p>
							</div>
							<!-- Delete action -->
							<button
								class="px-3 py-1.5 bg-transparent text-danger-light border border-danger-light/50 rounded cursor-pointer font-mono text-xs transition-colors duration-100 hover:bg-danger-dark hover:border-danger-light shrink-0"
							>Delete</button>
						</div>

						<!-- Meta row -->
						<div class="flex gap-4 mt-4 flex-wrap items-center">
							<div class="flex items-center gap-1.5 text-muted text-sm">
								<Clock size={11} strokeWidth={2} />
								{formatLong(selected.receivedAt)}
							</div>
							<div class="flex items-center gap-1.5 text-success text-sm">
								<Lock size={11} strokeWidth={2} />
								End-to-end encrypted
							</div>
							<span class="text-muted bg-surface border border-border-canvas rounded px-1.5 py-0.5 font-mono text-xs">en</span>
						</div>

						<button
							class="sm:hidden mt-3 text-xs text-muted hover:text-subtle bg-transparent border-none cursor-pointer p-0 font-mono transition-colors duration-100"
							onclick={() => { selectedId = null; }}
						>← All responses</button>
					</div>

					<!-- Answer cards -->
					<div class="flex-1 overflow-y-auto px-6 py-6">
						<div class="flex flex-col gap-5 max-w-2xl">
							{#each answerFields as field (field.id)}
								{@const value = selected.answers[field.id]}
								{@const t = translation.fields[field.id]}
								{@const rendered = renderAnswer(field.id, value as string | string[])}
								<div>
									<p class="font-bold tracking-[0.1em] uppercase text-muted m-0 mb-2 text-sm">
										{t?.label ?? field.id}{#if field.required}<span class="text-danger ml-0.5">*</span>{/if}
									</p>
									<div class="bg-canvas border border-border-canvas rounded-lg px-4 py-3">
										<p class="text-sm text-text m-0 leading-relaxed whitespace-pre-wrap break-words
											{rendered === '—' ? 'text-muted italic' : ''}">
											{rendered}
										</p>
									</div>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			{/if}

		</div>
	</div>
</div>
