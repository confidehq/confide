/**
 * Confide — WebAuthn PRF extension support detection (DQ1 resolution)
 *
 * PRF support cannot be fully determined statically — the extension is only
 * confirmed during an actual WebAuthn ceremony (registration or assertion).
 * This module provides static pre-checks and surfaces ceremony-time errors.
 *
 * Supported browsers: Chrome/Edge 116+, Safari 17+, Firefox 119+
 */

export interface PRFSupportResult {
	supported: boolean;
	webAuthnSupported: boolean;
	platformAuthenticatorAvailable: boolean;
	reason: string | null; // human-readable, shown in UI
}

/**
 * Perform static pre-checks for PRF support.
 *
 * Detection layers:
 *   1. Static: PublicKeyCredential API availability
 *   2. Platform authenticator: isUserVerifyingPlatformAuthenticatorAvailable()
 *   3. PRF extension: only determinable during a WebAuthn ceremony
 *      (detected at signup; if PRF output is absent post-ceremony, surface
 *       an explicit error)
 *
 * Layer 3 is NOT checked here — it requires a user gesture and a real
 * authenticator. Call surfacePrfError() if the ceremony completes without
 * PRF output.
 */
export async function detectPRFSupport(): Promise<PRFSupportResult> {
	// Layer 1: WebAuthn API availability
	const webAuthnSupported =
		typeof window !== 'undefined' &&
		typeof window.PublicKeyCredential !== 'undefined';

	if (!webAuthnSupported) {
		return {
			supported: false,
			webAuthnSupported: false,
			platformAuthenticatorAvailable: false,
			reason:
				'WebAuthn is not supported in this browser. ' +
				'Please use Chrome/Edge 116+, Safari 17+, or Firefox 119+.'
		};
	}

	// Layer 2: platform authenticator availability
	let platformAuthenticatorAvailable = false;
	try {
		platformAuthenticatorAvailable =
			await PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable();
	} catch {
		// API exists but threw — treat as unavailable
		platformAuthenticatorAvailable = false;
	}

	if (!platformAuthenticatorAvailable) {
		return {
			supported: false,
			webAuthnSupported: true,
			platformAuthenticatorAvailable: false,
			reason:
				'No platform authenticator (Touch ID, Face ID, Windows Hello, etc.) ' +
				'was detected. Confide requires a built-in authenticator for secure key derivation.'
		};
	}

	// Layer 3 check deferred to ceremony time.
	// Return optimistic result — actual PRF support confirmed during signup.
	return {
		supported: true,
		webAuthnSupported: true,
		platformAuthenticatorAvailable: true,
		reason: null
	};
}

/**
 * Surface a ceremony-time PRF failure.
 *
 * Call this when a WebAuthn ceremony completed but the PRF extension output
 * was absent. This is the Layer 3 detection path.
 *
 * @returns A PRFSupportResult with supported=false and a user-facing message.
 */
export function surfacePrfError(): PRFSupportResult {
	return {
		supported: false,
		webAuthnSupported: true,
		platformAuthenticatorAvailable: true,
		reason:
			'Your browser or authenticator does not support WebAuthn PRF. ' +
			'Please use Chrome/Edge 116+, Safari 17+, or Firefox 119+ with a compatible authenticator.'
	};
}
