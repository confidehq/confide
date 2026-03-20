<script lang="ts">
	import { writable } from 'svelte/store';
	import { onMount } from 'svelte';
	import { getPublicSchema, importRenderKey, ApiError } from '$lib/forms';

	type State = 'loading' | 'ready' | 'closed' | 'invalid' | 'error';

	const state = writable<State>('loading');
	const schemaInfo = writable({ layout: '', fieldCount: 0 });
	const errorMessage = writable('');

	onMount(async () => {
		// Extract formId from path: /f/<formId>
		const pathParts = window.location.pathname.split('/');
		const formId = pathParts[pathParts.length - 1] ?? '';

		// Parse #rk=<base64url> from the URL fragment.
		// The fragment is never sent to the server by the browser.
		const hash = window.location.hash.slice(1);
		const params = new URLSearchParams(hash);
		const rkParam = params.get('rk');

		if (!rkParam || !formId) {
			state.set('invalid');
			return;
		}

		try {
			const renderKey = await importRenderKey(rkParam);
			const result = await getPublicSchema(formId, renderKey);

			if (result.status === 'closed') {
				state.set('closed');
				return;
			}

			schemaInfo.set({ layout: result.schema.layout, fieldCount: result.schema.fields.length });
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
{:else if $state === 'ready'}
	<div class="shell">
		<!-- Phase 6 will render the full form. For now confirm the schema loaded. -->
		<p class="muted">Form loaded. Renderer coming in Phase 6.</p>
		<p class="muted small">Layout: {$schemaInfo.layout} · Fields: {$schemaInfo.fieldCount}</p>
	</div>
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
