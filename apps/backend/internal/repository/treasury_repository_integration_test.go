//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func TestTreasuryRepository_PostgreSQL_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL for integration test: %v", err)
	}
	defer db.Close()

	repo := NewPgxTreasuryRepository(db)

	// 1. Create Treasury
	uniqueName := "Integration Treasury " + time.Now().Format("2006-01-02 15:04:05.000")
	input := domain.CreateTreasuryInput{
		Name:              uniqueName,
		BaseCurrency:      "INR",
		TotalCapital:      75000000.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.25,
	}

	created, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}

	if created.ID == "" {
		t.Fatalf("expected non-empty UUID, got empty")
	}
	if created.Name != uniqueName {
		t.Errorf("expected name %s, got %s", uniqueName, created.Name)
	}
	if created.TotalCapital != 75000000.0 {
		t.Errorf("expected total_capital 75000000.0, got %f", created.TotalCapital)
	}

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("repo.GetByID failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
	if fetched.Name != uniqueName {
		t.Errorf("expected Name %s, got %s", uniqueName, fetched.Name)
	}

	// 3. GetByID with non-existent UUID returns ErrNotFound
	_, err = repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent UUID, got %v", err)
	}

	// 4. List Treasuries
	list, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("repo.List failed: %v", err)
	}
	if len(list) == 0 {
		t.Errorf("expected at least 1 treasury in list, got 0")
	}

	found := false
	for _, tr := range list {
		if tr.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created treasury %s was not returned in List query", created.ID)
	}
}
