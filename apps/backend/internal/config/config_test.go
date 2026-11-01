package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("AI_ENGINE_URL")
	os.Unsetenv("OPTIMIZATION_ENGINE_URL")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got '%s'", cfg.Port)
	}
	if !strings.Contains(cfg.DatabaseURL, "quanta_db") {
		t.Errorf("expected default DatabaseURL to contain 'quanta_db', got '%s'", cfg.DatabaseURL)
	}
	if cfg.AIEngineURL != "http://localhost:8000" {
		t.Errorf("expected default AIEngineURL 'http://localhost:8000', got '%s'", cfg.AIEngineURL)
	}
	if cfg.OptimizationEngineURL != "http://localhost:8001" {
		t.Errorf("expected default OptimizationEngineURL 'http://localhost:8001', got '%s'", cfg.OptimizationEngineURL)
	}
}

func TestLoadCustomEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://custom_user:secret123@db.example.com:5432/custom_db?sslmode=require")
	t.Setenv("AI_ENGINE_URL", "http://ai.prod:8000")
	t.Setenv("OPTIMIZATION_ENGINE_URL", "http://opt.prod:8001")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected Port '9090', got '%s'", cfg.Port)
	}
	if cfg.AIEngineURL != "http://ai.prod:8000" {
		t.Errorf("expected AIEngineURL 'http://ai.prod:8000', got '%s'", cfg.AIEngineURL)
	}
	if cfg.OptimizationEngineURL != "http://opt.prod:8001" {
		t.Errorf("expected OptimizationEngineURL 'http://opt.prod:8001', got '%s'", cfg.OptimizationEngineURL)
	}
}

func TestMaskedDatabaseURL(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "postgres://quanta_admin:super_secret_password@localhost:5432/quanta_db?sslmode=disable",
	}

	masked := cfg.MaskedDatabaseURL()

	if strings.Contains(masked, "super_secret_password") {
		t.Errorf("MaskedDatabaseURL must not contain raw password, got: %s", masked)
	}
	if !strings.Contains(masked, "*****") {
		t.Errorf("MaskedDatabaseURL should replace password with '*****', got: %s", masked)
	}
	if !strings.Contains(masked, "quanta_admin") {
		t.Errorf("MaskedDatabaseURL should preserve username, got: %s", masked)
	}

	cfgString := cfg.String()
	if strings.Contains(cfgString, "super_secret_password") {
		t.Errorf("cfg.String() must not contain raw password, got: %s", cfgString)
	}
}
