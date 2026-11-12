package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// OrchestrationHandler manages multi-engine allocation orchestration endpoints.
type OrchestrationHandler struct {
	service *service.OrchestrationService
}

// NewOrchestrationHandler constructs an OrchestrationHandler instance.
func NewOrchestrationHandler(service *service.OrchestrationService) *OrchestrationHandler {
	return &OrchestrationHandler{service: service}
}

// Allocate handles POST /api/v1/orchestrate/allocation.
func (h *OrchestrationHandler) Allocate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.OrchestrateAllocationInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	resp, err := h.service.Orchestrate(r.Context(), input)
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
		if strings.Contains(err.Error(), "algorithm") || strings.Contains(err.Error(), "rejected problem input") {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("[ERROR] Orchestration allocation failed: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// GetRunByID handles GET /api/v1/orchestrate/runs/{id}.
func (h *OrchestrationHandler) GetRunByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	run, err := h.service.GetRunByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid orchestration run ID format")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "orchestration run not found")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, run)
}
