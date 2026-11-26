// Package domain provides core enterprise domain models and validation rules
// mapped directly to the shared QUANTA canonical JSON schemas.
package domain

// AgentRole defines the role of an intelligent agent on the AI Council.
type AgentRole string

const (
	AgentRoleTreasury   AgentRole = "treasury"
	AgentRoleRisk       AgentRole = "risk"
	AgentRoleCompliance AgentRole = "compliance"
	AgentRoleDecision   AgentRole = "decision"
)

// FinancialAction models a recommended operational step in the treasury portfolio.
type FinancialAction struct {
	ActionType  string  `json:"action_type"`
	AssetID     *string `json:"asset_id,omitempty"`
	LiabilityID *string `json:"liability_id,omitempty"`
	Amount      float64 `json:"amount"`
	Cost        float64 `json:"cost"`
	Reason      string  `json:"reason"`
}

// AgentActionProposal models an action suggested by a specialized council agent.
type AgentActionProposal struct {
	ProposedBy           AgentRole       `json:"proposed_by"`
	Action               FinancialAction `json:"action"`
	Confidence           float64         `json:"confidence"`
	ExpectedUtilityDelta float64         `json:"expected_utility_delta"`
	Rationale            string          `json:"rationale"`
}

// AgentAssessment captures the diagnostic opinion of an individual council member.
type AgentAssessment struct {
	AgentName        string                `json:"agent_name"`
	Role             AgentRole             `json:"role"`
	IsAcceptable     bool                  `json:"is_acceptable"`
	Score            float64               `json:"score"`
	Findings         []string              `json:"findings"`
	SuggestedActions []AgentActionProposal `json:"suggested_actions"`
}

// StrategySummary captures the final recommended allocation strategy synthesized by DecisionAgent.
type StrategySummary struct {
	Actions        []FinancialAction      `json:"actions"`
	TotalAllocated float64                `json:"total_allocated"`
	ExpectedReturn float64                `json:"expected_return"`
	RiskScore      float64                `json:"risk_score"`
	LiquidityRatio float64                `json:"liquidity_ratio"`
	ObjectiveValue float64                `json:"objective_value"`
	IsCompliant    bool                   `json:"is_compliant"`
	Explanation    string                 `json:"explanation"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// SearchMetrics captures algorithmic search execution telemetry.
type SearchMetrics struct {
	Algorithm       string                 `json:"algorithm"`
	Success         bool                   `json:"success"`
	ObjectiveValue  float64                `json:"objective_value"`
	ExpectedReturn  float64                `json:"expected_return"`
	RiskScore       float64                `json:"risk_score"`
	LiquidityRatio  float64                `json:"liquidity_ratio"`
	StatesExplored  int                    `json:"states_explored"`
	ExecutionTimeMS float64                `json:"execution_time_ms"`
	Depth           int                    `json:"depth"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// CouncilDeliberationSummary captures the synthesized consensus and individual agent evaluations.
type CouncilDeliberationSummary struct {
	TreasuryOpinion     AgentAssessment `json:"treasury_opinion"`
	RiskOpinion         AgentAssessment `json:"risk_opinion"`
	ComplianceOpinion   AgentAssessment `json:"compliance_opinion"`
	ConsensusReached    bool            `json:"consensus_reached"`
	RecommendedStrategy StrategySummary `json:"recommended_strategy"`
	SearchMetrics       SearchMetrics   `json:"search_metrics"`
}

// EvaluateCouncilInput models the request payload sent by the client to evaluate the Multi-Agent Council.
type EvaluateCouncilInput struct {
	Assets      []Asset      `json:"assets"`
	Liabilities []Liability  `json:"liabilities"`
	Constraints []Constraint `json:"constraints"`
	StepSize    *float64     `json:"step_size,omitempty"`
	Objective   string       `json:"objective,omitempty"`
	MarketState string       `json:"market_state,omitempty"`
}
