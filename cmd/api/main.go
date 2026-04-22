package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/cmd/api/app"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	app.Version = version
	app.Commit = commit

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
