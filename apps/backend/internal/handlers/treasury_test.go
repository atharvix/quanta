package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// MockRepo implements repository.TreasuryRepository for handler unit testing.
type MockRepo struct {
	listFn    func(ctx context.Context, limit, offset int) ([]domain.Treasury, error)
	getByIDFn func(ctx context.Context, id string) (*domain.Treasury, error)
	createFn  func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error)
}

func (m *MockRepo) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return nil, nil
}

func (m *MockRepo) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockRepo) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}
	return nil, nil
}

func setupTestRouter(h *TreasuryHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/treasuries", func(tr chi.Router) {
		tr.Get("/", h.List)
		tr.Post("/", h.Create)
		tr.Get("/{id}", h.GetByID)
	})
	return r
}

func TestTreasuryHandler_List(t *testing.T) {
	t.Run("Success 200 with data", func(t *testing.T) {
		mock := &MockRepo{
			listFn: func(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
				return []domain.Treasury{
					{ID: "11111111-1111-1111-1111-111111111111", Name: "Treasury 1"},
				}, nil
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries?limit=10&offset=5", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var res ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(res.Data) != 1 || res.Limit != 10 || res.Offset != 5 {
			t.Errorf("unexpected list response: %+v", res)
		}
	})

	t.Run("Limit exceeding 100 returns 400", func(t *testing.T) {
		svc := service.NewTreasuryService(&MockRepo{})
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries?limit=500", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Database unavailable returns 503", func(t *testing.T) {
		mock := &MockRepo{
			listFn: func(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
				return nil, domain.ErrDatabaseUnavailable
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", rec.Code)
		}
	})
}

func TestTreasuryHandler_GetByID(t *testing.T) {
	t.Run("Invalid UUID returns 400", func(t *testing.T) {
		svc := service.NewTreasuryService(&MockRepo{})
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/invalid-uuid", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Not found returns 404", func(t *testing.T) {
		mock := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/00000000-0000-0000-0000-000000000000", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("Database unavailable returns 503", func(t *testing.T) {
		mock := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrDatabaseUnavailable
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", rec.Code)
		}
	})

	t.Run("Success 200 with valid entity", func(t *testing.T) {
		validID := "123e4567-e89b-12d3-a456-426614174000"
		mock := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:        id,
					Name:      "Alpha Treasury",
					CreatedAt: time.Now(),
				}, nil
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/"+validID, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})
}

func TestTreasuryHandler_Create(t *testing.T) {
	t.Run("Success 201 Created", func(t *testing.T) {
		mock := &MockRepo{
			createFn: func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                "created-uuid",
					Name:              input.Name,
					BaseCurrency:      input.BaseCurrency,
					TotalCapital:      input.TotalCapital,
					RiskTolerance:     input.RiskTolerance,
					MinLiquidityRatio: input.MinLiquidityRatio,
				}, nil
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"Gamma Corp","total_capital":10000000.0}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Validation failure returns 400 with details", func(t *testing.T) {
		svc := service.NewTreasuryService(&MockRepo{})
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		// Invalid: missing name and non-positive total_capital
		body := `{"name":"","total_capital":-50}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}

		var res ValidationErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode validation error: %v", err)
		}
		if len(res.Details) == 0 {
			t.Errorf("expected details in validation response, got none")
		}
	})

	t.Run("Unknown JSON fields rejected with 400", func(t *testing.T) {
		svc := service.NewTreasuryService(&MockRepo{})
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"Corp","total_capital":1000,"hacker_field":"malicious"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for unknown fields, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "hacker_field") {
			t.Errorf("expected error message to mention unknown field, got: %s", rec.Body.String())
		}
	})

	t.Run("Database unavailable returns 503", func(t *testing.T) {
		mock := &MockRepo{
			createFn: func(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
				return nil, domain.ErrDatabaseUnavailable
			},
		}
		svc := service.NewTreasuryService(mock)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"Valid Corp","total_capital":500000}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", rec.Code)
		}
	})
}
