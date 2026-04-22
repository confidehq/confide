package db

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Open creates a connection pool and runs all pending migrations.
func Open(ctx context.Context, databaseURL string, migrationsFS fs.FS) (*pgxpool.Pool, error) {
	log.Info().Str("db", safeURL(databaseURL)).Msg("connecting to database")

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	log.Info().Msg("database connection established")

	if err := runMigrations(databaseURL, migrationsFS); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return pool, nil
}

func runMigrations(databaseURL string, migrationsFS fs.FS) error {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("iofs.New: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	defer m.Close() //nolint:errcheck

	before, _, _ := m.Version()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate.Up: %w", err)
	}

	after, dirty, err := m.Version()
	if err != nil {
		return nil
	}
	if after != before {
		log.Info().Uint("from", before).Uint("to", after).Int("applied", int(after-before)).Msg("migrations applied")
	}
	log.Info().Uint("schema_version", after).Bool("dirty", dirty).Msg("database schema ready")
	return nil
}

// safeURL strips the password from a database URL before logging.
func safeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "(unparseable)"
	}
	u.User = nil
	return u.Scheme + "://[user]:[pass]@" + u.Host + u.Path
}
