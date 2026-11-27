package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/atharvix/quanta/apps/backend/internal/verifier"
)

type CreateProposalInput struct {
	OrganizationID       uuid.UUID
	OptimizationRunID    uuid.UUID
	SupersedesProposalID *uuid.UUID
	ActorID              uuid.UUID
	ActorRole            string
	CorrelationID        string
}

type SubmitProposalInput struct {
	OrganizationID uuid.UUID
	ProposalID     uuid.UUID
	ActorID        uuid.UUID
	ActorRole      string
	ExpiresIn      time.Duration
	CorrelationID  string
}

type DecideProposalInput struct {
	OrganizationID uuid.UUID
	ProposalID     uuid.UUID
	ApproverID     uuid.UUID
	ApproverRole   string
	Rationale      string
	CorrelationID  string
}

type ProposalService struct {
	txManager    repository.TxManager
	runRepo      repository.OptimizationRunRepository
	proposalRepo repository.ProposalRepository
	decisionRepo repository.ApprovalDecisionRepository
	treasuryRepo repository.TreasuryRepository
	accountRepo  repository.AccountRepository
	policyRepo   repository.LiquidityPolicyRepository
	cpRepo       repository.CounterpartyRepository
	auditRepo    repository.AuditRepository
	nowFn        func() time.Time
}

func NewProposalService(
	txManager repository.TxManager,
	runRepo repository.OptimizationRunRepository,
	proposalRepo repository.ProposalRepository,
	decisionRepo repository.ApprovalDecisionRepository,
	treasuryRepo repository.TreasuryRepository,
	accountRepo repository.AccountRepository,
	policyRepo repository.LiquidityPolicyRepository,
	cpRepo repository.CounterpartyRepository,
	auditRepo repository.AuditRepository,
) *ProposalService {
	return &ProposalService{
		txManager:    txManager,
		runRepo:      runRepo,
		proposalRepo: proposalRepo,
		decisionRepo: decisionRepo,
		treasuryRepo: treasuryRepo,
		accountRepo:  accountRepo,
		policyRepo:   policyRepo,
		cpRepo:       cpRepo,
		auditRepo:    auditRepo,
		nowFn:        time.Now,
	}
}

// CreateProposal creates a DRAFT AllocationProposal strictly from a COMPLETED OptimizationRun.
func (s *ProposalService) CreateProposal(ctx context.Context, in CreateProposalInput) (domain.AllocationProposal, error) {
	run, err := s.runRepo.GetRun(ctx, in.OrganizationID, in.OptimizationRunID)
	if err != nil {
		return domain.AllocationProposal{}, err
	}
	if run.Status != domain.OptStatusCompleted || !run.Feasible {
		return domain.AllocationProposal{}, domain.ErrProposalNotCompletedRun
	}

	if in.SupersedesProposalID != nil {
		prev, err := s.proposalRepo.GetProposal(ctx, in.OrganizationID, *in.SupersedesProposalID)
		if err != nil {
			return domain.AllocationProposal{}, err
		}
		if prev.Status != domain.ProposalStatusRejected && prev.Status != domain.ProposalStatusExpired {
			return domain.AllocationProposal{}, domain.ErrInvalidProposalTransition
		}
	}

	var problem struct {
		Request         intelligence.OptimizeRequest `json:"request"`
		AccountVersions map[uuid.UUID]int64          `json:"account_versions"`
		PolicyVersion   int64                        `json:"policy_version"`
	}
	if err := json.Unmarshal(run.ProblemSnapshot, &problem); err != nil {
		return domain.AllocationProposal{}, err
	}
	var raw struct {
		PythonResponse intelligence.OptimizeResponse `json:"python_response"`
		Verification   verifier.VerificationResult   `json:"verification"`
	}
	if err := json.Unmarshal(run.RawResponse, &raw); err != nil {
		return domain.AllocationProposal{}, err
	}

	invCap, err := money.FromString(problem.Request.InvestableCapital)
	if err != nil {
		return domain.AllocationProposal{}, err
	}

	allocBytes, _ := json.Marshal(raw.PythonResponse.Allocations)
	acctVerBytes, _ := json.Marshal(problem.AccountVersions)

	proposal := domain.AllocationProposal{
		ID:                      uuid.New(),
		OrganizationID:          in.OrganizationID,
		OptimizationRunID:       run.ID,
		TreasuryEntityID:        run.TreasuryEntityID,
		SupersedesProposalID:    in.SupersedesProposalID,
		Status:                  domain.ProposalStatusDraft,
		InvestableCapital:       invCap,
		TotalAllocated:          raw.Verification.TotalAllocated,
		ResidualCash:            raw.Verification.UnallocatedCash,
		VerifiedAllocations:     allocBytes,
		SnapshotAccountVersions: acctVerBytes,
		SnapshotPolicyVersion:   problem.PolicyVersion,
		Version:                 1,
	}
	if err := s.proposalRepo.CreateProposal(ctx, proposal); err != nil {
		return domain.AllocationProposal{}, err
	}

	if s.auditRepo != nil {
		meta, _ := json.Marshal(map[string]any{
			"optimization_run_id": run.ID.String(),
			"investable_capital":  proposal.InvestableCapital.String(),
		})
		_ = s.auditRepo.AppendAuditEvent(ctx, domain.AuditEvent{
			ID:             uuid.New(),
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorRole:      in.ActorRole,
			Action:         "PROPOSAL_CREATED",
			EntityType:     "allocation_proposal",
			EntityID:       &proposal.ID,
			CorrelationID:  in.CorrelationID,
			Metadata:       meta,
		})
	}
	return proposal, nil
}

// SubmitProposal transitions DRAFT -> PENDING_APPROVAL and sets expiry and submitter (maker).
func (s *ProposalService) SubmitProposal(ctx context.Context, in SubmitProposalInput) (domain.AllocationProposal, error) {
	if in.ExpiresIn <= 0 {
		in.ExpiresIn = 24 * time.Hour
	}

	var updated domain.AllocationProposal
	err := s.txManager.WithTx(ctx, func(ctx context.Context, repos repository.TxRepositories) error {
		p, err := repos.Proposals.GetProposalForUpdate(ctx, in.OrganizationID, in.ProposalID)
		if err != nil {
			return err
		}
		if p.Status != domain.ProposalStatusDraft {
			return domain.ErrInvalidProposalTransition
		}

		now := s.nowFn().UTC()
		subUnix := now.Unix()
		expUnix := now.Add(in.ExpiresIn).Unix()
		updated, err = repos.Proposals.UpdateProposalStatus(
			ctx, in.OrganizationID, p.ID, p.Version,
			domain.ProposalStatusPendingApproval,
			&in.ActorID, &subUnix, &expUnix,
		)
		if err != nil {
			return err
		}

		return repos.Audit.AppendAuditEvent(ctx, domain.AuditEvent{
			ID:             uuid.New(),
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorRole:      in.ActorRole,
			Action:         "PROPOSAL_SUBMITTED",
			EntityType:     "allocation_proposal",
			EntityID:       &p.ID,
			CorrelationID:  in.CorrelationID,
		})
	})
	return updated, err
}

// ApproveProposal enforces Maker-Checker separation, expiry, and live approval-time revalidation.
func (s *ProposalService) ApproveProposal(ctx context.Context, in DecideProposalInput) (domain.AllocationProposal, domain.ApprovalDecision, error) {
	var updated domain.AllocationProposal
	var decision domain.ApprovalDecision
	var expireErr error

	err := s.txManager.WithTx(ctx, func(ctx context.Context, repos repository.TxRepositories) error {
		p, err := repos.Proposals.GetProposalForUpdate(ctx, in.OrganizationID, in.ProposalID)
		if err != nil {
			return err
		}
		if p.Status != domain.ProposalStatusPendingApproval {
			return domain.ErrInvalidProposalTransition
		}

		now := s.nowFn().UTC()
		if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
			updated, err = repos.Proposals.UpdateProposalStatus(ctx, in.OrganizationID, p.ID, p.Version, domain.ProposalStatusExpired, nil, nil, nil)
			if err != nil {
				return err
			}
			_ = repos.Audit.AppendAuditEvent(ctx, domain.AuditEvent{
				ID:             uuid.New(),
				OrganizationID: &in.OrganizationID,
				ActorID:        &in.ApproverID,
				ActorRole:      in.ApproverRole,
				Action:         "PROPOSAL_EXPIRED",
				EntityType:     "allocation_proposal",
				EntityID:       &p.ID,
				CorrelationID:  in.CorrelationID,
			})
			expireErr = domain.ErrProposalExpired
			return nil
		}

		// Maker-Checker separation
		if p.SubmittedBy == nil {
			return domain.ErrMakerCheckerViolation
		}
		if err := auth.EnforceMakerChecker(*p.SubmittedBy, in.ApproverID); err != nil {
			return err
		}

		// Live revalidation against current authoritative accounts, policy, and counterparties
		run, err := s.runRepo.GetRun(ctx, in.OrganizationID, p.OptimizationRunID)
		if err != nil {
			return err
		}
		entity, err := s.treasuryRepo.GetTreasuryEntity(ctx, in.OrganizationID, p.TreasuryEntityID)
		if err != nil {
			return err
		}
		accounts, err := s.accountRepo.ListAccountsByEntity(ctx, in.OrganizationID, p.TreasuryEntityID)
		if err != nil {
			return err
		}
		policy, err := s.policyRepo.GetActivePolicy(ctx, in.OrganizationID, p.TreasuryEntityID)
		if err != nil {
			return err
		}
		cps, err := s.cpRepo.ListCounterpartiesByEntity(ctx, in.OrganizationID, p.TreasuryEntityID)
		if err != nil {
			return err
		}

		var snapAcctVersions map[uuid.UUID]int64
		_ = json.Unmarshal(p.SnapshotAccountVersions, &snapAcctVersions)
		curAcctVersions := make(map[uuid.UUID]int64, len(accounts))
		totalAvailable := money.Zero()
		for _, acc := range accounts {
			curAcctVersions[acc.ID] = acc.Version
			totalAvailable = totalAvailable.Add(acc.AvailableBalance())
		}

		var problem struct {
			Request intelligence.OptimizeRequest `json:"request"`
		}
		_ = json.Unmarshal(run.ProblemSnapshot, &problem)
		var raw struct {
			PythonResponse intelligence.OptimizeResponse `json:"python_response"`
		}
		_ = json.Unmarshal(run.RawResponse, &raw)

		cpMap := make(map[uuid.UUID]domain.Counterparty, len(cps))
		for _, cp := range cps {
			cpMap[cp.ID] = cp
		}
		verifyCands := make([]verifier.CandidateSpec, 0, len(problem.Request.Candidates))
		for _, c := range problem.Request.Candidates {
			minA, _ := money.FromString(c.MinAllocation)
			maxA, _ := money.FromString(c.MaxAllocation)
			var cpID *uuid.UUID
			if c.CounterpartyID != nil {
				if parsed, err := uuid.Parse(*c.CounterpartyID); err == nil {
					cpID = &parsed
				}
			}
			verifyCands = append(verifyCands, verifier.CandidateSpec{
				ID:             c.ID,
				MinAllocation:  minA,
				MaxAllocation:  maxA,
				CounterpartyID: cpID,
			})
		}

		recheck := verifier.VerifyOptimization(verifier.VerifyInput{
			BaseCurrency:            entity.BaseCurrency,
			TotalAvailableCash:      totalAvailable,
			InvestableCapital:       p.InvestableCapital,
			OperatingLiquidityFloor: policy.MinOperatingLiquidity,
			EmergencyReserveFloor:   policy.MinEmergencyReserve,
			Candidates:              verifyCands,
			Counterparties:          cpMap,
			ExpectedAccountVersions: snapAcctVersions,
			CurrentAccountVersions:  curAcctVersions,
			ExpectedPolicyVersion:   p.SnapshotPolicyVersion,
			CurrentPolicyVersion:    policy.Version,
			Response:                raw.PythonResponse,
		})
		if !recheck.Passed {
			return domain.ErrStaleProposalState
		}

		updated, err = repos.Proposals.UpdateProposalStatus(ctx, in.OrganizationID, p.ID, p.Version, domain.ProposalStatusApproved, nil, nil, nil)
		if err != nil {
			return err
		}

		liveSnap, _ := json.Marshal(map[string]any{
			"total_available":  totalAvailable.String(),
			"account_versions": curAcctVersions,
			"policy_version":   policy.Version,
		})
		decision = domain.ApprovalDecision{
			ID:                  uuid.New(),
			OrganizationID:      in.OrganizationID,
			ProposalID:          p.ID,
			ApproverID:          in.ApproverID,
			Decision:            domain.DecisionApprove,
			Rationale:           strings.TrimSpace(in.Rationale),
			LiveBalanceSnapshot: liveSnap,
		}
		if err := repos.Decisions.AppendDecision(ctx, decision); err != nil {
			return err
		}

		return repos.Audit.AppendAuditEvent(ctx, domain.AuditEvent{
			ID:             uuid.New(),
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ApproverID,
			ActorRole:      in.ApproverRole,
			Action:         "PROPOSAL_APPROVED",
			EntityType:     "allocation_proposal",
			EntityID:       &p.ID,
			CorrelationID:  in.CorrelationID,
		})
	})

	if err != nil {
		return domain.AllocationProposal{}, domain.ApprovalDecision{}, err
	}
	if expireErr != nil {
		return updated, domain.ApprovalDecision{}, expireErr
	}
	return updated, decision, nil
}

// RejectProposal transitions PENDING_APPROVAL -> REJECTED (terminal) with recorded rationale.
func (s *ProposalService) RejectProposal(ctx context.Context, in DecideProposalInput) (domain.AllocationProposal, domain.ApprovalDecision, error) {
	if strings.TrimSpace(in.Rationale) == "" {
		return domain.AllocationProposal{}, domain.ApprovalDecision{}, domain.ErrInvalidConfiguration
	}
	var updated domain.AllocationProposal
	var decision domain.ApprovalDecision

	err := s.txManager.WithTx(ctx, func(ctx context.Context, repos repository.TxRepositories) error {
		p, err := repos.Proposals.GetProposalForUpdate(ctx, in.OrganizationID, in.ProposalID)
		if err != nil {
			return err
		}
		if p.Status != domain.ProposalStatusPendingApproval {
			return domain.ErrInvalidProposalTransition
		}

		updated, err = repos.Proposals.UpdateProposalStatus(ctx, in.OrganizationID, p.ID, p.Version, domain.ProposalStatusRejected, nil, nil, nil)
		if err != nil {
			return err
		}

		liveSnap, _ := json.Marshal(map[string]any{"proposal_version": p.Version})
		decision = domain.ApprovalDecision{
			ID:                  uuid.New(),
			OrganizationID:      in.OrganizationID,
			ProposalID:          p.ID,
			ApproverID:          in.ApproverID,
			Decision:            domain.DecisionReject,
			Rationale:           strings.TrimSpace(in.Rationale),
			LiveBalanceSnapshot: liveSnap,
		}
		if err := repos.Decisions.AppendDecision(ctx, decision); err != nil {
			return err
		}

		return repos.Audit.AppendAuditEvent(ctx, domain.AuditEvent{
			ID:             uuid.New(),
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ApproverID,
			ActorRole:      in.ApproverRole,
			Action:         "PROPOSAL_REJECTED",
			EntityType:     "allocation_proposal",
			EntityID:       &p.ID,
			CorrelationID:  in.CorrelationID,
		})
	})
	return updated, decision, err
}

func (s *ProposalService) ListProposals(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.AllocationProposal, error) {
	return s.proposalRepo.ListProposalsByEntity(ctx, orgID, entityID)
}
