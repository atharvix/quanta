package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockPinger implements database.Pinger for testing health endpoints.
type MockPinger struct {
	err error
}

func (m *MockPinger) Ping(ctx context.Context) error {
	return m.err
}

func TestHealthHandler_Root(t *testing.T) {
	h := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.Root(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["service"] != "quanta-backend" || res["version"] != "0.1.0" {
		t.Errorf("unexpected body: %v", res)
	}
}

func TestHealthHandler_Liveness(t *testing.T) {
	h := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Liveness(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", res["status"])
	}
}

func TestHealthHandler_DatabaseHealth_NilPinger(t *testing.T) {
	// When database could not connect on startup, pinger is nil
	h := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.DatabaseHealth(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}

	var res map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "degraded" || res["database"] != "down" {
		t.Errorf("expected degraded/down response, got: %v", res)
	}
}

func TestHealthHandler_DatabaseHealth_FailingPing(t *testing.T) {
	// When database ping fails with network or timeout error
	mock := &MockPinger{err: errors.New("connection refused")}
	h := NewHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.DatabaseHealth(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}

	var res map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Internal error message must never leak
	if res["status"] != "degraded" || res["database"] != "down" {
		t.Errorf("expected degraded/down response, got: %v", res)
	}
}

func TestHealthHandler_DatabaseHealth_Success(t *testing.T) {
	mock := &MockPinger{err: nil}
	h := NewHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.DatabaseHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "ok" || res["database"] != "up" {
		t.Errorf("expected ok/up response, got: %v", res)
	}
}
