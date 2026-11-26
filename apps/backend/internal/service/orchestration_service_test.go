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

		svc := NewOrchestrationService(tRepo, orchRepo, nil, aiClient, nil)
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

	t.Run("Success with comparison and optimization not_available", func(t *testing.T) {
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
		stratRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "strat-orch-1"
				return &log, nil
			},
		}
		orchRepo := &MockOrchestrationRepository{
			createFn: func(ctx context.Context, run domain.OrchestrationRun) (*domain.OrchestrationRun, error) {
				run.ID = "run-success-1"
				run.CreatedAt = time.Now()
				return &run, nil
			},
		}

		svc := NewOrchestrationService(tRepo, orchRepo, stratRepo, aiClient, nil)
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
		if resp.Optimization.Status != "not_available" {
			t.Errorf("expected optimization status 'not_available', got %s", resp.Optimization.Status)
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

	svc := NewOrchestrationService(tRepo, orchRepo, nil, nil, nil)

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
