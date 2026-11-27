package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/router"
)

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(context.Context) error {
	return s.err
}

type stubIntelligence struct {
	err error
}

func (s stubIntelligence) Health(context.Context) error {
	return s.err
}

func TestLiveness_Returns200WithCorrelationID(t *testing.T) {
	h := handlers.NewHealthHandler(stubPinger{}, stubIntelligence{})
	r := router.NewRouter(&config.Config{}, router.Dependencies{Health: h})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(middleware.CorrelationIDHeader, "corr-liveness-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get(middleware.CorrelationIDHeader); got != "corr-liveness-1" {
		t.Fatalf("expected header corr-liveness-1, got %q", got)
	}
}

func TestReadiness_AllHealthy(t *testing.T) {
	h := handlers.NewHealthHandler(stubPinger{}, stubIntelligence{})
	r := router.NewRouter(&config.Config{}, router.Dependencies{Health: h})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
}

func TestReadiness_DegradedWhenIntelligenceUnavailable(t *testing.T) {
	h := handlers.NewHealthHandler(stubPinger{}, stubIntelligence{err: errors.New("connection refused")})
	r := router.NewRouter(&config.Config{}, router.Dependencies{Health: h})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 degraded, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %v", body["status"])
	}
}

func TestReadiness_503StructuredErrorWhenPostgresUnavailable(t *testing.T) {
	h := handlers.NewHealthHandler(stubPinger{err: errors.New("dial tcp 127.0.0.1:5432: connectex: refused")}, stubIntelligence{})
	r := router.NewRouter(&config.Config{}, router.Dependencies{Health: h})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set(middleware.CorrelationIDHeader, "corr-db-down")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var envelope handlers.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal error envelope: %v", err)
	}
	if envelope.Error.Code != "SERVICE_UNAVAILABLE" {
		t.Fatalf("expected code SERVICE_UNAVAILABLE, got %q", envelope.Error.Code)
	}
	if envelope.Error.CorrelationID != "corr-db-down" {
		t.Fatalf("expected correlation_id corr-db-down, got %q", envelope.Error.CorrelationID)
	}
	if envelope.Error.Details == nil {
		t.Fatal("expected structured details in error envelope")
	}
}
