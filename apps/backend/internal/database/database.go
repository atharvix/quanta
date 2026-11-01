// Package database manages PostgreSQL connection pooling via pgxpool.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pinger defines an interface for health-checking database connectivity.
// This interface allows handlers and unit tests to verify database availability without tight coupling.
type Pinger interface {
	Ping(ctx context.Context) error
}

// DB wraps the pgxpool connection pool and implements the Pinger interface.
type DB struct {
	Pool *pgxpool.Pool
}

// NewPool initializes a new PostgreSQL connection pool with sensible pool limits and verifies reachability.
// If the database is unreachable, it returns an error so the caller can log and continue in degraded mode.
func NewPool(ctx context.Context, databaseURL string) (*DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL cannot be empty")
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database configuration: %w", err)
	}

	// Configure connection pool thresholds
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping database with a 3-second timeout to confirm reachability
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Ping verifies active database reachability with a 3-second timeout.
func (d *DB) Ping(ctx context.Context) error {
	if d == nil || d.Pool == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return d.Pool.Ping(pingCtx)
}

// Close safely drains and shuts down the connection pool.
func (d *DB) Close() {
	if d != nil && d.Pool != nil {
		d.Pool.Close()
	}
}
