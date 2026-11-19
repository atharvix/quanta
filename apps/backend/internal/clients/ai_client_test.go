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

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockAIClient(handler http.HandlerFunc, timeout ...time.Duration) *HTTPAIClient {
	t := 60 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Result(), nil
	})
	return NewHTTPAIClientWithClient("http://ai-engine.local", &http.Client{
		Transport: transport,
		Timeout:   t,
	})
}

func TestHTTPAIClient_Health(t *testing.T) {
	ctx := context.Background()

	t.Run("Health 200 OK", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("expected /health path, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		})

		if err := client.Health(ctx); err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
	})

	t.Run("Health 500 Server Error", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

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

		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/strategy/generate" {
				t.Errorf("expected /api/v1/strategy/generate, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(responseJSON))
		})

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
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":"internal crash"}`))
		})

		_, err := client.GenerateStrategy(ctx, nil, "astar")
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Client timeout maps to ErrUpstreamUnavailable", func(t *testing.T) {
		slowTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			select {
			case <-time.After(50 * time.Millisecond):
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusOK)
				return rec.Result(), nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})
		client := NewHTTPAIClientWithClient("http://ai-engine.local", &http.Client{
			Transport: slowTransport,
			Timeout:   10 * time.Millisecond,
		})

		_, err := client.GenerateStrategy(ctx, nil, "astar")
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})

	t.Run("Bad JSON response returns decode error", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{not-json`))
		})

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

		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/search/compare" {
				t.Errorf("expected /api/v1/search/compare, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(compareJSON))
		})

		raw, err := client.CompareSearches(ctx, map[string]any{"capital": 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(raw) != compareJSON {
			t.Errorf("expected %s, got %s", compareJSON, string(raw))
		}
	})

	t.Run("Upstream 500 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":"crash"}`))
		})

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

		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/risk/analyze" {
				t.Errorf("expected /api/v1/risk/analyze, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(riskJSON))
		})

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
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})

		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Input rejected with 400 detail", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"detail":"invalid financial state allocations"}`))
		})

		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if err == nil || !strings.Contains(err.Error(), "invalid financial state allocations") {
			t.Fatalf("expected rejection detail in error, got: %v", err)
		}
	})

	t.Run("Upstream 422 Unprocessable Entity with detail extraction", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"unprocessable risk payload"}`))
		})

		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if err == nil || !strings.Contains(err.Error(), "unprocessable risk payload") {
			t.Fatalf("expected 422 detail in error, got: %v", err)
		}
	})

	t.Run("Client timeout maps to ErrUpstreamUnavailable", func(t *testing.T) {
		slowTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			select {
			case <-time.After(50 * time.Millisecond):
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusOK)
				return rec.Result(), nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})
		client := NewHTTPAIClientWithClient("http://ai-engine.local", &http.Client{
			Transport: slowTransport,
			Timeout:   10 * time.Millisecond,
		})

		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})

	t.Run("Connection failure to unreachable server maps to ErrUpstreamUnavailable", func(t *testing.T) {
		failTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp 127.0.0.1:59999: connectex: connection refused")
		})
		client := NewHTTPAIClientWithClient("http://unreachable.local", &http.Client{Transport: failTransport})
		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on connection failure, got: %v", err)
		}
	})

	t.Run("Malformed JSON response returns decode error", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid-json`))
		})

		_, err := client.AnalyzeRisk(ctx, domain.AIRiskAnalyzeRequest{})
		if err == nil || !strings.Contains(err.Error(), "failed to decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})

	t.Run("Unexpected HTTP status 404 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

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

		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/stress-test/adversarial" {
				t.Errorf("expected /api/v1/stress-test/adversarial, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(stressJSON))
		})

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
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Input rejected with 400 detail", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"detail":"invalid stress problem constraints"}`))
		})

		_, err := client.AdversarialStressTest(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid stress problem constraints") {
			t.Fatalf("expected rejection detail in error, got: %v", err)
		}
	})

	t.Run("Upstream 422 Unprocessable Entity with detail extraction", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"unprocessable stress problem"}`))
		})

		_, err := client.AdversarialStressTest(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "unprocessable stress problem") {
			t.Fatalf("expected 422 detail in error, got: %v", err)
		}
	})

	t.Run("Client timeout maps to ErrUpstreamUnavailable", func(t *testing.T) {
		slowTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			select {
			case <-time.After(50 * time.Millisecond):
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusOK)
				return rec.Result(), nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})
		client := NewHTTPAIClientWithClient("http://ai-engine.local", &http.Client{
			Transport: slowTransport,
			Timeout:   10 * time.Millisecond,
		})

		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})

	t.Run("Connection failure to unreachable server maps to ErrUpstreamUnavailable", func(t *testing.T) {
		failTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp 127.0.0.1:59999: connectex: connection refused")
		})
		client := NewHTTPAIClientWithClient("http://unreachable.local", &http.Client{Transport: failTransport})
		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on connection failure, got: %v", err)
		}
	})

	t.Run("Malformed JSON response returns decode error", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{broken-json`))
		})

		_, err := client.AdversarialStressTest(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})

	t.Run("Unexpected HTTP status 404 maps to ErrUpstreamUnavailable", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.AdversarialStressTest(ctx, nil)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable for 404, got: %v", err)
		}
	})
}

func TestReadBoundedResponseBody(t *testing.T) {
	t.Run("Response within limit succeeds", func(t *testing.T) {
		input := "valid small response"
		data, err := readBoundedResponseBody(strings.NewReader(input), 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != input {
			t.Errorf("expected %q, got %q", input, string(data))
		}
	})

	t.Run("Response exactly at limit succeeds", func(t *testing.T) {
		input := "1234567890"
		data, err := readBoundedResponseBody(strings.NewReader(input), 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 10 {
			t.Errorf("expected 10 bytes, got %d", len(data))
		}
	})

	t.Run("Response exceeding limit returns ErrUpstreamUnavailable with size message", func(t *testing.T) {
		input := "12345678901" // 11 bytes with 10 byte limit
		_, err := readBoundedResponseBody(strings.NewReader(input), 10)
		if err == nil {
			t.Fatal("expected error for response exceeding limit, got nil")
		}
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Errorf("expected ErrUpstreamUnavailable, got: %v", err)
		}
		if !strings.Contains(err.Error(), "upstream response exceeded maximum allowed size of 10 bytes") {
			t.Errorf("expected descriptive size error, got: %v", err)
		}
	})
}

func TestHTTPAIClient_ResponseLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("Oversized response body returns ErrUpstreamUnavailable with size description", func(t *testing.T) {
		client := newMockAIClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// Write slightly more than maxResponseBodyBytes
			chunk := make([]byte, 1024*1024) // 1MB chunk
			for i := 0; i < 11; i++ {        // 11MB total > 10MB
				_, _ = w.Write(chunk)
			}
		})

		_, err := client.GenerateStrategy(ctx, map[string]any{"capital": 100}, "astar")
		if err == nil {
			t.Fatal("expected error on oversized response, got nil")
		}
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Errorf("expected ErrUpstreamUnavailable, got: %v", err)
		}
		if !strings.Contains(err.Error(), "exceeded maximum allowed size") {
			t.Errorf("expected message to mention size limit, got: %v", err)
		}
	})
}
