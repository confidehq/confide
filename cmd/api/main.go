package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/cmd/api/app"
	"github.com/phantompunk/confide/internal/buildinfo"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--health-check" {
		if err := app.HealthCheck(); err != nil {
			log.Fatal().Err(err).Msg("health check failed")
		}
		os.Exit(0)
	}

	app.Version, app.Commit = buildinfo.Version(version, commit)

	a, err := app.New()
	if err != nil {
		log.Fatal().Err(err).Msg("init failed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("run failed")
	}
}
