package database

import (
	"strings"
	"testing"
)

func TestLoadMigrations_ValidatesSchemaAndInvariants(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations failed: %v", err)
	}
	if len(migrations) < 2 {
		t.Fatalf("expected at least 2 migrations, got %d", len(migrations))
	}

	combinedUp := migrations[0].UpSQL + "\n" + migrations[1].UpSQL

	requiredTables := []string{
		"organizations",
		"users",
		"organization_memberships",
		"treasury_entities",
		"treasury_accounts",
		"ledger_entries",
		"fx_rate_snapshots",
		"liquidity_policies",
		"counterparties",
		"financial_constraints",
		"optimization_runs",
		"allocation_proposals",
		"approval_decisions",
		"audit_events",
	}

	for _, tbl := range requiredTables {
		if !strings.Contains(combinedUp, "CREATE TABLE IF NOT EXISTS "+tbl) {
			t.Fatalf("migration missing required table %q", tbl)
		}
	}

	forbiddenTypes := []string{"REAL", "DOUBLE PRECISION", "FLOAT"}
	upperSQL := strings.ToUpper(combinedUp)
	for _, ft := range forbiddenTypes {
		if strings.Contains(upperSQL, ft) {
			t.Fatalf("migration contains forbidden floating-point SQL type %q", ft)
		}
	}

	if !strings.Contains(combinedUp, "fk_treasury_accounts_single_currency") {
		t.Fatal("expected single-currency-per-treasury foreign key constraint")
	}
	if !strings.Contains(combinedUp, "quanta_prevent_append_only_mutation") {
		t.Fatal("expected append-only immutability trigger function")
	}
}
