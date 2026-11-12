package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// MockTreasuryRepository implements repository.TreasuryRepository for unit testing.
type MockTreasuryRepository struct {
	listFn    func(ctx context.Context, limit, offset int) ([]domain.Treasury, error)
	getByIDFn func(ctx context.Context, id string) (*domain.Treasury, error)
	createFn  func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error)
}

func (m *MockTreasuryRepository) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return nil, nil
}

func (m *MockTreasuryRepository) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockTreasuryRepository) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}
	return nil, nil
}

func TestTreasuryService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid pagination with default limit", func(t *testing.T) {
		mock := &MockTreasuryRepository{
			listFn: func(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
				if limit != 20 {
					t.Errorf("expected default limit 20, got %d", limit)
				}
				if offset != 0 {
					t.Errorf("expected offset 0, got %d", offset)
				}
				return []domain.Treasury{{ID: "uuid-1", Name: "Corp 1"}}, nil
			},
		}
		s := NewTreasuryService(mock)
		res, err := s.List(ctx, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
	})

	t.Run("Limit exceeding 100 rejected", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		_, err := s.List(ctx, 500, 0)
		if err == nil || !errors.Is(err, domain.ErrInvalidPagination) {
			t.Fatalf("expected ErrInvalidPagination for limit 500, got %v", err)
		}
	})

	t.Run("Negative offset rejected", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		_, err := s.List(ctx, 20, -5)
		if err == nil || !errors.Is(err, domain.ErrInvalidPagination) {
			t.Fatalf("expected ErrInvalidPagination for negative offset, got %v", err)
		}
	})
}

func TestTreasuryService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid UUID format rejected", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		_, err := s.GetByID(ctx, "not-a-valid-uuid")
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("expected ErrInvalidID, got: %v", err)
		}
	})

	t.Run("Valid UUID format succeeds", func(t *testing.T) {
		validUUID := "123e4567-e89b-12d3-a456-426614174000"
		mock := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:        id,
					Name:      "Test Corp",
					CreatedAt: time.Now(),
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		tRecord, err := s.GetByID(ctx, validUUID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tRecord.ID != validUUID {
			t.Errorf("expected ID '%s', got '%s'", validUUID, tRecord.ID)
		}
	})
}

func TestTreasuryService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("Validation failure on empty name", func(t *testing.T) {
		s := NewTreasuryService(&MockTreasuryRepository{})
		input := domain.CreateTreasuryInput{
			Name:         "",
			TotalCapital: 10000.0,
		}
		_, err := s.Create(ctx, input)
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
	})

	t.Run("Successful creation with defaults applied", func(t *testing.T) {
		mock := &MockTreasuryRepository{
			createFn: func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
				if input.BaseCurrency != "INR" {
					t.Errorf("expected default BaseCurrency INR, got %s", input.BaseCurrency)
				}
				return &domain.Treasury{
					ID:           "test-id",
					Name:         input.Name,
					BaseCurrency: input.BaseCurrency,
					TotalCapital: input.TotalCapital,
				}, nil
			},
		}
		s := NewTreasuryService(mock)
		input := domain.CreateTreasuryInput{
			Name:         "Valid Corp",
			TotalCapital: 5000000.0,
		}
		created, err := s.Create(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.Name != "Valid Corp" {
			t.Errorf("expected Name 'Valid Corp', got '%s'", created.Name)
		}
	})
}
