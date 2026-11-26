package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type MockOrchestrationRepository struct {
	createFn  func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error)
	getByIDFn func(ctx context.Context, id string) (*domain.OrchestrationRun, error)
}

func (m *MockOrchestrationRepository) Create(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
	if m.createFn != nil {
		return m.createFn(ctx, run)
	}
	return nil, nil
}

func (m *MockOrchestrationRepository) GetByID(ctx context.Context, id string) (*domain.OrchestrationRun, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

type MockOptimizationClient struct {
	healthFn            func(ctx context.Context) error
	optimizeClassicalFn func(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error)
	optimizeQuantumFn   func(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error)
	compareOptimizersFn func(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error)
}

func (m *MockOptimizationClient) Health(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

func (m *MockOptimizationClient) OptimizeClassical(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error) {
	if m.optimizeClassicalFn != nil {
		return m.optimizeClassicalFn(ctx, req)
	}
	return nil, nil
}

func (m *MockOptimizationClient) OptimizeQuantum(ctx context.Context, req clients.OptimizationRequest) (*clients.OptimizationResult, error) {
	if m.optimizeQuantumFn != nil {
		return m.optimizeQuantumFn(ctx, req)
	}
	return nil, nil
}

func (m *MockOptimizationClient) CompareOptimizers(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error) {
	if m.compareOptimizersFn != nil {
		return m.compareOptimizersFn(ctx, req)
	}
	return nil, nil
}

func TestOrchestrationService_Orchestrate(t *testing.T) {
	ctx := context.Background()
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleInput := domain.OrchestrateAllocationInput{
		TreasuryID: validTreasuryID,
		Algorithm:  "astar",
		Assets: []domain.Asset{
			{
				ID:             "gov_bond",
				Name:           "Bonds",
				AssetType:      "gov_bond",
				ExpectedReturn: 0.05,
				Risk:           0.02,
				Liquidity:      0.95,
			},
		},
		Liabilities:       []domain.Liability{},
		Constraints:       []domain.Constraint{},
		IncludeComparison: false,
	}

	t.Run("AI down records failed run and returns ErrUpstreamUnavailable", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0, RiskTolerance: 0.5, MinLiquidityRatio: 0.2}, nil
			},
		}
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return nil, errors.New("Post http://ai-engine:8000/api/v1/strategy/generate: dial tcp: lookup ai-engine: no such host")
			},
		}

		var recordedStatus string
		var recordedError string
		orchRepo := &MockOrchestrationRepository{
			createFn: func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
				recordedStatus = run.Status
				if run.ErrorMessage != nil {
					recordedError = *run.ErrorMessage
				}
				run.ID = "run-fail"
				return &run, nil
			},
		}

		svc := NewOrchestrationService(tRepo, orchRepo, nil, aiClient, nil, nil)
		_, err := svc.Orchestrate(ctx, sampleInput)
		if !errors.Is(err, domain.ErrUpstreamUnavailable) {
			t.Fatalf("expected ErrUpstreamUnavailable, got %v", err)
		}
		if recordedStatus != domain.OrchestrationStatusFailed {
			t.Errorf("expected failed run status, got %s", recordedStatus)
		}
		if recordedError != "AI engine unavailable" {
			t.Errorf("expected sanitized error 'AI engine unavailable', got '%s'", recordedError)
		}
		if errors.New(recordedError).Error() != "AI engine unavailable" {
			t.Errorf("error message should be exactly 'AI engine unavailable'")
		}
	})

	t.Run("Success with comparison and healthy optimization engine", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0, RiskTolerance: 0.5, MinLiquidityRatio: 0.2}, nil
			},
		}
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{Explanation: "Orchestrated allocation complete"},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{Allocations: map[string]float64{"gov_bond": 100.0}},
					},
				}, nil
			},
		}
		approx := 0.94
		optClient := &MockOptimizationClient{
			compareOptimizersFn: func(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error) {
				return &clients.OptimizationCompareResponse{
					ProblemSummary:         map[string]any{"capital": 100.0, "n_assets": 1},
					Results:                map[string]clients.OptimizationResult{"exact": {Method: "exact", Status: "optimal", Feasible: true}},
					SummaryTable:           []map[string]any{{"method": "exact", "objective_value": 0.05}},
					BestClassicalMethod:    "exact",
					QAOAApproximationRatio: &approx,
					SimulationNote:         "Statevector CPU simulation",
				}, nil
			},
		}
		stratRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "strat-orch-1"
				return &log, nil
			},
		}
		var savedOptStatus string
		orchRepo := &MockOrchestrationRepository{
			createFn: func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
				savedOptStatus = run.OptimizationStatus
				run.ID = "run-success-1"
				run.CreatedAt = time.Now()
				return &run, nil
			},
		}

		svc := NewOrchestrationService(tRepo, orchRepo, stratRepo, aiClient, optClient, nil)
		input := sampleInput
		input.IncludeComparison = true

		resp, err := svc.Orchestrate(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.RunID != "run-success-1" {
			t.Errorf("expected run ID 'run-success-1', got %s", resp.RunID)
		}
		if resp.Status != domain.OrchestrationStatusCompleted {
			t.Errorf("expected status 'completed', got %s", resp.Status)
		}
		if resp.Optimization.Status != "completed" || savedOptStatus != "completed" {
			t.Errorf("expected optimization status 'completed', got resp=%s, db=%s", resp.Optimization.Status, savedOptStatus)
		}
		if resp.Optimization.BestClassicalMethod != "exact" {
			t.Errorf("expected best_classical_method 'exact', got %s", resp.Optimization.BestClassicalMethod)
		}
		if resp.Optimization.QAOAApproximationRatio == nil || *resp.Optimization.QAOAApproximationRatio != 0.94 {
			t.Errorf("expected qaoa_approximation_ratio 0.94, got %v", resp.Optimization.QAOAApproximationRatio)
		}
	})

	t.Run("Degrades gracefully to unavailable when optimization engine fails", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0, RiskTolerance: 0.5, MinLiquidityRatio: 0.2}, nil
			},
		}
		aiClient := &MockAIClient{
			generateStrategyFn: func(ctx context.Context, problem any, algorithm string) (*clients.AIGenerateResponse, error) {
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{Explanation: "AI allocation complete"},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{Allocations: map[string]float64{"gov_bond": 100.0}},
					},
				}, nil
			},
		}
		optClient := &MockOptimizationClient{
			compareOptimizersFn: func(ctx context.Context, req clients.OptimizationCompareRequest) (*clients.OptimizationCompareResponse, error) {
				return nil, errors.New("connection refused on port 8001")
			},
		}
		stratRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "strat-orch-2"
				return &log, nil
			},
		}
		var savedOptStatus string
		orchRepo := &MockOrchestrationRepository{
			createFn: func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
				savedOptStatus = run.OptimizationStatus
				run.ID = "run-degraded-1"
				run.CreatedAt = time.Now()
				return &run, nil
			},
		}

		svc := NewOrchestrationService(tRepo, orchRepo, stratRepo, aiClient, optClient, nil)
		resp, err := svc.Orchestrate(ctx, sampleInput)
		if err != nil {
			t.Fatalf("expected orchestration to succeed gracefully, got: %v", err)
		}
		if resp.Status != domain.OrchestrationStatusCompleted {
			t.Errorf("expected completed overall status, got %s", resp.Status)
		}
		if resp.Optimization.Status != "unavailable" || savedOptStatus != "unavailable" {
			t.Errorf("expected optimization status 'unavailable', got resp=%s, db=%s", resp.Optimization.Status, savedOptStatus)
		}
		if resp.Optimization.Reason == "" {
			t.Errorf("expected non-empty error reason in degraded optimization status")
		}
	})
}

func TestOrchestrationService_GetRunByID_Authorization(t *testing.T) {
	ownerID := "user-owner-123"
	nonOwnerID := "user-analyst-456"
	adminID := "user-admin-789"
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"
	validRunID := "423e4567-e89b-12d3-a456-426614174000"

	tRepo := &MockTreasuryRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
			return &domain.Treasury{
				ID:        validTreasuryID,
				Name:      "Owned Corp",
				OwnerID:   &ownerID,
				CreatedAt: time.Now(),
			}, nil
		},
	}
	orchRepo := &MockOrchestrationRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.OrchestrationRun, error) {
			return &domain.OrchestrationRun{
				ID:         id,
				TreasuryID: validTreasuryID,
				Status:     domain.OrchestrationStatusCompleted,
			}, nil
		},
	}

	svc := NewOrchestrationService(tRepo, orchRepo, nil, nil, nil, nil)

	ownerCtx := domain.WithUserContext(context.Background(), ownerID, "owner@example.com", domain.RoleAnalyst)
	nonOwnerCtx := domain.WithUserContext(context.Background(), nonOwnerID, "other@example.com", domain.RoleAnalyst)
	adminCtx := domain.WithUserContext(context.Background(), adminID, "admin@example.com", domain.RoleAdmin)

	t.Run("GetRunByID - Owner access succeeds", func(t *testing.T) {
		run, err := svc.GetRunByID(ownerCtx, validRunID)
		if err != nil {
			t.Fatalf("unexpected error for owner: %v", err)
		}
		if run.ID != validRunID {
			t.Errorf("expected ID '%s', got '%s'", validRunID, run.ID)
		}
	})

	t.Run("GetRunByID - Non-owner returns ErrForbidden", func(t *testing.T) {
		_, err := svc.GetRunByID(nonOwnerCtx, validRunID)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for non-owner, got: %v", err)
		}
	})

	t.Run("GetRunByID - Admin access succeeds", func(t *testing.T) {
		run, err := svc.GetRunByID(adminCtx, validRunID)
		if err != nil {
			t.Fatalf("unexpected error for admin: %v", err)
		}
		if run.ID != validRunID {
			t.Errorf("expected ID '%s', got '%s'", validRunID, run.ID)
		}
	})
}
