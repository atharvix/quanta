package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// AuditService coordinates best-effort system audit trail recording and queries.
type AuditService struct {
	repo repository.AuditRepository
}

// NewAuditService constructs an AuditService instance.
func NewAuditService(repo repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

// Record saves an audit entry in a best-effort manner. If insertion fails, it logs a warning
// and does NOT fail the parent business operation.
func (s *AuditService) Record(ctx context.Context, action, entityType string, entityID *string, metadata any) {
	if s == nil || s.repo == nil {
		return
	}

	var metaBytes []byte
	if metadata != nil {
		var err error
		metaBytes, err = json.Marshal(metadata)
		if err != nil {
			log.Printf("[WARN] Failed to serialize audit metadata for action '%s': %v", action, err)
		}
	}

	// Extract authenticated actor email from security context, defaulting to "anonymous"
	actor := domain.ActorFromContext(ctx)

	auditEntry := domain.AuditLog{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Actor:      actor,
		Metadata:   metaBytes,
	}

	if _, err := s.repo.Create(ctx, auditEntry); err != nil {
		log.Printf("[WARN] Best-effort audit log failed for action '%s': %v", action, err)
	}
}

// List queries audit logs ordered newest first with validation and filtering.
func (s *AuditService) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, fmt.Errorf("%w: limit must not exceed 100", domain.ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidPagination)
	}

	return s.repo.List(ctx, limit, offset, entityType, action)
}
