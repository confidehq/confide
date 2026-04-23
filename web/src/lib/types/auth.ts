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
	challengeKey: string;
	options: unknown; // PublicKeyCredentialRequestOptionsJSON
}

export interface LoginFinishResponse {
	accountId: string;
	wrappedMasterKey: string; // base64 standard
}

export interface ReauthBeginResponse {
	challengeKey: string;
	options: unknown; // PublicKeyCredentialRequestOptionsJSON
}

export interface ReauthFinishResponse {
	accountId: string;
	wrappedMasterKey: string; // base64 standard
	addCredentialToken?: string; // present when purpose == "add-credential"
}

export interface AddCredentialBeginResponse {
	options: unknown; // PublicKeyCredentialCreationOptionsJSON
	prfSalt: string; // base64 standard
}

export interface AddCredentialFinishResponse {
	id: string;
	name: string;
	createdAt: string;
}

export interface CredentialSummary {
	id: string;
	name: string;
	createdAt: string;
	backupEligible: boolean;
	isCurrentSession: boolean;
}

export interface RecoverResponse {
	accountId: string;
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
	credentialId?: string; // base64 standard
	userAgent?: string;
}

export interface ApiError {
	code: string;
	message: string;
}
