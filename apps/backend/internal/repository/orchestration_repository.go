// Package repository provides data access interfaces and pgx PostgreSQL implementations.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

// OrchestrationRepository defines persistence operations for orchestration runs.
type OrchestrationRepository interface {
	Create(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error)
	GetByID(ctx context.Context, id string) (*domain.OrchestrationRun, error)
}

// PgxOrchestrationRepository implements OrchestrationRepository backed by pgxpool.
type PgxOrchestrationRepository struct {
	db *database.DB
}

// NewPgxOrchestrationRepository constructs an OrchestrationRepository instance.
func NewPgxOrchestrationRepository(db *database.DB) *PgxOrchestrationRepository {
	return &PgxOrchestrationRepository{db: db}
}

// Create inserts an orchestration run record.
func (r *PgxOrchestrationRepository) Create(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	query := `
		INSERT INTO orchestration_runs (
			treasury_id,
			status,
			ai_status,
			optimization_status,
			strategy_log_id,
			response_json,
			error_message,
			duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, treasury_id, status, ai_status, optimization_status, strategy_log_id, response_json, error_message, duration_ms, created_at;
	`

	var saved domain.OrchestrationRun
	err := pool.QueryRow(
		ctx,
		query,
		run.TreasuryID,
		run.Status,
		run.AIStatus,
		run.OptimizationStatus,
		run.StrategyLogID,
		run.ResponseJSON,
		run.ErrorMessage,
		run.DurationMS,
	).Scan(
		&saved.ID,
		&saved.TreasuryID,
		&saved.Status,
		&saved.AIStatus,
		&saved.OptimizationStatus,
		&saved.StrategyLogID,
		&saved.ResponseJSON,
		&saved.ErrorMessage,
		&saved.DurationMS,
		&saved.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert orchestration run: %w", err)
	}

	return &saved, nil
}

// GetByID retrieves an orchestration run by UUID.
func (r *PgxOrchestrationRepository) GetByID(ctx context.Context, id string) (*domain.OrchestrationRun, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	query := `
		SELECT id, treasury_id, status, ai_status, optimization_status, strategy_log_id, response_json, error_message, duration_ms, created_at
		FROM orchestration_runs
		WHERE id = $1;
	`

	var run domain.OrchestrationRun
	err := pool.QueryRow(ctx, query, id).Scan(
		&run.ID,
		&run.TreasuryID,
		&run.Status,
		&run.AIStatus,
		&run.OptimizationStatus,
		&run.StrategyLogID,
		&run.ResponseJSON,
		&run.ErrorMessage,
		&run.DurationMS,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get orchestration run: %w", err)
	}

	return &run, nil
}
