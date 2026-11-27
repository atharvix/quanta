// Package service provides business logic orchestration, validation, and domain rules.
package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// uuidRegex validates standard RFC 4122 canonical UUID format.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// TreasuryService defines business operations on corporate treasuries.
type TreasuryService struct {
	repo      repository.TreasuryRepository
	entryRepo repository.FinancialEntryRepository
	auditSvc  *AuditService
}

// NewTreasuryService constructs a new service instance wrapping the repository layer.
func NewTreasuryService(repo repository.TreasuryRepository, auditSvc ...*AuditService) *TreasuryService {
	var a *AuditService
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &TreasuryService{repo: repo, auditSvc: a}
}

// SetEntryRepository attaches the financial entry repository for custom assets and liabilities.
func (s *TreasuryService) SetEntryRepository(entryRepo repository.FinancialEntryRepository) {
	s.entryRepo = entryRepo
}

// List validates pagination arguments and queries treasuries.
// Limit defaults to 20, cannot exceed 100. Offset must be >= 0.
func (s *TreasuryService) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, fmt.Errorf("%w: limit must not exceed 100", domain.ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidPagination)
	}

	items, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	filtered := make([]domain.Treasury, 0, len(items))
	for _, t := range items {
		tCopy := t
		if domain.CheckTreasuryAccess(ctx, &tCopy) == nil {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

// GetByID validates the UUID format before querying the repository and enforcing resource-level authorization.
func (s *TreasuryService) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	if !uuidRegex.MatchString(id) {
		return nil, domain.ErrInvalidID
	}

	treasury, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := domain.CheckTreasuryAccess(ctx, treasury); err != nil {
		return nil, err
	}

	return treasury, nil
}

// Update validates the treasury ID, verifies resource authorization, validates patch input, and persists updates.
func (s *TreasuryService) Update(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error) {
	if _, err := s.GetByID(ctx, id); err != nil {
		return nil, err
	}

	if errs := input.Validate(); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	updated, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		changes := make(map[string]any)
		if input.Name != nil {
			changes["name"] = *input.Name
		}
		if input.BaseCurrency != nil {
			changes["base_currency"] = *input.BaseCurrency
		}
		if input.TotalCapital != nil {
			changes["total_capital"] = *input.TotalCapital
		}
		if input.RiskTolerance != nil {
			changes["risk_tolerance"] = *input.RiskTolerance
		}
		if input.MinLiquidityRatio != nil {
			changes["min_liquidity_ratio"] = *input.MinLiquidityRatio
		}
		s.auditSvc.Record(ctx, domain.ActionTreasuryUpdated, "treasury", &updated.ID, changes)
	}

	return updated, nil
}

// Create applies domain defaults, validates input invariants, and creates the treasury.
func (s *TreasuryService) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	input.ApplyDefaults()

	if errs := input.Validate(); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	// Associate treasury with authenticated creator if not explicitly set
	if input.OwnerID == nil {
		if uid := domain.UserIDFromContext(ctx); uid != "" {
			input.OwnerID = &uid
		}
	}

	saved, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionTreasuryCreated, "treasury", &saved.ID, map[string]any{
			"name":          saved.Name,
			"total_capital": saved.TotalCapital,
		})
	}

	// Initialize baseline asset and liability universe for the new corporate treasury
	if s.entryRepo != nil {
		defaultAssets := []domain.Asset{
			{ID: "gov_bond", Name: "Government Bonds", AssetType: "gov_bond", Cost: 0.001, ExpectedReturn: 0.07, Risk: 0.05, Liquidity: 0.95, Volatility: 0.03},
			{ID: "bluechip_equity", Name: "Bluechip Equity Index", AssetType: "equity", Cost: 0.002, ExpectedReturn: 0.12, Risk: 0.18, Liquidity: 0.85, Volatility: 0.15},
			{ID: "commercial_paper", Name: "Corporate Commercial Paper", AssetType: "corp_bond", Cost: 0.001, ExpectedReturn: 0.08, Risk: 0.08, Liquidity: 0.75, Volatility: 0.06},
		}
		_, _ = s.entryRepo.BulkSyncAssets(ctx, saved.ID, defaultAssets)

		defaultLiabilities := []domain.Liability{
			{ID: "senior_loan", Name: "Senior Revolving Facility", Principal: 15.0, InterestRate: 0.065, MaturityMonths: 12, Priority: 1, IsMandatoryReserve: true},
		}
		_, _ = s.entryRepo.BulkSyncLiabilities(ctx, saved.ID, defaultLiabilities)
	}

	return saved, nil
}

// ListAssets returns all custom investment assets for a treasury after verifying authorization.
func (s *TreasuryService) ListAssets(ctx context.Context, treasuryID string) ([]domain.Asset, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	// Enforce valid UUID format and treasury access control
	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	return s.entryRepo.ListAssets(ctx, treasuryID)
}

// GetAsset returns a single investment asset after verifying authorization.
func (s *TreasuryService) GetAsset(ctx context.Context, treasuryID, assetID string) (*domain.Asset, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	return s.entryRepo.GetAsset(ctx, treasuryID, assetID)
}

// CreateAsset validates and persists a new custom investment asset.
func (s *TreasuryService) CreateAsset(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	if errs := domain.ValidateAsset(asset); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	saved, err := s.entryRepo.CreateAsset(ctx, treasuryID, asset)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionAssetCreated, "asset", &saved.ID, map[string]any{
			"treasury_id": treasuryID,
			"name":        saved.Name,
			"asset_type":  saved.AssetType,
		})
	}

	return saved, nil
}

// UpdateAsset modifies an existing asset in the treasury.
func (s *TreasuryService) UpdateAsset(ctx context.Context, treasuryID, assetID string, asset domain.Asset) (*domain.Asset, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	asset.ID = assetID
	if errs := domain.ValidateAsset(asset); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	updated, err := s.entryRepo.UpdateAsset(ctx, treasuryID, assetID, asset)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionAssetUpdated, "asset", &updated.ID, map[string]any{
			"treasury_id": treasuryID,
			"name":        updated.Name,
			"asset_type":  updated.AssetType,
		})
	}

	return updated, nil
}

// DeleteAsset removes a candidate asset from the treasury portfolio.
func (s *TreasuryService) DeleteAsset(ctx context.Context, treasuryID, assetID string) error {
	if s.entryRepo == nil {
		return domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return err
	}

	if err := s.entryRepo.DeleteAsset(ctx, treasuryID, assetID); err != nil {
		return err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionAssetDeleted, "asset", &assetID, map[string]any{
			"treasury_id": treasuryID,
			"asset_id":    assetID,
		})
	}

	return nil
}

// BulkSyncAssets validates and replaces the entire asset universe for a treasury.
func (s *TreasuryService) BulkSyncAssets(ctx context.Context, treasuryID string, assets []domain.Asset) ([]domain.Asset, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	if len(assets) == 0 {
		return nil, &domain.ValidationError{Details: []string{"at least one investment asset must be configured"}}
	}

	seenIDs := make(map[string]bool)
	var allErrors []string
	for i, a := range assets {
		if seenIDs[a.ID] {
			allErrors = append(allErrors, fmt.Sprintf("duplicate asset id '%s' detected", a.ID))
		}
		seenIDs[a.ID] = true
		if errs := domain.ValidateAsset(a); len(errs) > 0 {
			for _, errStr := range errs {
				allErrors = append(allErrors, fmt.Sprintf("assets[%d]: %s", i, errStr))
			}
		}
	}

	if len(allErrors) > 0 {
		return nil, &domain.ValidationError{Details: allErrors}
	}

	synced, err := s.entryRepo.BulkSyncAssets(ctx, treasuryID, assets)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionAssetUpdated, "asset", nil, map[string]any{
			"treasury_id": treasuryID,
			"action":      "bulk_sync",
			"count":       len(synced),
		})
	}

	return synced, nil
}

// ListLiabilities returns all debt obligations for a treasury after verifying authorization.
func (s *TreasuryService) ListLiabilities(ctx context.Context, treasuryID string) ([]domain.Liability, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	return s.entryRepo.ListLiabilities(ctx, treasuryID)
}

// GetLiability returns a single debt obligation after verifying authorization.
func (s *TreasuryService) GetLiability(ctx context.Context, treasuryID, liabilityID string) (*domain.Liability, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	return s.entryRepo.GetLiability(ctx, treasuryID, liabilityID)
}

// CreateLiability validates and persists a new debt obligation.
func (s *TreasuryService) CreateLiability(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	if errs := domain.ValidateLiability(liability); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	saved, err := s.entryRepo.CreateLiability(ctx, treasuryID, liability)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionLiabilityCreated, "liability", &saved.ID, map[string]any{
			"treasury_id":   treasuryID,
			"name":          saved.Name,
			"principal":     saved.Principal,
			"interest_rate": saved.InterestRate,
		})
	}

	return saved, nil
}

// UpdateLiability modifies an existing debt obligation in the treasury.
func (s *TreasuryService) UpdateLiability(ctx context.Context, treasuryID, liabilityID string, liability domain.Liability) (*domain.Liability, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	liability.ID = liabilityID
	if errs := domain.ValidateLiability(liability); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	updated, err := s.entryRepo.UpdateLiability(ctx, treasuryID, liabilityID, liability)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionLiabilityUpdated, "liability", &updated.ID, map[string]any{
			"treasury_id":   treasuryID,
			"name":          updated.Name,
			"principal":     updated.Principal,
			"interest_rate": updated.InterestRate,
		})
	}

	return updated, nil
}

// DeleteLiability removes a debt obligation from the treasury.
func (s *TreasuryService) DeleteLiability(ctx context.Context, treasuryID, liabilityID string) error {
	if s.entryRepo == nil {
		return domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return err
	}

	if err := s.entryRepo.DeleteLiability(ctx, treasuryID, liabilityID); err != nil {
		return err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionLiabilityDeleted, "liability", &liabilityID, map[string]any{
			"treasury_id":  treasuryID,
			"liability_id": liabilityID,
		})
	}

	return nil
}

// BulkSyncLiabilities validates and replaces all liabilities for a treasury.
func (s *TreasuryService) BulkSyncLiabilities(ctx context.Context, treasuryID string, liabilities []domain.Liability) ([]domain.Liability, error) {
	if s.entryRepo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	if _, err := s.GetByID(ctx, treasuryID); err != nil {
		return nil, err
	}

	seenIDs := make(map[string]bool)
	var allErrors []string
	for i, l := range liabilities {
		if seenIDs[l.ID] {
			allErrors = append(allErrors, fmt.Sprintf("duplicate liability id '%s' detected", l.ID))
		}
		seenIDs[l.ID] = true
		if errs := domain.ValidateLiability(l); len(errs) > 0 {
			for _, errStr := range errs {
				allErrors = append(allErrors, fmt.Sprintf("liabilities[%d]: %s", i, errStr))
			}
		}
	}

	if len(allErrors) > 0 {
		return nil, &domain.ValidationError{Details: allErrors}
	}

	synced, err := s.entryRepo.BulkSyncLiabilities(ctx, treasuryID, liabilities)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionLiabilityUpdated, "liability", nil, map[string]any{
			"treasury_id": treasuryID,
			"action":      "bulk_sync",
			"count":       len(synced),
		})
	}

	return synced, nil
}
