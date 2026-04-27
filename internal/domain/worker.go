package domain

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/internal/db/queries"
)

// workerDB is the subset of queries.Queries the worker needs.
type workerDB interface {
	ListAllUnverifiedDomains(ctx context.Context) ([]queries.CustomDomain, error)
	UpdateDNSStatus(ctx context.Context, arg queries.UpdateDNSStatusParams) error
	EnableCustomDomain(ctx context.Context, id string) error
}

// Worker polls unverified custom domains and drives them to enabled once both
// CNAME and TXT records are confirmed. It runs until ctx is cancelled.
type Worker struct {
	db               workerDB
	verifier         *Verifier
	registry         *Registry
	interval         time.Duration
	traefikConfigDir string
}

func NewWorker(db workerDB, verifier *Verifier, registry *Registry, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	return &Worker{db: db, verifier: verifier, registry: registry, interval: interval}
}

// WithTraefikConfigDir enables writing per-domain Traefik config files so
// Traefik can obtain Let's Encrypt certs for custom domains.
func (w *Worker) WithTraefikConfigDir(dir string) {
	w.traefikConfigDir = dir
}

func (w *Worker) Run(ctx context.Context) {
	w.tick(ctx)
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	domains, err := w.db.ListAllUnverifiedDomains(ctx)
	if err != nil {
		log.Error().Err(err).Msg("domain worker: list unverified domains")
		return
	}
	for _, cd := range domains {
		w.check(ctx, cd)
	}
}

func (w *Worker) check(ctx context.Context, cd queries.CustomDomain) {
	cnameOK, err := w.verifier.CheckCNAME(ctx, cd.Domain)
	if err != nil {
		log.Debug().Err(err).Str("domain", cd.Domain).Msg("domain worker: CNAME lookup")
	}
	txtOK, err := w.verifier.CheckTXT(ctx, cd.Domain, cd.TxtToken)
	if err != nil {
		log.Debug().Err(err).Str("domain", cd.Domain).Msg("domain worker: TXT lookup")
	}

	if cnameOK != cd.CnameOk || txtOK != cd.TxtOk {
		if err := w.db.UpdateDNSStatus(ctx, queries.UpdateDNSStatusParams{
			ID:      cd.ID,
			CnameOk: cnameOK,
			TxtOk:   txtOK,
		}); err != nil {
			log.Error().Err(err).Str("domain", cd.Domain).Msg("domain worker: update DNS status")
		}
	}

	if cnameOK && txtOK {
		if err := w.db.EnableCustomDomain(ctx, cd.ID); err != nil {
			log.Error().Err(err).Str("domain", cd.Domain).Msg("domain worker: enable domain")
			return
		}
		w.registry.Enable(cd.Domain)
		if w.traefikConfigDir != "" {
			if err := writeTraefikConfig(w.traefikConfigDir, cd.Domain); err != nil {
				log.Error().Err(err).Str("domain", cd.Domain).Msg("domain worker: write traefik config")
			} else {
				log.Info().Str("domain", cd.Domain).Msg("domain worker: traefik config written")
			}
		}
		log.Info().Str("domain", cd.Domain).Msg("domain worker: custom domain enabled")
	}
}

// CheckNow runs a single verification pass for the given domain record.
// Used by the "Check now" API endpoint.
func (w *Worker) CheckNow(ctx context.Context, cd queries.CustomDomain) (cnameOK, txtOK bool) {
	w.check(ctx, cd)
	// Re-read state after check — worker.check updates the DB.
	cnameOK, _ = w.verifier.CheckCNAME(ctx, cd.Domain)
	txtOK, _ = w.verifier.CheckTXT(ctx, cd.Domain, cd.TxtToken)
	return cnameOK, txtOK
}
