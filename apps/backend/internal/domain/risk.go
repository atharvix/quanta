// Package domain defines core entities, value objects, and repository interfaces.
package domain

// FinancialState represents the balance-sheet state of a corporate treasury.
type FinancialState struct {
	TotalCapital    float64            `json:"total_capital"`
	Cash            float64            `json:"cash"`
	Allocations     map[string]float64 `json:"allocations,omitempty"`
	Liabilities     []Liability        `json:"liabilities,omitempty"`
	ReservedForDebt float64            `json:"reserved_for_debt"`
	LiquidityRatio  float64            `json:"liquidity_ratio"`
	RiskScore       float64            `json:"risk_score"`
	ExpectedReturn  float64            `json:"expected_return"`
	MarketState     string             `json:"market_state,omitempty"`
	History         []any              `json:"history,omitempty"`
}

// BayesianEvidence models observed macroeconomic signals for Bayesian risk inference.
type BayesianEvidence struct {
	InflationHigh   *bool `json:"inflation_high,omitempty"`
	RateHike        *bool `json:"rate_hike,omitempty"`
	RecessionSignal *bool `json:"recession_signal,omitempty"`
}

// BayesianRiskInferenceResult captures the posterior probabilities inferred from the Bayesian network.
type BayesianRiskInferenceResult struct {
	PRecession            float64 `json:"p_recession"`
	PLiquidityCrisis      float64 `json:"p_liquidity_crisis"`
	PEquityDownturn       float64 `json:"p_equity_downturn"`
	PCreditSpreadWidening float64 `json:"p_credit_spread_widening"`
	StressMultiplier      float64 `json:"stress_multiplier"`
	Explanation           string  `json:"explanation"`
}

// RiskFactorBreakdown provides granular explainability of portfolio risk components.
type RiskFactorBreakdown struct {
	WeightedAssetRisk       float64  `json:"weighted_asset_risk"`
	ConcentrationIndexHHI   float64  `json:"concentration_index_hhi"`
	ConcentrationPenalty    float64  `json:"concentration_penalty"`
	LiquidityDeficitPenalty float64  `json:"liquidity_deficit_penalty"`
	LeveragePenalty         float64  `json:"leverage_penalty"`
	CompositeRiskScore      float64  `json:"composite_risk_score"`
	RiskTier                string   `json:"risk_tier"`
	Explanations            []string `json:"explanations"`
}

// MonteCarloSimulationReport contains statistical distribution parameters from Monte Carlo return simulations.
type MonteCarloSimulationReport struct {
	NumScenarios        int     `json:"num_scenarios"`
	MeanSimulatedReturn float64 `json:"mean_simulated_return"`
	StdSimulatedReturn  float64 `json:"std_simulated_return"`
	VaR95               float64 `json:"var_95"`
	CVaR95              float64 `json:"cvar_95"`
	VaR99               float64 `json:"var_99"`
	CVaR99              float64 `json:"cvar_99"`
	WorstCaseLossPct    float64 `json:"worst_case_loss_pct"`
	ProbabilityOfLoss   float64 `json:"probability_of_loss"`
}

// AIRiskAnalyzeRequest models the payload sent to POST /api/v1/risk/analyze on the AI Engine.
type AIRiskAnalyzeRequest struct {
	State         FinancialState    `json:"state"`
	Problem       FinancialProblem  `json:"problem"`
	Evidence      *BayesianEvidence `json:"evidence,omitempty"`
	RunMonteCarlo bool              `json:"run_monte_carlo"`
	NumScenarios  int               `json:"num_scenarios"`
}

// AIRiskAnalyzeResponse models the response from POST /api/v1/risk/analyze on the AI Engine.
type AIRiskAnalyzeResponse struct {
	RiskBreakdown     RiskFactorBreakdown         `json:"risk_breakdown"`
	BayesianInference BayesianRiskInferenceResult `json:"bayesian_inference"`
	MonteCarloReport  *MonteCarloSimulationReport `json:"monte_carlo_report,omitempty"`
}

// AdversarialStressTestResult models the response from POST /api/v1/stress-test/adversarial.
type AdversarialStressTestResult struct {
	WorstCasePayoff    float64 `json:"worst_case_payoff"`
	BestTreasuryAction *string `json:"best_treasury_action"`
	WorstMarketShock   string  `json:"worst_market_shock"`
	NodesEvaluated     int     `json:"nodes_evaluated"`
	PrunedBranches     int     `json:"pruned_branches"`
	ExecutionTimeMS    float64 `json:"execution_time_ms"`
	Explanation        string  `json:"explanation"`
}

// MarketShockScenario models a discrete macroeconomic shock scenario.
type MarketShockScenario struct {
	ShockName         string  `json:"shock_name"`
	EquityDrawdown    float64 `json:"equity_drawdown"`
	CreditSpreadSpike float64 `json:"credit_spread_spike"`
	LiquidityFreeze   float64 `json:"liquidity_freeze"`
	Description       string  `json:"description"`
}

// StandardMarketShocks defines the four discrete market stress scenarios modeled in the AI Engine.
var StandardMarketShocks = []MarketShockScenario{
	{
		ShockName:         "Status Quo",
		EquityDrawdown:    0.0,
		CreditSpreadSpike: 0.0,
		LiquidityFreeze:   0.0,
		Description:       "Benign macroeconomic environment",
	},
	{
		ShockName:         "Liquidity Squeeze",
		EquityDrawdown:    -0.10,
		CreditSpreadSpike: -0.05,
		LiquidityFreeze:   -0.25,
		Description:       "Short-term commercial paper and liquidity freeze",
	},
	{
		ShockName:         "Macro Stagflation & Rate Shock",
		EquityDrawdown:    -0.20,
		CreditSpreadSpike: -0.12,
		LiquidityFreeze:   -0.15,
		Description:       "Central bank rate hike spike causing credit spread widening",
	},
	{
		ShockName:         "Severe Equity & Credit Crash",
		EquityDrawdown:    -0.35,
		CreditSpreadSpike: -0.20,
		LiquidityFreeze:   -0.30,
		Description:       "Systemic black swan crisis with simultaneous drawdowns",
	},
}

// ScenarioEvaluation captures deterministic shock impacts calculated for a specific portfolio.
type ScenarioEvaluation struct {
	Scenario          MarketShockScenario `json:"scenario"`
	StressedReturn    float64             `json:"stressed_return"`
	StressedLiquidity float64             `json:"stressed_liquidity"`
	StressedRisk      float64             `json:"stressed_risk"`
	EstimatedPayoff   float64             `json:"estimated_payoff"`
	IsCompliant       bool                `json:"is_compliant"`
}

// AnalyzeTreasuryRiskInput models the service request to perform Bayesian and multi-factor risk analysis on a Treasury.
type AnalyzeTreasuryRiskInput struct {
	Assets        []Asset            `json:"assets"`
	Liabilities   []Liability        `json:"liabilities"`
	Constraints   []Constraint       `json:"constraints"`
	Allocations   map[string]float64 `json:"allocations,omitempty"`
	StepSize      *float64           `json:"step_size,omitempty"`
	Objective     string             `json:"objective,omitempty"`
	MarketState   string             `json:"market_state,omitempty"`
	Evidence      *BayesianEvidence  `json:"evidence,omitempty"`
	RunMonteCarlo *bool              `json:"run_monte_carlo,omitempty"`
	NumScenarios  *int               `json:"num_scenarios,omitempty"`
}

// AdversarialStressTestInput models the service request to perform Alpha-Beta adversarial stress testing on a Treasury.
type AdversarialStressTestInput struct {
	Assets      []Asset            `json:"assets"`
	Liabilities []Liability        `json:"liabilities"`
	Constraints []Constraint       `json:"constraints"`
	Allocations map[string]float64 `json:"allocations,omitempty"`
	StepSize    *float64           `json:"step_size,omitempty"`
	Objective   string             `json:"objective,omitempty"`
	MarketState string             `json:"market_state,omitempty"`
}

// TreasuryStressTestResponse combines the Alpha-Beta game tree search result with the 4 discrete scenario evaluations.
type TreasuryStressTestResponse struct {
	AdversarialResult AdversarialStressTestResult `json:"adversarial_result"`
	Scenarios         []ScenarioEvaluation        `json:"scenarios"`
}
