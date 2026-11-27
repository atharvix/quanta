package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type CounterpartyHandler struct {
	cpSvc *service.CounterpartyService
}

func NewCounterpartyHandler(cpSvc *service.CounterpartyService) *CounterpartyHandler {
	return &CounterpartyHandler{cpSvc: cpSvc}
}

type createCounterpartyRequest struct {
	Name             string `json:"name"`
	CurrencyCode     string `json:"currency_code"`
	MaxExposureLimit string `json:"max_exposure_limit"`
	CurrentExposure  string `json:"current_exposure"`
}

type updateCounterpartyRequest struct {
	Name             string `json:"name"`
	MaxExposureLimit string `json:"max_exposure_limit"`
	CurrentExposure  string `json:"current_exposure"`
	ExpectedVersion  int64  `json:"expected_version"`
}

type validateExposureRequest struct {
	CurrencyCode      string `json:"currency_code"`
	ProposedIncrement string `json:"proposed_increment"`
}

func (h *CounterpartyHandler) ListCounterparties(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	cps, err := h.cpSvc.ListCounterparties(r.Context(), orgCtx.OrganizationID, entityID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(cps))
	for _, cp := range cps {
		out = append(out, serializeCounterparty(cp))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"counterparties": out})
}

func (h *CounterpartyHandler) CreateCounterparty(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	var req createCounterpartyRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	maxLim, err := money.FromString(req.MaxExposureLimit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid max_exposure_limit")
		return
	}
	curExp := money.Zero()
	if req.CurrentExposure != "" {
		curExp, err = money.FromString(req.CurrentExposure)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid current_exposure")
			return
		}
	}

	cp, err := h.cpSvc.CreateCounterparty(r.Context(), service.CreateCounterpartyInput{
		OrganizationID:   orgCtx.OrganizationID,
		TreasuryEntityID: entityID,
		Name:             req.Name,
		CurrencyCode:     req.CurrencyCode,
		MaxExposureLimit: maxLim,
		CurrentExposure:  curExp,
		ActorID:          userCtx.UserID,
		ActorRole:        string(orgCtx.Role),
		CorrelationID:    middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"counterparty": serializeCounterparty(cp)})
}

func (h *CounterpartyHandler) UpdateCounterparty(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	cpID, err := uuid.Parse(chi.URLParam(r, "counterpartyId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid counterparty ID")
		return
	}
	var req updateCounterpartyRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	maxLim, err := money.FromString(req.MaxExposureLimit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid max_exposure_limit")
		return
	}
	curExp, err := money.FromString(req.CurrentExposure)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid current_exposure")
		return
	}

	cp, err := h.cpSvc.UpdateCounterparty(r.Context(), service.UpdateCounterpartyInput{
		OrganizationID:   orgCtx.OrganizationID,
		CounterpartyID:   cpID,
		Name:             req.Name,
		MaxExposureLimit: maxLim,
		CurrentExposure:  curExp,
		ExpectedVersion:  req.ExpectedVersion,
		ActorID:          userCtx.UserID,
		ActorRole:        string(orgCtx.Role),
		CorrelationID:    middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"counterparty": serializeCounterparty(cp)})
}

func (h *CounterpartyHandler) ValidateExposure(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	cpID, err := uuid.Parse(chi.URLParam(r, "counterpartyId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid counterparty ID")
		return
	}
	var req validateExposureRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	increment, err := money.FromString(req.ProposedIncrement)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid proposed_increment")
		return
	}

	res, err := h.cpSvc.ValidateExposure(r.Context(), orgCtx.OrganizationID, cpID, req.CurrencyCode, increment)
	payload := map[string]any{
		"allowed":            res.Allowed,
		"counterparty_id":    res.CounterpartyID,
		"currency_code":      res.CurrencyCode,
		"current_exposure":   res.CurrentExposure.String(),
		"proposed_increment": res.ProposedIncrement.String(),
		"projected_exposure": res.ProjectedExposure.String(),
		"max_exposure_limit": res.MaxExposureLimit.String(),
	}
	if err != nil {
		WriteDomainError(w, r, err, payload)
		return
	}
	WriteJSON(w, http.StatusOK, payload)
}

func serializeCounterparty(cp domain.Counterparty) map[string]any {
	return map[string]any{
		"id":                 cp.ID,
		"organization_id":    cp.OrganizationID,
		"treasury_entity_id": cp.TreasuryEntityID,
		"name":               cp.Name,
		"currency_code":      cp.CurrencyCode,
		"max_exposure_limit": cp.MaxExposureLimit.String(),
		"current_exposure":   cp.CurrentExposure.String(),
		"remaining_headroom": cp.RemainingHeadroom().String(),
		"version":            cp.Version,
		"created_at":         cp.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":         cp.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
