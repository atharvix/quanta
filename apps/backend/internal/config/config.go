package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Environment     string
	Port            string
	DatabaseURL     string
	IntelligenceURL string
	JWTSecret       string
	JWTIssuer       string
	JWTAudience     string
	JWTTTL          time.Duration
}

// Load reads configuration from environment variables with non-secret defaults.
func Load() *Config {
	ttlMinutes := 60
	if raw := strings.TrimSpace(os.Getenv("JWT_TTL_MINUTES")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			ttlMinutes = parsed
		}
	}

	return &Config{
		Environment:     getEnv("APP_ENV", "development"),
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		IntelligenceURL: strings.TrimRight(getEnv("INTELLIGENCE_URL", "http://127.0.0.1:8001"), "/"),
		JWTSecret:       strings.TrimSpace(os.Getenv("JWT_SECRET")),
		JWTIssuer:       getEnv("JWT_ISSUER", "quanta-backend"),
		JWTAudience:     getEnv("JWT_AUDIENCE", "quanta-api"),
		JWTTTL:          time.Duration(ttlMinutes) * time.Minute,
	}
}

// Validate checks that required configuration fields are present and well-formed.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("PORT must not be empty")
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a valid postgres:// or postgresql:// URL")
	}
	if len(strings.TrimSpace(c.JWTSecret)) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" || strings.TrimSpace(c.JWTAudience) == "" {
		return fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE must not be empty")
	}
	if c.JWTTTL <= 0 {
		return fmt.Errorf("JWT_TTL must be positive")
	}
	return nil
}

// MaskedDatabaseURL returns DatabaseURL with any userinfo password redacted for safe logging.
func (c *Config) MaskedDatabaseURL() string {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return ""
	}
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return "<invalid>"
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), "REDACTED")
		}
	}
	return parsed.String()
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
