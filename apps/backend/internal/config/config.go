// Package config manages application configuration loaded from environment variables.
// Default values allow the application to start in a local development environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds all environment-based configuration for the QUANTA backend service.
type Config struct {
	// Environment indicates deployment environment (e.g. "development", "staging", "production").
	Environment string

	// Port specifies the HTTP port the backend listens on (default: 8080).
	Port string

	// DatabaseURL specifies the PostgreSQL connection string.
	DatabaseURL string

	// AIEngineURL specifies the base URL of Member 1's AI Decision Engine (default: http://localhost:8000).
	AIEngineURL string

	// OptimizationEngineURL specifies the base URL of Member 2's Optimization Engine (default: http://localhost:8001).
	OptimizationEngineURL string

	// JWTSecret is the secret key used for signing and verifying HMAC-SHA256 JWT tokens.
	JWTSecret string

	// JWTTTL defines the expiration duration of issued JWT tokens (default: 1h).
	JWTTTL time.Duration

	// CORSAllowedOrigins specifies the list of allowed CORS origins (default: http://localhost:3000).
	CORSAllowedOrigins []string
}

// Load reads configuration from environment variables or applies sensible defaults.
func Load() *Config {
	env := getEnv("APP_ENV", "")
	if env == "" {
		env = getEnv("GO_ENV", "")
	}
	if env == "" {
		env = getEnv("ENVIRONMENT", "development")
	}

	ttlStr := getEnv("JWT_TTL", "1h")
	jwtTTL, err := time.ParseDuration(ttlStr)
	if err != nil {
		jwtTTL = 1 * time.Hour
	}

	corsOriginsStr := getEnv("CORS_ALLOWED_ORIGINS", "")
	if corsOriginsStr == "" {
		corsOriginsStr = getEnv("ALLOWED_ORIGINS", "http://localhost:3000")
	}
	var corsOrigins []string
	for _, o := range strings.Split(corsOriginsStr, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			corsOrigins = append(corsOrigins, trimmed)
		}
	}
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"http://localhost:3000"}
	}

	port := getEnv("PORT", "")
	if port == "" {
		port = getEnv("BACKEND_PORT", "8080")
	}

	return &Config{
		Environment:           strings.ToLower(env),
		Port:                  port,
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable"),
		AIEngineURL:           getEnv("AI_ENGINE_URL", "http://localhost:8000"),
		OptimizationEngineURL: getEnv("OPTIMIZATION_ENGINE_URL", "http://localhost:8001"),
		JWTSecret:             getEnv("JWT_SECRET", "dev-only-change-me-please"),
		JWTTTL:                jwtTTL,
		CORSAllowedOrigins:    corsOrigins,
	}
}

// Validate verifies configuration sanity and rejects insecure settings in production.
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT must not be empty")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	if c.Environment == "production" && c.IsDefaultJWTSecret() {
		return fmt.Errorf("insecure default JWT_SECRET is strictly forbidden in production mode")
	}
	if len(c.JWTSecret) < 32 && c.Environment == "production" {
		return fmt.Errorf("production JWT_SECRET must be at least 32 characters for HMAC-SHA256 security")
	}
	return nil
}

// IsDefaultJWTSecret returns true if the JWT secret matches the default development fallback.
func (c *Config) IsDefaultJWTSecret() bool {
	return c.JWTSecret == "dev-only-change-me-please"
}

// MaskedDatabaseURL masks the password component in the database URL so it is never exposed in logs.
func (c *Config) MaskedDatabaseURL() string {
	if c.DatabaseURL == "" {
		return ""
	}

	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil {
		// If URL parsing fails, redact everything to prevent credential leakage
		return "[REDACTED_INVALID_URL]"
	}

	if parsed.User != nil {
		if pwd, hasPassword := parsed.User.Password(); hasPassword && pwd != "" {
			uStr := parsed.String()
			// Replace URL-encoded password with mask
			maskedStr := strings.Replace(uStr, ":"+parsed.User.String()+"@", ":"+parsed.User.Username()+":*****@", 1)
			if maskedStr != uStr {
				return maskedStr
			}
			return strings.Replace(uStr, ":"+pwd+"@", ":*****@", 1)
		}
	}

	return parsed.String()
}

// String returns a safe, log-friendly representation of the configuration.
func (c *Config) String() string {
	jwtStatus := "custom"
	if c.IsDefaultJWTSecret() {
		jwtStatus = "default-insecure"
	}
	return fmt.Sprintf("Config{Environment: %s, Port: %s, DatabaseURL: %s, AIEngineURL: %s, OptimizationEngineURL: %s, JWTSecret: [%s], JWTTTL: %s, CORSAllowedOrigins: %v}",
		c.Environment, c.Port, c.MaskedDatabaseURL(), c.AIEngineURL, c.OptimizationEngineURL, jwtStatus, c.JWTTTL, c.CORSAllowedOrigins)
}

// getEnv retrieves an environment variable or returns defaultVal if unset or empty.
func getEnv(key, defaultVal string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	return val
}
