package relay

import (
	"context"
	"log/slog"
	"time"
)

// BatchStorer is implemented by responses.Service.
// The interface is defined here to avoid an import cycle.
type BatchStorer interface {
	CreateBatch(ctx context.Context, items []SubmissionItem) error
}

// StartFlusher drains the queue on each tick and writes to the database via storer.
// It performs one final flush when ctx is cancelled (graceful shutdown).
func StartFlusher(ctx context.Context, q *Queue, storer BatchStorer, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	flush := func() {
		items := q.Drain()
		if len(items) == 0 {
			return
		}
		if err := storer.CreateBatch(ctx, items); err != nil {
			slog.Error("relay flush failed", "dropped", len(items), "err", err)
			return
		}
		slog.Info("relay flushed", "count", len(items))
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
