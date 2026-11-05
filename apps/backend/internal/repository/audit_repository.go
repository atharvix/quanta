// Package repository provides data access interfaces and pgx PostgreSQL implementations.
package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// AuditRepository defines persistence operations for platform audit records.
type AuditRepository interface {
	Create(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error)
	List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error)
}

// PgxAuditRepository implements AuditRepository backed by PostgreSQL via pgxpool.
type PgxAuditRepository struct {
	db *database.DB
}

// NewPgxAuditRepository constructs an AuditRepository instance.
func NewPgxAuditRepository(db *database.DB) *PgxAuditRepository {
	return &PgxAuditRepository{db: db}
}

// Create inserts a new audit log record.
func (r *PgxAuditRepository) Create(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	query := `
		INSERT INTO audit_logs (
			action,
			entity_type,
			entity_id,
			actor,
			metadata
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, action, entity_type, entity_id, actor, metadata, created_at;
	`

	var saved domain.AuditLog
	err := pool.QueryRow(
		ctx,
		query,
		log.Action,
		log.EntityType,
		log.EntityID,
		log.Actor,
		log.Metadata,
	).Scan(
		&saved.ID,
		&saved.Action,
		&saved.EntityType,
		&saved.EntityID,
		&saved.Actor,
		&saved.Metadata,
		&saved.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert audit log: %w", err)
	}

	return &saved, nil
}

// List queries audit logs ordered newest first with optional entity_type and action filters.
func (r *PgxAuditRepository) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	var sb strings.Builder
	sb.WriteString(`
		SELECT id, action, entity_type, entity_id, actor, metadata, created_at
		FROM audit_logs
		WHERE 1=1
	`)

	args := []any{}
	argIdx := 1

	if strings.TrimSpace(entityType) != "" {
		sb.WriteString(fmt.Sprintf(" AND entity_type = $%d", argIdx))
		args = append(args, strings.TrimSpace(entityType))
		argIdx++
	}

	if strings.TrimSpace(action) != "" {
		sb.WriteString(fmt.Sprintf(" AND action = $%d", argIdx))
		args = append(args, strings.TrimSpace(action))
		argIdx++
	}

	sb.WriteString(fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d;", argIdx, argIdx+1))
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(
			&a.ID,
			&a.Action,
			&a.EntityType,
			&a.EntityID,
			&a.Actor,
			&a.Metadata,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}
		logs = append(logs, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading audit logs rows: %w", err)
	}

	if logs == nil {
		logs = []domain.AuditLog{}
	}

	return logs, nil
}
