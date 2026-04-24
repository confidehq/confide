/**
 * Confide forms cache store (Svelte 5 runes).
 *
 * Workspace-aware: the cache is keyed to the currently-active workspace.
 * Switching workspaces resets the cache and triggers a fresh load.
 */

import { listForms, getForm, deriveShareUrl, type FormSummary } from '$lib/forms';

let _workspaceId = $state<string | null>(null);
let _forms = $state<FormSummary[]>([]);
let _formNames = $state<Map<string, string>>(new Map());
let _formDescriptions = $state<Map<string, string>>(new Map());
let _shareUrls = $state<Map<string, string>>(new Map());
let _loaded = $state(false);
let _loading = $state(false);
let _error = $state('');

export const formsStore = {
	get forms() {
		return _forms;
	},
	get formNames() {
		return _formNames;
	},
	get formDescriptions() {
		return _formDescriptions;
	},
	get shareUrls() {
		return _shareUrls;
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
	get workspaceId() {
		return _workspaceId;
	},

	/** Load forms for a workspace. No-ops if already loaded for the same workspace. */
	async load(masterKey: CryptoKey, workspaceId: string) {
		if (_workspaceId === workspaceId && (_loaded || _loading)) return;

		// New workspace — reset state before loading
		_workspaceId = workspaceId;
		_loaded = false;
		_loading = true;
		_error = '';
		_forms = [];
		_formNames = new Map();
		_formDescriptions = new Map();
		_shareUrls = new Map();

		try {
			const forms = await listForms(workspaceId);
			// Guard against a concurrent workspace switch overtaking this load
			if (_workspaceId !== workspaceId) return;
			_forms = forms;
			_loaded = true;
		} catch (err) {
			if (_workspaceId !== workspaceId) return;
			_error = err instanceof Error ? err.message : 'Failed to load forms';
		} finally {
			if (_workspaceId === workspaceId) _loading = false;
		}

		// Decrypt names and derive share URLs in parallel (best-effort; failures silently ignored)
		if (_loaded && _forms.length > 0 && _workspaceId === workspaceId) {
			const snap = _forms;
			const results = await Promise.allSettled(snap.map((f) => getForm(masterKey, f.formId)));
			if (_workspaceId !== workspaceId) return; // stale
			const names = new Map(_formNames);
			const descriptions = new Map(_formDescriptions);
			const urlEntries: Array<Promise<void>> = [];
			results.forEach((r, i) => {
				if (r.status === 'fulfilled') {
					const { schema, record, formKey } = r.value;
					const t = schema.translations[schema.defaultLocale];
					const name = schema.name || t?.formTitle;
					if (name) names.set(snap[i].formId, name);
					const desc = t?.formDescription;
					if (desc) descriptions.set(snap[i].formId, desc);
					if (record.renderKeySalt && snap[i].status !== 'draft') {
						urlEntries.push(
							deriveShareUrl(snap[i].formId, record.renderKeySalt, formKey).then(url => {
								_shareUrls = new Map([..._shareUrls, [snap[i].formId, url]]);
							}).catch(() => {})
						);
					}
				}
			});
			_formNames = names;
			_formDescriptions = descriptions;
			await Promise.allSettled(urlEntries);
		}
	},

	/** Mark cache stale so the next load() re-fetches. */
	invalidate() {
		_loaded = false;
		_formNames = new Map();
		_formDescriptions = new Map();
		_shareUrls = new Map();
	},

	updateName(formId: string, name: string) {
		const names = new Map(_formNames);
		names.set(formId, name);
		_formNames = names;
	},

	updateStatus(formId: string, status: 'draft' | 'open' | 'closed') {
		_forms = _forms.map((f) => (f.formId === formId ? { ...f, status } : f));
	},

	remove(formId: string) {
		_forms = _forms.filter((f) => f.formId !== formId);
		const names = new Map(_formNames);
		names.delete(formId);
		_formNames = names;
		const descs = new Map(_formDescriptions);
		descs.delete(formId);
		_formDescriptions = descs;
		const urls = new Map(_shareUrls);
		urls.delete(formId);
		_shareUrls = urls;
	},

	/** Wipe everything — must be called when master key is cleared. */
	clear() {
		_workspaceId = null;
		_forms = [];
		_formNames = new Map();
		_formDescriptions = new Map();
		_shareUrls = new Map();
		_loaded = false;
		_loading = false;
		_error = '';
	}
};
