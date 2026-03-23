<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { listForms, getForm, updateFormStatus, deleteForm, ApiError, type FormSummary } from '$lib/forms';
	import { auth } from '$lib/stores/auth.svelte';

	let forms = $state<FormSummary[]>([]);
	let loading = $state(true);
	let error = $state('');
	let formNames = $state<Map<string, string>>(new Map());

	onMount(async () => {
		try {
			forms = await listForms();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load forms';
		} finally {
			loading = false;
		}

		// Decrypt form names in parallel (best-effort; failures are silently ignored)
		if (auth.masterKey && forms.length > 0) {
			const results = await Promise.allSettled(
				forms.map((f) => getForm(auth.masterKey!, f.formId))
			);
			const names = new Map(formNames);
			results.forEach((r, i) => {
				if (r.status === 'fulfilled') {
					const { schema } = r.value;
					const title = schema.translations[schema.defaultLocale]?.formTitle;
					if (title) names.set(forms[i].formId, title);
				}
			});
			formNames = names;
		}
	});

	async function toggleStatus(form: FormSummary) {
		const next = form.status === 'open' ? 'closed' : 'open';
		try {
			await updateFormStatus(form.formId, next);
			forms = forms.map((f) => (f.formId === form.formId ? { ...f, status: next } : f));
		} catch {
			alert('Failed to update status');
		}
	}

	async function handleDelete(form: FormSummary) {
		if (!confirm(`Delete this form and all ${form.responseCount} response(s)? This cannot be undone.`)) return;
		try {
			await deleteForm(form.formId);
			forms = forms.filter((f) => f.formId !== form.formId);
		} catch {
			alert('Failed to delete form');
		}
	}
</script>

<svelte:head>
	<title>GhostForm — Forms</title>
</svelte:head>

<div style="font-family: monospace; max-width: 960px; padding: 32px 32px 48px;">
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 28px;">
		<h1 style="font-size: 1.4rem; margin: 0; color: #e2e8f0;">Forms</h1>
		<button
			onclick={() => goto('/forms/new')}
			style="
				padding: 8px 16px;
				background: #1d4ed8;
				color: #fff;
				border: none;
				border-radius: 4px;
				cursor: pointer;
				font-family: monospace;
				font-size: 0.85rem;
			"
		>
			+ New form
		</button>
	</div>

	{#if loading}
		<p style="color: #8899aa; font-size: 0.9rem;">Loading…</p>
	{:else if error}
		<p style="color: #f87171; font-size: 0.9rem;">{error}</p>
	{:else if forms.length === 0}
		<div style="
			padding: 48px 32px;
			border: 1px dashed #374151;
			border-radius: 8px;
			text-align: center;
			color: #8899aa;
		">
			<p style="margin: 0 0 8px; font-size: 0.95rem;">No forms yet</p>
			<p style="margin: 0; font-size: 0.8rem;">Create your first form to get started</p>
		</div>
	{:else}
		<table style="width: 100%; border-collapse: collapse; font-size: 0.85rem;">
			<thead>
				<tr style="border-bottom: 1px solid #2d3f55; color: #7a90a8;">
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Title</th>
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Form ID</th>
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Status</th>
					<th style="text-align: right; padding: 8px 12px; font-weight: normal;">Responses</th>
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Created</th>
					<th style="padding: 8px 12px;"></th>
				</tr>
			</thead>
			<tbody>
				{#each forms as form (form.formId)}
					<tr style="border-bottom: 1px solid #1e2d3e;">
						<td style="padding: 12px; color: #c5d3e0; font-size: 0.85rem;">
							{formNames.get(form.formId) ?? '—'}
						</td>
						<td style="padding: 12px; color: #4b6280; font-size: 0.8rem;">
							{form.formId.slice(0, 12)}…
						</td>
						<td style="padding: 12px;">
							<span style="
								padding: 2px 8px;
								border-radius: 9999px;
								font-size: 0.75rem;
								background: {form.status === 'open' ? '#14532d' : '#1a2332'};
								color: {form.status === 'open' ? '#86efac' : '#7a90a8'};
								border: 1px solid {form.status === 'open' ? '#166534' : '#2d3f55'};
							">
								{form.status}
							</span>
						</td>
						<td style="padding: 12px; text-align: right; color: #c5d3e0;">
							{form.responseCount}
						</td>
						<td style="padding: 12px; color: #7a90a8;">
							{form.createdAt}
						</td>
						<td style="padding: 12px; white-space: nowrap;">
							<div style="display: flex; gap: 8px; justify-content: flex-end;">
								<button
									onclick={() => goto(`/forms/${form.formId}/edit`)}
									style="
										padding: 4px 10px;
										background: transparent;
										color: #93c5fd;
										border: 1px solid #2d3f55;
										border-radius: 4px;
										cursor: pointer;
										font-family: monospace;
										font-size: 0.75rem;
									"
								>
									Edit
								</button>
								<button
									onclick={() => goto(`/forms/${form.formId}/responses`)}
									style="
										padding: 4px 10px;
										background: transparent;
										color: #a3e635;
										border: 1px solid #2d3f55;
										border-radius: 4px;
										cursor: pointer;
										font-family: monospace;
										font-size: 0.75rem;
									"
								>
									Responses ({form.responseCount})
								</button>
								<button
									onclick={() => toggleStatus(form)}
									style="
										padding: 4px 10px;
										background: transparent;
										color: #8899aa;
										border: 1px solid #2d3f55;
										border-radius: 4px;
										cursor: pointer;
										font-family: monospace;
										font-size: 0.75rem;
									"
								>
									{form.status === 'open' ? 'Close' : 'Open'}
								</button>
								<button
									onclick={() => handleDelete(form)}
									style="
										padding: 4px 10px;
										background: transparent;
										color: #f87171;
										border: 1px solid #2d3f55;
										border-radius: 4px;
										cursor: pointer;
										font-family: monospace;
										font-size: 0.75rem;
									"
								>
									Delete
								</button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>
