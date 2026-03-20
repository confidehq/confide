package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/wisp/internal/auth"
	"github.com/phantompunk/wisp/internal/config"
	mw "github.com/phantompunk/wisp/internal/middleware"
)

func New(cfg *config.Config, pool *pgxpool.Pool, wa *webauthn.WebAuthn) http.Handler {
	svc := auth.NewService(pool, wa)

	r := chi.NewRouter()
	r.Use(mw.SecurityHeaders)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// All /auth/* routes share a general rate limiter.
	r.Route("/api/auth", func(r chi.Router) {
		r.Use(mw.RateLimit(cfg.HMACKey))
		r.Mount("/", auth.Handler(svc, cfg.HMACKey))
	})

	return r
}
