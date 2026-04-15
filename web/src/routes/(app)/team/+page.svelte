<script lang="ts">
	import { workspacesStore } from '$lib/stores/workspaces.svelte';
	import { teamStore } from '$lib/stores/team.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		updateMemberRole, removeMember,
		createInvitation, revokeInvitation,
		grantKey,
		WorkspaceError,
		type WorkspaceMember, type WorkspaceInvitation
	} from '$lib/workspaces';
	import { MoreHorizontal, ShieldCheck, UserMinus, RefreshCw, UserPlus, X, Mail, KeyRound } from '@lucide/svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	// ─── Invite form state ───────────────────────────────────────────────────────

	let showInviteForm = $state(false);
	let inviteEmail = $state('');
	let inviteRole = $state<'admin' | 'member' | 'viewer'>('member');
	let inviting = $state(false);
	let inviteError = $state('');
	let inviteSuccess = $state('');

	// ─── Revoke / actions state ──────────────────────────────────────────────────

	let revokingId = $state<string | null>(null);
	let openMenuId = $state<string | null>(null);
	let roleLoading = $state(false);
	let roleError = $state('');
	let removeTarget = $state<WorkspaceMember | null>(null);
	let removing = $state(false);
	let removeError = $state('');
	let grantingId = $state<string | null>(null);
	let grantError = $state('');

	// ─── Load on workspace change (cached — no re-fetch on back-navigation) ─────

	$effect(() => {
		const ws = workspacesStore.active;
		if (ws) {
			const isAdmin = ws.role === 'owner' || ws.role === 'admin';
			teamStore.load(ws.id, isAdmin);
		}
	});

	async function refresh() {
		const ws = workspacesStore.active;
		if (!ws) return;
		teamStore.invalidate();
		teamStore.load(ws.id, ws.role === 'owner' || ws.role === 'admin');
	}

	// ─── Invite ─────────────────────────────────────────────────────────────────

	function openInviteForm() {
		inviteEmail = '';
		inviteRole = 'member';
		inviteError = '';
		inviteSuccess = '';
		showInviteForm = true;
	}

	function closeInviteForm() {
		showInviteForm = false;
		inviteEmail = '';
		inviteError = '';
		inviteSuccess = '';
	}

	async function handleInvite() {
		const email = inviteEmail.trim();
		if (!email) return;
		const ws = workspacesStore.active;
		if (!ws) return;

		inviting = true;
		inviteError = '';
		inviteSuccess = '';

		try {
			const inv = await createInvitation(ws.id, email, inviteRole);
			teamStore.addInvitation(inv);
			inviteSuccess = `Invitation sent to ${email}`;
			inviteEmail = '';
		} catch (e) {
			if (e instanceof WorkspaceError) {
				if (e.code === 'plan_limit') {
					inviteError = 'Free plan allows only one collaborator. Upgrade to Pro to invite more.';
				} else {
					inviteError = e.message;
				}
			} else {
				inviteError = e instanceof Error ? e.message : 'Failed to send invitation.';
			}
		} finally {
			inviting = false;
		}
	}

	async function handleRevoke(inv: WorkspaceInvitation) {
		const ws = workspacesStore.active;
		if (!ws) return;
		revokingId = inv.id;
		try {
			await revokeInvitation(ws.id, inv.id);
			teamStore.removeInvitation(inv.id);
		} catch { /* non-fatal */ } finally {
			revokingId = null;
		}
	}

	// ─── Key grant ──────────────────────────────────────────────────────────────

	async function handleGrant(member: WorkspaceMember) {
		const ws = workspacesStore.active;
		const masterKey = auth.masterKey;
		if (!ws || !masterKey) return;

		const targetPubKey = teamStore.identityKeys.get(member.accountId);
		if (!targetPubKey) return;

		grantingId = member.accountId;
		grantError = '';
		try {
			await grantKey(ws.id, member.accountId, targetPubKey, masterKey);
			teamStore.updateMember(member.accountId, { status: 'active' });
		} catch (e) {
			grantError = e instanceof Error ? e.message : 'Failed to grant access.';
		} finally {
			grantingId = null;
		}
	}

	// ─── Role update ────────────────────────────────────────────────────────────

	async function handleRoleChange(member: WorkspaceMember, newRole: string) {
		const ws = workspacesStore.active;
		if (!ws) return;
		roleLoading = true;
		roleError = '';
		openMenuId = null;
		try {
			await updateMemberRole(ws.id, member.accountId, newRole);
			teamStore.updateMember(member.accountId, { role: newRole as WorkspaceMember['role'] });
		} catch (e) {
			roleError =
				e instanceof WorkspaceError && e.code === 'last_owner'
					? 'Cannot demote the sole owner.'
					: e instanceof Error ? e.message : 'Failed to update role.';
		} finally {
			roleLoading = false;
		}
	}

	// ─── Remove member ──────────────────────────────────────────────────────────

	async function handleRemove() {
		const ws = workspacesStore.active;
		if (!ws || !removeTarget) return;
		removing = true;
		removeError = '';
		try {
			await removeMember(ws.id, removeTarget.accountId);
			teamStore.removeMember(removeTarget.accountId);
			removeTarget = null;
		} catch (e) {
			removeError =
				e instanceof WorkspaceError && e.code === 'last_owner'
					? 'Cannot remove the sole owner.'
					: e instanceof Error ? e.message : 'Failed to remove member.';
		} finally {
			removing = false;
		}
	}

	// ─── Helpers ────────────────────────────────────────────────────────────────

	const myRole = $derived(workspacesStore.active?.role ?? 'viewer');
	const canManage = $derived(myRole === 'owner' || myRole === 'admin');

	// Alias store references for template conciseness
	const members = $derived(teamStore.members);
	const invitations = $derived(teamStore.invitations);
	const identityKeys = $derived(teamStore.identityKeys);
	const loading = $derived(teamStore.loading);
	const error = $derived(teamStore.error);

	function roleRank(role: string): number {
		return { owner: 4, admin: 3, member: 2, viewer: 1 }[role] ?? 0;
	}

	const allRoles = ['owner', 'admin', 'member', 'viewer'] as const;

	// Roles available to assign when inviting (never offer 'owner')
	const inviteRoleOptions = $derived(
		(['admin', 'member', 'viewer'] as const).filter(r => roleRank(r) <= roleRank(myRole))
	);

	function availableRoles(target: WorkspaceMember): readonly string[] {
		return allRoles.filter(r => roleRank(r) <= roleRank(myRole) && r !== target.role);
	}

	function formatDate(s: string): string {
		if (!s) return '—';
		const d = new Date(s);
		if (isNaN(d.getTime())) return '—';
		return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}

	function formatExpiry(iso: string): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (isNaN(d.getTime())) return '';
		const diff = Math.ceil((d.getTime() - Date.now()) / (1000 * 60 * 60 * 24));
		if (diff <= 0) return 'Expired';
		if (diff === 1) return 'Expires tomorrow';
		return `Expires in ${diff}d`;
	}
</script>

<svelte:head>
	<title>Confide — Members</title>
</svelte:head>

<ConfirmDialog
	open={!!removeTarget}
	title="Remove member?"
	description={removeTarget
		? `Remove ${removeTarget.username || removeTarget.accountId.slice(0, 8)} from ${workspacesStore.active?.name ?? 'this workspace'}? They will lose access immediately.`
		: ''}
	loading={removing}
	error={removeError}
	onconfirm={handleRemove}
	oncancel={() => { removeTarget = null; removeError = ''; }}
/>

<div class="flex justify-center w-full">
<div class="font-mono w-full max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl px-4 pt-10 pb-12 sm:px-8 sm:pt-10">

	<!-- Header -->
	<div class="flex items-start justify-between mb-6 gap-4">
		<div class="min-w-0">
			<h1 class="text-2xl m-0 mb-1 text-[#e2e8f0] font-semibold">Members</h1>
			{#if workspacesStore.active}
				<p class="m-0 text-sm text-[#4b6280]">{workspacesStore.active.name}</p>
			{/if}
		</div>
		<div class="flex items-center gap-2 shrink-0">
			<button
				onclick={refresh}
				disabled={loading}
				title="Refresh"
				class="flex items-center justify-center w-9 h-9 bg-transparent border border-border-deep rounded cursor-pointer text-[#4b6280] hover:text-[#c5d3e0] hover:border-border-subtle transition-colors duration-100 disabled:opacity-40 disabled:cursor-not-allowed"
			>
				<RefreshCw size={15} strokeWidth={1.75} class={loading ? 'animate-spin' : ''} />
			</button>
			{#if canManage}
				<button
					onclick={openInviteForm}
					class="flex items-center gap-2 px-4 py-2 bg-primary text-white border-none rounded cursor-pointer font-mono text-sm hover:bg-primary-hover transition-colors duration-100"
				>
					<UserPlus size={15} strokeWidth={1.75} />
					Add member
				</button>
			{/if}
		</div>
	</div>

	<!-- Inline invite form -->
	{#if showInviteForm}
		<div class="mb-6 p-5 border border-[#1e3347] rounded-lg bg-[#080f18]">
			<div class="flex items-center justify-between gap-2 mb-4">
				<p class="m-0 text-sm text-[#c5d3e0] font-medium">Invite a new member</p>
				<button
					onclick={closeInviteForm}
					class="bg-transparent border-none cursor-pointer text-[#374d63] hover:text-[#8899aa] transition-colors duration-100 p-1 rounded"
					aria-label="Close"
				><X size={15} strokeWidth={1.75} /></button>
			</div>

			<div class="flex flex-col sm:flex-row gap-3">
				<!-- Email -->
				<div class="flex-1 min-w-0">
					<label class="block text-xs text-[#374d63] mb-1.5 uppercase tracking-wider">Email address</label>
					<div class="relative">
						<span class="absolute left-3 top-1/2 -translate-y-1/2 text-[#374d63] pointer-events-none">
							<Mail size={14} strokeWidth={1.75} />
						</span>
						<input
							type="email"
							placeholder="colleague@company.com"
							bind:value={inviteEmail}
							disabled={inviting}
							onkeydown={e => { if (e.key === 'Enter') handleInvite(); if (e.key === 'Escape') closeInviteForm(); }}
							class="input-base w-full text-sm pl-8 pr-3 py-2.5"
							autofocus
						/>
					</div>
				</div>

				<!-- Role selector -->
				<div class="shrink-0">
					<label class="block text-xs text-[#374d63] mb-1.5 uppercase tracking-wider">Role</label>
					<div class="flex rounded-md overflow-hidden border border-border-deep">
						{#each inviteRoleOptions as role}
							<button
								type="button"
								onclick={() => (inviteRole = role)}
								class="px-3 py-2.5 text-sm font-mono capitalize border-r border-border-deep last:border-r-0 transition-colors duration-100 cursor-pointer
									{inviteRole === role
										? 'bg-[#0f2d4a] text-[#93c5fd] border-b-2 border-b-[#3b82f6]'
										: 'bg-transparent text-[#4b6280] hover:text-[#8899aa] hover:bg-[#0a1929]'}"
							>{role}</button>
						{/each}
					</div>
				</div>
			</div>

			<!-- Role description -->
			<p class="mt-2 mb-0 text-xs text-[#374d63]">
				{#if inviteRole === 'admin'}
					Can manage members, forms, and workspace settings.
				{:else if inviteRole === 'member'}
					Can create and manage their own forms.
				{:else}
					Can view forms and responses, but cannot create or edit.
				{/if}
			</p>

			{#if inviteError}
				<p class="mt-3 mb-0 text-xs text-error-muted">{inviteError}</p>
			{/if}
			{#if inviteSuccess}
				<p class="mt-3 mb-0 text-xs text-[#4ade80]">{inviteSuccess}</p>
			{/if}

			<div class="flex gap-2 mt-4">
				<button
					onclick={handleInvite}
					disabled={inviting || !inviteEmail.trim()}
					class="px-4 py-2 text-white border-none rounded cursor-pointer font-mono text-sm transition-colors duration-100
						{inviting || !inviteEmail.trim() ? 'bg-[#374d63] cursor-not-allowed' : 'bg-primary hover:bg-primary-hover'}"
				>{inviting ? 'Sending…' : 'Send invite'}</button>
				<button
					onclick={closeInviteForm}
					class="px-4 py-2 bg-transparent text-[#4b6280] border border-border-deep rounded cursor-pointer font-mono text-sm hover:text-[#c5d3e0] hover:border-border-subtle transition-colors duration-100"
				>Cancel</button>
			</div>
		</div>
	{/if}

	{#if roleError}
		<div class="mb-4 px-4 py-3 border border-[#7f1d1d] rounded-lg text-sm text-error-muted bg-[#1a0e0e]">
			{roleError}
		</div>
	{/if}

	{#if grantError}
		<div class="mb-4 px-4 py-3 border border-[#7f1d1d] rounded-lg text-sm text-error-muted bg-[#1a0e0e]">
			{grantError}
		</div>
	{/if}

	{#if loading && members.length === 0}
		<p class="text-[#4b6280] text-base">Loading…</p>
	{:else if error}
		<p class="text-error-light text-base">{error}</p>
	{:else if members.length === 0}
		<div class="py-12 border border-dashed border-border rounded-lg text-center">
			<p class="m-0 text-[#374d63] text-base">No members found</p>
		</div>
	{:else}

		<!-- Mobile card list -->
		<div class="flex flex-col gap-2 sm:hidden">
			{#each members as member (member.accountId)}
				<div class="p-4 border border-border-deep rounded-lg">
					<div class="flex items-start justify-between gap-2 mb-2">
						<div class="min-w-0">
							<p class="m-0 text-[#c5d3e0] text-base truncate">{member.username || '—'}</p>
							<p class="m-0 text-[#374d63] text-xs mt-0.5 truncate" title={member.accountId}>
								{member.accountId.slice(0, 16)}…
							</p>
						</div>
						<div class="flex items-center gap-2 shrink-0">
							<span class="inline-flex items-center gap-1 text-xs">
								<span class="w-1.5 h-1.5 rounded-full {member.status === 'active' ? 'bg-[#4ade80]' : 'bg-[#a16207]'}"></span>
								<span class="{member.status === 'active' ? 'text-[#4b6280]' : 'text-[#a16207]'} capitalize">
									{member.status === 'pending' && !identityKeys.has(member.accountId) ? 'awaiting setup' : member.status}
								</span>
							</span>
							<span class="px-2 py-0.5 rounded-full text-xs text-[#4b6280] border border-border-deep capitalize">
								{member.role}
							</span>
						</div>
					</div>
					<p class="m-0 text-[#374d63] text-xs">
						Joined {formatDate(member.joinedAt)}
						{#if member.lastSeen}· Last login {formatDate(member.lastSeen)}{/if}
					</p>
					{#if canManage && member.accountId !== auth.accountId}
						<div class="mt-3 flex gap-2 flex-wrap">
							{#if member.status === 'pending' && identityKeys.has(member.accountId)}
								<button
									onclick={() => handleGrant(member)}
									disabled={grantingId === member.accountId}
									class="px-3 py-1.5 bg-[#0a1f10] text-[#4ade80] border border-[#166534] rounded cursor-pointer font-mono text-xs hover:bg-[#052e16] transition-colors duration-100 disabled:opacity-40 disabled:cursor-not-allowed inline-flex items-center gap-1.5"
								>
									<KeyRound size={11} strokeWidth={1.75} />
									{grantingId === member.accountId ? 'Granting…' : 'Grant access'}
								</button>
							{/if}
							{#each availableRoles(member) as role}
								<button
									onclick={() => handleRoleChange(member, role)}
									disabled={roleLoading}
									class="px-3 py-1.5 bg-transparent text-[#8899aa] border border-border-subtle rounded cursor-pointer font-mono text-xs hover:border-border hover:text-[#c5d3e0] transition-colors duration-100 disabled:opacity-40 disabled:cursor-not-allowed capitalize"
								>Make {role}</button>
							{/each}
							<button
								onclick={() => { removeTarget = member; removeError = ''; }}
								class="px-3 py-1.5 bg-transparent text-error-light border border-border-subtle rounded cursor-pointer font-mono text-xs hover:border-[#7f1d1d] transition-colors duration-100"
							>Remove</button>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		<!-- Desktop table -->
		<div class="hidden sm:block border border-border-deep rounded-lg overflow-visible">
			<table class="w-full border-collapse text-sm">
				<thead>
					<tr class="border-b border-border-deep">
						<th class="text-left px-4 py-3 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs rounded-tl-lg">Member</th>
						<th class="text-left px-4 py-3 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Role</th>
						<th class="text-left px-4 py-3 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Status</th>
						<th class="text-left px-4 py-3 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Last login</th>
						<th class="text-left px-4 py-3 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Joined</th>
						<th class="px-4 py-3 rounded-tr-lg"></th>
					</tr>
				</thead>
				<tbody>
					{#each members as member (member.accountId)}
						<tr class="border-b border-border-deep last:border-b-0 hover:bg-[#0d1926] transition-colors duration-75">

							<!-- Member -->
							<td class="px-4 py-3.5">
								<div class="flex items-center gap-3">
									<span class="shrink-0 w-7 h-7 rounded-md flex items-center justify-center bg-[#0f1e30] border border-[#1e3347] text-[#4b6280] text-xs font-semibold select-none">
										{(member.username || '?').slice(0, 2).toUpperCase()}
									</span>
									<div class="min-w-0">
										<p class="m-0 text-[#c5d3e0] truncate">{member.username || '—'}</p>
										<p class="m-0 text-[#374d63] text-xs mt-0.5 truncate" title={member.accountId}>
											{member.accountId.slice(0, 16)}…
										</p>
									</div>
								</div>
							</td>

							<!-- Role -->
							<td class="px-4 py-3.5">
								<span class="px-2.5 py-0.5 rounded-full text-xs text-[#4b6280] border border-border-deep capitalize">
									{member.role}
								</span>
							</td>

							<!-- Status -->
							<td class="px-4 py-3.5">
								{#if member.status === 'pending' && canManage && member.accountId !== auth.accountId}
									{#if identityKeys.has(member.accountId)}
										<button
											onclick={() => handleGrant(member)}
											disabled={grantingId === member.accountId}
											class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-mono border transition-colors duration-100 cursor-pointer
												{grantingId === member.accountId
													? 'bg-transparent text-[#4b6280] border-border-deep cursor-not-allowed'
													: 'bg-[#0a1f10] text-[#4ade80] border-[#166534] hover:bg-[#052e16]'}"
										>
											<KeyRound size={11} strokeWidth={1.75} />
											{grantingId === member.accountId ? 'Granting…' : 'Grant access'}
										</button>
									{:else}
										<span class="inline-flex items-center gap-1.5 text-xs text-[#a16207]">
											<span class="w-1.5 h-1.5 rounded-full bg-[#a16207]"></span>
											Awaiting setup
										</span>
									{/if}
								{:else}
									<span class="inline-flex items-center gap-1.5 text-xs">
										<span class="w-1.5 h-1.5 rounded-full {member.status === 'active' ? 'bg-[#4ade80]' : 'bg-[#a16207]'}"></span>
										<span class="{member.status === 'active' ? 'text-[#4b6280]' : 'text-[#a16207]'} capitalize">{member.status}</span>
									</span>
								{/if}
							</td>

							<!-- Last login -->
							<td class="px-4 py-3.5 text-[#4b6280]">{formatDate(member.lastSeen)}</td>

							<!-- Joined -->
							<td class="px-4 py-3.5 text-[#4b6280]">{formatDate(member.joinedAt)}</td>

							<!-- Actions -->
							<td class="px-4 py-3.5">
								{#if canManage && member.accountId !== auth.accountId}
									<div class="relative flex justify-end">
										{#if openMenuId === member.accountId}
											<div
												class="fixed inset-0 z-10"
												onclick={() => (openMenuId = null)}
												role="presentation"
											></div>
										{/if}
										<button
											onclick={() => (openMenuId = openMenuId === member.accountId ? null : member.accountId)}
											class="flex items-center justify-center w-7 h-7 bg-transparent border rounded cursor-pointer text-[#374d63] transition-colors duration-100
												{openMenuId === member.accountId
													? 'text-[#c5d3e0] border-border-subtle bg-[#1e2c3d]'
													: 'border-transparent hover:border-border-deep hover:text-[#8899aa]'}"
											aria-label="Member options"
										>
											<MoreHorizontal size={15} strokeWidth={1.75} />
										</button>

										{#if openMenuId === member.accountId}
											<div class="absolute right-0 top-[calc(100%+4px)] z-20 min-w-[180px] bg-[#0b1623] border border-[#1e3347] rounded-lg shadow-[0_8px_24px_rgba(0,0,0,0.6)] overflow-hidden py-1">
												{#each availableRoles(member) as role}
													<button
														onclick={() => handleRoleChange(member, role)}
														disabled={roleLoading}
														class="flex items-center gap-2.5 w-full px-3.5 py-2.5 bg-transparent border-none cursor-pointer font-mono text-sm text-[#8899aa] text-left transition-colors duration-100 hover:bg-[#0f1e2e] hover:text-[#c5d3e0] disabled:opacity-40 disabled:cursor-not-allowed capitalize"
													>
														<ShieldCheck size={13} strokeWidth={1.75} class="shrink-0 text-[#4b6280]" />
														Make {role}
													</button>
												{/each}
												{#if availableRoles(member).length > 0}
													<div class="border-t border-[#1e3347] my-1"></div>
												{/if}
												<button
													onclick={() => { openMenuId = null; removeTarget = member; removeError = ''; }}
													class="flex items-center gap-2.5 w-full px-3.5 py-2.5 bg-transparent border-none cursor-pointer font-mono text-sm text-[#f87171] text-left transition-colors duration-100 hover:bg-[#2d1515]"
												>
													<UserMinus size={13} strokeWidth={1.75} class="shrink-0" />
													Remove member
												</button>
											</div>
										{/if}
									</div>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

	{/if}

	<!-- Pending invitations -->
	{#if canManage && invitations.length > 0}
		<div class="mt-8">
			<h2 class="m-0 mb-3 text-xs font-normal text-[#374d63] tracking-[0.06em] uppercase">
				Pending invitations
			</h2>

			<!-- Mobile -->
			<div class="flex flex-col gap-2 sm:hidden">
				{#each invitations as inv (inv.id)}
					<div class="flex items-center justify-between gap-3 px-4 py-3 border border-border-deep rounded-lg">
						<div class="min-w-0">
							<p class="m-0 text-sm text-[#c5d3e0] truncate">{inv.email}</p>
							<p class="m-0 text-xs text-[#374d63] mt-0.5 capitalize">{inv.role} · {formatExpiry(inv.expiresAt)}</p>
						</div>
						<button
							onclick={() => handleRevoke(inv)}
							disabled={revokingId === inv.id}
							class="shrink-0 px-3 py-1.5 bg-transparent text-[#4b6280] border border-border-deep rounded cursor-pointer font-mono text-xs hover:text-error-light hover:border-[#7f1d1d] transition-colors duration-100 disabled:opacity-40 disabled:cursor-not-allowed"
						>{revokingId === inv.id ? 'Revoking…' : 'Revoke'}</button>
					</div>
				{/each}
			</div>

			<!-- Desktop -->
			<div class="hidden sm:block border border-border-deep rounded-lg">
				<table class="w-full border-collapse text-sm">
					<thead>
						<tr class="border-b border-border-deep">
							<th class="text-left px-4 py-2.5 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Email</th>
							<th class="text-left px-4 py-2.5 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Role</th>
							<th class="text-left px-4 py-2.5 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Sent</th>
							<th class="text-left px-4 py-2.5 font-normal text-[#374d63] tracking-[0.06em] uppercase text-xs">Expiry</th>
							<th class="px-4 py-2.5"></th>
						</tr>
					</thead>
					<tbody>
						{#each invitations as inv (inv.id)}
							<tr class="border-b border-border-deep last:border-b-0">
								<td class="px-4 py-3 text-[#c5d3e0]">{inv.email}</td>
								<td class="px-4 py-3">
									<span class="px-2.5 py-0.5 rounded-full text-xs text-[#4b6280] border border-border-deep capitalize">{inv.role}</span>
								</td>
								<td class="px-4 py-3 text-[#4b6280]">{formatDate(inv.createdAt)}</td>
								<td class="px-4 py-3 text-[#4b6280] text-xs">{formatExpiry(inv.expiresAt)}</td>
								<td class="px-4 py-3 text-right">
									<button
										onclick={() => handleRevoke(inv)}
										disabled={revokingId === inv.id}
										class="px-3 py-1.5 bg-transparent text-[#4b6280] border border-border-deep rounded cursor-pointer font-mono text-xs hover:text-error-light hover:border-[#7f1d1d] transition-colors duration-100 disabled:opacity-40 disabled:cursor-not-allowed"
									>{revokingId === inv.id ? 'Revoking…' : 'Revoke'}</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}

</div>
</div>
