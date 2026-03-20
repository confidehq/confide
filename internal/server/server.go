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
	"github.com/phantompunk/wisp/internal/forms"
	mw "github.com/phantompunk/wisp/internal/middleware"
	"github.com/phantompunk/wisp/internal/relay"
	"github.com/phantompunk/wisp/internal/responses"
)

// Services groups the application services passed into the server.
type Services struct {
	Auth      *auth.Service
	Forms     *forms.Service
	Responses *responses.Service
	RelayQ    *relay.Queue
}

// NewServices constructs all application services from the pool and webauthn instance.
func NewServices(pool *pgxpool.Pool, wa *webauthn.WebAuthn) *Services {
	return &Services{
		Auth:      auth.NewService(pool, wa),
		Forms:     forms.NewService(pool),
		Responses: responses.NewService(pool),
		RelayQ:    &relay.Queue{},
	}
}

func New(cfg *config.Config, svc *Services) http.Handler {
	r := chi.NewRouter()
	r.Use(mw.SecurityHeaders)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Auth routes — general rate limit.
	r.Route("/api/auth", func(r chi.Router) {
		r.Use(mw.RateLimit(cfg.HMACKey))
		r.Mount("/", auth.Handler(svc.Auth, cfg.HMACKey, cfg.Env == "development"))
	})

	// Authenticated form + response routes.
	r.Group(func(r chi.Router) {
		r.Use(mw.Authenticator(svc.Auth))
		r.Mount("/api/forms", forms.Handler(svc.Forms))
		r.Route("/api/forms/{formId}/responses", func(r chi.Router) {
			r.Mount("/", responses.Handler(svc.Responses))
		})
	})

	// Public unauthenticated schema endpoint — no cookies, cache-control set in handler.
	r.With(mw.PublicSchemaRateLimit(cfg.HMACKey)).
		Get("/api/f/{id}/schema", forms.PublicSchemaHandler(svc.Forms))

	// Relay submit — no auth, no Chi logger, rate limited.
	r.With(mw.RelayRateLimit(cfg.HMACKey)).
		Post("/relay/submit", relay.SubmitHandler(svc.RelayQ))

	return r
}
