package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pinger abstracts database reachability checks for readiness probes and testing.
type Pinger interface {
	Ping(ctx context.Context) error
}

// DB manages a PostgreSQL connection pool.
type DB struct {
	databaseURL string
	mu          sync.RWMutex
	pool        *pgxpool.Pool
}

// NewDB creates a DB wrapper configured with the given connection string.
func NewDB(databaseURL string) *DB {
	return &DB{databaseURL: databaseURL}
}

// Connect initializes the pgxpool and verifies connectivity with a bounded ping.
func (d *DB) Connect(ctx context.Context) error {
	if d.databaseURL == "" {
		return errors.New("database URL is empty")
	}

	poolCfg, err := pgxpool.ParseConfig(d.databaseURL)
	if err != nil {
		return fmt.Errorf("parse database config: %w", err)
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}

	d.mu.Lock()
	old := d.pool
	d.pool = pool
	d.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

// Ping verifies PostgreSQL connectivity using the active pool.
func (d *DB) Ping(ctx context.Context) error {
	d.mu.RLock()
	pool := d.pool
	d.mu.RUnlock()

	if pool == nil {
		return errors.New("database pool is not initialized")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return pool.Ping(pingCtx)
}

// Pool returns the underlying pgxpool.Pool, or nil if not connected.
func (d *DB) Pool() *pgxpool.Pool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.pool
}

// Close shuts down the connection pool if initialized.
func (d *DB) Close() {
	d.mu.Lock()
	pool := d.pool
	d.pool = nil
	d.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
}
