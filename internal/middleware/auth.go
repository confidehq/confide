package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const (
	accountIDKey  contextKey = "accountID"
	sessionIDKey  contextKey = "sessionID"
	masterKeyKey  contextKey = "wrappedMasterKey"
)

// SessionValidator is implemented by auth.Service.
type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (accountID string, wrappedMasterKey []byte, sessionID string, err error)
}

// Authenticator returns middleware that requires a valid session cookie.
// On success it injects accountID, sessionID, and wrappedMasterKey into context.
func Authenticator(svc SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"code":"unauthenticated","message":"session required"}`, http.StatusUnauthorized)
				return
			}

			accountID, wrappedMasterKey, sessionID, err := svc.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"code":"unauthenticated","message":"invalid or expired session"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), accountIDKey, accountID)
			ctx = context.WithValue(ctx, sessionIDKey, sessionID)
			ctx = context.WithValue(ctx, masterKeyKey, wrappedMasterKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccountID retrieves the authenticated account ID from the context.
func AccountID(ctx context.Context) string {
	v, _ := ctx.Value(accountIDKey).(string)
	return v
}

// SessionID retrieves the current session ID from the context.
func SessionID(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey).(string)
	return v
}
