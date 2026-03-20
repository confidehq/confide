<script lang="ts">
	import { onMount } from 'svelte';
	import { listForms, updateFormStatus, deleteForm, ApiError, type FormSummary } from '$lib/forms.ts';

	let forms = $state<FormSummary[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			forms = await listForms();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load forms';
		} finally {
			loading = false;
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

<div style="font-family: monospace; max-width: 900px; margin: 60px auto; padding: 0 24px;">
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 32px;">
		<h1 style="font-size: 1.4rem; margin: 0;">Forms</h1>
		<div style="display: flex; gap: 12px;">
			<a href="/dashboard" style="color: #9ca3af; font-size: 0.85rem; text-decoration: none;">← Dashboard</a>
			<button
				onclick={() => alert('Form builder coming in Phase 5')}
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
	</div>

	{#if loading}
		<p style="color: #6b7280; font-size: 0.9rem;">Loading…</p>
	{:else if error}
		<p style="color: #f87171; font-size: 0.9rem;">{error}</p>
	{:else if forms.length === 0}
		<div style="
			padding: 48px 32px;
			border: 1px dashed #374151;
			border-radius: 8px;
			text-align: center;
			color: #6b7280;
		">
			<p style="margin: 0 0 8px; font-size: 0.95rem;">No forms yet</p>
			<p style="margin: 0; font-size: 0.8rem;">Create your first form to get started</p>
		</div>
	{:else}
		<table style="width: 100%; border-collapse: collapse; font-size: 0.85rem;">
			<thead>
				<tr style="border-bottom: 1px solid #374151; color: #9ca3af;">
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Form ID</th>
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Status</th>
					<th style="text-align: right; padding: 8px 12px; font-weight: normal;">Responses</th>
					<th style="text-align: left; padding: 8px 12px; font-weight: normal;">Created</th>
					<th style="padding: 8px 12px;"></th>
				</tr>
			</thead>
			<tbody>
				{#each forms as form (form.formId)}
					<tr style="border-bottom: 1px solid #1f2937;">
						<td style="padding: 12px; color: #d1d5db; font-size: 0.8rem;">
							{form.formId.slice(0, 12)}…
						</td>
						<td style="padding: 12px;">
							<span style="
								padding: 2px 8px;
								border-radius: 9999px;
								font-size: 0.75rem;
								background: {form.status === 'open' ? '#14532d' : '#1f2937'};
								color: {form.status === 'open' ? '#86efac' : '#9ca3af'};
							">
								{form.status}
							</span>
						</td>
						<td style="padding: 12px; text-align: right; color: #d1d5db;">
							{form.responseCount}
						</td>
						<td style="padding: 12px; color: #6b7280;">
							{form.createdAt}
						</td>
						<td style="padding: 12px; white-space: nowrap;">
							<div style="display: flex; gap: 8px; justify-content: flex-end;">
								<button
									onclick={() => toggleStatus(form)}
									style="
										padding: 4px 10px;
										background: transparent;
										color: #9ca3af;
										border: 1px solid #374151;
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
										border: 1px solid #374151;
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
