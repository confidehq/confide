package app

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/internal/config"
	"github.com/phantompunk/confide/internal/db"
	"github.com/phantompunk/confide/internal/reaper"
	"github.com/phantompunk/confide/internal/relay"
	"github.com/phantompunk/confide/internal/server"
	"github.com/phantompunk/confide/migrations"
	"github.com/phantompunk/confide/ui"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

type App struct {
	cfg       *config.Config
	srv       *http.Server
	runCancel context.CancelFunc
	pool      *pgxpool.Pool
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if cfg.Env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Info().Msg("initializing application")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL, migrations.FS)
	if err != nil {
		return nil, err
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: cfg.RPDisplayName,
		RPOrigins:     []string{cfg.RPOrigin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification:   protocol.VerificationRequired,
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: boolPtr(true),
		},
		Debug: cfg.Env == "development",
	})
	if err != nil {
		pool.Close()
		return nil, err
	}

	svc := server.NewServices(pool, wa, cfg)

	runCtx, runCancel := context.WithCancel(context.Background())
	go relay.StartFlusher(runCtx, svc.RelayQ, svc.Responses, cfg.RelayFlushInterval)
	go reaper.Start(runCtx, svc.Responses, cfg.ReaperInterval)

	h := server.New(cfg, svc, ui.FS, Version, Commit)

	srv := &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &App{
		cfg:       cfg,
		srv:       srv,
		runCancel: runCancel,
		pool:      pool,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	go func() {
		log.Info().Str("version", Version).Str("commit", Commit).Str("env", a.cfg.Env).Str("addr", a.cfg.BindAddr).Msg("starting confide server")
		if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")

	a.runCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := a.srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}

	a.pool.Close()
	return nil
}

func boolPtr(b bool) *bool { return &b }
