package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type MockPinger struct {
	err error
}

func (m *MockPinger) Ping(ctx context.Context) error {
	return m.err
}

type MockAIHealthChecker struct {
	err error
}

func (m *MockAIHealthChecker) Health(ctx context.Context) error {
	return m.err
}

func (m *MockAIHealthChecker) GenerateStrategy(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
	return nil, nil
}

func (m *MockAIHealthChecker) CompareSearches(ctx context.Context, problem any) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockAIHealthChecker) AnalyzeRisk(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
	return nil, nil
}

func (m *MockAIHealthChecker) AdversarialStressTest(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
	return nil, nil
}

func TestHealthHandler_Root(t *testing.T) {
	h := NewHealthHandler(nil, nil)

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
	h := NewHealthHandler(nil, nil)

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
	h := NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.DatabaseHealth(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestHealthHandler_DatabaseHealth_DBUp_AIDown(t *testing.T) {
	dbMock := &MockPinger{err: nil}
	aiMock := &MockAIHealthChecker{err: errors.New("ai engine down")}
	h := NewHealthHandler(dbMock, aiMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.DatabaseHealth(rr, req)

	// DB is primary, so overall status is 200 OK
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res struct {
		Status       string            `json:"status"`
		Database     string            `json:"database"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if res.Database != "up" || res.Dependencies["ai_engine"] != "down" {
		t.Errorf("unexpected health status payload: %+v", res)
	}
}

func TestHealthHandler_DatabaseHealth_AllUp(t *testing.T) {
	dbMock := &MockPinger{err: nil}
	aiMock := &MockAIHealthChecker{err: nil}
	h := NewHealthHandler(dbMock, aiMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.DatabaseHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res struct {
		Status       string            `json:"status"`
		Database     string            `json:"database"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if res.Database != "up" || res.Dependencies["ai_engine"] != "up" {
		t.Errorf("unexpected health status payload: %+v", res)
	}
}
