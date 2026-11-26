// Package clients provides HTTP client wrappers for external AI and Optimization engines.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// OptimizationClient defines outbound operations to Member 2's Quantum/Classical Optimization Engine.
type OptimizationClient interface {
	Health(ctx context.Context) error
	OptimizeClassical(ctx context.Context, req OptimizationRequest) (*OptimizationResult, error)
	OptimizeQuantum(ctx context.Context, req OptimizationRequest) (*OptimizationResult, error)
	CompareOptimizers(ctx context.Context, req OptimizationCompareRequest) (*OptimizationCompareResponse, error)
}

// OptimizationRequest models the payload sent to Member 2's solver endpoints.
type OptimizationRequest struct {
	Problem         any            `json:"problem"`
	Method          string         `json:"method,omitempty"`
	Options         map[string]any `json:"options,omitempty"`
	CandidateSubset []string       `json:"candidate_subset,omitempty"`
}

// OptimizationResult represents the canonical output from classical and quantum solvers.
type OptimizationResult struct {
	Method               string             `json:"method"`
	Status               string             `json:"status"`
	Allocation           map[string]float64 `json:"allocation"`
	ObjectiveValue       float64            `json:"objective_value"`
	PortfolioReturn      float64            `json:"portfolio_return"`
	PortfolioRisk        float64            `json:"portfolio_risk"`
	Liquidity            float64            `json:"liquidity"`
	CapitalUsed          float64            `json:"capital_used"`
	Feasible             bool               `json:"feasible"`
	ConstraintViolations []string           `json:"constraint_violations"`
	RuntimeMS            float64            `json:"runtime_ms"`
	Metadata             map[string]any     `json:"metadata"`
}

// OptimizationCompareRequest models request to POST /api/v1/optimization/compare.
type OptimizationCompareRequest struct {
	Problem         any            `json:"problem"`
	Options         map[string]any `json:"options,omitempty"`
	CandidateSubset []string       `json:"candidate_subset,omitempty"`
}

// OptimizationCompareResponse models comparative results across solvers.
type OptimizationCompareResponse struct {
	ProblemSummary         map[string]any                `json:"problem_summary"`
	Results                map[string]OptimizationResult `json:"results"`
	SummaryTable           []map[string]any              `json:"summary_table"`
	BestClassicalMethod    string                        `json:"best_classical_method"`
	QAOAApproximationRatio *float64                      `json:"qaoa_approximation_ratio,omitempty"`
	SimulationNote         string                        `json:"simulation_note"`
}

// HTTPOptimizationClient implements OptimizationClient using net/http.
type HTTPOptimizationClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPOptimizationClient constructs an HTTP client pointing to the Optimization Engine with a 60s timeout.
func NewHTTPOptimizationClient(baseURL string) *HTTPOptimizationClient {
	return NewHTTPOptimizationClientWithClient(baseURL, &http.Client{
		Timeout: 60 * time.Second,
	})
}

// NewHTTPOptimizationClientWithClient constructs an HTTPOptimizationClient with custom HTTP client.
func NewHTTPOptimizationClientWithClient(baseURL string, client *http.Client) *HTTPOptimizationClient {
	if client == nil {
		client = &http.Client{
			Timeout: 60 * time.Second,
		}
	}
	return &HTTPOptimizationClient{
		baseURL: baseURL,
		client:  client,
	}
}

// Health pings GET /health on the Optimization Engine.
func (c *HTTPOptimizationClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	return nil
}

// OptimizeClassical calls POST /api/v1/optimization/classical on the Optimization Engine.
func (c *HTTPOptimizationClient) OptimizeClassical(ctx context.Context, req OptimizationRequest) (*OptimizationResult, error) {
	return c.doOptimizationRequest(ctx, "/api/v1/optimization/classical", req)
}

// OptimizeQuantum calls POST /api/v1/optimization/quantum on the Optimization Engine.
func (c *HTTPOptimizationClient) OptimizeQuantum(ctx context.Context, req OptimizationRequest) (*OptimizationResult, error) {
	return c.doOptimizationRequest(ctx, "/api/v1/optimization/quantum", req)
}

// CompareOptimizers calls POST /api/v1/optimization/compare on the Optimization Engine.
func (c *HTTPOptimizationClient) CompareOptimizers(ctx context.Context, req OptimizationCompareRequest) (*OptimizationCompareResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode compare request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/optimization/compare", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := readBoundedResponseBody(resp.Body, maxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		var errResp struct {
			Detail any `json:"detail"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errResp); jsonErr == nil && errResp.Detail != nil {
			return nil, fmt.Errorf("optimization engine rejected compare input: %v", errResp.Detail)
		}
		return nil, fmt.Errorf("optimization engine rejected compare input (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	var compareResp OptimizationCompareResponse
	if err := json.Unmarshal(bodyBytes, &compareResp); err != nil {
		return nil, fmt.Errorf("failed to decode compare response: %w", err)
	}

	return &compareResp, nil
}

func (c *HTTPOptimizationClient) doOptimizationRequest(ctx context.Context, endpoint string, req OptimizationRequest) (*OptimizationResult, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode optimization request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := readBoundedResponseBody(resp.Body, maxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		var errResp struct {
			Detail any `json:"detail"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errResp); jsonErr == nil && errResp.Detail != nil {
			return nil, fmt.Errorf("optimization engine rejected request: %v", errResp.Detail)
		}
		return nil, fmt.Errorf("optimization engine rejected request (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	var optResult OptimizationResult
	if err := json.Unmarshal(bodyBytes, &optResult); err != nil {
		return nil, fmt.Errorf("failed to decode optimization result: %w", err)
	}

	return &optResult, nil
}
