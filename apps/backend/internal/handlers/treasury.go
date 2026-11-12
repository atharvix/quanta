package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// TreasuryHandler handles corporate treasury management endpoints.
type TreasuryHandler struct {
	service *service.TreasuryService
}

// NewTreasuryHandler constructs a new TreasuryHandler instance.
func NewTreasuryHandler(service *service.TreasuryService) *TreasuryHandler {
	return &TreasuryHandler{service: service}
}

// ListResponse models the paginated JSON response for corporate treasuries.
type ListResponse struct {
	Data   []domain.Treasury `json:"data"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// List handles GET /api/v1/treasuries: returns paginated list of treasuries.
func (h *TreasuryHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		parsedLimit, err := strconv.Atoi(lStr)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "query parameter 'limit' must be an integer")
			return
		}
		limit = parsedLimit
	}

	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		parsedOffset, err := strconv.Atoi(oStr)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "query parameter 'offset' must be an integer")
			return
		}
		offset = parsedOffset
	}

	treasuries, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPagination) {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		log.Printf("[ERROR] Failed to list treasuries: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, ListResponse{
		Data:   treasuries,
		Limit:  limit,
		Offset: offset,
	})
}

// GetByID handles GET /api/v1/treasuries/{id}: returns a single treasury by UUID.
func (h *TreasuryHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	treasury, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid treasury ID format")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "treasury not found")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		log.Printf("[ERROR] Failed to get treasury by ID: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, treasury)
}

// Create handles POST /api/v1/treasuries: creates a new corporate treasury with strict body validation.
func (h *TreasuryHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Restrict payload size to 1MB to prevent memory exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.CreateTreasuryInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Reject unexpected fields

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	created, err := h.service.Create(r.Context(), input)
	if err != nil {
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			WriteValidationError(w, valErr.Details)
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		log.Printf("[ERROR] Failed to create corporate treasury: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusCreated, created)
}
