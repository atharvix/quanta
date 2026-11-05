package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type mockFPRepoForHandler struct {
	createFn  func(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error)
	getByIDFn func(ctx context.Context, id string) (*domain.FinancialProblemEntity, error)
	listFn    func(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error)
}

func (m *mockFPRepoForHandler) Create(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error) {
	if m.createFn != nil {
		return m.createFn(ctx, fp)
	}
	return nil, nil
}

func (m *mockFPRepoForHandler) GetByID(ctx context.Context, id string) (*domain.FinancialProblemEntity, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockFPRepoForHandler) List(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset, treasuryID)
	}
	return nil, nil
}

func setupFPRouter(h *FinancialProblemHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/financial-problems", func(f chi.Router) {
		f.Post("/", h.Create)
		f.Get("/", h.List)
		f.Get("/{id}", h.GetByID)
	})
	return r
}

func TestFinancialProblemHandler_Create(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	payload := `{
		"treasury_id": "` + validTreasuryID + `",
		"algorithm": "astar",
		"problem": {
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
			"constraints": []
		}
	}`

	t.Run("Success 201 Created", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0, RiskTolerance: 0.5, MinLiquidityRatio: 0.2}, nil
			},
		}
		fpRepo := &mockFPRepoForHandler{
			createFn: func(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error) {
				fp.ID = "fp-123"
				return &fp, nil
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

		svc := service.NewFinancialProblemService(tRepo, fpRepo, stratRepo, aiClient, nil)
		h := NewFinancialProblemHandler(svc)
		r := setupFPRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/financial-problems", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Treasury Not Found returns 404", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}
		svc := service.NewFinancialProblemService(tRepo, nil, nil, nil, nil)
		h := NewFinancialProblemHandler(svc)
		r := setupFPRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/financial-problems", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}
