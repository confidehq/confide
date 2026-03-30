/**
 * Confide forms cache store (Svelte 5 runes).
 *
 * Caches the decrypted forms list in memory so navigating back to /forms
 * is instant. Cache is invalidated on create/delete and cleared whenever
 * the master key is cleared (zero-knowledge constraint: decrypted data
 * must never outlive the in-memory key).
 */

import { listForms, getForm, type FormSummary } from '$lib/forms';

let _forms = $state<FormSummary[]>([]);
let _formNames = $state<Map<string, string>>(new Map());
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
	get loaded() {
		return _loaded;
	},
	get loading() {
		return _loading;
	},
	get error() {
		return _error;
	},

	/** Load forms list and decrypt names. No-ops if already loaded. */
	async load(masterKey: CryptoKey) {
		if (_loaded || _loading) return;
		_loading = true;
		_error = '';
		try {
			_forms = await listForms();
			_loaded = true;
		} catch (err) {
			_error = err instanceof Error ? err.message : 'Failed to load forms';
		} finally {
			_loading = false;
		}

		// Decrypt names in parallel (best-effort; failures silently ignored)
		if (_loaded && masterKey && _forms.length > 0) {
			const results = await Promise.allSettled(_forms.map((f) => getForm(masterKey, f.formId)));
			const names = new Map(_formNames);
			results.forEach((r, i) => {
				if (r.status === 'fulfilled') {
					const { schema } = r.value;
					const name = schema.name || schema.translations[schema.defaultLocale]?.formTitle;
					if (name) names.set(_forms[i].formId, name);
				}
			});
			_formNames = names;
		}
	},

	/** Mark cache stale so the next load() re-fetches. */
	invalidate() {
		_loaded = false;
		_formNames = new Map();
	},

	updateStatus(formId: string, status: 'open' | 'closed') {
		_forms = _forms.map((f) => (f.formId === formId ? { ...f, status } : f));
	},

	remove(formId: string) {
		_forms = _forms.filter((f) => f.formId !== formId);
		const names = new Map(_formNames);
		names.delete(formId);
		_formNames = names;
	},

	/** Wipe everything — must be called when master key is cleared. */
	clear() {
		_forms = [];
		_formNames = new Map();
		_loaded = false;
		_loading = false;
		_error = '';
	}
};
