<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.ts';
	import { logout } from '$lib/auth.ts';
	import { goto } from '$app/navigation';

	async function handleLogout() {
		await logout();
		auth.clearAll();
		goto('/login');
	}
</script>

<svelte:head>
	<title>GhostForm — Dashboard</title>
</svelte:head>

<div style="font-family: monospace; max-width: 800px; margin: 60px auto; padding: 0 24px;">
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 40px;">
		<h1 style="font-size: 1.4rem; margin: 0;">Dashboard</h1>
		<div style="display: flex; gap: 12px; align-items: center;">
			<a href="/settings/sessions" style="color: #60a5fa; font-size: 0.85rem; text-decoration: none;">Sessions</a>
			<button
				onclick={handleLogout}
				style="
					padding: 8px 16px;
					background: transparent;
					color: #9ca3af;
					border: 1px solid #374151;
					border-radius: 4px;
					cursor: pointer;
					font-family: monospace;
					font-size: 0.85rem;
				"
			>
				Sign out
			</button>
		</div>
	</div>

	<div style="
		padding: 32px;
		border: 1px dashed #374151;
		border-radius: 8px;
		text-align: center;
		color: #6b7280;
	">
		<p style="font-size: 0.9rem; margin: 0 0 16px;">Account: {auth.accountId ?? '—'}</p>
		<a href="/forms" style="
			display: inline-block;
			padding: 8px 20px;
			background: transparent;
			color: #60a5fa;
			border: 1px solid #374151;
			border-radius: 4px;
			font-family: monospace;
			font-size: 0.85rem;
			text-decoration: none;
		">View forms →</a>
	</div>
</div>
