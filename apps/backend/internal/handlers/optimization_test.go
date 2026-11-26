package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type mockOptClientForHandler struct {
	classicalFn func(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error)
	quantumFn   func(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error)
	compareFn   func(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error)
}

func (m *mockOptClientForHandler) Health(ctx context.Context) error { return nil }

func (m *mockOptClientForHandler) OptimizeClassical(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error) {
	if m.classicalFn != nil {
		return m.classicalFn(ctx, req)
	}
	return nil, nil
}

func (m *mockOptClientForHandler) OptimizeQuantum(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error) {
	if m.quantumFn != nil {
		return m.quantumFn(ctx, req)
	}
	return nil, nil
}

func (m *mockOptClientForHandler) CompareOptimizers(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error) {
	if m.compareFn != nil {
		return m.compareFn(ctx, req)
	}
	return nil, nil
}

type mockOptAuditRepo struct {
	recorded []domain.AuditLog
}

func (m *mockOptAuditRepo) Create(ctx context.Context, logEntry domain.AuditLog) (*domain.AuditLog, error) {
	m.recorded = append(m.recorded, logEntry)
	return &logEntry, nil
}

func (m *mockOptAuditRepo) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	return m.recorded, nil
}

func setupOptimizationRouter(h *OptimizationHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/optimization", func(opt chi.Router) {
		opt.Post("/classical", h.SolveClassical)
		opt.Post("/quantum", h.SolveQuantum)
		opt.Post("/compare", h.Compare)
	})
	return r
}

func TestOptimizationHandler(t *testing.T) {
	validProblem := `{
		"problem": {
			"capital": 100.0,
			"assets": [
				{"name": "Gov Bond", "expected_return": 0.07, "risk": 0.05, "liquidity": 0.95, "cost": 20.0}
			],
			"liabilities": [],
			"constraints": [],
			"risk_tolerance": 0.5,
			"minimum_liquidity": 0.2,
			"objective": "maximize_risk_adjusted_return"
		},
		"options": {"p": 2, "shots": 1024, "seed": 42}
	}`

	t.Run("POST /classical returns 200 OK", func(t *testing.T) {
		optClient := &mockOptClientForHandler{
			classicalFn: func(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error) {
				return &clients.OptimizationResult{
					Method:         "exact",
					Status:         "optimal",
					Allocation:     map[string]float64{"Gov Bond": 1.0},
					ObjectiveValue: 0.07,
					Feasible:       true,
					RuntimeMS:      1.2,
				}, nil
			},
		}
		h := NewOptimizationHandler(optClient, nil)
		r := setupOptimizationRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/optimization/classical", bytes.NewBufferString(validProblem))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST /quantum forwards 400 rejection cleanly", func(t *testing.T) {
		optClient := &mockOptClientForHandler{
			quantumFn: func(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error) {
				return nil, errors.New("optimization engine rejected request: exceeds qubit cap")
			},
		}
		h := NewOptimizationHandler(optClient, nil)
		r := setupOptimizationRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/optimization/quantum", bytes.NewBufferString(validProblem))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST /compare returns 200 OK and records audit log", func(t *testing.T) {
		auditRepo := &mockOptAuditRepo{}
		auditSvc := service.NewAuditService(auditRepo)
		approx := 0.96
		optClient := &mockOptClientForHandler{
			compareFn: func(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error) {
				return &clients.OptimizationCompareResponse{
					ProblemSummary:         map[string]any{"capital": 100.0, "n_assets": 1},
					Results:                map[string]clients.OptimizationResult{"exact": {Method: "exact", Feasible: true}, "qaoa": {Method: "qaoa", Feasible: true}},
					SummaryTable:           []map[string]any{{"method": "exact"}},
					BestClassicalMethod:    "exact",
					QAOAApproximationRatio: &approx,
					SimulationNote:         "Statevector CPU simulation",
				}, nil
			},
		}
		h := NewOptimizationHandler(optClient, auditSvc)
		r := setupOptimizationRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/optimization/compare", bytes.NewBufferString(validProblem))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var decoded clients.OptimizationCompareResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("failed to decode compare response: %v", err)
		}
		if decoded.BestClassicalMethod != "exact" {
			t.Errorf("expected best_classical_method 'exact', got %s", decoded.BestClassicalMethod)
		}
		if len(auditRepo.recorded) != 1 || auditRepo.recorded[0].Action != domain.ActionOptimizationCompared {
			t.Errorf("expected 1 audit record with action %s, got %+v", domain.ActionOptimizationCompared, auditRepo.recorded)
		}
	})

	t.Run("POST /compare returns 502 Bad Gateway when optimization engine unavailable", func(t *testing.T) {
		optClient := &mockOptClientForHandler{
			compareFn: func(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}
		h := NewOptimizationHandler(optClient, nil)
		r := setupOptimizationRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/optimization/compare", bytes.NewBufferString(validProblem))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d", rec.Code)
		}
	})

	t.Run("POST /compare rejects invalid empty problem payload with 400", func(t *testing.T) {
		h := NewOptimizationHandler(&mockOptClientForHandler{}, nil)
		r := setupOptimizationRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/optimization/compare", bytes.NewBufferString(`{"problem":{"capital":0,"assets":[]}}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}
