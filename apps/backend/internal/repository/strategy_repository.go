package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

// StrategyRepository handles database persistence of strategy execution audit logs.
type StrategyRepository interface {
	CreateLog(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error)
	GetLogByID(ctx context.Context, id string) (*domain.StrategyExecutionLog, error)
	ListLogsByTreasuryID(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error)
}

// PgxStrategyRepository implements StrategyRepository using pgxpool.
type PgxStrategyRepository struct {
	db *database.DB
}

// NewPgxStrategyRepository constructs a strategy repository backed by PostgreSQL.
func NewPgxStrategyRepository(db *database.DB) *PgxStrategyRepository {
	return &PgxStrategyRepository{db: db}
}

// validateNumericBounds guarantees metrics comply with PostgreSQL NUMERIC column constraints
// to avoid uncaught driver panics or database-level constraint violations.
func validateNumericBounds(log *domain.StrategyExecutionLog) error {
	const maxNum54 = 9.9999
	const minNum54 = -9.9999

	if math.IsNaN(log.RiskScore) || log.RiskScore < minNum54 || log.RiskScore > maxNum54 {
		return fmt.Errorf("risk_score %.4f exceeds PostgreSQL NUMERIC(5,4) bounds [-9.9999, 9.9999]", log.RiskScore)
	}
	if math.IsNaN(log.ExpectedReturn) || log.ExpectedReturn < minNum54 || log.ExpectedReturn > maxNum54 {
		return fmt.Errorf("expected_return %.4f exceeds PostgreSQL NUMERIC(5,4) bounds [-9.9999, 9.9999]", log.ExpectedReturn)
	}
	if math.IsNaN(log.LiquidityRatio) || log.LiquidityRatio < minNum54 || log.LiquidityRatio > maxNum54 {
		return fmt.Errorf("liquidity_ratio %.4f exceeds PostgreSQL NUMERIC(5,4) bounds [-9.9999, 9.9999]", log.LiquidityRatio)
	}
	if math.IsNaN(log.ObjectiveValue) || log.ObjectiveValue < -999999.999999 || log.ObjectiveValue > 999999.999999 {
		return fmt.Errorf("objective_value %.6f exceeds PostgreSQL NUMERIC(12,6) bounds", log.ObjectiveValue)
	}
	return nil
}

// CreateLog writes a complete execution log record to strategy_execution_logs.
func (r *PgxStrategyRepository) CreateLog(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if err := validateNumericBounds(&log); err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		INSERT INTO strategy_execution_logs (
			treasury_id, algorithm_used, objective_value, risk_score, expected_return,
			liquidity_ratio, allocations_json, constraints_audit_json, execution_time_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text, treasury_id::text, algorithm_used, objective_value, risk_score,
		          expected_return, liquidity_ratio, allocations_json, constraints_audit_json,
		          execution_time_ms, created_at;
	`

	var saved domain.StrategyExecutionLog
	err := r.db.Pool().QueryRow(
		timeoutCtx,
		query,
		log.TreasuryID,
		log.AlgorithmUsed,
		log.ObjectiveValue,
		log.RiskScore,
		log.ExpectedReturn,
		log.LiquidityRatio,
		log.AllocationsJSON,
		log.ConstraintsAuditJSON,
		log.ExecutionTimeMS,
	).Scan(
		&saved.ID,
		&saved.TreasuryID,
		&saved.AlgorithmUsed,
		&saved.ObjectiveValue,
		&saved.RiskScore,
		&saved.ExpectedReturn,
		&saved.LiquidityRatio,
		&saved.AllocationsJSON,
		&saved.ConstraintsAuditJSON,
		&saved.ExecutionTimeMS,
		&saved.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &saved, nil
}

// GetLogByID retrieves a single strategy execution audit log by its UUID.
func (r *PgxStrategyRepository) GetLogByID(ctx context.Context, id string) (*domain.StrategyExecutionLog, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, treasury_id::text, algorithm_used, objective_value, risk_score,
		       expected_return, liquidity_ratio, allocations_json, constraints_audit_json,
		       execution_time_ms, created_at
		FROM strategy_execution_logs
		WHERE id = $1;
	`

	var log domain.StrategyExecutionLog
	err := r.db.Pool().QueryRow(timeoutCtx, query, id).Scan(
		&log.ID,
		&log.TreasuryID,
		&log.AlgorithmUsed,
		&log.ObjectiveValue,
		&log.RiskScore,
		&log.ExpectedReturn,
		&log.LiquidityRatio,
		&log.AllocationsJSON,
		&log.ConstraintsAuditJSON,
		&log.ExecutionTimeMS,
		&log.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &log, nil
}

// ListLogsByTreasuryID queries execution logs for a corporate treasury sorted newest first.
func (r *PgxStrategyRepository) ListLogsByTreasuryID(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, treasury_id::text, algorithm_used, objective_value, risk_score,
		       expected_return, liquidity_ratio, allocations_json, constraints_audit_json,
		       execution_time_ms, created_at
		FROM strategy_execution_logs
		WHERE treasury_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := r.db.Pool().Query(timeoutCtx, query, treasuryID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]domain.StrategyExecutionLog, 0)
	for rows.Next() {
		var l domain.StrategyExecutionLog
		err := rows.Scan(
			&l.ID,
			&l.TreasuryID,
			&l.AlgorithmUsed,
			&l.ObjectiveValue,
			&l.RiskScore,
			&l.ExpectedReturn,
			&l.LiquidityRatio,
			&l.AllocationsJSON,
			&l.ConstraintsAuditJSON,
			&l.ExecutionTimeMS,
			&l.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}
