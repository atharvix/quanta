package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// Treasury represents a corporate entity whose balance sheet capital is optimized by QUANTA.
// Maps directly to the `corporate_treasuries` table in infrastructure/postgres/init.sql.
type Treasury struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	BaseCurrency      string    `json:"base_currency"`
	TotalCapital      float64   `json:"total_capital"`
	RiskTolerance     float64   `json:"risk_tolerance"`
	MinLiquidityRatio float64   `json:"min_liquidity_ratio"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateTreasuryInput defines the payload required to onboard a corporate treasury.
type CreateTreasuryInput struct {
	Name              string  `json:"name"`
	BaseCurrency      string  `json:"base_currency"`
	TotalCapital      float64 `json:"total_capital"`
	RiskTolerance     float64 `json:"risk_tolerance"`
	MinLiquidityRatio float64 `json:"min_liquidity_ratio"`
}

// ApplyDefaults assigns production defaults if optional fields are omitted.
func (c *CreateTreasuryInput) ApplyDefaults() {
	if strings.TrimSpace(c.BaseCurrency) == "" {
		c.BaseCurrency = "INR"
	}
	if c.RiskTolerance == 0 {
		c.RiskTolerance = 0.50
	}
	if c.MinLiquidityRatio == 0 {
		c.MinLiquidityRatio = 0.20
	}
}

// Validate checks business rules for creating a treasury.
func (c *CreateTreasuryInput) Validate() []string {
	var errs []string

	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, "name is required")
	}
	if c.TotalCapital <= 0 {
		errs = append(errs, "total_capital must be greater than 0")
	}
	if c.RiskTolerance < 0.0 || c.RiskTolerance > 1.0 {
		errs = append(errs, "risk_tolerance must be between 0.0 and 1.0")
	}
	if c.MinLiquidityRatio < 0.0 || c.MinLiquidityRatio > 1.0 {
		errs = append(errs, "min_liquidity_ratio must be between 0.0 and 1.0")
	}

	return errs
}

// StrategyExecutionLog stores audit records of each optimization run.
// Maps directly to `strategy_execution_logs` in infrastructure/postgres/init.sql.
type StrategyExecutionLog struct {
	ID                   string          `json:"id"`
	TreasuryID           string          `json:"treasury_id"`
	AlgorithmUsed        string          `json:"algorithm_used"`
	ObjectiveValue       float64         `json:"objective_value"`
	RiskScore            float64         `json:"risk_score"`
	ExpectedReturn       float64         `json:"expected_return"`
	LiquidityRatio       float64         `json:"liquidity_ratio"`
	AllocationsJSON      json.RawMessage `json:"allocations_json"`
	ConstraintsAuditJSON json.RawMessage `json:"constraints_audit_json"`
	ExecutionTimeMS      float64         `json:"execution_time_ms"`
	CreatedAt            time.Time       `json:"created_at"`
}
