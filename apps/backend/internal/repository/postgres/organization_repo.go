package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type OrganizationRepository struct {
	db DBTX
}

func NewOrganizationRepository(db DBTX) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) CreateOrganization(ctx context.Context, org domain.Organization) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO organizations (id, name, status)
		VALUES ($1, $2, $3)
	`, org.ID, org.Name, string(org.Status))
	return err
}

func (r *OrganizationRepository) GetOrganization(ctx context.Context, orgID uuid.UUID) (domain.Organization, error) {
	var org domain.Organization
	var status string
	err := r.db.QueryRow(ctx, `
		SELECT id, name, status, created_at
		FROM organizations
		WHERE id = $1
	`, orgID).Scan(&org.ID, &org.Name, &status, &org.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Organization{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Organization{}, err
	}
	org.Status = domain.OrganizationStatus(status)
	return org, nil
}

func (r *OrganizationRepository) GetMembership(ctx context.Context, orgID, userID uuid.UUID) (domain.OrganizationMembership, error) {
	var m domain.OrganizationMembership
	var role, status string
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, user_id, role, status, created_at
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2
	`, orgID, userID).Scan(&m.ID, &m.OrganizationID, &m.UserID, &role, &status, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrganizationMembership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.OrganizationMembership{}, err
	}
	m.Role = domain.MemberRole(role)
	m.Status = domain.OrganizationStatus(status)
	return m, nil
}
