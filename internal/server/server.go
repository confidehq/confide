package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/auth"
	"github.com/phantompunk/confide/internal/botguard"
	"github.com/phantompunk/confide/internal/config"
	"github.com/phantompunk/confide/internal/forms"
	mw "github.com/phantompunk/confide/internal/middleware"
	"github.com/phantompunk/confide/internal/relay"
	"github.com/phantompunk/confide/internal/responses"
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

func New(cfg *config.Config, svc *Services, version, commit string) http.Handler {
	guard := botguard.New(cfg.HMACKey)
	r := chi.NewRouter()
	r.Use(mw.SecurityHeaders)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	// API routes — CORS restricted to configured origin, general CSP applied.
	r.Route("/api", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{cfg.CORSOrigin},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
		r.Use(mw.AppCSP)

		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version, "commit": commit})
		})

		// Auth routes — general rate limit.
		r.Route("/auth", func(r chi.Router) {
			r.Use(mw.RateLimit(cfg.HMACKey))
			r.Mount("/", auth.Handler(svc.Auth, cfg.HMACKey, cfg.Env == "development", cfg.RegistrationOpen))
		})

		// Authenticated form + response routes.
		r.Group(func(r chi.Router) {
			r.Use(mw.Authenticator(svc.Auth))
			r.Mount("/forms", forms.Handler(svc.Forms))
			r.Route("/forms/{formId}/responses", func(r chi.Router) {
				r.Mount("/", responses.Handler(svc.Responses))
			})
		})

		// Public unauthenticated schema endpoint — stricter CSP, own rate limit.
		r.With(mw.FormPageCSP, mw.PublicSchemaRateLimit(cfg.HMACKey)).
			Get("/f/{id}/schema", forms.PublicSchemaHandler(svc.Forms, guard))
	})

	// Relay submit — open CORS (respondents arrive from arbitrary origins), rate limited.
	r.With(
		cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"POST", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type"},
			AllowCredentials: false,
			MaxAge:           300,
		}),
		mw.RelayRateLimit(cfg.HMACKey),
	).Post("/relay/submit", relay.SubmitHandler(svc.RelayQ, guard))

	return r
}
