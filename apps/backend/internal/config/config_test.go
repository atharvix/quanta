package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "postgres://quanta:secret123@127.0.0.1:5432/quanta_db?sslmode=disable")
	t.Setenv("INTELLIGENCE_URL", "http://127.0.0.1:8001/")
	t.Setenv("JWT_SECRET", "test-only-32-byte-minimum-secret-key")

	cfg := Load()
	if cfg.Environment != "development" {
		t.Fatalf("expected default Environment development, got %q", cfg.Environment)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default Port 8080, got %q", cfg.Port)
	}
	if cfg.IntelligenceURL != "http://127.0.0.1:8001" {
		t.Fatalf("expected trimmed IntelligenceURL, got %q", cfg.IntelligenceURL)
	}
	if cfg.JWTTTL != 60*time.Minute {
		t.Fatalf("expected 60m TTL, got %v", cfg.JWTTTL)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestConfig_ValidateAndMask(t *testing.T) {
	cfg := &Config{
		Port:        "8080",
		DatabaseURL: "postgres://quanta:supersecret@127.0.0.1:5432/quanta_db?sslmode=disable",
		JWTSecret:   "test-only-32-byte-minimum-secret-key",
		JWTIssuer:   "quanta-backend",
		JWTAudience: "quanta-api",
		JWTTTL:      time.Hour,
	}
	masked := cfg.MaskedDatabaseURL()
	if strings.Contains(masked, "supersecret") {
		t.Fatalf("MaskedDatabaseURL leaked password: %q", masked)
	}
	if !strings.Contains(masked, "REDACTED") {
		t.Fatalf("MaskedDatabaseURL should contain REDACTED, got %q", masked)
	}

	missingDB := &Config{Port: "8080", DatabaseURL: ""}
	if err := missingDB.Validate(); err == nil {
		t.Fatal("expected Validate to fail when DatabaseURL is empty")
	}

	shortSecret := &Config{
		Port:        "8080",
		DatabaseURL: cfg.DatabaseURL,
		JWTSecret:   "too-short",
		JWTIssuer:   "quanta-backend",
		JWTAudience: "quanta-api",
		JWTTTL:      time.Hour,
	}
	if err := shortSecret.Validate(); err == nil {
		t.Fatal("expected Validate to fail when JWTSecret is shorter than 32 chars")
	}
}
