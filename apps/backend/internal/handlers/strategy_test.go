package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type MockStratRepo struct {
	createLogFn            func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error)
	getLogByIDFn           func(ctx context.Context, id string) (*domain.StrategyExecutionLog, error)
	listLogsByTreasuryIDFn func(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error)
}

func (m *MockStratRepo) CreateLog(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
	if m.createLogFn != nil {
		return m.createLogFn(ctx, log)
	}
	return nil, nil
}

func (m *MockStratRepo) GetLogByID(ctx context.Context, id string) (*domain.StrategyExecutionLog, error) {
	if m.getLogByIDFn != nil {
		return m.getLogByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStratRepo) ListLogsByTreasuryID(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error) {
	if m.listLogsByTreasuryIDFn != nil {
		return m.listLogsByTreasuryIDFn(ctx, treasuryID, limit, offset)
	}
	return nil, nil
}

type mockAIClientImpl struct {
	generateFn func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error)
	compareFn  func(ctx context.Context, problem any) (json.RawMessage, error)
	riskFn     func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error)
	stressFn   func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error)
}

func (m *mockAIClientImpl) Health(ctx context.Context) error {
	return nil
}

func (m *mockAIClientImpl) GenerateStrategy(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, problem, algorithm)
	}
	return nil, nil
}

func (m *mockAIClientImpl) CompareSearches(ctx context.Context, problem any) (json.RawMessage, error) {
	if m.compareFn != nil {
		return m.compareFn(ctx, problem)
	}
	return json.RawMessage(`{}`), nil
}

func (m *mockAIClientImpl) AnalyzeRisk(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
	if m.riskFn != nil {
		return m.riskFn(ctx, req)
	}
	return nil, nil
}

func (m *mockAIClientImpl) AdversarialStressTest(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
	if m.stressFn != nil {
		return m.stressFn(ctx, problem)
	}
	return nil, nil
}

func setupStrategyRouter(h *StrategyHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Route("/treasuries/{id}/strategies", func(st chi.Router) {
			st.Get("/", h.ListByTreasury)
			st.Post("/generate", h.Generate)
		})
		v1.Get("/strategies/{id}", h.GetByID)
	})
	return r
}

func TestStrategyHandler_Generate(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleBody := `{
		"algorithm": "astar",
		"assets": [
			{
				"id": "gov_bond",
				"name": "Government Bonds",
				"asset_type": "gov_bond",
				"expected_return": 0.07,
				"risk": 0.05,
				"liquidity": 0.95
			}
		],
		"liabilities": [],
		"constraints": []
	}`

	t.Run("Success 201 Created", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		sRepo := &MockStratRepo{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "log-123"
				log.CreatedAt = time.Now()
				return &log, nil
			},
		}
		aiClient := &mockAIClientImpl{
			generateFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{
						Explanation: "Allocated via A*",
					},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{
							Allocations: map[string]float64{"gov_bond": 100.0},
						},
					},
				}, nil
			},
		}

		svc := service.NewStrategyService(tRepo, sRepo, aiClient)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/strategies/generate", bytes.NewBufferString(sampleBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Invalid treasury ID returns 400", func(t *testing.T) {
		svc := service.NewStrategyService(nil, nil, nil)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/invalid-uuid/strategies/generate", bytes.NewBufferString(sampleBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Step size too small returns 400 with details", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		svc := service.NewStrategyService(tRepo, nil, nil)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		bodyWithTinyStep := `{
			"algorithm": "astar",
			"step_size": 0.1,
			"assets": [
				{
					"id": "gov_bond",
					"name": "Government Bonds",
					"asset_type": "gov_bond",
					"expected_return": 0.07,
					"risk": 0.05,
					"liquidity": 0.95
				}
			],
			"liabilities": [],
			"constraints": []
		}`

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/strategies/generate", bytes.NewBufferString(bodyWithTinyStep))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}

		var valResp ValidationErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&valResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(valResp.Details) == 0 || valResp.Details[0] != "step_size too small for capital, max 200 steps" {
			t.Fatalf("unexpected validation details: %+v", valResp)
		}
	})

	t.Run("Step size <= 0 returns 400 with details", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		svc := service.NewStrategyService(tRepo, nil, nil)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		bodyWithZeroStep := `{
			"algorithm": "astar",
			"step_size": 0,
			"assets": [
				{
					"id": "gov_bond",
					"name": "Government Bonds",
					"asset_type": "gov_bond",
					"expected_return": 0.07,
					"risk": 0.05,
					"liquidity": 0.95
				}
			],
			"liabilities": [],
			"constraints": []
		}`

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/strategies/generate", bytes.NewBufferString(bodyWithZeroStep))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}

		var valResp ValidationErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&valResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(valResp.Details) == 0 || valResp.Details[0] != "step_size must be greater than 0" {
			t.Fatalf("unexpected validation details: %+v", valResp)
		}
	})

	t.Run("AI Engine down returns 502 Bad Gateway", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		aiClient := &mockAIClientImpl{
			generateFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}
		svc := service.NewStrategyService(tRepo, nil, aiClient)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/strategies/generate", bytes.NewBufferString(sampleBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d", rec.Code)
		}
	})
}

func TestStrategyHandler_ListByTreasury(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	t.Run("Success", func(t *testing.T) {
		sRepo := &MockStratRepo{
			listLogsByTreasuryIDFn: func(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error) {
				return []domain.StrategyExecutionLog{
					{ID: "log-1", TreasuryID: treasuryID},
				}, nil
			},
		}
		svc := service.NewStrategyService(nil, sRepo, nil)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/"+validTreasuryID+"/strategies?limit=10&offset=0", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp ListStrategiesResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].ID != "log-1" {
			t.Errorf("unexpected data: %+v", resp)
		}
	})
}

func TestStrategyHandler_GetByID(t *testing.T) {
	validID := "123e4567-e89b-12d3-a456-426614174000"

	t.Run("Success", func(t *testing.T) {
		sRepo := &MockStratRepo{
			getLogByIDFn: func(ctx context.Context, id string) (*domain.StrategyExecutionLog, error) {
				return &domain.StrategyExecutionLog{ID: id}, nil
			},
		}
		svc := service.NewStrategyService(nil, sRepo, nil)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/strategies/"+validID, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("Not Found", func(t *testing.T) {
		sRepo := &MockStratRepo{
			getLogByIDFn: func(ctx context.Context, id string) (*domain.StrategyExecutionLog, error) {
				return nil, domain.ErrNotFound
			},
		}
		svc := service.NewStrategyService(nil, sRepo, nil)
		h := NewStrategyHandler(svc)
		r := setupStrategyRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/strategies/"+validID, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}
