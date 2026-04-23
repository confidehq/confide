package reaper

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// Deleter is implemented by a combined type in app that aggregates responses
// and invitation services. Defined here to avoid import cycles.
type Deleter interface {
	DeleteExpiredResponses(ctx context.Context) error
	DeleteExpiredInvitations(ctx context.Context) error
}

// Start runs a background goroutine that periodically deletes expired and
// burn-after-reading responses and expired invitations. It performs one final
// sweep when ctx is cancelled (graceful shutdown).
func Start(ctx context.Context, d Deleter, interval time.Duration) {
	logger := log.With().Str("module", "reaper").Logger()
	logger.Info().Dur("interval", interval).Msg("starting reaper")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sweep := func() {
		if err := d.DeleteExpiredResponses(ctx); err != nil {
			logger.Error().Err(err).Msg("reaper sweep failed (responses)")
		}
		if err := d.DeleteExpiredInvitations(ctx); err != nil {
			logger.Error().Err(err).Msg("reaper sweep failed (invitations)")
		}
		logger.Debug().Msg("reaper sweep complete")
	}

	for {
		select {
		case <-ticker.C:
			sweep()
		case <-ctx.Done():
			// Final sweep on graceful shutdown.
			_ = d.DeleteExpiredResponses(context.Background())
			_ = d.DeleteExpiredInvitations(context.Background())
			return
		}
	}
}
