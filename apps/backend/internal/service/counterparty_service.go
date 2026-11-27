package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// CreateCounterpartyInput defines parameters to register a treasury counterparty and exposure cap.
type CreateCounterpartyInput struct {
	OrganizationID   uuid.UUID
	TreasuryEntityID uuid.UUID
	Name             string
	CurrencyCode     string
	MaxExposureLimit money.Amount
	CurrentExposure  money.Amount
	ActorID          uuid.UUID
	ActorRole        string
	CorrelationID    string
}

// UpdateCounterpartyInput defines parameters to update counterparty name or limits.
type UpdateCounterpartyInput struct {
	OrganizationID   uuid.UUID
	CounterpartyID   uuid.UUID
	Name             string
	MaxExposureLimit money.Amount
	CurrentExposure  money.Amount
	ExpectedVersion  int64
	ActorID          uuid.UUID
	ActorRole        string
	CorrelationID    string
}

// ExposureValidationResult reports whether additional exposure fits within counterparty limits.
type ExposureValidationResult struct {
	Allowed           bool
	CounterpartyID    uuid.UUID
	CurrencyCode      string
	CurrentExposure   money.Amount
	ProposedIncrement money.Amount
	ProjectedExposure money.Amount
	MaxExposureLimit  money.Amount
}

// CounterpartyService manages counterparty limits and currency-isolated exposure checks.
type CounterpartyService struct {
	treasuryRepo     repository.TreasuryRepository
	counterpartyRepo repository.CounterpartyRepository
	auditRepo        repository.AuditRepository
}

// NewCounterpartyService constructs a CounterpartyService.
func NewCounterpartyService(
	treasuryRepo repository.TreasuryRepository,
	counterpartyRepo repository.CounterpartyRepository,
	auditRepo repository.AuditRepository,
) *CounterpartyService {
	return &CounterpartyService{
		treasuryRepo:     treasuryRepo,
		counterpartyRepo: counterpartyRepo,
		auditRepo:        auditRepo,
	}
}

// CreateCounterparty creates a counterparty enforcing currency isolation with TreasuryEntity.BaseCurrency.
func (s *CounterpartyService) CreateCounterparty(ctx context.Context, in CreateCounterpartyInput) (domain.Counterparty, error) {
	if strings.TrimSpace(in.Name) == "" || in.MaxExposureLimit.IsNegative() || in.CurrentExposure.IsNegative() {
		return domain.Counterparty{}, domain.ErrInvalidConfiguration
	}
	if in.CurrentExposure.Cmp(in.MaxExposureLimit) > 0 {
		return domain.Counterparty{}, domain.ErrInvalidConfiguration
	}

	entity, err := s.treasuryRepo.GetTreasuryEntity(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		return domain.Counterparty{}, err
	}

	normalizedCurrency := strings.ToUpper(strings.TrimSpace(in.CurrencyCode))
	if normalizedCurrency == "" || normalizedCurrency != strings.ToUpper(strings.TrimSpace(entity.BaseCurrency)) {
		return domain.Counterparty{}, domain.ErrCurrencyMismatch
	}

	cp := domain.Counterparty{
		ID:               uuid.New(),
		OrganizationID:   in.OrganizationID,
		TreasuryEntityID: in.TreasuryEntityID,
		Name:             strings.TrimSpace(in.Name),
		CurrencyCode:     entity.BaseCurrency,
		MaxExposureLimit: in.MaxExposureLimit,
		CurrentExposure:  in.CurrentExposure,
		Version:          1,
	}
	if err := s.counterpartyRepo.CreateCounterparty(ctx, cp); err != nil {
		return domain.Counterparty{}, err
	}

	s.auditCounterparty(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "COUNTERPARTY_CREATED", cp, in.CorrelationID)
	return cp, nil
}

// ListCounterparties lists counterparties for an organization-scoped treasury entity.
func (s *CounterpartyService) ListCounterparties(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.Counterparty, error) {
	return s.counterpartyRepo.ListCounterpartiesByEntity(ctx, orgID, entityID)
}

// UpdateCounterparty updates counterparty limits with optimistic concurrency verification.
func (s *CounterpartyService) UpdateCounterparty(ctx context.Context, in UpdateCounterpartyInput) (domain.Counterparty, error) {
	if strings.TrimSpace(in.Name) == "" || in.MaxExposureLimit.IsNegative() || in.CurrentExposure.IsNegative() {
		return domain.Counterparty{}, domain.ErrInvalidConfiguration
	}
	if in.CurrentExposure.Cmp(in.MaxExposureLimit) > 0 {
		return domain.Counterparty{}, domain.ErrInvalidConfiguration
	}

	existing, err := s.counterpartyRepo.GetCounterparty(ctx, in.OrganizationID, in.CounterpartyID)
	if err != nil {
		return domain.Counterparty{}, err
	}

	existing.Name = strings.TrimSpace(in.Name)
	existing.MaxExposureLimit = in.MaxExposureLimit
	existing.CurrentExposure = in.CurrentExposure

	updated, err := s.counterpartyRepo.UpdateCounterparty(ctx, existing, in.ExpectedVersion)
	if err != nil {
		return domain.Counterparty{}, err
	}

	s.auditCounterparty(ctx, in.OrganizationID, in.ActorID, in.ActorRole, "COUNTERPARTY_UPDATED", updated, in.CorrelationID)
	return updated, nil
}

// ValidateExposure checks whether adding proposedIncrement in currencyCode stays within max_exposure_limit.
// Rejects unlike currencies to preserve strict currency isolation.
func (s *CounterpartyService) ValidateExposure(
	ctx context.Context,
	orgID, counterpartyID uuid.UUID,
	currencyCode string,
	proposedIncrement money.Amount,
) (ExposureValidationResult, error) {
	if proposedIncrement.IsNegative() {
		return ExposureValidationResult{}, domain.ErrInvalidAmount
	}

	cp, err := s.counterpartyRepo.GetCounterparty(ctx, orgID, counterpartyID)
	if err != nil {
		return ExposureValidationResult{}, err
	}

	if strings.ToUpper(strings.TrimSpace(currencyCode)) != strings.ToUpper(strings.TrimSpace(cp.CurrencyCode)) {
		return ExposureValidationResult{}, domain.ErrCurrencyMismatch
	}

	projected := cp.CurrentExposure.Add(proposedIncrement)
	result := ExposureValidationResult{
		Allowed:           projected.Cmp(cp.MaxExposureLimit) <= 0,
		CounterpartyID:    cp.ID,
		CurrencyCode:      cp.CurrencyCode,
		CurrentExposure:   cp.CurrentExposure,
		ProposedIncrement: proposedIncrement,
		ProjectedExposure: projected,
		MaxExposureLimit:  cp.MaxExposureLimit,
	}
	if !result.Allowed {
		return result, domain.ErrCounterpartyLimitExceeded
	}
	return result, nil
}

func (s *CounterpartyService) auditCounterparty(
	ctx context.Context,
	orgID, actorID uuid.UUID,
	actorRole, action string,
	cp domain.Counterparty,
	correlationID string,
) {
	if s.auditRepo == nil {
		return
	}
	meta, _ := json.Marshal(map[string]any{
		"name":               cp.Name,
		"currency_code":      cp.CurrencyCode,
		"max_exposure_limit": cp.MaxExposureLimit.String(),
		"current_exposure":   cp.CurrentExposure.String(),
		"version":            cp.Version,
	})
	_ = s.auditRepo.AppendAuditEvent(ctx, domain.AuditEvent{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		ActorID:        &actorID,
		ActorRole:      actorRole,
		Action:         action,
		EntityType:     "counterparty",
		EntityID:       &cp.ID,
		CorrelationID:  correlationID,
		Metadata:       meta,
	})
}
