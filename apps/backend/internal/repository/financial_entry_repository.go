// Package repository provides data access interfaces and PostgreSQL implementations.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// FinancialEntryRepository defines database operations for custom assets and liabilities.
type FinancialEntryRepository interface {
	ListAssets(ctx context.Context, treasuryID string) ([]domain.Asset, error)
	GetAsset(ctx context.Context, treasuryID, assetID string) (*domain.Asset, error)
	CreateAsset(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error)
	UpdateAsset(ctx context.Context, treasuryID, assetID string, asset domain.Asset) (*domain.Asset, error)
	DeleteAsset(ctx context.Context, treasuryID, assetID string) error
	BulkSyncAssets(ctx context.Context, treasuryID string, assets []domain.Asset) ([]domain.Asset, error)

	ListLiabilities(ctx context.Context, treasuryID string) ([]domain.Liability, error)
	GetLiability(ctx context.Context, treasuryID, liabilityID string) (*domain.Liability, error)
	CreateLiability(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error)
	UpdateLiability(ctx context.Context, treasuryID, liabilityID string, liability domain.Liability) (*domain.Liability, error)
	DeleteLiability(ctx context.Context, treasuryID, liabilityID string) error
	BulkSyncLiabilities(ctx context.Context, treasuryID string, liabilities []domain.Liability) ([]domain.Liability, error)
}

// PgxFinancialEntryRepository implements FinancialEntryRepository using parameterized PostgreSQL queries.
type PgxFinancialEntryRepository struct {
	db *database.DB
}

// NewPgxFinancialEntryRepository constructs a new repository instance tied to the PostgreSQL pool.
func NewPgxFinancialEntryRepository(db *database.DB) *PgxFinancialEntryRepository {
	return &PgxFinancialEntryRepository{db: db}
}

// ListAssets returns all custom investment assets configured for a treasury.
func (r *PgxFinancialEntryRepository) ListAssets(ctx context.Context, treasuryID string) ([]domain.Asset, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility, metadata
		FROM treasury_assets
		WHERE treasury_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := r.db.Pool().Query(timeoutCtx, query, treasuryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := make([]domain.Asset, 0)
	for rows.Next() {
		var a domain.Asset
		var rawMeta []byte
		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.AssetType,
			&a.Cost,
			&a.ExpectedReturn,
			&a.Risk,
			&a.Liquidity,
			&a.Volatility,
			&rawMeta,
		)
		if err != nil {
			return nil, err
		}
		if len(rawMeta) > 0 {
			_ = json.Unmarshal(rawMeta, &a.Metadata)
		}
		assets = append(assets, a)
	}

	return assets, rows.Err()
}

// GetAsset retrieves a single asset by identifier within the scope of a treasury.
func (r *PgxFinancialEntryRepository) GetAsset(ctx context.Context, treasuryID, assetID string) (*domain.Asset, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility, metadata
		FROM treasury_assets
		WHERE treasury_id = $1 AND asset_id = $2;
	`

	var a domain.Asset
	var rawMeta []byte
	err := r.db.Pool().QueryRow(timeoutCtx, query, treasuryID, assetID).Scan(
		&a.ID,
		&a.Name,
		&a.AssetType,
		&a.Cost,
		&a.ExpectedReturn,
		&a.Risk,
		&a.Liquidity,
		&a.Volatility,
		&rawMeta,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &a.Metadata)
	}

	return &a, nil
}

// CreateAsset inserts a new investment asset for the designated treasury.
func (r *PgxFinancialEntryRepository) CreateAsset(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var metaJSON []byte
	var err error
	if asset.Metadata != nil {
		metaJSON, err = json.Marshal(asset.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize asset metadata: %w", err)
		}
	}

	const query = `
		INSERT INTO treasury_assets (treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility, metadata, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		RETURNING asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility, metadata;
	`

	var a domain.Asset
	var rawMeta []byte
	err = r.db.Pool().QueryRow(
		timeoutCtx,
		query,
		treasuryID,
		asset.ID,
		asset.Name,
		asset.AssetType,
		asset.Cost,
		asset.ExpectedReturn,
		asset.Risk,
		asset.Liquidity,
		asset.Volatility,
		metaJSON,
	).Scan(
		&a.ID,
		&a.Name,
		&a.AssetType,
		&a.Cost,
		&a.ExpectedReturn,
		&a.Risk,
		&a.Liquidity,
		&a.Volatility,
		&rawMeta,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrConflict
		}
		return nil, err
	}
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &a.Metadata)
	}

	return &a, nil
}

// UpdateAsset modifies an existing asset in the specified treasury.
func (r *PgxFinancialEntryRepository) UpdateAsset(ctx context.Context, treasuryID, assetID string, asset domain.Asset) (*domain.Asset, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var metaJSON []byte
	var err error
	if asset.Metadata != nil {
		metaJSON, err = json.Marshal(asset.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize asset metadata: %w", err)
		}
	}

	const query = `
		UPDATE treasury_assets
		SET name = $3, asset_type = $4, cost = $5, expected_return = $6, risk = $7, liquidity = $8, volatility = $9, metadata = $10, updated_at = now()
		WHERE treasury_id = $1 AND asset_id = $2
		RETURNING asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility, metadata;
	`

	var a domain.Asset
	var rawMeta []byte
	err = r.db.Pool().QueryRow(
		timeoutCtx,
		query,
		treasuryID,
		assetID,
		asset.Name,
		asset.AssetType,
		asset.Cost,
		asset.ExpectedReturn,
		asset.Risk,
		asset.Liquidity,
		asset.Volatility,
		metaJSON,
	).Scan(
		&a.ID,
		&a.Name,
		&a.AssetType,
		&a.Cost,
		&a.ExpectedReturn,
		&a.Risk,
		&a.Liquidity,
		&a.Volatility,
		&rawMeta,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &a.Metadata)
	}

	return &a, nil
}

// DeleteAsset removes a candidate asset from the treasury's universe.
func (r *PgxFinancialEntryRepository) DeleteAsset(ctx context.Context, treasuryID, assetID string) error {
	if r.db == nil || r.db.Pool() == nil {
		return domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		DELETE FROM treasury_assets
		WHERE treasury_id = $1 AND asset_id = $2;
	`

	tag, err := r.db.Pool().Exec(timeoutCtx, query, treasuryID, assetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// BulkSyncAssets atomically replaces all assets for a treasury inside a single transaction.
func (r *PgxFinancialEntryRepository) BulkSyncAssets(ctx context.Context, treasuryID string, assets []domain.Asset) ([]domain.Asset, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := r.db.Pool().Begin(timeoutCtx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(timeoutCtx) }()

	// Clear existing assets for this treasury
	if _, err := tx.Exec(timeoutCtx, "DELETE FROM treasury_assets WHERE treasury_id = $1", treasuryID); err != nil {
		return nil, err
	}

	// Insert each asset
	const insertQuery = `
		INSERT INTO treasury_assets (treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility, metadata, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now());
	`

	for _, a := range assets {
		var metaJSON []byte
		if a.Metadata != nil {
			metaJSON, _ = json.Marshal(a.Metadata)
		}
		_, err := tx.Exec(
			timeoutCtx,
			insertQuery,
			treasuryID,
			a.ID,
			a.Name,
			a.AssetType,
			a.Cost,
			a.ExpectedReturn,
			a.Risk,
			a.Liquidity,
			a.Volatility,
			metaJSON,
		)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, domain.ErrConflict
			}
			return nil, err
		}
	}

	if err := tx.Commit(timeoutCtx); err != nil {
		return nil, err
	}

	return r.ListAssets(ctx, treasuryID)
}

// ListLiabilities returns all debt obligations configured for a treasury.
func (r *PgxFinancialEntryRepository) ListLiabilities(ctx context.Context, treasuryID string) ([]domain.Liability, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve
		FROM treasury_liabilities
		WHERE treasury_id = $1
		ORDER BY priority ASC, created_at ASC;
	`

	rows, err := r.db.Pool().Query(timeoutCtx, query, treasuryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	liabilities := make([]domain.Liability, 0)
	for rows.Next() {
		var l domain.Liability
		err := rows.Scan(
			&l.ID,
			&l.Name,
			&l.Principal,
			&l.InterestRate,
			&l.MaturityMonths,
			&l.Priority,
			&l.IsMandatoryReserve,
		)
		if err != nil {
			return nil, err
		}
		liabilities = append(liabilities, l)
	}

	return liabilities, rows.Err()
}

// GetLiability retrieves a single liability by identifier within the scope of a treasury.
func (r *PgxFinancialEntryRepository) GetLiability(ctx context.Context, treasuryID, liabilityID string) (*domain.Liability, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve
		FROM treasury_liabilities
		WHERE treasury_id = $1 AND liability_id = $2;
	`

	var l domain.Liability
	err := r.db.Pool().QueryRow(timeoutCtx, query, treasuryID, liabilityID).Scan(
		&l.ID,
		&l.Name,
		&l.Principal,
		&l.InterestRate,
		&l.MaturityMonths,
		&l.Priority,
		&l.IsMandatoryReserve,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &l, nil
}

// CreateLiability inserts a new debt obligation for the designated treasury.
func (r *PgxFinancialEntryRepository) CreateLiability(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		INSERT INTO treasury_liabilities (treasury_id, liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		RETURNING liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve;
	`

	var l domain.Liability
	err := r.db.Pool().QueryRow(
		timeoutCtx,
		query,
		treasuryID,
		liability.ID,
		liability.Name,
		liability.Principal,
		liability.InterestRate,
		liability.MaturityMonths,
		liability.Priority,
		liability.IsMandatoryReserve,
	).Scan(
		&l.ID,
		&l.Name,
		&l.Principal,
		&l.InterestRate,
		&l.MaturityMonths,
		&l.Priority,
		&l.IsMandatoryReserve,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrConflict
		}
		return nil, err
	}

	return &l, nil
}

// UpdateLiability modifies an existing liability in the specified treasury.
func (r *PgxFinancialEntryRepository) UpdateLiability(ctx context.Context, treasuryID, liabilityID string, liability domain.Liability) (*domain.Liability, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		UPDATE treasury_liabilities
		SET name = $3, principal = $4, interest_rate = $5, maturity_months = $6, priority = $7, is_mandatory_reserve = $8, updated_at = now()
		WHERE treasury_id = $1 AND liability_id = $2
		RETURNING liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve;
	`

	var l domain.Liability
	err := r.db.Pool().QueryRow(
		timeoutCtx,
		query,
		treasuryID,
		liabilityID,
		liability.Name,
		liability.Principal,
		liability.InterestRate,
		liability.MaturityMonths,
		liability.Priority,
		liability.IsMandatoryReserve,
	).Scan(
		&l.ID,
		&l.Name,
		&l.Principal,
		&l.InterestRate,
		&l.MaturityMonths,
		&l.Priority,
		&l.IsMandatoryReserve,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &l, nil
}

// DeleteLiability removes a debt obligation from the treasury.
func (r *PgxFinancialEntryRepository) DeleteLiability(ctx context.Context, treasuryID, liabilityID string) error {
	if r.db == nil || r.db.Pool() == nil {
		return domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		DELETE FROM treasury_liabilities
		WHERE treasury_id = $1 AND liability_id = $2;
	`

	tag, err := r.db.Pool().Exec(timeoutCtx, query, treasuryID, liabilityID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// BulkSyncLiabilities atomically replaces all liabilities for a treasury inside a single transaction.
func (r *PgxFinancialEntryRepository) BulkSyncLiabilities(ctx context.Context, treasuryID string, liabilities []domain.Liability) ([]domain.Liability, error) {
	if r.db == nil || r.db.Pool() == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := r.db.Pool().Begin(timeoutCtx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(timeoutCtx) }()

	// Clear existing liabilities for this treasury
	if _, err := tx.Exec(timeoutCtx, "DELETE FROM treasury_liabilities WHERE treasury_id = $1", treasuryID); err != nil {
		return nil, err
	}

	// Insert each liability
	const insertQuery = `
		INSERT INTO treasury_liabilities (treasury_id, liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now());
	`

	for _, l := range liabilities {
		_, err := tx.Exec(
			timeoutCtx,
			insertQuery,
			treasuryID,
			l.ID,
			l.Name,
			l.Principal,
			l.InterestRate,
			l.MaturityMonths,
			l.Priority,
			l.IsMandatoryReserve,
		)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, domain.ErrConflict
			}
			return nil, err
		}
	}

	if err := tx.Commit(timeoutCtx); err != nil {
		return nil, err
	}

	return r.ListLiabilities(ctx, treasuryID)
}

// CleanString strips trailing/leading whitespaces.
func cleanString(s string) string {
	return strings.TrimSpace(s)
}
