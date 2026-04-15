<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	// Logged-in users go straight to the dashboard.
	if (typeof window !== 'undefined' && auth.accountId) {
		goto('/dashboard');
	}
</script>

<svelte:head>
	<title>Confide — Private, end-to-end encrypted forms</title>
	<meta name="description" content="Create private forms. Only you can read the responses." />
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;600;700&family=IBM+Plex+Sans:wght@300;400;500&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<main class="landing">
	<div class="bg-grid" aria-hidden="true"></div>
	<div class="glow glow-top" aria-hidden="true"></div>
	<div class="glow glow-mid" aria-hidden="true"></div>

	<header class="site-header">
		<a href="/" class="wordmark">Confide</a>
		<nav class="header-nav">
			<a href="https://github.com/phantompunk/confide" rel="noopener">GitHub</a>
			<a href="/docs/self-hosting">Docs</a>
			<a href="/login" class="btn btn-ghost btn-sm">Sign in</a>
		</nav>
	</header>

	<section class="hero">
		<div class="tagline-block">
			<span class="word word-1">collect.</span>
			<span class="word word-2">encrypt.</span>
			<span class="word word-3">forget.</span>
		</div>
		<p class="subtitle">Forms without footprints.</p>
		<div class="cta-group">
			<a href="/signup" class="btn btn-primary">Create an account</a>
			<a href="/login" class="btn btn-ghost">Sign in</a>
		</div>
	</section>

	<section class="prose-sections">
		<div class="prose-item">
			<h3 class="prose-heading">The problem with<br/>form providers</h3>
			<div class="prose-body">
				<p>Most form builders <em>are</em> surveillance. They log IP addresses, fingerprint devices, embed 3rd-party tracking scripts, and store every response in plaintext on servers they fully control. One subpoena, one breach, one policy change — and every answer, every respondent is exposed.</p>
			</div>
		</div>

		<div class="prose-divider" aria-hidden="true"></div>

		<div class="prose-item">
			<h3 class="prose-heading">What is<br/>Confide?</h3>
			<div class="prose-body">
				<p>You share a link. Someone fills it out. You read it. Nobody else — not us, not a subpoena, not a breach — ever could.</p>
				<p>Anyone can create a form and share it, but answers can only be read by the person who created it. Respondents leave no trace.</p>
			</div>
		</div>

		<div class="prose-divider" aria-hidden="true"></div>

		<div class="prose-item">
			<h3 class="prose-heading">How is Confide<br/>different?</h3>
			<div class="prose-body">
				<p>Responses are encrypted before they leave your device, using keys only you hold. We store encrypted text we cannot read, on servers that log nothing that could identify who submitted it.</p>
				<!-- <p class="prose-emphasis">Anonymity is architectural — not a policy promise, not a checkbox, not a feature you have to turn on. It's the only way the system works.</p> -->
			</div>
		</div>
	</section>

	<section class="bottom-cta">
		<p class="bottom-cta-line">Convinced? It's free to start.</p>
		<a href="/signup" class="btn btn-primary">Create an account →</a>
	</section>

	<footer class="landing-footer">
		<a href="https://github.com/phantompunk/confide" rel="noopener">GitHub</a>
		<span aria-hidden="true">·</span>
		<a href="/docs/self-hosting">Self-hosting guide</a>
		<span aria-hidden="true">·</span>
		<span>Open source, AGPL licensed</span>
	</footer>
</main>

<style>
	:global(body) {
		background-color: var(--color-brand-bg);
		color: var(--color-brand-text);
		font-family: 'IBM Plex Sans', sans-serif;
		margin: 0;
	}

	.landing {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: 0 1.5rem 3rem;
		position: relative;
		overflow: hidden;
	}

	.bg-grid {
		position: fixed;
		inset: 0;
		background-image: radial-gradient(circle, rgba(255, 255, 255, 0.04) 1px, transparent 1px);
		background-size: 28px 28px;
		pointer-events: none;
		z-index: 0;
	}

	.glow {
		position: absolute;
		pointer-events: none;
		z-index: 0;
	}

	.glow-top {
		top: -10%;
		left: 50%;
		transform: translateX(-50%);
		width: 700px;
		height: 500px;
		background: radial-gradient(ellipse at center, rgba(99, 179, 175, 0.08) 0%, transparent 70%);
	}

	.glow-mid {
		top: 120vh;
		left: 30%;
		width: 500px;
		height: 500px;
		background: radial-gradient(ellipse at center, rgba(99, 179, 175, 0.05) 0%, transparent 70%);
	}

	/* ── Header ── */
	.site-header {
		position: relative;
		z-index: 1;
		width: 100%;
		max-width: 860px;
		padding: 1.75rem 0;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.wordmark {
		font-family: 'IBM Plex Mono', monospace;
		font-size: 0.875rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--color-brand-text);
		text-decoration: none;
	}

	.header-nav {
		display: flex;
		align-items: center;
		gap: 1.5rem;
	}

	.header-nav a:not(.btn) {
		font-family: 'IBM Plex Mono', monospace;
		font-size: 0.75rem;
		letter-spacing: 0.05em;
		color: rgba(232, 232, 240, 0.4);
		text-decoration: none;
		transition: color 0.15s;
	}

	.header-nav a:not(.btn):hover {
		color: rgba(232, 232, 240, 0.8);
	}

	/* ── Hero ── */
	.hero {
		position: relative;
		z-index: 1;
		text-align: center;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1.75rem;
		padding: 6rem 0 5rem;
		max-width: 860px;
		width: 100%;
	}

	.tagline-block {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 0.15em 0.55em;
		line-height: 1;
	}

	.word {
		font-family: 'IBM Plex Mono', monospace;
		font-size: clamp(2.6rem, 8vw, 3.5rem);
		font-weight: 700;
		letter-spacing: -0.02em;
		display: inline-block;
		opacity: 0;
		animation: rise 0.6s cubic-bezier(0.16, 1, 0.3, 1) forwards;
	}

	.word-1 { animation-delay: 0.1s;  color: var(--color-brand-text); }
	.word-2 { animation-delay: 0.28s; color: var(--color-brand-text-mid); }
	.word-3 { animation-delay: 0.46s; color: var(--color-brand-accent); }

	@keyframes rise {
		from { opacity: 0; transform: translateY(18px); }
		to   { opacity: 1; transform: translateY(0); }
	}

	.subtitle {
		font-family: 'IBM Plex Sans', sans-serif;
		font-size: clamp(1rem, 2.5vw, 1.2rem);
		font-weight: 300;
		color: rgba(232, 232, 240, 0.45);
		letter-spacing: 0.015em;
		margin: 0;
		opacity: 0;
		animation: rise 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.65s forwards;
	}

	.cta-group {
		display: flex;
		gap: 0.75rem;
		flex-wrap: wrap;
		justify-content: center;
		opacity: 0;
		animation: rise 0.5s cubic-bezier(0.16, 1, 0.3, 1) 0.8s forwards;
	}

	/* ── Buttons ── */
	.btn {
		font-family: 'IBM Plex Mono', monospace;
		font-size: 0.8rem;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 0.7rem 1.6rem;
		border-radius: 4px;
		text-decoration: none;
		transition: background 0.15s, color 0.15s, border-color 0.15s, box-shadow 0.15s;
		display: inline-block;
	}

	.btn-sm {
		padding: 0.45rem 1rem;
		font-size: 0.72rem;
	}

	.btn-primary {
		background: var(--color-brand-accent);
		color: var(--color-brand-bg);
		border: 1px solid var(--color-brand-accent);
	}

	.btn-primary:hover {
		background: var(--color-brand-accent-hover);
		border-color: var(--color-brand-accent-hover);
		box-shadow: 0 0 20px rgba(129, 191, 187, 0.3);
	}

	.btn-ghost {
		background: transparent;
		color: rgba(232, 232, 240, 0.6);
		border: 1px solid rgba(232, 232, 240, 0.15);
	}

	.btn-ghost:hover {
		border-color: rgba(232, 232, 240, 0.35);
		color: var(--color-brand-text);
	}

	/* ── Prose sections ── */
	.prose-sections {
		position: relative;
		z-index: 1;
		max-width: 860px;
		width: 100%;
		margin-top: 4rem;
	}

	.prose-item {
		display: grid;
		grid-template-columns: 25% 1fr;
		gap: 3rem;
		padding: 3.5rem 0;
		align-items: start;
	}

	.prose-divider {
		width: 100%;
		height: 1px;
		background: rgba(232, 232, 240, 0.07);
	}

	.prose-heading {
		font-family: 'IBM Plex Mono', monospace;
		font-size: 0.72rem;
		font-weight: 600;
		letter-spacing: 0.07em;
		text-transform: uppercase;
		color: var(--color-brand-accent);
		margin: 0;
		padding-top: 0.25rem;
		line-height: 1.6;
	}

	.prose-body {
		display: flex;
		flex-direction: column;
		gap: 0;
	}

	.prose-body p {
		font-family: 'IBM Plex Sans', sans-serif;
		font-size: 0.975rem;
		font-weight: 300;
		color: rgba(232, 232, 240, 0.55);
		margin: 0 0 0.9rem;
		line-height: 1.8;
	}

	.prose-body p:last-child {
		margin-bottom: 0;
	}

	.prose-body em {
		font-style: normal;
		color: rgba(232, 232, 240, 0.85);
	}

	.prose-emphasis {
		color: rgba(232, 232, 240, 0.7) !important;
		font-style: italic !important;
		border-left: 2px solid rgba(129, 191, 187, 0.35);
		padding-left: 1rem;
	}

	/* ── Bottom CTA ── */
	.bottom-cta {
		position: relative;
		z-index: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1.25rem;
		padding: 5rem 0 2rem;
		max-width: 860px;
		width: 100%;
		text-align: center;
	}

	.bottom-cta-line {
		font-family: 'IBM Plex Sans', sans-serif;
		font-size: 1rem;
		font-weight: 300;
		color: rgba(232, 232, 240, 0.35);
		margin: 0;
		letter-spacing: 0.01em;
	}

	/* ── Footer ── */
	.landing-footer {
		position: relative;
		z-index: 1;
		display: flex;
		gap: 0.75rem;
		align-items: center;
		font-family: 'IBM Plex Mono', monospace;
		font-size: 0.72rem;
		letter-spacing: 0.04em;
		color: rgba(232, 232, 240, 0.2);
		margin-top: 2rem;
		padding-bottom: 1rem;
	}

	.landing-footer a {
		color: inherit;
		text-decoration: none;
		transition: color 0.15s;
	}

	.landing-footer a:hover {
		color: rgba(232, 232, 240, 0.55);
	}

	/* ── Responsive ── */
	@media (max-width: 640px) {
		.header-nav a:not(.btn) {
			display: none;
		}

		.prose-item {
			grid-template-columns: 1fr;
			gap: 0.75rem;
			padding: 2.25rem 0;
		}
	}
</style>
