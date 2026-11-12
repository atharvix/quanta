package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func TestRiskService_AnalyzeRisk(t *testing.T) {
	ctx := domain.WithUserContext(context.Background(), "admin-user", "admin@quanta.local", domain.RoleAdmin)
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Acme Global Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	sampleAssets := []domain.Asset{
		{
			ID:             "gov_bond",
			Name:           "Government Bond",
			AssetType:      "gov_bond",
			Cost:           0.001,
			ExpectedReturn: 0.07,
			Risk:           0.05,
			Liquidity:      0.95,
		},
		{
			ID:             "bluechip_equity",
			Name:           "Bluechip Equity",
			AssetType:      "equity",
			Cost:           0.002,
			ExpectedReturn: 0.12,
			Risk:           0.18,
			Liquidity:      0.85,
		},
	}

	sampleLiabilities := []domain.Liability{
		{
			ID:                 "loan_1",
			Name:               "Senior Facility",
			Principal:          10.0,
			InterestRate:       0.06,
			IsMandatoryReserve: true,
		},
	}

	t.Run("Success with defaults for Monte Carlo and nil evidence", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		var capturedReq domain.AIRiskAnalyzeRequest
		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				capturedReq = req
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{
						CompositeRiskScore: 0.065,
						RiskTier:           "Moderate",
					},
					BayesianInference: domain.BayesianRiskInferenceResult{
						PRecession:       0.35,
						StressMultiplier: 1.25,
						Explanation:      "Inference complete",
					},
					MonteCarloReport: &domain.MonteCarloSimulationReport{
						NumScenarios: 1000,
						VaR95:        0.042,
					},
				}, nil
			},
		}

		svc := NewRiskService(tRepo, aiClient)
		input := domain.AnalyzeTreasuryRiskInput{
			Assets:      sampleAssets,
			Liabilities: sampleLiabilities,
			Allocations: map[string]float64{
				"gov_bond":        60.0,
				"bluechip_equity": 30.0,
			},
		}

		resp, err := svc.AnalyzeRisk(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.RiskBreakdown.RiskTier != "Moderate" {
			t.Errorf("expected Moderate, got %s", resp.RiskBreakdown.RiskTier)
		}
		if capturedReq.RunMonteCarlo != true {
			t.Errorf("expected RunMonteCarlo default true, got %v", capturedReq.RunMonteCarlo)
		}
		if capturedReq.NumScenarios != 1000 {
			t.Errorf("expected NumScenarios default 1000, got %d", capturedReq.NumScenarios)
		}
		if capturedReq.Evidence != nil {
			t.Errorf("expected nil Evidence, got %v", capturedReq.Evidence)
		}
		if capturedReq.State.ReservedForDebt != 10.0 {
			t.Errorf("expected 10.0 debt reserve, got %f", capturedReq.State.ReservedForDebt)
		}
		if capturedReq.State.Cash != 0.0 {
			t.Errorf("expected 0.0 cash (100 - 90 - 10), got %f", capturedReq.State.Cash)
		}
	})

	t.Run("Success with custom evidence and explicit Monte Carlo false", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		runMC := false
		numScen := 500
		infHigh := true
		rateHike := false

		var capturedReq domain.AIRiskAnalyzeRequest
		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				capturedReq = req
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{
						RiskTier: "High Growth / Moderate Risk",
					},
					BayesianInference: domain.BayesianRiskInferenceResult{
						PRecession: 0.65,
					},
				}, nil
			},
		}

		svc := NewRiskService(tRepo, aiClient)
		input := domain.AnalyzeTreasuryRiskInput{
			Assets:        sampleAssets,
			Liabilities:   sampleLiabilities,
			RunMonteCarlo: &runMC,
			NumScenarios:  &numScen,
			Evidence: &domain.BayesianEvidence{
				InflationHigh:   &infHigh,
				RateHike:        &rateHike,
				RecessionSignal: nil,
			},
		}

		resp, err := svc.AnalyzeRisk(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.RiskBreakdown.RiskTier != "High Growth / Moderate Risk" {
			t.Errorf("unexpected tier: %s", resp.RiskBreakdown.RiskTier)
		}
		if capturedReq.RunMonteCarlo != false {
			t.Errorf("expected RunMonteCarlo false, got true")
		}
		if capturedReq.NumScenarios != 500 {
			t.Errorf("expected 500 scenarios, got %d", capturedReq.NumScenarios)
		}
		if capturedReq.Evidence == nil || *capturedReq.Evidence.InflationHigh != true || *capturedReq.Evidence.RateHike != false || capturedReq.Evidence.RecessionSignal != nil {
			t.Errorf("evidence not preserved correctly: %+v", capturedReq.Evidence)
		}
	})

	t.Run("Invalid Treasury UUID returns ErrInvalidID", func(t *testing.T) {
		svc := NewRiskService(nil, nil)
		_, err := svc.AnalyzeRisk(ctx, "invalid-uuid", domain.AnalyzeTreasuryRiskInput{})
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("expected ErrInvalidID, got: %v", err)
		}
	})

	t.Run("Treasury not found returns ErrNotFound", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}

		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("Empty assets returns validation error", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: []domain.Asset{},
		})

		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError for empty assets, got: %v", err)
		}
	})

	t.Run("Allocations exceeding capital returns validation error", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets:      sampleAssets,
			Liabilities: sampleLiabilities, // 10 mandatory reserve
			Allocations: map[string]float64{
				"gov_bond": 95.0, // 95 + 10 = 105 > 100
			},
		})

		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError for exceeding capital, got: %v", err)
		}
	})

	t.Run("Negative allocation value returns validation error", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
			Allocations: map[string]float64{
				"gov_bond": -10.0,
			},
		})

		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError for negative allocation, got: %v", err)
		}
	})

	t.Run("Allocated asset ID not in catalog returns validation error", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
			Allocations: map[string]float64{
				"unknown_crypto": 50.0,
			},
		})

		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError for asset not in catalog, got: %v", err)
		}
	})

	t.Run("Custom step size less than or equal to zero returns validation error", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		zeroStep := 0.0
		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets:   sampleAssets,
			StepSize: &zeroStep,
		})

		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError for step_size <= 0, got: %v", err)
		}
	})

	t.Run("Step size too small resulting in > 200 steps returns validation error", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil // TotalCapital = 100.0
			},
		}

		// step_size = 0.2 -> 100 / 0.2 = 500 steps > 200
		smallStep := 0.2
		svc := NewRiskService(tRepo, nil)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets:   sampleAssets,
			StepSize: &smallStep,
		})

		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError for step_size too small, got: %v", err)
		}
	})

	t.Run("All false Bayesian evidence correctly propagated to AI request", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		var capturedReq domain.AIRiskAnalyzeRequest
		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				capturedReq = req
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{RiskTier: "Low"},
				}, nil
			},
		}

		fFalse := false
		svc := NewRiskService(tRepo, aiClient)
		input := domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
			Evidence: &domain.BayesianEvidence{
				InflationHigh:   &fFalse,
				RateHike:        &fFalse,
				RecessionSignal: &fFalse,
			},
		}

		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedReq.Evidence == nil {
			t.Fatal("expected non-nil evidence")
		}
		if *capturedReq.Evidence.InflationHigh != false || *capturedReq.Evidence.RateHike != false || *capturedReq.Evidence.RecessionSignal != false {
			t.Errorf("expected all false evidence, got: %+v", capturedReq.Evidence)
		}
	})

	t.Run("Explicit Monte Carlo true with custom scenarios preserved", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		var capturedReq domain.AIRiskAnalyzeRequest
		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				capturedReq = req
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{RiskTier: "Moderate"},
				}, nil
			},
		}

		runMC := true
		numScen := 5000
		svc := NewRiskService(tRepo, aiClient)
		input := domain.AnalyzeTreasuryRiskInput{
			Assets:        sampleAssets,
			RunMonteCarlo: &runMC,
			NumScenarios:  &numScen,
		}

		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedReq.RunMonteCarlo != true {
			t.Errorf("expected RunMonteCarlo true, got %v", capturedReq.RunMonteCarlo)
		}
		if capturedReq.NumScenarios != 5000 {
			t.Errorf("expected 5000 scenarios, got %d", capturedReq.NumScenarios)
		}
	})

	t.Run("Non-positive NumScenarios defaults to 1000", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		var capturedReq domain.AIRiskAnalyzeRequest
		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				capturedReq = req
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{RiskTier: "Moderate"},
				}, nil
			},
		}

		negScen := -50
		svc := NewRiskService(tRepo, aiClient)
		input := domain.AnalyzeTreasuryRiskInput{
			Assets:       sampleAssets,
			NumScenarios: &negScen,
		}

		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedReq.NumScenarios != 1000 {
			t.Errorf("expected default 1000 scenarios for non-positive input, got %d", capturedReq.NumScenarios)
		}
	})

	t.Run("Zero mandatory debt reserve correctly calculated without deductions", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil // TotalCapital = 100.0
			},
		}

		var capturedReq domain.AIRiskAnalyzeRequest
		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				capturedReq = req
				return &domain.AIRiskAnalyzeResponse{
					RiskBreakdown: domain.RiskFactorBreakdown{RiskTier: "Low"},
				}, nil
			},
		}

		nonMandatoryLiabilities := []domain.Liability{
			{
				ID:                 "revolving_credit",
				Name:               "Credit Line",
				Principal:          15.0,
				InterestRate:       0.05,
				IsMandatoryReserve: false, // NOT mandatory
			},
		}

		svc := NewRiskService(tRepo, aiClient)
		input := domain.AnalyzeTreasuryRiskInput{
			Assets:      sampleAssets,
			Liabilities: nonMandatoryLiabilities,
			Allocations: map[string]float64{
				"gov_bond": 40.0,
			},
		}

		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedReq.State.ReservedForDebt != 0.0 {
			t.Errorf("expected 0.0 debt reserve, got %f", capturedReq.State.ReservedForDebt)
		}
		// Cash = 100 - 40 - 0 = 60.0
		if math.Abs(capturedReq.State.Cash-60.0) > 1e-4 {
			t.Errorf("expected 60.0 cash, got %f", capturedReq.State.Cash)
		}
	})

	t.Run("Upstream AI Engine failure maps to ErrUpstreamUnavailable", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &MockAIClient{
			analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}

		svc := NewRiskService(tRepo, aiClient)
		_, err := svc.AnalyzeRisk(ctx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
		})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})
}

func TestRiskService_AdversarialStressTest(t *testing.T) {
	ctx := domain.WithUserContext(context.Background(), "admin-user", "admin@quanta.local", domain.RoleAdmin)
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Acme Global Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	sampleAssets := []domain.Asset{
		{
			ID:             "gov_bond",
			Name:           "Government Bond",
			AssetType:      "gov_bond",
			Cost:           0.001,
			ExpectedReturn: 0.07,
			Risk:           0.05,
			Liquidity:      0.95,
		},
		{
			ID:             "bluechip_equity",
			Name:           "Bluechip Equity",
			AssetType:      "equity",
			Cost:           0.002,
			ExpectedReturn: 0.12,
			Risk:           0.18,
			Liquidity:      0.85,
		},
	}

	t.Run("Success returns AdversarialResult and 4 Scenarios", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		actionDesc := "Action(INVEST, gov_bond, 5.0)"
		aiClient := &MockAIClient{
			adversarialStressTestFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
				return &domain.AdversarialStressTestResult{
					WorstCasePayoff:    0.045,
					BestTreasuryAction: &actionDesc,
					WorstMarketShock:   "Severe Equity & Credit Crash",
					NodesEvaluated:     36,
					PrunedBranches:     8,
					ExecutionTimeMS:    14.2,
					Explanation:        "Adversarial search evaluated 36 nodes",
				}, nil
			},
		}

		svc := NewRiskService(tRepo, aiClient)
		input := domain.AdversarialStressTestInput{
			Assets: sampleAssets,
			Allocations: map[string]float64{
				"gov_bond":        60.0,
				"bluechip_equity": 40.0,
			},
		}

		resp, err := svc.AdversarialStressTest(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.AdversarialResult.WorstMarketShock != "Severe Equity & Credit Crash" {
			t.Errorf("unexpected shock: %s", resp.AdversarialResult.WorstMarketShock)
		}
		if resp.AdversarialResult.NodesEvaluated != 36 {
			t.Errorf("expected 36 nodes, got %d", resp.AdversarialResult.NodesEvaluated)
		}
		if len(resp.Scenarios) != 4 {
			t.Fatalf("expected 4 discrete shock evaluations, got %d", len(resp.Scenarios))
		}

		// Verify first scenario is Status Quo
		if resp.Scenarios[0].Scenario.ShockName != "Status Quo" {
			t.Errorf("expected Status Quo, got %s", resp.Scenarios[0].Scenario.ShockName)
		}

		// Verify severe crash scenario is evaluated
		severeScenario := resp.Scenarios[3]
		if severeScenario.Scenario.ShockName != "Severe Equity & Credit Crash" {
			t.Errorf("expected Severe Equity & Credit Crash, got %s", severeScenario.Scenario.ShockName)
		}
		if severeScenario.StressedReturn >= resp.Scenarios[0].StressedReturn {
			t.Errorf("severe crash return (%f) should be lower than status quo (%f)", severeScenario.StressedReturn, resp.Scenarios[0].StressedReturn)
		}
	})

	t.Run("Invalid Treasury UUID returns ErrInvalidID", func(t *testing.T) {
		svc := NewRiskService(nil, nil)
		_, err := svc.AdversarialStressTest(ctx, "bad-id", domain.AdversarialStressTestInput{})
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("expected ErrInvalidID, got: %v", err)
		}
	})

	t.Run("Treasury not found returns ErrNotFound", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}

		svc := NewRiskService(tRepo, nil)
		_, err := svc.AdversarialStressTest(ctx, validTreasuryID, domain.AdversarialStressTestInput{
			Assets: sampleAssets,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("Upstream AI Engine failure maps to ErrUpstreamUnavailable", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return sampleTreasury, nil
			},
		}

		aiClient := &MockAIClient{
			adversarialStressTestFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}

		svc := NewRiskService(tRepo, aiClient)
		_, err := svc.AdversarialStressTest(ctx, validTreasuryID, domain.AdversarialStressTestInput{
			Assets: sampleAssets,
		})
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got: %v", err)
		}
	})
}

func TestRiskService_TreasuryAuthorization(t *testing.T) {
	ctx := context.Background()
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"
	ownerUserID := "user-analyst-alpha"
	otherUserID := "user-analyst-beta"

	restrictedTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Restricted Treasury Alpha",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		OwnerID:           &ownerUserID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	sampleAssets := []domain.Asset{
		{
			ID:             "gov_bond",
			Name:           "Gov Bond",
			AssetType:      "gov_bond",
			Cost:           0.001,
			ExpectedReturn: 0.07,
			Risk:           0.05,
			Liquidity:      0.95,
		},
	}

	tRepo := &MockTreasuryRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
			return restrictedTreasury, nil
		},
	}

	aiClient := &MockAIClient{
		analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
			return &domain.AIRiskAnalyzeResponse{
				RiskBreakdown: domain.RiskFactorBreakdown{RiskTier: "Low"},
			}, nil
		},
		adversarialStressTestFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
			return &domain.AdversarialStressTestResult{WorstMarketShock: "Status Quo"}, nil
		},
	}

	svc := NewRiskService(tRepo, aiClient)

	riskInput := domain.AnalyzeTreasuryRiskInput{
		Assets: sampleAssets,
	}
	stressInput := domain.AdversarialStressTestInput{
		Assets: sampleAssets,
	}

	t.Run("Analyst cannot access another user's restricted treasury", func(t *testing.T) {
		unauthorizedCtx := domain.WithUserContext(ctx, otherUserID, "other@quanta.local", domain.RoleAnalyst)

		_, err := svc.AnalyzeRisk(unauthorizedCtx, validTreasuryID, riskInput)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for AnalyzeRisk, got: %v", err)
		}

		_, err = svc.AdversarialStressTest(unauthorizedCtx, validTreasuryID, stressInput)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for AdversarialStressTest, got: %v", err)
		}
	})

	t.Run("Owner analyst can access their own restricted treasury", func(t *testing.T) {
		ownerCtx := domain.WithUserContext(ctx, ownerUserID, "owner@quanta.local", domain.RoleAnalyst)

		resp, err := svc.AnalyzeRisk(ownerCtx, validTreasuryID, riskInput)
		if err != nil {
			t.Fatalf("expected success for owner on AnalyzeRisk, got: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}

		stressResp, err := svc.AdversarialStressTest(ownerCtx, validTreasuryID, stressInput)
		if err != nil {
			t.Fatalf("expected success for owner on AdversarialStressTest, got: %v", err)
		}
		if stressResp == nil {
			t.Fatal("expected non-nil stress response")
		}
	})

	t.Run("Admin can access any user's restricted treasury", func(t *testing.T) {
		adminCtx := domain.WithUserContext(ctx, "admin-user", "admin@quanta.local", domain.RoleAdmin)

		resp, err := svc.AnalyzeRisk(adminCtx, validTreasuryID, riskInput)
		if err != nil {
			t.Fatalf("expected success for admin on AnalyzeRisk, got: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}

		stressResp, err := svc.AdversarialStressTest(adminCtx, validTreasuryID, stressInput)
		if err != nil {
			t.Fatalf("expected success for admin on AdversarialStressTest, got: %v", err)
		}
		if stressResp == nil {
			t.Fatal("expected non-nil stress response")
		}
	})

	t.Run("Unauthenticated context cannot access restricted treasury", func(t *testing.T) {
		anonCtx := context.Background()

		_, err := svc.AnalyzeRisk(anonCtx, validTreasuryID, riskInput)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for unauthenticated context on AnalyzeRisk, got: %v", err)
		}

		_, err = svc.AdversarialStressTest(anonCtx, validTreasuryID, stressInput)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for unauthenticated context on AdversarialStressTest, got: %v", err)
		}
	})

	t.Run("Analyst cannot access unassigned/legacy treasury (OwnerID == nil)", func(t *testing.T) {
		unassignedTreasury := &domain.Treasury{
			ID:                validTreasuryID,
			Name:              "Unassigned Legacy Treasury",
			BaseCurrency:      "INR",
			TotalCapital:      100.0,
			RiskTolerance:     0.40,
			MinLiquidityRatio: 0.20,
			OwnerID:           nil,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		uRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return unassignedTreasury, nil
			},
		}
		uSvc := NewRiskService(uRepo, aiClient)

		analystCtx := domain.WithUserContext(ctx, "any-analyst", "analyst@quanta.local", domain.RoleAnalyst)

		_, err := uSvc.AnalyzeRisk(analystCtx, validTreasuryID, riskInput)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for Analyst on unassigned treasury in AnalyzeRisk, got: %v", err)
		}

		_, err = uSvc.AdversarialStressTest(analystCtx, validTreasuryID, stressInput)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for Analyst on unassigned treasury in AdversarialStressTest, got: %v", err)
		}
	})

	t.Run("Admin can access unassigned/legacy treasury (OwnerID == nil)", func(t *testing.T) {
		unassignedTreasury := &domain.Treasury{
			ID:                validTreasuryID,
			Name:              "Unassigned Legacy Treasury",
			BaseCurrency:      "INR",
			TotalCapital:      100.0,
			RiskTolerance:     0.40,
			MinLiquidityRatio: 0.20,
			OwnerID:           nil,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		uRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return unassignedTreasury, nil
			},
		}
		uSvc := NewRiskService(uRepo, aiClient)

		adminCtx := domain.WithUserContext(ctx, "admin-user", "admin@quanta.local", domain.RoleAdmin)

		resp, err := uSvc.AnalyzeRisk(adminCtx, validTreasuryID, riskInput)
		if err != nil {
			t.Fatalf("expected Admin to access unassigned treasury in AnalyzeRisk, got: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}

		stressResp, err := uSvc.AdversarialStressTest(adminCtx, validTreasuryID, stressInput)
		if err != nil {
			t.Fatalf("expected Admin to access unassigned treasury in AdversarialStressTest, got: %v", err)
		}
		if stressResp == nil {
			t.Fatal("expected non-nil response")
		}
	})
}

func TestRiskService_AuditLogging(t *testing.T) {
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"
	analystUserID := "user-analyst-1"
	analystEmail := "analyst1@quanta.local"
	authCtx := domain.WithUserContext(context.Background(), analystUserID, analystEmail, domain.RoleAnalyst)

	sampleTreasury := &domain.Treasury{
		ID:                validTreasuryID,
		Name:              "Audit Test Treasury",
		BaseCurrency:      "INR",
		TotalCapital:      100.0,
		RiskTolerance:     0.40,
		MinLiquidityRatio: 0.20,
		OwnerID:           &analystUserID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	sampleAssets := []domain.Asset{
		{
			ID:             "gov_bond",
			Name:           "Gov Bond",
			AssetType:      "gov_bond",
			Cost:           0.001,
			ExpectedReturn: 0.07,
			Risk:           0.05,
			Liquidity:      0.95,
		},
	}

	tRepo := &MockTreasuryRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
			return sampleTreasury, nil
		},
	}

	aiClient := &MockAIClient{
		analyzeRiskFn: func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
			return &domain.AIRiskAnalyzeResponse{
				RiskBreakdown: domain.RiskFactorBreakdown{
					RiskTier:           "Moderate",
					CompositeRiskScore: 0.055,
				},
				BayesianInference: domain.BayesianRiskInferenceResult{
					StressMultiplier: 1.25,
					Explanation:      "Audit test inference",
				},
			}, nil
		},
		adversarialStressTestFn: func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
			return &domain.AdversarialStressTestResult{
				WorstMarketShock: "Liquidity Squeeze",
				WorstCasePayoff:  0.035,
				NodesEvaluated:   50,
			}, nil
		},
	}

	t.Run("AnalyzeRisk records risk.analyzed with correct actor, treasury ID and metadata", func(t *testing.T) {
		var capturedLog domain.AuditLog
		auditRepo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				capturedLog = log
				return &log, nil
			},
		}

		auditSvc := NewAuditService(auditRepo)
		svc := NewRiskService(tRepo, aiClient, auditSvc)

		resp, err := svc.AnalyzeRisk(authCtx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}

		if capturedLog.Action != domain.ActionRiskAnalyzed {
			t.Errorf("expected action %s, got %s", domain.ActionRiskAnalyzed, capturedLog.Action)
		}
		if capturedLog.EntityType != "treasury" {
			t.Errorf("expected entity_type treasury, got %s", capturedLog.EntityType)
		}
		if capturedLog.EntityID == nil || *capturedLog.EntityID != validTreasuryID {
			t.Errorf("expected entity_id %s, got %v", validTreasuryID, capturedLog.EntityID)
		}
		if capturedLog.Actor != analystEmail {
			t.Errorf("expected actor %s, got %s", analystEmail, capturedLog.Actor)
		}

		var meta map[string]any
		if err := json.Unmarshal(capturedLog.Metadata, &meta); err != nil {
			t.Fatalf("failed to unmarshal metadata: %v", err)
		}
		if meta["risk_tier"] != "Moderate" {
			t.Errorf("expected risk_tier Moderate, got %v", meta["risk_tier"])
		}
		if meta["treasury_id"] != validTreasuryID {
			t.Errorf("expected treasury_id %s in metadata, got %v", validTreasuryID, meta["treasury_id"])
		}
	})

	t.Run("AnalyzeRisk succeeds even if audit logging fails (best-effort)", func(t *testing.T) {
		auditRepo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				return nil, errors.New("db disk failure")
			},
		}

		auditSvc := NewAuditService(auditRepo)
		svc := NewRiskService(tRepo, aiClient, auditSvc)

		resp, err := svc.AnalyzeRisk(authCtx, validTreasuryID, domain.AnalyzeTreasuryRiskInput{
			Assets: sampleAssets,
		})
		if err != nil {
			t.Fatalf("AnalyzeRisk should not fail when audit logging fails: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})

	t.Run("AdversarialStressTest records stress_test.executed with correct actor, treasury ID and metadata", func(t *testing.T) {
		var capturedLog domain.AuditLog
		auditRepo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				capturedLog = log
				return &log, nil
			},
		}

		auditSvc := NewAuditService(auditRepo)
		svc := NewRiskService(tRepo, aiClient, auditSvc)

		resp, err := svc.AdversarialStressTest(authCtx, validTreasuryID, domain.AdversarialStressTestInput{
			Assets: sampleAssets,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}

		if capturedLog.Action != domain.ActionStressTested {
			t.Errorf("expected action %s, got %s", domain.ActionStressTested, capturedLog.Action)
		}
		if capturedLog.EntityType != "treasury" {
			t.Errorf("expected entity_type treasury, got %s", capturedLog.EntityType)
		}
		if capturedLog.EntityID == nil || *capturedLog.EntityID != validTreasuryID {
			t.Errorf("expected entity_id %s, got %v", validTreasuryID, capturedLog.EntityID)
		}
		if capturedLog.Actor != analystEmail {
			t.Errorf("expected actor %s, got %s", analystEmail, capturedLog.Actor)
		}

		var meta map[string]any
		if err := json.Unmarshal(capturedLog.Metadata, &meta); err != nil {
			t.Fatalf("failed to unmarshal metadata: %v", err)
		}
		if meta["worst_market_shock"] != "Liquidity Squeeze" {
			t.Errorf("expected worst_market_shock Liquidity Squeeze, got %v", meta["worst_market_shock"])
		}
		if meta["treasury_id"] != validTreasuryID {
			t.Errorf("expected treasury_id %s in metadata, got %v", validTreasuryID, meta["treasury_id"])
		}
	})

	t.Run("AdversarialStressTest succeeds even if audit logging fails (best-effort)", func(t *testing.T) {
		auditRepo := &MockAuditRepository{
			createFn: func(ctx context.Context, log domain.AuditLog) (*domain.AuditLog, error) {
				return nil, errors.New("db disk failure")
			},
		}

		auditSvc := NewAuditService(auditRepo)
		svc := NewRiskService(tRepo, aiClient, auditSvc)

		resp, err := svc.AdversarialStressTest(authCtx, validTreasuryID, domain.AdversarialStressTestInput{
			Assets: sampleAssets,
		})
		if err != nil {
			t.Fatalf("AdversarialStressTest should not fail when audit logging fails: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})
}

func TestEvaluateShockScenario_LiquidityBehavior(t *testing.T) {
	st := &domain.FinancialState{
		TotalCapital:   100.0,
		ExpectedReturn: 0.08,
		LiquidityRatio: 0.40,
		RiskScore:      0.05,
		Allocations: map[string]float64{
			"equity":    30.0,
			"corp_bond": 20.0,
		},
	}
	riskTolerance := 0.50
	minLiquidity := 0.20

	t.Run("Negative liquidity freeze reduces liquidity using absolute magnitude", func(t *testing.T) {
		shock := domain.MarketShockScenario{
			ShockName:         "Liquidity Squeeze",
			EquityDrawdown:    -0.10,
			CreditSpreadSpike: -0.05,
			LiquidityFreeze:   -0.25, // negative float defined in AI Engine
		}

		eval := evaluateShockScenario(st, riskTolerance, minLiquidity, shock)

		// Expected: 0.40 - |-0.25| = 0.15
		expectedLiq := 0.15
		if math.Abs(eval.StressedLiquidity-expectedLiq) > 1e-6 {
			t.Errorf("expected stressed liquidity %f, got %f", expectedLiq, eval.StressedLiquidity)
		}
		// Stressed liquidity 0.15 < minLiquidity 0.20 => non-compliant
		if eval.IsCompliant != false {
			t.Errorf("expected is_compliant to be false (0.15 < 0.20), got %v", eval.IsCompliant)
		}
	})

	t.Run("Zero liquidity freeze leaves liquidity unchanged", func(t *testing.T) {
		shock := domain.MarketShockScenario{
			ShockName:         "Status Quo",
			EquityDrawdown:    0.0,
			CreditSpreadSpike: 0.0,
			LiquidityFreeze:   0.0,
		}

		eval := evaluateShockScenario(st, riskTolerance, minLiquidity, shock)

		// Expected: 0.40 - 0 = 0.40
		expectedLiq := 0.40
		if math.Abs(eval.StressedLiquidity-expectedLiq) > 1e-6 {
			t.Errorf("expected stressed liquidity %f, got %f", expectedLiq, eval.StressedLiquidity)
		}
		// Stressed liquidity 0.40 >= minLiquidity 0.20 => compliant
		if eval.IsCompliant != true {
			t.Errorf("expected is_compliant to be true (0.40 >= 0.20), got %v", eval.IsCompliant)
		}
	})

	t.Run("Liquidity ratio that would otherwise become negative is clamped to 0", func(t *testing.T) {
		shock := domain.MarketShockScenario{
			ShockName:         "Extreme Crisis",
			EquityDrawdown:    -0.50,
			CreditSpreadSpike: -0.30,
			LiquidityFreeze:   -0.70, // 0.40 - 0.70 = -0.30 -> clamped to 0.0
		}

		eval := evaluateShockScenario(st, riskTolerance, minLiquidity, shock)

		if eval.StressedLiquidity < 0.0 {
			t.Errorf("stressed liquidity must not be negative, got %f", eval.StressedLiquidity)
		}
		if eval.StressedLiquidity != 0.0 {
			t.Errorf("expected stressed liquidity 0.0, got %f", eval.StressedLiquidity)
		}
		if eval.IsCompliant != false {
			t.Errorf("expected is_compliant to be false (0.0 < 0.20), got %v", eval.IsCompliant)
		}
	})

	t.Run("Compliance evaluation after liquidity reduction remains true when above threshold", func(t *testing.T) {
		shock := domain.MarketShockScenario{
			ShockName:         "Mild Shock",
			EquityDrawdown:    -0.05,
			CreditSpreadSpike: -0.02,
			LiquidityFreeze:   -0.15, // 0.40 - 0.15 = 0.25 >= 0.20
		}

		eval := evaluateShockScenario(st, riskTolerance, minLiquidity, shock)

		expectedLiq := 0.25
		if math.Abs(eval.StressedLiquidity-expectedLiq) > 1e-6 {
			t.Errorf("expected stressed liquidity %f, got %f", expectedLiq, eval.StressedLiquidity)
		}
		if eval.IsCompliant != true {
			t.Errorf("expected is_compliant to be true (0.25 >= 0.20), got %v", eval.IsCompliant)
		}
	})

	t.Run("Equity drawdown and credit spread spike correctly calculate return loss", func(t *testing.T) {
		stressState := &domain.FinancialState{
			TotalCapital:   100.0,
			ExpectedReturn: 0.10,
			LiquidityRatio: 0.50,
			RiskScore:      0.05,
			Allocations: map[string]float64{
				"bluechip_equity": 40.0,
				"corp_paper":      30.0,
			},
		}

		shock := domain.MarketShockScenario{
			ShockName:         "Severe Crash",
			EquityDrawdown:    -0.20,
			CreditSpreadSpike: -0.10,
			LiquidityFreeze:   -0.10,
		}

		eval := evaluateShockScenario(stressState, riskTolerance, minLiquidity, shock)

		// Expected shockReturnLoss = (40.0 * 0.20 + 30.0 * 0.10) / 100.0 = (8.0 + 3.0) / 100.0 = 0.11
		// Expected stressedReturn = 0.10 - 0.11 = -0.01
		expectedReturn := -0.01
		if math.Abs(eval.StressedReturn-expectedReturn) > 1e-6 {
			t.Errorf("expected stressed return %f, got %f", expectedReturn, eval.StressedReturn)
		}
	})
}
