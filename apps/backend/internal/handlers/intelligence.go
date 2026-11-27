package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type IntelligenceHandler struct {
	optSvc *service.OptimizationService
}

func NewIntelligenceHandler(optSvc *service.OptimizationService) *IntelligenceHandler {
	return &IntelligenceHandler{optSvc: optSvc}
}

type forecastHTTPRequest struct {
	History      []*string `json:"history"`
	Horizon      int       `json:"horizon"`
	SeasonLength int       `json:"season_length"`
	Model        string    `json:"model"`
}

type riskHTTPRequest struct {
	PortfolioValue  string                             `json:"portfolio_value"`
	ReturnsHistory  []string                           `json:"returns_history"`
	ConfidenceLevel string                             `json:"confidence_level"`
	NumSimulations  int                                `json:"num_simulations"`
	Seed            int                                `json:"seed"`
	StressScenarios []intelligence.StressScenarioInput `json:"stress_scenarios"`
}

type optimizeCandidateHTTP struct {
	ID             string  `json:"id"`
	ExpectedReturn string  `json:"expected_return"`
	MinAllocation  string  `json:"min_allocation"`
	MaxAllocation  string  `json:"max_allocation"`
	CounterpartyID *string `json:"counterparty_id,omitempty"`
	LotSize        *string `json:"lot_size,omitempty"`
}

type optimizeHTTPRequest struct {
	InvestableCapital string                  `json:"investable_capital"`
	SolverMode        string                  `json:"solver_mode"`
	AllowUnallocated  *bool                   `json:"allow_unallocated_cash"`
	Candidates        []optimizeCandidateHTTP `json:"candidates"`
}

func (h *IntelligenceHandler) Forecast(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	var req forecastHTTPRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	if req.SeasonLength == 0 {
		req.SeasonLength = 7
	}
	if req.Model == "" {
		req.Model = "ets"
	}

	resp, err := h.optSvc.Forecast(r.Context(), orgCtx.OrganizationID, entityID, userCtx.UserID, string(orgCtx.Role), intelligence.ForecastRequest{
		CorrelationID: middleware.CorrelationIDFromContext(r.Context()),
		History:       req.History,
		Horizon:       req.Horizon,
		SeasonLength:  req.SeasonLength,
		Model:         req.Model,
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}

func (h *IntelligenceHandler) EvaluateRisk(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	var req riskHTTPRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	if req.ConfidenceLevel == "" {
		req.ConfidenceLevel = "0.95"
	}
	if req.NumSimulations == 0 {
		req.NumSimulations = 2000
	}

	resp, err := h.optSvc.EvaluateRisk(r.Context(), orgCtx.OrganizationID, entityID, userCtx.UserID, string(orgCtx.Role), intelligence.RiskRequest{
		CorrelationID:   middleware.CorrelationIDFromContext(r.Context()),
		PortfolioValue:  req.PortfolioValue,
		ReturnsHistory:  req.ReturnsHistory,
		ConfidenceLevel: req.ConfidenceLevel,
		NumSimulations:  req.NumSimulations,
		Seed:            req.Seed,
		StressScenarios: req.StressScenarios,
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}

func (h *IntelligenceHandler) Optimize(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	var req optimizeHTTPRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	invCap, err := money.FromString(req.InvestableCapital)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid investable_capital")
		return
	}
	solverMode := req.SolverMode
	if solverMode == "" {
		solverMode = "lp"
	}
	allowUnalloc := true
	if req.AllowUnallocated != nil {
		allowUnalloc = *req.AllowUnallocated
	}

	cands := make([]service.OptimizeCandidateRequest, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		minA := money.Zero()
		if c.MinAllocation != "" {
			minA, err = money.FromString(c.MinAllocation)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid candidate min_allocation")
				return
			}
		}
		maxA, err := money.FromString(c.MaxAllocation)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid candidate max_allocation")
			return
		}
		var cpID *uuid.UUID
		if c.CounterpartyID != nil && *c.CounterpartyID != "" {
			parsed, err := uuid.Parse(*c.CounterpartyID)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid candidate counterparty_id")
				return
			}
			cpID = &parsed
		}
		var lot *money.Amount
		if c.LotSize != nil && *c.LotSize != "" {
			l, err := money.FromString(*c.LotSize)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid candidate lot_size")
				return
			}
			lot = &l
		}
		cands = append(cands, service.OptimizeCandidateRequest{
			ID:             c.ID,
			ExpectedReturn: c.ExpectedReturn,
			MinAllocation:  minA,
			MaxAllocation:  maxA,
			CounterpartyID: cpID,
			LotSize:        lot,
		})
	}

	outcome, err := h.optSvc.RunOptimization(r.Context(), service.RunOptimizationInput{
		OrganizationID:    orgCtx.OrganizationID,
		TreasuryEntityID:  entityID,
		ActorID:           userCtx.UserID,
		ActorRole:         string(orgCtx.Role),
		CorrelationID:     middleware.CorrelationIDFromContext(r.Context()),
		InvestableCapital: invCap,
		SolverMode:        solverMode,
		Candidates:        cands,
		AllowUnallocated:  allowUnalloc,
	})
	if err != nil {
		WriteDomainError(w, r, err, map[string]any{"optimization_run": serializeRun(outcome.Run)})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"optimization_run": serializeRun(outcome.Run),
		"verification": map[string]any{
			"passed":           outcome.Verification.Passed,
			"total_allocated":  outcome.Verification.TotalAllocated.String(),
			"unallocated_cash": outcome.Verification.UnallocatedCash.String(),
			"failures":         outcome.Verification.Failures,
		},
	})
}

func serializeRun(run domain.OptimizationRun) map[string]any {
	return map[string]any{
		"id":                 run.ID,
		"organization_id":    run.OrganizationID,
		"treasury_entity_id": run.TreasuryEntityID,
		"requested_by":       run.RequestedBy,
		"correlation_id":     run.CorrelationID,
		"solver_category":    run.SolverCategory,
		"status":             run.Status,
		"feasible":           run.Feasible,
		"runtime_ms":         run.RuntimeMs,
		"raw_response":       run.RawResponse,
		"created_at":         run.CreatedAt.UTC().Format(time.RFC3339),
	}
}
