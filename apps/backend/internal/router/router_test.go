package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
)

func TestRouter_Routes(t *testing.T) {
	cfg := config.Load()
	healthHandler := handlers.NewHealthHandler(nil)
	r := NewRouter(cfg, healthHandler, nil)

	tests := []struct {
		method         string
		path           string
		expectedStatus int
	}{
		{http.MethodGet, "/", http.StatusOK},
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/api/v1/health", http.StatusServiceUnavailable}, // nil DB returns 503
		{http.MethodGet, "/non-existent", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("%s %s expected status %d, got %d", tt.method, tt.path, tt.expectedStatus, rr.Code)
			}
		})
	}
}
