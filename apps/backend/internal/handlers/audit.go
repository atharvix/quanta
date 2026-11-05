package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

// AuditHandler manages HTTP endpoints for system audit log inspection.
type AuditHandler struct {
	service *service.AuditService
}

// NewAuditHandler constructs an AuditHandler instance.
func NewAuditHandler(service *service.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

// ListAuditLogsResponse formats paginated audit log queries.
type ListAuditLogsResponse struct {
	Data   []domain.AuditLog `json:"data"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// List handles GET /api/v1/audit-logs with optional filters and pagination.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil {
			limit = val
		} else {
			WriteError(w, http.StatusBadRequest, "query parameter 'limit' must be an integer")
			return
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if val, err := strconv.Atoi(oStr); err == nil {
			offset = val
		} else {
			WriteError(w, http.StatusBadRequest, "query parameter 'offset' must be an integer")
			return
		}
	}

	entityType := r.URL.Query().Get("entity_type")
	action := r.URL.Query().Get("action")

	logs, err := h.service.List(r.Context(), limit, offset, entityType, action)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPagination) {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, ListAuditLogsResponse{
		Data:   logs,
		Limit:  limit,
		Offset: offset,
	})
}
