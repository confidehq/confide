<script lang="ts">
import { page } from "$app/state";
import faviconSvg from "$lib/assets/favicon.svg?raw";
import { auth } from "$lib/stores/auth.svelte";

const is404 = $derived(page.status === 404);
const isLoggedIn = $derived(auth.masterKey !== null);
</script>

<svelte:head>
	<title>Confide — {page.status} {is404 ? 'Not Found' : 'Error'}</title>
</svelte:head>

<div class="min-h-screen flex flex-col items-center justify-center px-4 font-mono">
	<div class="w-full max-w-[360px] text-center">

		<a href="https://useconfide.app" class="w-14 h-14 mb-6 [&>svg]:w-full [&>svg]:h-full inline-block">{@html faviconSvg}</a>

		<p class="text-5xl font-semibold text-text tabular-nums mb-2">{page.status}</p>
		<h1 class="text-lg font-semibold text-text tracking-tight mb-1">
			{is404 ? 'Page not found' : 'Something went wrong'}
		</h1>
		<p class="text-sm text-subtle mb-8">
			{is404
				? "The page you're looking for doesn't exist or has been moved."
				: page.error?.message ?? 'An unexpected error occurred.'}
		</p>

		{#if isLoggedIn}
			<a
				href="/dashboard"
				class="inline-block px-5 py-2.5 bg-primary text-white rounded-lg text-sm font-medium no-underline hover:bg-primary-hover transition-colors duration-100"
			>
				Go to dashboard
			</a>
		{:else}
			<a
				href="/login"
				class="inline-block px-5 py-2.5 bg-primary text-white rounded-lg text-sm font-medium no-underline hover:bg-primary-hover transition-colors duration-100"
			>
				Sign in
			</a>
		{/if}

	</div>
</div>
