/**
 * AAD migration helpers.
 *
 * Pre-AAD blobs decrypt with no additionalData (legacy path).
 * Post-AAD blobs decrypt with additionalData = encode(contextId).
 *
 * Migration strategy: try with AAD → if DOMException, retry without →
 * if successful, re-encrypt with AAD and persist.
 *
 * Scope of this migration:
 *   ✓ encryptedSchema          — re-encrypted via PUT /api/forms/{id}
 *   ✓ workspaceWrappedFormKey  — re-encrypted via PUT /api/forms/{id}/workspace-form-key
 *   ✗ renderEncryptedSchema    — needs a dedicated backend endpoint (no status-neutral update)
 *   ✗ responses                — append-only, no update endpoint
 *   ✗ workspace key blobs      — separate migration; loadWorkspaceKey falls back gracefully
 */

import {
	decryptSchema,
	deriveFormKey,
	encryptSchema,
	unwrapFormKey,
	wrapFormKey,
} from "$lib/crypto";
import type { FormRecord } from "$lib/forms";
import { loadWorkspaceKey } from "$lib/workspaces";

const enc = new TextEncoder();
const aad = (id: string) => enc.encode(id);

export interface MigrationResult {
	migrated: number;
	alreadyCurrent: number;
	failed: Array<{ formId: string; error: string }>;
}

/**
 * Attempt decryption with AAD. If that fails (legacy blob), fall back to
 * no-AAD decryption. Returns the plaintext and whether a fallback was used.
 */
async function tryDecryptSchema(
	blob: ArrayBuffer,
	key: CryptoKey,
	contextId: string,
): Promise<{ schema: object; wasLegacy: boolean }> {
	try {
		const schema = await decryptSchema(blob, key, aad(contextId));
		return { schema, wasLegacy: false };
	} catch {
		// Legacy blob — no AAD was used at encrypt time
		const schema = await decryptSchema(blob, key);
		return { schema, wasLegacy: true };
	}
}

async function tryUnwrapFormKey(
	blob: ArrayBuffer,
	workspaceKey: CryptoKey,
	formId: string,
): Promise<{ key: CryptoKey; wasLegacy: boolean }> {
	try {
		const key = await unwrapFormKey(blob, workspaceKey, aad(formId));
		return { key, wasLegacy: false };
	} catch {
		const key = await unwrapFormKey(blob, workspaceKey);
		return { key, wasLegacy: true };
	}
}

async function apiGet<T>(path: string): Promise<T> {
	const res = await fetch(path, { credentials: "include" });
	if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
	return res.json() as Promise<T>;
}

async function apiPut(
	path: string,
	body: Record<string, string>,
): Promise<void> {
	const res = await fetch(path, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		credentials: "include",
		body: JSON.stringify(body),
	});
	if (!res.ok) throw new Error(`PUT ${path} → ${res.status}`);
}

const MIGRATION_DONE_KEY = "confide:aad-migration-v1:done";

/**
 * Returns true if this account has already completed the AAD migration.
 * Keyed by accountId so multi-account scenarios work correctly.
 */
export function isAadMigrationDone(accountId: string): boolean {
	return localStorage.getItem(`${MIGRATION_DONE_KEY}:${accountId}`) === "1";
}

/**
 * Migrate all form blobs for the authenticated account to use AAD.
 *
 * Checks the localStorage gate first — if migration was already completed for
 * this account, returns immediately with zero API calls. Once all blobs are
 * current, marks the migration as done so future logins skip it entirely.
 */
export async function migrateFormBlobsToAAD(
	masterKey: CryptoKey,
	accountId: string,
): Promise<MigrationResult> {
	if (isAadMigrationDone(accountId)) {
		return { migrated: 0, alreadyCurrent: 0, failed: [] };
	}

	const result: MigrationResult = {
		migrated: 0,
		alreadyCurrent: 0,
		failed: [],
	};

	const { forms } = await apiGet<{ forms: Array<{ formId: string }> }>(
		"/api/forms",
	);

	await Promise.allSettled(
		forms.map(async ({ formId }) => {
			try {
				await migrateForm(formId, masterKey, result);
			} catch (err) {
				result.failed.push({ formId, error: String(err) });
			}
		}),
	);

	// Mark done only when every blob is current and nothing failed.
	// On partial failure, the next login will retry the remaining forms.
	if (result.migrated === 0 && result.failed.length === 0) {
		localStorage.setItem(`${MIGRATION_DONE_KEY}:${accountId}`, "1");
	}

	return result;
}

async function migrateForm(
	formId: string,
	masterKey: CryptoKey,
	result: MigrationResult,
): Promise<void> {
	const record = await apiGet<FormRecord>(`/api/forms/${formId}`);
	let needsPersist = false;
	const updates: Record<string, string> = {};

	// ── Resolve form key ──────────────────────────────────────────────────────

	let formKey: CryptoKey;
	let wsKey: CryptoKey | undefined;

	try {
		formKey = await deriveFormKey(masterKey, formId);
		// Probe: try to decrypt with AAD to confirm key is correct
		await decryptSchema(
			base64ToBuffer(record.encryptedSchema),
			formKey,
			aad(formId),
		);
		// Decryption with AAD succeeded — schema already migrated
	} catch {
		// Either not the creator or schema lacks AAD — try workspace path first,
		// then fall back to creator key with legacy decryption
		if (record.workspaceId && record.workspaceWrappedFormKey) {
			wsKey = await loadWorkspaceKey(record.workspaceId, masterKey);
		}

		if (wsKey) {
			formKey = (
				await tryUnwrapFormKey(
					base64ToBuffer(record.workspaceWrappedFormKey!),
					wsKey,
					formId,
				)
			).key;
		} else {
			formKey = await deriveFormKey(masterKey, formId);
		}
	}

	// ── encryptedSchema ───────────────────────────────────────────────────────

	const { schema, wasLegacy: schemaLegacy } = await tryDecryptSchema(
		base64ToBuffer(record.encryptedSchema),
		formKey,
		formId,
	);

	if (schemaLegacy) {
		updates.encryptedSchema = bufferToBase64(
			await encryptSchema(schema as never, formKey, aad(formId)),
		);
		needsPersist = true;
	}

	// ── workspaceWrappedFormKey ───────────────────────────────────────────────

	if (record.workspaceWrappedFormKey && wsKey) {
		const { wasLegacy: wfkLegacy } = await tryUnwrapFormKey(
			base64ToBuffer(record.workspaceWrappedFormKey),
			wsKey,
			formId,
		);
		if (wfkLegacy) {
			const rewrapped = await wrapFormKey(formKey, wsKey, aad(formId));
			updates.workspaceWrappedFormKey = bufferToBase64(rewrapped);
			needsPersist = true;
		}
	}

	if (!needsPersist) {
		result.alreadyCurrent++;
		return;
	}

	// ── Persist ───────────────────────────────────────────────────────────────

	if (updates.encryptedSchema) {
		await apiPut(`/api/forms/${formId}`, {
			encryptedSchema: updates.encryptedSchema,
		});
	}
	if (updates.workspaceWrappedFormKey) {
		await apiPut(`/api/forms/${formId}/workspace-form-key`, {
			workspaceWrappedFormKey: updates.workspaceWrappedFormKey,
		});
	}

	result.migrated++;
}

// ── Encoding helpers ──────────────────────────────────────────────────────────

function base64ToBuffer(b64: string): ArrayBuffer {
	const binary = atob(b64);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
	return bytes.buffer;
}

function bufferToBase64(buf: ArrayBuffer): string {
	const bytes = new Uint8Array(buf);
	let binary = "";
	for (const b of bytes) binary += String.fromCharCode(b);
	return btoa(binary);
}
