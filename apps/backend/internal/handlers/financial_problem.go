package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// FinancialProblemHandler manages financial problem creation, AI generation, and retrieval.
type FinancialProblemHandler struct {
	service *service.FinancialProblemService
}

// NewFinancialProblemHandler constructs a FinancialProblemHandler instance.
func NewFinancialProblemHandler(service *service.FinancialProblemService) *FinancialProblemHandler {
	return &FinancialProblemHandler{service: service}
}

// ListFinancialProblemsResponse formats paginated problem queries.
type ListFinancialProblemsResponse struct {
	Data   []domain.FinancialProblemEntity `json:"data"`
	Limit  int                             `json:"limit"`
	Offset int                             `json:"offset"`
}

// Create handles POST /api/v1/financial-problems.
func (h *FinancialProblemHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.CreateFinancialProblemInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	resp, err := h.service.Create(r.Context(), input)
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
			WriteError(w, http.StatusBadGateway, "upstream AI decision engine unavailable")
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
		WriteError(w, http.StatusInternalServerError, "internal server error: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// GetByID handles GET /api/v1/financial-problems/{id}.
func (h *FinancialProblemHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	fp, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid problem ID format")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "financial problem not found")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, fp)
}

// List handles GET /api/v1/financial-problems with pagination and treasury_id filter.
func (h *FinancialProblemHandler) List(w http.ResponseWriter, r *http.Request) {
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

	treasuryID := r.URL.Query().Get("treasury_id")

	problems, err := h.service.List(r.Context(), limit, offset, treasuryID)
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
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, ListFinancialProblemsResponse{
		Data:   problems,
		Limit:  limit,
		Offset: offset,
	})
}
