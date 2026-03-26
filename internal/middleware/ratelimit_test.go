package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/phantompunk/confide/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	handler := mw.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}
	for h, want := range headers {
		if got := rr.Header().Get(h); got != want {
			t.Errorf("%s = %q, want %q", h, got, want)
		}
	}
}

func TestRateLimit_Allows(t *testing.T) {
	seed := make([]byte, 32)
	handler := mw.RateLimit(seed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRecoveryRateLimit_Enforces(t *testing.T) {
	seed := make([]byte, 32)
	handler := mw.RecoveryRateLimit(seed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send 6 requests from the same IP; the 6th should be rate-limited (limit is 5).
	var lastCode int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/recover", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		lastCode = rr.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 on 6th request, got %d", lastCode)
	}
}
