/**
 * Base64 encoding/decoding utilities.
 *
 * All decode functions return Uint8Array.
 * All encode functions accept Uint8Array.
 * ArrayBuffer callers: wrap input in `new Uint8Array(buf)` or use the
 * `Buf` variants which do that for you.
 *
 * Never uses the spread operator (String.fromCharCode(...bytes)) — that
 * overflows the call stack for large inputs. Always uses a for-of loop.
 */

// ── Standard base64 (RFC 4648 §4, padding, + /) ──────────────────────────────

export function bytesToBase64(bytes: Uint8Array): string {
	let binary = '';
	for (const b of bytes) binary += String.fromCharCode(b);
	return btoa(binary);
}

export function base64ToBytes(b64: string): Uint8Array {
	const binary = atob(b64);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
	return bytes;
}

// ── Base64url (RFC 4648 §5, no padding, - _) ─────────────────────────────────

export function bytesToBase64url(bytes: Uint8Array): string {
	return bytesToBase64(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64urlToBytes(b64url: string): Uint8Array {
	const pad = (4 - (b64url.length % 4)) % 4;
	const b64 = b64url.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat(pad);
	return base64ToBytes(b64);
}

// ── ArrayBuffer convenience variants ──────────────────────────────────────────

export function bufToBase64(buf: ArrayBuffer): string {
	return bytesToBase64(new Uint8Array(buf));
}

export function base64ToBuf(b64: string): ArrayBuffer {
	return base64ToBytes(b64).buffer as ArrayBuffer;
}

export function bufToBase64url(buf: ArrayBuffer): string {
	return bytesToBase64url(new Uint8Array(buf));
}

export function base64urlToBuf(b64url: string): ArrayBuffer {
	return base64urlToBytes(b64url).buffer as ArrayBuffer;
}

// ── Random ────────────────────────────────────────────────────────────────────

export function randomBase64url(byteCount: number): string {
	return bufToBase64url(crypto.getRandomValues(new Uint8Array(byteCount)).buffer as ArrayBuffer);
}
