package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// MockTreasuryRepository implements repository.TreasuryRepository for unit testing.
type MockTreasuryRepository struct {
	listFn    func(ctx context.Context, limit, offset int) ([]domain.Treasury, error)
	getByIDFn func(ctx context.Context, id string) (*domain.Treasury, error)
	createFn  func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error)
	updateFn  func(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error)
}

func (m *MockTreasuryRepository) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return nil, nil
}

func (m *MockTreasuryRepository) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockTreasuryRepository) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}
	return nil, nil
}

func (m *MockTreasuryRepository) Update(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}
	return nil, nil
}

func TestTreasuryService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid pagination with default limit", func(t *testing.T) {
		mock := &MockTreasuryRepository{
			listFn: func(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
				if limit != 20 {
					t.Errorf("expected default limit 20, got %d", limit)
				}
				if offset != 0 {
					t.Errorf("expected offset 0, got %d", offset)
				}
				return []domain.Treasury{{ID: "uuid-1", Name: "Corp 1"}}, nil
			},
		}
		s := NewTreasuryService(mock)
		res, err := s.List(ctx, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
	})

	t.Run("Limit exceeding 100 rejected", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		_, err := s.List(ctx, 500, 0)
		if err == nil || !errors.Is(err, domain.ErrInvalidPagination) {
			t.Fatalf("expected ErrInvalidPagination for limit 500, got %v", err)
		}
	})

	t.Run("Negative offset rejected", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		_, err := s.List(ctx, 20, -5)
		if err == nil || !errors.Is(err, domain.ErrInvalidPagination) {
			t.Fatalf("expected ErrInvalidPagination for negative offset, got %v", err)
		}
	})

	t.Run("List enforces resource access filtering by caller role and owner", func(t *testing.T) {
		ownerA := "owner-a"
		ownerB := "owner-b"
		allRows := []domain.Treasury{
			{ID: "t-a", Name: "Owned by A", OwnerID: &ownerA},
			{ID: "t-b", Name: "Owned by B", OwnerID: &ownerB},
			{ID: "t-unowned", Name: "Legacy Shared", OwnerID: nil},
		}
		s := NewTreasuryService(&MockTreasuryRepository{
			listFn: func(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
				return allRows, nil
			},
		})

		analystA := domain.WithUserContext(context.Background(), ownerA, "a@quanta.local", domain.RoleAnalyst)
		gotA, err := s.List(analystA, 20, 0)
		if err != nil {
			t.Fatalf("unexpected error for analyst: %v", err)
		}
		if len(gotA) != 2 || gotA[0].ID != "t-a" || gotA[1].ID != "t-unowned" {
			t.Fatalf("expected analyst A to see own + unassigned treasuries, got %+v", gotA)
		}

		viewerA := domain.WithUserContext(context.Background(), ownerA, "v@quanta.local", domain.RoleViewer)
		gotViewer, err := s.List(viewerA, 20, 0)
		if err != nil {
			t.Fatalf("unexpected error for viewer: %v", err)
		}
		if len(gotViewer) != 1 || gotViewer[0].ID != "t-a" {
			t.Fatalf("expected viewer A to see only own treasury, got %+v", gotViewer)
		}

		adminCtx := domain.WithUserContext(context.Background(), "admin-1", "admin@quanta.local", domain.RoleAdmin)
		gotAdmin, err := s.List(adminCtx, 20, 0)
		if err != nil {
			t.Fatalf("unexpected error for admin: %v", err)
		}
		if len(gotAdmin) != 3 {
			t.Fatalf("expected admin to see all 3 treasuries, got %d", len(gotAdmin))
		}
	})
}

func TestTreasuryService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid UUID format rejected", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		_, err := s.GetByID(ctx, "not-a-valid-uuid")
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("expected ErrInvalidID, got: %v", err)
		}
	})

	t.Run("Valid UUID format succeeds", func(t *testing.T) {
		validUUID := "123e4567-e89b-12d3-a456-426614174000"
		mock := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:        id,
					Name:      "Test Corp",
					CreatedAt: time.Now(),
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		tRecord, err := s.GetByID(ctx, validUUID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tRecord.ID != validUUID {
			t.Errorf("expected ID '%s', got '%s'", validUUID, tRecord.ID)
		}
	})

	t.Run("Owner access to owned treasury succeeds", func(t *testing.T) {
		ownerID := "user-owner-123"
		validUUID := "123e4567-e89b-12d3-a456-426614174000"
		mock := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:        id,
					Name:      "Owned Corp",
					OwnerID:   &ownerID,
					CreatedAt: time.Now(),
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		authCtx := domain.WithUserContext(context.Background(), ownerID, "owner@example.com", domain.RoleAnalyst)
		tRecord, err := s.GetByID(authCtx, validUUID)
		if err != nil {
			t.Fatalf("unexpected error for owner: %v", err)
		}
		if tRecord.ID != validUUID {
			t.Errorf("expected ID '%s', got '%s'", validUUID, tRecord.ID)
		}
	})

	t.Run("Non-owner analyst access to owned treasury returns ErrForbidden", func(t *testing.T) {
		ownerID := "user-owner-123"
		otherAnalystID := "user-analyst-456"
		validUUID := "123e4567-e89b-12d3-a456-426614174000"
		mock := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:        id,
					Name:      "Owned Corp",
					OwnerID:   &ownerID,
					CreatedAt: time.Now(),
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		authCtx := domain.WithUserContext(context.Background(), otherAnalystID, "other@example.com", domain.RoleAnalyst)
		_, err := s.GetByID(authCtx, validUUID)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for non-owner, got: %v", err)
		}
	})

	t.Run("Admin access to another user's treasury succeeds", func(t *testing.T) {
		ownerID := "user-owner-123"
		adminID := "user-admin-999"
		validUUID := "123e4567-e89b-12d3-a456-426614174000"
		mock := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:        id,
					Name:      "Owned Corp",
					OwnerID:   &ownerID,
					CreatedAt: time.Now(),
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		adminCtx := domain.WithUserContext(context.Background(), adminID, "admin@example.com", domain.RoleAdmin)
		tRecord, err := s.GetByID(adminCtx, validUUID)
		if err != nil {
			t.Fatalf("unexpected error for admin: %v", err)
		}
		if tRecord.ID != validUUID {
			t.Errorf("expected ID '%s', got '%s'", validUUID, tRecord.ID)
		}
	})
}

func TestTreasuryService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("Validation failure on empty name", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		input := domain.CreateTreasuryInput{
			Name:         "",
			TotalCapital: 10000.0,
		}
		_, err := s.Create(ctx, input)
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
	})

	t.Run("Successful creation with defaults applied", func(t *testing.T) {
		mock := &MockTreasuryRepository{
			createFn: func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
				if input.BaseCurrency != "INR" {
					t.Errorf("expected default BaseCurrency INR, got %s", input.BaseCurrency)
				}
				return &domain.Treasury{
					ID:           "test-id",
					Name:         input.Name,
					BaseCurrency: input.BaseCurrency,
					TotalCapital: input.TotalCapital,
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		input := domain.CreateTreasuryInput{
			Name:         "Valid Corp",
			TotalCapital: 5000000.0,
		}
		created, err := s.Create(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.Name != "Valid Corp" {
			t.Errorf("expected Name 'Valid Corp', got '%s'", created.Name)
		}
	})
}

// MockFinancialEntryRepository implements repository.FinancialEntryRepository for unit testing.
type MockFinancialEntryRepository struct {
	listAssetsFn          func(ctx context.Context, treasuryID string) ([]domain.Asset, error)
	getAssetFn           func(ctx context.Context, treasuryID, assetID string) (*domain.Asset, error)
	createAssetFn        func(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error)
	updateAssetFn        func(ctx context.Context, treasuryID, assetID string, asset domain.Asset) (*domain.Asset, error)
	deleteAssetFn        func(ctx context.Context, treasuryID, assetID string) error
	bulkSyncAssetsFn     func(ctx context.Context, treasuryID string, assets []domain.Asset) ([]domain.Asset, error)
	listLiabilitiesFn    func(ctx context.Context, treasuryID string) ([]domain.Liability, error)
	getLiabilityFn       func(ctx context.Context, treasuryID, liabilityID string) (*domain.Liability, error)
	createLiabilityFn    func(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error)
	updateLiabilityFn    func(ctx context.Context, treasuryID, liabilityID string, liability domain.Liability) (*domain.Liability, error)
	deleteLiabilityFn    func(ctx context.Context, treasuryID, liabilityID string) error
	bulkSyncLiabilitiesFn func(ctx context.Context, treasuryID string, liabilities []domain.Liability) ([]domain.Liability, error)
}

func (m *MockFinancialEntryRepository) ListAssets(ctx context.Context, treasuryID string) ([]domain.Asset, error) {
	if m.listAssetsFn != nil {
		return m.listAssetsFn(ctx, treasuryID)
	}
	return nil, nil
}
func (m *MockFinancialEntryRepository) GetAsset(ctx context.Context, treasuryID, assetID string) (*domain.Asset, error) {
	if m.getAssetFn != nil {
		return m.getAssetFn(ctx, treasuryID, assetID)
	}
	return nil, nil
}
func (m *MockFinancialEntryRepository) CreateAsset(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error) {
	if m.createAssetFn != nil {
		return m.createAssetFn(ctx, treasuryID, asset)
	}
	return &asset, nil
}
func (m *MockFinancialEntryRepository) UpdateAsset(ctx context.Context, treasuryID, assetID string, asset domain.Asset) (*domain.Asset, error) {
	if m.updateAssetFn != nil {
		return m.updateAssetFn(ctx, treasuryID, assetID, asset)
	}
	return &asset, nil
}
func (m *MockFinancialEntryRepository) DeleteAsset(ctx context.Context, treasuryID, assetID string) error {
	if m.deleteAssetFn != nil {
		return m.deleteAssetFn(ctx, treasuryID, assetID)
	}
	return nil
}
func (m *MockFinancialEntryRepository) BulkSyncAssets(ctx context.Context, treasuryID string, assets []domain.Asset) ([]domain.Asset, error) {
	if m.bulkSyncAssetsFn != nil {
		return m.bulkSyncAssetsFn(ctx, treasuryID, assets)
	}
	return assets, nil
}
func (m *MockFinancialEntryRepository) ListLiabilities(ctx context.Context, treasuryID string) ([]domain.Liability, error) {
	if m.listLiabilitiesFn != nil {
		return m.listLiabilitiesFn(ctx, treasuryID)
	}
	return nil, nil
}
func (m *MockFinancialEntryRepository) GetLiability(ctx context.Context, treasuryID, liabilityID string) (*domain.Liability, error) {
	if m.getLiabilityFn != nil {
		return m.getLiabilityFn(ctx, treasuryID, liabilityID)
	}
	return nil, nil
}
func (m *MockFinancialEntryRepository) CreateLiability(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error) {
	if m.createLiabilityFn != nil {
		return m.createLiabilityFn(ctx, treasuryID, liability)
	}
	return &liability, nil
}
func (m *MockFinancialEntryRepository) UpdateLiability(ctx context.Context, treasuryID, liabilityID string, liability domain.Liability) (*domain.Liability, error) {
	if m.updateLiabilityFn != nil {
		return m.updateLiabilityFn(ctx, treasuryID, liabilityID, liability)
	}
	return &liability, nil
}
func (m *MockFinancialEntryRepository) DeleteLiability(ctx context.Context, treasuryID, liabilityID string) error {
	if m.deleteLiabilityFn != nil {
		return m.deleteLiabilityFn(ctx, treasuryID, liabilityID)
	}
	return nil
}
func (m *MockFinancialEntryRepository) BulkSyncLiabilities(ctx context.Context, treasuryID string, liabilities []domain.Liability) ([]domain.Liability, error) {
	if m.bulkSyncLiabilitiesFn != nil {
		return m.bulkSyncLiabilitiesFn(ctx, treasuryID, liabilities)
	}
	return liabilities, nil
}

func TestTreasuryService_CustomAssetsAndLiabilities(t *testing.T) {
	const validUUID = "550e8400-e29b-41d4-a716-446655440000"
	ownerID := "user-owner-1"
	otherID := "user-other-2"

	t.Run("CreateAsset - Success and Validation", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: validUUID, Name: "Treasury 1", OwnerID: &ownerID}, nil
			},
		}
		entryRepo := &MockFinancialEntryRepository{}

		svc := NewTreasuryService(treasuryRepo)
		svc.SetEntryRepository(entryRepo)

		// 1. Authorized context as owner
		ownerCtx := domain.WithUserContext(context.Background(), ownerID, "owner@quanta.local", domain.RoleAnalyst)

		validAsset := domain.Asset{
			ID:             "green_bond",
			Name:           "Green Energy Bond",
			AssetType:      "gov_bond",
			Cost:           0.001,
			ExpectedReturn: 0.08,
			Risk:           0.04,
			Liquidity:      0.90,
			Volatility:     0.03,
		}

		created, err := svc.CreateAsset(ownerCtx, validUUID, validAsset)
		if err != nil {
			t.Fatalf("expected successful asset creation, got error: %v", err)
		}
		if created.ID != "green_bond" {
			t.Errorf("expected ID 'green_bond', got %s", created.ID)
		}

		// 2. Validation failure - risk > 1.0
		invalidAsset := validAsset
		invalidAsset.Risk = 1.5
		_, err = svc.CreateAsset(ownerCtx, validUUID, invalidAsset)
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected validation error for risk > 1.0, got: %v", err)
		}

		// Validation failure - invalid asset type "bond"
		invalidTypeAsset := validAsset
		invalidTypeAsset.AssetType = "bond"
		_, err = svc.CreateAsset(ownerCtx, validUUID, invalidTypeAsset)
		if !errors.As(err, &valErr) {
			t.Fatalf("expected validation error for invalid asset type 'bond', got: %v", err)
		}

		// 3. Unauthorized caller (different user ID, not admin)
		unauthCtx := domain.WithUserContext(context.Background(), otherID, "other@quanta.local", domain.RoleAnalyst)
		_, err = svc.CreateAsset(unauthCtx, validUUID, validAsset)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for non-owner, got: %v", err)
		}

		// 4. Admin override allows access
		adminCtx := domain.WithUserContext(context.Background(), "admin-1", "admin@quanta.local", domain.RoleAdmin)
		_, err = svc.CreateAsset(adminCtx, validUUID, validAsset)
		if err != nil {
			t.Fatalf("expected admin to bypass ownership check, got error: %v", err)
		}
	})

	t.Run("CreateLiability - Success and Validation", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: validUUID, Name: "Treasury 1", OwnerID: &ownerID}, nil
			},
		}
		entryRepo := &MockFinancialEntryRepository{}

		svc := NewTreasuryService(treasuryRepo)
		svc.SetEntryRepository(entryRepo)

		ownerCtx := domain.WithUserContext(context.Background(), ownerID, "owner@quanta.local", domain.RoleAnalyst)

		validLiab := domain.Liability{
			ID:                 "term_loan_1",
			Name:               "5-Year Bank Term Loan",
			Principal:          25.0,
			InterestRate:       0.055,
			MaturityMonths:     60,
			Priority:           1,
			IsMandatoryReserve: true,
		}

		created, err := svc.CreateLiability(ownerCtx, validUUID, validLiab)
		if err != nil {
			t.Fatalf("expected successful liability creation, got error: %v", err)
		}
		if created.Principal != 25.0 {
			t.Errorf("expected principal 25.0, got %f", created.Principal)
		}

		// Validation failure - negative principal
		invalidLiab := validLiab
		invalidLiab.Principal = -5.0
		_, err = svc.CreateLiability(ownerCtx, validUUID, invalidLiab)
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected validation error for negative principal, got: %v", err)
		}
	})

	t.Run("Update and Delete Asset & Liability", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: validUUID, Name: "Treasury 1", OwnerID: &ownerID}, nil
			},
		}
		entryRepo := &MockFinancialEntryRepository{}

		svc := NewTreasuryService(treasuryRepo)
		svc.SetEntryRepository(entryRepo)

		ownerCtx := domain.WithUserContext(context.Background(), ownerID, "owner@quanta.local", domain.RoleAnalyst)

		// Update asset
		updatedAsset, err := svc.UpdateAsset(ownerCtx, validUUID, "green_bond", domain.Asset{
			Name:           "Updated Green Energy Bond",
			AssetType:      "gov_bond",
			Cost:           0.001,
			ExpectedReturn: 0.09,
			Risk:           0.04,
			Liquidity:      0.90,
		})
		if err != nil {
			t.Fatalf("failed to update asset: %v", err)
		}
		if updatedAsset.Name != "Updated Green Energy Bond" {
			t.Errorf("expected name 'Updated Green Energy Bond', got %s", updatedAsset.Name)
		}

		// Delete asset
		if err := svc.DeleteAsset(ownerCtx, validUUID, "green_bond"); err != nil {
			t.Fatalf("failed to delete asset: %v", err)
		}

		// Delete liability
		if err := svc.DeleteLiability(ownerCtx, validUUID, "term_loan_1"); err != nil {
			t.Fatalf("failed to delete liability: %v", err)
		}
	})
}

func TestTreasuryService_Update(t *testing.T) {
	validUUID := "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"
	ownerID := "user-owner-123"
	nonOwnerID := "user-attacker-456"

	existingTreasury := domain.Treasury{
		ID:                validUUID,
		Name:              "Original Corporate Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      100000000.0,
		RiskTolerance:     0.50,
		MinLiquidityRatio: 0.20,
		OwnerID:           &ownerID,
		CreatedAt:         time.Now().Add(-24 * time.Hour),
		UpdatedAt:         time.Now().Add(-24 * time.Hour),
	}

	ownerCtx := domain.WithUserContext(context.Background(), ownerID, "owner@quanta.local", domain.RoleAnalyst)
	nonOwnerCtx := domain.WithUserContext(context.Background(), nonOwnerID, "attacker@quanta.local", domain.RoleAnalyst)
	adminCtx := domain.WithUserContext(context.Background(), "admin-789", "admin@quanta.local", domain.RoleAdmin)

	t.Run("Successful partial update by owner - Name only", func(t *testing.T) {
		mockRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existingTreasury, nil
			},
			updateFn: func(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error) {
				if id != validUUID {
					t.Errorf("expected id %s, got %s", validUUID, id)
				}
				updated := existingTreasury
				if input.Name != nil {
					updated.Name = *input.Name
				}
				updated.UpdatedAt = time.Now()
				return &updated, nil
			},
		}

		svc := NewTreasuryService(mockRepo)
		newName := "Updated Global Treasury"
		res, err := svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{Name: &newName})
		if err != nil {
			t.Fatalf("expected successful update, got: %v", err)
		}
		if res.Name != "Updated Global Treasury" {
			t.Errorf("expected name 'Updated Global Treasury', got '%s'", res.Name)
		}
		if res.TotalCapital != existingTreasury.TotalCapital {
			t.Errorf("expected capital %.2f preserved, got %.2f", existingTreasury.TotalCapital, res.TotalCapital)
		}
		if res.BaseCurrency != existingTreasury.BaseCurrency {
			t.Errorf("expected currency %s preserved, got %s", existingTreasury.BaseCurrency, res.BaseCurrency)
		}
	})

	t.Run("Successful multi-field update by Admin", func(t *testing.T) {
		mockRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existingTreasury, nil
			},
			updateFn: func(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error) {
				updated := existingTreasury
				if input.Name != nil {
					updated.Name = *input.Name
				}
				if input.BaseCurrency != nil {
					updated.BaseCurrency = *input.BaseCurrency
				}
				if input.TotalCapital != nil {
					updated.TotalCapital = *input.TotalCapital
				}
				if input.RiskTolerance != nil {
					updated.RiskTolerance = *input.RiskTolerance
				}
				if input.MinLiquidityRatio != nil {
					updated.MinLiquidityRatio = *input.MinLiquidityRatio
				}
				updated.UpdatedAt = time.Now()
				return &updated, nil
			},
		}

		svc := NewTreasuryService(mockRepo)
		newName := "Admin Renamed Treasury"
		newCurrency := "USD"
		newCapital := 250000000.0
		newRisk := 0.75
		newLiquidity := 0.35

		res, err := svc.Update(adminCtx, validUUID, domain.UpdateTreasuryInput{
			Name:              &newName,
			BaseCurrency:      &newCurrency,
			TotalCapital:      &newCapital,
			RiskTolerance:     &newRisk,
			MinLiquidityRatio: &newLiquidity,
		})
		if err != nil {
			t.Fatalf("expected admin update to succeed, got: %v", err)
		}
		if res.Name != newName || res.BaseCurrency != newCurrency || res.TotalCapital != newCapital ||
			res.RiskTolerance != newRisk || res.MinLiquidityRatio != newLiquidity {
			t.Errorf("multi-field update fields did not match expected values")
		}
	})

	t.Run("Unauthorized user (non-owner analyst) rejected with ErrForbidden", func(t *testing.T) {
		mockRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existingTreasury, nil
			},
		}

		svc := NewTreasuryService(mockRepo)
		newName := "Hacked Name"
		_, err := svc.Update(nonOwnerCtx, validUUID, domain.UpdateTreasuryInput{Name: &newName})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("expected ErrForbidden for non-owner, got: %v", err)
		}
	})

	t.Run("Missing treasury returns ErrNotFound", func(t *testing.T) {
		mockRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}

		svc := NewTreasuryService(mockRepo)
		newName := "Updated Name"
		_, err := svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{Name: &newName})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound for missing treasury, got: %v", err)
		}
	})

	t.Run("Invalid UUID format returns ErrInvalidID", func(t *testing.T) {
		svc := NewTreasuryService(&MockTreasuryRepository{})
		newName := "Updated Name"
		_, err := svc.Update(ownerCtx, "invalid-uuid-123", domain.UpdateTreasuryInput{Name: &newName})
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Errorf("expected ErrInvalidID for invalid UUID, got: %v", err)
		}
	})

	t.Run("Validation invariants", func(t *testing.T) {
		mockRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existingTreasury, nil
			},
		}
		svc := NewTreasuryService(mockRepo)

		// 1. Empty payload
		_, err := svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{})
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("expected validation error for empty patch payload, got: %v", err)
		}

		// 2. Empty name
		emptyName := "   "
		_, err = svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{Name: &emptyName})
		if !errors.As(err, &valErr) {
			t.Errorf("expected validation error for whitespace name, got: %v", err)
		}

		// 3. Negative total capital
		negCapital := -100.0
		_, err = svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{TotalCapital: &negCapital})
		if !errors.As(err, &valErr) {
			t.Errorf("expected validation error for negative capital, got: %v", err)
		}

		// 4. Out of range risk tolerance
		invalidRisk := 1.5
		_, err = svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{RiskTolerance: &invalidRisk})
		if !errors.As(err, &valErr) {
			t.Errorf("expected validation error for risk tolerance > 1.0, got: %v", err)
		}

		// 5. Out of range min liquidity ratio
		invalidLiq := -0.1
		_, err = svc.Update(ownerCtx, validUUID, domain.UpdateTreasuryInput{MinLiquidityRatio: &invalidLiq})
		if !errors.As(err, &valErr) {
			t.Errorf("expected validation error for liquidity < 0.0, got: %v", err)
		}
	})
}
