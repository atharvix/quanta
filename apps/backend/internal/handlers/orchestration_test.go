package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type mockOrchRepoForHandler struct {
	createFn  func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error)
	getByIDFn func(ctx context.Context, id string) (*domain.OrchestrationRun, error)
}

func (m *mockOrchRepoForHandler) Create(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
	if m.createFn != nil {
		return m.createFn(ctx, run)
	}
	return nil, nil
}

func (m *mockOrchRepoForHandler) GetByID(ctx context.Context, id string) (*domain.OrchestrationRun, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func setupOrchRouter(h *OrchestrationHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/orchestrate", func(o chi.Router) {
		o.Post("/allocation", h.Allocate)
		o.Get("/runs/{id}", h.GetRunByID)
	})
	return r
}

func TestOrchestrationHandler_Allocate(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	payload := `{
		"treasury_id": "` + validTreasuryID + `",
		"algorithm": "astar",
		"assets": [
			{
				"id": "gov_bond",
				"name": "Bonds",
				"asset_type": "gov_bond",
				"expected_return": 0.05,
				"risk": 0.02,
				"liquidity": 0.95
			}
		],
		"liabilities": [],
		"constraints": [],
		"include_comparison": false
	}`

	t.Run("Success 201 Created", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0, RiskTolerance: 0.5, MinLiquidityRatio: 0.2}, nil
			},
		}
		orchRepo := &mockOrchRepoForHandler{
			createFn: func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
				run.ID = "run-123"
				return &run, nil
			},
		}
		stratRepo := &MockStratRepo{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "strat-456"
				return &log, nil
			},
		}
		aiClient := &mockAIClientImpl{
			generateFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{Explanation: "Allocated"},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{Allocations: map[string]float64{}},
					},
				}, nil
			},
		}

		svc := service.NewOrchestrationService(tRepo, orchRepo, stratRepo, aiClient, nil)
		h := NewOrchestrationHandler(svc)
		r := setupOrchRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/allocation", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("AI down returns 502 Bad Gateway", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0, RiskTolerance: 0.5, MinLiquidityRatio: 0.2}, nil
			},
		}
		orchRepo := &mockOrchRepoForHandler{
			createFn: func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
				return &run, nil
			},
		}
		aiClient := &mockAIClientImpl{
			generateFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}

		svc := service.NewOrchestrationService(tRepo, orchRepo, nil, aiClient, nil)
		h := NewOrchestrationHandler(svc)
		r := setupOrchRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/allocation", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "AI engine unavailable") {
			t.Errorf("expected 'AI engine unavailable' in body, got: %s", rec.Body.String())
		}
	})
}
