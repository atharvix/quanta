// Package service provides business logic orchestration, validation, and domain rules.
package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// uuidRegex validates standard RFC 4122 canonical UUID format.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// TreasuryService defines business operations on corporate treasuries.
type TreasuryService struct {
	repo repository.TreasuryRepository
}

// NewTreasuryService constructs a new service instance wrapping the repository layer.
func NewTreasuryService(repo repository.TreasuryRepository) *TreasuryService {
	return &TreasuryService{repo: repo}
}

// List validates pagination arguments and queries treasuries.
// Limit defaults to 20, cannot exceed 100. Offset must be >= 0.
func (s *TreasuryService) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, fmt.Errorf("%w: limit must not exceed 100", domain.ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", domain.ErrInvalidPagination)
	}

	return s.repo.List(ctx, limit, offset)
}

// GetByID validates the UUID format before querying the repository.
func (s *TreasuryService) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	if !uuidRegex.MatchString(id) {
		return nil, domain.ErrInvalidID
	}

	return s.repo.GetByID(ctx, id)
}

// Create applies domain defaults, validates input invariants, and creates the treasury.
func (s *TreasuryService) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	input.ApplyDefaults()

	if errs := input.Validate(); len(errs) > 0 {
		return nil, &domain.ValidationError{Details: errs}
	}

	return s.repo.Create(ctx, input)
}
