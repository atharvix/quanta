package domain

import (
	"encoding/json"
	"time"
)

// Standard status values for orchestration runs.
const (
	OrchestrationStatusCompleted = "completed"
	OrchestrationStatusFailed    = "failed"
)

// OrchestrationRun represents an end-to-end multi-engine orchestration record.
type OrchestrationRun struct {
	ID                 string          `json:"id"`
	TreasuryID         string          `json:"treasury_id"`
	Status             string          `json:"status"`              // "completed" | "failed"
	AIStatus           string          `json:"ai_status"`           // "completed" | "failed"
	OptimizationStatus string          `json:"optimization_status"` // "not_available"
	StrategyLogID      *string         `json:"strategy_log_id,omitempty"`
	ResponseJSON       json.RawMessage `json:"response_json,omitempty"`
	ErrorMessage       *string         `json:"error_message,omitempty"`
	DurationMS         float64         `json:"duration_ms"`
	CreatedAt          time.Time       `json:"created_at"`
}

// OrchestrateAllocationInput models POST /api/v1/orchestrate/allocation.
type OrchestrateAllocationInput struct {
	TreasuryID        string       `json:"treasury_id"`
	Algorithm         string       `json:"algorithm"`
	StepSize          *float64     `json:"step_size,omitempty"`
	Assets            []Asset      `json:"assets"`
	Liabilities       []Liability  `json:"liabilities"`
	Constraints       []Constraint `json:"constraints"`
	Objective         string       `json:"objective,omitempty"`
	MarketState       string       `json:"market_state,omitempty"`
	IncludeComparison bool         `json:"include_comparison"`
}

// OptimizationStatusResponse indicates Member 2's engine status.
type OptimizationStatusResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// OrchestrateAllocationResponse returns unified orchestration outcome.
type OrchestrateAllocationResponse struct {
	RunID        string                     `json:"run_id"`
	Status       string                     `json:"status"`
	AI           StrategyExecutionResponse  `json:"ai"`
	Comparison   json.RawMessage            `json:"comparison,omitempty"`
	Optimization OptimizationStatusResponse `json:"optimization"`
}
