package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

type MockFPRepository struct {
	createFn  func(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error)
	getByIDFn func(ctx context.Context, id string) (*domain.FinancialProblemEntity, error)
	listFn    func(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error)
}

func (m *MockFPRepository) Create(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error) {
	if m.createFn != nil {
		return m.createFn(ctx, fp)
	}
	return nil, nil
}

func (m *MockFPRepository) GetByID(ctx context.Context, id string) (*domain.FinancialProblemEntity, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockFPRepository) List(ctx context.Context, limit, offset int, treasuryID string) ([]domain.FinancialProblemEntity, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset, treasuryID)
	}
	return nil, nil
}

func TestFinancialProblemService_Create(t *testing.T) {
	ctx := context.Background()
	validTreasuryID := "123e4567-e89b-12d3-a456-426614174000"

	sampleInput := domain.CreateFinancialProblemInput{
		TreasuryID: validTreasuryID,
		Algorithm:  "astar",
		Problem: domain.FinancialProblem{
			Assets: []domain.Asset{
				{
					ID:             "gov_bond",
					Name:           "Bond",
					AssetType:      "gov_bond",
					ExpectedReturn: 0.05,
					Risk:           0.02,
					Liquidity:      0.95,
				},
			},
			Liabilities: []domain.Liability{},
			Constraints: []domain.Constraint{},
		},
	}

	t.Run("Treasury not found returns ErrNotFound", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return nil, domain.ErrNotFound
			},
		}
		svc := NewFinancialProblemService(tRepo, nil, nil, nil, nil)
		_, err := svc.Create(ctx, sampleInput)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Validation error on empty assets", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
			getByIDFn: func(ctx context.Context, id string) (*domain.Treasury, error) {
				return &domain.Treasury{ID: id, TotalCapital: 100.0}, nil
			},
		}
		svc := NewFinancialProblemService(tRepo, nil, nil, nil, nil)
		badInput := sampleInput
		badInput.Problem.Assets = []domain.Asset{}
		_, err := svc.Create(ctx, badInput)
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
	})

	t.Run("Successful creation and strategy linkage", func(t *testing.T) {
		tRepo := &MockTreasuryRepository{
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
				return &clients.AIGenerateResponse{
					Strategy: clients.AIStrategy{
						ExpectedReturn: 0.08,
						Explanation:    "AI strategy achieved",
					},
					SearchResult: clients.AISearchResult{
						FinalState: &clients.AIFinalState{Allocations: map[string]float64{"gov_bond": 100.0}},
					},
				}, nil
			},
		}
		stratRepo := &MockStrategyRepository{
			createLogFn: func(ctx context.Context, log domain.StrategyExecutionLog) (*domain.StrategyExecutionLog, error) {
				log.ID = "strat-123"
				return &log, nil
			},
		}
		fpRepo := &MockFPRepository{
			createFn: func(ctx context.Context, fp domain.FinancialProblemEntity) (*domain.FinancialProblemEntity, error) {
				fp.ID = "fp-456"
				fp.CreatedAt = time.Now()
				return &fp, nil
			},
		}

		svc := NewFinancialProblemService(tRepo, fpRepo, stratRepo, aiClient, nil)
		resp, err := svc.Create(ctx, sampleInput)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ID != "fp-456" {
			t.Errorf("expected ID 'fp-456', got %s", resp.ID)
		}
		if resp.Strategy.ID != "strat-123" {
			t.Errorf("expected strategy ID 'strat-123', got %s", resp.Strategy.ID)
		}
	})
}
