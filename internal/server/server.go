package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/auth"
	"github.com/phantompunk/confide/internal/billing"
	"github.com/phantompunk/confide/internal/botguard"
	"github.com/phantompunk/confide/internal/config"
	"github.com/phantompunk/confide/internal/db/queries"
	"github.com/phantompunk/confide/internal/domain"
	"github.com/phantompunk/confide/internal/forms"
	"github.com/phantompunk/confide/internal/identity"
	"github.com/phantompunk/confide/internal/invitation"
	"github.com/phantompunk/confide/internal/mailer"
	"github.com/rs/zerolog/log"

	mw "github.com/phantompunk/confide/internal/middleware"
	"github.com/phantompunk/confide/internal/permission"
	"github.com/phantompunk/confide/internal/relay"
	"github.com/phantompunk/confide/internal/responses"
	"github.com/phantompunk/confide/internal/workspace"
)

// Services groups the application services passed into the server.
type Services struct {
	Auth         *auth.Service
	Forms        *forms.Service
	Responses    *responses.Service
	Workspace    *workspace.Service
	Identity     *identity.Service
	Invitation   *invitation.Service
	Billing      *billing.Service
	RelayQ       *relay.Queue
	domainWorker *domain.Worker
}

// Start launches background workers. Call this after New.
func (s *Services) Start(ctx context.Context) {
	go s.domainWorker.Run(ctx)
}

// NewServices constructs all application services from the pool and webauthn instance.
func NewServices(pool *pgxpool.Pool, wa *webauthn.WebAuthn, cfg *config.Config) *Services {
	m := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.FromEmail, cfg.ResendAPIKey)
	wsSvc := workspace.NewService(pool)

	// Build domain registry from DB and wire it into the workspace service.
	enabledDomains, err := queries.New(pool).ListAllEnabledDomains(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("domain: failed to load enabled domains")
	}
	reg := domain.NewRegistry(enabledDomains)
	wsSvc.WithRegistry(reg)

	// Start the background DNS verification worker.
	verifier := &domain.Verifier{CNAMETarget: mw.StripScheme(cfg.CustomDomainTarget)}
	worker := domain.NewWorker(queries.New(pool), verifier, reg, cfg.DomainVerifyInterval)
	if cfg.TraefikConfigDir != "" {
		worker.WithTraefikConfigDir(cfg.TraefikConfigDir)
	}
	wsSvc.WithDomainChecker(worker)

	billingSvc := billing.NewService(pool, cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripePriceIDPro, cfg.StripePriceIDOrg)
	wsSvc.WithSubscriptionCanceler(billingSvc)

	return &Services{
		Auth:       auth.NewService(pool, wa),
		Forms:      forms.NewService(pool),
		Responses:  responses.NewService(pool, m),
		Workspace:  wsSvc,
		Identity:   identity.NewService(pool),
		Invitation: invitation.NewService(pool, m, cfg.AppDomain),
		Billing:    billingSvc,
		RelayQ:     &relay.Queue{},
		domainWorker: worker,
	}
}

func New(cfg *config.Config, svc *Services, uiFS fs.FS, version, commit string) http.Handler {
	guard := botguard.New(cfg.HMACKey)
	r := chi.NewRouter()
	r.Use(mw.SecurityHeaders)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(mw.FormsDomainGate(cfg.AppDomain, cfg.FormsDomain, svc.Workspace.IsEnabledCustomDomain))

	// API routes — CORS restricted to configured origin, general CSP applied.
	r.Route("/api", func(r chi.Router) {
		staticOrigins := make(map[string]struct{}, len(cfg.CORSOrigins))
		for _, o := range cfg.CORSOrigins {
			staticOrigins[o] = struct{}{}
		}
		r.Use(cors.Handler(cors.Options{
			AllowOriginFunc: func(r *http.Request, origin string) bool {
				if _, ok := staticOrigins[origin]; ok {
					return true
				}
				host := mw.StripScheme(origin)
				if cfg.FormsDomain != "" && host == mw.StripScheme(cfg.FormsDomain) {
					return true
				}
				return svc.Workspace.IsEnabledCustomDomain(host)
			},
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

		r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"formsDomain": cfg.FormsDomain})
		})

		// Auth routes — general rate limit.
		r.Route("/auth", func(r chi.Router) {
			r.Use(mw.RateLimit(cfg.HMACKey))
			r.Mount("/", auth.Handler(svc.Auth, svc.Billing, cfg.HMACKey, cfg.Env == "development", cfg.RegistrationOpen))
		})

		// Authenticated form, response, identity, workspace, and invitation routes.
		r.Group(func(r chi.Router) {
			r.Use(mw.Authenticator(svc.Auth))
			r.Mount("/forms", forms.Handler(svc.Forms, svc.Workspace))
			r.Route("/forms/{formId}/responses", func(r chi.Router) {
				r.Mount("/", responses.Handler(svc.Responses, svc.Forms, svc.Workspace))
			})
			r.Mount("/", identity.Handler(svc.Identity))

			roleCache := svc.Workspace.Cache()
			r.Mount("/workspaces", workspace.Handler(svc.Workspace, roleCache, cfg.CustomDomainTarget))
			r.Route("/workspaces/{workspaceId}/invitations", func(r chi.Router) {
				r.Use(permission.ResolveWorkspaceRole(svc.Workspace, roleCache, "workspaceId"))
				r.Use(permission.RequireAction(permission.ActionInviteMembers))
				r.Mount("/", invitation.WorkspaceHandler(svc.Invitation))
			})
			r.Post("/invitations/{token}/accept", invitation.AcceptHandler(svc.Invitation))
			r.Route("/workspaces/{workspaceId}/billing", func(r chi.Router) {
				r.Use(permission.ResolveWorkspaceRole(svc.Workspace, roleCache, "workspaceId"))
				r.Use(permission.RequireAction(permission.ActionManageBilling))
				r.Mount("/", billing.Handler(svc.Billing))
			})
		})

		// Public invitation resolve — no auth required.
		r.Mount("/invitations", invitation.PublicHandler(svc.Invitation))

		// Public unauthenticated schema endpoint — stricter CSP, own rate limit.
		r.With(mw.FormPageCSP, mw.PublicSchemaRateLimit(cfg.HMACKey)).
			Get("/f/{id}/schema", forms.PublicSchemaHandler(svc.Forms, guard, mw.StripScheme(cfg.AppDomain), svc.Workspace))
	})

	// Stripe webhook — public, no auth, signature verified inside handler.
	r.Post("/api/stripe/webhook", billing.WebhookHandler(svc.Billing))

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
	).Post("/relay/submit", relay.SubmitHandler(svc.RelayQ, svc.Responses, guard))

	// SPA catch-all: serve the embedded frontend for any path not matched above.
	if uiFS != nil {
		r.Get("/*", spaHandler(uiFS))
	}

	return r
}

// spaHandler serves the embedded SvelteKit static build. Unknown paths fall
// back to index.html so the client-side router handles navigation.
func spaHandler(uiFS fs.FS) http.HandlerFunc {
	sub, _ := fs.Sub(uiFS, "build")
	index, _ := fs.ReadFile(sub, "index.html")
	fileServer := http.FileServer(http.FS(sub))

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(sub, path); err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(index)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}
}
