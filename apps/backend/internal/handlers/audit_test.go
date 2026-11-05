package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type mockAuditRepo struct {
	listFn func(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error)
}

func (m *mockAuditRepo) Create(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
	return &log, nil
}

func (m *mockAuditRepo) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset, entityType, action)
	}
	return []domain.AuditLog{}, nil
}

func TestAuditHandler_List(t *testing.T) {
	t.Run("Success 200 with data", func(t *testing.T) {
		repo := &mockAuditRepo{
			listFn: func(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
				return []domain.AuditLog{
					{ID: "audit-1", Action: "treasury.created", CreatedAt: time.Now()},
				}, nil
			},
		}
		svc := service.NewAuditService(repo)
		h := NewAuditHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?limit=10&offset=0", nil)
		rec := httptest.NewRecorder()
		h.List(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp ListAuditLogsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].ID != "audit-1" {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("Invalid limit parameter returns 400", func(t *testing.T) {
		h := NewAuditHandler(service.NewAuditService(nil))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?limit=abc", nil)
		rec := httptest.NewRecorder()
		h.List(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}
