package domain

import (
	"encoding/json"
	"time"
)

// Standard audit action types across the QUANTA platform.
const (
	ActionTreasuryCreated         = "treasury.created"
	ActionTreasuryUpdated         = "treasury.updated"
	ActionCouncilEvaluated        = "council.evaluated"
	ActionStrategyGenerated       = "strategy.generated"
	ActionFinancialProblemCreated = "financial_problem.created"
	ActionOrchestrationCompleted  = "orchestration.completed"
	ActionOrchestrationFailed     = "orchestration.failed"
	ActionAuthRegistered          = "auth.registered"
	ActionAuthLogin               = "auth.login"
	ActionAuthLoginFailed         = "auth.login_failed"
	ActionUserRoleChanged         = "user.role_changed"
	ActionRiskAnalyzed            = "risk.analyzed"
	ActionStressTested            = "stress_test.executed"
	ActionAssetCreated            = "asset.created"
	ActionAssetUpdated            = "asset.updated"
	ActionAssetDeleted            = "asset.deleted"
	ActionLiabilityCreated        = "liability.created"
	ActionLiabilityUpdated        = "liability.updated"
	ActionLiabilityDeleted        = "liability.deleted"
	ActionOptimizationClassical   = "optimization.classical"
	ActionOptimizationQuantum     = "optimization.quantum"
	ActionOptimizationCompared    = "optimization.compared"
)

// AuditLog represents an immutable audit trail entry tracking state mutations.
type AuditLog struct {
	ID         string          `json:"id"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   *string         `json:"entity_id,omitempty"`
	Actor      string          `json:"actor"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
