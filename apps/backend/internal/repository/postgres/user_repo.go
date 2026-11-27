package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.scanUser(ctx, `
		SELECT id, email, password_hash, full_name, is_active, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`, email)
}

func (r *UserRepository) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return r.scanUser(ctx, `
		SELECT id, email, password_hash, full_name, is_active, created_at
		FROM users
		WHERE id = $1
	`, userID)
}

func (r *UserRepository) ListMembershipsByUser(ctx context.Context, userID uuid.UUID) ([]domain.OrganizationMembership, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, user_id, role, status, created_at
		FROM organization_memberships
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memberships []domain.OrganizationMembership
	for rows.Next() {
		var m domain.OrganizationMembership
		var role, status string
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &role, &status, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Role = domain.MemberRole(role)
		m.Status = domain.OrganizationStatus(status)
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

func (r *UserRepository) scanUser(ctx context.Context, query string, arg any) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, query, arg).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.IsActive, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}
