<script lang="ts">
	import { Bold, Italic, List, Link2, Check, X } from '@lucide/svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		minRows?: number;
		onchange: (v: string) => void;
	}

	const { value = '', placeholder = '', minRows = 4, onchange }: Props = $props();

	let editorEl = $state<HTMLDivElement | undefined>();
	let focused = $state(false);
	let showLinkBar = $state(false);
	let linkUrl = $state('');
	let savedRange: Range | null = null;
	let initialized = false;
	let linkInputEl = $state<HTMLInputElement | undefined>();

	$effect(() => {
		if (showLinkBar && linkInputEl) {
			linkInputEl.focus();
		}
	});

	$effect(() => {
		if (editorEl && !initialized) {
			editorEl.innerHTML = value ?? '';
			initialized = true;
		}
	});

	function exec(cmd: string, arg?: string) {
		document.execCommand(cmd, false, arg);
		editorEl?.focus();
	}

	function handleInput() {
		onchange(editorEl?.innerHTML ?? '');
	}

	function onToolbarMousedown(e: MouseEvent) {
		// Prevent toolbar clicks from stealing focus from the editor
		e.preventDefault();
	}

	function openLinkBar() {
		const sel = window.getSelection();
		if (sel && sel.rangeCount > 0) {
			savedRange = sel.getRangeAt(0).cloneRange();
		}
		showLinkBar = true;
		linkUrl = '';
	}

	function confirmLink() {
		if (!linkUrl.trim()) { showLinkBar = false; return; }
		const url = /^https?:\/\//i.test(linkUrl) ? linkUrl : `https://${linkUrl}`;
		if (savedRange) {
			const sel = window.getSelection();
			sel?.removeAllRanges();
			sel?.addRange(savedRange);
		}
		exec('createLink', url);
		showLinkBar = false;
		savedRange = null;
		linkUrl = '';
	}

	function cancelLink() {
		showLinkBar = false;
		savedRange = null;
		linkUrl = '';
		editorEl?.focus();
	}

	function handleLinkKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') { e.preventDefault(); confirmLink(); }
		if (e.key === 'Escape') { cancelLink(); }
	}
</script>

<div class="rich-editor" class:focused>
	<div class="toolbar" onmousedown={onToolbarMousedown} role="toolbar" tabindex="-1" aria-label="Text formatting">
		<button type="button" onclick={() => exec('bold')} title="Bold (Ctrl+B)" aria-label="Bold">
			<Bold size={13} strokeWidth={2.5} />
		</button>
		<button type="button" onclick={() => exec('italic')} title="Italic (Ctrl+I)" aria-label="Italic">
			<Italic size={13} strokeWidth={2.5} />
		</button>
		<button type="button" onclick={() => exec('insertUnorderedList')} title="Bullet list" aria-label="Bullet list">
			<List size={13} strokeWidth={2} />
		</button>
		<div class="toolbar-sep" role="separator"></div>
		<button type="button" onclick={openLinkBar} title="Insert link" aria-label="Insert link">
			<Link2 size={13} strokeWidth={2} />
		</button>
	</div>

	{#if showLinkBar}
		<div class="link-bar">
			<input
				bind:this={linkInputEl}
				type="url"
				bind:value={linkUrl}
				placeholder="https://example.com"
				onkeydown={handleLinkKeydown}
				class="link-input"
			/>
			<button type="button" onclick={confirmLink} class="link-confirm" aria-label="Confirm link">
				<Check size={12} strokeWidth={2.5} />
			</button>
			<button type="button" onclick={cancelLink} class="link-cancel" aria-label="Cancel">
				<X size={12} strokeWidth={2.5} />
			</button>
		</div>
	{/if}

	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		bind:this={editorEl}
		contenteditable="true"
		role="textbox"
		aria-multiline="true"
		aria-placeholder={placeholder}
		data-placeholder={placeholder}
		class="editor-content"
		style="min-height: {minRows * 1.625}rem"
		oninput={handleInput}
		onfocus={() => (focused = true)}
		onblur={() => (focused = false)}
	></div>
</div>

<style>
	.rich-editor {
		width: 100%;
		border: 1.5px solid var(--color-form-border);
		border-radius: 6px;
		box-sizing: border-box;
		font-family: inherit;
		background: white;
	}

	.rich-editor.focused {
		border-color: var(--color-form-border-focus);
	}

	.toolbar {
		display: flex;
		align-items: center;
		gap: 1px;
		padding: 4px 6px;
		border-bottom: 1px solid var(--color-form-border-light, #e5e7eb);
		background: var(--color-form-surface);
		border-radius: 5px 5px 0 0;
	}

	.toolbar button {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border: none;
		background: transparent;
		border-radius: 4px;
		cursor: pointer;
		color: var(--color-form-muted);
		padding: 0;
		transition: background 0.1s, color 0.1s;
	}

	.toolbar button:hover {
		background: var(--color-form-border-light, #e5e7eb);
		color: var(--color-form-text);
	}

	.toolbar-sep {
		width: 1px;
		height: 14px;
		background: var(--color-form-border-light, #e5e7eb);
		margin: 0 4px;
		flex-shrink: 0;
	}

	.link-bar {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 5px 8px;
		border-bottom: 1px solid var(--color-form-border-light, #e5e7eb);
		background: var(--color-form-surface);
	}

	.link-input {
		flex: 1;
		border: 1px solid var(--color-form-border);
		border-radius: 4px;
		padding: 3px 8px;
		font-size: 0.8125rem;
		font-family: inherit;
		outline: none;
		min-width: 0;
	}

	.link-input:focus {
		border-color: var(--color-form-border-focus);
	}

	.link-confirm,
	.link-cancel {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		padding: 0;
		flex-shrink: 0;
	}

	.link-confirm {
		background: var(--color-form-primary);
		color: white;
	}

	.link-cancel {
		background: var(--color-form-border-light, #e5e7eb);
		color: var(--color-form-muted);
	}

	.editor-content {
		padding: 8px 12px;
		outline: none;
		font-size: 1rem;
		line-height: 1.5;
		color: var(--color-form-text);
		border-radius: 0 0 5px 5px;
		word-break: break-word;
		overflow-wrap: break-word;
	}

	.editor-content:empty::before {
		content: attr(data-placeholder);
		color: var(--color-form-muted-light, #9ca3af);
		pointer-events: none;
	}

	.editor-content :global(a) {
		color: var(--color-form-primary);
		text-decoration: underline;
	}

	.editor-content :global(ul) {
		list-style-type: '—';
		padding-left: 1.5rem;
		margin: 0.25rem 0;
	}

	.editor-content :global(li) {
		margin: 0.4rem 0;
		padding-left: 0.35rem;
		line-height: 1.7;
	}
</style>
