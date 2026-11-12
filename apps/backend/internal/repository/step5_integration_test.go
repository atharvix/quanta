//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStep5_PostgreSQL_Integration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Ensure migrations applied
	if err := database.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	db := database.NewDB(databaseURL)
	if err := db.Connect(ctx); err != nil {
		t.Fatalf("failed to connect db wrapper: %v", err)
	}
	defer db.Close()

	// 1. Create a parent corporate treasury for foreign keys
	treasuryRepo := NewPgxTreasuryRepository(db)
	treasury, err := treasuryRepo.Create(ctx, domain.CreateTreasuryInput{
		Name:              fmt.Sprintf("Step 5 Integration Treasury %d", time.Now().UnixNano()),
		TotalCapital:      100.0,
		RiskTolerance:     0.5,
		MinLiquidityRatio: 0.2,
	})
	if err != nil {
		t.Fatalf("failed to create test treasury: %v", err)
	}

	// 2. Test AuditRepository Create and List
	auditRepo := NewPgxAuditRepository(db)
	meta := json.RawMessage(`{"test":true,"value":123}`)
	auditEntry, err := auditRepo.Create(ctx, domain.AuditLog{
		Action:     domain.ActionTreasuryCreated,
		EntityType: "treasury",
		EntityID:   &treasury.ID,
		Actor:      "system",
		Metadata:   meta,
	})
	if err != nil {
		t.Fatalf("auditRepo.Create failed: %v", err)
	}
	if auditEntry.ID == "" {
		t.Fatal("expected non-empty audit ID")
	}

	auditList, err := auditRepo.List(ctx, 10, 0, "treasury", domain.ActionTreasuryCreated)
	if err != nil {
		t.Fatalf("auditRepo.List failed: %v", err)
	}
	if len(auditList) == 0 {
		t.Fatal("expected at least 1 audit log record")
	}

	// 3. Test FinancialProblemRepository Create, GetByID, and List
	fpRepo := NewPgxFinancialProblemRepository(db)
	rawProblem := json.RawMessage(`{"capital":100,"assets":[]}`)
	savedFP, err := fpRepo.Create(ctx, domain.FinancialProblemEntity{
		TreasuryID:  treasury.ID,
		Algorithm:   "astar",
		Objective:   "maximize_risk_adjusted_return",
		MarketState: "normal",
		ProblemJSON: rawProblem,
	})
	if err != nil {
		t.Fatalf("fpRepo.Create failed: %v", err)
	}
	if savedFP.ID == "" {
		t.Fatal("expected non-empty problem ID")
	}

	fetchedFP, err := fpRepo.GetByID(ctx, savedFP.ID)
	if err != nil {
		t.Fatalf("fpRepo.GetByID failed: %v", err)
	}
	if fetchedFP.TreasuryID != treasury.ID {
		t.Errorf("expected treasury_id %s, got %s", treasury.ID, fetchedFP.TreasuryID)
	}

	fpList, err := fpRepo.List(ctx, 10, 0, treasury.ID)
	if err != nil {
		t.Fatalf("fpRepo.List failed: %v", err)
	}
	if len(fpList) == 0 {
		t.Fatal("expected at least 1 financial problem in list")
	}

	// 4. Test OrchestrationRepository Create and GetByID
	orchRepo := NewPgxOrchestrationRepository(db)
	respJSON := json.RawMessage(`{"status":"completed"}`)
	savedRun, err := orchRepo.Create(ctx, domain.OrchestrationRun{
		TreasuryID:         treasury.ID,
		Status:             domain.OrchestrationStatusCompleted,
		AIStatus:           "completed",
		OptimizationStatus: "not_available",
		ResponseJSON:       respJSON,
		DurationMS:         125.50,
	})
	if err != nil {
		t.Fatalf("orchRepo.Create failed: %v", err)
	}
	if savedRun.ID == "" {
		t.Fatal("expected non-empty run ID")
	}

	fetchedRun, err := orchRepo.GetByID(ctx, savedRun.ID)
	if err != nil {
		t.Fatalf("orchRepo.GetByID failed: %v", err)
	}
	if fetchedRun.Status != domain.OrchestrationStatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", fetchedRun.Status)
	}
	if fetchedRun.DurationMS != 125.50 {
		t.Errorf("expected duration 125.50, got %f", fetchedRun.DurationMS)
	}
}
