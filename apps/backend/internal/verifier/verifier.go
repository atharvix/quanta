package verifier

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/money"
)

// CandidateSpec defines authoritative Go-side bounds and counterparty mapping for an allocation candidate.
type CandidateSpec struct {
	ID             string
	MinAllocation  money.Amount
	MaxAllocation  money.Amount
	CounterpartyID *uuid.UUID
}

// VerifyInput holds authoritative Go state and an untrusted Python OptimizeResponse.
type VerifyInput struct {
	BaseCurrency            string
	TotalAvailableCash      money.Amount
	InvestableCapital       money.Amount
	OperatingLiquidityFloor money.Amount
	EmergencyReserveFloor   money.Amount
	Candidates              []CandidateSpec
	Counterparties          map[uuid.UUID]domain.Counterparty
	ExpectedAccountVersions map[uuid.UUID]int64
	CurrentAccountVersions  map[uuid.UUID]int64
	ExpectedPolicyVersion   int64
	CurrentPolicyVersion    int64
	Response                intelligence.OptimizeResponse
}

// VerificationResult summarizes deterministic financial verification outcomes.
type VerificationResult struct {
	Passed              bool
	TotalAllocated      money.Amount
	UnallocatedCash     money.Amount
	VerifiedAllocations map[string]money.Amount
	Failures            []string
}

// VerifyOptimization deterministically verifies an untrusted Python optimization response using exact decimals.
func VerifyOptimization(in VerifyInput) VerificationResult {
	res := VerificationResult{
		VerifiedAllocations: make(map[string]money.Amount, len(in.Response.Allocations)),
	}

	if in.Response.Status != "COMPLETED" || !in.Response.Feasible {
		res.Failures = append(res.Failures, fmt.Sprintf("solver status %q (feasible=%v) is not eligible for proposal creation", in.Response.Status, in.Response.Feasible))
		return res
	}

	// 1. Account and policy version freshness
	if in.ExpectedPolicyVersion != in.CurrentPolicyVersion {
		res.Failures = append(res.Failures, fmt.Sprintf("stale policy version: expected %d, current %d", in.ExpectedPolicyVersion, in.CurrentPolicyVersion))
	}
	for accID, expVer := range in.ExpectedAccountVersions {
		curVer, ok := in.CurrentAccountVersions[accID]
		if !ok || curVer != expVer {
			res.Failures = append(res.Failures, fmt.Sprintf("stale account version for %s: expected %d, current %d", accID, expVer, curVer))
		}
	}

	// 2. Operating & emergency reserve floor invariant
	requiredFloor := in.OperatingLiquidityFloor.Add(in.EmergencyReserveFloor)
	retainedCash := in.TotalAvailableCash.Sub(in.InvestableCapital)
	if retainedCash.IsNegative() || retainedCash.Cmp(requiredFloor) < 0 {
		res.Failures = append(res.Failures, fmt.Sprintf("retained cash %s breaches reserve floor %s", retainedCash.String(), requiredFloor.String()))
	}

	// 3. Candidate whitelist, decimal parsing, and individual bounds
	candMap := make(map[string]CandidateSpec, len(in.Candidates))
	for _, c := range in.Candidates {
		candMap[c.ID] = c
	}

	totalAlloc := money.Zero()
	cpIncremental := make(map[uuid.UUID]money.Amount)

	for id, rawAmt := range in.Response.Allocations {
		cand, exists := candMap[id]
		if !exists {
			res.Failures = append(res.Failures, fmt.Sprintf("unknown candidate %q returned by solver", id))
			continue
		}
		amt, err := money.FromString(rawAmt)
		if err != nil {
			res.Failures = append(res.Failures, fmt.Sprintf("invalid decimal amount for %q: %v", id, err))
			continue
		}
		if amt.IsNegative() {
			res.Failures = append(res.Failures, fmt.Sprintf("negative allocation %s for %q", amt.String(), id))
			continue
		}
		if amt.Cmp(cand.MinAllocation) < 0 || amt.Cmp(cand.MaxAllocation) > 0 {
			res.Failures = append(res.Failures, fmt.Sprintf("allocation %s for %q outside bounds [%s, %s]", amt.String(), id, cand.MinAllocation.String(), cand.MaxAllocation.String()))
		}
		res.VerifiedAllocations[id] = amt
		totalAlloc = totalAlloc.Add(amt)

		if cand.CounterpartyID != nil {
			prev := cpIncremental[*cand.CounterpartyID]
			if prev.Equal(money.Amount{}) {
				prev = money.Zero()
			}
			cpIncremental[*cand.CounterpartyID] = prev.Add(amt)
		}
	}

	// Ensure candidates omitted from map but with MinAllocation > 0 are caught
	for _, c := range in.Candidates {
		if _, ok := res.VerifiedAllocations[c.ID]; !ok {
			if c.MinAllocation.IsPositive() {
				res.Failures = append(res.Failures, fmt.Sprintf("missing candidate %q with min_allocation %s", c.ID, c.MinAllocation.String()))
			}
			res.VerifiedAllocations[c.ID] = money.Zero()
		}
	}

	// 4. Capital conservation
	unallocated, err := money.FromString(in.Response.UnallocatedCash)
	if err != nil || unallocated.IsNegative() {
		res.Failures = append(res.Failures, fmt.Sprintf("invalid unallocated_cash %q", in.Response.UnallocatedCash))
	} else {
		res.UnallocatedCash = unallocated
		if !totalAlloc.Add(unallocated).Equal(in.InvestableCapital) {
			res.Failures = append(res.Failures, fmt.Sprintf("capital conservation failed: allocated (%s) + unallocated (%s) != investable (%s)", totalAlloc.String(), unallocated.String(), in.InvestableCapital.String()))
		}
	}
	res.TotalAllocated = totalAlloc

	// 5. Counterparty exposure limits & currency consistency
	baseCurr := strings.ToUpper(strings.TrimSpace(in.BaseCurrency))
	for cpID, addExp := range cpIncremental {
		cp, ok := in.Counterparties[cpID]
		if !ok {
			res.Failures = append(res.Failures, fmt.Sprintf("counterparty %s not found in authoritative state", cpID))
			continue
		}
		if strings.ToUpper(strings.TrimSpace(cp.CurrencyCode)) != baseCurr {
			res.Failures = append(res.Failures, fmt.Sprintf("currency mismatch for counterparty %s: %s != %s", cpID, cp.CurrencyCode, baseCurr))
			continue
		}
		projected := cp.CurrentExposure.Add(addExp)
		if projected.Cmp(cp.MaxExposureLimit) > 0 {
			res.Failures = append(res.Failures, fmt.Sprintf("counterparty %s exposure %s exceeds limit %s", cp.Name, projected.String(), cp.MaxExposureLimit.String()))
		}
	}

	res.Passed = len(res.Failures) == 0
	return res
}
