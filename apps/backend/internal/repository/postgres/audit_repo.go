package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type AuditRepository struct {
	db DBTX
}

func NewAuditRepository(db DBTX) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) AppendAuditEvent(ctx context.Context, event domain.AuditEvent) error {
	metadata := event.Metadata
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_events (
			id, organization_id, actor_id, actor_role, action,
			entity_type, entity_id, correlation_id, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::JSONB)
	`,
		event.ID,
		event.OrganizationID,
		event.ActorID,
		event.ActorRole,
		event.Action,
		event.EntityType,
		event.EntityID,
		event.CorrelationID,
		string(metadata),
	)
	return err
}

func (r *AuditRepository) ListAuditEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, actor_id, actor_role, action,
		       entity_type, entity_id, correlation_id, metadata, created_at
		FROM audit_events
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(
			&e.ID, &e.OrganizationID, &e.ActorID, &e.ActorRole, &e.Action,
			&e.EntityType, &e.EntityID, &e.CorrelationID, &e.Metadata, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
