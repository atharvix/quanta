package domain

import (
	"regexp"
	"strings"
	"time"
)

// Role defines authorized permission levels within the QUANTA system.
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleAnalyst Role = "analyst"
	RoleViewer  Role = "viewer"
)

// IsValid checks whether the role string is a recognized system role.
func (r Role) IsValid() bool {
	return r == RoleAdmin || r == RoleAnalyst || r == RoleViewer
}

// User represents an authenticated identity within the platform.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose password hash in JSON responses
	FullName     string    `json:"full_name"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateEmail checks if an email string adheres to basic RFC-like email formatting.
func ValidateEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

// ValidatePassword checks password requirements (minimum 8 characters).
func ValidatePassword(password string) bool {
	return len(password) >= 8
}
