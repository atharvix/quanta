// Package repository provides data access interfaces and PostgreSQL implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

// TreasuryRepository defines database persistence operations for corporate treasuries.
type TreasuryRepository interface {
	List(ctx context.Context, limit, offset int) ([]domain.Treasury, error)
	GetByID(ctx context.Context, id string) (*domain.Treasury, error)
	Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error)
}

// PgxTreasuryRepository implements TreasuryRepository using pgxpool parameterized queries.
type PgxTreasuryRepository struct {
	db *database.DB
}

// NewPgxTreasuryRepository constructs a repository instance tied to the thread-safe database container.
func NewPgxTreasuryRepository(db *database.DB) *PgxTreasuryRepository {
	return &PgxTreasuryRepository{db: db}
}

// List retrieves paginated treasury records sorted by creation timestamp descending.
func (r *PgxTreasuryRepository) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, name, base_currency, total_capital, risk_tolerance, min_liquidity_ratio, created_at, updated_at
		FROM corporate_treasuries
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := r.db.Pool().Query(timeoutCtx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	treasuries := make([]domain.Treasury, 0)
	for rows.Next() {
		var t domain.Treasury
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.BaseCurrency,
			&t.TotalCapital,
			&t.RiskTolerance,
			&t.MinLiquidityRatio,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		treasuries = append(treasuries, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return treasuries, nil
}

// GetByID finds a single treasury record by its UUID.
func (r *PgxTreasuryRepository) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, name, base_currency, total_capital, risk_tolerance, min_liquidity_ratio, created_at, updated_at
		FROM corporate_treasuries
		WHERE id = $1;
	`

	var t domain.Treasury
	err := r.db.Pool().QueryRow(timeoutCtx, query, id).Scan(
		&t.ID,
		&t.Name,
		&t.BaseCurrency,
		&t.TotalCapital,
		&t.RiskTolerance,
		&t.MinLiquidityRatio,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &t, nil
}

// Create inserts a new treasury record and returns the persisted entity with its generated UUID.
func (r *PgxTreasuryRepository) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		INSERT INTO corporate_treasuries (name, base_currency, total_capital, risk_tolerance, min_liquidity_ratio)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, name, base_currency, total_capital, risk_tolerance, min_liquidity_ratio, created_at, updated_at;
	`

	var t domain.Treasury
	err := r.db.Pool().QueryRow(
		timeoutCtx,
		query,
		input.Name,
		input.BaseCurrency,
		input.TotalCapital,
		input.RiskTolerance,
		input.MinLiquidityRatio,
	).Scan(
		&t.ID,
		&t.Name,
		&t.BaseCurrency,
		&t.TotalCapital,
		&t.RiskTolerance,
		&t.MinLiquidityRatio,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
