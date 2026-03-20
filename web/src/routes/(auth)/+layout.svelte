<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	$effect(() => {
		if (auth.masterKey !== null) {
			goto('/dashboard');
		}
	});
</script>

{@render children()}
