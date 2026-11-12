// Package clients provides HTTP client wrappers for external AI and Optimization engines.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// AIClient defines outbound calls to Member 1's AI Decision Engine.
type AIClient interface {
	Health(ctx context.Context) error
	GenerateStrategy(ctx context.Context, problem any, algorithm string) (*AIGenerateResponse, error)
	CompareSearches(ctx context.Context, problem any) (json.RawMessage, error)
	AnalyzeRisk(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error)
	AdversarialStressTest(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error)
}

// HTTPAIClient implements AIClient using net/http.
type HTTPAIClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPAIClient constructs an HTTP client pointing to the AI Engine with a 60s timeout for complex search comparisons.
func NewHTTPAIClient(baseURL string) *HTTPAIClient {
	return &HTTPAIClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// AIGenerateRequest models the payload sent to POST /api/v1/strategy/generate.
type AIGenerateRequest struct {
	Problem   any    `json:"problem"`
	Algorithm string `json:"algorithm"`
}

// AIGenerateResponse models the response from the AI Engine.
type AIGenerateResponse struct {
	Strategy        AIStrategy          `json:"strategy"`
	SearchResult    AISearchResult      `json:"search_result"`
	RiskBreakdown   map[string]any      `json:"risk_breakdown"`
	ConstraintAudit []AIConstraintAudit `json:"constraint_audit"`
}

// AIStrategy represents the strategy summary synthesized by the AI engine.
type AIStrategy struct {
	Actions        []any          `json:"actions"`
	TotalAllocated float64        `json:"total_allocated"`
	ExpectedReturn float64        `json:"expected_return"`
	RiskScore      float64        `json:"risk_score"`
	LiquidityRatio float64        `json:"liquidity_ratio"`
	ObjectiveValue float64        `json:"objective_value"`
	IsCompliant    bool           `json:"is_compliant"`
	Explanation    string         `json:"explanation"`
	Metadata       map[string]any `json:"metadata"`
}

// AISearchResult represents the search algorithm diagnostics and terminal state.
type AISearchResult struct {
	Algorithm       string         `json:"algorithm"`
	Success         bool           `json:"success"`
	FinalState      *AIFinalState  `json:"final_state"`
	ObjectiveValue  float64        `json:"objective_value"`
	ExpectedReturn  float64        `json:"expected_return"`
	RiskScore       float64        `json:"risk_score"`
	LiquidityRatio  float64        `json:"liquidity_ratio"`
	StatesExplored  int            `json:"states_explored"`
	ExecutionTimeMS float64        `json:"execution_time_ms"`
	Depth           int            `json:"depth"`
	Metadata        map[string]any `json:"metadata"`
}

// AIFinalState houses the final portfolio allocations and balance sheet metrics.
type AIFinalState struct {
	Allocations     map[string]float64 `json:"allocations"`
	Cash            float64            `json:"cash"`
	TotalCapital    float64            `json:"total_capital"`
	ReservedForDebt float64            `json:"reserved_for_debt"`
	LiquidityRatio  float64            `json:"liquidity_ratio"`
	RiskScore       float64            `json:"risk_score"`
	ExpectedReturn  float64            `json:"expected_return"`
}

// AIConstraintAudit details the individual compliance checks evaluated by the AI engine.
type AIConstraintAudit struct {
	ConstraintID   string  `json:"constraint_id"`
	ConstraintType string  `json:"constraint_type"`
	Satisfied      bool    `json:"satisfied"`
	Actual         float64 `json:"actual"`
	Required       float64 `json:"required"`
	Severity       string  `json:"severity"`
	Message        string  `json:"message"`
	Penalty        float64 `json:"penalty"`
}

// Health pings GET /health on the AI Engine.
func (c *HTTPAIClient) Health(ctx context.Context) error {
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

// GenerateStrategy calls POST /api/v1/strategy/generate on the AI Engine.
func (c *HTTPAIClient) GenerateStrategy(ctx context.Context, problem any, algorithm string) (*AIGenerateResponse, error) {
	reqBody, err := json.Marshal(AIGenerateRequest{
		Problem:   problem,
		Algorithm: algorithm,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode AI request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/strategy/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		var errResp struct {
			Detail any `json:"detail"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errResp); jsonErr == nil && errResp.Detail != nil {
			return nil, fmt.Errorf("AI engine rejected problem input: %v", errResp.Detail)
		}
		return nil, fmt.Errorf("AI engine rejected problem input (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	var aiResp AIGenerateResponse
	if err := json.Unmarshal(bodyBytes, &aiResp); err != nil {
		return nil, fmt.Errorf("failed to decode AI engine response: %w", err)
	}

	return &aiResp, nil
}

// CompareSearches calls POST /api/v1/search/compare on the AI Engine with direct problem payload.
func (c *HTTPAIClient) CompareSearches(ctx context.Context, problem any) (json.RawMessage, error) {
	reqBody, err := json.Marshal(problem)
	if err != nil {
		return nil, fmt.Errorf("failed to encode search compare request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/search/compare", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search compare response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d (%s)", domain.ErrUpstreamUnavailable, resp.StatusCode, string(bodyBytes))
	}

	return json.RawMessage(bodyBytes), nil
}

// AnalyzeRisk calls POST /api/v1/risk/analyze on the AI Engine.
func (c *HTTPAIClient) AnalyzeRisk(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode risk analyze request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/risk/analyze", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		var errResp struct {
			Detail any `json:"detail"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errResp); jsonErr == nil && errResp.Detail != nil {
			return nil, fmt.Errorf("AI engine rejected risk analyze input: %v", errResp.Detail)
		}
		return nil, fmt.Errorf("AI engine rejected risk analyze input (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	var riskResp domain.AIRiskAnalyzeResponse
	if err := json.Unmarshal(bodyBytes, &riskResp); err != nil {
		return nil, fmt.Errorf("failed to decode AI risk response: %w", err)
	}

	return &riskResp, nil
}

// AdversarialStressTest calls POST /api/v1/stress-test/adversarial on the AI Engine with the financial problem.
func (c *HTTPAIClient) AdversarialStressTest(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
	reqBody, err := json.Marshal(problem)
	if err != nil {
		return nil, fmt.Errorf("failed to encode stress test request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/stress-test/adversarial", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		var errResp struct {
			Detail any `json:"detail"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errResp); jsonErr == nil && errResp.Detail != nil {
			return nil, fmt.Errorf("AI engine rejected stress test input: %v", errResp.Detail)
		}
		return nil, fmt.Errorf("AI engine rejected stress test input (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
	}

	var stressResp domain.AdversarialStressTestResult
	if err := json.Unmarshal(bodyBytes, &stressResp); err != nil {
		return nil, fmt.Errorf("failed to decode stress test response: %w", err)
	}

	return &stressResp, nil
}
