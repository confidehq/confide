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

<div style="
	font-family: monospace;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	flex: 1;
	background: #111827;
	color: #d1d5db;
">
	{#if status === 'creating'}
		<p style="color: #9ca3af; font-size: 0.95rem;">Creating form…</p>
	{:else}
		<p style="color: #f87171; font-size: 0.95rem; margin-bottom: 16px;">
			Failed to create form: {errorMessage}
		</p>
		<a
			href="/forms"
			style="color: #6b7280; font-size: 0.85rem; text-decoration: none;"
		>
			← Go back to forms
		</a>
	{/if}
</div>
