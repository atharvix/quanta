package verifier_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/verifier"
)

func baseVerifyInput(t *testing.T) (verifier.VerifyInput, uuid.UUID, uuid.UUID) {
	t.Helper()
	accID := uuid.New()
	cpID := uuid.New()
	in := verifier.VerifyInput{
		BaseCurrency:            "USD",
		TotalAvailableCash:      money.MustFromString("1500.0000"),
		InvestableCapital:       money.MustFromString("1000.0000"),
		OperatingLiquidityFloor: money.MustFromString("300.0000"),
		EmergencyReserveFloor:   money.MustFromString("200.0000"),
		Candidates: []verifier.CandidateSpec{
			{
				ID:             "mmf_a",
				MinAllocation:  money.Zero(),
				MaxAllocation:  money.MustFromString("800.0000"),
				CounterpartyID: &cpID,
			},
			{
				ID:            "tbill_b",
				MinAllocation: money.Zero(),
				MaxAllocation: money.MustFromString("1000.0000"),
			},
		},
		Counterparties: map[uuid.UUID]domain.Counterparty{
			cpID: {
				ID:               cpID,
				Name:             "Alpha MMF",
				CurrencyCode:     "USD",
				CurrentExposure:  money.MustFromString("100.0000"),
				MaxExposureLimit: money.MustFromString("500.0000"),
			},
		},
		ExpectedAccountVersions: map[uuid.UUID]int64{accID: 2},
		CurrentAccountVersions:  map[uuid.UUID]int64{accID: 2},
		ExpectedPolicyVersion:   1,
		CurrentPolicyVersion:    1,
		Response: intelligence.OptimizeResponse{
			Status:          "COMPLETED",
			Feasible:        true,
			Allocations:     map[string]string{"mmf_a": "350.0000", "tbill_b": "650.0000"},
			UnallocatedCash: "0.0000",
		},
	}
	return in, accID, cpID
}

func TestVerifyOptimization_ValidPasses(t *testing.T) {
	in, _, _ := baseVerifyInput(t)
	res := verifier.VerifyOptimization(in)
	if !res.Passed || len(res.Failures) > 0 {
		t.Fatalf("expected verification to pass, got failures: %v", res.Failures)
	}
}

func TestVerifyOptimization_RejectsConservationDriftCounterpartyAndStaleVersions(t *testing.T) {
	// 1. Capital conservation penny drift
	in1, _, _ := baseVerifyInput(t)
	in1.Response.UnallocatedCash = "0.0001"
	if res := verifier.VerifyOptimization(in1); res.Passed {
		t.Fatal("expected penny drift to fail capital conservation")
	}

	// 2. Counterparty limit exceeded (100 current + 450 proposed = 550 > 500)
	in2, _, _ := baseVerifyInput(t)
	in2.Response.Allocations["mmf_a"] = "450.0000"
	in2.Response.Allocations["tbill_b"] = "550.0000"
	if res := verifier.VerifyOptimization(in2); res.Passed {
		t.Fatal("expected counterparty cap violation to fail")
	}

	// 3. Stale account version
	in3, accID, _ := baseVerifyInput(t)
	in3.CurrentAccountVersions[accID] = 3
	if res := verifier.VerifyOptimization(in3); res.Passed {
		t.Fatal("expected stale account version to fail")
	}

	// 4. Liquidity reserve floor breach
	in4, _, _ := baseVerifyInput(t)
	in4.TotalAvailableCash = money.MustFromString("1200.0000") // 1200 - 1000 = 200 < 500 floor
	if res := verifier.VerifyOptimization(in4); res.Passed {
		t.Fatal("expected liquidity floor breach to fail")
	}
}
