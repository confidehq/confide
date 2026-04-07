<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { createForm } from '$lib/forms';
	import { emptySchema } from '$lib/stores/builder.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';

	let status = $state<'creating' | 'error'>('creating');
	let errorMessage = $state('');

	onMount(async () => {
		const masterKey = auth.masterKey;
		if (!masterKey) {
			goto('/login');
			return;
		}

		try {
			const schema = emptySchema();
			const { formId } = await createForm(masterKey, schema);
			formsStore.invalidate();
			goto(`/forms/${formId}/edit`);
		} catch (err) {
			status = 'error';
			errorMessage = err instanceof Error ? err.message : 'Unknown error';
		}
	});
</script>

<svelte:head>
	<title>Creating form…</title>
</svelte:head>

<div class="font-mono flex flex-col items-center justify-center flex-1 bg-canvas text-text-dim">
	{#if status === 'creating'}
		<p class="text-muted text-[0.95rem]">Creating form…</p>
	{:else}
		<p class="text-error-light text-[0.95rem] mb-4">
			Failed to create form: {errorMessage}
		</p>
		<a href="/forms" class="text-muted-dark text-[0.975rem] no-underline">
			← Go back to forms
		</a>
	{/if}
</div>
