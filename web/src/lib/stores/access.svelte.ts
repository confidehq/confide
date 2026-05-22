/**
 * Access store — single source of truth for role, plan, and feature-flag checks.
 *
 * Mirrors the Go permission package: role checks gate actions, plan checks gate
 * features. Components read from this store instead of inlining string comparisons.
 */

import { workspacesStore } from './workspaces.svelte';
import { getAppConfig } from '$lib/config';

// Feature names match Go's permission.Feature constants exactly.
export type Feature =
	| 'custom_styles'
	| 'whitelabel'
	| 'custom_domains'
	| 'advanced_analytics'
	| 'partial_submissions'
	| 'version_history'
	| 'extended_email_forwarding';

const PRO_FEATURES = new Set<Feature>([
	'custom_styles',
	'whitelabel',
	'custom_domains',
	'advanced_analytics',
	'partial_submissions',
	'version_history',
	'extended_email_forwarding',
]);

let _edition = $state('');

// Fetch once — getAppConfig is cached after the first call.
getAppConfig().then((c) => {
	_edition = c.edition;
});

export const access = {
	// ── Edition ───────────────────────────────────────────────────────────────

	/** True when running as the managed cloud service (Stripe billing active). */
	get managed(): boolean {
		return _edition !== 'community';
	},

	// ── Plan ──────────────────────────────────────────────────────────────────

	/**
	 * True when the active workspace has Pro-tier features available.
	 * This is true for Pro plan workspaces AND for any workspace on a
	 * community (self-hosted) instance, where all features are unlocked.
	 */
	get isPro(): boolean {
		return workspacesStore.active?.plan === 'pro' || _edition === 'community';
	},

	// ── Role ──────────────────────────────────────────────────────────────────

	/** True when the current user is the workspace owner. */
	get isOwner(): boolean {
		return workspacesStore.active?.role === 'owner';
	},

	/**
	 * True when the current user is an admin or owner.
	 * Matches Go's ActionInviteMembers / ActionChangeRoles minimum rank.
	 */
	get isAdmin(): boolean {
		const role = workspacesStore.active?.role;
		return role === 'owner' || role === 'admin';
	},

	/**
	 * True when the current user can create and edit forms.
	 * Matches Go's ActionManageForms minimum rank (member and above).
	 */
	get canManageForms(): boolean {
		const role = workspacesStore.active?.role;
		return role === 'owner' || role === 'admin' || role === 'member';
	},

	// ── Feature flags ─────────────────────────────────────────────────────────

	/**
	 * Returns true when the active workspace may use the given Pro feature.
	 * Mirrors Go's permission.PlanAllows — always true for non-gated features.
	 */
	can(feature: Feature): boolean {
		if (!PRO_FEATURES.has(feature)) return true;
		return this.isPro;
	},

	// ── Workspace creation ────────────────────────────────────────────────────

	/**
	 * True when the account has reached the workspace limit and creating another
	 * would require a Pro upgrade. Always false in community mode.
	 */
	get atWorkspaceLimit(): boolean {
		if (!this.managed) return false;
		return workspacesStore.workspaces.some(
			(w) => w.plan === 'free' && w.role === 'owner'
		);
	},
};
