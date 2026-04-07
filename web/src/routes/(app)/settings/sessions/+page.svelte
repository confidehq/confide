<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { listSessions, revokeSession, logout } from '$lib/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import type { SessionInfo } from '$lib/types/auth';
	import { Smartphone, Monitor } from '@lucide/svelte';

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
	<title>Confide — Sessions</title>
</svelte:head>

<div style="font-family: monospace; max-width: 640px; padding: 32px 32px 48px;">
	<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 28px;">
		<h1 style="font-size: 1.6rem; margin: 0; color: #e2e8f0;">Active Sessions</h1>
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
				font-size: 0.975rem;
			"
		>
			Sign out
		</button>
	</div>

	{#if loading}
		<p style="color: #8899aa; font-size: 1.025rem;">Loading sessions…</p>
	{:else if error}
		<p style="color: #f87171; font-size: 0.975rem;">{error}</p>
	{:else if sessions.length === 0}
		<p style="color: #8899aa; font-size: 1.025rem;">No active sessions.</p>
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
								<Smartphone size={18} strokeWidth={1.75} />
							{:else}
								<Monitor size={18} strokeWidth={1.75} />
							{/if}
						</div>
						<div>
							<div style="color: #c5d3e0; font-size: 0.975rem;">{session.id.slice(0, 12)}…</div>
							<div style="color: #4b6280; font-size: 0.875rem; margin-top: 3px;">
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
							font-size: 0.925rem;
						"
					>
						{revoking === session.id ? 'Revoking…' : 'Revoke'}
					</button>
				</div>
			{/each}
		</div>
	{/if}
</div>
