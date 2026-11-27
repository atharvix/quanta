package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// DBTX abstracts *pgxpool.Pool and pgx.Tx for shared repository SQL execution.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxManager implements repository.TxManager using pgxpool transactions.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a PostgreSQL transaction manager.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx executes fn inside a PostgreSQL ACID transaction, rolling back on error or panic.
func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context, repos repository.TxRepositories) error) error {
	if m.pool == nil {
		return errors.New("database pool is not initialized")
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin postgres tx: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	repos := repository.TxRepositories{
		Accounts:  NewAccountRepository(tx),
		Ledger:    NewLedgerRepository(tx),
		Audit:     NewAuditRepository(tx),
		Proposals: NewProposalRepository(tx),
		Decisions: NewApprovalDecisionRepository(tx),
	}

	if err := fn(ctx, repos); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres tx: %w", err)
	}
	return nil
}
