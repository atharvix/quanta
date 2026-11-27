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

type PolicyHandler struct {
	policySvc *service.PolicyService
}

func NewPolicyHandler(policySvc *service.PolicyService) *PolicyHandler {
	return &PolicyHandler{policySvc: policySvc}
}

type policyUpsertRequest struct {
	MinOperatingLiquidity string `json:"min_operating_liquidity"`
	MinEmergencyReserve   string `json:"min_emergency_reserve"`
	IsActive              bool   `json:"is_active"`
}

type validatePolicyRequest struct {
	TotalAvailable     string `json:"total_available"`
	ProposedAllocation string `json:"proposed_allocation"`
}

func (h *PolicyHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	policies, err := h.policySvc.ListPolicies(r.Context(), orgCtx.OrganizationID, entityID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(policies))
	for _, p := range policies {
		out = append(out, serializePolicy(p))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"policies": out})
}

func (h *PolicyHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	var req policyUpsertRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	opFloor, err := money.FromString(req.MinOperatingLiquidity)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid min_operating_liquidity")
		return
	}
	emFloor, err := money.FromString(req.MinEmergencyReserve)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid min_emergency_reserve")
		return
	}

	policy, err := h.policySvc.CreatePolicy(r.Context(), service.CreateLiquidityPolicyInput{
		OrganizationID:        orgCtx.OrganizationID,
		TreasuryEntityID:      entityID,
		MinOperatingLiquidity: opFloor,
		MinEmergencyReserve:   emFloor,
		IsActive:              req.IsActive,
		ActorID:               userCtx.UserID,
		ActorRole:             string(orgCtx.Role),
		CorrelationID:         middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"policy": serializePolicy(policy)})
}

func (h *PolicyHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	policyID, err := uuid.Parse(chi.URLParam(r, "policyId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid policy ID")
		return
	}
	var req policyUpsertRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	opFloor, err := money.FromString(req.MinOperatingLiquidity)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid min_operating_liquidity")
		return
	}
	emFloor, err := money.FromString(req.MinEmergencyReserve)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid min_emergency_reserve")
		return
	}

	policy, err := h.policySvc.UpdatePolicy(r.Context(), service.UpdateLiquidityPolicyInput{
		OrganizationID:        orgCtx.OrganizationID,
		PolicyID:              policyID,
		MinOperatingLiquidity: opFloor,
		MinEmergencyReserve:   emFloor,
		IsActive:              req.IsActive,
		ActorID:               userCtx.UserID,
		ActorRole:             string(orgCtx.Role),
		CorrelationID:         middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"policy": serializePolicy(policy)})
}

func (h *PolicyHandler) ValidateAllocation(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	var req validatePolicyRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	totalAvail, err := money.FromString(req.TotalAvailable)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid total_available")
		return
	}
	proposedAlloc, err := money.FromString(req.ProposedAllocation)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid proposed_allocation")
		return
	}

	outcome, err := h.policySvc.ValidateAllocation(r.Context(), orgCtx.OrganizationID, entityID, totalAvail, proposedAlloc)
	payload := map[string]any{
		"valid":                outcome.Valid,
		"policy_id":            outcome.PolicyID,
		"policy_version":       outcome.PolicyVersion,
		"total_available":      outcome.TotalAvailable.String(),
		"proposed_allocation":  outcome.ProposedAllocation.String(),
		"required_floor":       outcome.RequiredFloor.String(),
		"post_allocation_cash": outcome.PostAllocationCash.String(),
		"investable_capacity":  outcome.InvestableCapacity.String(),
		"reason":               outcome.Reason,
	}
	if err != nil {
		WriteDomainError(w, r, err, payload)
		return
	}
	WriteJSON(w, http.StatusOK, payload)
}

func serializePolicy(p domain.LiquidityPolicy) map[string]any {
	return map[string]any{
		"id":                      p.ID,
		"organization_id":         p.OrganizationID,
		"treasury_entity_id":      p.TreasuryEntityID,
		"version":                 p.Version,
		"min_operating_liquidity": p.MinOperatingLiquidity.String(),
		"min_emergency_reserve":   p.MinEmergencyReserve.String(),
		"total_reserve_floor":     p.TotalReserveFloor().String(),
		"is_active":               p.IsActive,
		"created_by":              p.CreatedBy,
		"created_at":              p.CreatedAt.UTC().Format(time.RFC3339),
	}
}
