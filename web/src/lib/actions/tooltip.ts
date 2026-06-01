export function tooltip(node: HTMLElement, text: string | null) {
	let el: HTMLDivElement | null = null;
	let timer: ReturnType<typeof setTimeout>;

	function show() {
		if (!text) return;
		timer = setTimeout(() => {
			el = document.createElement("div");
			el.textContent = text;
			el.className =
				"fixed z-[9999] px-2 py-1 text-xs font-mono rounded pointer-events-none whitespace-nowrap";
			el.style.cssText += [
				"background: var(--color-surface-card)",
				"border: 1px solid var(--color-border)",
				"color: var(--color-text-body)",
			].join("; ");
			document.body.appendChild(el);

			const rect = node.getBoundingClientRect();
			el.style.top = `${rect.top + rect.height / 2 - el.offsetHeight / 2}px`;
			el.style.left = `${rect.right + 8}px`;
		}, 120);
	}

	function hide() {
		clearTimeout(timer);
		el?.remove();
		el = null;
	}

	node.addEventListener("mouseenter", show);
	node.addEventListener("mouseleave", hide);
	node.addEventListener("click", hide);

	return {
		update(newText: string | null) {
			text = newText;
			if (!text) hide();
		},
		destroy() {
			clearTimeout(timer);
			el?.remove();
			node.removeEventListener("mouseenter", show);
			node.removeEventListener("mouseleave", hide);
			node.removeEventListener("click", hide);
		},
	};
}
