<script lang="ts">
	import { onMount } from 'svelte';
	import { listSessions, revokeSession } from '$lib/auth';
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

</script>

<svelte:head>
	<title>Confide — Sessions</title>
</svelte:head>

<div class="font-mono max-w-7xl mx-auto px-4 pt-12 pb-12 sm:p-8 sm:pb-12">
	<div class="mb-7">
		<h1 class="text-2xl m-0 text-text-bright">Active Sessions</h1>
	</div>

	{#if loading}
		<p class="text-muted-blue text-base">Loading sessions…</p>
	{:else if error}
		<p class="text-error-light text-sm">{error}</p>
	{:else if sessions.length === 0}
		<p class="text-muted-blue text-base">No active sessions.</p>
	{:else}
		<div class="flex flex-col gap-1.5">
			{#each sessions as session}
				<div class="flex items-start justify-between gap-3 px-4 py-3 border border-border-deep rounded-md">
					<div class="flex items-start gap-3 min-w-0">
						<div class="text-muted-dim shrink-0 mt-0.5" title={session.userAgent ?? 'Unknown device'}>
							{#if isMobile(session.userAgent)}
								<Smartphone size={18} strokeWidth={1.75} />
							{:else}
								<Monitor size={18} strokeWidth={1.75} />
							{/if}
						</div>
						<div class="min-w-0">
							<div class="text-text-body text-sm">{session.id.slice(0, 12)}…</div>
							<div class="text-muted-dim text-xs mt-0.5">Created {session.createdAt}</div>
							<div class="text-muted-dim text-xs">Last seen {session.lastSeen}</div>
						</div>
					</div>
					<button
						onclick={() => handleRevoke(session.id)}
						disabled={revoking === session.id}
						class="shrink-0 px-3 py-1 bg-transparent border rounded cursor-pointer font-mono text-sm transition-[color,border-color] duration-100
							{revoking === session.id
								? 'text-muted-dim border-border-subtle cursor-not-allowed'
								: 'text-error-light border-border-danger-dark hover:bg-danger-hover'}"
					>
						{revoking === session.id ? 'Revoking…' : 'Revoke'}
					</button>
				</div>
			{/each}
		</div>
	{/if}
</div>
