// Package handlers implements HTTP controllers and converts domain logic to REST API responses.
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

// OptimizationHandler exposes gateway endpoints proxying to Member 2's Optimization Engine.
type OptimizationHandler struct {
	optClient clients.OptimizationClient
	auditSvc  *service.AuditService
}

// NewOptimizationHandler constructs an OptimizationHandler instance.
func NewOptimizationHandler(optClient clients.OptimizationClient, auditSvc *service.AuditService) *OptimizationHandler {
	return &OptimizationHandler{
		optClient: optClient,
		auditSvc:  auditSvc,
	}
}

// SolveClassical handles POST /api/v1/optimization/classical.
func (h *OptimizationHandler) SolveClassical(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeOptimizationRequest(w, r)
	if !ok {
		return
	}
	if h.optClient == nil {
		WriteError(w, http.StatusBadGateway, "optimization engine unavailable")
		return
	}

	res, err := h.optClient.OptimizeClassical(r.Context(), req)
	if err != nil {
		writeOptimizationError(w, "classical", err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(r.Context(), domain.ActionOptimizationClassical, "optimization", nil, map[string]any{
			"method":          res.Method,
			"status":          res.Status,
			"feasible":        res.Feasible,
			"objective_value": res.ObjectiveValue,
			"runtime_ms":      res.RuntimeMS,
		})
	}

	WriteJSON(w, http.StatusOK, res)
}

// SolveQuantum handles POST /api/v1/optimization/quantum.
func (h *OptimizationHandler) SolveQuantum(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeOptimizationRequest(w, r)
	if !ok {
		return
	}
	if h.optClient == nil {
		WriteError(w, http.StatusBadGateway, "optimization engine unavailable")
		return
	}

	res, err := h.optClient.OptimizeQuantum(r.Context(), req)
	if err != nil {
		writeOptimizationError(w, "quantum", err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(r.Context(), domain.ActionOptimizationQuantum, "optimization", nil, map[string]any{
			"method":          res.Method,
			"status":          res.Status,
			"feasible":        res.Feasible,
			"objective_value": res.ObjectiveValue,
			"runtime_ms":      res.RuntimeMS,
			"metadata":        res.Metadata,
		})
	}

	WriteJSON(w, http.StatusOK, res)
}

// Compare handles POST /api/v1/optimization/compare.
func (h *OptimizationHandler) Compare(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var req clients.OptimizationCompareRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	if errs := validateOptimizationProblemPayload(req.Problem); len(errs) > 0 {
		WriteValidationError(w, errs)
		return
	}
	if h.optClient == nil {
		WriteError(w, http.StatusBadGateway, "optimization engine unavailable")
		return
	}

	resp, err := h.optClient.CompareOptimizers(r.Context(), req)
	if err != nil {
		writeOptimizationError(w, "compare", err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(r.Context(), domain.ActionOptimizationCompared, "optimization", nil, map[string]any{
			"best_classical_method":    resp.BestClassicalMethod,
			"qaoa_approximation_ratio": resp.QAOAApproximationRatio,
			"solvers_count":            len(resp.Results),
			"problem_summary":          resp.ProblemSummary,
			"options":                  req.Options,
		})
	}

	WriteJSON(w, http.StatusOK, resp)
}

func decodeOptimizationRequest(w http.ResponseWriter, r *http.Request) (clients.OptimizationRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var req clients.OptimizationRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return req, false
	}

	if errs := validateOptimizationProblemPayload(req.Problem); len(errs) > 0 {
		WriteValidationError(w, errs)
		return req, false
	}

	return req, true
}

func validateOptimizationProblemPayload(problem any) []string {
	probMap, ok := problem.(map[string]any)
	if !ok || probMap == nil {
		return []string{"problem object is required"}
	}

	var errs []string
	capitalRaw, hasCapital := probMap["capital"]
	if !hasCapital {
		errs = append(errs, "problem.capital is required")
	} else if capVal, ok := capitalRaw.(float64); !ok || capVal <= 0 {
		errs = append(errs, "problem.capital must be greater than 0")
	}

	assetsRaw, hasAssets := probMap["assets"]
	if !hasAssets {
		errs = append(errs, "problem.assets is required")
	} else if assetsSlice, ok := assetsRaw.([]any); !ok || len(assetsSlice) == 0 {
		errs = append(errs, "problem.assets must contain at least 1 asset")
	}

	return errs
}

func writeOptimizationError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, domain.ErrUpstreamUnavailable) {
		WriteError(w, http.StatusBadGateway, "optimization engine unavailable")
		return
	}
	if strings.Contains(err.Error(), "rejected") {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[ERROR] Optimization %s request failed: %v", operation, err)
	WriteError(w, http.StatusInternalServerError, "internal server error")
}
