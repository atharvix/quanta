package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migration represents a single paired up/down SQL migration.
type Migration struct {
	Version string
	Name    string
	UpSQL   string
	DownSQL string
}

// LoadMigrations parses and validates embedded *.up.sql and *.down.sql migration files.
func LoadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	byPrefix := make(map[string]*Migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		raw, err := fs.ReadFile(migrationFiles, "migrations/"+filename)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", filename, err)
		}
		sqlText := strings.TrimSpace(string(raw))
		if sqlText == "" {
			return nil, fmt.Errorf("migration file %s is empty", filename)
		}

		var prefix, kind string
		switch {
		case strings.HasSuffix(filename, ".up.sql"):
			prefix = strings.TrimSuffix(filename, ".up.sql")
			kind = "up"
		case strings.HasSuffix(filename, ".down.sql"):
			prefix = strings.TrimSuffix(filename, ".down.sql")
			kind = "down"
		default:
			return nil, fmt.Errorf("unexpected migration filename %q", filename)
		}

		parts := strings.SplitN(prefix, "_", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("migration filename %q must follow <version>_<name>.(up|down).sql", filename)
		}

		m, exists := byPrefix[prefix]
		if !exists {
			m = &Migration{
				Version: parts[0],
				Name:    parts[1],
			}
			byPrefix[prefix] = m
		}
		if kind == "up" {
			m.UpSQL = sqlText
		} else {
			m.DownSQL = sqlText
		}
	}

	if len(byPrefix) == 0 {
		return nil, errors.New("no embedded migrations found")
	}

	keys := make([]string, 0, len(byPrefix))
	for k := range byPrefix {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]Migration, 0, len(keys))
	for _, k := range keys {
		m := byPrefix[k]
		if m.UpSQL == "" {
			return nil, fmt.Errorf("migration %s is missing .up.sql", k)
		}
		if m.DownSQL == "" {
			return nil, fmt.Errorf("migration %s is missing .down.sql", k)
		}
		result = append(result, *m)
	}
	return result, nil
}

// MigrateUp applies all unapplied embedded up-migrations inside PostgreSQL transactions.
func (d *DB) MigrateUp(ctx context.Context) error {
	pool := d.Pool()
	if pool == nil {
		return errors.New("database pool is not initialized")
	}

	migrations, err := LoadMigrations()
	if err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(32) PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	for _, m := range migrations {
		var alreadyApplied bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, m.Version).Scan(&alreadyApplied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", m.Version, err)
		}
		if alreadyApplied {
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", m.Version, err)
		}

		if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", m.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.Version, err)
		}
	}

	return nil
}
