/** API response and error types for the auth flow. */

export interface RegisterBeginResponse {
	accountId: string;
	prfSalt: string; // base64 standard
	options: unknown; // PublicKeyCredentialCreationOptionsJSON
}

export interface RegisterFinishResponse {
	accountId: string;
}

export interface LoginBeginResponse {
	credentialIdBase64: string;
	options: unknown; // PublicKeyCredentialRequestOptionsJSON
}

export interface LoginFinishResponse {
	accountId: string;
	wrappedMasterKey: string; // base64 standard
}

export interface RecoverResponse {
	recoveryWrappedMasterKey: string; // base64 standard
	rekeyToken: string;
}

export interface RekeyBeginResponse {
	options: unknown; // PublicKeyCredentialCreationOptionsJSON (go-webauthn wrapper)
	prfSalt: string; // base64 standard — must be sent back in rekeyFinish
}

export interface RekeyFinishResponse {
	credentialIdBase64: string; // base64 standard
}

export interface SessionInfo {
	id: string;
	createdAt: string; // YYYY-MM-DD
	lastSeen: string; // YYYY-MM-DD
}

export interface ApiError {
	code: string;
	message: string;
}
