<script lang="ts">
	import { writable } from 'svelte/store';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getPublicSchema, importRenderKey, ApiError } from '$lib/forms';
	import type { FormSchema } from '$lib/forms';
	import ScrollRenderer from '$lib/components/form/ScrollRenderer.svelte';
	import StepsRenderer from '$lib/components/form/StepsRenderer.svelte';

	type State = 'loading' | 'ready' | 'submitted' | 'closed' | 'invalid' | 'error';

	const state = writable<State>('loading');
	const errorMessage = writable('');

	let schema: FormSchema | null = null;
	let publicFormKey: ArrayBuffer | null = null;
	let schemaVersion = 0;
	let locale = 'en';

	onMount(async () => {
		const formId = $page.params.id ?? '';

		// Parse #rk=<base64url> from the URL fragment.
		// The fragment is never sent to the server by the browser.
		const hash = window.location.hash.slice(1);
		const params = new URLSearchParams(hash);
		const rkParam = params.get('rk');

		if (!rkParam || !formId) {
			state.set('invalid');
			return;
		}

		// Parse ?locale= from query string, fallback applied after schema loads
		const queryLocale = new URLSearchParams(window.location.search).get('locale');

		try {
			const renderKey = await importRenderKey(rkParam);
			const result = await getPublicSchema(formId, renderKey);

			if (result.status === 'closed') {
				state.set('closed');
				return;
			}

			schema = result.schema;
			publicFormKey = result.publicFormKey;
			schemaVersion = result.schemaVersion;

			// Use requested locale if the form supports it, else default
			const supported = result.schema.locales ?? [result.schema.defaultLocale];
			locale = queryLocale && supported.includes(queryLocale)
				? queryLocale
				: result.schema.defaultLocale;

			state.set('ready');
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				state.set('invalid');
			} else {
				errorMessage.set(err instanceof Error ? err.message : 'Unknown error');
				state.set('error');
			}
		}
	});

	function handleSubmitted() {
		state.set('submitted');
	}
</script>

<svelte:head>
	<meta name="referrer" content="no-referrer" />
</svelte:head>

{#if $state === 'loading'}
	<div class="shell">
		<p class="muted">Loading…</p>
	</div>
{:else if $state === 'invalid'}
	<div class="shell">
		<p class="muted">This link is invalid or the form no longer exists.</p>
	</div>
{:else if $state === 'closed'}
	<div class="shell">
		<p class="muted">This form is no longer accepting responses.</p>
	</div>
{:else if $state === 'error'}
	<div class="shell">
		<p class="muted">Something went wrong. Please try again later.</p>
		{#if $errorMessage}<p class="muted small">{$errorMessage}</p>{/if}
	</div>
{:else if $state === 'submitted'}
	<div class="shell">
		<p class="muted">{schema?.translations?.[locale]?.convoCompletionMessage ?? 'Your response has been submitted.'}</p>
	</div>
{:else if $state === 'ready' && schema && publicFormKey}
	{#if schema.layout === 'steps'}
		<StepsRenderer
			{schema}
			formId={$page.params.id}
			{publicFormKey}
			{schemaVersion}
			{locale}
			onsubmitted={handleSubmitted}
		/>
	{:else}
		<ScrollRenderer
			{schema}
			formId={$page.params.id}
			{publicFormKey}
			{schemaVersion}
			{locale}
			onsubmitted={handleSubmitted}
		/>
	{/if}
{/if}

<style>
	.shell {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		padding: 2rem;
		font-family: system-ui, sans-serif;
	}

	.muted {
		color: #666;
		font-size: 1rem;
	}

	.small {
		font-size: 0.85rem;
	}
</style>
