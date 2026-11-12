package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

type dummyUserRepo struct{}

func (d *dummyUserRepo) Create(ctx context.Context, u domain.User) (*domain.User, error) {
	return nil, nil
}
func (d *dummyUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (d *dummyUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Email: "user@quanta.local", Role: domain.RoleViewer}, nil
}
func (d *dummyUserRepo) Count(ctx context.Context) (int, error) {
	return 1, nil
}
func (d *dummyUserRepo) List(ctx context.Context) ([]domain.User, error) {
	return []domain.User{}, nil
}
func (d *dummyUserRepo) UpdateRole(ctx context.Context, id string, role domain.Role) (*domain.User, error) {
	return &domain.User{ID: id, Role: role}, nil
}
func (d *dummyUserRepo) CountAdmins(ctx context.Context) (int, error) {
	return 1, nil
}

type dummyAuditRepo struct{}

func (d *dummyAuditRepo) Create(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
	return &log, nil
}
func (d *dummyAuditRepo) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	return []domain.AuditLog{}, nil
}

type dummyTreasuryRepo struct{}

func (d *dummyTreasuryRepo) List(ctx context.Context, limit, offset int) ([]domain.Treasury, error) {
	return []domain.Treasury{}, nil
}
func (d *dummyTreasuryRepo) GetByID(ctx context.Context, id string) (*domain.Treasury, error) {
	ownerID := "a-1"
	return &domain.Treasury{ID: id, TotalCapital: 1000000, OwnerID: &ownerID}, nil
}
func (d *dummyTreasuryRepo) Create(ctx context.Context, input domain.CreateTreasuryInput) (*domain.Treasury, error) {
	return &domain.Treasury{ID: "t-1", Name: input.Name, TotalCapital: input.TotalCapital}, nil
}

type mockAIClientForRouter struct{}

func (m *mockAIClientForRouter) Health(ctx context.Context) error { return nil }
func (m *mockAIClientForRouter) GenerateStrategy(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
	return nil, nil
}
func (m *mockAIClientForRouter) CompareSearches(ctx context.Context, problem any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockAIClientForRouter) AnalyzeRisk(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
	return &domain.AIRiskAnalyzeResponse{}, nil
}
func (m *mockAIClientForRouter) AdversarialStressTest(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
	return &domain.AdversarialStressTestResult{WorstMarketShock: "Status Quo"}, nil
}

func generateTestToken(secret, sub, email string, role domain.Role) string {
	claims := jwt.MapClaims{
		"sub":   sub,
		"email": email,
		"role":  string(role),
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}

func TestRouter_PublicAndProtectedRoutes(t *testing.T) {
	jwtSecret := "router-test-secret-1234567890"
	cfg := &config.Config{
		JWTSecret:          jwtSecret,
		JWTTTL:             time.Hour,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	authSvc := service.NewAuthService(&dummyUserRepo{}, nil, cfg)
	authMW := middleware.JWTAuth(authSvc)
	authHandler := handlers.NewAuthHandler(authSvc)
	healthHandler := handlers.NewHealthHandler(nil, nil)

	r := NewRouter(
		cfg,
		healthHandler,
		nil, // treasuryHandler
		nil, // strategyHandler
		nil, // fpHandler
		nil, // orchHandler
		nil, // auditHandler
		authHandler,
		nil, // riskHandler
		authMW,
	)

	t.Run("Public routes accessible without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for /, got %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/health", nil)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for /health, got %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503 for /api/v1/health with nil DB, got %d", rr.Code)
		}
	})

	t.Run("Protected /auth/me without token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for /auth/me without token, got %d", rr.Code)
		}
	})

	t.Run("Protected /auth/me with valid token returns 200", func(t *testing.T) {
		token := generateTestToken(jwtSecret, "user-1", "user@quanta.local", domain.RoleViewer)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for /auth/me with token, got %d", rr.Code)
		}
	})
}

func TestRouter_RBACMatrix(t *testing.T) {
	jwtSecret := "rbac-matrix-test-secret"
	cfg := &config.Config{
		JWTSecret:          jwtSecret,
		JWTTTL:             time.Hour,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	auditSvc := service.NewAuditService(&dummyAuditRepo{})
	auditHandler := handlers.NewAuditHandler(auditSvc)

	treasurySvc := service.NewTreasuryService(&dummyTreasuryRepo{}, auditSvc)
	treasuryHandler := handlers.NewTreasuryHandler(treasurySvc)

	riskSvc := service.NewRiskService(&dummyTreasuryRepo{}, &mockAIClientForRouter{}, auditSvc)
	riskHandler := handlers.NewRiskHandler(riskSvc)

	authSvc := service.NewAuthService(&dummyUserRepo{}, auditSvc, cfg)
	authMW := middleware.JWTAuth(authSvc)
	authHandler := handlers.NewAuthHandler(authSvc)

	r := NewRouter(
		cfg,
		nil,
		treasuryHandler,
		nil,
		nil,
		nil,
		auditHandler,
		authHandler,
		riskHandler,
		authMW,
	)

	viewerToken := generateTestToken(jwtSecret, "v-1", "viewer@quanta.local", domain.RoleViewer)
	analystToken := generateTestToken(jwtSecret, "a-1", "analyst@quanta.local", domain.RoleAnalyst)
	adminToken := generateTestToken(jwtSecret, "adm-1", "admin@quanta.local", domain.RoleAdmin)

	treasuryPayload := `{"name":"Acme Corp","base_currency":"USD","total_capital":5000000,"risk_tolerance":0.5,"min_liquidity_ratio":0.2}`

	t.Run("Bina token returns 401 on protected GET and POST endpoints", func(t *testing.T) {
		// GET treasuries without token
		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for GET /treasuries without token, got %d", rr.Code)
		}

		// POST treasuries without token
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(treasuryPayload))
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for POST /treasuries without token, got %d", rr.Code)
		}

		// GET audit-logs without token
		req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for GET /audit-logs without token, got %d", rr.Code)
		}

		// POST risk analyze without token
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/risk/analyze", bytes.NewBufferString(`{}`))
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for POST /risk/analyze without token, got %d", rr.Code)
		}

		// POST stress test without token
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/stress-test", bytes.NewBufferString(`{}`))
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for POST /stress-test without token, got %d", rr.Code)
		}
	})

	t.Run("Viewer permissions: GET treasuries 200, POST treasuries 403, GET audit-logs 403", func(t *testing.T) {
		// GET treasuries allowed for viewer (200)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries", nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK for viewer on GET /treasuries, got %d", rr.Code)
		}

		// POST treasuries forbidden for viewer (403)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(treasuryPayload))
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer on POST /treasuries, got %d", rr.Code)
		}

		// POST risk analyze forbidden for viewer (403)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/risk/analyze", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer on POST /risk/analyze, got %d", rr.Code)
		}

		// POST stress test forbidden for viewer (403)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/stress-test", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer on POST /stress-test, got %d", rr.Code)
		}

		// GET audit-logs forbidden for viewer (403)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer on GET /audit-logs, got %d", rr.Code)
		}
	})

	t.Run("Analyst permissions: GET treasuries 200, POST treasuries 201, GET audit-logs 403", func(t *testing.T) {
		// GET treasuries allowed for analyst (200)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries", nil)
		req.Header.Set("Authorization", "Bearer "+analystToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK for analyst on GET /treasuries, got %d", rr.Code)
		}

		// POST treasuries allowed for analyst (201)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(treasuryPayload))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201 Created for analyst on POST /treasuries, got %d", rr.Code)
		}

		// POST risk analyze allowed for analyst (reaches handler, passes RBAC)
		riskPayload := `{"assets":[{"id":"gov_bond","name":"Gov Bond","asset_type":"gov_bond","cost":0.001,"expected_return":0.07,"risk":0.05,"liquidity":0.95}]}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/risk/analyze", bytes.NewBufferString(riskPayload))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
			t.Errorf("expected analyst to be authorized for POST /risk/analyze, got %d", rr.Code)
		}

		// POST stress test allowed for analyst (reaches handler, passes RBAC)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/stress-test", bytes.NewBufferString(riskPayload))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
			t.Errorf("expected analyst to be authorized for POST /stress-test, got %d", rr.Code)
		}

		// GET audit-logs forbidden for analyst (403)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
		req.Header.Set("Authorization", "Bearer "+analystToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for analyst on GET /audit-logs, got %d", rr.Code)
		}
	})

	t.Run("Admin permissions: GET audit-logs 200, GET users 200, POST treasuries 201", func(t *testing.T) {
		// GET audit-logs allowed for admin (200)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin on GET /audit-logs, got %d", rr.Code)
		}

		// GET users allowed for admin (200)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin on GET /users, got %d", rr.Code)
		}

		// POST treasuries allowed for admin (201)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries", bytes.NewBufferString(treasuryPayload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201 Created for admin on POST /treasuries, got %d", rr.Code)
		}

		// POST risk analyze allowed for admin
		riskPayload := `{"assets":[{"id":"gov_bond","name":"Gov Bond","asset_type":"gov_bond","cost":0.001,"expected_return":0.07,"risk":0.05,"liquidity":0.95}]}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/risk/analyze", bytes.NewBufferString(riskPayload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
			t.Errorf("expected admin to be authorized for POST /risk/analyze, got %d", rr.Code)
		}

		// POST stress test allowed for admin
		req = httptest.NewRequest(http.MethodPost, "/api/v1/treasuries/123e4567-e89b-12d3-a456-426614174000/stress-test", bytes.NewBufferString(riskPayload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
			t.Errorf("expected admin to be authorized for POST /stress-test, got %d", rr.Code)
		}
	})

	t.Run("CORS preflight on router returns 204 without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/treasuries", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for preflight OPTIONS, got %d", rr.Code)
		}
		if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})
}
