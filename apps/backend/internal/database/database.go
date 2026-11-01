// Package database manages PostgreSQL connection pooling via pgxpool with thread-safe background retry.
package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pinger defines an interface for health-checking database connectivity.
// This interface allows handlers and unit tests to verify database availability without tight coupling.
type Pinger interface {
	Ping(ctx context.Context) error
}

// DB wraps the pgxpool connection pool in a thread-safe container with background reconnection capability.
type DB struct {
	mu          sync.RWMutex
	pool        *pgxpool.Pool
	databaseURL string
}

// NewDB constructs a database container with configuration for connection pooling and retry.
func NewDB(databaseURL string) *DB {
	return &DB{
		databaseURL: databaseURL,
	}
}

// Connect attempts an immediate connection and ping to PostgreSQL with pool configuration.
func (d *DB) Connect(ctx context.Context) error {
	if d.databaseURL == "" {
		return fmt.Errorf("database URL cannot be empty")
	}

	poolConfig, err := pgxpool.ParseConfig(d.databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database configuration: %w", err)
	}

	// Configure enterprise connection pool thresholds
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping database with a 3-second timeout to confirm reachability
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	d.mu.Lock()
	if d.pool != nil {
		d.pool.Close()
	}
	d.pool = pool
	d.mu.Unlock()

	return nil
}

// Pool returns the active *pgxpool.Pool in a thread-safe manner, or nil if currently disconnected.
func (d *DB) Pool() *pgxpool.Pool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.pool
}

// IsConnected returns true if the connection pool is active and initialized.
func (d *DB) IsConnected() bool {
	return d.Pool() != nil
}

// Ping verifies active database reachability with a 3-second timeout.
func (d *DB) Ping(ctx context.Context) error {
	p := d.Pool()
	if p == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return p.Ping(pingCtx)
}

// StartBackgroundRetry periodically attempts to connect to PostgreSQL every interval (e.g. 5 seconds)
// until connected or until ctx is cancelled during graceful shutdown.
func (d *DB) StartBackgroundRetry(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// If already connected, verify it's still alive
				if d.IsConnected() {
					if err := d.Ping(ctx); err == nil {
						continue
					}
					// Connection was lost, reset pool and prepare to retry
					log.Println("[WARN] PostgreSQL connection lost, attempting reconnection...")
					d.mu.Lock()
					if d.pool != nil {
						d.pool.Close()
						d.pool = nil
					}
					d.mu.Unlock()
				}

				// Attempt reconnection
				retryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				err := d.Connect(retryCtx)
				cancel()

				if err == nil {
					log.Println("[INFO] database connected after retry")
				}
			}
		}
	}()
}

// Close safely drains and shuts down the connection pool.
func (d *DB) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pool != nil {
		d.pool.Close()
		d.pool = nil
	}
}

// NewPool initializes a new PostgreSQL connection pool and attempts an immediate connection.
// Maintained for backward compatibility with Step 1 bootstrap.
func NewPool(ctx context.Context, databaseURL string) (*DB, error) {
	d := NewDB(databaseURL)
	if err := d.Connect(ctx); err != nil {
		return d, err
	}
	return d, nil
}
