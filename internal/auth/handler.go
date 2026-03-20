package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/wisp/internal/middleware"
)

// Handler builds the /auth sub-router. recoveryHMACKey is used to apply a
// stricter rate limit to the /recover endpoint.


const sessionCookieName = "session"

func Handler(svc *Service, recoveryHMACKey []byte) http.Handler {
	r := chi.NewRouter()

	r.Post("/register/begin", registerBegin(svc))
	r.Post("/register/finish", registerFinish(svc))
	r.Post("/login/begin", loginBegin(svc))
	r.Post("/login/finish", loginFinish(svc))
	r.With(mw.RecoveryRateLimit(recoveryHMACKey)).Post("/recover", recover_(svc))
	r.Post("/recover/rekey/begin", rekeyBegin(svc))
	r.Post("/recover/rekey/finish", rekeyFinish(svc))

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(mw.Authenticator(svc))
		r.Post("/logout", logout(svc))
		r.Get("/sessions", listSessions(svc))
		r.Delete("/sessions/{id}", deleteSession(svc))
	})

	return r
}

// ─── Register ─────────────────────────────────────────────────────────────────

func registerBegin(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := svc.RegisterBegin(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "register_begin_failed", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"accountId": res.AccountID,
			"prfSalt":   base64.StdEncoding.EncodeToString(res.PRFSalt),
			"options":   res.Creation,
		})
	}
}

func registerFinish(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var req struct {
			AccountID             string   `json:"accountId"`
			WrappedMasterKey      string   `json:"wrappedMasterKey"`
			RecoveryWrappedMaster string   `json:"recoveryWrappedMasterKey"`
			RecoveryVerifier      string   `json:"recoveryVerifier"`
			RecoveryCodes         []string `json:"recoveryCodes"`
			PRFSalt               string   `json:"prfSalt"`
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
			WrappedMasterKey:      wmk,
			RecoveryWrappedMaster: rwmk,
			RecoveryVerifier:      rv,
			RecoveryCodes:         codeHashes,
			PRFSalt:               prfSalt,
		}

		accountID, err := svc.RegisterFinish(r.Context(), svcReq, newReq)
		if err != nil {
			status := http.StatusInternalServerError
			code := "register_finish_failed"
			if err == ErrDuplicateAccount {
				status = http.StatusConflict
				code = "credential_exists"
			}
			writeError(w, status, code, safeErr(err))
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"accountId": accountID})
	}
}

// ─── Login ─────────────────────────────────────────────────────────────────────

func loginBegin(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CredentialIDBase64 string `json:"credentialIdBase64"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CredentialIDBase64 == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "credentialIdBase64 required")
			return
		}

		credID, err := base64.StdEncoding.DecodeString(req.CredentialIDBase64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "credentialIdBase64 must be base64")
			return
		}

		res, err := svc.LoginBegin(r.Context(), credID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusUnauthorized
			}
			writeError(w, status, "login_begin_failed", safeErr(err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"credentialIdBase64": req.CredentialIDBase64,
			"options":            res.Assertion,
		})
	}
}

func loginFinish(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read_body", "failed to read body")
			return
		}

		var envelope struct {
			CredentialIDBase64 string          `json:"credentialIdBase64"`
			Credential         json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		credID, err := base64.StdEncoding.DecodeString(envelope.CredentialIDBase64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_field", "credentialIdBase64 must be base64")
			return
		}

		// Reconstruct request containing only the credential payload.
		newReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, r.URL.String(),
			io.NopCloser(bytes.NewReader(envelope.Credential)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to rebuild request")
			return
		}
		newReq.Header.Set("Content-Type", "application/json")

		res, err := svc.LoginFinish(r.Context(), credID, newReq)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusUnauthorized
			}
			writeError(w, status, "login_finish_failed", safeErr(err))
			return
		}

		setSessionCookie(w, res.Token)
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
			AccountID string `json:"accountId"`
			Code      string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "accountId and code required")
			return
		}

		res, err := svc.Recover(r.Context(), req.AccountID, req.Code)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_code", "invalid or expired recovery code")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
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

func rekeyFinish(svc *Service) http.HandlerFunc {
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

		credentialIDBase64, err := svc.RekeyFinish(r.Context(), svcReq, newReq)
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

		writeJSON(w, http.StatusOK, map[string]any{"credentialIdBase64": credentialIDBase64})
	}
}

// ─── Authenticated ─────────────────────────────────────────────────────────────

func logout(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := mw.SessionID(r.Context())
		_ = svc.Logout(r.Context(), sessionID)
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
			ID        string `json:"id"`
			CreatedAt string `json:"createdAt"`
			LastSeen  string `json:"lastSeen"`
		}
		out := make([]sessionInfo, len(rows))
		for i, s := range rows {
			out[i] = sessionInfo{
				ID:        s.ID,
				CreatedAt: s.CreatedAt.Time.Format("2006-01-02"),
				LastSeen:  s.LastSeen.Time.Format("2006-01-02"),
			}
		}
		writeJSON(w, http.StatusOK, out)
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

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
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
