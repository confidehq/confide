<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="icon" href="/favicon.ico" sizes="any" />
	<link rel="apple-touch-icon" href="/icon.png" />
	<link rel="manifest" href="/manifest.webmanifest" />
	<meta property="og:image" content="/icon.png" />
	<meta name="twitter:image" content="/icon.png" />
	<meta name="twitter:card" content="summary_large_image" />
</svelte:head>

{@render children()}
