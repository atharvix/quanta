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

func newMockOptimizationClient(handler http.HandlerFunc, timeout ...time.Duration) *HTTPOptimizationClient {
	t := 60 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Result(), nil
	})
	return NewHTTPOptimizationClientWithClient("http://optimization-engine.local", &http.Client{
		Transport: transport,
		Timeout:   t,
	})
}

func TestHTTPOptimizationClient_Health(t *testing.T) {
	ctx := context.Background()

	t.Run("Health 200 OK", func(t *testing.T) {
		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
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

	t.Run("Health 500 Server Error maps to ErrUpstreamUnavailable", func(t *testing.T) {
		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		err := client.Health(ctx)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})

	t.Run("Connection error maps to ErrUpstreamUnavailable", func(t *testing.T) {
		failTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		})
		client := NewHTTPOptimizationClientWithClient("http://unreachable.local", &http.Client{Transport: failTransport})
		err := client.Health(ctx)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})
}

func TestHTTPOptimizationClient_OptimizeClassical(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 with valid classical result", func(t *testing.T) {
		classicalJSON := `{
			"method": "exact",
			"status": "optimal",
			"allocation": {"gov_bond": 0.6, "corp_bond": 0.4},
			"objective_value": 0.085,
			"portfolio_return": 0.09,
			"portfolio_risk": 0.04,
			"liquidity": 0.85,
			"capital_used": 100.0,
			"feasible": true,
			"constraint_violations": [],
			"runtime_ms": 12.4,
			"metadata": {"assets_count": 2}
		}`

		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/optimization/classical" {
				t.Errorf("expected /api/v1/optimization/classical, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(classicalJSON))
		})

		res, err := client.OptimizeClassical(ctx, OptimizationRequest{
			Problem: map[string]any{"capital": 100.0},
			Method:  "exact",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Method != "exact" || res.Status != "optimal" {
			t.Errorf("unexpected method or status: %s, %s", res.Method, res.Status)
		}
		if !res.Feasible {
			t.Errorf("expected feasible result")
		}
		if res.Allocation["gov_bond"] != 0.6 {
			t.Errorf("expected 0.6 for gov_bond, got %f", res.Allocation["gov_bond"])
		}
	})

	t.Run("Input rejected with 400 detail", func(t *testing.T) {
		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"detail":"unknown solver method 'magic'"}`))
		})

		_, err := client.OptimizeClassical(ctx, OptimizationRequest{
			Method: "magic",
		})
		if err == nil || !strings.Contains(err.Error(), "unknown solver method 'magic'") {
			t.Fatalf("expected rejection detail in error, got: %v", err)
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
		client := NewHTTPOptimizationClientWithClient("http://optimization-engine.local", &http.Client{
			Transport: slowTransport,
			Timeout:   10 * time.Millisecond,
		})

		_, err := client.OptimizeClassical(ctx, OptimizationRequest{})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable on timeout, got: %v", err)
		}
	})
}

func TestHTTPOptimizationClient_OptimizeQuantum(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 with QAOA result", func(t *testing.T) {
		qaoaJSON := `{
			"method": "qaoa",
			"status": "feasible",
			"allocation": {"gov_bond": 1.0},
			"objective_value": 0.075,
			"portfolio_return": 0.08,
			"portfolio_risk": 0.03,
			"liquidity": 0.90,
			"capital_used": 50.0,
			"feasible": true,
			"constraint_violations": [],
			"runtime_ms": 45.2,
			"metadata": {"p": 2, "qubits": 4}
		}`

		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/optimization/quantum" {
				t.Errorf("expected /api/v1/optimization/quantum, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(qaoaJSON))
		})

		res, err := client.OptimizeQuantum(ctx, OptimizationRequest{
			Method: "qaoa",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Method != "qaoa" {
			t.Errorf("expected qaoa method, got %s", res.Method)
		}
		if res.Metadata["qubits"] != float64(4) {
			t.Errorf("expected 4 qubits in metadata")
		}
	})

	t.Run("Upstream 422 unprocessable entity", func(t *testing.T) {
		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"n_assets exceeds maximum qubits cap"}`))
		})

		_, err := client.OptimizeQuantum(ctx, OptimizationRequest{})
		if err == nil || !strings.Contains(err.Error(), "n_assets exceeds maximum qubits cap") {
			t.Fatalf("expected detail in error, got: %v", err)
		}
	})
}

func TestHTTPOptimizationClient_CompareOptimizers(t *testing.T) {
	ctx := context.Background()

	t.Run("Success 200 compare response", func(t *testing.T) {
		compareJSON := `{
			"problem_summary": {"assets": 3},
			"results": {
				"exact": {
					"method": "exact",
					"status": "optimal",
					"allocation": {"gov_bond": 0.5, "equity": 0.5},
					"objective_value": 0.1,
					"portfolio_return": 0.1,
					"portfolio_risk": 0.05,
					"liquidity": 0.8,
					"capital_used": 100.0,
					"feasible": true,
					"constraint_violations": [],
					"runtime_ms": 10.0,
					"metadata": {}
				}
			},
			"summary_table": [{"method": "exact", "status": "optimal"}],
			"best_classical_method": "exact",
			"simulation_note": "Scientific note on QAOA simulation"
		}`

		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/optimization/compare" {
				t.Errorf("expected /api/v1/optimization/compare, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(compareJSON))
		})

		res, err := client.CompareOptimizers(ctx, OptimizationCompareRequest{
			Problem: map[string]any{"capital": 100.0},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.BestClassicalMethod != "exact" {
			t.Errorf("expected best method 'exact', got %s", res.BestClassicalMethod)
		}
		if _, ok := res.Results["exact"]; !ok {
			t.Errorf("expected exact in results map")
		}
	})

	t.Run("Malformed JSON response returns decode error", func(t *testing.T) {
		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{not-json-broken`))
		})

		_, err := client.CompareOptimizers(ctx, OptimizationCompareRequest{})
		if err == nil || !strings.Contains(err.Error(), "failed to decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})

	t.Run("Oversized response body returns ErrUpstreamUnavailable with size description", func(t *testing.T) {
		client := newMockOptimizationClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			chunk := make([]byte, 1024*1024)
			for i := 0; i < 11; i++ {
				_, _ = w.Write(chunk)
			}
		})

		_, err := client.OptimizeClassical(ctx, OptimizationRequest{Problem: map[string]any{"capital": 100.0}})
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
