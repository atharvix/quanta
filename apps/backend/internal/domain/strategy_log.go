package domain

import (
	"fmt"
	"strings"
)

// Allowed search algorithms supported by Member 1's AI Decision Engine.
var AllowedAlgorithms = map[string]bool{
	"bfs":               true,
	"dfs":               true,
	"generate_and_test": true,
	"hill_climbing":     true,
	"best_first":        true,
	"astar":             true,
}

// GenerateStrategyInput models the user payload sent to POST /api/v1/treasuries/{id}/strategies/generate.
type GenerateStrategyInput struct {
	Algorithm   string       `json:"algorithm"`
	Assets      []Asset      `json:"assets"`
	Liabilities []Liability  `json:"liabilities"`
	Constraints []Constraint `json:"constraints"`
	Objective   string       `json:"objective,omitempty"`
	MarketState string       `json:"market_state,omitempty"`
	StepSize    *float64     `json:"step_size,omitempty"`
}

// ApplyDefaults populates default algorithm and parameters if omitted.
func (g *GenerateStrategyInput) ApplyDefaults() {
	if strings.TrimSpace(g.Algorithm) == "" {
		g.Algorithm = "astar"
	}
	if strings.TrimSpace(g.Objective) == "" {
		g.Objective = DefaultObjective
	}
	if strings.TrimSpace(g.MarketState) == "" {
		g.MarketState = DefaultMarketState
	}
	// Note: StepSize default is not assigned here; it is dynamically calculated in
	// StrategyService (capital / 20) based on the corporate treasury's total_capital.
}

// ValidateAlgorithm verifies if the requested algorithm is supported by the AI engine.
func (g *GenerateStrategyInput) ValidateAlgorithm() error {
	algo := strings.ToLower(strings.TrimSpace(g.Algorithm))
	if !AllowedAlgorithms[algo] {
		return fmt.Errorf("algorithm '%s' is not supported, allowed: [bfs, dfs, generate_and_test, hill_climbing, best_first, astar]", g.Algorithm)
	}
	return nil
}

// StrategyExecutionResponse embeds the persisted audit record along with the AI engine's explainability summary.
type StrategyExecutionResponse struct {
	StrategyExecutionLog
	Explanation string `json:"explanation"`
}
