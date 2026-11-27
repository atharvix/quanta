package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// CreateLiquidityPolicyInput defines inputs for creating an org-scoped liquidity policy.
type CreateLiquidityPolicyInput struct {
	OrganizationID        uuid.UUID
	TreasuryEntityID      uuid.UUID
	MinOperatingLiquidity money.Amount
	MinEmergencyReserve   money.Amount
	IsActive              bool
	ActorID               uuid.UUID
	ActorRole             string
	CorrelationID         string
}

// UpdateLiquidityPolicyInput defines inputs for updating an existing liquidity policy.
type UpdateLiquidityPolicyInput struct {
	OrganizationID        uuid.UUID
	PolicyID              uuid.UUID
	MinOperatingLiquidity money.Amount
	MinEmergencyReserve   money.Amount
	IsActive              bool
	ActorID               uuid.UUID
	ActorRole             string
	CorrelationID         string
}

// PolicyValidationOutcome reports whether a proposed allocation respects active liquidity floors.
type PolicyValidationOutcome struct {
	Valid              bool
	PolicyID           uuid.UUID
	PolicyVersion      int64
	TotalAvailable     money.Amount
	ProposedAllocation money.Amount
	RequiredFloor      money.Amount
	PostAllocationCash money.Amount
	InvestableCapacity money.Amount
	Reason             string
}

// PolicyService manages liquidity policies and decimal-safe allocation floor validation.
type PolicyService struct {
	treasuryRepo repository.TreasuryRepository
	policyRepo   repository.LiquidityPolicyRepository
	auditRepo    repository.AuditRepository
}

// NewPolicyService constructs a PolicyService.
func NewPolicyService(
	treasuryRepo repository.TreasuryRepository,
	policyRepo repository.LiquidityPolicyRepository,
	auditRepo repository.AuditRepository,
) *PolicyService {
	return &PolicyService{
		treasuryRepo: treasuryRepo,
		policyRepo:   policyRepo,
		auditRepo:    auditRepo,
	}
}

// CreatePolicy creates a versioned liquidity policy for a treasury entity.
func (s *PolicyService) CreatePolicy(ctx context.Context, in CreateLiquidityPolicyInput) (domain.LiquidityPolicy, error) {
	if in.MinOperatingLiquidity.IsNegative() || in.MinEmergencyReserve.IsNegative() {
		return domain.LiquidityPolicy{}, domain.ErrInvalidConfiguration
	}
	if _, err := s.treasuryRepo.GetTreasuryEntity(ctx, in.OrganizationID, in.TreasuryEntityID); err != nil {
		return domain.LiquidityPolicy{}, err
	}

	existing, err := s.policyRepo.ListPoliciesByEntity(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		return domain.LiquidityPolicy{}, err
	}
	var maxVersion int64
	for _, p := range existing {
		if p.Version > maxVersion {
			maxVersion = p.Version
		}
	}

	policy := domain.LiquidityPolicy{
		ID:                    uuid.New(),
		OrganizationID:        in.OrganizationID,
		TreasuryEntityID:      in.TreasuryEntityID,
		Version:               maxVersion + 1,
		MinOperatingLiquidity: in.MinOperatingLiquidity,
		MinEmergencyReserve:   in.MinEmergencyReserve,
		IsActive:              in.IsActive,
		CreatedBy:             in.ActorID,
	}
	if err := s.policyRepo.CreatePolicy(ctx, policy); err != nil {
		return domain.LiquidityPolicy{}, err
	}

	s.auditPolicyEvent(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "LIQUIDITY_POLICY_CREATED", policy, in.CorrelationID)
	return policy, nil
}

// ListPolicies lists all liquidity policies for an organization-scoped treasury entity.
func (s *PolicyService) ListPolicies(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.LiquidityPolicy, error) {
	return s.policyRepo.ListPoliciesByEntity(ctx, orgID, entityID)
}

// UpdatePolicy updates an existing liquidity policy after validating non-negative reserve floors.
func (s *PolicyService) UpdatePolicy(ctx context.Context, in UpdateLiquidityPolicyInput) (domain.LiquidityPolicy, error) {
	if in.MinOperatingLiquidity.IsNegative() || in.MinEmergencyReserve.IsNegative() {
		return domain.LiquidityPolicy{}, domain.ErrInvalidConfiguration
	}
	existing, err := s.policyRepo.GetPolicyByID(ctx, in.OrganizationID, in.PolicyID)
	if err != nil {
		return domain.LiquidityPolicy{}, err
	}

	existing.MinOperatingLiquidity = in.MinOperatingLiquidity
	existing.MinEmergencyReserve = in.MinEmergencyReserve
	existing.IsActive = in.IsActive

	updated, err := s.policyRepo.UpdatePolicy(ctx, existing)
	if err != nil {
		return domain.LiquidityPolicy{}, err
	}

	s.auditPolicyEvent(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "LIQUIDITY_POLICY_UPDATED", updated, in.CorrelationID)
	return updated, nil
}

// ValidateAllocation checks whether proposedAllocation keeps remaining cash >= MinOperatingLiquidity + MinEmergencyReserve.
func (s *PolicyService) ValidateAllocation(
	ctx context.Context,
	orgID, entityID uuid.UUID,
	totalAvailable, proposedAllocation money.Amount,
) (PolicyValidationOutcome, error) {
	if totalAvailable.IsNegative() || proposedAllocation.IsNegative() {
		return PolicyValidationOutcome{}, domain.ErrInvalidAmount
	}

	policy, err := s.policyRepo.GetActivePolicy(ctx, orgID, entityID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return PolicyValidationOutcome{}, domain.ErrInvalidConfiguration
		}
		return PolicyValidationOutcome{}, err
	}

	requiredFloor := policy.TotalReserveFloor()
	investable := totalAvailable.Sub(requiredFloor)
	if investable.IsNegative() {
		investable = money.Zero()
	}
	postCash := totalAvailable.Sub(proposedAllocation)

	outcome := PolicyValidationOutcome{
		Valid:              true,
		PolicyID:           policy.ID,
		PolicyVersion:      policy.Version,
		TotalAvailable:     totalAvailable,
		ProposedAllocation: proposedAllocation,
		RequiredFloor:      requiredFloor,
		PostAllocationCash: postCash,
		InvestableCapacity: investable,
	}

	if postCash.IsNegative() || postCash.Cmp(requiredFloor) < 0 {
		outcome.Valid = false
		outcome.Reason = "proposed allocation breaches active operating and emergency reserve floors"
		return outcome, domain.ErrPolicyViolation
	}
	return outcome, nil
}

func (s *PolicyService) auditPolicyEvent(
	ctx context.Context,
	orgID, actorID uuid.UUID,
	actorRole, action string,
	policy domain.LiquidityPolicy,
	correlationID string,
) {
	if s.auditRepo == nil {
		return
	}
	meta, _ := json.Marshal(map[string]any{
		"treasury_entity_id":      policy.TreasuryEntityID.String(),
		"version":                 policy.Version,
		"min_operating_liquidity": policy.MinOperatingLiquidity.String(),
		"min_emergency_reserve":   policy.MinEmergencyReserve.String(),
		"is_active":               policy.IsActive,
	})
	_ = s.auditRepo.AppendAuditEvent(ctx, domain.AuditEvent{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		ActorID:        &actorID,
		ActorRole:      actorRole,
		Action:         action,
		EntityType:     "liquidity_policy",
		EntityID:       &policy.ID,
		CorrelationID:  correlationID,
		Metadata:       meta,
	})
}
