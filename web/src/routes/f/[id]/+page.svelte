<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { getPublicSchema, importRenderKey, ApiError } from '$lib/forms';
	import type { FormSchema } from '$lib/forms';
	import ScrollRenderer from '$lib/components/form/ScrollRenderer.svelte';
	import StepsRenderer from '$lib/components/form/StepsRenderer.svelte';

	type FormState = 'loading' | 'ready' | 'submitted' | 'closed' | 'invalid' | 'error';

	let formState = $state<FormState>('loading');
	let errorMessage = $state('');

	let schema = $state<FormSchema | null>(null);
	let publicFormKey = $state<ArrayBuffer | null>(null);
	let pgpPublicKey = $state<string | null>(null);
	let schemaVersion = $state(0);
	let locale = $state('en');
	let honeypotFields = $state<string[]>([]);
	let loadToken = $state('');

	const locales = $derived(schema ? (schema.locales ?? [schema.defaultLocale]) : []);

	onMount(async () => {
		const formId = $page.params.id ?? '';

		// Parse #rk=<base64url> from the URL fragment.
		// The fragment is never sent to the server by the browser.
		const hash = window.location.hash.slice(1);
		const params = new URLSearchParams(hash);
		const rkParam = params.get('rk');

		if (!rkParam || !formId) {
			formState = 'invalid';
			return;
		}

		// Parse ?locale= from query string, fallback applied after schema loads
		const queryLocale = new URLSearchParams(window.location.search).get('locale');

		try {
			const renderKey = await importRenderKey(rkParam);
			const result = await getPublicSchema(formId, renderKey);

			if (result.status === 'closed') {
				formState = 'closed';
				return;
			}

			schema = result.schema;
			publicFormKey = result.publicFormKey;
			pgpPublicKey = result.pgpPublicKey;
			schemaVersion = result.schemaVersion;
			honeypotFields = result.honeypotFields;
			loadToken = result.loadToken;

			// Use requested locale if the form supports it, else default
			const supported = result.schema.locales ?? [result.schema.defaultLocale];
			locale = queryLocale && supported.includes(queryLocale)
				? queryLocale
				: result.schema.defaultLocale;

			formState = 'ready';
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				formState = 'invalid';
			} else {
				errorMessage = err instanceof Error ? err.message : 'Unknown error';
				formState = 'error';
			}
		}
	});

	function handleSubmitted() {
		formState = 'submitted';
	}

	function switchLocale(code: string) {
		locale = code;
	}
</script>

<svelte:head>
	<meta name="referrer" content="no-referrer" />
</svelte:head>

{#if formState === 'loading'}
	<div class="shell">
		<p class="muted">Loading…</p>
	</div>
{:else if formState === 'invalid'}
	<div class="shell">
		<p class="muted">This link is invalid or the form no longer exists.</p>
	</div>
{:else if formState === 'closed'}
	<div class="shell">
		<p class="muted">This form is no longer accepting responses.</p>
	</div>
{:else if formState === 'error'}
	<div class="shell">
		<p class="muted">Something went wrong. Please try again later.</p>
		{#if errorMessage}<p class="muted small">{errorMessage}</p>{/if}
	</div>
{:else if formState === 'submitted'}
	<div class="shell">
		<p class="muted">{schema?.translations?.[locale]?.convoCompletionMessage ?? 'Your response has been submitted.'}</p>
	</div>
{:else if formState === 'ready' && schema && publicFormKey}
	{#if schema.layout === 'steps'}
		<StepsRenderer
			{schema}
			formId={$page.params.id!}
			{publicFormKey}
			{pgpPublicKey}
			{schemaVersion}
			{locale}
			{locales}
			{honeypotFields}
			{loadToken}
			onsubmitted={handleSubmitted}
			onlocalechange={switchLocale}
		/>
	{:else}
		<ScrollRenderer
			{schema}
			formId={$page.params.id!}
			{publicFormKey}
			{pgpPublicKey}
			{schemaVersion}
			{locale}
			{locales}
			{honeypotFields}
			{loadToken}
			onsubmitted={handleSubmitted}
			onlocalechange={switchLocale}
		/>
	{/if}
{/if}

<style>
	:global(html),
	:global(body) {
		background: #fff;
		color: #111;
	}

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
