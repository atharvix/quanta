package database

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/atharvix/quanta/apps/backend/migrations"
)

func TestMigrations_EmbeddedDiscovery(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("failed to read embedded migrations: %v", err)
	}

	foundMap := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			foundMap[entry.Name()] = true
		}
	}

	expectedMigrations := []string{
		"001_step5_tables.sql",
		"002_users.sql",
		"003_treasury_owner.sql",
	}

	for _, expected := range expectedMigrations {
		if !foundMap[expected] {
			t.Errorf("expected migration '%s' to be embedded in migrations.Files", expected)
		}
	}

	// Verify content of 003_treasury_owner.sql
	content, err := fs.ReadFile(migrations.Files, "003_treasury_owner.sql")
	if err != nil {
		t.Fatalf("failed to read 003_treasury_owner.sql: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "owner_id UUID REFERENCES users(id)") {
		t.Errorf("expected owner_id foreign key reference to users(id), got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "idx_corporate_treasuries_owner_id") {
		t.Errorf("expected idx_corporate_treasuries_owner_id index creation, got: %s", contentStr)
	}
}

