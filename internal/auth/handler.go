package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	mw "github.com/phantompunk/confide/internal/middleware"
)

// SubscriptionCanceller is implemented by the billing service.
type SubscriptionCanceller interface {
	CancelSubscription(ctx context.Context, subscriptionID string) error
}

// Handler builds the /auth sub-router. recoveryHMACKey is used to apply a
// stricter rate limit to the /recover endpoint.


const sessionCookieName = "session"

func Handler(svc *Service, billing SubscriptionCanceller, recoveryHMACKey []byte, dev bool, registrationOpen bool) http.Handler {
	r := chi.NewRouter()

	r.With(mw.UsernameCheckRateLimit(recoveryHMACKey)).Get("/check-username", checkUsername(svc))
	r.Post("/register/begin", registerBegin(svc, registrationOpen))
	r.Post("/register/finish", registerFinish(svc, dev))
	r.Post("/login/begin", loginBegin(svc))
	r.Post("/login/finish", loginFinish(svc, dev))
	r.With(mw.RecoveryRateLimit(recoveryHMACKey)).Post("/recover", recover_(svc))
	r.Post("/recover/rekey/begin", rekeyBegin(svc))
	r.Post("/recover/rekey/finish", rekeyFinish(svc, dev))

	// Pairing — unauthenticated endpoints (order matters: specific before wildcard).
	r.Get("/pairing/code/{code}", pairingByCode(svc))
	r.Get("/pairing/{token}", pairingPoll(svc))
	r.Post("/pairing/{token}/request", pairingRequest(svc))
	r.Post("/pairing/{token}/complete", pairingComplete(svc, dev))

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(mw.Authenticator(svc))
		r.Get("/me", getMe(svc))
		r.Post("/logout", logout(svc))
		r.Get("/sessions", listSessions(svc))
		r.Delete("/sessions", deleteOtherSessions(svc))
		r.Delete("/sessions/{id}", deleteSession(svc))
		r.Post("/reauth/begin", reauthBegin(svc))
		r.Post("/reauth/finish", reauthFinish(svc))
		r.Post("/credentials/add/begin", addCredentialBegin(svc))
		r.Post("/credentials/add/finish", addCredentialFinish(svc))
		r.Get("/credentials", listCredentials(svc))
		r.Patch("/credentials/{id}", renameCredential(svc))
		r.Delete("/credentials/{id}", deleteCredential(svc))
		r.Post("/recovery-code/rotate", rotateRecoveryCodes(svc))
		r.Delete("/account", deleteAccount(svc, billing))

		// Pairing — authenticated endpoints.
		r.Post("/pairing", createPairing(svc))
		r.Post("/pairing/{token}/fulfill", pairingFulfill(svc))
	})

	return r
}

// ─── Username availability ────────────────────────────────────────────────────

func checkUsername(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		if username == "" {
			writeError(w, http.StatusBadRequest, "missing_param", "username required")
			return
		}
		_, err := svc.db.GetAccountByUsername(r.Context(), pgtype.Text{String: username, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusOK, map[string]bool{"available": true})
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"available": false})
	}
}

// ─── Register ─────────────────────────────────────────────────────────────────

func registerBegin(svc *Service, registrationOpen bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !registrationOpen {
			writeError(w, http.StatusForbidden, "registration_closed", "registration is closed")
			return
		}
		var req struct {
			Username string `json:"username"`
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req) //nolint:errcheck
		res, err := svc.RegisterBegin(r.Context(), req.Username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "register_begin_failed", safeErr(err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"accountId": res.AccountID,
			"prfSalt":   base64.StdEncoding.EncodeToString(res.PRFSalt),
			"options":   res.Creation,
		})
	}
}

func registerFinish(svc *Service, dev bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var req struct {
			AccountID             string          `json:"accountId"`
			Username              string          `json:"username"`
			Name                  string          `json:"name"`
			WrappedMasterKey      string          `json:"wrappedMasterKey"`
			RecoveryWrappedMaster string          `json:"recoveryWrappedMasterKey"`
			RecoveryVerifier      string          `json:"recoveryVerifier"`
			RecoveryCodes         []string        `json:"recoveryCodes"`
			PRFSalt               string          `json:"prfSalt"`
			Credential            json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		wmk, err := base64.StdEncoding.DecodeString(req.WrappedMasterKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedMasterKey must be base64")
			return
		}
		rwmk, err := base64.StdEncoding.DecodeString(req.RecoveryWrappedMaster)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "recoveryWrappedMasterKey must be base64")
			return
		}
		rv, err := base64.StdEncoding.DecodeString(req.RecoveryVerifier)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "recoveryVerifier must be base64")
			return
		}
		prfSalt, err := base64.StdEncoding.DecodeString(req.PRFSalt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "prfSalt must be base64")
			return
		}

		if len(req.RecoveryCodes) != 12 {
			writeError(w, http.StatusBadRequest, "invalid_field", "expected 12 recoveryCodes")
			return
		}
		codeHashes := make([][]byte, 12)
		for i, c := range req.RecoveryCodes {
			h, err := base64.StdEncoding.DecodeString(c)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "recoveryCodes must be base64")
				return
			}
			codeHashes[i] = h
		}

		// Reconstruct request with the credential JSON for FinishRegistration.
		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(req.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		svcReq := &RegisterFinishRequest{
			AccountID:             req.AccountID,
			Username:              req.Username,
			Name:                  req.Name,
			WrappedMasterKey:      wmk,
			RecoveryWrappedMaster: rwmk,
			RecoveryVerifier:      rv,
			RecoveryCodes:         codeHashes,
			PRFSalt:               prfSalt,
		}

		res, err := svc.RegisterFinish(r.Context(), svcReq, r.Header.Get("User-Agent"), newReq)
		if err != nil {
			status := http.StatusInternalServerError
			code := "register_finish_failed"
			if err == ErrDuplicateAccount {
				status = http.StatusConflict
				code = "credential_exists"
			}
			log.Error().Err(err).Msg("register_finish_failed")
			writeError(w, status, code, safeErr(err))
			return
		}

		setSessionCookie(w, res.Token, dev)
		writeJSON(w, http.StatusCreated, map[string]any{"accountId": res.AccountID})
	}
}

// ─── Login ─────────────────────────────────────────────────────────────────────

func loginBegin(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CredentialIDBase64 string `json:"credentialIdBase64"` // optional
			Username           string `json:"username"`           // optional; preferred over credentialIdBase64
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req) //nolint:errcheck // optional body; missing/malformed defaults to discoverable mode

		var credID []byte
		if req.CredentialIDBase64 != "" {
			var err error
			credID, err = base64.StdEncoding.DecodeString(req.CredentialIDBase64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "credentialIdBase64 must be base64")
				return
			}
		}

		res, err := svc.LoginBegin(r.Context(), credID, req.Username)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusUnauthorized
			}
			writeError(w, status, "login_begin_failed", safeErr(err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"challengeKey": res.ChallengeKey,
			"options":      res.Assertion,
		})
	}
}

func loginFinish(svc *Service, dev bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var envelope struct {
			ChallengeKey string          `json:"challengeKey"`
			Credential   json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.ChallengeKey == "" {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		syncBackupEligible(svc, r, envelope.Credential)

		// Reconstruct request containing only the credential payload.
		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(envelope.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		res, err := svc.LoginFinish(r.Context(), envelope.ChallengeKey, r.Header.Get("User-Agent"), newReq)
		if err != nil {
			log.Error().Err(err).Msg("login_finish_failed")
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusUnauthorized
			}
			writeError(w, status, "login_finish_failed", safeErr(err))
			return
		}

		setSessionCookie(w, res.Token, dev)
		writeJSON(w, http.StatusOK, map[string]any{
			"accountId":        res.AccountID,
			"wrappedMasterKey": base64.StdEncoding.EncodeToString(res.WrappedMasterKey),
		})
	}
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

func recover_(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			CodeHash string `json:"codeHash"` // base64 SHA-256 of segments[0], never plaintext
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "username and codeHash required")
			return
		}

		codeHash, err := base64.StdEncoding.DecodeString(req.CodeHash)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "codeHash must be base64-encoded")
			return
		}

		res, err := svc.Recover(r.Context(), req.Username, codeHash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_code", "invalid or expired recovery code")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"accountId":                res.AccountID,
			"recoveryWrappedMasterKey": base64.StdEncoding.EncodeToString(res.RecoveryWrappedMaster),
			"rekeyToken":               res.RekeyToken,
		})
	}
}

// ─── Rekey ────────────────────────────────────────────────────────────────────

func rekeyBegin(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RekeyToken string `json:"rekeyToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RekeyToken == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "rekeyToken required")
			return
		}

		res, err := svc.RekeyBegin(r.Context(), req.RekeyToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "rekey_begin_failed", "invalid or expired rekey token")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"options": res.Creation,
			"prfSalt": base64.StdEncoding.EncodeToString(res.PRFSalt),
		})
	}
}

func rekeyFinish(svc *Service, dev bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var req struct {
			RekeyToken            string          `json:"rekeyToken"`
			PRFSalt               string          `json:"prfSalt"`
			WrappedMasterKey      string          `json:"wrappedMasterKey"`
			RecoveryWrappedMaster string          `json:"recoveryWrappedMasterKey"`
			RecoveryVerifier      string          `json:"recoveryVerifier"`
			RecoveryCodes         []string        `json:"recoveryCodes"`
			Credential            json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		prfSalt, err := base64.StdEncoding.DecodeString(req.PRFSalt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "prfSalt must be base64")
			return
		}
		wmk, err := base64.StdEncoding.DecodeString(req.WrappedMasterKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedMasterKey must be base64")
			return
		}
		rwmk, err := base64.StdEncoding.DecodeString(req.RecoveryWrappedMaster)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "recoveryWrappedMasterKey must be base64")
			return
		}
		rv, err := base64.StdEncoding.DecodeString(req.RecoveryVerifier)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "recoveryVerifier must be base64")
			return
		}
		if len(req.RecoveryCodes) != 12 {
			writeError(w, http.StatusBadRequest, "invalid_field", "expected 12 recoveryCodes")
			return
		}
		codeHashes := make([][]byte, 12)
		for i, c := range req.RecoveryCodes {
			h, err := base64.StdEncoding.DecodeString(c)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "recoveryCodes must be base64")
				return
			}
			codeHashes[i] = h
		}

		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(req.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		svcReq := &RekeyFinishRequest{
			RekeyToken:            req.RekeyToken,
			PRFSalt:               prfSalt,
			WrappedMasterKey:      wmk,
			RecoveryWrappedMaster: rwmk,
			RecoveryVerifier:      rv,
			RecoveryCodes:         codeHashes,
		}

		res, err := svc.RekeyFinish(r.Context(), svcReq, r.Header.Get("User-Agent"), newReq)
		if err != nil {
			status := http.StatusInternalServerError
			code := "rekey_finish_failed"
			if err == ErrNotFound {
				status = http.StatusUnauthorized
				code = "account_not_found"
			}
			writeError(w, status, code, safeErr(err))
			return
		}

		setSessionCookie(w, res.SessionToken, dev)
		writeJSON(w, http.StatusOK, map[string]any{"credentialIdBase64": res.CredentialIDBase64})
	}
}

// ─── Authenticated ─────────────────────────────────────────────────────────────

// ─── Me ──────────────────────────────────────────────────────────────────────

func getMe(svc *Service) http.HandlerFunc {
	type meResponse struct {
		AccountID string `json:"accountId"`
		Username  string `json:"username,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		account, err := svc.db.GetAccountByID(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", safeErr(err))
			return
		}
		resp := meResponse{AccountID: accountID}
		if account.Username.Valid {
			resp.Username = account.Username.String
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func logout(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		sessionID := mw.SessionID(r.Context())
		_ = svc.Logout(r.Context(), accountID, sessionID)
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func listSessions(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		rows, err := svc.ListSessions(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list_sessions_failed", safeErr(err))
			return
		}

		type sessionInfo struct {
			ID           string `json:"id"`
			CreatedAt    string `json:"createdAt"`
			LastSeen     string `json:"lastSeen"`
			CredentialID string `json:"credentialId,omitempty"`
			UserAgent    string `json:"userAgent,omitempty"`
		}
		out := make([]sessionInfo, len(rows))
		for i, s := range rows {
			out[i] = sessionInfo{
				ID:           s.ID,
				CreatedAt:    s.CreatedAt.Time.UTC().Format(time.RFC3339),
				LastSeen:     s.LastSeen.Time.UTC().Format(time.RFC3339),
				CredentialID: base64.StdEncoding.EncodeToString(s.CredentialID),
				UserAgent:    s.UserAgent,
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func deleteOtherSessions(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		sessionID := mw.SessionID(r.Context())

		if err := svc.DeleteOtherSessions(r.Context(), accountID, sessionID); err != nil {
			writeError(w, http.StatusInternalServerError, "delete_sessions_failed", safeErr(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteSession(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		sessionID := chi.URLParam(r, "id")

		if err := svc.DeleteSession(r.Context(), accountID, sessionID); err != nil {
			if err == ErrNotFound {
				writeError(w, http.StatusNotFound, "session_not_found", "session not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "delete_session_failed", safeErr(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Reauth ────────────────────────────────────────────────────────────────────

func reauthBegin(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		res, err := svc.ReauthBegin(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reauth_begin_failed", safeErr(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"challengeKey": res.ChallengeKey,
			"options":      res.Assertion,
		})
	}
}

func reauthFinish(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var envelope struct {
			ChallengeKey string          `json:"challengeKey"`
			Credential   json.RawMessage `json:"credential"`
			Purpose      string          `json:"purpose"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.ChallengeKey == "" {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		syncBackupEligible(svc, r, envelope.Credential)

		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(envelope.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		res, err := svc.ReauthFinish(r.Context(), envelope.ChallengeKey, accountID, envelope.Purpose, newReq)
		if err != nil {
			log.Error().Err(err).Msg("reauth_finish_failed")
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusUnauthorized
			}
			writeError(w, status, "reauth_finish_failed", safeErr(err))
			return
		}

		resp := map[string]any{
			"accountId":        res.AccountID,
			"wrappedMasterKey": base64.StdEncoding.EncodeToString(res.WrappedMasterKey),
		}
		if res.AddCredToken != "" {
			resp["addCredentialToken"] = res.AddCredToken
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// ─── Credential Management ─────────────────────────────────────────────────────

func addCredentialBegin(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var req struct {
			AddCredentialToken string `json:"addCredentialToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AddCredentialToken == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "addCredentialToken required")
			return
		}

		res, err := svc.AddCredentialBegin(r.Context(), accountID, req.AddCredentialToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "add_cred_begin_failed", safeErr(err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"options": res.Creation,
			"prfSalt": base64.StdEncoding.EncodeToString(res.PRFSalt),
		})
	}
}

func addCredentialFinish(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var req struct {
			AddCredentialToken string          `json:"addCredentialToken"`
			PRFSalt            string          `json:"prfSalt"`
			WrappedMasterKey   string          `json:"wrappedMasterKey"`
			Name               string          `json:"name"`
			Credential         json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		prfSalt, err := base64.StdEncoding.DecodeString(req.PRFSalt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "prfSalt must be base64")
			return
		}
		wmk, err := base64.StdEncoding.DecodeString(req.WrappedMasterKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedMasterKey must be base64")
			return
		}

		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(req.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		svcReq := &AddCredentialFinishRequest{
			AddCredToken:     req.AddCredentialToken,
			WrappedMasterKey: wmk,
			PRFSalt:          prfSalt,
			Name:             req.Name,
		}

		result, err := svc.AddCredentialFinish(r.Context(), accountID, svcReq, newReq)
		if err != nil {
			status := http.StatusInternalServerError
			code := "add_cred_finish_failed"
			if err == ErrDuplicateAccount {
				status = http.StatusConflict
				code = "credential_exists"
			}
			writeError(w, status, code, safeErr(err))
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":        result.ID,
			"name":      result.Name,
			"createdAt": result.CreatedAt,
		})
	}
}

func listCredentials(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		sessionCredIDBase64 := r.Header.Get("X-Session-Credential-ID")

		creds, err := svc.ListCredentials(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list_creds_failed", safeErr(err))
			return
		}

		type credInfo struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			CreatedAt        string `json:"createdAt"`
			BackupEligible   bool   `json:"backupEligible"`
			IsCurrentSession bool   `json:"isCurrentSession"`
		}
		out := make([]credInfo, len(creds))
		for i, c := range creds {
			credB64 := base64.StdEncoding.EncodeToString(c.CredentialID)
			out[i] = credInfo{
				ID:               c.ID,
				Name:             c.Name,
				CreatedAt:        c.CreatedAt,
				BackupEligible:   c.BackupEligible,
				IsCurrentSession: credB64 == sessionCredIDBase64,
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func renameCredential(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		credID := chi.URLParam(r, "id")

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "name required")
			return
		}

		if err := svc.RenameCredential(r.Context(), accountID, credID, req.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "rename_cred_failed", safeErr(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteCredential(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		credID := chi.URLParam(r, "id")

		if err := svc.DeleteCredential(r.Context(), accountID, credID); err != nil {
			if err == ErrLastCredential {
				writeError(w, http.StatusConflict, "last_credential", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "delete_cred_failed", safeErr(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Recovery Code Rotation ────────────────────────────────────────────────────

func rotateRecoveryCodes(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var req struct {
			RecoveryWrappedMaster string   `json:"recoveryWrappedMasterKey"`
			RecoveryVerifier      string   `json:"recoveryVerifier"`
			RecoveryCodes         []string `json:"recoveryCodes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		rwmk, err := base64.StdEncoding.DecodeString(req.RecoveryWrappedMaster)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "recoveryWrappedMasterKey must be base64")
			return
		}
		rv, err := base64.StdEncoding.DecodeString(req.RecoveryVerifier)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "recoveryVerifier must be base64")
			return
		}
		if len(req.RecoveryCodes) != 12 {
			writeError(w, http.StatusBadRequest, "invalid_field", "expected 12 recoveryCodes")
			return
		}
		codeHashes := make([][]byte, 12)
		for i, c := range req.RecoveryCodes {
			h, err := base64.StdEncoding.DecodeString(c)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "recoveryCodes must be base64")
				return
			}
			codeHashes[i] = h
		}

		svcReq := &RotateRecoveryCodesRequest{
			RecoveryWrappedMaster: rwmk,
			RecoveryVerifier:      rv,
			RecoveryCodes:         codeHashes,
		}
		if err := svc.RotateRecoveryCodes(r.Context(), accountID, svcReq); err != nil {
			writeError(w, http.StatusInternalServerError, "rotate_failed", "failed to rotate recovery codes")
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

// ─── Account deletion ──────────────────────────────────────────────────────────

func deleteAccount(svc *Service, billing SubscriptionCanceller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		subs, err := svc.DeleteAccount(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "delete_account_failed", safeErr(err))
			return
		}

		clearSessionCookie(w)

		// Cancel Stripe subscriptions after the DB commit. Failures are
		// logged but do not affect the response — the account is already gone.
		for _, sub := range subs {
			if err := billing.CancelSubscription(r.Context(), sub.StripeSubscriptionID); err != nil {
				log.Error().
					Str("subscriptionID", sub.StripeSubscriptionID).
					Str("workspaceID", sub.WorkspaceID).
					Err(err).
					Msg("failed to cancel stripe subscription after account deletion")
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Device Pairing ───────────────────────────────────────────────────────────

func createPairing(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		res, err := svc.PairingCreate(r.Context(), accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "pairing_create_failed", safeErr(err))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"token":     res.Token,
			"shortCode": res.ShortCode,
			"expiresAt": res.ExpiresAt.UTC().Format(time.RFC3339),
		})
	}
}

func pairingByCode(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := chi.URLParam(r, "code")
		token, ok := svc.PairingByCode(code)
		if !ok {
			writeError(w, http.StatusNotFound, "pairing_not_found", "pairing expired or not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

func pairingPoll(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		res, ok := svc.PairingPoll(r.Context(), token)
		if !ok {
			writeError(w, http.StatusNotFound, "pairing_not_found", "pairing expired or not found")
			return
		}
		resp := map[string]any{"state": res.State}
		if len(res.NewDevicePubKey) > 0 {
			resp["newDevicePublicKey"] = base64.StdEncoding.EncodeToString(res.NewDevicePubKey)
		}
		if len(res.WrappedMasterKey) > 0 {
			resp["wrappedMasterKey"] = base64.StdEncoding.EncodeToString(res.WrappedMasterKey)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func pairingRequest(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")

		var req struct {
			EphemeralPublicKey string `json:"ephemeralPublicKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EphemeralPublicKey == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "ephemeralPublicKey required")
			return
		}
		pubKey, err := base64.StdEncoding.DecodeString(req.EphemeralPublicKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "ephemeralPublicKey must be base64")
			return
		}

		res, err := svc.PairingRequest(r.Context(), token, pubKey)
		if err != nil {
			switch err {
			case ErrNotFound:
				writeError(w, http.StatusNotFound, "pairing_not_found", "pairing expired or not found")
			case ErrConflict:
				writeError(w, http.StatusConflict, "pairing_claimed", "this pairing request was already accepted by another device")
			case ErrTooManyAttempts:
				writeError(w, http.StatusTooManyRequests, "too_many_attempts", "too many attempts")
			default:
				writeError(w, http.StatusInternalServerError, "pairing_request_failed", safeErr(err))
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"options": res.Creation,
			"prfSalt": base64.StdEncoding.EncodeToString(res.PRFSalt),
		})
	}
}

func pairingFulfill(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		token := chi.URLParam(r, "token")

		var req struct {
			WrappedMasterKey string `json:"wrappedMasterKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WrappedMasterKey == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "wrappedMasterKey required")
			return
		}
		wmk, err := base64.StdEncoding.DecodeString(req.WrappedMasterKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedMasterKey must be base64")
			return
		}

		if err := svc.PairingFulfill(r.Context(), accountID, token, wmk); err != nil {
			switch err {
			case ErrNotFound:
				writeError(w, http.StatusNotFound, "pairing_not_found", "pairing expired or not found")
			case ErrConflict:
				writeError(w, http.StatusConflict, "pairing_conflict", "pairing is not in the expected state")
			default:
				writeError(w, http.StatusInternalServerError, "pairing_fulfill_failed", safeErr(err))
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func pairingComplete(svc *Service, dev bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var req struct {
			PRFSalt          string          `json:"prfSalt"`
			WrappedMasterKey string          `json:"wrappedMasterKey"`
			Name             string          `json:"name"`
			Credential       json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		prfSalt, err := base64.StdEncoding.DecodeString(req.PRFSalt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "prfSalt must be base64")
			return
		}
		wmk, err := base64.StdEncoding.DecodeString(req.WrappedMasterKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedMasterKey must be base64")
			return
		}

		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(req.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		res, err := svc.PairingComplete(r.Context(), token, prfSalt, wmk, req.Name, r.Header.Get("User-Agent"), newReq)
		if err != nil {
			switch err {
			case ErrNotFound:
				writeError(w, http.StatusNotFound, "pairing_not_found", "pairing expired or not found")
			case ErrConflict:
				writeError(w, http.StatusConflict, "pairing_conflict", "pairing is not in the expected state")
			case ErrTooManyAttempts:
				writeError(w, http.StatusTooManyRequests, "too_many_attempts", "too many attempts")
			case ErrDuplicateAccount:
				writeError(w, http.StatusConflict, "credential_exists", safeErr(err))
			default:
				log.Error().Err(err).Msg("pairing_complete_failed")
				writeError(w, http.StatusInternalServerError, "pairing_complete_failed", safeErr(err))
			}
			return
		}

		setSessionCookie(w, res.SessionToken, dev)
		writeJSON(w, http.StatusOK, map[string]any{
			"accountId":        res.AccountID,
			"sessionToken":     res.SessionToken,
			"sessionId":        res.SessionID,
			"credentialId":     base64.StdEncoding.EncodeToString(res.CredentialID),
			"wrappedMasterKey": base64.StdEncoding.EncodeToString(res.WrappedMasterKey),
		})
	}
}

// syncBackupEligible pre-syncs the BackupEligible flag from an assertion JSON
// so FinishDiscoverableLogin's consistency check passes.
func syncBackupEligible(svc *Service, r *http.Request, credJSON []byte) {
	if credID, err := extractCredentialID(credJSON); err == nil {
		if be, err := extractBackupEligible(credJSON); err == nil {
			svc.SyncBackupEligible(r.Context(), credID, be)
		}
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func safeErr(err error) string {
	// In production, never leak internal error details.
	// Return a generic message for 5xx; for known errors return their text.
	if err == ErrNotFound || err == ErrDuplicateAccount || err == ErrInvalidCode {
		return err.Error()
	}
	return "internal server error"
}

func setSessionCookie(w http.ResponseWriter, token string, dev bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   1209600, // 14 days
		HttpOnly: true,
		Secure:   !dev, // Secure requires HTTPS; disable for local HTTP dev
		SameSite: http.SameSiteStrictMode,
	})
}

// extractBackupEligible reads the BackupEligible flag (bit 3 of the authenticator
// flags byte) from a raw WebAuthn assertion JSON without consuming an http.Request.
// extractCredentialID parses the rawId field from a WebAuthn assertion JSON.
func extractCredentialID(credJSON []byte) ([]byte, error) {
	var parsed struct {
		RawID string `json:"rawId"`
	}
	if err := json.Unmarshal(credJSON, &parsed); err != nil {
		return nil, err
	}
	return base64.RawURLEncoding.DecodeString(parsed.RawID)
}

func extractBackupEligible(credJSON []byte) (bool, error) {
	var parsed struct {
		Response struct {
			AuthenticatorData string `json:"authenticatorData"`
		} `json:"response"`
	}
	if err := json.Unmarshal(credJSON, &parsed); err != nil {
		return false, err
	}
	// authenticatorData is base64url-encoded; flags byte is at offset 32.
	authData, err := base64.RawURLEncoding.DecodeString(parsed.Response.AuthenticatorData)
	if err != nil {
		return false, err
	}
	if len(authData) < 33 {
		return false, nil
	}
	// Bit 3 (0x08) is the BE (BackupEligible) flag per the WebAuthn spec.
	return authData[32]&0x08 != 0, nil
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
