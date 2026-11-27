package intelligence_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
)

func TestHTTPClient_OptimizeContractAndCorrelationPropagation(t *testing.T) {
	var seenCorr string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCorr = r.Header.Get(middleware.CorrelationIDHeader)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contract_version":                    "1.0",
			"correlation_id":                      seenCorr,
			"status":                              "COMPLETED",
			"feasible":                            true,
			"solver_used":                         "scipy_linprog_highs",
			"solver_version":                      "1.18.1",
			"is_authoritative":                    false,
			"authoritative_verification_required": true,
			"allocations":                         map[string]string{"mmf_a": "700.0000"},
			"unallocated_cash":                    "300.0000",
			"runtime_ms":                          5,
		})
	}))
	defer srv.Close()

	client := intelligence.NewHTTPClient(srv.URL, 2*time.Second)
	resp, err := client.Optimize(context.Background(), intelligence.OptimizeRequest{
		ContractVersion:         "1.0",
		CorrelationID:           "corr-contract-1",
		TreasuryID:              "t-1",
		BaseCurrency:            "USD",
		InvestableCapital:       "1000.0000",
		OperatingLiquidityFloor: "100.0000",
		EmergencyReserveFloor:   "100.0000",
		Candidates:              []intelligence.AllocationCandidateInput{{ID: "mmf_a", ExpectedReturn: "0.05", MinAllocation: "0", MaxAllocation: "1000"}},
		SolverMode:              "lp",
	})
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}
	if seenCorr != "corr-contract-1" || resp.Allocations["mmf_a"] != "700.0000" {
		t.Fatalf("unexpected contract result: corr=%q resp=%+v", seenCorr, resp)
	}
}

func TestHTTPClient_RejectsMalformedTimeoutAndUnavailable(t *testing.T) {
	// Malformed decimal
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contract_version": "1.0",
			"correlation_id":   "c1",
			"status":           "COMPLETED",
			"is_authoritative": false,
			"allocations":      map[string]string{"mmf_a": "not-a-number"},
			"unallocated_cash": "0.0000",
		})
	}))
	defer badSrv.Close()

	c1 := intelligence.NewHTTPClient(badSrv.URL, time.Second)
	if _, err := c1.Optimize(context.Background(), intelligence.OptimizeRequest{CorrelationID: "c1"}); !errors.Is(err, intelligence.ErrMalformedResponse) {
		t.Fatalf("expected ErrMalformedResponse, got %v", err)
	}

	// Timeout
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowSrv.Close()

	c2 := intelligence.NewHTTPClient(slowSrv.URL, 40*time.Millisecond)
	if _, err := c2.Optimize(context.Background(), intelligence.OptimizeRequest{CorrelationID: "c2"}); !errors.Is(err, intelligence.ErrUpstreamTimeout) {
		t.Fatalf("expected ErrUpstreamTimeout, got %v", err)
	}

	// Unavailable
	c3 := intelligence.NewHTTPClient("http://127.0.0.1:1", 200*time.Millisecond)
	if _, err := c3.Optimize(context.Background(), intelligence.OptimizeRequest{CorrelationID: "c3"}); !errors.Is(err, intelligence.ErrUpstreamUnavailable) {
		t.Fatalf("expected ErrUpstreamUnavailable, got %v", err)
	}
}
