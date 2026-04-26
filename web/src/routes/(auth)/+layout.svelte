<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	$effect(() => {
		if (auth.masterKey !== null) {
			const next = page.url.searchParams.get('next') ?? '/dashboard';
			goto(next);
		}
	});
</script>

{@render children()}
