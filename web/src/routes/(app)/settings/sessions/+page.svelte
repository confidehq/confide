<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { listSessions, revokeSession, logout } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import type { SessionInfo } from '$lib/types/auth';

	function isMobile(ua: string | undefined): boolean {
		if (!ua) return false;
		return /Mobile|Android|iPhone|iPad|iPod/i.test(ua);
	}

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

<div style="font-family: monospace; max-width: 640px; padding: 32px 32px 48px;">
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 28px;">
		<h1 style="font-size: 1.4rem; margin: 0; color: #e2e8f0;">Active Sessions</h1>
		<button
			onclick={handleLogout}
			style="
				padding: 8px 16px;
				background: transparent;
				color: #8899aa;
				border: 1px solid #2d3f55;
				border-radius: 4px;
				cursor: pointer;
				font-family: monospace;
				font-size: 0.85rem;
			"
		>
			Sign out
		</button>
	</div>

	{#if loading}
		<p style="color: #8899aa; font-size: 0.9rem;">Loading sessions…</p>
	{:else if error}
		<p style="color: #f87171; font-size: 0.85rem;">{error}</p>
	{:else if sessions.length === 0}
		<p style="color: #8899aa; font-size: 0.9rem;">No active sessions.</p>
	{:else}
		<div style="display: flex; flex-direction: column; gap: 6px;">
			{#each sessions as session}
				<div style="
					display: flex;
					align-items: center;
					justify-content: space-between;
					padding: 12px 16px;
					border: 1px solid #1e2d3e;
					border-radius: 6px;
				">
					<div style="display: flex; align-items: center; gap: 12px;">
						<div style="color: #4b6280; flex-shrink: 0;" title={session.userAgent ?? 'Unknown device'}>
							{#if isMobile(session.userAgent)}
								<!-- Phone icon -->
								<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
									<rect x="5" y="2" width="14" height="20" rx="2" ry="2"/>
									<line x1="12" y1="18" x2="12.01" y2="18"/>
								</svg>
							{:else}
								<!-- Monitor icon -->
								<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
									<rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
									<line x1="8" y1="21" x2="16" y2="21"/>
									<line x1="12" y1="17" x2="12" y2="21"/>
								</svg>
							{/if}
						</div>
						<div>
							<div style="color: #c5d3e0; font-size: 0.85rem;">{session.id.slice(0, 12)}…</div>
							<div style="color: #4b6280; font-size: 0.75rem; margin-top: 3px;">
								Created {session.createdAt} · Last seen {session.lastSeen}
							</div>
						</div>
					</div>
					<button
						onclick={() => handleRevoke(session.id)}
						disabled={revoking === session.id}
						style="
							padding: 5px 12px;
							background: transparent;
							color: {revoking === session.id ? '#4b6280' : '#f87171'};
							border: 1px solid {revoking === session.id ? '#2d3f55' : '#7f1d1d'};
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
