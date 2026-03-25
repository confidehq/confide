<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	// Logged-in users go straight to the dashboard.
	if (typeof window !== 'undefined' && auth.accountId) {
		goto('/dashboard');
	}
</script>

<svelte:head>
	<title>Wisp — Private, end-to-end encrypted forms</title>
	<meta name="description" content="Create private forms. Only you can read the responses." />
</svelte:head>

<main class="landing">
	<section class="hero">
		<h1>Wisp</h1>
		<p class="tagline">Private forms. End-to-end encrypted.</p>
		<div class="cta-group">
			<a href="/register" class="btn btn-primary">Create an account</a>
			<a href="/login" class="btn btn-ghost">Sign in</a>
		</div>
	</section>

	<section class="features">
		<div class="feature">
			<span class="icon" aria-hidden="true">🚫</span>
			<h2>Zero tracking</h2>
			<p>No IP logs. No cookies for respondents. No fingerprinting. Nothing.</p>
		</div>
		<div class="feature">
			<span class="icon" aria-hidden="true">🔒</span>
			<h2>End-to-end encrypted</h2>
			<p>
				Responses are encrypted in your browser before they leave the page. The server never sees
				plaintext.
			</p>
		</div>
		<div class="feature">
			<span class="icon" aria-hidden="true">🖥️</span>
			<h2>Self-hostable</h2>
			<p>Open source. One <code>docker compose up</code>. Your data, your server, your rules.</p>
		</div>
	</section>

	<footer class="landing-footer">
		<a href="https://github.com/phantompunk/wisp" rel="noopener">GitHub</a>
		<span aria-hidden="true">·</span>
		<a href="/docs/self-hosting">Self-hosting guide</a>
	</footer>
</main>

<style>
	.landing {
		max-width: 720px;
		margin: 0 auto;
		padding: 4rem 1.5rem 2rem;
		display: flex;
		flex-direction: column;
		gap: 4rem;
	}

	.hero {
		text-align: center;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}

	.hero h1 {
		font-size: 3rem;
		font-weight: 700;
		margin: 0;
		letter-spacing: -0.02em;
	}

	.tagline {
		font-size: 1.25rem;
		color: var(--color-text-muted, #666);
		margin: 0;
	}

	.cta-group {
		display: flex;
		gap: 0.75rem;
		flex-wrap: wrap;
		justify-content: center;
		margin-top: 0.5rem;
	}

	.features {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
		gap: 2rem;
	}

	.feature {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.feature .icon {
		font-size: 1.75rem;
	}

	.feature h2 {
		font-size: 1rem;
		font-weight: 600;
		margin: 0;
	}

	.feature p {
		font-size: 0.9rem;
		color: var(--color-text-muted, #666);
		margin: 0;
		line-height: 1.5;
	}

	.landing-footer {
		display: flex;
		gap: 0.75rem;
		align-items: center;
		font-size: 0.875rem;
		color: var(--color-text-muted, #888);
	}

	.landing-footer a {
		color: inherit;
		text-decoration: none;
	}

	.landing-footer a:hover {
		text-decoration: underline;
	}
</style>
