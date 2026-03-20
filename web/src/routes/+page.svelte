<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	// Redirect on mount: has stored credential → dashboard, else → login
	if (typeof window !== 'undefined') {
		if (auth.accountId) {
			goto('/dashboard');
		} else {
			goto('/login');
		}
	}
</script>
