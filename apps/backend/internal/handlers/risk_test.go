package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

func setupRiskRouter(h *RiskHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if domain.RoleFromContext(r.Context()) == "" {
				ctx := domain.WithUserContext(r.Context(), "default-admin", "admin@quanta.local", domain.RoleAdmin)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Route("/api/v1/treasuries/{id}", func(tr chi.Router) {
		tr.Post("/risk/analyze", h.AnalyzeRisk)
		tr.Post("/stress-test", h.AdversarialStressTest)
	})
	return r
}

func TestRiskHandler_AnalyzeRisk(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Acme Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.35,
		MinLiquidityRatio: 0.20,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	validBody := `{
		"assets": [
			{
				"id": "gov_bond",
				"name": "Gov Bond",
				"asset_type": "gov_bond",
				"cost": 0.001,
				"expected_return": 0.07,
				"risk": 0.05,
				"liquidity": 0.95
			}
		],
		"run_monte_carlo": true,
		"num_scenarios": 1000
	}`

	t.Run("200 OK with valid payload", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			riskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{
						CompositeRiskScore: 0.065,
						RiskTier:           "Moderate",
					},
					BayesianInference: domain.BayesianRiskInferenceResult{
						PRecession:       0.40,
						StressMultiplier: 1.30,
						Explanation:      "Bayesian inference complete",
					},
					MonteCarloReport: &domain.MonteCarloSimulationReport{
						NumScenarios: 1000,
						VaR95:        0.038,
					},
				}, nil
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var resp domain.AIRiskAnalyzeResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.RiskBreakdown.RiskTier != "Moderate" {
			t.Errorf("expected Moderate, got %s", resp.RiskBreakdown.RiskTier)
		}
	})

	t.Run("400 Bad Request on invalid JSON", func(t *testing.T) {
		h := NewRiskHandler(nil)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(`{bad-json`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on unknown JSON fields", func(t *testing.T) {
		h := NewRiskHandler(nil)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(`{"unknown_field": 123}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on invalid UUID format", func(t *testing.T) {
		h := NewRiskHandler(service.NewRiskService(nil, nil))
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/not-a-uuid/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on validation error (empty assets)", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		svc := service.NewRiskService(tRepo, nil)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(`{"assets":[]}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for empty assets, got %d", rec.Code)
		}
	})

	t.Run("404 Not Found when Treasury does not exist", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}

		svc := service.NewRiskService(tRepo, nil)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("502 Bad Gateway when AI Engine is unavailable", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			riskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d", rec.Code)
		}
	})

	t.Run("503 Service Unavailable when Database is down", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrDatabaseUnavailable
			},
		}

		svc := service.NewRiskService(tRepo, nil)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on oversized payload (> 1MB)", func(t *testing.T) {
		h := NewRiskHandler(nil)
		r := setupRiskRouter(h)

		hugeName := strings.Repeat("A", 1024*1024+50)
		oversized := fmt.Sprintf(`{"assets":[{"id":"gov_bond","name":"%s","asset_type":"gov_bond","cost":0.001,"expected_return":0.07,"risk":0.05,"liquidity":0.95}]}`, hugeName)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(oversized))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for oversized payload, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "request body too large") {
			t.Errorf("expected body to mention 'request body too large', got: %s", rec.Body.String())
		}
	})

	t.Run("400 Bad Request when AI Engine rejects input with custom error", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			riskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				return nil, errors.New("AI engine rejected risk analyze input: allocation invalid")
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for AI rejection, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "AI engine rejected risk analyze input") {
			t.Errorf("expected rejection message in body, got: %s", rec.Body.String())
		}
	})

	t.Run("500 Internal Server Error on unexpected service error", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			riskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				return nil, errors.New("unexpected crash in computation")
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 Internal Server Error, got %d", rec.Code)
		}
	})
}

func TestRiskHandler_AdversarialStressTest(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Acme Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.35,
		MinLiquidityRatio: 0.20,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	validBody := `{
		"assets": [
			{
				"id": "gov_bond",
				"name": "Gov Bond",
				"asset_type": "gov_bond",
				"cost": 0.001,
				"expected_return": 0.07,
				"risk": 0.05,
				"liquidity": 0.95
			}
		],
		"allocations": {
			"gov_bond": 80.0
		}
	}`

	t.Run("200 OK with valid payload", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		action := "Action(INVEST, gov_bond, 5.0)"
		aiClient := &mockAIClientImpl{
			stressFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
				return &domain.AdversarialStressTestResult{
					WorstCasePayoff:    0.042,
					BestTreasuryAction: &action,
					WorstMarketShock:   "Severe Equity & Credit Crash",
					NodesEvaluated:     32,
					PrunedBranches:     7,
					ExecutionTimeMS:    12.4,
					Explanation:        "Adversarial search evaluated 32 nodes",
				}, nil
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var resp domain.TreasuryStressTestResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.AdversarialResult.WorstMarketShock != "Severe Equity & Credit Crash" {
			t.Errorf("expected Severe Equity & Credit Crash, got %s", resp.AdversarialResult.WorstMarketShock)
		}
		if len(resp.Scenarios) != 4 {
			t.Errorf("expected 4 scenarios, got %d", len(resp.Scenarios))
		}
	})

	t.Run("400 Bad Request on invalid JSON", func(t *testing.T) {
		h := NewRiskHandler(nil)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(`{invalid`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on unknown JSON fields", func(t *testing.T) {
		h := NewRiskHandler(nil)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(`{"extra_key":"foo"}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on invalid UUID format", func(t *testing.T) {
		h := NewRiskHandler(service.NewRiskService(nil, nil))
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/bad-uuid/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("404 Not Found when Treasury does not exist", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}

		svc := service.NewRiskService(tRepo, nil)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("502 Bad Gateway when AI Engine fails", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			stressFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on validation error (empty assets)", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		svc := service.NewRiskService(tRepo, nil)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(`{"assets":[]}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for empty assets, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request on oversized payload (> 1MB)", func(t *testing.T) {
		h := NewRiskHandler(nil)
		r := setupRiskRouter(h)

		hugeName := strings.Repeat("A", 1024*1024+50)
		oversized := fmt.Sprintf(`{"assets":[{"id":"gov_bond","name":"%s","asset_type":"gov_bond","cost":0.001,"expected_return":0.07,"risk":0.05,"liquidity":0.95}]}`, hugeName)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(oversized))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for oversized payload, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "request body too large") {
			t.Errorf("expected body to mention 'request body too large', got: %s", rec.Body.String())
		}
	})

	t.Run("503 Service Unavailable when Database is down", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrDatabaseUnavailable
			},
		}

		svc := service.NewRiskService(tRepo, nil)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rec.Code)
		}
	})

	t.Run("400 Bad Request when AI Engine rejects input with custom error", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			stressFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
				return nil, errors.New("AI engine rejected stress test input: constraint violation")
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for AI rejection, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "AI engine rejected stress test input") {
			t.Errorf("expected rejection message in body, got: %s", rec.Body.String())
		}
	})

	t.Run("500 Internal Server Error on unexpected service error", func(t *testing.T) {
		tRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &mockAIClientImpl{
			stressFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
				return nil, errors.New("unexpected crash in minimax search")
			},
		}

		svc := service.NewRiskService(tRepo, aiClient)
		h := NewRiskHandler(svc)
		r := setupRiskRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 Internal Server Error, got %d", rec.Code)
		}
	})
}

func TestRiskHandler_ResourceAuthorization(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"
	ownerUserID := "user-analyst-alpha"
	otherUserID := "user-analyst-beta"

	restrictedTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Restricted Treasury Alpha",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		OwnerID:           &ownerUserID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	validBody := `{
		"assets": [
			{
				"id": "gov_bond",
				"name": "Gov Bond",
				"asset_type": "gov_bond",
				"cost": 0.001,
				"expected_return": 0.07,
				"risk": 0.05,
				"liquidity": 0.95
			}
		]
	}`

	tRepo := &MockRepo{
		getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
			return restrictedTreasury, nil
		},
	}

	aiClient := &mockAIClientImpl{
		riskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
			return &domain.AIRiskAnalyzeResponse{
				RiskBreakdown: domain.RiskFactorBreakdown{RiskTier: "Low"},
			}, nil
		},
		stressFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
			return &domain.AdversarialStressTestResult{WorstMarketShock: "Status Quo"}, nil
		},
	}

	svc := service.NewRiskService(tRepo, aiClient)
	h := NewRiskHandler(svc)
	r := setupRiskRouter(h)

	t.Run("403 Forbidden when unauthorized analyst accesses restricted treasury on POST /risk/analyze", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		unauthorizedCtx := domain.WithUserContext(req.Context(), otherUserID, "other@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(unauthorizedCtx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("403 Forbidden when unauthorized analyst accesses restricted treasury on POST /stress-test", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		unauthorizedCtx := domain.WithUserContext(req.Context(), otherUserID, "other@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(unauthorizedCtx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("200 OK when owner analyst accesses restricted treasury on POST /risk/analyze", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		ownerCtx := domain.WithUserContext(req.Context(), ownerUserID, "owner@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(ownerCtx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for owner, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("200 OK when admin accesses restricted treasury on POST /stress-test", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		adminCtx := domain.WithUserContext(req.Context(), "admin-1", "admin@quanta.local", domain.RoleAdmin)
		req = req.WithContext(adminCtx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for admin, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("403 Forbidden when analyst accesses unassigned/legacy treasury on POST /risk/analyze", func(t *testing.T) {
		unassignedTreasury := &domain.Treasury{
			ID:                validTreasuryID,
			Name:              "Unassigned Legacy Treasury",
			BaseCurrency:      "INR",
			TotalCapital:      100.0,
			RiskTolerance:     0.40,
			MinLiquidityRatio: 0.20,
			OwnerID:           nil,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		uRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return unassignedTreasury, nil
			},
		}
		uHandler := NewRiskHandler(service.NewRiskService(uRepo, aiClient))
		uRouter := setupRiskRouter(uHandler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		analystCtx := domain.WithUserContext(req.Context(), "any-analyst", "analyst@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(analystCtx)

		rec := httptest.NewRecorder()
		uRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for analyst on unassigned treasury, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("403 Forbidden when analyst accesses unassigned/legacy treasury on POST /stress-test", func(t *testing.T) {
		unassignedTreasury := &domain.Treasury{
			ID:                validTreasuryID,
			Name:              "Unassigned Legacy Treasury",
			BaseCurrency:      "INR",
			TotalCapital:      100.0,
			RiskTolerance:     0.40,
			MinLiquidityRatio: 0.20,
			OwnerID:           nil,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		uRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return unassignedTreasury, nil
			},
		}
		uHandler := NewRiskHandler(service.NewRiskService(uRepo, aiClient))
		uRouter := setupRiskRouter(uHandler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/stress-test", bytes.NewBufferString(validBody))
		analystCtx := domain.WithUserContext(req.Context(), "any-analyst", "analyst@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(analystCtx)

		rec := httptest.NewRecorder()
		uRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for analyst on unassigned treasury, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("200 OK when admin accesses unassigned/legacy treasury on POST /risk/analyze", func(t *testing.T) {
		unassignedTreasury := &domain.Treasury{
			ID:                validTreasuryID,
			Name:              "Unassigned Legacy Treasury",
			BaseCurrency:      "INR",
			TotalCapital:      100.0,
			RiskTolerance:     0.40,
			MinLiquidityRatio: 0.20,
			OwnerID:           nil,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		uRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return unassignedTreasury, nil
			},
		}
		uHandler := NewRiskHandler(service.NewRiskService(uRepo, aiClient))
		uRouter := setupRiskRouter(uHandler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validTreasuryID+"/risk/analyze", bytes.NewBufferString(validBody))
		adminCtx := domain.WithUserContext(req.Context(), "admin-1", "admin@quanta.local", domain.RoleAdmin)
		req = req.WithContext(adminCtx)

		rec := httptest.NewRecorder()
		uRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for admin on unassigned treasury, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
}
