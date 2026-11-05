// Package service implements platform business use cases and coordinates domain workflows.
package service

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// RiskService orchestrates portfolio risk decomposition, Bayesian shock reasoning,
// Monte Carlo VaR simulation, and Alpha-Beta adversarial stress testing.
type RiskService struct {
	treasuryRepo repository.TreasuryRepository
	aiClient     clients.AIClient
	auditSvc     *AuditService
}

// NewRiskService constructs a RiskService instance.
func NewRiskService(
	treasuryRepo repository.TreasuryRepository,
	aiClient clients.AIClient,
	auditSvc ...*AuditService,
) *RiskService {
	var a *AuditService
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &RiskService{
		treasuryRepo: treasuryRepo,
		aiClient:     aiClient,
		auditSvc:     a,
	}
}

// buildFinancialStateAndProblem constructs canonical FinancialState and FinancialProblem entities
// from the corporate Treasury profile and provided investment assets/liabilities.
func (s *RiskService) buildFinancialStateAndProblem(
	treasury *domain.Treasury,
	assets []domain.Asset,
	liabilities []domain.Liability,
	constraints []domain.Constraint,
	allocationsInput map[string]float64,
	customStepSize *float64,
	objective string,
	marketState string,
) (*domain.FinancialState, *domain.FinancialProblem, error) {
	if len(assets) == 0 {
		return nil, nil, &domain.ValidationError{
			Details: []string{"at least one investment asset must be configured"},
		}
	}

	if liabilities == nil {
		liabilities = []domain.Liability{}
	}
	if constraints == nil {
		constraints = []domain.Constraint{}
	}

	// 1. Calculate and validate discrete search step size
	var stepSize float64
	if customStepSize == nil {
		stepSize = treasury.TotalCapital / 20.0
	} else {
		if *customStepSize <= 0 {
			return nil, nil, &domain.ValidationError{
				Details: []string{"step_size must be greater than 0"},
			}
		}
		stepSize = *customStepSize
	}

	if (treasury.TotalCapital / stepSize) > 200.0 {
		return nil, nil, &domain.ValidationError{
			Details: []string{"step_size too small for capital, max 200 steps"},
		}
	}

	// 2. Mandatory debt reserve calculation
	mandatoryDebtReserve := 0.0
	for _, l := range liabilities {
		if l.IsMandatoryReserve {
			mandatoryDebtReserve += l.Principal
		}
	}

	// 3. Map investment catalog and validate allocations
	assetsMap := make(map[string]domain.Asset, len(assets))
	for _, a := range assets {
		assetsMap[a.ID] = a
	}

	allocations := make(map[string]float64, len(assets))
	for _, a := range assets {
		allocations[a.ID] = 0.0
	}

	allocatedSum := 0.0
	for aID, amt := range allocationsInput {
		if amt < 0 {
			return nil, nil, &domain.ValidationError{
				Details: []string{fmt.Sprintf("allocation for %s cannot be negative", aID)},
			}
		}
		if _, exists := assetsMap[aID]; !exists && amt > 0 {
			return nil, nil, &domain.ValidationError{
				Details: []string{fmt.Sprintf("allocated asset %s does not exist in investment catalog", aID)},
			}
		}
		allocations[aID] = amt
		allocatedSum += amt
	}

	if allocatedSum+mandatoryDebtReserve > treasury.TotalCapital+1e-4 {
		return nil, nil, &domain.ValidationError{
			Details: []string{"allocated capital and mandatory debt reserve exceed total treasury capital"},
		}
	}

	cash := treasury.TotalCapital - allocatedSum - mandatoryDebtReserve
	if cash < 0 {
		cash = 0
	}

	// 4. Calculate analytical metrics on the portfolio state
	cashReturn := 0.03
	weightedReturnSum := (cash + mandatoryDebtReserve) * cashReturn
	weightedLiqSum := (cash + mandatoryDebtReserve) * 1.0
	weightedRiskSum := (cash + mandatoryDebtReserve) * 0.01

	for aID, amt := range allocations {
		if a, ok := assetsMap[aID]; ok && amt > 0 {
			netReturn := a.ExpectedReturn - a.Cost
			weightedReturnSum += amt * netReturn
			weightedLiqSum += amt * a.Liquidity
			weightedRiskSum += amt * a.Risk
		}
	}

	expectedReturn := weightedReturnSum / treasury.TotalCapital
	liquidityRatio := weightedLiqSum / treasury.TotalCapital
	if liquidityRatio > 1.0 {
		liquidityRatio = 1.0
	} else if liquidityRatio < 0.0 {
		liquidityRatio = 0.0
	}

	baseRisk := weightedRiskSum / treasury.TotalCapital

	// Herfindahl-Hirschman Index (HHI)
	hhi := math.Pow((cash+mandatoryDebtReserve)/treasury.TotalCapital, 2)
	for _, amt := range allocations {
		if amt > 0 {
			hhi += math.Pow(amt/treasury.TotalCapital, 2)
		}
	}

	// Liquidity deficit penalty
	liqDeficit := 0.0
	if treasury.MinLiquidityRatio > liquidityRatio {
		liqDeficit = treasury.MinLiquidityRatio - liquidityRatio
	}
	minLiqFloor := treasury.MinLiquidityRatio
	if minLiqFloor < 0.01 {
		minLiqFloor = 0.01
	}
	liqPenalty := math.Pow(liqDeficit/minLiqFloor, 2)

	compositeRisk := 0.65*baseRisk + 0.15*(hhi*0.5) + 0.20*liqPenalty
	if compositeRisk > 1.0 {
		compositeRisk = 1.0
	} else if compositeRisk < 0.0 {
		compositeRisk = 0.0
	}

	st := &domain.FinancialState{
		TotalCapital:    treasury.TotalCapital,
		Cash:            cash,
		Allocations:     allocations,
		Liabilities:     liabilities,
		ReservedForDebt: mandatoryDebtReserve,
		LiquidityRatio:  liquidityRatio,
		RiskScore:       compositeRisk,
		ExpectedReturn:  expectedReturn,
		MarketState:     marketState,
	}

	prob := &domain.FinancialProblem{
		Capital:          treasury.TotalCapital,
		StepSize:         stepSize,
		Assets:           assets,
		Liabilities:      liabilities,
		Constraints:      constraints,
		RiskTolerance:    treasury.RiskTolerance,
		MinimumLiquidity: treasury.MinLiquidityRatio,
		Objective:        objective,
		MarketState:      marketState,
	}
	prob.ApplyDefaults()

	if errs := prob.Validate(); len(errs) > 0 {
		return nil, nil, &domain.ValidationError{Details: errs}
	}

	return st, prob, nil
}

// AnalyzeRisk performs multi-factor risk decomposition, Bayesian shock reasoning, and Monte Carlo VaR simulation.
func (s *RiskService) AnalyzeRisk(
	ctx context.Context,
	treasuryID string,
	input domain.AnalyzeTreasuryRiskInput,
) (*domain.AIRiskAnalyzeResponse, error) {
	if !uuidRegex.MatchString(treasuryID) {
		return nil, domain.ErrInvalidID
	}

	treasury, err := s.treasuryRepo.GetByID(ctx, treasuryID)
	if err != nil {
		return nil, err
	}

	// Enforce resource-level authorization:
	// Admin has unrestricted platform-wide access to all corporate treasuries.
	// Analyst can access only a Treasury they own or are explicitly assigned to.
	// Legacy / unassigned treasuries (OwnerID == nil) return ErrForbidden to Analysts until explicitly assigned.
	callerRole := domain.RoleFromContext(ctx)
	callerID := domain.UserIDFromContext(ctx)
	if callerRole != domain.RoleAdmin {
		if treasury.OwnerID == nil || *treasury.OwnerID == "" || callerID == "" || *treasury.OwnerID != callerID {
			return nil, domain.ErrForbidden
		}
	}

	st, prob, err := s.buildFinancialStateAndProblem(
		treasury,
		input.Assets,
		input.Liabilities,
		input.Constraints,
		input.Allocations,
		input.StepSize,
		input.Objective,
		input.MarketState,
	)
	if err != nil {
		return nil, err
	}

	// Python default handling: run_monte_carlo = true, num_scenarios = 1000
	runMonteCarlo := true
	if input.RunMonteCarlo != nil {
		runMonteCarlo = *input.RunMonteCarlo
	}

	numScenarios := 1000
	if input.NumScenarios != nil && *input.NumScenarios > 0 {
		numScenarios = *input.NumScenarios
	}

	aiReq := domain.AIRiskAnalyzeRequest{
		State:         *st,
		Problem:       *prob,
		Evidence:      input.Evidence,
		RunMonteCarlo: runMonteCarlo,
		NumScenarios:  numScenarios,
	}

	aiResp, err := s.aiClient.AnalyzeRisk(ctx, aiReq)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionRiskAnalyzed, "treasury", &treasury.ID, map[string]any{
			"treasury_id":       treasury.ID,
			"risk_tier":         aiResp.RiskBreakdown.RiskTier,
			"composite_score":   aiResp.RiskBreakdown.CompositeRiskScore,
			"stress_multiplier": aiResp.BayesianInference.StressMultiplier,
		})
	}

	return aiResp, nil
}

// AdversarialStressTest executes Alpha-Beta adversarial game search against worst-case adversary
// and evaluates deterministic impact across all 4 standard macroeconomic shock scenarios.
func (s *RiskService) AdversarialStressTest(
	ctx context.Context,
	treasuryID string,
	input domain.AdversarialStressTestInput,
) (*domain.TreasuryStressTestResponse, error) {
	if !uuidRegex.MatchString(treasuryID) {
		return nil, domain.ErrInvalidID
	}

	treasury, err := s.treasuryRepo.GetByID(ctx, treasuryID)
	if err != nil {
		return nil, err
	}

	// Enforce resource-level authorization:
	// Admin has unrestricted platform-wide access to all corporate treasuries.
	// Analyst can access only a Treasury they own or are explicitly assigned to.
	// Legacy / unassigned treasuries (OwnerID == nil) return ErrForbidden to Analysts until explicitly assigned.
	callerRole := domain.RoleFromContext(ctx)
	callerID := domain.UserIDFromContext(ctx)
	if callerRole != domain.RoleAdmin {
		if treasury.OwnerID == nil || *treasury.OwnerID == "" || callerID == "" || *treasury.OwnerID != callerID {
			return nil, domain.ErrForbidden
		}
	}

	st, prob, err := s.buildFinancialStateAndProblem(
		treasury,
		input.Assets,
		input.Liabilities,
		input.Constraints,
		input.Allocations,
		input.StepSize,
		input.Objective,
		input.MarketState,
	)
	if err != nil {
		return nil, err
	}

	aiResp, err := s.aiClient.AdversarialStressTest(ctx, *prob)
	if err != nil {
		return nil, err
	}

	// Deterministic scenario evaluations for all 4 standard shocks
	scenarioEvals := make([]domain.ScenarioEvaluation, len(domain.StandardMarketShocks))
	for i, shock := range domain.StandardMarketShocks {
		scenarioEvals[i] = evaluateShockScenario(st, prob.RiskTolerance, prob.MinimumLiquidity, shock)
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionStressTested, "treasury", &treasury.ID, map[string]any{
			"treasury_id":        treasury.ID,
			"worst_market_shock": aiResp.WorstMarketShock,
			"worst_case_payoff":  aiResp.WorstCasePayoff,
			"nodes_evaluated":    aiResp.NodesEvaluated,
		})
	}

	return &domain.TreasuryStressTestResponse{
		AdversarialResult: *aiResp,
		Scenarios:         scenarioEvals,
	}, nil
}

// evaluateShockScenario evaluates portfolio return and liquidity under discrete macroeconomic shocks.
func evaluateShockScenario(
	st *domain.FinancialState,
	riskTolerance float64,
	minLiquidity float64,
	shock domain.MarketShockScenario,
) domain.ScenarioEvaluation {
	totalCap := st.TotalCapital
	if totalCap <= 0 {
		totalCap = 1.0
	}

	equityAlloc := 0.0
	corpBondAlloc := 0.0
	for aID, amt := range st.Allocations {
		aLower := strings.ToLower(aID)
		if strings.Contains(aLower, "equity") {
			equityAlloc += amt
		} else if strings.Contains(aLower, "corp") || strings.Contains(aLower, "paper") || strings.Contains(aLower, "credit") {
			corpBondAlloc += amt
		}
	}

	shockReturnLoss := (equityAlloc*math.Abs(shock.EquityDrawdown) + corpBondAlloc*math.Abs(shock.CreditSpreadSpike)) / totalCap
	stressedReturn := st.ExpectedReturn - shockReturnLoss

	stressedLiq := st.LiquidityRatio - math.Abs(shock.LiquidityFreeze)
	if stressedLiq < 0 {
		stressedLiq = 0
	}

	stressedRisk := st.RiskScore * 1.4
	if stressedRisk > 1.0 {
		stressedRisk = 1.0
	}

	riskPenalty := (1.0 - riskTolerance) * 1.5 * math.Pow(stressedRisk, 2)
	payoff := stressedReturn - riskPenalty + 0.20*stressedLiq

	isCompliant := stressedLiq >= minLiquidity

	return domain.ScenarioEvaluation{
		Scenario:          shock,
		StressedReturn:    stressedReturn,
		StressedLiquidity: stressedLiq,
		StressedRisk:      stressedRisk,
		EstimatedPayoff:   payoff,
		IsCompliant:       isCompliant,
	}
}
