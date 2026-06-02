<script lang="ts">
import { Bold, Check, Italic, Link2, List, X } from "@lucide/svelte";

interface Props {
	value?: string;
	placeholder?: string;
	class?: string;
	style?: string;
	onfocus?: (e: FocusEvent) => void;
	onclick?: (e: MouseEvent) => void;
	onkeydown?: (e: KeyboardEvent) => void;
	onchange: (html: string) => void;
}

let {
	value = "",
	placeholder = "",
	class: cls = "",
	style = "",
	onfocus,
	onclick,
	onkeydown,
	onchange,
}: Props = $props();

let editorEl = $state<HTMLDivElement | undefined>();
let toolbar = $state<{ x: number; y: number } | null>(null);
let showLinkInput = $state(false);
let linkUrl = $state("");
let savedRange: Range | null = null;
let linkInputEl = $state<HTMLInputElement | undefined>();

// Update innerHTML when value changes externally (e.g. locale switch).
// Skip while focused — the browser may have inserted a <br> placeholder and resetting
// innerHTML here would wipe the cursor on the first click into an empty field.
$effect(() => {
	if (!editorEl) return;
	if (document.activeElement === editorEl) return;
	const next = value ?? "";
	if (editorEl.innerHTML !== next) {
		editorEl.innerHTML = next;
	}
});

$effect(() => {
	if (showLinkInput && linkInputEl) linkInputEl.focus();
});

function handleSelectionChange() {
	if (showLinkInput) return;
	const sel = window.getSelection();
	if (!sel || sel.isCollapsed || !editorEl) {
		toolbar = null;
		return;
	}
	const range = sel.getRangeAt(0);
	if (!editorEl.contains(range.commonAncestorContainer)) {
		toolbar = null;
		return;
	}
	const rect = range.getBoundingClientRect();
	toolbar = { x: rect.left + rect.width / 2, y: rect.top };
}

$effect(() => {
	document.addEventListener("selectionchange", handleSelectionChange);
	return () =>
		document.removeEventListener("selectionchange", handleSelectionChange);
});

function exec(cmd: string, arg?: string) {
	document.execCommand(cmd, false, arg);
	editorEl?.focus();
}

function handlePaste(e: ClipboardEvent) {
	e.preventDefault();
	const text = e.clipboardData?.getData("text/plain") ?? "";
	document.execCommand("insertText", false, text);
}

function handleInput() {
	const el = editorEl;
	if (!el) return;
	onchange(el.textContent?.trim() === "" ? "" : el.innerHTML);
}

function stopToolbarBlur(e: MouseEvent) {
	e.preventDefault(); // keep focus in editor
}

function openLink() {
	const sel = window.getSelection();
	if (sel && sel.rangeCount > 0) savedRange = sel.getRangeAt(0).cloneRange();
	showLinkInput = true;
	linkUrl = "";
}

function confirmLink() {
	if (!linkUrl.trim()) {
		cancelLink();
		return;
	}
	const url = /^https?:\/\//i.test(linkUrl) ? linkUrl : `https://${linkUrl}`;
	if (savedRange) {
		const sel = window.getSelection();
		sel?.removeAllRanges();
		sel?.addRange(savedRange);
	}
	exec("createLink", url);
	showLinkInput = false;
	savedRange = null;
	linkUrl = "";
	toolbar = null;
}

function cancelLink() {
	showLinkInput = false;
	savedRange = null;
	linkUrl = "";
	editorEl?.focus();
}

function handleLinkKeydown(e: KeyboardEvent) {
	if (e.key === "Enter") {
		e.preventDefault();
		confirmLink();
	}
	if (e.key === "Escape") cancelLink();
}
</script>

{#if toolbar}
	<div
		class="rich-toolbar"
		style="left: {toolbar.x}px; top: {toolbar.y}px;"
		onmousedown={stopToolbarBlur}
		role="toolbar"
		tabindex="-1"
		aria-label="Text formatting"
	>
		{#if showLinkInput}
			<input
				bind:this={linkInputEl}
				type="url"
				bind:value={linkUrl}
				placeholder="https://…"
				onkeydown={handleLinkKeydown}
				class="link-input"
			/>
			<button type="button" onclick={confirmLink} class="toolbar-btn confirm" aria-label="Apply link">
				<Check size={11} strokeWidth={2.5} />
			</button>
			<button type="button" onclick={cancelLink} class="toolbar-btn" aria-label="Cancel">
				<X size={11} strokeWidth={2} />
			</button>
		{:else}
			<button type="button" onclick={() => exec('bold')} class="toolbar-btn" title="Bold (Ctrl+B)" aria-label="Bold">
				<Bold size={12} strokeWidth={2.5} />
			</button>
			<button type="button" onclick={() => exec('italic')} class="toolbar-btn" title="Italic (Ctrl+I)" aria-label="Italic">
				<Italic size={12} strokeWidth={2.5} />
			</button>
			<button type="button" onclick={() => exec('insertUnorderedList')} class="toolbar-btn" title="Bullet list" aria-label="Bullet list">
				<List size={12} strokeWidth={2} />
			</button>
			<div class="toolbar-sep" role="separator"></div>
			<button type="button" onclick={openLink} class="toolbar-btn" title="Insert link" aria-label="Insert link">
				<Link2 size={12} strokeWidth={2} />
			</button>
		{/if}
	</div>
{/if}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	bind:this={editorEl}
	contenteditable="true"
	role="textbox"
	tabindex="0"
	aria-multiline="true"
	data-placeholder={placeholder}
	class="rich-editable {cls}"
	{style}
	oninput={handleInput}
	onpaste={handlePaste}
	onfocus={onfocus}
	onclick={onclick}
	onkeydown={onkeydown}
></div>

<style>
	.rich-toolbar {
		position: fixed;
		transform: translate(-50%, calc(-100% - 8px));
		z-index: 9999;
		display: inline-flex;
		align-items: center;
		gap: 1px;
		background: var(--color-canvas);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		padding: 3px 4px;
		box-shadow: 0 8px 32px var(--color-overlay);
		white-space: nowrap;
		pointer-events: auto;
	}

	/* Two-layer caret: outer matches border, inner matches surface */
	.rich-toolbar::after {
		content: '';
		position: absolute;
		top: 100%;
		left: 50%;
		transform: translateX(-50%);
		border: 5px solid transparent;
		border-top-color: var(--color-border);
	}

	.rich-toolbar::before {
		content: '';
		position: absolute;
		top: calc(100% - 1px);
		left: 50%;
		transform: translateX(-50%);
		border: 4px solid transparent;
		border-top-color: var(--color-canvas);
		z-index: 1;
	}

	.toolbar-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 24px;
		border: none;
		background: transparent;
		border-radius: 4px;
		cursor: pointer;
		color: var(--color-subtle);
		padding: 0;
		transition: background 0.1s, color 0.1s;
	}

	.toolbar-btn:hover {
		background: var(--color-canvas);
		color: var(--color-text);
	}

	.toolbar-btn.confirm {
		background: var(--color-primary);
		color: white;
	}

	.toolbar-btn.confirm:hover {
		background: var(--color-primary-hover);
	}

	.toolbar-sep {
		width: 1px;
		height: 14px;
		background: var(--color-border);
		margin: 0 2px;
		flex-shrink: 0;
	}

	.link-input {
		background: var(--color-canvas);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 2px 7px;
		font-size: 0.75rem;
		color: var(--color-text);
		outline: none;
		width: 170px;
		font-family: inherit;
	}

	.link-input:focus {
		border-color: var(--color-border);
	}

	.link-input::placeholder {
		color: var(--color-subtle);
	}

	.rich-editable {
		outline: none;
		cursor: text;
		caret-color: var(--color-text);
	}

	.rich-editable:empty::before {
		content: attr(data-placeholder);
		color: var(--color-border);
		pointer-events: none;
	}

	.rich-editable :global(a) {
		color: var(--color-primary);
		text-decoration: underline;
	}

	.rich-editable :global(strong),
	.rich-editable :global(b) {
		font-weight: bold;
	}

	.rich-editable :global(em),
	.rich-editable :global(i) {
		font-style: italic;
	}

	.rich-editable :global(ul) {
		list-style-type: '—';
		padding-left: 1.5rem;
		margin: 0.25rem 0;
	}

	.rich-editable :global(li) {
		margin: 0.4rem 0;
		padding-left: 0.35rem;
		line-height: 1.7;
	}
</style>
