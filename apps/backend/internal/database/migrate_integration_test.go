//go:build integration

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrate_PostgreSQL_Idempotency(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// 1. First execution of migrations
	if err := RunMigrations(ctx, pool); err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}

	// 2. Second execution to verify idempotency
	if err := RunMigrations(ctx, pool); err != nil {
		t.Fatalf("second migration run failed (idempotency violation): %v", err)
	}

	// 3. Verify schema_migrations entry
	var versionCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations WHERE version = '001_step5_tables'").Scan(&versionCount)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("expected 1 migration record for '001_step5_tables', got %d", versionCount)
	}

	// 4. Verify created tables exist
	tables := []string{"financial_problems", "orchestration_runs", "audit_logs"}
	for _, tbl := range tables {
		var exists bool
		query := "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = $1)"
		if err := pool.QueryRow(ctx, query, tbl).Scan(&exists); err != nil {
			t.Fatalf("failed to check existence of table %s: %v", tbl, err)
		}
		if !exists {
			t.Errorf("table %s was not found in database", tbl)
		}
	}
}
