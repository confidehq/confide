/**
 * Confide crypto layer — type definitions.
 *
 * The crypto layer treats form content as opaque bytes and must not be
 * coupled to the domain model. BuilderSchema is re-exported as FormSchema
 * so callers use the strongly-typed form; the crypto functions remain agnostic.
 */

export type { BuilderSchema as FormSchema } from './builder';

export interface Field {
	id: string;
	type: string;
	config: Record<string, unknown>;
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
