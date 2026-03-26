package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phantompunk/confide/internal/auth"
)

func TestHealthEndpoint(t *testing.T) {
	// The health endpoint lives in server.New, but we sanity-check that
	// auth.Handler mounts without panicking.
	seed := make([]byte, 32)
	// We can't call auth.Handler with nil service in production, but we can
	// instantiate it to verify the router compiles.
	_ = auth.Handler((*auth.Service)(nil), seed, true, true)
}

func TestRegisterBegin_BadMethod(t *testing.T) {
	// Handler is accessible via the router. A GET to /register/begin should 405.
	// (chi returns 405 for wrong method when the path is registered for POST.)
	seed := make([]byte, 32)
	h := auth.Handler((*auth.Service)(nil), seed, true, true)

	req := httptest.NewRequest(http.MethodGet, "/register/begin", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
