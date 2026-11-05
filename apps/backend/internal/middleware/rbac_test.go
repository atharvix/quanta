package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func TestRequireRole(t *testing.T) {
	dummyNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authorized"))
	})

	t.Run("Unauthenticated request (missing role in context) returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		mw := RequireRole(domain.RoleViewer, domain.RoleAdmin)
		mw(dummyNext).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Viewer role forbidden when analyst role required (403)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", nil)
		ctx := domain.WithUserContext(context.Background(), "u-viewer", "viewer@quanta.local", domain.RoleViewer)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		mw := RequireRole(domain.RoleAnalyst, domain.RoleAdmin)
		mw(dummyNext).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer on analyst route, got %d", rec.Code)
		}
	})

	t.Run("Analyst role forbidden when admin role required (403)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
		ctx := domain.WithUserContext(context.Background(), "u-analyst", "analyst@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		mw := RequireRole(domain.RoleAdmin)
		mw(dummyNext).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for analyst on admin route, got %d", rec.Code)
		}
	})

	t.Run("Admin role succeeds on admin route (200)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
		ctx := domain.WithUserContext(context.Background(), "u-admin", "admin@quanta.local", domain.RoleAdmin)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		mw := RequireRole(domain.RoleAdmin)
		mw(dummyNext).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin, got %d", rec.Code)
		}
	})

	t.Run("Analyst role succeeds on analyst route (200)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", nil)
		ctx := domain.WithUserContext(context.Background(), "u-analyst", "analyst@quanta.local", domain.RoleAnalyst)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		mw := RequireRole(domain.RoleAnalyst, domain.RoleAdmin)
		mw(dummyNext).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK for analyst on analyst route, got %d", rec.Code)
		}
	})
}
