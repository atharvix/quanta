package domain

import (
	"encoding/json"
	"time"
)

// FinancialProblemEntity represents a persisted financial problem record in PostgreSQL.
type FinancialProblemEntity struct {
	ID            string          `json:"id"`
	TreasuryID    string          `json:"treasury_id"`
	Algorithm     string          `json:"algorithm"`
	Objective     string          `json:"objective"`
	MarketState   string          `json:"market_state"`
	ProblemJSON   json.RawMessage `json:"problem_json"`
	StrategyLogID *string         `json:"strategy_log_id,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// CreateFinancialProblemInput models the user payload for POST /api/v1/financial-problems.
type CreateFinancialProblemInput struct {
	TreasuryID string           `json:"treasury_id"`
	Algorithm  string           `json:"algorithm"`
	Problem    FinancialProblem `json:"problem"`
}

// CreateFinancialProblemResponse is returned after persisting problem and executing AI strategy.
type CreateFinancialProblemResponse struct {
	ID       string                    `json:"id"`
	Strategy StrategyExecutionResponse `json:"strategy"`
}
