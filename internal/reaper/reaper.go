package reaper

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// Deleter is implemented by responses.Service.
// The interface is defined here to avoid an import cycle.
type Deleter interface {
	DeleteExpiredResponses(ctx context.Context) error
}

// Start runs a background goroutine that periodically deletes expired and
// burn-after-reading responses. It performs one final sweep when ctx is
// cancelled (graceful shutdown).
func Start(ctx context.Context, d Deleter, interval time.Duration) {
	log.Info().Dur("interval", interval).Msg("starting reaper")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sweep := func() {
		if err := d.DeleteExpiredResponses(ctx); err != nil {
			log.Error().Err(err).Msg("reaper sweep failed")
			return
		}
		log.Debug().Msg("reaper sweep complete")
	}

	for {
		select {
		case <-ticker.C:
			sweep()
		case <-ctx.Done():
			// Final sweep on graceful shutdown.
			_ = d.DeleteExpiredResponses(context.Background())
			return
		}
	}
}
