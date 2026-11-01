package domain

import (
	"errors"
	"strings"
)

var (
	// ErrNotFound is returned when a requested record does not exist in the database.
	ErrNotFound = errors.New("resource not found")

	// ErrDatabaseUnavailable is returned when PostgreSQL is disconnected or unreachable.
	ErrDatabaseUnavailable = errors.New("database unavailable")

	// ErrInvalidID is returned when an ID parameter is not a valid UUID format.
	ErrInvalidID = errors.New("invalid id format")

	// ErrInvalidPagination is returned when limit or offset values violate bounds.
	ErrInvalidPagination = errors.New("invalid pagination parameters")
)

// ValidationError represents one or more business validation failures with field-level details.
type ValidationError struct {
	Details []string `json:"details"`
}

func (v *ValidationError) Error() string {
	return strings.Join(v.Details, "; ")
}
