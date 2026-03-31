package reaper

import (
	"context"
	"log/slog"
	"time"
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
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sweep := func() {
		if err := d.DeleteExpiredResponses(ctx); err != nil {
			slog.Error("reaper sweep failed", "err", err)
			return
		}
		slog.Debug("reaper sweep complete")
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
