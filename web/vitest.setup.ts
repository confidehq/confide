// Smoke test: verify Node Web Crypto is available
// Node 19+ ships FIPS-validated Web Crypto with X25519 support.
import { describe, it, expect } from 'vitest';

describe('Web Crypto availability', () => {
	it('crypto.subtle is available', () => {
		expect(typeof crypto).toBe('object');
		expect(typeof crypto.subtle).toBe('object');
	});

	it('can run SHA-256 digest', async () => {
		const result = await crypto.subtle.digest('SHA-256', new Uint8Array(0));
		expect(result.byteLength).toBe(32);
	});

	it('supports X25519', async () => {
		const kp = await crypto.subtle.generateKey({ name: 'X25519' }, true, ['deriveKey', 'deriveBits']);
		expect(kp.privateKey).toBeDefined();
		expect(kp.publicKey).toBeDefined();
	});
});
