package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

type AuditHandler struct {
	auditRepo repository.AuditRepository
}

func NewAuditHandler(auditRepo repository.AuditRepository) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo}
}

func (h *AuditHandler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := h.auditRepo.ListAuditEvents(r.Context(), orgCtx.OrganizationID, limit)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		out = append(out, map[string]any{
			"id":              ev.ID,
			"organization_id": ev.OrganizationID,
			"actor_id":        ev.ActorID,
			"actor_role":      ev.ActorRole,
			"action":          ev.Action,
			"entity_type":     ev.EntityType,
			"entity_id":       ev.EntityID,
			"correlation_id":  ev.CorrelationID,
			"metadata":        ev.Metadata,
			"created_at":      ev.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"audit_events": out})
}
