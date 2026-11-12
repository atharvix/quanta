package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type MockAuditRepository struct {
	createFn func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error)
	listFn   func(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error)
}

func (m *MockAuditRepository) Create(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
	if m.createFn != nil {
		return m.createFn(ctx, log)
	}
	return nil, nil
}

func (m *MockAuditRepository) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset, entityType, action)
	}
	return nil, nil
}

func TestAuditService_Record_BestEffort(t *testing.T) {
	ctx := context.Background()

	t.Run("Best effort does not fail when repo returns error", func(t *testing.T) {
		repo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				return nil, errors.New("db disk failure")
			},
		}

		svc := NewAuditService(repo)
		entityID := "123"
		// Should not panic or return error
		svc.Record(ctx, "test.action", "test_entity", &entityID, map[string]string{"foo": "bar"})
	})

	t.Run("Successful record", func(t *testing.T) {
		var capturedAction string
		repo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				capturedAction = log.Action
				return &log, nil
			},
		}

		svc := NewAuditService(repo)
		svc.Record(ctx, "test.success", "test_entity", nil, nil)
		if capturedAction != "test.success" {
			t.Errorf("expected test.success, got %s", capturedAction)
		}
	})

	t.Run("Actor extracted from context", func(t *testing.T) {
		var capturedActor string
		repo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				capturedActor = log.Actor
				return &log, nil
			},
		}

		svc := NewAuditService(repo)
		authCtx := domain.WithUserContext(context.Background(), "u-1", "analyst@quanta.local", domain.RoleAnalyst)
		svc.Record(authCtx, "test.actor", "test_entity", nil, nil)
		if capturedActor != "analyst@quanta.local" {
			t.Errorf("expected analyst@quanta.local, got %s", capturedActor)
		}

		svc.Record(context.Background(), "test.anon", "test_entity", nil, nil)
		if capturedActor != "anonymous" {
			t.Errorf("expected anonymous, got %s", capturedActor)
		}
	})
}

func TestAuditService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid pagination and filters", func(t *testing.T) {
		repo := &MockAuditRepository{
			listFn: func(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
				return []domain.AuditLog{
					{ID: "audit-1", Action: action, EntityType: entityType, CreatedAt: time.Now()},
				}, nil
			},
		}

		svc := NewAuditService(repo)
		logs, err := svc.List(ctx, 10, 0, "treasury", "treasury.created")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 || logs[0].Action != "treasury.created" {
			t.Fatalf("unexpected log result: %+v", logs)
		}
	})

	t.Run("Invalid limit > 100 returns ErrInvalidPagination", func(t *testing.T) {
		svc := NewAuditService(nil)
		_, err := svc.List(ctx, 101, 0, "", "")
		if !errors.Is(err, domain.ErrInvalidPagination) {
			t.Fatalf("expected ErrInvalidPagination, got %v", err)
		}
	})

	t.Run("Invalid negative offset returns ErrInvalidPagination", func(t *testing.T) {
		svc := NewAuditService(nil)
		_, err := svc.List(ctx, 20, -1, "", "")
		if !errors.Is(err, domain.ErrInvalidPagination) {
			t.Fatalf("expected ErrInvalidPagination, got %v", err)
		}
	})
}
