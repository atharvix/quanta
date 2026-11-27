package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
)

type CounterpartyRepository struct {
	db DBTX
}

func NewCounterpartyRepository(db DBTX) *CounterpartyRepository {
	return &CounterpartyRepository{db: db}
}

func (r *CounterpartyRepository) CreateCounterparty(ctx context.Context, cp domain.Counterparty) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO counterparties (
			id, organization_id, treasury_entity_id, name, currency_code,
			max_exposure_limit, current_exposure, version
		)
		VALUES ($1, $2, $3, $4, $5, $6::NUMERIC(20,4), $7::NUMERIC(20,4), $8)
	`,
		cp.ID,
		cp.OrganizationID,
		cp.TreasuryEntityID,
		cp.Name,
		cp.CurrencyCode,
		cp.MaxExposureLimit.String(),
		cp.CurrentExposure.String(),
		cp.Version,
	)
	return err
}

func (r *CounterpartyRepository) GetCounterparty(ctx context.Context, orgID, counterpartyID uuid.UUID) (domain.Counterparty, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, organization_id, treasury_entity_id, name, currency_code,
		       max_exposure_limit::TEXT, current_exposure::TEXT, version, created_at, updated_at
		FROM counterparties
		WHERE organization_id = $1 AND id = $2
	`, orgID, counterpartyID)
	cp, err := scanCounterpartyRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Counterparty{}, domain.ErrNotFound
	}
	return cp, err
}

func (r *CounterpartyRepository) ListCounterpartiesByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.Counterparty, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, treasury_entity_id, name, currency_code,
		       max_exposure_limit::TEXT, current_exposure::TEXT, version, created_at, updated_at
		FROM counterparties
		WHERE organization_id = $1 AND treasury_entity_id = $2
		ORDER BY name ASC
	`, orgID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cps []domain.Counterparty
	for rows.Next() {
		cp, err := scanCounterpartyRow(rows)
		if err != nil {
			return nil, err
		}
		cps = append(cps, cp)
	}
	return cps, rows.Err()
}

func (r *CounterpartyRepository) UpdateCounterparty(ctx context.Context, cp domain.Counterparty, expectedVersion int64) (domain.Counterparty, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE counterparties
		SET name = $4,
		    max_exposure_limit = $5::NUMERIC(20,4),
		    current_exposure = $6::NUMERIC(20,4),
		    version = version + 1,
		    updated_at = NOW()
		WHERE organization_id = $1 AND id = $2 AND version = $3
		RETURNING id, organization_id, treasury_entity_id, name, currency_code,
		          max_exposure_limit::TEXT, current_exposure::TEXT, version, created_at, updated_at
	`, cp.OrganizationID, cp.ID, expectedVersion, cp.Name, cp.MaxExposureLimit.String(), cp.CurrentExposure.String())
	updated, err := scanCounterpartyRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Counterparty{}, domain.ErrVersionConflict
	}
	return updated, err
}

func scanCounterpartyRow(row pgx.Row) (domain.Counterparty, error) {
	var cp domain.Counterparty
	var maxRaw, curRaw string
	err := row.Scan(
		&cp.ID, &cp.OrganizationID, &cp.TreasuryEntityID, &cp.Name, &cp.CurrencyCode,
		&maxRaw, &curRaw, &cp.Version, &cp.CreatedAt, &cp.UpdatedAt,
	)
	if err != nil {
		return domain.Counterparty{}, err
	}
	maxLim, err := money.FromString(maxRaw)
	if err != nil {
		return domain.Counterparty{}, fmt.Errorf("parse max_exposure_limit %q: %w", maxRaw, err)
	}
	curExp, err := money.FromString(curRaw)
	if err != nil {
		return domain.Counterparty{}, fmt.Errorf("parse current_exposure %q: %w", curRaw, err)
	}
	cp.MaxExposureLimit = maxLim
	cp.CurrentExposure = curExp
	return cp, nil
}
