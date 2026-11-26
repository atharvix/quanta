package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func getTestDB(t *testing.T) *database.DB {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping repository test: PostgreSQL unavailable (%v)", err)
		return nil
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

func createIsolatedTreasury(t *testing.T, db *database.DB) *domain.Treasury {
	t.Helper()
	treasuryRepo := NewPgxTreasuryRepository(db)
	ctx := context.Background()

	name := fmt.Sprintf("Repo Test Treasury %d", time.Now().UnixNano())
	input := domain.CreateTreasuryInput{
		Name:              name,
		BaseCurrency:      "USD",
		TotalCapital:      50000000.0,
		RiskTolerance:     0.50,
		MinLiquidityRatio: 0.20,
	}

	treasury, err := treasuryRepo.Create(ctx, input)
	if err != nil {
		t.Fatalf("failed to create isolated test treasury: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if pool := db.Pool(); pool != nil {
			_, err := pool.Exec(cleanupCtx, "DELETE FROM corporate_treasuries WHERE id = $1", treasury.ID)
			if err != nil {
				t.Logf("cleanup warning: failed to delete test treasury %s: %v", treasury.ID, err)
			}
		}
	})

	return treasury
}

func TestFinancialEntryRepository_AssetsCRUD(t *testing.T) {
	db := getTestDB(t)
	treasury := createIsolatedTreasury(t, db)
	repo := NewPgxFinancialEntryRepository(db)
	ctx := context.Background()

	// 1. Create Asset
	assetID := fmt.Sprintf("asset_%d", time.Now().UnixNano()%100000)
	asset := domain.Asset{
		ID:             assetID,
		Name:           "Institutional US Treasury Bill 3M",
		AssetType:      "gov_bond",
		Cost:           0.001,
		ExpectedReturn: 0.052,
		Risk:           0.015,
		Liquidity:      0.98,
		Volatility:     0.008,
		Metadata: map[string]any{
			"cusip":  "912796XX0",
			"rating": "AAA",
		},
	}

	created, err := repo.CreateAsset(ctx, treasury.ID, asset)
	if err != nil {
		t.Fatalf("CreateAsset failed: %v", err)
	}
	if created.ID != assetID {
		t.Errorf("expected ID %s, got %s", assetID, created.ID)
	}
	if created.Name != asset.Name {
		t.Errorf("expected Name %s, got %s", asset.Name, created.Name)
	}

	// 2. Get Asset
	fetched, err := repo.GetAsset(ctx, treasury.ID, assetID)
	if err != nil {
		t.Fatalf("GetAsset failed: %v", err)
	}
	if fetched.ID != assetID {
		t.Errorf("expected ID %s, got %s", assetID, fetched.ID)
	}
	if fetched.ExpectedReturn != 0.052 {
		t.Errorf("expected ExpectedReturn 0.052, got %f", fetched.ExpectedReturn)
	}

	// 3. List Assets
	list, err := repo.ListAssets(ctx, treasury.ID)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(list))
	}
	if list[0].ID != assetID {
		t.Errorf("expected asset in list to have ID %s, got %s", assetID, list[0].ID)
	}

	// 4. Update Asset
	asset.ExpectedReturn = 0.055
	asset.Risk = 0.018
	updated, err := repo.UpdateAsset(ctx, treasury.ID, assetID, asset)
	if err != nil {
		t.Fatalf("UpdateAsset failed: %v", err)
	}
	if updated.ExpectedReturn != 0.055 {
		t.Errorf("expected updated ExpectedReturn 0.055, got %f", updated.ExpectedReturn)
	}

	// 5. Delete Asset
	err = repo.DeleteAsset(ctx, treasury.ID, assetID)
	if err != nil {
		t.Fatalf("DeleteAsset failed: %v", err)
	}

	// 6. Verify Deletion
	_, err = repo.GetAsset(ctx, treasury.ID, assetID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestFinancialEntryRepository_BulkSyncAssets(t *testing.T) {
	db := getTestDB(t)
	treasury := createIsolatedTreasury(t, db)
	repo := NewPgxFinancialEntryRepository(db)
	ctx := context.Background()

	// 1. Initial bulk sync with 3 assets
	initialAssets := []domain.Asset{
		{ID: "sync_asset_1", Name: "Asset 1", AssetType: "gov_bond", Cost: 0.001, ExpectedReturn: 0.05, Risk: 0.02, Liquidity: 0.90, Volatility: 0.01},
		{ID: "sync_asset_2", Name: "Asset 2", AssetType: "equity", Cost: 0.002, ExpectedReturn: 0.10, Risk: 0.15, Liquidity: 0.80, Volatility: 0.12},
		{ID: "sync_asset_3", Name: "Asset 3", AssetType: "corp_bond", Cost: 0.0015, ExpectedReturn: 0.07, Risk: 0.06, Liquidity: 0.70, Volatility: 0.04},
	}

	synced, err := repo.BulkSyncAssets(ctx, treasury.ID, initialAssets)
	if err != nil {
		t.Fatalf("BulkSyncAssets failed: %v", err)
	}
	if len(synced) != 3 {
		t.Fatalf("expected 3 synced assets, got %d", len(synced))
	}

	// 2. Second bulk sync replacing with 2 assets (removing sync_asset_3, updating sync_asset_1)
	updatedAssets := []domain.Asset{
		{ID: "sync_asset_1", Name: "Asset 1 Updated", AssetType: "gov_bond", Cost: 0.001, ExpectedReturn: 0.06, Risk: 0.02, Liquidity: 0.90, Volatility: 0.01},
		{ID: "sync_asset_4", Name: "Asset 4 New", AssetType: "cash", Cost: 0.0, ExpectedReturn: 0.03, Risk: 0.0, Liquidity: 1.0, Volatility: 0.0},
	}

	synced2, err := repo.BulkSyncAssets(ctx, treasury.ID, updatedAssets)
	if err != nil {
		t.Fatalf("second BulkSyncAssets failed: %v", err)
	}
	if len(synced2) != 2 {
		t.Fatalf("expected 2 synced assets, got %d", len(synced2))
	}

	list, err := repo.ListAssets(ctx, treasury.ID)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 assets in list, got %d", len(list))
	}

	// sync_asset_3 should be gone
	_, err = repo.GetAsset(ctx, treasury.ID, "sync_asset_3")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected sync_asset_3 to be removed, got err: %v", err)
	}
}

func TestFinancialEntryRepository_LiabilitiesCRUD(t *testing.T) {
	db := getTestDB(t)
	treasury := createIsolatedTreasury(t, db)
	repo := NewPgxFinancialEntryRepository(db)
	ctx := context.Background()

	// 1. Create Liability
	liabilityID := fmt.Sprintf("liab_%d", time.Now().UnixNano()%100000)
	liability := domain.Liability{
		ID:                 liabilityID,
		Name:               "Syndicated Term Loan B",
		Principal:          12500000.0,
		InterestRate:       0.075,
		MaturityMonths:     36,
		Priority:           1,
		IsMandatoryReserve: true,
	}

	created, err := repo.CreateLiability(ctx, treasury.ID, liability)
	if err != nil {
		t.Fatalf("CreateLiability failed: %v", err)
	}
	if created.ID != liabilityID {
		t.Errorf("expected ID %s, got %s", liabilityID, created.ID)
	}
	if created.Principal != 12500000.0 {
		t.Errorf("expected Principal 12500000.0, got %f", created.Principal)
	}

	// 2. Get Liability
	fetched, err := repo.GetLiability(ctx, treasury.ID, liabilityID)
	if err != nil {
		t.Fatalf("GetLiability failed: %v", err)
	}
	if fetched.ID != liabilityID {
		t.Errorf("expected ID %s, got %s", liabilityID, fetched.ID)
	}
	if !fetched.IsMandatoryReserve {
		t.Errorf("expected IsMandatoryReserve to be true")
	}

	// 3. List Liabilities
	list, err := repo.ListLiabilities(ctx, treasury.ID)
	if err != nil {
		t.Fatalf("ListLiabilities failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 liability, got %d", len(list))
	}

	// 4. Update Liability
	liability.Principal = 10000000.0
	liability.InterestRate = 0.072
	updated, err := repo.UpdateLiability(ctx, treasury.ID, liabilityID, liability)
	if err != nil {
		t.Fatalf("UpdateLiability failed: %v", err)
	}
	if updated.Principal != 10000000.0 {
		t.Errorf("expected updated Principal 10000000.0, got %f", updated.Principal)
	}

	// 5. Delete Liability
	err = repo.DeleteLiability(ctx, treasury.ID, liabilityID)
	if err != nil {
		t.Fatalf("DeleteLiability failed: %v", err)
	}

	// 6. Verify Deletion
	_, err = repo.GetLiability(ctx, treasury.ID, liabilityID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestFinancialEntryRepository_BulkSyncLiabilities(t *testing.T) {
	db := getTestDB(t)
	treasury := createIsolatedTreasury(t, db)
	repo := NewPgxFinancialEntryRepository(db)
	ctx := context.Background()

	initialLiabilities := []domain.Liability{
		{ID: "sync_l_1", Name: "Facility A", Principal: 5000000.0, InterestRate: 0.06, MaturityMonths: 12, Priority: 1, IsMandatoryReserve: true},
		{ID: "sync_l_2", Name: "Facility B", Principal: 3000000.0, InterestRate: 0.07, MaturityMonths: 24, Priority: 2, IsMandatoryReserve: false},
	}

	synced, err := repo.BulkSyncLiabilities(ctx, treasury.ID, initialLiabilities)
	if err != nil {
		t.Fatalf("BulkSyncLiabilities failed: %v", err)
	}
	if len(synced) != 2 {
		t.Fatalf("expected 2 synced liabilities, got %d", len(synced))
	}

	list, err := repo.ListLiabilities(ctx, treasury.ID)
	if err != nil {
		t.Fatalf("ListLiabilities failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 liabilities in list, got %d", len(list))
	}
}

func TestFinancialEntryRepository_FailureCasesAndIDOR(t *testing.T) {
	db := getTestDB(t)
	treasury1 := createIsolatedTreasury(t, db)
	treasury2 := createIsolatedTreasury(t, db)
	repo := NewPgxFinancialEntryRepository(db)
	ctx := context.Background()

	// 1. ErrNotFound on non-existent asset
	_, err := repo.GetAsset(ctx, treasury1.ID, "non_existent_asset_xyz")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent asset, got %v", err)
	}

	// 2. ErrNotFound on non-existent liability
	_, err = repo.GetLiability(ctx, treasury1.ID, "non_existent_liability_xyz")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent liability, got %v", err)
	}

	// 3. ErrNotFound on deleting non-existent asset
	err = repo.DeleteAsset(ctx, treasury1.ID, "non_existent_asset_xyz")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting non-existent asset, got %v", err)
	}

	// 4. ErrNotFound on deleting non-existent liability
	err = repo.DeleteLiability(ctx, treasury1.ID, "non_existent_liability_xyz")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting non-existent liability, got %v", err)
	}

	// 5. Cross-treasury IDOR isolation
	assetID := "idor_asset_1"
	_, err = repo.CreateAsset(ctx, treasury1.ID, domain.Asset{
		ID: assetID, Name: "Secret Asset", AssetType: "gov_bond", ExpectedReturn: 0.05, Risk: 0.02, Liquidity: 0.9,
	})
	if err != nil {
		t.Fatalf("failed to create asset under treasury1: %v", err)
	}

	liabilityID := "idor_liab_1"
	_, err = repo.CreateLiability(ctx, treasury1.ID, domain.Liability{
		ID: liabilityID, Name: "Secret Liability", Principal: 1000000.0, InterestRate: 0.05, MaturityMonths: 12, Priority: 1,
	})
	if err != nil {
		t.Fatalf("failed to create liability under treasury1: %v", err)
	}

	// Attempt to access treasury1's asset through treasury2 -> MUST return ErrNotFound
	_, err = repo.GetAsset(ctx, treasury2.ID, assetID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("IDOR vulnerability: treasury2 accessed treasury1 asset! Expected ErrNotFound, got %v", err)
	}

	// Attempt to access treasury1's liability through treasury2 -> MUST return ErrNotFound
	_, err = repo.GetLiability(ctx, treasury2.ID, liabilityID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("IDOR vulnerability: treasury2 accessed treasury1 liability! Expected ErrNotFound, got %v", err)
	}

	// Attempt to update treasury1's asset through treasury2 -> MUST return ErrNotFound
	_, err = repo.UpdateAsset(ctx, treasury2.ID, assetID, domain.Asset{
		ID: assetID, Name: "Hacked Asset", AssetType: "gov_bond",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("IDOR vulnerability: treasury2 updated treasury1 asset! Expected ErrNotFound, got %v", err)
	}

	// Attempt to delete treasury1's asset through treasury2 -> MUST return ErrNotFound
	err = repo.DeleteAsset(ctx, treasury2.ID, assetID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("IDOR vulnerability: treasury2 deleted treasury1 asset! Expected ErrNotFound, got %v", err)
	}

	// 6. Nil database pool returns ErrDatabaseUnavailable
	nilRepo := NewPgxFinancialEntryRepository(nil)
	if _, err := nilRepo.ListAssets(ctx, treasury1.ID); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable for nil db, got %v", err)
	}
	if _, err := nilRepo.GetAsset(ctx, treasury1.ID, assetID); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable for nil db, got %v", err)
	}
	if _, err := nilRepo.CreateAsset(ctx, treasury1.ID, domain.Asset{}); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable for nil db, got %v", err)
	}
	if _, err := nilRepo.UpdateAsset(ctx, treasury1.ID, assetID, domain.Asset{}); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable for nil db, got %v", err)
	}
	if err := nilRepo.DeleteAsset(ctx, treasury1.ID, assetID); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable for nil db, got %v", err)
	}
	if _, err := nilRepo.BulkSyncAssets(ctx, treasury1.ID, nil); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable for nil db, got %v", err)
	}
}

func TestTreasuryRepository_FailureCases(t *testing.T) {
	db := getTestDB(t)
	repo := NewPgxTreasuryRepository(db)
	ctx := context.Background()

	// 1. GetByID with non-existent UUID -> ErrNotFound
	nonExistentUUID := "00000000-0000-0000-0000-000000000000"
	_, err := repo.GetByID(ctx, nonExistentUUID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent treasury ID, got %v", err)
	}

	// 2. Update with non-existent UUID -> ErrNotFound
	fakeName := "Ghost Treasury"
	updateInput := domain.UpdateTreasuryInput{
		Name: &fakeName,
	}
	_, err = repo.Update(ctx, nonExistentUUID, updateInput)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound updating non-existent treasury, got %v", err)
	}

	// 3. Nil database pool returns ErrDatabaseUnavailable
	nilRepo := NewPgxTreasuryRepository(nil)
	if _, err := nilRepo.GetByID(ctx, nonExistentUUID); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable on nil db, got %v", err)
	}
	if _, err := nilRepo.Create(ctx, domain.CreateTreasuryInput{}); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable on nil db, got %v", err)
	}
	if _, err := nilRepo.Update(ctx, nonExistentUUID, updateInput); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable on nil db, got %v", err)
	}
	if _, err := nilRepo.List(ctx, 10, 0); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Errorf("expected ErrDatabaseUnavailable on nil db, got %v", err)
	}
}
