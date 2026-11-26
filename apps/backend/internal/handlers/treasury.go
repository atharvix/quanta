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
	Data []domain.Treasury `json:"data"`

	Limit int `json:"limit"`

	Offset int `json:"offset"`
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

		Data: treasuries,

		Limit: limit,

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

		if errors.Is(err, domain.ErrForbidden) {

			WriteError(w, http.StatusForbidden, "forbidden: insufficient permissions for this treasury")

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

// Update handles PATCH /api/v1/treasuries/{id}: updates existing corporate treasury metadata.
func (h *TreasuryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input domain.UpdateTreasuryInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	updated, err := h.service.Update(r.Context(), id, input)
	if err != nil {
		h.handleError(w, err, "failed to update corporate treasury")
		return
	}

	WriteJSON(w, http.StatusOK, updated)
}

// ListAssetsResponse formats the assets JSON response.
type ListAssetsResponse struct {
	Data []domain.Asset `json:"data"`
}

// ListLiabilitiesResponse formats the liabilities JSON response.
type ListLiabilitiesResponse struct {
	Data []domain.Liability `json:"data"`
}

// ListAssets handles GET /api/v1/treasuries/{id}/assets.
func (h *TreasuryHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")

	assets, err := h.service.ListAssets(r.Context(), treasuryID)
	if err != nil {
		h.handleError(w, err, "failed to list treasury assets")
		return
	}

	WriteJSON(w, http.StatusOK, ListAssetsResponse{Data: assets})
}

// GetAsset handles GET /api/v1/treasuries/{id}/assets/{assetId}.
func (h *TreasuryHandler) GetAsset(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	assetID := chi.URLParam(r, "assetId")

	asset, err := h.service.GetAsset(r.Context(), treasuryID, assetID)
	if err != nil {
		h.handleError(w, err, "failed to get asset")
		return
	}

	WriteJSON(w, http.StatusOK, asset)
}

// CreateAsset handles POST /api/v1/treasuries/{id}/assets.
func (h *TreasuryHandler) CreateAsset(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var asset domain.Asset
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&asset); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	created, err := h.service.CreateAsset(r.Context(), treasuryID, asset)
	if err != nil {
		h.handleError(w, err, "failed to create asset")
		return
	}

	WriteJSON(w, http.StatusCreated, created)
}

// UpdateAsset handles PUT /api/v1/treasuries/{id}/assets/{assetId}.
func (h *TreasuryHandler) UpdateAsset(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	assetID := chi.URLParam(r, "assetId")
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var asset domain.Asset
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&asset); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	updated, err := h.service.UpdateAsset(r.Context(), treasuryID, assetID, asset)
	if err != nil {
		h.handleError(w, err, "failed to update asset")
		return
	}

	WriteJSON(w, http.StatusOK, updated)
}

// DeleteAsset handles DELETE /api/v1/treasuries/{id}/assets/{assetId}.
func (h *TreasuryHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	assetID := chi.URLParam(r, "assetId")

	if err := h.service.DeleteAsset(r.Context(), treasuryID, assetID); err != nil {
		h.handleError(w, err, "failed to delete asset")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "asset deleted successfully"})
}

// BulkSyncAssets handles PUT /api/v1/treasuries/{id}/assets.
func (h *TreasuryHandler) BulkSyncAssets(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var assets []domain.Asset
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&assets); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	synced, err := h.service.BulkSyncAssets(r.Context(), treasuryID, assets)
	if err != nil {
		h.handleError(w, err, "failed to bulk sync assets")
		return
	}

	WriteJSON(w, http.StatusOK, ListAssetsResponse{Data: synced})
}

// ListLiabilities handles GET /api/v1/treasuries/{id}/liabilities.
func (h *TreasuryHandler) ListLiabilities(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")

	liabilities, err := h.service.ListLiabilities(r.Context(), treasuryID)
	if err != nil {
		h.handleError(w, err, "failed to list treasury liabilities")
		return
	}

	WriteJSON(w, http.StatusOK, ListLiabilitiesResponse{Data: liabilities})
}

// GetLiability handles GET /api/v1/treasuries/{id}/liabilities/{liabilityId}.
func (h *TreasuryHandler) GetLiability(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	liabilityID := chi.URLParam(r, "liabilityId")

	liability, err := h.service.GetLiability(r.Context(), treasuryID, liabilityID)
	if err != nil {
		h.handleError(w, err, "failed to get liability")
		return
	}

	WriteJSON(w, http.StatusOK, liability)
}

// CreateLiability handles POST /api/v1/treasuries/{id}/liabilities.
func (h *TreasuryHandler) CreateLiability(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var liability domain.Liability
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&liability); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	created, err := h.service.CreateLiability(r.Context(), treasuryID, liability)
	if err != nil {
		h.handleError(w, err, "failed to create liability")
		return
	}

	WriteJSON(w, http.StatusCreated, created)
}

// UpdateLiability handles PUT /api/v1/treasuries/{id}/liabilities/{liabilityId}.
func (h *TreasuryHandler) UpdateLiability(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	liabilityID := chi.URLParam(r, "liabilityId")
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var liability domain.Liability
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&liability); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	updated, err := h.service.UpdateLiability(r.Context(), treasuryID, liabilityID, liability)
	if err != nil {
		h.handleError(w, err, "failed to update liability")
		return
	}

	WriteJSON(w, http.StatusOK, updated)
}

// DeleteLiability handles DELETE /api/v1/treasuries/{id}/liabilities/{liabilityId}.
func (h *TreasuryHandler) DeleteLiability(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	liabilityID := chi.URLParam(r, "liabilityId")

	if err := h.service.DeleteLiability(r.Context(), treasuryID, liabilityID); err != nil {
		h.handleError(w, err, "failed to delete liability")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "liability deleted successfully"})
}

// BulkSyncLiabilities handles PUT /api/v1/treasuries/{id}/liabilities.
func (h *TreasuryHandler) BulkSyncLiabilities(w http.ResponseWriter, r *http.Request) {
	treasuryID := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var liabilities []domain.Liability
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&liabilities); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	synced, err := h.service.BulkSyncLiabilities(r.Context(), treasuryID, liabilities)
	if err != nil {
		h.handleError(w, err, "failed to bulk sync liabilities")
		return
	}

	WriteJSON(w, http.StatusOK, ListLiabilitiesResponse{Data: synced})
}

// handleError maps domain errors to standard HTTP response formats.
func (h *TreasuryHandler) handleError(w http.ResponseWriter, err error, actionMsg string) {
	if errors.Is(err, domain.ErrInvalidID) {
		WriteError(w, http.StatusBadRequest, "invalid treasury ID format")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "resource not found")
		return
	}
	if errors.Is(err, domain.ErrForbidden) {
		WriteError(w, http.StatusForbidden, "forbidden: insufficient permissions for this treasury")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		WriteError(w, http.StatusConflict, "resource with this identifier already exists in treasury")
		return
	}
	var valErr *domain.ValidationError
	if errors.As(err, &valErr) {
		WriteValidationError(w, valErr.Details)
		return
	}
	if errors.Is(err, domain.ErrDatabaseUnavailable) {
		WriteError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	log.Printf("[ERROR] %s: %v", actionMsg, err)
	WriteError(w, http.StatusInternalServerError, "internal server error")
}
