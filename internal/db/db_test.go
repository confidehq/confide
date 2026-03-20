package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/phantompunk/wisp/internal/db"
	"github.com/phantompunk/wisp/migrations"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("GHOSTFORM_TEST_DATABASE_URL")
	if u == "" {
		u = "postgresql://ghostform:ghostform@localhost:5432/ghostform_test?sslmode=disable"
	}
	return u
}

func TestOpen_MigrationsApply(t *testing.T) {
	url := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := db.Open(ctx, url, migrations.FS)
	if err != nil {
		t.Skipf("skipping (no test DB): %v", err)
	}
	defer pool.Close()

	// Verify tables exist.
	tables := []string{"accounts", "recovery_codes", "sessions"}
	for _, table := range tables {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)", table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s not found after migrations", table)
		}
	}
}
