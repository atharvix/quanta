package clients

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func TestHTTPAIClient_Health(t *testing.T) {
	ctx := context.Background()

	t.Run("Health 200 OK", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("expected /health path, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		if err := client.Health(ctx); err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
	})

	t.Run("Health 500 Server Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		err := client.Health(ctx)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})
}

func TestHTTPAIClient_GenerateStrategy(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 with valid response", func(t *testing.T) {
		responseJSON := `{
			"strategy": {
				"total_allocated": 100.0,
				"expected_return": 0.08,
				"risk_score": 0.12,
				"liquidity_ratio": 0.90,
				"objective_value": 0.28,
				"is_compliant": true,
				"explanation": "Allocated via A* Search"
			},
			"search_result": {
				"algorithm": "astar",
				"success": true,
				"final_state": {
					"allocations": {"gov_bond": 70.0, "corp_bond": 30.0},
					"cash": 0.0
				}
			},
			"constraint_audit": [
				{
					"constraint_id": "c_min_liq",
					"constraint_type": "minimum_liquidity",
					"satisfied": true,
					"actual": 0.90,
					"required": 0.20,
					"severity": "critical",
					"message": "Compliant"
				}
			]
		}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/strategy/generate" {
				t.Errorf("expected /api/v1/strategy/generate, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(responseJSON))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		res, err := client.GenerateStrategy(ctx, map[string]any{"capital": 100}, "astar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Strategy.Explanation != "Allocated via A* Search" {
			t.Errorf("unexpected explanation: %s", res.Strategy.Explanation)
		}
		if res.SearchResult.FinalState.Allocations["gov_bond"] != 70.0 {
			t.Errorf("expected 70.0 for gov_bond, got %f", res.SearchResult.FinalState.Allocations["gov_bond"])
		}
		if len(res.ConstraintAudit) != 1 {
			t.Fatalf("expected 1 constraint audit result, got %d", len(res.ConstraintAudit))
		}
	})

	t.Run("Upstream 500 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":"internal crash"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.GenerateStrategy(ctx, nil, "astar")
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Client timeout maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		client.client.Timeout = 10 * time.Millisecond

		_, err := client.GenerateStrategy(ctx, nil, "astar")
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})

	t.Run("Bad JSON response returns decode error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.GenerateStrategy(ctx, nil, "astar")
		if err == nil || !strings.Contains(err.Error(), "failed to decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})
}

func TestHTTPAIClient_CompareSearches(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 with raw JSON pass-through", func(t *testing.T) {
		compareJSON := `{"problem_summary":{"capital":100},"results":{"astar":{"success":true}}}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/search/compare" {
				t.Errorf("expected /api/v1/search/compare, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(compareJSON))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		raw, err := client.CompareSearches(ctx, map[string]any{"capital": 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(raw) != compareJSON {
			t.Errorf("expected %s, got %s", compareJSON, string(raw))
		}
	})

	t.Run("Upstream 500 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":"crash"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.CompareSearches(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})
}

func TestHTTPAIClient_AnalyzeRisk(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 with valid Bayesian and Risk breakdown", func(t *testing.T) {
		riskJSON := `{
			"risk_breakdown": {
				"weighted_asset_risk": 0.045,
				"concentration_index_hhi": 0.32,
				"concentration_penalty": 0.024,
				"liquidity_deficit_penalty": 0.0,
				"leverage_penalty": 0.015,
				"composite_risk_score": 0.068,
				"risk_tier": "Moderate",
				"explanations": ["Base asset risk is 0.0450", "Liquidity 80.0% satisfies floor"]
			},
			"bayesian_inference": {
				"p_recession": 0.42,
				"p_liquidity_crisis": 0.18,
				"p_equity_downturn": 0.457,
				"p_credit_spread_widening": 0.378,
				"stress_multiplier": 1.35,
				"explanation": "Posterior probability of recession is 42.0%"
			},
			"monte_carlo_report": {
				"num_scenarios": 1000,
				"mean_simulated_return": 0.078,
				"std_simulated_return": 0.025,
				"var_95": 0.038,
				"cvar_95": 0.052,
				"var_99": 0.065,
				"cvar_99": 0.079,
				"worst_case_loss_pct": 0.112,
				"probability_of_loss": 0.045
			}
		}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/risk/analyze" {
				t.Errorf("expected /api/v1/risk/analyze, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(riskJSON))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		req := domain.AIRiskAnalyzeRequest{
			State: domain.FinancialState{
				TotalCapital: 100.0,
				Cash:         20.0,
				Allocations:  map[string]float64{"gov_bond": 80.0},
			},
			Problem: domain.FinancialProblem{
				Capital: 100.0,
			},
			RunMonteCarlo: true,
			NumScenarios:  1000,
		}

		res, err := client.AnalyzeRisk(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.RiskBreakdown.RiskTier != "Moderate" {
			t.Errorf("expected Moderate, got %s", res.RiskBreakdown.RiskTier)
		}
		if res.BayesianInference.StressMultiplier != 1.35 {
			t.Errorf("expected 1.35 multiplier, got %f", res.BayesianInference.StressMultiplier)
		}
		if res.MonteCarloReport == nil || res.MonteCarloReport.NumScenarios != 1000 {
			t.Fatalf("expected 1000 scenarios in monte carlo report")
		}
	})

	t.Run("Upstream 502/500 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Input rejected with 400 detail", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"detail":"invalid financial state allocations"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if err == nil || !strings.Contains(err.Error(), "invalid financial state allocations") {
			t.Fatalf("expected rejection detail in error, got: %v", err)
		}
	})

	t.Run("Upstream 422 Unprocessable Entity with detail extraction", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"unprocessable risk payload"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if err == nil || !strings.Contains(err.Error(), "unprocessable risk payload") {
			t.Fatalf("expected 422 detail in error, got: %v", err)
		}
	})

	t.Run("Client timeout maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		client.client.Timeout = 10 * time.Millisecond

		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})

	t.Run("Connection failure to unreachable server maps to ErrUpstreamUnavailable", func(t *testing.T) {
		client := NewHTTPAIClient("http://127.0.0.1:59999") // closed port
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on connection failure, got: %v", err)
		}
	})

	t.Run("Malformed JSON response returns decode error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid-json`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if err == nil || !strings.Contains(err.Error(), "failed to decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})

	t.Run("Unexpected HTTP status 404 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable for 404, got: %v", err)
		}
	})
}

func TestHTTPAIClient_AdversarialStressTest(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 with valid Adversarial Stress Test result", func(t *testing.T) {
		stressJSON := `{
			"worst_case_payoff": 0.0412,
			"best_treasury_action": "Action(INVEST, gov_bond, 5.0)",
			"worst_market_shock": "Severe Equity & Credit Crash",
			"nodes_evaluated": 48,
			"pruned_branches": 12,
			"execution_time_ms": 18.5,
			"explanation": "Adversarial search evaluated 48 nodes with 12 pruned branches."
		}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/stress-test/adversarial" {
				t.Errorf("expected /api/v1/stress-test/adversarial, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(stressJSON))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		problem := domain.FinancialProblem{
			Capital:       100.0,
			RiskTolerance: 0.50,
		}

		res, err := client.AdversarialStressTest(ctx, problem)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.WorstMarketShock != "Severe Equity & Credit Crash" {
			t.Errorf("expected Severe Equity & Credit Crash, got %s", res.WorstMarketShock)
		}
		if res.NodesEvaluated != 48 {
			t.Errorf("expected 48 nodes, got %d", res.NodesEvaluated)
		}
		if res.PrunedBranches != 12 {
			t.Errorf("expected 12 pruned branches, got %d", res.PrunedBranches)
		}
	})

	t.Run("Upstream 500 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Input rejected with 400 detail", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"detail":"invalid stress problem constraints"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AdversarialStressTest(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid stress problem constraints") {
			t.Fatalf("expected rejection detail in error, got: %v", err)
		}
	})

	t.Run("Upstream 422 Unprocessable Entity with detail extraction", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"unprocessable stress problem"}`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AdversarialStressTest(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "unprocessable stress problem") {
			t.Fatalf("expected 422 detail in error, got: %v", err)
		}
	})

	t.Run("Client timeout maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		client.client.Timeout = 10 * time.Millisecond

		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})

	t.Run("Connection failure to unreachable server maps to ErrUpstreamUnavailable", func(t *testing.T) {
		client := NewHTTPAIClient("http://127.0.0.1:59999") // closed port
		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on connection failure, got: %v", err)
		}
	})

	t.Run("Malformed JSON response returns decode error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{broken-json`))
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AdversarialStressTest(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})

	t.Run("Unexpected HTTP status 404 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := NewHTTPAIClient(ts.URL)
		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable for 404, got: %v", err)
		}
	})
}
