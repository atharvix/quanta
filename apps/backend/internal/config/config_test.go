package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("BACKEND_PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("AI_ENGINE_URL")
	os.Unsetenv("OPTIMIZATION_ENGINE_URL")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_TTL")
	os.Unsetenv("CORS_ALLOWED_ORIGINS")
	os.Unsetenv("ALLOWED_ORIGINS")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("GO_ENV")
	os.Unsetenv("ENVIRONMENT")

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
	if cfg.JWTSecret != "dev-only-change-me-please" {
		t.Errorf("expected default JWTSecret 'dev-only-change-me-please', got '%s'", cfg.JWTSecret)
	}
	if !cfg.IsDefaultJWTSecret() {
		t.Errorf("expected IsDefaultJWTSecret to be true for default secret")
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("expected default CORSAllowedOrigins ['http://localhost:3000'], got %v", cfg.CORSAllowedOrigins)
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

func TestConfig_Validate(t *testing.T) {
	t.Run("Valid dev config passes", func(t *testing.T) {
		cfg := &Config{
			Environment: "development",
			Port:        "8080",
			DatabaseURL: "postgres://localhost/quanta",
			JWTSecret:   "dev-only-change-me-please",
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid config, got: %v", err)
		}
	})

	t.Run("Production with default secret rejected", func(t *testing.T) {
		cfg := &Config{
			Environment: "production",
			Port:        "8080",
			DatabaseURL: "postgres://localhost/quanta",
			JWTSecret:   "dev-only-change-me-please",
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected production config with default secret to fail validation")
		}
	})

	t.Run("Production with short secret rejected", func(t *testing.T) {
		cfg := &Config{
			Environment: "production",
			Port:        "8080",
			DatabaseURL: "postgres://localhost/quanta",
			JWTSecret:   "short-secret-less-than-32-chars", // 31 chars
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected production config with short secret to fail validation")
		}
	})

	t.Run("Valid production config passes", func(t *testing.T) {
		cfg := &Config{
			Environment: "production",
			Port:        "8080",
			DatabaseURL: "postgres://localhost/quanta",
			JWTSecret:   "this-is-a-valid-production-secret-with-32-chars", // >= 32 chars
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid production config to pass, got: %v", err)
		}
	})
}

func TestLoadEnvironmentFallback(t *testing.T) {
	t.Run("Falls back to ENVIRONMENT when APP_ENV and GO_ENV are empty", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		t.Setenv("GO_ENV", "")
		t.Setenv("ENVIRONMENT", "staging")

		cfg := Load()
		if cfg.Environment != "staging" {
			t.Errorf("expected Environment 'staging', got '%s'", cfg.Environment)
		}
	})

	t.Run("APP_ENV and GO_ENV take precedence over ENVIRONMENT", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		t.Setenv("GO_ENV", "production")
		t.Setenv("ENVIRONMENT", "development")

		cfg := Load()
		if cfg.Environment != "production" {
			t.Errorf("expected GO_ENV 'production' to take precedence, got '%s'", cfg.Environment)
		}

		t.Setenv("APP_ENV", "staging")
		cfg = Load()
		if cfg.Environment != "staging" {
			t.Errorf("expected APP_ENV 'staging' to take precedence, got '%s'", cfg.Environment)
		}
	})
}
