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

// FinancialProblemService manages canonical financial problem creation, AI execution, and retrieval.
type FinancialProblemService struct {
	treasuryRepo repository.TreasuryRepository
	fpRepo       repository.FinancialProblemRepository
	strategyRepo repository.StrategyRepository
	aiClient     clients.AIClient
	auditSvc     *AuditService
}

// NewFinancialProblemService constructs a FinancialProblemService instance.
func NewFinancialProblemService(
	treasuryRepo repository.TreasuryRepository,
	fpRepo repository.FinancialProblemRepository,
	strategyRepo repository.StrategyRepository,
	aiClient clients.AIClient,
	auditSvc *AuditService,
) *FinancialProblemService {
	return &FinancialProblemService{
		treasuryRepo: treasuryRepo,
		fpRepo:       fpRepo,
		strategyRepo: strategyRepo,
		aiClient:     aiClient,
		auditSvc:     auditSvc,
	}
}

// Create validates treasury and problem input, runs AI strategy synthesis, persists problem & strategy, and logs audit.
func (s *FinancialProblemService) Create(
	ctx context.Context,
	input domain.CreateFinancialProblemInput,
) (*domain.CreateFinancialProblemResponse, error) {
	// 1. Validate treasury ID format
	if !uuidRegex.MatchString(input.TreasuryID) {
		return nil, domain.ErrInvalidID
	}

	// 2. Verify treasury exists
	treasury, err := s.treasuryRepo.GetByID(ctx, input.TreasuryID)
	if err != nil {
		return nil, err
	}

	// 3. Validate algorithm
	if input.Algorithm == "" {
		input.Algorithm = "astar"
	}
	if !domain.AllowedAlgorithms[input.Algorithm] {
		return nil, fmt.Errorf("algorithm '%s' is not supported", input.Algorithm)
	}

	// 4. Resolve step size using shared logic (default capital / 20, max 200 steps)
	var stepSizePtr *float64
	if input.Problem.StepSize > 0 {
		val := input.Problem.StepSize
		stepSizePtr = &val
	}
	stepSize, err := domain.ResolveStepSize(treasury.TotalCapital, stepSizePtr)
	if err != nil {
		return nil, err
	}

	// 5. Build and validate canonical problem
	input.Problem.Capital = treasury.TotalCapital
	input.Problem.StepSize = stepSize
	if input.Problem.RiskTolerance == 0 {
		input.Problem.RiskTolerance = treasury.RiskTolerance
	}
	if input.Problem.MinimumLiquidity == 0 {
		input.Problem.MinimumLiquidity = treasury.MinLiquidityRatio
	}
	input.Problem.ApplyDefaults()

	if errs := input.Problem.Validate(); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	// 6. Invoke AI Decision Engine
	start := time.Now()
	aiResp, err := s.aiClient.GenerateStrategy(ctx, input.Problem, input.Algorithm)
	executionTimeMS := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return nil, err
	}

	// 7. Persist strategy log
	allocations := make(map[string]float64)
	if aiResp.SearchResult.FinalState != nil && len(aiResp.SearchResult.FinalState.Allocations) > 0 {
		allocations = aiResp.SearchResult.FinalState.Allocations
	}
	allocationsJSON, _ := json.Marshal(allocations)
	constraintsJSON, _ := json.Marshal(aiResp.ConstraintAudit)

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

	// 8. Persist financial problem record
	problemJSON, _ := json.Marshal(input.Problem)
	savedFP, err := s.fpRepo.Create(ctx, domain.FinancialProblemEntity{
		TreasuryID:    treasury.ID,
		Algorithm:     input.Algorithm,
		Objective:     input.Problem.Objective,
		MarketState:   input.Problem.MarketState,
		ProblemJSON:   problemJSON,
		StrategyLogID: &savedLog.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to persist financial problem: %w", err)
	}

	// 9. Best-effort audit log
	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionFinancialProblemCreated, "financial_problem", &savedFP.ID, map[string]any{
			"treasury_id":     treasury.ID,
			"algorithm":       input.Algorithm,
			"strategy_log_id": savedLog.ID,
		})
	}

	return &domain.CreateFinancialProblemResponse{
		ID: savedFP.ID,
		Strategy: domain.StrategyExecutionResponse{
			StrategyExecutionLog: *savedLog,
			Explanation:          aiResp.Strategy.Explanation,
		},
	}, nil
}

// GetByID retrieves a financial problem by ID.
func (s *FinancialProblemService) GetByID(ctx context.Context, id string) (*domain.FinancialProblemEntity, error) {
	if !uuidRegex.MatchString(id) {
		return nil, domain.ErrInvalidID
	}
	return s.fpRepo.GetByID(ctx, id)
}

// List retrieves financial problems with pagination and optional treasury filter.
func (s *FinancialProblemService) List(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, fmt.Errorf("%w: limit must not exceed 100", domain.ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidPagination)
	}
	if treasuryID != "" && !uuidRegex.MatchString(treasuryID) {
		return nil, domain.ErrInvalidID
	}

	return s.fpRepo.List(ctx, limit, offset, treasuryID)
}
