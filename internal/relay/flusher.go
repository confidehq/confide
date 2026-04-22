package relay

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// BatchStorer is implemented by responses.Service.
// The interface is defined here to avoid an import cycle.
type BatchStorer interface {
	CreateBatch(ctx context.Context, items []SubmissionItem) error
}

// StartFlusher drains the queue on each tick and writes to the database via storer.
// It performs one final flush when ctx is cancelled (graceful shutdown).
func StartFlusher(ctx context.Context, q *Queue, storer BatchStorer, interval time.Duration) {
	log.Info().Dur("interval", interval).Msg("starting relay flusher")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	flush := func() {
		items := q.Drain()
		if len(items) == 0 {
			return
		}
		if err := storer.CreateBatch(ctx, items); err != nil {
			log.Error().Int("dropped", len(items)).Err(err).Msg("relay flush failed")
			return
		}
		log.Info().Int("count", len(items)).Msg("relay flushed")
	}

	for {
		select {
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush() // drain remaining on shutdown
			return
		}
	}
}
