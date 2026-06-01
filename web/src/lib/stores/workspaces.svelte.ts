/**
 * Confide workspace store (Svelte 5 runes).
 *
 * Tracks the list of workspaces and the currently-active one.
 * Active workspace ID is persisted in localStorage so it survives refreshes.
 */

import { listWorkspaces, type Workspace } from "$lib/workspaces";

const ACTIVE_WS_KEY = "confide.activeWorkspaceId";

function readStoredId(): string | null {
	if (typeof localStorage === "undefined") return null;
	return localStorage.getItem(ACTIVE_WS_KEY);
}

function persistId(id: string | null) {
	if (typeof localStorage === "undefined") return;
	if (id) localStorage.setItem(ACTIVE_WS_KEY, id);
	else localStorage.removeItem(ACTIVE_WS_KEY);
}

let _workspaces = $state<Workspace[]>([]);
let _activeId = $state<string | null>(null);
let _loaded = $state(false);
let _loading = $state(false);
let _error = $state("");

export const workspacesStore = {
	get workspaces() {
		return _workspaces;
	},
	get active(): Workspace | null {
		if (_activeId) {
			const found = _workspaces.find((w) => w.id === _activeId);
			if (found) return found;
		}
		return _workspaces[0] ?? null;
	},
	get loaded() {
		return _loaded;
	},
	get loading() {
		return _loading;
	},
	get error() {
		return _error;
	},

	switchTo(id: string) {
		_activeId = id;
		persistId(id);
	},

	async load() {
		if (_loaded || _loading) return;
		_loading = true;
		_error = "";
		try {
			_workspaces = await listWorkspaces();
			_loaded = true;

			// Restore or default the active workspace
			const stored = readStoredId();
			if (stored && _workspaces.find((w) => w.id === stored)) {
				_activeId = stored;
			} else if (_workspaces.length > 0) {
				_activeId = _workspaces[0].id;
				persistId(_activeId);
			}
		} catch (err) {
			_error = err instanceof Error ? err.message : "Failed to load workspaces";
		} finally {
			_loading = false;
		}
	},

	/** Call after successfully creating a workspace — adds it and switches to it. */
	add(ws: Workspace) {
		_workspaces = [..._workspaces, ws];
		_activeId = ws.id;
		persistId(ws.id);
	},

	/** Update fields on a workspace in the list (e.g. after rename). */
	update(id: string, changes: Partial<Workspace>) {
		_workspaces = _workspaces.map((w) =>
			w.id === id ? { ...w, ...changes } : w,
		);
	},

	/** Call after deleting a workspace. */
	remove(id: string) {
		_workspaces = _workspaces.filter((w) => w.id !== id);
		if (_activeId === id) {
			const next = _workspaces[0]?.id ?? null;
			_activeId = next;
			persistId(next);
		}
	},

	/** Wipe everything — call on logout. */
	clear() {
		_workspaces = [];
		_activeId = null;
		_loaded = false;
		_loading = false;
		_error = "";
		persistId(null);
	},
};
