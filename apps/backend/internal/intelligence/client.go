package intelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/money"
)

const ContractVersion = "1.0"

var (
	ErrUpstreamUnavailable = errors.New("python intelligence service is unavailable")
	ErrUpstreamTimeout     = errors.New("python intelligence service request timed out")
	ErrMalformedResponse   = errors.New("python intelligence service returned malformed contract payload")
)

type ForecastRequest struct {
	ContractVersion string    `json:"contract_version"`
	CorrelationID   string    `json:"correlation_id"`
	TreasuryID      string    `json:"treasury_id"`
	BaseCurrency    string    `json:"base_currency"`
	History         []*string `json:"history"`
	Horizon         int       `json:"horizon"`
	SeasonLength    int       `json:"season_length"`
	Model           string    `json:"model"`
}

type ForecastPoint struct {
	Step int     `json:"step"`
	P10  *string `json:"p10"`
	P50  string  `json:"p50"`
	P90  *string `json:"p90"`
}

type BacktestMetrics struct {
	Supported  bool    `json:"supported"`
	Folds      int     `json:"folds"`
	MAE        *string `json:"mae"`
	RMSE       *string `json:"rmse"`
	Coverage80 *string `json:"coverage_80"`
	Reason     *string `json:"reason"`
}

type ForecastResponse struct {
	ContractVersion    string          `json:"contract_version"`
	CorrelationID      string          `json:"correlation_id"`
	ModelName          string          `json:"model_name"`
	ModelVersion       string          `json:"model_version"`
	IsAuthoritative    bool            `json:"is_authoritative"`
	InputChecksPassed  bool            `json:"input_checks_passed"`
	MissingValuesCount int             `json:"missing_values_count"`
	Forecasts          []ForecastPoint `json:"forecasts"`
	Backtest           BacktestMetrics `json:"backtest"`
	Warnings           []string        `json:"warnings"`
}

type StressScenarioInput struct {
	Name     string `json:"name"`
	ShockPct string `json:"shock_pct"`
}

type RiskRequest struct {
	ContractVersion string                `json:"contract_version"`
	CorrelationID   string                `json:"correlation_id"`
	TreasuryID      string                `json:"treasury_id"`
	BaseCurrency    string                `json:"base_currency"`
	PortfolioValue  string                `json:"portfolio_value"`
	ReturnsHistory  []string              `json:"returns_history"`
	ConfidenceLevel string                `json:"confidence_level"`
	NumSimulations  int                   `json:"num_simulations"`
	Seed            int                   `json:"seed"`
	StressScenarios []StressScenarioInput `json:"stress_scenarios"`
}

type RiskResponse struct {
	ContractVersion   string   `json:"contract_version"`
	CorrelationID     string   `json:"correlation_id"`
	ModelName         string   `json:"model_name"`
	ModelVersion      string   `json:"model_version"`
	Seed              int      `json:"seed"`
	IsAuthoritative   bool     `json:"is_authoritative"`
	InputChecksPassed bool     `json:"input_checks_passed"`
	ConfidenceLevel   string   `json:"confidence_level"`
	HistoricalVaR     string   `json:"historical_var"`
	HistoricalCVaR    string   `json:"historical_cvar"`
	ParametricVaR     string   `json:"parametric_var"`
	ParametricCVaR    string   `json:"parametric_cvar"`
	MonteCarloVaR     string   `json:"monte_carlo_var"`
	MonteCarloCVaR    string   `json:"monte_carlo_cvar"`
	StressResults     []any    `json:"stress_results"`
	Anomalies         any      `json:"anomalies"`
	Assumptions       []string `json:"assumptions"`
	Warnings          []string `json:"warnings"`
}

type AllocationCandidateInput struct {
	ID             string  `json:"id"`
	ExpectedReturn string  `json:"expected_return"`
	MinAllocation  string  `json:"min_allocation"`
	MaxAllocation  string  `json:"max_allocation"`
	CounterpartyID *string `json:"counterparty_id,omitempty"`
	LotSize        *string `json:"lot_size,omitempty"`
}

type CounterpartyLimitInput struct {
	CounterpartyID   string `json:"counterparty_id"`
	MaxExposureLimit string `json:"max_exposure_limit"`
	CurrentExposure  string `json:"current_exposure"`
}

type OptimizeRequest struct {
	ContractVersion         string                     `json:"contract_version"`
	CorrelationID           string                     `json:"correlation_id"`
	TreasuryID              string                     `json:"treasury_id"`
	BaseCurrency            string                     `json:"base_currency"`
	InvestableCapital       string                     `json:"investable_capital"`
	OperatingLiquidityFloor string                     `json:"operating_liquidity_floor"`
	EmergencyReserveFloor   string                     `json:"emergency_reserve_floor"`
	TotalAvailableCash      *string                    `json:"total_available_cash,omitempty"`
	Candidates              []AllocationCandidateInput `json:"candidates"`
	CounterpartyLimits      []CounterpartyLimitInput   `json:"counterparty_limits"`
	SolverMode              string                     `json:"solver_mode"`
	AllowUnallocatedCash    bool                       `json:"allow_unallocated_cash"`
	TimeoutMs               int                        `json:"timeout_ms"`
}

type OptimizeResponse struct {
	ContractVersion                 string            `json:"contract_version"`
	CorrelationID                   string            `json:"correlation_id"`
	Status                          string            `json:"status"`
	Feasible                        bool              `json:"feasible"`
	SolverUsed                      string            `json:"solver_used"`
	SolverVersion                   string            `json:"solver_version"`
	IsAuthoritative                 bool              `json:"is_authoritative"`
	AuthoritativeVerificationNeeded bool              `json:"authoritative_verification_required"`
	Allocations                     map[string]string `json:"allocations"`
	UnallocatedCash                 string            `json:"unallocated_cash"`
	ObjectiveValue                  *string           `json:"objective_value"`
	ConstraintDiagnostics           []string          `json:"constraint_diagnostics"`
	Warnings                        []string          `json:"warnings"`
	RuntimeMs                       int               `json:"runtime_ms"`
	RawMetadata                     map[string]any    `json:"raw_metadata"`
}

// Client defines the Go -> Python intelligence boundary.
type Client interface {
	Forecast(ctx context.Context, req ForecastRequest) (ForecastResponse, error)
	EvaluateRisk(ctx context.Context, req RiskRequest) (RiskResponse, error)
	Optimize(ctx context.Context, req OptimizeRequest) (OptimizeResponse, error)
}

// HTTPClient calls the stateless Python FastAPI service with timeouts and strict validation.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// NewHTTPClient creates an HTTPClient for the Python intelligence service.
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
	}
}

func (c *HTTPClient) Forecast(ctx context.Context, req ForecastRequest) (ForecastResponse, error) {
	if req.ContractVersion == "" {
		req.ContractVersion = ContractVersion
	}
	var resp ForecastResponse
	if err := c.postJSON(ctx, "/internal/v1/forecast", req.CorrelationID, req, &resp); err != nil {
		return ForecastResponse{}, err
	}
	if resp.ContractVersion != ContractVersion || resp.IsAuthoritative || len(resp.Forecasts) == 0 {
		return ForecastResponse{}, fmt.Errorf("%w: invalid forecast response envelope", ErrMalformedResponse)
	}
	for _, pt := range resp.Forecasts {
		if _, err := money.FromString(pt.P50); err != nil {
			return ForecastResponse{}, fmt.Errorf("%w: invalid forecast p50 %q", ErrMalformedResponse, pt.P50)
		}
	}
	return resp, nil
}

func (c *HTTPClient) EvaluateRisk(ctx context.Context, req RiskRequest) (RiskResponse, error) {
	if req.ContractVersion == "" {
		req.ContractVersion = ContractVersion
	}
	var resp RiskResponse
	if err := c.postJSON(ctx, "/internal/v1/risk/evaluate", req.CorrelationID, req, &resp); err != nil {
		return RiskResponse{}, err
	}
	if resp.ContractVersion != ContractVersion || resp.IsAuthoritative {
		return RiskResponse{}, fmt.Errorf("%w: invalid risk response envelope", ErrMalformedResponse)
	}
	for _, field := range []string{resp.HistoricalVaR, resp.HistoricalCVaR, resp.ParametricVaR, resp.ParametricCVaR, resp.MonteCarloVaR, resp.MonteCarloCVaR} {
		if _, err := money.FromString(field); err != nil {
			return RiskResponse{}, fmt.Errorf("%w: invalid risk decimal %q", ErrMalformedResponse, field)
		}
	}
	return resp, nil
}

func (c *HTTPClient) Optimize(ctx context.Context, req OptimizeRequest) (OptimizeResponse, error) {
	if req.ContractVersion == "" {
		req.ContractVersion = ContractVersion
	}
	var resp OptimizeResponse
	if err := c.postJSON(ctx, "/internal/v1/optimize", req.CorrelationID, req, &resp); err != nil {
		return OptimizeResponse{}, err
	}
	if resp.ContractVersion != ContractVersion || resp.IsAuthoritative {
		return OptimizeResponse{}, fmt.Errorf("%w: invalid optimize response envelope", ErrMalformedResponse)
	}
	switch resp.Status {
	case "COMPLETED", "INFEASIBLE", "SOLVER_FAILED":
	default:
		return OptimizeResponse{}, fmt.Errorf("%w: unknown solver status %q", ErrMalformedResponse, resp.Status)
	}
	if _, err := money.FromString(resp.UnallocatedCash); err != nil {
		return OptimizeResponse{}, fmt.Errorf("%w: invalid unallocated_cash %q", ErrMalformedResponse, resp.UnallocatedCash)
	}
	for k, v := range resp.Allocations {
		if _, err := money.FromString(v); err != nil {
			return OptimizeResponse{}, fmt.Errorf("%w: invalid allocation for %s: %q", ErrMalformedResponse, k, v)
		}
	}
	return resp, nil
}

func (c *HTTPClient) postJSON(ctx context.Context, path, corrID string, payload any, dst any) error {
	if c.baseURL == "" {
		return ErrUpstreamUnavailable
	}
	if corrID == "" {
		corrID = middleware.CorrelationIDFromContext(ctx)
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if corrID != "" {
		httpReq.Header.Set(middleware.CorrelationIDHeader, corrID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrUpstreamTimeout
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return ErrUpstreamTimeout
		}
		return fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", ErrMalformedResponse, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrMalformedResponse, resp.StatusCode)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%w: unmarshal json: %v", ErrMalformedResponse, err)
	}
	return nil
}
