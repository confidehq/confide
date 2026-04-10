package identity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	mw "github.com/phantompunk/confide/internal/middleware"
)

// Handler builds the authenticated /api/account and /api/accounts sub-router.
func Handler(svc *Service) http.Handler {
	r := chi.NewRouter()
	// Own key — full keypair
	r.Get("/account/identity-key", getOwnIdentityKey(svc))
	r.Put("/account/identity-key", upsertIdentityKey(svc))
	// Another account's key — public key only
	r.Get("/accounts/{id}/identity-key", getPublicIdentityKey(svc))
	return r
}

func getOwnIdentityKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())
		kp, err := svc.Get(r.Context(), accountID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "identity key not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get identity key")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"identityPublicKey":        base64.StdEncoding.EncodeToString(kp.PublicKey),
			"wrappedIdentityPrivateKey": base64.StdEncoding.EncodeToString(kp.WrappedPrivateKey),
		})
	}
}

func upsertIdentityKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := mw.AccountID(r.Context())

		var req struct {
			IdentityPublicKey        string `json:"identityPublicKey"`
			WrappedIdentityPrivateKey string `json:"wrappedIdentityPrivateKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
			return
		}

		pubKey, err := base64.StdEncoding.DecodeString(req.IdentityPublicKey)
		if err != nil || len(pubKey) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_field", "identityPublicKey must be non-empty base64")
			return
		}
		wrappedKey, err := base64.StdEncoding.DecodeString(req.WrappedIdentityPrivateKey)
		if err != nil || len(wrappedKey) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_field", "wrappedIdentityPrivateKey must be non-empty base64")
			return
		}

		if err := svc.Upsert(r.Context(), accountID, pubKey, wrappedKey); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to save identity key")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func getPublicIdentityKey(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetAccountID := chi.URLParam(r, "id")
		pub, err := svc.GetPublicKey(r.Context(), targetAccountID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "identity key not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get identity key")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"identityPublicKey": base64.StdEncoding.EncodeToString(pub),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
