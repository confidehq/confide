package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/phantompunk/confide/internal/config"
	"github.com/phantompunk/confide/internal/db"
	"github.com/phantompunk/confide/internal/relay"
	"github.com/phantompunk/confide/internal/server"
	"github.com/phantompunk/confide/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL, migrations.FS)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

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
		log.Fatalf("webauthn: %v", err)
	}

	svc := server.NewServices(pool, wa)

	// Start relay flusher — runs until ctx is cancelled on shutdown.
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	go relay.StartFlusher(runCtx, svc.RelayQ, svc.Responses, cfg.RelayFlushInterval)

	h := server.New(cfg, svc)

	srv := &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("listening on %s", cfg.BindAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down…")

	// Cancel relay flusher — triggers final flush before exit.
	runCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func boolPtr(b bool) *bool { return &b }
