// Package config manages application configuration loaded from environment variables.
// Default values allow the application to start in a local development environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds all environment-based configuration for the QUANTA backend service.
type Config struct {
	// Port specifies the HTTP port the backend listens on (default: 8080).
	Port string

	// DatabaseURL specifies the PostgreSQL connection string.
	DatabaseURL string

	// AIEngineURL specifies the base URL of Member 1's AI Decision Engine (default: http://localhost:8000).
	AIEngineURL string

	// OptimizationEngineURL specifies the base URL of Member 2's Optimization Engine (default: http://localhost:8001).
	OptimizationEngineURL string
}

// Load reads configuration from environment variables or applies sensible defaults.
func Load() *Config {
	return &Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable"),
		AIEngineURL:           getEnv("AI_ENGINE_URL", "http://localhost:8000"),
		OptimizationEngineURL: getEnv("OPTIMIZATION_ENGINE_URL", "http://localhost:8001"),
	}
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
	return fmt.Sprintf("Config{Port: %s, DatabaseURL: %s, AIEngineURL: %s, OptimizationEngineURL: %s}",
		c.Port, c.MaskedDatabaseURL(), c.AIEngineURL, c.OptimizationEngineURL)
}

// getEnv retrieves an environment variable or returns defaultVal if unset or empty.
func getEnv(key, defaultVal string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	return val
}
