package domain

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/phantompunk/confide/internal/db/queries"
)

const maxCNAMEFailures = 3

// workerDB is the subset of queries.Queries the worker needs.
type workerDB interface {
	ListAllUnverifiedDomains(ctx context.Context) ([]queries.CustomDomain, error)
	ListAllEnabledCustomDomains(ctx context.Context) ([]queries.CustomDomain, error)
	UpdateDNSStatus(ctx context.Context, arg queries.UpdateDNSStatusParams) error
	EnableCustomDomain(ctx context.Context, id string) error
	DisableCustomDomain(ctx context.Context, id string) error
}

// Worker polls unverified custom domains and drives them to enabled once both
// CNAME and TXT records are confirmed. It runs until ctx is cancelled.
type Worker struct {
	db               workerDB
	verifier         *Verifier
	registry         *Registry
	interval         time.Duration
	traefikConfigDir string
	// cnameFailures tracks consecutive CNAME check failures per domain ID for
	// already-enabled domains. Cleared when CNAME recovers.
	cnameFailures map[string]int
}

func NewWorker(db workerDB, verifier *Verifier, registry *Registry, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	return &Worker{
		db:            db,
		verifier:      verifier,
		registry:      registry,
		interval:      interval,
		cnameFailures: make(map[string]int),
	}
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

	enabled, err := w.db.ListAllEnabledCustomDomains(ctx)
	if err != nil {
		log.Error().Err(err).Msg("domain worker: list enabled domains")
		return
	}
	for _, cd := range enabled {
		w.recheck(ctx, cd)
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

// recheck verifies that an already-enabled domain still has a valid CNAME. If
// CNAME fails maxCNAMEFailures consecutive times the domain is disabled and its
// Traefik config removed.
func (w *Worker) recheck(ctx context.Context, cd queries.CustomDomain) {
	cnameOK, err := w.verifier.CheckCNAME(ctx, cd.Domain)
	if err != nil {
		log.Debug().Err(err).Str("domain", cd.Domain).Msg("domain worker: re-check CNAME lookup")
	}

	if cnameOK {
		delete(w.cnameFailures, cd.ID)
		return
	}

	w.cnameFailures[cd.ID]++
	failures := w.cnameFailures[cd.ID]
	log.Warn().Str("domain", cd.Domain).Int("consecutive_failures", failures).
		Msg("domain worker: enabled domain CNAME no longer resolves")

	if failures < maxCNAMEFailures {
		return
	}

	log.Error().Str("domain", cd.Domain).Msg("domain worker: disabling domain after repeated CNAME failures")
	if err := w.db.DisableCustomDomain(ctx, cd.ID); err != nil {
		log.Error().Err(err).Str("domain", cd.Domain).Msg("domain worker: disable domain")
		return
	}
	delete(w.cnameFailures, cd.ID)
	w.registry.Disable(cd.Domain)
	if w.traefikConfigDir != "" {
		if err := DeleteTraefikConfig(w.traefikConfigDir, cd.Domain); err != nil {
			log.Error().Err(err).Str("domain", cd.Domain).Msg("domain worker: delete traefik config")
		} else {
			log.Info().Str("domain", cd.Domain).Msg("domain worker: traefik config deleted")
		}
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
