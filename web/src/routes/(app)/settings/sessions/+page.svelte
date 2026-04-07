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

<div class="font-mono max-w-[640px] p-8 pb-12">
	<div class="flex items-center justify-between mb-7">
		<h1 class="text-[1.6rem] m-0 text-[#e2e8f0]">Active Sessions</h1>
		<button
			onclick={handleLogout}
			class="px-4 py-2 bg-transparent text-[#8899aa] border border-border-subtle rounded cursor-pointer font-mono text-[0.975rem] hover:text-muted transition-colors duration-100"
		>
			Sign out
		</button>
	</div>

	{#if loading}
		<p class="text-[#8899aa] text-[1.025rem]">Loading sessions…</p>
	{:else if error}
		<p class="text-error-light text-[0.975rem]">{error}</p>
	{:else if sessions.length === 0}
		<p class="text-[#8899aa] text-[1.025rem]">No active sessions.</p>
	{:else}
		<div class="flex flex-col gap-1.5">
			{#each sessions as session}
				<div class="flex items-center justify-between px-4 py-3 border border-border-deep rounded-md">
					<div class="flex items-center gap-3">
						<div class="text-[#4b6280] shrink-0" title={session.userAgent ?? 'Unknown device'}>
							{#if isMobile(session.userAgent)}
								<Smartphone size={18} strokeWidth={1.75} />
							{:else}
								<Monitor size={18} strokeWidth={1.75} />
							{/if}
						</div>
						<div>
							<div class="text-[#c5d3e0] text-[0.975rem]">{session.id.slice(0, 12)}…</div>
							<div class="text-[#4b6280] text-[0.875rem] mt-0.5">
								Created {session.createdAt} · Last seen {session.lastSeen}
							</div>
						</div>
					</div>
					<button
						onclick={() => handleRevoke(session.id)}
						disabled={revoking === session.id}
						class="px-3 py-1 bg-transparent border rounded cursor-pointer font-mono text-[0.925rem] transition-[color,border-color] duration-100
							{revoking === session.id
								? 'text-[#4b6280] border-border-subtle cursor-not-allowed'
								: 'text-error-light border-[#7f1d1d] hover:bg-[#1a0e0e]'}"
					>
						{revoking === session.id ? 'Revoking…' : 'Revoke'}
					</button>
				</div>
			{/each}
		</div>
	{/if}
</div>
