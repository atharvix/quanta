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

	// ErrUpstreamUnavailable is returned when an upstream engine (AI or Optimization) is unreachable or times out.
	ErrUpstreamUnavailable = errors.New("upstream service unavailable")

	// ErrUnauthorized is returned when authentication is missing, invalid, or expired.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when the authenticated user does not have sufficient role privileges.
	ErrForbidden = errors.New("forbidden")

	// ErrInvalidCredentials is returned when login authentication fails.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrEmailAlreadyExists is returned when registering with an already existing email.
	ErrEmailAlreadyExists = errors.New("email already registered")

	// ErrCannotDemoteOnlyAdmin is returned when an admin attempts to demote the sole admin in the system.
	ErrCannotDemoteOnlyAdmin = errors.New("cannot demote the only admin")

	// ErrInvalidRole is returned when an unknown role is supplied.
	ErrInvalidRole = errors.New("invalid role")
)

// ValidationError represents one or more business validation failures with field-level details.
type ValidationError struct {
	Details []string `json:"details"`
}

func (v *ValidationError) Error() string {
	return strings.Join(v.Details, "; ")
}
