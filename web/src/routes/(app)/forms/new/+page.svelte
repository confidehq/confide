<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { createForm } from '$lib/forms';
	import { emptySchema } from '$lib/stores/builder.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { formsStore } from '$lib/stores/forms.svelte';
	import { workspacesStore } from '$lib/stores/workspaces.svelte';

	let status = $state<'creating' | 'error'>('creating');
	let errorMessage = $state('');

	onMount(async () => {
		const masterKey = auth.masterKey;
		if (!masterKey) {
			goto('/login');
			return;
		}

		// Prefer explicit query param, then fall back to active workspace
		const workspaceId =
			$page.url.searchParams.get('workspaceId') ?? workspacesStore.active?.id ?? undefined;

		try {
			const schema = emptySchema();
			const { formId } = await createForm(masterKey, schema, workspaceId);
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
		<p class="text-muted text-sm">Creating form…</p>
	{:else}
		<p class="text-error-light text-sm mb-4">
			Failed to create form: {errorMessage}
		</p>
		<a href="/forms" class="text-muted-dark text-sm no-underline">
			← Go back to forms
		</a>
	{/if}
</div>
