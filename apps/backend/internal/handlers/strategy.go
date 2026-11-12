package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// StrategyHandler manages AI-driven strategy synthesis and audit log retrieval.
type StrategyHandler struct {
	service *service.StrategyService
}

// NewStrategyHandler constructs a new StrategyHandler instance.
func NewStrategyHandler(service *service.StrategyService) *StrategyHandler {
	return &StrategyHandler{service: service}
}

// ListStrategiesResponse formats the paginated JSON response for strategy logs.
type ListStrategiesResponse struct {
	Data   []domain.StrategyExecutionLog `json:"data"`
	Limit  int                           `json:"limit"`
	Offset int                           `json:"offset"`
}

// Generate handles POST /api/v1/treasuries/{id}/strategies/generate.
func (h *StrategyHandler) Generate(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")

	// 1 MB maximum request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.GenerateStrategyInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	result, err := h.service.GenerateStrategy(r.Context(), treasuryID, input)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid treasury ID format")
			return
		}
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			WriteValidationError(w, valErr.Details)
			return
		}
		if strings.Contains(err.Error(), "algorithm") || strings.Contains(err.Error(), "rejected problem input") {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "treasury not found")
			return
		}
		if errors.Is(err, domain.ErrUpstreamUnavailable) {
			WriteError(w, http.StatusBadGateway, "AI engine unavailable")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		log.Printf("[ERROR] Strategy generation failed: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusCreated, result)
}

// ListByTreasury handles GET /api/v1/treasuries/{id}/strategies.
func (h *StrategyHandler) ListByTreasury(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")

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

	logs, err := h.service.ListStrategiesByTreasuryID(r.Context(), treasuryID, limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid treasury ID format")
			return
		}
		if errors.Is(err, domain.ErrInvalidPagination) {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		log.Printf("[ERROR] Failed to list strategy logs: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, ListStrategiesResponse{
		Data:   logs,
		Limit:  limit,
		Offset: offset,
	})
}

// GetByID handles GET /api/v1/strategies/{id}.
func (h *StrategyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	strategyLog, err := h.service.GetStrategyByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid strategy ID format")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "strategy log not found")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		log.Printf("[ERROR] Failed to get strategy log: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, strategyLog)
}
