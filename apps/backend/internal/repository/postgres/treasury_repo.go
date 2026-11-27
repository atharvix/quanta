package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type TreasuryRepository struct {
	db DBTX
}

func NewTreasuryRepository(db DBTX) *TreasuryRepository {
	return &TreasuryRepository{db: db}
}

func (r *TreasuryRepository) CreateTreasuryEntity(ctx context.Context, entity domain.TreasuryEntity) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO treasury_entities (id, organization_id, name, base_currency, is_synthetic)
		VALUES ($1, $2, $3, $4, $5)
	`, entity.ID, entity.OrganizationID, entity.Name, entity.BaseCurrency, entity.IsSynthetic)
	return err
}

func (r *TreasuryRepository) GetTreasuryEntity(ctx context.Context, orgID, entityID uuid.UUID) (domain.TreasuryEntity, error) {
	var e domain.TreasuryEntity
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, name, base_currency, is_synthetic, created_at
		FROM treasury_entities
		WHERE organization_id = $1 AND id = $2
	`, orgID, entityID).Scan(&e.ID, &e.OrganizationID, &e.Name, &e.BaseCurrency, &e.IsSynthetic, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TreasuryEntity{}, domain.ErrNotFound
	}
	return e, err
}

func (r *TreasuryRepository) ListTreasuryEntities(ctx context.Context, orgID uuid.UUID) ([]domain.TreasuryEntity, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, name, base_currency, is_synthetic, created_at
		FROM treasury_entities
		WHERE organization_id = $1
		ORDER BY name ASC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []domain.TreasuryEntity
	for rows.Next() {
		var e domain.TreasuryEntity
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.Name, &e.BaseCurrency, &e.IsSynthetic, &e.CreatedAt); err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}
