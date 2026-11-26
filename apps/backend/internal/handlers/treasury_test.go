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
	updateFn  func(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error)
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

func (m *MockRepo) Update(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}
	return nil, nil
}

func setupTestRouter(h *TreasuryHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/treasuries", func(tr chi.Router) {
		tr.Get("/", h.List)
		tr.Post("/", h.Create)
		tr.Get("/{id}", h.GetByID)
		tr.Patch("/{id}", h.Update)
		tr.Get("/{id}/assets", h.ListAssets)
		tr.Get("/{id}/assets/{assetId}", h.GetAsset)
		tr.Post("/{id}/assets", h.CreateAsset)
		tr.Put("/{id}/assets/{assetId}", h.UpdateAsset)
		tr.Delete("/{id}/assets/{assetId}", h.DeleteAsset)
		tr.Put("/{id}/assets", h.BulkSyncAssets)
		tr.Get("/{id}/liabilities", h.ListLiabilities)
		tr.Get("/{id}/liabilities/{liabilityId}", h.GetLiability)
		tr.Post("/{id}/liabilities", h.CreateLiability)
		tr.Put("/{id}/liabilities/{liabilityId}", h.UpdateLiability)
		tr.Delete("/{id}/liabilities/{liabilityId}", h.DeleteLiability)
		tr.Put("/{id}/liabilities", h.BulkSyncLiabilities)
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

type mockFinancialEntryRepo struct {
	listAssetsFn      func(ctx context.Context, treasuryID string) ([]domain.Asset, error)
	createAssetFn     func(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error)
	deleteAssetFn     func(ctx context.Context, treasuryID, assetID string) error
	listLiabilitiesFn func(ctx context.Context, treasuryID string) ([]domain.Liability, error)
	createLiabilityFn func(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error)
	deleteLiabilityFn func(ctx context.Context, treasuryID, liabilityID string) error
}

func (m *mockFinancialEntryRepo) ListAssets(ctx context.Context, treasuryID string) ([]domain.Asset, error) {
	if m.listAssetsFn != nil {
		return m.listAssetsFn(ctx, treasuryID)
	}
	return []domain.Asset{}, nil
}
func (m *mockFinancialEntryRepo) GetAsset(ctx context.Context, treasuryID, assetID string) (*domain.Asset, error) {
	return nil, nil
}
func (m *mockFinancialEntryRepo) CreateAsset(ctx context.Context, treasuryID string, asset domain.Asset) (*domain.Asset, error) {
	if m.createAssetFn != nil {
		return m.createAssetFn(ctx, treasuryID, asset)
	}
	return &asset, nil
}
func (m *mockFinancialEntryRepo) UpdateAsset(ctx context.Context, treasuryID, assetID string, asset domain.Asset) (*domain.Asset, error) {
	return &asset, nil
}
func (m *mockFinancialEntryRepo) DeleteAsset(ctx context.Context, treasuryID, assetID string) error {
	if m.deleteAssetFn != nil {
		return m.deleteAssetFn(ctx, treasuryID, assetID)
	}
	return nil
}
func (m *mockFinancialEntryRepo) BulkSyncAssets(ctx context.Context, treasuryID string, assets []domain.Asset) ([]domain.Asset, error) {
	return assets, nil
}
func (m *mockFinancialEntryRepo) ListLiabilities(ctx context.Context, treasuryID string) ([]domain.Liability, error) {
	if m.listLiabilitiesFn != nil {
		return m.listLiabilitiesFn(ctx, treasuryID)
	}
	return []domain.Liability{}, nil
}
func (m *mockFinancialEntryRepo) GetLiability(ctx context.Context, treasuryID, liabilityID string) (*domain.Liability, error) {
	return nil, nil
}
func (m *mockFinancialEntryRepo) CreateLiability(ctx context.Context, treasuryID string, liability domain.Liability) (*domain.Liability, error) {
	if m.createLiabilityFn != nil {
		return m.createLiabilityFn(ctx, treasuryID, liability)
	}
	return &liability, nil
}
func (m *mockFinancialEntryRepo) UpdateLiability(ctx context.Context, treasuryID, liabilityID string, liability domain.Liability) (*domain.Liability, error) {
	return &liability, nil
}
func (m *mockFinancialEntryRepo) DeleteLiability(ctx context.Context, treasuryID, liabilityID string) error {
	if m.deleteLiabilityFn != nil {
		return m.deleteLiabilityFn(ctx, treasuryID, liabilityID)
	}
	return nil
}
func (m *mockFinancialEntryRepo) BulkSyncLiabilities(ctx context.Context, treasuryID string, liabilities []domain.Liability) ([]domain.Liability, error) {
	return liabilities, nil
}

func TestTreasuryHandler_AssetsAndLiabilities(t *testing.T) {
	const validUUID = "550e8400-e29b-41d4-a716-446655440000"
	mockTreasury := &MockRepo{
		getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
			return &domain.Treasury{ID: validUUID, Name: "Corp 1"}, nil
		},
	}

	t.Run("ListAssets returns 200 with data list", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{
			listAssetsFn: func(ctx context.Context, treasuryID string) ([]domain.Asset, error) {
				return []domain.Asset{
					{ID: "gov_bond", Name: "Gov Bonds", AssetType: "gov_bond", ExpectedReturn: 0.07, Risk: 0.05, Liquidity: 0.95},
				}, nil
			},
		}

		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/"+validUUID+"/assets", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp ListAssetsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].ID != "gov_bond" {
			t.Errorf("expected 1 asset with ID 'gov_bond', got %+v", resp.Data)
		}
	})

	t.Run("CreateAsset returns 201 on valid input", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{}
		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"id":"corp_bond","name":"Corporate Bond","asset_type":"corp_bond","cost":0.001,"expected_return":0.08,"risk":0.06,"liquidity":0.8,"volatility":0.05}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validUUID+"/assets", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateAsset returns 400 on invalid asset_type enum", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{}
		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		// Testing legacy invalid enum 'bond'
		body := `{"id":"legacy_bond","name":"Legacy Bond","asset_type":"bond","cost":0.001,"expected_return":0.08,"risk":0.06,"liquidity":0.8,"volatility":0.05}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validUUID+"/assets", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "allowed values: [cash, gov_bond, corp_bond, equity, real_estate, commodity, other]") {
			t.Errorf("expected canonical enum list in error details, got: %s", rec.Body.String())
		}
	})

	t.Run("DeleteAsset returns 200", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{}
		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/treasuries/"+validUUID+"/assets/corp_bond", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ListLiabilities returns 200 with data list", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{
			listLiabilitiesFn: func(ctx context.Context, treasuryID string) ([]domain.Liability, error) {
				return []domain.Liability{
					{ID: "loan_1", Name: "Term Loan", Principal: 10.0, InterestRate: 0.05, Priority: 1},
				}, nil
			},
		}

		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries/"+validUUID+"/liabilities", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp ListLiabilitiesResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].ID != "loan_1" {
			t.Errorf("expected 1 liability with ID 'loan_1', got %+v", resp.Data)
		}
	})

	t.Run("CreateLiability returns 201 on valid input", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{}
		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"id":"revolving_credit","name":"Revolving Credit","principal":15.0,"interest_rate":0.06,"priority":1,"is_mandatory_reserve":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/"+validUUID+"/liabilities", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteLiability returns 200", func(t *testing.T) {
		mockEntry := &mockFinancialEntryRepo{}
		svc := service.NewTreasuryService(mockTreasury)
		svc.SetEntryRepository(mockEntry)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/treasuries/"+validUUID+"/liabilities/revolving_credit", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestTreasuryHandler_Update(t *testing.T) {
	const validUUID = "550e8400-e29b-41d4-a716-446655440000"
	ownerID := "user-owner"

	existing := domain.Treasury{
		ID:                validUUID,
		Name:              "Alpha Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      50000000.0,
		RiskTolerance:     0.5,
		MinLiquidityRatio: 0.2,
		OwnerID:           &ownerID,
	}

	t.Run("Valid PATCH update returns 200 with updated treasury", func(t *testing.T) {
		mockRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existing, nil
			},
			updateFn: func(ctx context.Context, id string, input domain.UpdateTreasuryInput) (*domain.Treasury, error) {
				updated := existing
				if input.Name != nil {
					updated.Name = *input.Name
				}
				return &updated, nil
			},
		}

		svc := service.NewTreasuryService(mockRepo)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"Beta Global Treasury"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/treasuries/"+validUUID, bytes.NewBufferString(body))
		req = req.WithContext(domain.WithUserContext(req.Context(), ownerID, "owner@quanta.local", domain.RoleAnalyst))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var res domain.Treasury
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.Name != "Beta Global Treasury" {
			t.Errorf("expected updated name 'Beta Global Treasury', got '%s'", res.Name)
		}
	})

	t.Run("Unknown field in PATCH body rejected with 400", func(t *testing.T) {
		svc := service.NewTreasuryService(&MockRepo{})
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"New Name","malicious_field":"injected"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/treasuries/"+validUUID, bytes.NewBufferString(body))
		req = req.WithContext(domain.WithUserContext(req.Context(), ownerID, "owner@quanta.local", domain.RoleAnalyst))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for unknown field, got %d", rec.Code)
		}
	})

	t.Run("Invalid validation in PATCH returns 400 with details", func(t *testing.T) {
		mockRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existing, nil
			},
		}
		svc := service.NewTreasuryService(mockRepo)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":""}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/treasuries/"+validUUID, bytes.NewBufferString(body))
		req = req.WithContext(domain.WithUserContext(req.Context(), ownerID, "owner@quanta.local", domain.RoleAnalyst))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for empty name, got %d", rec.Code)
		}
	})

	t.Run("Non-owner analyst returns 403 Forbidden", func(t *testing.T) {
		mockRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &existing, nil
			},
		}
		svc := service.NewTreasuryService(mockRepo)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"Hacked Name"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/treasuries/"+validUUID, bytes.NewBufferString(body))
		req = req.WithContext(domain.WithUserContext(req.Context(), "attacker-456", "attacker@quanta.local", domain.RoleAnalyst))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403 for non-owner, got %d", rec.Code)
		}
	})

	t.Run("Missing treasury returns 404", func(t *testing.T) {
		mockRepo := &MockRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}
		svc := service.NewTreasuryService(mockRepo)
		h := NewTreasuryHandler(svc)
		r := setupTestRouter(h)

		body := `{"name":"New Name"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/treasuries/"+validUUID, bytes.NewBufferString(body))
		req = req.WithContext(domain.WithUserContext(req.Context(), ownerID, "owner@quanta.local", domain.RoleAnalyst))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404 for missing treasury, got %d", rec.Code)
		}
	})
}
