package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// StrategyService orchestrates strategy synthesis by coordinating corporate treasuries,
// the AI Decision Engine, and audit log persistence.
type StrategyService struct {
	treasuryRepo repository.TreasuryRepository
	strategyRepo repository.StrategyRepository
	aiClient     clients.AIClient
	auditSvc     *AuditService
}

// NewStrategyService constructs a StrategyService instance.
func NewStrategyService(
	treasuryRepo repository.TreasuryRepository,
	strategyRepo repository.StrategyRepository,
	aiClient clients.AIClient,
	auditSvc ...*AuditService,
) *StrategyService {
	var a *AuditService
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &StrategyService{
		treasuryRepo: treasuryRepo,
		strategyRepo: strategyRepo,
		aiClient:     aiClient,
		auditSvc:     a,
	}
}

// GenerateStrategy validates the financial parameters, queries Member 1's AI Decision Engine,
// records the execution wall-clock time, and persists an audit record into strategy_execution_logs.
func (s *StrategyService) GenerateStrategy(
	ctx context.Context,
	treasuryID string,
	input domain.GenerateStrategyInput,
) (*domain.StrategyExecutionResponse, error) {
	// 1. Validate canonical UUID format for treasuryID
	if !uuidRegex.MatchString(treasuryID) {
		return nil, domain.ErrInvalidID
	}

	// 2. Validate requested AI algorithm
	input.ApplyDefaults()
	if err := input.ValidateAlgorithm(); err != nil {
		return nil, err
	}

	// 3. Fetch corporate balance sheet from PostgreSQL
	treasury, err := s.treasuryRepo.GetByID(ctx, treasuryID)
	if err != nil {
		return nil, err
	}

	// 4. Calculate and validate step_size:
	// If omitted or not provided in the request, default step_size = capital / 20.
	// This bounds the discrete search tree depth to ~20 steps, preventing state space explosion in search solvers.
	var stepSize float64
	if input.StepSize == nil {
		stepSize = treasury.TotalCapital / 20.0
	} else {
		if *input.StepSize <= 0 {
			return nil, &domain.ValidationError{
				Details: []string{"step_size must be greater than 0"},
			}
		}
		stepSize = *input.StepSize
	}

	// Guard against excessive search tree depth (max 200 steps).
	// If capital / step_size > 200, the AI engine search queue will exhaust or stall.
	if (treasury.TotalCapital / stepSize) > 200.0 {
		return nil, &domain.ValidationError{
			Details: []string{"step_size too small for capital, max 200 steps"},
		}
	}

	// 5. Construct canonical FinancialProblem mapped to shared schema
	problem := domain.FinancialProblem{
		Capital:          treasury.TotalCapital,
		RiskTolerance:    treasury.RiskTolerance,
		MinimumLiquidity: treasury.MinLiquidityRatio,
		Assets:           input.Assets,
		Liabilities:      input.Liabilities,
		Constraints:      input.Constraints,
		Objective:        input.Objective,
		MarketState:      input.MarketState,
		StepSize:         stepSize,
	}
	problem.ApplyDefaults()

	if errs := problem.Validate(); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	// 6. Invoke AI Decision Engine and measure round-trip execution latency
	start := time.Now()
	aiResp, err := s.aiClient.GenerateStrategy(ctx, problem, input.Algorithm)
	executionTimeMS := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return nil, err
	}

	// 7. Extract allocations JSON from AI response final_state
	allocations := make(map[string]float64)
	if aiResp.SearchResult.FinalState != nil && len(aiResp.SearchResult.FinalState.Allocations) > 0 {
		allocations = aiResp.SearchResult.FinalState.Allocations
	}
	allocationsJSON, err := json.Marshal(allocations)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize allocations to JSONB: %w", err)
	}

	// 8. Extract constraint evaluations JSON
	constraintsJSON, err := json.Marshal(aiResp.ConstraintAudit)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize constraint audit to JSONB: %w", err)
	}

	// 9. Persist audit log into strategy_execution_logs table
	logRecord := domain.StrategyExecutionLog{
		TreasuryID:           treasury.ID,
		AlgorithmUsed:        input.Algorithm,
		ObjectiveValue:       aiResp.Strategy.ObjectiveValue,
		RiskScore:            aiResp.Strategy.RiskScore,
		ExpectedReturn:       aiResp.Strategy.ExpectedReturn,
		LiquidityRatio:       aiResp.Strategy.LiquidityRatio,
		AllocationsJSON:      allocationsJSON,
		ConstraintsAuditJSON: constraintsJSON,
		ExecutionTimeMS:      executionTimeMS,
	}

	savedLog, err := s.strategyRepo.CreateLog(ctx, logRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to persist strategy execution log: %w", err)
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionStrategyGenerated, "strategy", &savedLog.ID, map[string]any{
			"treasury_id":       treasury.ID,
			"algorithm":         input.Algorithm,
			"execution_time_ms": executionTimeMS,
			"objective_value":   aiResp.Strategy.ObjectiveValue,
		})
	}

	return &domain.StrategyExecutionResponse{
		StrategyExecutionLog: *savedLog,
		Explanation:          aiResp.Strategy.Explanation,
	}, nil
}

// GetStrategyByID fetches an individual strategy execution log by UUID.
func (s *StrategyService) GetStrategyByID(ctx context.Context, id string) (*domain.StrategyExecutionLog, error) {
	if !uuidRegex.MatchString(id) {
		return nil, domain.ErrInvalidID
	}
	return s.strategyRepo.GetLogByID(ctx, id)
}

// ListStrategiesByTreasuryID queries execution logs for a corporate treasury.
func (s *StrategyService) ListStrategiesByTreasuryID(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error) {
	if !uuidRegex.MatchString(treasuryID) {
		return nil, domain.ErrInvalidID
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, fmt.Errorf("%w: limit must not exceed 100", domain.ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidPagination)
	}

	return s.strategyRepo.ListLogsByTreasuryID(ctx, treasuryID, limit, offset)
}
