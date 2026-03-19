/**
 * GhostForm crypto layer — type definitions.
 *
 * FieldConfig is intentionally Record<string, unknown>.
 * The crypto layer treats form content as opaque bytes and must not be
 * coupled to the domain model.
 */

export interface Field {
	id: string;
	type: string;
	config: Record<string, unknown>;
}

export interface FormSchema {
	version: number;
	defaultLocale: string;
	locales: string[];
	layout: 'scroll' | 'steps' | 'convo';
	convoAllowEdit?: boolean;
	fields: Field[];
	translations: Record<string, Record<string, unknown>>;
}

export interface ResponsePayload {
	submittedAt: string; // ISO 8601, client-generated
	locale: string;
	answers: Record<string, string | string[] | number | null>;
}

export interface EncryptedResponse {
	encryptedData: ArrayBuffer;
	ephemeralPublicKey: ArrayBuffer; // raw X25519, 32 bytes
}
