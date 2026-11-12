package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// OrchestrationService manages end-to-end multi-engine allocation orchestration.
type OrchestrationService struct {
	treasuryRepo repository.TreasuryRepository
	orchRepo     repository.OrchestrationRepository
	strategyRepo repository.StrategyRepository
	aiClient     clients.AIClient
	auditSvc     *AuditService
}

// NewOrchestrationService constructs an OrchestrationService instance.
func NewOrchestrationService(
	treasuryRepo repository.TreasuryRepository,
	orchRepo repository.OrchestrationRepository,
	strategyRepo repository.StrategyRepository,
	aiClient clients.AIClient,
	auditSvc *AuditService,
) *OrchestrationService {
	return &OrchestrationService{
		treasuryRepo: treasuryRepo,
		orchRepo:     orchRepo,
		strategyRepo: strategyRepo,
		aiClient:     aiClient,
		auditSvc:     auditSvc,
	}
}

// Orchestrate executes full capital allocation flow across engines, persists runs, and handles failure tracking.
func (s *OrchestrationService) Orchestrate(
	ctx context.Context,
	input domain.OrchestrateAllocationInput,
) (*domain.OrchestrateAllocationResponse, error) {
	// 1. Validate treasury ID
	if !uuidRegex.MatchString(input.TreasuryID) {
		return nil, domain.ErrInvalidID
	}

	// 2. Fetch corporate treasury
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

	// 4. Resolve step size using shared logic
	stepSize, err := domain.ResolveStepSize(treasury.TotalCapital, input.StepSize)
	if err != nil {
		return nil, err
	}

	// 5. Construct canonical FinancialProblem
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

	// 6. Execute AI Decision Engine strategy synthesis
	start := time.Now()
	aiResp, aiErr := s.aiClient.GenerateStrategy(ctx, problem, input.Algorithm)
	durationMS := float64(time.Since(start).Microseconds()) / 1000.0

	// Handle AI failure: record failed run with sanitized error message and return 502
	if aiErr != nil {
		log.Printf("[ERROR] Upstream AI decision engine execution failed: %v", aiErr)
		sanitizedErrMsg := "AI engine unavailable"
		failedRun, _ := s.orchRepo.Create(ctx, domain.OrchestrationRun{
			TreasuryID:         treasury.ID,
			Status:             domain.OrchestrationStatusFailed,
			AIStatus:           "failed",
			OptimizationStatus: "not_available",
			ErrorMessage:       &sanitizedErrMsg,
			DurationMS:         durationMS,
		})

		if s.auditSvc != nil {
			var runID *string
			if failedRun != nil {
				runID = &failedRun.ID
			}
			s.auditSvc.Record(ctx, domain.ActionOrchestrationFailed, "orchestration_run", runID, map[string]any{
				"treasury_id": treasury.ID,
				"algorithm":   input.Algorithm,
				"error":       sanitizedErrMsg,
			})
		}

		return nil, domain.ErrUpstreamUnavailable
	}

	// 7. Optional algorithmic search comparison
	var comparisonResult json.RawMessage
	if input.IncludeComparison {
		compRaw, compErr := s.aiClient.CompareSearches(ctx, problem)
		if compErr == nil {
			comparisonResult = compRaw
		}
	}

	// 8. Persist strategy log
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
		ExecutionTimeMS:      durationMS,
	}

	savedLog, err := s.strategyRepo.CreateLog(ctx, logRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to persist strategy execution log: %w", err)
	}

	// 9. Build orchestration response with optimization not_available status
	optStatus := domain.OptimizationStatusResponse{
		Status: "not_available",
		Reason: "optimization engine not yet integrated",
	}

	response := domain.OrchestrateAllocationResponse{
		Status: domain.OrchestrationStatusCompleted,
		AI: domain.StrategyExecutionResponse{
			StrategyExecutionLog: *savedLog,
			Explanation:          aiResp.Strategy.Explanation,
		},
		Comparison:   comparisonResult,
		Optimization: optStatus,
	}

	respBytes, _ := json.Marshal(response)

	// 10. Persist successful orchestration run
	savedRun, err := s.orchRepo.Create(ctx, domain.OrchestrationRun{
		TreasuryID:         treasury.ID,
		Status:             domain.OrchestrationStatusCompleted,
		AIStatus:           "completed",
		OptimizationStatus: "not_available",
		StrategyLogID:      &savedLog.ID,
		ResponseJSON:       respBytes,
		DurationMS:         durationMS,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to persist orchestration run: %w", err)
	}

	response.RunID = savedRun.ID

	// 11. Best-effort audit log
	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionOrchestrationCompleted, "orchestration_run", &savedRun.ID, map[string]any{
			"treasury_id":     treasury.ID,
			"algorithm":       input.Algorithm,
			"strategy_log_id": savedLog.ID,
			"duration_ms":     durationMS,
		})
	}

	return &response, nil
}

// GetRunByID retrieves an orchestration run record by UUID.
func (s *OrchestrationService) GetRunByID(ctx context.Context, id string) (*domain.OrchestrationRun, error) {
	if !uuidRegex.MatchString(id) {
		return nil, domain.ErrInvalidID
	}
	return s.orchRepo.GetByID(ctx, id)
}
