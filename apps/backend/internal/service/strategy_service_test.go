package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// MockStrategyRepository implements repository.StrategyRepository for unit testing.
type MockStrategyRepository struct {
	createLogFn            func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error)
	getLogByIDFn           func(ctx context.Context, id string) (*domain.StrategyExecutionLog, error)
	listLogsByTreasuryIDFn func(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error)
}

func (m *MockStrategyRepository) CreateLog(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
	if m.createLogFn != nil {
		return m.createLogFn(ctx, log)
	}
	return nil, nil
}

func (m *MockStrategyRepository) GetLogByID(ctx context.Context, id string) (*domain.StrategyExecutionLog, error) {
	if m.getLogByIDFn != nil {
		return m.getLogByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStrategyRepository) ListLogsByTreasuryID(ctx context.Context, treasuryID string, limit, offset int) ([]domain.StrategyExecutionLog, error) {
	if m.listLogsByTreasuryIDFn != nil {
		return m.listLogsByTreasuryIDFn(ctx, treasuryID, limit, offset)
	}
	return nil, nil
}

// MockAIClient implements clients.AIClient for unit testing.
type MockAIClient struct {
	healthFn                func(ctx context.Context) error
	generateStrategyFn      func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error)
	compareSearchesFn       func(ctx context.Context, problem any) (json.RawMessage, error)
	analyzeRiskFn           func(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error)
	adversarialStressTestFn func(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error)
}

func (m *MockAIClient) Health(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

func (m *MockAIClient) GenerateStrategy(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
	if m.generateStrategyFn != nil {
		return m.generateStrategyFn(ctx, problem, algorithm)
	}
	return nil, nil
}

func (m *MockAIClient) CompareSearches(ctx context.Context, problem any) (json.RawMessage, error) {
	if m.compareSearchesFn != nil {
		return m.compareSearchesFn(ctx, problem)
	}
	return json.RawMessage(`{}`), nil
}

func (m *MockAIClient) AnalyzeRisk(ctx context.Context, req domain.AIRiskAnalyzeRequest) (*domain.AIRiskAnalyzeResponse, error) {
	if m.analyzeRiskFn != nil {
		return m.analyzeRiskFn(ctx, req)
	}
	return nil, nil
}

func (m *MockAIClient) AdversarialStressTest(ctx context.Context, problem any) (*domain.AdversarialStressTestResult, error) {
	if m.adversarialStressTestFn != nil {
		return m.adversarialStressTestFn(ctx, problem)
	}
	return nil, nil
}

func TestStrategyService_GenerateStrategy(t *testing.T) {
	ctx := context.Background()
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleInput := domain.GenerateStrategyInput{
		Algorithm: "astar",
		Assets: []domain.Asset{
			{
				ID:             "gov_bond",
				Name:           "Government Bond",
				AssetType:      "gov_bond",
				ExpectedReturn: 0.07,
				Risk:           0.05,
				Liquidity:      0.95,
			},
		},
		Liabilities: []domain.Liability{
			{
				ID:           "debt_1",
				Name:         "Term Debt",
				Principal:    10.0,
				InterestRate: 0.08,
				Priority:     1,
			},
		},
		Constraints: []domain.Constraint{
			{
				ID:             "c_min_liq",
				ConstraintType: "minimum_liquidity",
				Parameters:     map[string]any{"threshold": 0.20},
			},
		},
	}

	t.Run("Invalid treasury ID returns ErrInvalidID", func(t *testing.T) {
		s := NewStrategyService(nil, nil, nil)
		_, err := s.GenerateStrategy(ctx, "invalid-id", sampleInput)
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("expected ErrInvalidID, got %v", err)
		}
	})

	t.Run("Invalid algorithm returns error", func(t *testing.T) {
		s := NewStrategyService(nil, nil, nil)
		input := sampleInput
		input.Algorithm = "unsupported_genetic_algorithm"
		_, err := s.GenerateStrategy(ctx, validTreasuryID, input)
		if err == nil {
			t.Fatalf("expected algorithm validation error, got nil")
		}
	})

	t.Run("Treasury not found returns ErrNotFound", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}
		s := NewStrategyService(treasuryRepo, nil, nil)
		_, err := s.GenerateStrategy(ctx, validTreasuryID, sampleInput)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("AI Engine down maps to ErrUpstreamUnavailable", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return nil, domain.ErrUpstreamUnavailable
			},
		}
		s := NewStrategyService(treasuryRepo, nil, aiClient)
		_, err := s.GenerateStrategy(ctx, validTreasuryID, sampleInput)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got %v", err)
		}
	})

	t.Run("Step size defaults to capital div 20", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      200.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		var capturedStepSize float64
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				prob := problem.(domain.FinancialProblem)
				capturedStepSize = prob.StepSize
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{Explanation: "OK"},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{Allocations: map[string]float64{}},
					},
				}, nil
			},
		}
		strategyRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				return &log, nil
			},
		}
		s := NewStrategyService(treasuryRepo, strategyRepo, aiClient)

		input := sampleInput
		input.StepSize = nil
		_, err := s.GenerateStrategy(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedStepSize := 200.0 / 20.0 // 10.0
		if capturedStepSize != expectedStepSize {
			t.Fatalf("expected stepSize %f, got %f", expectedStepSize, capturedStepSize)
		}
	})

	t.Run("Explicit step_size is respected", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		var capturedStepSize float64
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				prob := problem.(domain.FinancialProblem)
				capturedStepSize = prob.StepSize
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{Explanation: "OK"},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{Allocations: map[string]float64{}},
					},
				}, nil
			},
		}
		strategyRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				return &log, nil
			},
		}
		s := NewStrategyService(treasuryRepo, strategyRepo, aiClient)

		customStep := 25.0
		input := sampleInput
		input.StepSize = &customStep
		_, err := s.GenerateStrategy(ctx, validTreasuryID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedStepSize != customStep {
			t.Fatalf("expected stepSize %f, got %f", customStep, capturedStepSize)
		}
	})

	t.Run("Step size <= 0 returns validation error", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		s := NewStrategyService(treasuryRepo, nil, nil)

		zeroStep := 0.0
		input := sampleInput
		input.StepSize = &zeroStep
		_, err := s.GenerateStrategy(ctx, validTreasuryID, input)
		if err == nil {
			t.Fatal("expected validation error for step_size <= 0, got nil")
		}
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) || valErr.Details[0] != "step_size must be greater than 0" {
			t.Fatalf("expected 'step_size must be greater than 0', got: %v", err)
		}
	})

	t.Run("Step size too small returns validation error", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					TotalCapital:      100.0,
					RiskTolerance:     0.5,
					MinLiquidityRatio: 0.2,
				}, nil
			},
		}
		s := NewStrategyService(treasuryRepo, nil, nil)

		// capital 100 / step 0.1 = 1000 steps > 200 steps
		tinyStep := 0.1
		input := sampleInput
		input.StepSize = &tinyStep
		_, err := s.GenerateStrategy(ctx, validTreasuryID, input)
		if err == nil {
			t.Fatal("expected validation error for too many steps, got nil")
		}
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) || valErr.Details[0] != "step_size too small for capital, max 200 steps" {
			t.Fatalf("expected 'step_size too small for capital, max 200 steps', got: %v", err)
		}
	})

	t.Run("Successful strategy generation and log persistence", func(t *testing.T) {
		treasuryRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{
					ID:                id,
					Name:              "Acme Treasury",
					TotalCapital:      100.0,
					RiskTolerance:     0.50,
					MinLiquidityRatio: 0.20,
				}, nil
			},
		}
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{
						ExpectedReturn: 0.0825,
						RiskScore:      0.1150,
						LiquidityRatio: 0.9200,
						ObjectiveValue: 0.2850,
						Explanation:    "Optimal portfolio achieved via A* Search",
					},
					SearchResult: clients.AISearchResult{
						Algorithm: algorithm,
						Success:   true,
						FinalState: &clients.AIFinalState{
							Allocations: map[string]float64{"gov_bond": 80.0},
						},
					},
				}, nil
			},
		}
		strategyRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "log-uuid-123"
				log.CreatedAt = time.Now()
				return &log, nil
			},
		}

		s := NewStrategyService(treasuryRepo, strategyRepo, aiClient)
		resp, err := s.GenerateStrategy(ctx, validTreasuryID, sampleInput)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.ID != "log-uuid-123" {
			t.Errorf("expected ID 'log-uuid-123', got %s", resp.ID)
		}
		if resp.Explanation != "Optimal portfolio achieved via A* Search" {
			t.Errorf("unexpected explanation: %s", resp.Explanation)
		}
		if resp.ExpectedReturn != 0.0825 {
			t.Errorf("expected return 0.0825, got %f", resp.ExpectedReturn)
		}
	})
}
