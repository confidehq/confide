package middleware

import (
	"context"
	"net/http"
	"strings"
)

// DomainVerifier is implemented by any service that can mark a custom domain as verified.
type DomainVerifier interface {
	MarkCustomDomainVerified(ctx context.Context, domain string) error
}

// VerifyCustomDomain marks a workspace's custom domain as verified on the first
// inbound request whose Host header matches that domain. It is a no-op for the
// app's own domain and for hostnames that are not registered as custom domains.
func VerifyCustomDomain(appDomain string, verifier DomainVerifier) func(http.Handler) http.Handler {
	// Strip any scheme from appDomain so we can compare bare hostnames.
	appHost := stripScheme(appDomain)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Strip port if present.
			if i := strings.LastIndex(host, ":"); i > 0 {
				host = host[:i]
			}
			if host != "" && host != appHost {
				// Fire-and-forget: verification is best-effort. A failed update
				// (domain not registered, already verified) is not an error.
				_ = verifier.MarkCustomDomainVerified(r.Context(), host)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// StripScheme removes the scheme, path, and port from a URL-like string,
// returning a bare hostname suitable for comparison.
func StripScheme(s string) string {
	return stripScheme(s)
}

func stripScheme(s string) string {
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Also strip any trailing path.
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	// Strip port.
	if i := strings.LastIndex(s, ":"); i > 0 {
		s = s[:i]
	}
	return s
}
