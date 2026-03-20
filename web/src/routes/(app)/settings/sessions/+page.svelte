<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { listSessions, revokeSession, logout } from '$lib/auth.ts';
	import { auth } from '$lib/stores/auth.svelte.ts';
	import type { SessionInfo } from '$lib/auth.ts';

	let sessions = $state<SessionInfo[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let revoking = $state<string | null>(null);

	onMount(async () => {
		await loadSessions();
	});

	async function loadSessions() {
		loading = true;
		error = null;
		try {
			sessions = await listSessions();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load sessions.';
		} finally {
			loading = false;
		}
	}

	async function handleRevoke(sessionId: string) {
		revoking = sessionId;
		try {
			await revokeSession(sessionId);
			sessions = sessions.filter((s) => s.id !== sessionId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to revoke session.';
		} finally {
			revoking = null;
		}
	}

	async function handleLogout() {
		await logout();
		auth.clearAll();
		goto('/login');
	}
</script>

<svelte:head>
	<title>GhostForm — Sessions</title>
</svelte:head>

<div style="font-family: monospace; max-width: 640px; margin: 60px auto; padding: 0 24px;">
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 32px;">
		<h1 style="font-size: 1.4rem; margin: 0;">Active Sessions</h1>
		<div style="display: flex; gap: 12px; align-items: center;">
			<a href="/dashboard" style="color: #60a5fa; font-size: 0.85rem; text-decoration: none;">Dashboard</a>
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

	{#if loading}
		<p style="color: #888; font-size: 0.9rem;">Loading sessions…</p>
	{:else if error}
		<div style="color: #fca5a5; font-size: 0.85rem; margin-bottom: 12px;">{error}</div>
	{:else if sessions.length === 0}
		<p style="color: #888; font-size: 0.9rem;">No active sessions.</p>
	{:else}
		<div style="display: flex; flex-direction: column; gap: 8px;">
			{#each sessions as session}
				<div style="
					display: flex;
					align-items: center;
					justify-content: space-between;
					padding: 14px 16px;
					background: #0d0d0d;
					border: 1px solid #374151;
					border-radius: 6px;
				">
					<div>
						<div style="color: #e5e7eb; font-size: 0.85rem; font-family: monospace;">
							{session.id.slice(0, 12)}…
						</div>
						<div style="color: #6b7280; font-size: 0.75rem; margin-top: 2px;">
							Created {session.createdAt} · Last seen {session.lastSeen}
						</div>
					</div>
					<button
						onclick={() => handleRevoke(session.id)}
						disabled={revoking === session.id}
						style="
							padding: 6px 14px;
							background: transparent;
							color: {revoking === session.id ? '#6b7280' : '#f87171'};
							border: 1px solid {revoking === session.id ? '#374151' : '#7f1d1d'};
							border-radius: 4px;
							cursor: {revoking === session.id ? 'not-allowed' : 'pointer'};
							font-family: monospace;
							font-size: 0.8rem;
						"
					>
						{revoking === session.id ? 'Revoking…' : 'Revoke'}
					</button>
				</div>
			{/each}
		</div>
	{/if}
</div>
