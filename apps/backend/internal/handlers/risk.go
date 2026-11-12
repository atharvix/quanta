// Package handlers implements HTTP controllers and converts domain logic to REST API responses.
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

// RiskHandler manages risk analysis, Bayesian reasoning, and adversarial stress testing HTTP endpoints.
type RiskHandler struct {
	service *service.RiskService
}

// NewRiskHandler constructs a new RiskHandler instance.
func NewRiskHandler(service *service.RiskService) *RiskHandler {
	return &RiskHandler{service: service}
}

// AnalyzeRisk handles POST /api/v1/treasuries/{id}/risk/analyze.
func (h *RiskHandler) AnalyzeRisk(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")

	// 1 MB maximum request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.AnalyzeTreasuryRiskInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	result, err := h.service.AnalyzeRisk(r.Context(), treasuryID, input)
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
		if errors.Is(err, domain.ErrForbidden) {
			WriteError(w, http.StatusForbidden, "forbidden: access to this treasury is restricted")
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
		if strings.Contains(err.Error(), "rejected risk analyze input") {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("[ERROR] Risk analysis failed: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// AdversarialStressTest handles POST /api/v1/treasuries/{id}/stress-test.
func (h *RiskHandler) AdversarialStressTest(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")

	// 1 MB maximum request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.AdversarialStressTestInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	result, err := h.service.AdversarialStressTest(r.Context(), treasuryID, input)
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
		if errors.Is(err, domain.ErrForbidden) {
			WriteError(w, http.StatusForbidden, "forbidden: access to this treasury is restricted")
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
		if strings.Contains(err.Error(), "rejected stress test input") {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("[ERROR] Adversarial stress test failed: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, result)
}
