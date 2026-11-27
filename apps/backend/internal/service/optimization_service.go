package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/atharvix/quanta/apps/backend/internal/verifier"
)

type OptimizeCandidateRequest struct {
	ID             string
	ExpectedReturn string
	MinAllocation  money.Amount
	MaxAllocation  money.Amount
	CounterpartyID *uuid.UUID
	LotSize        *money.Amount
}

type RunOptimizationInput struct {
	OrganizationID    uuid.UUID
	TreasuryEntityID  uuid.UUID
	ActorID           uuid.UUID
	ActorRole         string
	CorrelationID     string
	InvestableCapital money.Amount
	SolverMode        string
	Candidates        []OptimizeCandidateRequest
	AllowUnallocated  bool
}

type RunOptimizationOutcome struct {
	Run          domain.OptimizationRun
	Verification verifier.VerificationResult
}

type OptimizationService struct {
	treasuryRepo repository.TreasuryRepository
	accountRepo  repository.AccountRepository
	policyRepo   repository.LiquidityPolicyRepository
	cpRepo       repository.CounterpartyRepository
	runRepo      repository.OptimizationRunRepository
	auditRepo    repository.AuditRepository
	intelClient  intelligence.Client
}

func NewOptimizationService(
	treasuryRepo repository.TreasuryRepository,
	accountRepo repository.AccountRepository,
	policyRepo repository.LiquidityPolicyRepository,
	cpRepo repository.CounterpartyRepository,
	runRepo repository.OptimizationRunRepository,
	auditRepo repository.AuditRepository,
	intelClient intelligence.Client,
) *OptimizationService {
	return &OptimizationService{
		treasuryRepo: treasuryRepo,
		accountRepo:  accountRepo,
		policyRepo:   policyRepo,
		cpRepo:       cpRepo,
		runRepo:      runRepo,
		auditRepo:    auditRepo,
		intelClient:  intelClient,
	}
}

func (s *OptimizationService) Forecast(
	ctx context.Context,
	orgID, entityID, actorID uuid.UUID,
	actorRole string,
	req intelligence.ForecastRequest,
) (intelligence.ForecastResponse, error) {
	entity, err := s.treasuryRepo.GetTreasuryEntity(ctx, orgID, entityID)
	if err != nil {
		return intelligence.ForecastResponse{}, err
	}
	req.TreasuryID = entity.ID.String()
	req.BaseCurrency = entity.BaseCurrency
	req.ContractVersion = intelligence.ContractVersion

	resp, err := s.intelClient.Forecast(ctx, req)
	if err != nil {
		return intelligence.ForecastResponse{}, err
	}
	s.auditAction(ctx, orgID, actorID, actorRole, "INTELLIGENCE_FORECAST_GENERATED", "treasury_entity", entityID, req.CorrelationID, map[string]any{
		"model_name":    resp.ModelName,
		"model_version": resp.ModelVersion,
	})
	return resp, nil
}

func (s *OptimizationService) EvaluateRisk(
	ctx context.Context,
	orgID, entityID, actorID uuid.UUID,
	actorRole string,
	req intelligence.RiskRequest,
) (intelligence.RiskResponse, error) {
	entity, err := s.treasuryRepo.GetTreasuryEntity(ctx, orgID, entityID)
	if err != nil {
		return intelligence.RiskResponse{}, err
	}
	req.TreasuryID = entity.ID.String()
	req.BaseCurrency = entity.BaseCurrency
	req.ContractVersion = intelligence.ContractVersion

	resp, err := s.intelClient.EvaluateRisk(ctx, req)
	if err != nil {
		return intelligence.RiskResponse{}, err
	}
	s.auditAction(ctx, orgID, actorID, actorRole, "INTELLIGENCE_RISK_EVALUATED", "treasury_entity", entityID, req.CorrelationID, map[string]any{
		"model_name": resp.ModelName,
		"seed":       resp.Seed,
	})
	return resp, nil
}

func (s *OptimizationService) RunOptimization(ctx context.Context, in RunOptimizationInput) (RunOptimizationOutcome, error) {
	if in.InvestableCapital.IsNegative() || len(in.Candidates) == 0 {
		return RunOptimizationOutcome{}, domain.ErrInvalidConfiguration
	}
	entity, err := s.treasuryRepo.GetTreasuryEntity(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		return RunOptimizationOutcome{}, err
	}
	accounts, err := s.accountRepo.ListAccountsByEntity(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		return RunOptimizationOutcome{}, err
	}
	policy, err := s.policyRepo.GetActivePolicy(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return RunOptimizationOutcome{}, domain.ErrInvalidConfiguration
		}
		return RunOptimizationOutcome{}, err
	}
	cps, err := s.cpRepo.ListCounterpartiesByEntity(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		return RunOptimizationOutcome{}, err
	}

	totalAvailable := money.Zero()
	accountVersions := make(map[uuid.UUID]int64, len(accounts))
	for _, acc := range accounts {
		totalAvailable = totalAvailable.Add(acc.AvailableBalance())
		accountVersions[acc.ID] = acc.Version
	}

	cpMap := make(map[uuid.UUID]domain.Counterparty, len(cps))
	cpInputs := make([]intelligence.CounterpartyLimitInput, 0, len(cps))
	for _, cp := range cps {
		cpMap[cp.ID] = cp
		cpInputs = append(cpInputs, intelligence.CounterpartyLimitInput{
			CounterpartyID:   cp.ID.String(),
			MaxExposureLimit: cp.MaxExposureLimit.String(),
			CurrentExposure:  cp.CurrentExposure.String(),
		})
	}

	candInputs := make([]intelligence.AllocationCandidateInput, 0, len(in.Candidates))
	verifyCands := make([]verifier.CandidateSpec, 0, len(in.Candidates))
	for _, c := range in.Candidates {
		var cpStr *string
		if c.CounterpartyID != nil {
			s := c.CounterpartyID.String()
			cpStr = &s
		}
		var lotStr *string
		if c.LotSize != nil {
			s := c.LotSize.String()
			lotStr = &s
		}
		candInputs = append(candInputs, intelligence.AllocationCandidateInput{
			ID:             c.ID,
			ExpectedReturn: c.ExpectedReturn,
			MinAllocation:  c.MinAllocation.String(),
			MaxAllocation:  c.MaxAllocation.String(),
			CounterpartyID: cpStr,
			LotSize:        lotStr,
		})
		verifyCands = append(verifyCands, verifier.CandidateSpec{
			ID:             c.ID,
			MinAllocation:  c.MinAllocation,
			MaxAllocation:  c.MaxAllocation,
			CounterpartyID: c.CounterpartyID,
		})
	}

	totalAvailStr := totalAvailable.String()
	pyReq := intelligence.OptimizeRequest{
		ContractVersion:         intelligence.ContractVersion,
		CorrelationID:           in.CorrelationID,
		TreasuryID:              entity.ID.String(),
		BaseCurrency:            entity.BaseCurrency,
		InvestableCapital:       in.InvestableCapital.String(),
		OperatingLiquidityFloor: policy.MinOperatingLiquidity.String(),
		EmergencyReserveFloor:   policy.MinEmergencyReserve.String(),
		TotalAvailableCash:      &totalAvailStr,
		Candidates:              candInputs,
		CounterpartyLimits:      cpInputs,
		SolverMode:              in.SolverMode,
		AllowUnallocatedCash:    in.AllowUnallocated,
		TimeoutMs:               10000,
	}

	problemJSON, _ := json.Marshal(map[string]any{
		"request":          pyReq,
		"account_versions": accountVersions,
		"policy_version":   policy.Version,
		"total_available":  totalAvailable.String(),
	})

	run := domain.OptimizationRun{
		ID:               uuid.New(),
		OrganizationID:   in.OrganizationID,
		TreasuryEntityID: in.TreasuryEntityID,
		RequestedBy:      in.ActorID,
		CorrelationID:    in.CorrelationID,
		SolverCategory:   "classical",
		ProblemSnapshot:  problemJSON,
		FXSnapshotIDs:    []byte("[]"),
	}

	pyResp, callErr := s.intelClient.Optimize(ctx, pyReq)
	if callErr != nil {
		run.Status = domain.OptStatusSolverFailed
		run.Feasible = false
		run.RawResponse, _ = json.Marshal(map[string]any{"error": callErr.Error()})
		_ = s.runRepo.CreateRun(ctx, run)
		s.auditAction(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "OPTIMIZATION_RUN_SOLVER_FAILED", "optimization_run", run.ID, in.CorrelationID, map[string]any{"error": callErr.Error()})
		return RunOptimizationOutcome{Run: run}, callErr
	}

	run.RuntimeMs = pyResp.RuntimeMs
	if pyResp.Status == "INFEASIBLE" {
		run.Status = domain.OptStatusInfeasible
		run.Feasible = false
		run.RawResponse, _ = json.Marshal(pyResp)
		if err := s.runRepo.CreateRun(ctx, run); err != nil {
			return RunOptimizationOutcome{}, err
		}
		s.auditAction(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "OPTIMIZATION_RUN_INFEASIBLE", "optimization_run", run.ID, in.CorrelationID, nil)
		return RunOptimizationOutcome{Run: run}, nil
	}
	if pyResp.Status != "COMPLETED" {
		run.Status = domain.OptStatusSolverFailed
		run.Feasible = false
		run.RawResponse, _ = json.Marshal(pyResp)
		if err := s.runRepo.CreateRun(ctx, run); err != nil {
			return RunOptimizationOutcome{}, err
		}
		return RunOptimizationOutcome{Run: run}, nil
	}

	// Independent authoritative Go-side verification
	ver := verifier.VerifyOptimization(verifier.VerifyInput{
		BaseCurrency:            entity.BaseCurrency,
		TotalAvailableCash:      totalAvailable,
		InvestableCapital:       in.InvestableCapital,
		OperatingLiquidityFloor: policy.MinOperatingLiquidity,
		EmergencyReserveFloor:   policy.MinEmergencyReserve,
		Candidates:              verifyCands,
		Counterparties:          cpMap,
		ExpectedAccountVersions: accountVersions,
		CurrentAccountVersions:  accountVersions,
		ExpectedPolicyVersion:   policy.Version,
		CurrentPolicyVersion:    policy.Version,
		Response:                pyResp,
	})

	rawPayload, _ := json.Marshal(map[string]any{
		"python_response": pyResp,
		"verification":    ver,
	})
	run.RawResponse = rawPayload

	if !ver.Passed {
		run.Status = domain.OptStatusVerificationFailed
		run.Feasible = false
		if err := s.runRepo.CreateRun(ctx, run); err != nil {
			return RunOptimizationOutcome{}, err
		}
		s.auditAction(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "OPTIMIZATION_VERIFICATION_FAILED", "optimization_run", run.ID, in.CorrelationID, map[string]any{
			"failures": ver.Failures,
		})
		return RunOptimizationOutcome{Run: run, Verification: ver}, nil
	}

	run.Status = domain.OptStatusCompleted
	run.Feasible = true
	if err := s.runRepo.CreateRun(ctx, run); err != nil {
		return RunOptimizationOutcome{}, err
	}
	s.auditAction(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "OPTIMIZATION_RUN_COMPLETED", "optimization_run", run.ID, in.CorrelationID, map[string]any{
		"solver_used": pyResp.SolverUsed,
	})
	return RunOptimizationOutcome{Run: run, Verification: ver}, nil
}

func (s *OptimizationService) GetRun(ctx context.Context, orgID, runID uuid.UUID) (domain.OptimizationRun, error) {
	return s.runRepo.GetRun(ctx, orgID, runID)
}

func (s *OptimizationService) auditAction(
	ctx context.Context,
	orgID, actorID uuid.UUID,
	actorRole, action, entityType string,
	entityID uuid.UUID,
	correlationID string,
	metadata map[string]any,
) {
	if s.auditRepo == nil {
		return
	}
	metaBytes, _ := json.Marshal(metadata)
	_ = s.auditRepo.AppendAuditEvent(ctx, domain.AuditEvent{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		ActorID:        &actorID,
		ActorRole:      actorRole,
		Action:         action,
		EntityType:     entityType,
		EntityID:       &entityID,
		CorrelationID:  correlationID,
		Metadata:       metaBytes,
	})
}
