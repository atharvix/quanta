// Package repository provides data access interfaces and pgx PostgreSQL implementations.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

// FinancialProblemRepository defines persistence operations for canonical financial problems.
type FinancialProblemRepository interface {
	Create(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error)
	GetByID(ctx context.Context, id string) (*domain.FinancialProblemEntity, error)
	List(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error)
}

// PgxFinancialProblemRepository implements FinancialProblemRepository backed by pgxpool.
type PgxFinancialProblemRepository struct {
	db *database.DB
}

// NewPgxFinancialProblemRepository constructs a FinancialProblemRepository instance.
func NewPgxFinancialProblemRepository(db *database.DB) *PgxFinancialProblemRepository {
	return &PgxFinancialProblemRepository{db: db}
}

// Create inserts a financial problem record with linked strategy_log_id.
func (r *PgxFinancialProblemRepository) Create(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	query := `
		INSERT INTO financial_problems (
			treasury_id,
			algorithm,
			objective,
			market_state,
			problem_json,
			strategy_log_id
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, treasury_id, algorithm, objective, market_state, problem_json, strategy_log_id, created_at;
	`

	var saved domain.FinancialProblemEntity
	err := pool.QueryRow(
		ctx,
		query,
		fp.TreasuryID,
		fp.Algorithm,
		fp.Objective,
		fp.MarketState,
		fp.ProblemJSON,
		fp.StrategyLogID,
	).Scan(
		&saved.ID,
		&saved.TreasuryID,
		&saved.Algorithm,
		&saved.Objective,
		&saved.MarketState,
		&saved.ProblemJSON,
		&saved.StrategyLogID,
		&saved.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert financial problem: %w", err)
	}

	return &saved, nil
}

// GetByID retrieves a financial problem by UUID.
func (r *PgxFinancialProblemRepository) GetByID(ctx context.Context, id string) (*domain.FinancialProblemEntity, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	query := `
		SELECT id, treasury_id, algorithm, objective, market_state, problem_json, strategy_log_id, created_at
		FROM financial_problems
		WHERE id = $1;
	`

	var fp domain.FinancialProblemEntity
	err := pool.QueryRow(ctx, query, id).Scan(
		&fp.ID,
		&fp.TreasuryID,
		&fp.Algorithm,
		&fp.Objective,
		&fp.MarketState,
		&fp.ProblemJSON,
		&fp.StrategyLogID,
		&fp.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get financial problem: %w", err)
	}

	return &fp, nil
}

// List retrieves a paginated list of financial problems with optional treasury_id filter.
func (r *PgxFinancialProblemRepository) List(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	var sb strings.Builder
	sb.WriteString(`
		SELECT id, treasury_id, algorithm, objective, market_state, problem_json, strategy_log_id, created_at
		FROM financial_problems
		WHERE 1=1
	`)

	args := []any{}
	argIdx := 1

	if strings.TrimSpace(treasuryID) != "" {
		sb.WriteString(fmt.Sprintf(" AND treasury_id = $%d", argIdx))
		args = append(args, strings.TrimSpace(treasuryID))
		argIdx++
	}

	sb.WriteString(fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d;", argIdx, argIdx+1))
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query financial problems: %w", err)
	}
	defer rows.Close()

	var problems []domain.FinancialProblemEntity
	for rows.Next() {
		var p domain.FinancialProblemEntity
		if err := rows.Scan(
			&p.ID,
			&p.TreasuryID,
			&p.Algorithm,
			&p.Objective,
			&p.MarketState,
			&p.ProblemJSON,
			&p.StrategyLogID,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan financial problem row: %w", err)
		}
		problems = append(problems, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading financial problems rows: %w", err)
	}

	if problems == nil {
		problems = []domain.FinancialProblemEntity{}
	}

	return problems, nil
}
