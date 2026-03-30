/**
 * Confide auth state store (Svelte 5 runes).
 *
 * masterKey lives only in memory — lost on tab close / refresh.
 * accountId and credentialId persist in localStorage.
 */

import { formsStore } from './forms.svelte';

const ACCOUNT_ID_KEY = 'confide.accountId';
const CREDENTIAL_ID_KEY = 'confide.credentialId';

function readStorage(key: string): string | null {
	if (typeof localStorage === 'undefined') return null;
	return localStorage.getItem(key);
}

let _masterKey = $state<CryptoKey | null>(null);
let _accountId = $state<string | null>(readStorage(ACCOUNT_ID_KEY));
let _credentialId = $state<string | null>(readStorage(CREDENTIAL_ID_KEY));

export const auth = {
	get masterKey() {
		return _masterKey;
	},
	get accountId() {
		return _accountId;
	},
	/** credentialId is a Base64URLString (from WebAuthn). */
	get credentialId() {
		return _credentialId;
	},
	get hasStoredCredential() {
		return _credentialId !== null;
	},

	setSession(masterKey: CryptoKey, accountId: string, credentialId: string) {
		_masterKey = masterKey;
		_accountId = accountId;
		_credentialId = credentialId;
		localStorage.setItem(ACCOUNT_ID_KEY, accountId);
		localStorage.setItem(CREDENTIAL_ID_KEY, credentialId);
	},

	clearMasterKey() {
		_masterKey = null;
		formsStore.clear();
	},

	updateCredentialId(credentialId: string) {
		_credentialId = credentialId;
		localStorage.setItem(CREDENTIAL_ID_KEY, credentialId);
	},

	clearAll() {
		_masterKey = null;
		_accountId = null;
		_credentialId = null;
		localStorage.removeItem(ACCOUNT_ID_KEY);
		localStorage.removeItem(CREDENTIAL_ID_KEY);
		formsStore.clear();
	}
};
