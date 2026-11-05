// Package database manages PostgreSQL connection pooling and schema migrations.
package database

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"github.com/atharvix/quanta/apps/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations executes all embedded SQL migrations idempotently inside transactions.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("cannot run migrations on nil pool")
	}

	// 1. Ensure schema_migrations table exists
	createTrackerSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`
	if _, err := pool.Exec(ctx, createTrackerSQL); err != nil {
		return fmt.Errorf("failed to initialize schema_migrations table: %w", err)
	}

	// 2. Discover all .sql migration files embedded in the binary
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			filenames = append(filenames, entry.Name())
		}
	}
	sort.Strings(filenames)

	// 3. Apply each migration in an isolated transaction
	for _, filename := range filenames {
		version := strings.TrimSuffix(filename, ".sql")

		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for '%s': %w", version, err)
		}

		if exists {
			continue
		}

		content, err := fs.ReadFile(migrations.Files, filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file '%s': %w", filename, err)
		}

		log.Printf("[INFO] Applying database migration: %s", version)

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration '%s': %w", version, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration '%s' execution failed: %w", version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record applied migration '%s': %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration '%s': %w", version, err)
		}

		log.Printf("[INFO] Database migration '%s' applied successfully", version)
	}

	log.Println("[INFO] Database schema is up to date.")
	return nil
}
