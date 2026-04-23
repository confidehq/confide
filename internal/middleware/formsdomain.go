package middleware

import (
	"context"
	"net/http"
	"strings"
)

// FormsDomainGate restricts requests arriving on the forms subdomain or a
// verified custom domain to the public form-serving paths only. Any other
// path is redirected to the admin app domain so users land in the right place.
//
// Allowed paths on forms/custom domains:
//   - /f/*            public form page (SPA catch-all)
//   - /api/f/*        public schema API
//   - /api/health     health check
//   - /api/config     runtime client config
//   - /relay/submit   encrypted response submission
//   - /_app/*         SvelteKit asset chunks
//   - static assets   files with a known extension (.js, .css, .svg, …)
func FormsDomainGate(
	appDomain, formsDomain string,
	isVerifiedCustomDomain func(context.Context, string) bool,
) func(http.Handler) http.Handler {
	appHost := stripScheme(appDomain)
	formsHost := stripScheme(formsDomain)

	appBase := strings.TrimRight(appDomain, "/")
	if !strings.HasPrefix(appBase, "http") {
		appBase = "https://" + appBase
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if i := strings.LastIndex(host, ":"); i > 0 {
				host = host[:i]
			}

			isFormsDomain := formsDomain != "" && host == formsHost
			isCustom := !isFormsDomain && host != "" && host != appHost &&
				isVerifiedCustomDomain(r.Context(), host)

			// On the app domain, redirect public form pages to the forms subdomain.
			if formsDomain != "" && host == appHost && strings.HasPrefix(r.URL.Path, "/f/") {
				http.Redirect(w, r, "https://"+formsHost+r.URL.RequestURI(), http.StatusFound)
				return
			}

			if !isFormsDomain && !isCustom {
				next.ServeHTTP(w, r)
				return
			}

			if isAllowedOnFormsDomain(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			http.Redirect(w, r, appBase+r.URL.RequestURI(), http.StatusFound)
		})
	}
}

func isAllowedOnFormsDomain(path string) bool {
	return strings.HasPrefix(path, "/f/") ||
		strings.HasPrefix(path, "/api/f/") ||
		strings.HasPrefix(path, "/_app/") ||
		path == "/relay/submit" ||
		path == "/api/health" ||
		path == "/api/config" ||
		isStaticAssetPath(path)
}

func isStaticAssetPath(path string) bool {
	for _, ext := range []string{
		".js", ".css", ".svg", ".png", ".ico",
		".woff", ".woff2", ".ttf", ".map", ".webp",
	} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
