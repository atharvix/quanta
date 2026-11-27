package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository/postgres"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

func requireLivePostgres(t *testing.T) *database.DB {
	t.Helper()
	dsn := os.Getenv("QUANTA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PENDING: requires live PostgreSQL instance (set QUANTA_TEST_DATABASE_URL on personal laptop)")
	}
	db := database.NewDB(dsn)
	if err := db.Connect(context.Background()); err != nil {
		t.Fatalf("connect live postgres: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.MigrateUp(context.Background()); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}
	return db
}

func TestPostgresIntegration_MigrationsAppendOnlyAndLedgerTx_PENDING(t *testing.T) {
	db := requireLivePostgres(t)
	ctx := context.Background()
	pool := db.Pool()

	orgRepo := postgres.NewOrganizationRepository(pool)
	treasuryRepo := postgres.NewTreasuryRepository(pool)
	accountRepo := postgres.NewAccountRepository(pool)
	ledgerRepo := postgres.NewLedgerRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	txMgr := postgres.NewTxManager(pool)

	acctSvc := service.NewAccountService(treasuryRepo, accountRepo, ledgerRepo, auditRepo)
	ledgerSvc := service.NewLedgerService(txMgr)

	org := domain.Organization{
		ID:     uuid.New(),
		Name:   "Integration Test Org " + uuid.NewString()[:8],
		Status: domain.OrgStatusActive,
	}
	if err := orgRepo.CreateOrganization(ctx, org); err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	entity := domain.TreasuryEntity{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		Name:           "USD Treasury",
		BaseCurrency:   "USD",
		IsSynthetic:    true,
	}
	if err := treasuryRepo.CreateTreasuryEntity(ctx, entity); err != nil {
		t.Fatalf("CreateTreasuryEntity failed: %v", err)
	}

	acc, err := acctSvc.CreateAccount(ctx, service.CreateAccountInput{
		OrganizationID:   org.ID,
		TreasuryEntityID: entity.ID,
		AccountCode:      "INT-USD-01",
		Name:             "Integration Cash",
		CurrencyCode:     "USD",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	entry, acc, err := ledgerSvc.Credit(ctx, service.LedgerOperationInput{
		OrganizationID: org.ID,
		AccountID:      acc.ID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("2500.0000"),
		ReferenceType:  "INTEGRATION_SEED",
		ReferenceID:    uuid.New(),
		CorrelationID:  "pg-int-1",
	})
	if err != nil {
		t.Fatalf("Credit failed: %v", err)
	}

	// Verify append-only trigger blocks UPDATE on ledger_entries.
	if _, err := pool.Exec(ctx, `UPDATE ledger_entries SET delta_amount = 1 WHERE id = $1`, entry.ID); err == nil {
		t.Fatal("expected append-only trigger to reject UPDATE on ledger_entries")
	}

	report, err := acctSvc.ReconcileAccount(ctx, org.ID, acc.ID, nil, string(domain.RoleAdmin), "pg-int-2")
	if err != nil || !report.Matched {
		t.Fatalf("expected clean reconciliation, got err=%v report=%+v", err, report)
	}
}
