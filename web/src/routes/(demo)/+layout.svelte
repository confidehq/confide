<script lang="ts">
import { page } from "$app/stores";
import { goto } from "$app/navigation";
import { onMount } from "svelte";
import { ChevronUp, ChevronDown } from "@lucide/svelte";
import { getAppConfig } from "$lib/config";
import type { Snippet } from "svelte";

let { children }: { children: Snippet } = $props();

let demoEnabled = $state(false);

onMount(async () => {
	const cfg = await getAppConfig();
	if (!cfg.demo) {
		goto("/");
		return;
	}
	demoEnabled = true;
});

const steps = [
	{ label: "Intake Form", path: "/demo/form" },
	{ label: "Submitted", path: "/demo/submitted" },
	{ label: "Workspace", path: "/demo/workspace" },
	{ label: "Responses", path: "/demo/responses" },
	{ label: "Dashboard", path: "/demo/dashboard" },
];

const currentPath = $derived($page.url.pathname);

const currentStep = $derived(
	steps.findIndex((s) => s.path === currentPath),
);

const isFormPage = $derived(
	currentPath === "/demo/form" || currentPath === "/demo/submitted",
);

let bannerVisible = $state(true);
</script>

{#if demoEnabled}
<div class="demo-shell" class:light={isFormPage}>
	{#if bannerVisible}
		<!-- Top nav -->
		<header class="demo-nav">
			<a href="/demo" class="demo-logo">
				<svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M12 2L4 6v6c0 5.25 3.5 10.15 8 11.35C16.5 22.15 20 17.25 20 12V6l-8-4z" stroke="currentColor" stroke-width="1.75" stroke-linejoin="round"/>
				</svg>
				<span>Confide</span>
			</a>
			<div class="demo-nav-right">
				<a href="/signup" class="demo-signup">Sign up free</a>
				<button class="demo-toggle" onclick={() => bannerVisible = false} title="Hide demo controls">
					<ChevronUp size={14} strokeWidth={2} />
					<span>Hide</span>
				</button>
			</div>
		</header>

		<!-- Demo banner -->
		<div class="demo-banner">
			<span class="demo-badge">Demo</span>
			<span class="demo-banner-text">No data is submitted or stored — this is a read-only walkthrough.</span>
		</div>

		<!-- Step indicator (only when on a flow step) -->
		{#if currentStep >= 0}
			<div class="demo-steps">
				{#each steps as step, i}
					<a
						href={step.path}
						class="demo-step"
						class:active={i === currentStep}
						class:done={i < currentStep}
					>
						<span class="demo-step-num">{i + 1}</span>
						<span class="demo-step-label">{step.label}</span>
					</a>
					{#if i < steps.length - 1}
						<span class="demo-step-arrow">→</span>
					{/if}
				{/each}
			</div>
		{/if}
	{:else}
		<button class="demo-toggle demo-toggle--collapsed" onclick={() => bannerVisible = true} title="Show demo controls">
			<ChevronDown size={14} strokeWidth={2} />
			<span>Show controls</span>
		</button>
	{/if}

	<!-- Page content -->
	<main class="demo-content">
		{@render children()}
	</main>
</div>
{/if}

<style>
	:global(html), :global(body) {
		margin: 0;
		padding: 0;
		font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, monospace;
	}

	.demo-shell {
		height: 100vh;
		overflow: hidden;
		background: var(--color-base);
		color: var(--color-text);
		display: flex;
		flex-direction: column;
		position: relative;
	}

	.demo-shell.light {
		background: #ffffff;
		color: #111;
	}

	/* Nav */
	.demo-nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.75rem 1.5rem;
		border-bottom: 1px solid var(--color-border-canvas);
		background: var(--color-base);
		position: sticky;
		top: 0;
		z-index: 50;
	}

	.demo-shell.light .demo-nav {
		background: #fff;
		border-bottom-color: #e5e7eb;
	}

	.demo-logo {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		text-decoration: none;
		color: var(--color-text);
		font-size: 0.9375rem;
		font-weight: 600;
	}

	.demo-shell.light .demo-logo {
		color: #111;
	}

	.demo-nav-right {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.demo-signup {
		font-size: 0.875rem;
		color: var(--color-primary);
		text-decoration: none;
		padding: 0.375rem 0.875rem;
		border: 1px solid var(--color-primary);
		border-radius: 6px;
		transition: background 0.1s;
	}

	.demo-signup:hover {
		background: rgba(37, 99, 234, 0.08);
	}

	.demo-toggle {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.3rem 0.625rem;
		font-size: 0.75rem;
		font-family: inherit;
		font-weight: 500;
		color: var(--color-muted);
		background: transparent;
		border: 1px solid var(--color-border-canvas);
		border-radius: 6px;
		cursor: pointer;
		transition: color 0.1s, border-color 0.1s;
		white-space: nowrap;
	}

	.demo-toggle:hover {
		color: var(--color-text);
		border-color: var(--color-subtle);
	}

	.demo-toggle--collapsed {
		position: absolute;
		top: 0.5rem;
		left: 50%;
		transform: translateX(-50%);
		z-index: 50;
	}

	.demo-shell.light .demo-toggle {
		color: #6b7280;
		border-color: #e5e7eb;
	}

	.demo-shell.light .demo-toggle:hover {
		color: #111;
		border-color: #9ca3af;
	}

	/* Banner */
	.demo-banner {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		padding: 0.5rem 1.5rem;
		background: color-mix(in srgb, var(--color-primary) 10%, transparent);
		border-bottom: 1px solid color-mix(in srgb, var(--color-primary) 20%, transparent);
		font-size: 0.8125rem;
		color: var(--color-subtle);
	}

	.demo-shell.light .demo-banner {
		background: #eff6ff;
		border-bottom-color: #bfdbfe;
		color: #6b7280;
	}

	.demo-badge {
		font-size: 0.6875rem;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 0.1rem 0.5rem;
		border-radius: 999px;
		background: var(--color-primary);
		color: #fff;
	}

	/* Steps */
	.demo-steps {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1.5rem;
		border-bottom: 1px solid var(--color-border-canvas);
		overflow-x: auto;
		background: var(--color-base);
	}

	.demo-shell.light .demo-steps {
		background: #fff;
		border-bottom-color: #e5e7eb;
	}

	.demo-step {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		text-decoration: none;
		color: var(--color-muted);
		font-size: 0.8125rem;
		white-space: nowrap;
		transition: color 0.1s;
	}

	.demo-shell.light .demo-step {
		color: #9ca3af;
	}

	.demo-step:hover {
		color: var(--color-text);
	}

	.demo-shell.light .demo-step:hover {
		color: #111;
	}

	.demo-step.active {
		color: var(--color-text);
		font-weight: 600;
	}

	.demo-shell.light .demo-step.active {
		color: #111;
	}

	.demo-step.done {
		color: var(--color-subtle);
	}

	.demo-step-num {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 1.375rem;
		height: 1.375rem;
		border-radius: 50%;
		border: 1px solid currentColor;
		font-size: 0.75rem;
		font-weight: 600;
		flex-shrink: 0;
	}

	.demo-step.active .demo-step-num {
		background: var(--color-primary);
		border-color: var(--color-primary);
		color: #fff;
	}

	.demo-step.done .demo-step-num {
		background: var(--color-primary);
		border-color: var(--color-primary);
		color: #fff;
		opacity: 0.6;
	}

	.demo-step-arrow {
		color: var(--color-muted);
		font-size: 0.8125rem;
		flex-shrink: 0;
	}

	.demo-shell.light .demo-step-arrow {
		color: #d1d5db;
	}

	/* Content */
	.demo-content {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
	}
</style>
