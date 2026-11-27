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

type LiquidityPolicyRepository struct {
	db DBTX
}

func NewLiquidityPolicyRepository(db DBTX) *LiquidityPolicyRepository {
	return &LiquidityPolicyRepository{db: db}
}

func (r *LiquidityPolicyRepository) CreatePolicy(ctx context.Context, policy domain.LiquidityPolicy) error {
	if policy.IsActive {
		if _, err := r.db.Exec(ctx, `
			UPDATE liquidity_policies
			SET is_active = FALSE
			WHERE organization_id = $1 AND treasury_entity_id = $2 AND is_active = TRUE
		`, policy.OrganizationID, policy.TreasuryEntityID); err != nil {
			return err
		}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO liquidity_policies (
			id, organization_id, treasury_entity_id, version,
			min_operating_liquidity, min_emergency_reserve, is_active, created_by
		)
		VALUES ($1, $2, $3, $4, $5::NUMERIC(20,4), $6::NUMERIC(20,4), $7, $8)
	`,
		policy.ID,
		policy.OrganizationID,
		policy.TreasuryEntityID,
		policy.Version,
		policy.MinOperatingLiquidity.String(),
		policy.MinEmergencyReserve.String(),
		policy.IsActive,
		policy.CreatedBy,
	)
	return err
}

func (r *LiquidityPolicyRepository) GetActivePolicy(ctx context.Context, orgID, entityID uuid.UUID) (domain.LiquidityPolicy, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, organization_id, treasury_entity_id, version,
		       min_operating_liquidity::TEXT, min_emergency_reserve::TEXT,
		       is_active, created_by, created_at
		FROM liquidity_policies
		WHERE organization_id = $1 AND treasury_entity_id = $2 AND is_active = TRUE
		ORDER BY version DESC
		LIMIT 1
	`, orgID, entityID)
	p, err := scanPolicyRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LiquidityPolicy{}, domain.ErrNotFound
	}
	return p, err
}

func (r *LiquidityPolicyRepository) GetPolicyByID(ctx context.Context, orgID, policyID uuid.UUID) (domain.LiquidityPolicy, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, organization_id, treasury_entity_id, version,
		       min_operating_liquidity::TEXT, min_emergency_reserve::TEXT,
		       is_active, created_by, created_at
		FROM liquidity_policies
		WHERE organization_id = $1 AND id = $2
	`, orgID, policyID)
	p, err := scanPolicyRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LiquidityPolicy{}, domain.ErrNotFound
	}
	return p, err
}

func (r *LiquidityPolicyRepository) ListPoliciesByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.LiquidityPolicy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, treasury_entity_id, version,
		       min_operating_liquidity::TEXT, min_emergency_reserve::TEXT,
		       is_active, created_by, created_at
		FROM liquidity_policies
		WHERE organization_id = $1 AND treasury_entity_id = $2
		ORDER BY version DESC
	`, orgID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []domain.LiquidityPolicy
	for rows.Next() {
		p, err := scanPolicyRow(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (r *LiquidityPolicyRepository) UpdatePolicy(ctx context.Context, policy domain.LiquidityPolicy) (domain.LiquidityPolicy, error) {
	if policy.IsActive {
		if _, err := r.db.Exec(ctx, `
			UPDATE liquidity_policies
			SET is_active = FALSE
			WHERE organization_id = $1 AND treasury_entity_id = $2 AND id <> $3 AND is_active = TRUE
		`, policy.OrganizationID, policy.TreasuryEntityID, policy.ID); err != nil {
			return domain.LiquidityPolicy{}, err
		}
	}
	row := r.db.QueryRow(ctx, `
		UPDATE liquidity_policies
		SET min_operating_liquidity = $3::NUMERIC(20,4),
		    min_emergency_reserve = $4::NUMERIC(20,4),
		    is_active = $5
		WHERE organization_id = $1 AND id = $2
		RETURNING id, organization_id, treasury_entity_id, version,
		          min_operating_liquidity::TEXT, min_emergency_reserve::TEXT,
		          is_active, created_by, created_at
	`, policy.OrganizationID, policy.ID, policy.MinOperatingLiquidity.String(), policy.MinEmergencyReserve.String(), policy.IsActive)
	p, err := scanPolicyRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LiquidityPolicy{}, domain.ErrNotFound
	}
	return p, err
}

func scanPolicyRow(row pgx.Row) (domain.LiquidityPolicy, error) {
	var p domain.LiquidityPolicy
	var opRaw, emRaw string
	err := row.Scan(
		&p.ID, &p.OrganizationID, &p.TreasuryEntityID, &p.Version,
		&opRaw, &emRaw, &p.IsActive, &p.CreatedBy, &p.CreatedAt,
	)
	if err != nil {
		return domain.LiquidityPolicy{}, err
	}
	op, err := money.FromString(opRaw)
	if err != nil {
		return domain.LiquidityPolicy{}, fmt.Errorf("parse min_operating_liquidity %q: %w", opRaw, err)
	}
	em, err := money.FromString(emRaw)
	if err != nil {
		return domain.LiquidityPolicy{}, fmt.Errorf("parse min_emergency_reserve %q: %w", emRaw, err)
	}
	p.MinOperatingLiquidity = op
	p.MinEmergencyReserve = em
	return p, nil
}
