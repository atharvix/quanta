//go:build integration

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

func setupIntegrationEnvironment(t *testing.T, customAIURL ...string) (http.Handler, *database.DB, *repository.PgxTreasuryRepository, *repository.PgxUserRepository, *repository.PgxAuditRepository, string) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db := database.NewDB(dbURL)
	if err := db.Connect(ctx); err != nil {
		t.Skipf("Skipping integration test: PostgreSQL unavailable at %s: %v", dbURL, err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	aiEngineURL := "http://localhost:8000"
	if len(customAIURL) > 0 && customAIURL[0] != "" {
		aiEngineURL = customAIURL[0]
	} else if envURL := os.Getenv("AI_ENGINE_URL"); envURL != "" {
		aiEngineURL = envURL
	}

	// Verify AI engine connectivity unless an invalid URL was explicitly tested
	if !strings.Contains(aiEngineURL, "59998") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, aiEngineURL+"/health", nil)
		if err != nil {
			t.Skipf("Skipping integration test: invalid AI engine URL: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Skipf("Skipping integration test: AI Engine not reachable at %s: %v", aiEngineURL, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Skipf("Skipping integration test: AI Engine returned non-200 health: %d", resp.StatusCode)
		}
	}

	jwtSecret := "integration-test-secret-key-32-bytes!!"
	cfg := &config.Config{
		Port:               "8080",
		DatabaseURL:        dbURL,
		JWTSecret:          jwtSecret,
		JWTTTL:             time.Hour,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	treasuryRepo := repository.NewPgxTreasuryRepository(db)
	userRepo := repository.NewPgxUserRepository(db)
	auditRepo := repository.NewPgxAuditRepository(db)

	auditSvc := service.NewAuditService(auditRepo)
	auditHandler := handlers.NewAuditHandler(auditSvc)

	authSvc := service.NewAuthService(userRepo, auditSvc, cfg)
	authHandler := handlers.NewAuthHandler(authSvc)
	authMW := middleware.JWTAuth(authSvc)

	treasurySvc := service.NewTreasuryService(treasuryRepo, auditSvc)
	treasuryHandler := handlers.NewTreasuryHandler(treasurySvc)

	aiClient := clients.NewHTTPAIClient(aiEngineURL)
	riskSvc := service.NewRiskService(treasuryRepo, aiClient, auditSvc)
	riskHandler := handlers.NewRiskHandler(riskSvc)

	appRouter := NewRouter(
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

	return appRouter, db, treasuryRepo, userRepo, auditRepo, jwtSecret
}

func TestF5_ServiceToService_LiveAIEngine_Integration(t *testing.T) {
	appRouter, db, treasuryRepo, userRepo, _, jwtSecret := setupIntegrationEnvironment(t)
	ctx := context.Background()

	// 1. Create unique test analyst owner in database
	testSuffix := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("analyst_owner_%d@quanta.local", testSuffix)
	user, err := userRepo.Create(ctx, domain.User{
		Email:        ownerEmail,
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxyz1234567890",
		FullName:     "Integration Analyst Owner",
		Role:         domain.RoleAnalyst,
	})
	if err != nil {
		t.Fatalf("failed to create test analyst user: %v", err)
	}

	// Clean up created user on test completion
	t.Cleanup(func() {
		_, _ = db.Pool().Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	// 2. Create isolated corporate treasury owned by this analyst
	treasuryName := fmt.Sprintf("Live AI Integration Treasury %d", testSuffix)
	treasury, err := treasuryRepo.Create(ctx, domain.CreateTreasuryInput{
		Name:              treasuryName,
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.45,
		MinLiquidityRatio: 0.20,
		OwnerID:           &user.ID,
	})
	if err != nil {
		t.Fatalf("failed to create test treasury: %v", err)
	}

	// Clean up created treasury and audit logs on completion
	t.Cleanup(func() {
		_, _ = db.Pool().Exec(ctx, "DELETE FROM audit_logs WHERE entity_id = $1", treasury.ID)
		_, _ = db.Pool().Exec(ctx, "DELETE FROM corporate_treasuries WHERE id = $1", treasury.ID)
	})

	analystToken := generateTestToken(jwtSecret, user.ID, ownerEmail, domain.RoleAnalyst)

	basePayload := map[string]any{
		"assets": []map[string]any{
			{
				"id":              "gov_bond",
				"name":            "Government Bond",
				"asset_type":      "gov_bond",
				"cost":            0.001,
				"expected_return": 0.07,
				"risk":            0.05,
				"liquidity":       0.95,
			},
			{
				"id":              "bluechip_equity",
				"name":            "Bluechip Equity",
				"asset_type":      "equity",
				"cost":            0.002,
				"expected_return": 0.14,
				"risk":            0.40,
				"liquidity":       0.80,
			},
		},
		"liabilities": []map[string]any{
			{
				"id":                   "loan_facility_1",
				"name":                 "Senior Facility",
				"principal":            10.0,
				"interest_rate":        0.08,
				"maturity_months":      12,
				"priority":              1,
				"is_mandatory_reserve": true,
			},
		},
		"allocations": map[string]float64{
			"gov_bond":        60.0,
			"bluechip_equity": 20.0,
		},
		"run_monte_carlo": true,
		"num_scenarios":   500,
	}

	t.Run("A. Full Risk Analysis via live Python AI Engine and Monte Carlo VaR", func(t *testing.T) {
		reqBody, _ := json.Marshal(basePayload)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/risk/analyze", treasury.ID), bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var resp domain.AIRiskAnalyzeResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode risk analyze response: %v", err)
		}

		// Verify Risk Breakdown computed by Python RiskEngine
		if resp.RiskBreakdown.CompositeRiskScore < 0.0 || resp.RiskBreakdown.CompositeRiskScore > 1.0 {
			t.Errorf("composite risk score out of bounds: %f", resp.RiskBreakdown.CompositeRiskScore)
		}
		if resp.RiskBreakdown.RiskTier == "" {
			t.Error("expected non-empty risk tier from Python AI Engine")
		}

		// Verify Bayesian Inference
		if resp.BayesianInference.StressMultiplier < 1.0 {
			t.Errorf("stress multiplier should be >= 1.0, got %f", resp.BayesianInference.StressMultiplier)
		}
		if resp.BayesianInference.PRecession < 0.0 || resp.BayesianInference.PRecession > 1.0 {
			t.Errorf("recession probability out of bounds: %f", resp.BayesianInference.PRecession)
		}

		// Verify Monte Carlo simulation report
		if resp.MonteCarloReport == nil {
			t.Fatal("expected non-nil Monte Carlo simulation report")
		}
		if resp.MonteCarloReport.NumScenarios != 500 {
			t.Errorf("expected 500 scenarios, got %d", resp.MonteCarloReport.NumScenarios)
		}
		if resp.MonteCarloReport.VaR95 < 0.0 {
			t.Errorf("VaR95 cannot be negative: %f", resp.MonteCarloReport.VaR95)
		}
		if resp.MonteCarloReport.CVaR95 < resp.MonteCarloReport.VaR95 {
			t.Errorf("CVaR95 (%f) should be >= VaR95 (%f)", resp.MonteCarloReport.CVaR95, resp.MonteCarloReport.VaR95)
		}
	})

	t.Run("B. Bayesian Evidence conditioning transmitted correctly to AI Engine", func(t *testing.T) {
		// 1. High stress evidence
		stressPayload := copyMap(basePayload)
		stressPayload["evidence"] = map[string]any{
			"inflation_high":   true,
			"rate_hike":        true,
			"recession_signal": true,
		}

		reqBody, _ := json.Marshal(stressPayload)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/risk/analyze", treasury.ID), bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for stressed evidence, got %d: %s", rec.Code, rec.Body.String())
		}

		var stressResp domain.AIRiskAnalyzeResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &stressResp)

		// 2. Benign evidence
		benignPayload := copyMap(basePayload)
		benignPayload["evidence"] = map[string]any{
			"inflation_high":   false,
			"rate_hike":        false,
			"recession_signal": false,
		}

		reqBody, _ = json.Marshal(benignPayload)
		req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/risk/analyze", treasury.ID), bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")

		rec = httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for benign evidence, got %d: %s", rec.Code, rec.Body.String())
		}

		var benignResp domain.AIRiskAnalyzeResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &benignResp)

		// Stressed multiplier must exceed benign multiplier
		if stressResp.BayesianInference.StressMultiplier <= benignResp.BayesianInference.StressMultiplier {
			t.Errorf("stressed multiplier (%f) must exceed benign multiplier (%f)",
				stressResp.BayesianInference.StressMultiplier, benignResp.BayesianInference.StressMultiplier)
		}
		if stressResp.BayesianInference.PRecession <= benignResp.BayesianInference.PRecession {
			t.Errorf("stressed p_recession (%f) must exceed benign p_recession (%f)",
				stressResp.BayesianInference.PRecession, benignResp.BayesianInference.PRecession)
		}
	})

	t.Run("C. Monte Carlo simulation disabled behavior preserved", func(t *testing.T) {
		noMCPayload := copyMap(basePayload)
		noMCPayload["run_monte_carlo"] = false

		reqBody, _ := json.Marshal(noMCPayload)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/risk/analyze", treasury.ID), bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp domain.AIRiskAnalyzeResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if resp.MonteCarloReport != nil {
			t.Errorf("expected nil monte_carlo_report when run_monte_carlo=false, got %+v", resp.MonteCarloReport)
		}
	})

	t.Run("D. Adversarial Stress Test executes live Alpha-Beta game search", func(t *testing.T) {
		stressTestPayload := map[string]any{
			"assets": []map[string]any{
				{
					"id":              "gov_bond",
					"name":            "Government Bond",
					"asset_type":      "gov_bond",
					"cost":            0.001,
					"expected_return": 0.07,
					"risk":            0.05,
					"liquidity":       0.95,
				},
				{
					"id":              "bluechip_equity",
					"name":            "Bluechip Equity",
					"asset_type":      "equity",
					"cost":            0.002,
					"expected_return": 0.14,
					"risk":            0.40,
					"liquidity":       0.80,
				},
			},
			"allocations": map[string]float64{
				"gov_bond":        60.0,
				"bluechip_equity": 40.0,
			},
		}

		reqBody, _ := json.Marshal(stressTestPayload)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/stress-test", treasury.ID), bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var resp domain.TreasuryStressTestResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode stress test response: %v", err)
		}

		// Verify Adversarial search result from Python
		if resp.AdversarialResult.NodesEvaluated <= 0 {
			t.Errorf("expected positive nodes evaluated, got %d", resp.AdversarialResult.NodesEvaluated)
		}
		if resp.AdversarialResult.BestTreasuryAction == nil || *resp.AdversarialResult.BestTreasuryAction == "" {
			t.Error("expected non-empty best treasury action from Alpha-Beta solver")
		}

		// Verify all 4 discrete macroeconomic shock scenario evaluations evaluated in Go
		if len(resp.Scenarios) != 4 {
			t.Fatalf("expected 4 discrete shock evaluations, got %d", len(resp.Scenarios))
		}
		if resp.Scenarios[0].Scenario.ShockName != "Status Quo" {
			t.Errorf("expected Status Quo first, got %s", resp.Scenarios[0].Scenario.ShockName)
		}
		if resp.Scenarios[3].Scenario.ShockName != "Severe Equity & Credit Crash" {
			t.Errorf("expected Severe Crash fourth, got %s", resp.Scenarios[3].Scenario.ShockName)
		}
		// Severe crash stressed return must be strictly worse than Status Quo
		if resp.Scenarios[3].StressedReturn >= resp.Scenarios[0].StressedReturn {
			t.Errorf("severe crash return (%f) should be worse than status quo (%f)",
				resp.Scenarios[3].StressedReturn, resp.Scenarios[0].StressedReturn)
		}
	})

	t.Run("E. Data Immutability and Audit Logging Verification", func(t *testing.T) {
		// Confirm treasury remains unchanged (read-only verification)
		fetchedTreasury, err := treasuryRepo.GetByID(ctx, treasury.ID)
		if err != nil {
			t.Fatalf("failed to fetch treasury: %v", err)
		}
		if fetchedTreasury.TotalCapital != 100.0 || fetchedTreasury.RiskTolerance != 0.45 || fetchedTreasury.MinLiquidityRatio != 0.20 {
			t.Errorf("treasury state was mutated during risk operations: %+v", fetchedTreasury)
		}

		// Verify audit logs were persisted to database
		var countRiskLogs int
		err = db.Pool().QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND action = $2 AND actor = $3", treasury.ID, domain.ActionRiskAnalyzed, user.Email).Scan(&countRiskLogs)
		if err != nil {
			t.Fatalf("failed to count risk audit logs: %v", err)
		}
		if countRiskLogs < 1 {
			t.Errorf("expected at least 1 risk.analyzed audit log, got %d", countRiskLogs)
		}

		var countStressLogs int
		err = db.Pool().QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND action = $2 AND actor = $3", treasury.ID, domain.ActionStressTested, user.Email).Scan(&countStressLogs)
		if err != nil {
			t.Fatalf("failed to count stress audit logs: %v", err)
		}
		if countStressLogs < 1 {
			t.Errorf("expected at least 1 stress_test.executed audit log, got %d", countStressLogs)
		}
	})
}

func TestF5_RBAC_And_TenantIsolation_Integration(t *testing.T) {
	appRouter, db, treasuryRepo, userRepo, _, jwtSecret := setupIntegrationEnvironment(t)
	ctx := context.Background()

	testSuffix := time.Now().UnixNano()

	// 1. Create Analyst User A (Owner of Treasury Alpha)
	emailA := fmt.Sprintf("analyst_a_%d@quanta.local", testSuffix)
	userA, err := userRepo.Create(ctx, domain.User{
		Email:        emailA,
		PasswordHash: "dummyhash",
		FullName:     "Analyst User A",
		Role:         domain.RoleAnalyst,
	})
	if err != nil {
		t.Fatalf("failed to create user A: %v", err)
	}

	// 2. Create Analyst User B (Not Owner of Treasury Alpha)
	emailB := fmt.Sprintf("analyst_b_%d@quanta.local", testSuffix)
	userB, err := userRepo.Create(ctx, domain.User{
		Email:        emailB,
		PasswordHash: "dummyhash",
		FullName:     "Analyst User B",
		Role:         domain.RoleAnalyst,
	})
	if err != nil {
		t.Fatalf("failed to create user B: %v", err)
	}

	// 3. Create Viewer User C
	emailC := fmt.Sprintf("viewer_c_%d@quanta.local", testSuffix)
	userC, err := userRepo.Create(ctx, domain.User{
		Email:        emailC,
		PasswordHash: "dummyhash",
		FullName:     "Viewer User C",
		Role:         domain.RoleViewer,
	})
	if err != nil {
		t.Fatalf("failed to create user C: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Pool().Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2, $3)", userA.ID, userB.ID, userC.ID)
	})

	// 4. Create Treasury Alpha (Restricted to User A)
	treasuryAlpha, err := treasuryRepo.Create(ctx, domain.CreateTreasuryInput{
		Name:              fmt.Sprintf("Treasury Alpha %d", testSuffix),
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		OwnerID:           &userA.ID,
	})
	if err != nil {
		t.Fatalf("failed to create treasury alpha: %v", err)
	}

	// 5. Create Treasury Beta (Unassigned / Legacy with OwnerID == nil)
	treasuryBeta, err := treasuryRepo.Create(ctx, domain.CreateTreasuryInput{
		Name:              fmt.Sprintf("Legacy Unassigned Treasury %d", testSuffix),
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		OwnerID:           nil,
	})
	if err != nil {
		t.Fatalf("failed to create legacy treasury beta: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Pool().Exec(ctx, "DELETE FROM audit_logs WHERE entity_id IN ($1, $2)", treasuryAlpha.ID, treasuryBeta.ID)
		_, _ = db.Pool().Exec(ctx, "DELETE FROM corporate_treasuries WHERE id IN ($1, $2)", treasuryAlpha.ID, treasuryBeta.ID)
	})

	// Generate JWT tokens for each persona
	tokenAnalystA := generateTestToken(jwtSecret, userA.ID, userA.Email, domain.RoleAnalyst)
	tokenAnalystB := generateTestToken(jwtSecret, userB.ID, userB.Email, domain.RoleAnalyst)
	tokenViewerC := generateTestToken(jwtSecret, userC.ID, userC.Email, domain.RoleViewer)
	tokenAdmin := generateTestToken(jwtSecret, "admin-system-id", "admin@quanta.local", domain.RoleAdmin)

	validPayload := map[string]any{
		"assets": []map[string]any{
			{
				"id":              "gov_bond",
				"name":            "Gov Bond",
				"asset_type":      "gov_bond",
				"cost":            0.001,
				"expected_return": 0.07,
				"risk":            0.05,
				"liquidity":       0.95,
			},
		},
		"allocations": map[string]float64{
			"gov_bond": 50.0,
		},
	}
	payloadBytes, _ := json.Marshal(validPayload)

	endpoints := []struct {
		name string
		path string
	}{
		{name: "POST /risk/analyze", path: "/risk/analyze"},
		{name: "POST /stress-test", path: "/stress-test"},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s - Unauthenticated request returns 401 Unauthorized", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryAlpha.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
			}
		})

		t.Run(fmt.Sprintf("%s - Viewer request returns 403 Forbidden", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryAlpha.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Authorization", "Bearer "+tokenViewerC)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for Viewer, got %d", rec.Code)
			}
		})

		t.Run(fmt.Sprintf("%s - Analyst Owner accessing their own restricted treasury returns 200 OK", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryAlpha.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Authorization", "Bearer "+tokenAnalystA)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 OK for Analyst Owner, got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(fmt.Sprintf("%s - Analyst Non-Owner accessing another user's restricted treasury returns 403 Forbidden", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryAlpha.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Authorization", "Bearer "+tokenAnalystB)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for Analyst Non-Owner, got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(fmt.Sprintf("%s - Analyst accessing unassigned/legacy treasury returns 403 Forbidden", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryBeta.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Authorization", "Bearer "+tokenAnalystA)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for Analyst on unassigned treasury, got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(fmt.Sprintf("%s - Admin accessing restricted treasury returns 200 OK", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryAlpha.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Authorization", "Bearer "+tokenAdmin)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 OK for Admin on restricted treasury, got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(fmt.Sprintf("%s - Admin accessing unassigned/legacy treasury returns 200 OK", ep.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s%s", treasuryBeta.ID, ep.path), bytes.NewReader(payloadBytes))
			req.Header.Set("Authorization", "Bearer "+tokenAdmin)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			appRouter.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 OK for Admin on unassigned treasury, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestF5_UpstreamFailure_Handling_Integration(t *testing.T) {
	// Custom dead AI engine URL to test connection refusal handling
	appRouter, db, treasuryRepo, userRepo, _, jwtSecret := setupIntegrationEnvironment(t, "http://127.0.0.1:59998")
	ctx := context.Background()

	testSuffix := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("failure_analyst_%d@quanta.local", testSuffix)
	user, err := userRepo.Create(ctx, domain.User{
		Email:        ownerEmail,
		PasswordHash: "dummy",
		FullName:     "Failure Test Analyst",
		Role:         domain.RoleAnalyst,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Pool().Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	treasury, err := treasuryRepo.Create(ctx, domain.CreateTreasuryInput{
		Name:              fmt.Sprintf("Failure Treasury %d", testSuffix),
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		OwnerID:           &user.ID,
	})
	if err != nil {
		t.Fatalf("failed to create treasury: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Pool().Exec(ctx, "DELETE FROM corporate_treasuries WHERE id = $1", treasury.ID)
	})

	analystToken := generateTestToken(jwtSecret, user.ID, user.Email, domain.RoleAnalyst)

	payload := map[string]any{
		"assets": []map[string]any{
			{
				"id":              "gov_bond",
				"name":            "Gov Bond",
				"asset_type":      "gov_bond",
				"cost":            0.001,
				"expected_return": 0.07,
				"risk":            0.05,
				"liquidity":       0.95,
			},
		},
		"allocations": map[string]float64{
			"gov_bond": 50.0,
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	t.Run("POST /risk/analyze returns 502 Bad Gateway when AI Engine is unavailable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/risk/analyze", treasury.ID), bytes.NewReader(payloadBytes))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Errorf("expected 502 Bad Gateway on AI engine outage, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "AI engine unavailable") {
			t.Errorf("expected 'AI engine unavailable' in body, got: %s", rec.Body.String())
		}
	})

	t.Run("POST /stress-test returns 502 Bad Gateway when AI Engine is unavailable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/treasuries/%s/stress-test", treasury.ID), bytes.NewReader(payloadBytes))
		req.Header.Set("Authorization", "Bearer "+analystToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		appRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Errorf("expected 502 Bad Gateway on AI engine outage, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "AI engine unavailable") {
			t.Errorf("expected 'AI engine unavailable' in body, got: %s", rec.Body.String())
		}
	})
}

func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
