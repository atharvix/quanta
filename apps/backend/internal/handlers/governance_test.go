package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/router"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

func (s *apiMemStore) CreateRun(_ context.Context, run domain.OptimizationRun) error {
	run.CreatedAt = time.Now().UTC()
	s.runs[run.ID] = run
	return nil
}

func (s *apiMemStore) GetRun(_ context.Context, orgID, runID uuid.UUID) (domain.OptimizationRun, error) {
	r, ok := s.runs[runID]
	if !ok || r.OrganizationID != orgID {
		return domain.OptimizationRun{}, domain.ErrNotFound
	}
	return r, nil
}

func (s *apiMemStore) ListRunsByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]domain.OptimizationRun, error) {
	var out []domain.OptimizationRun
	for _, r := range s.runs {
		if r.OrganizationID == orgID && r.TreasuryEntityID == entityID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *apiMemStore) CreateProposal(_ context.Context, p domain.AllocationProposal) error {
	p.CreatedAt = time.Now().UTC()
	s.proposals[p.ID] = p
	return nil
}

func (s *apiMemStore) GetProposal(_ context.Context, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error) {
	p, ok := s.proposals[proposalID]
	if !ok || p.OrganizationID != orgID {
		return domain.AllocationProposal{}, domain.ErrNotFound
	}
	return p, nil
}

func (s *apiMemStore) GetProposalForUpdate(ctx context.Context, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error) {
	return s.GetProposal(ctx, orgID, proposalID)
}

func (s *apiMemStore) ListProposalsByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]domain.AllocationProposal, error) {
	var out []domain.AllocationProposal
	for _, p := range s.proposals {
		if p.OrganizationID == orgID && p.TreasuryEntityID == entityID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *apiMemStore) UpdateProposalStatus(
	_ context.Context,
	orgID, proposalID uuid.UUID,
	expectedVersion int64,
	newStatus domain.ProposalStatus,
	submittedBy *uuid.UUID,
	submittedAtUnix, expiresAtUnix *int64,
) (domain.AllocationProposal, error) {
	p, ok := s.proposals[proposalID]
	if !ok || p.OrganizationID != orgID {
		return domain.AllocationProposal{}, domain.ErrNotFound
	}
	if p.Version != expectedVersion {
		return domain.AllocationProposal{}, domain.ErrVersionConflict
	}
	p.Status = newStatus
	if submittedBy != nil {
		p.SubmittedBy = submittedBy
	}
	if submittedAtUnix != nil {
		t := time.Unix(*submittedAtUnix, 0).UTC()
		p.SubmittedAt = &t
	}
	if expiresAtUnix != nil {
		t := time.Unix(*expiresAtUnix, 0).UTC()
		p.ExpiresAt = &t
	}
	p.Version++
	s.proposals[proposalID] = p
	return p, nil
}

func (s *apiMemStore) AppendDecision(_ context.Context, d domain.ApprovalDecision) error {
	d.DecidedAt = time.Now().UTC()
	s.decisions = append(s.decisions, d)
	return nil
}

func (s *apiMemStore) ListDecisionsByProposal(_ context.Context, orgID, proposalID uuid.UUID) ([]domain.ApprovalDecision, error) {
	var out []domain.ApprovalDecision
	for _, d := range s.decisions {
		if d.OrganizationID == orgID && d.ProposalID == proposalID {
			out = append(out, d)
		}
	}
	return out, nil
}

func setupGovernanceTestEnv(t *testing.T, intelURL string) (*apiTestEnv, http.Handler) {
	t.Helper()
	base := setupAPITestEnv(t)
	tokenSvc, _ := auth.NewTokenService("test-jwt-secret-32-characters-minimum!", "quanta-backend", "quanta-api", time.Hour)

	// Seed 1500.0000 USD available cash and a 500.0000 reserve floor (300 op + 200 em)
	acc := base.store.accounts[base.accountID]
	acc.LedgerBalance = money.MustFromString("1500.0000")
	base.store.accounts[base.accountID] = acc

	polID := uuid.New()
	base.store.policies[polID] = domain.LiquidityPolicy{
		ID:                    polID,
		OrganizationID:        base.orgAID,
		TreasuryEntityID:      base.entityAID,
		Version:               1,
		MinOperatingLiquidity: money.MustFromString("300.0000"),
		MinEmergencyReserve:   money.MustFromString("200.0000"),
		IsActive:              true,
		CreatedBy:             base.analystID,
	}

	intelClient := intelligence.NewHTTPClient(intelURL, 2*time.Second)
	authSvc := service.NewAuthService(base.store, base.store, tokenSvc)
	accountSvc := service.NewAccountService(base.store, base.store, base.store, base.store)
	ledgerSvc := service.NewLedgerService(base.store)
	policySvc := service.NewPolicyService(base.store, base.store, base.store)
	cpSvc := service.NewCounterpartyService(base.store, base.store, base.store)
	optSvc := service.NewOptimizationService(base.store, base.store, base.store, base.store, base.store, base.store, intelClient)
	propSvc := service.NewProposalService(base.store, base.store, base.store, base.store, base.store, base.store, base.store, base.store, base.store)

	r := router.NewRouter(&config.Config{}, router.Dependencies{
		Health:       handlers.NewHealthHandler(stubPinger{}, stubIntelligence{}),
		Auth:         handlers.NewAuthHandler(authSvc),
		Treasury:     handlers.NewTreasuryHandler(accountSvc, ledgerSvc),
		Policy:       handlers.NewPolicyHandler(policySvc),
		Counterparty: handlers.NewCounterpartyHandler(cpSvc),
		Intelligence: handlers.NewIntelligenceHandler(optSvc),
		Proposal:     handlers.NewProposalHandler(propSvc),
		Audit:        handlers.NewAuditHandler(base.store),
		Tokens:       tokenSvc,
		OrgRepo:      base.store,
	})
	return base, r
}

func TestGovernance_VerificationFailureBlocksProposalAndLifecycleEnforcesMakerCheckerAndStaleness(t *testing.T) {
	// Python mock server: first returns penny-drift allocation (fails Go verification), then valid allocation
	callNum := 0
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callNum++
		alloc := map[string]string{"mmf_a": "1000.0000"}
		unalloc := "0.0000"
		if callNum == 1 {
			unalloc = "0.0005" // breaks capital conservation!
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contract_version":                    "1.0",
			"correlation_id":                      r.Header.Get("X-Correlation-ID"),
			"status":                              "COMPLETED",
			"feasible":                            true,
			"solver_used":                         "scipy_linprog_highs",
			"solver_version":                      "1.18.1",
			"is_authoritative":                    false,
			"authoritative_verification_required": true,
			"allocations":                         alloc,
			"unallocated_cash":                    unalloc,
			"runtime_ms":                          4,
		})
	}))
	defer pySrv.Close()

	env, r := setupGovernanceTestEnv(t, pySrv.URL)
	analystTok := loginForToken(t, r, "analyst@quanta.local", "AnalystPass!2026")
	approverTok := loginForToken(t, r, "approver@quanta.local", "ApproverPass!2026")
	orgBase := "/api/v1/orgs/" + env.orgAID.String()
	optPath := orgBase + "/treasuries/" + env.entityAID.String() + "/optimize"

	optPayload := map[string]any{
		"investable_capital": "1000.0000",
		"solver_mode":        "lp",
		"candidates": []map[string]any{
			{"id": "mmf_a", "expected_return": "0.05", "min_allocation": "0", "max_allocation": "1000.0000"},
		},
	}

	// 1. Run 1 fails Go verification -> VERIFICATION_FAILED -> cannot create proposal
	rec1 := doJSONReq(t, r, http.MethodPost, optPath, analystTok, optPayload)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 with VERIFICATION_FAILED status, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var resp1 struct {
		OptimizationRun struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"optimization_run"`
	}
	_ = json.Unmarshal(rec1.Body.Bytes(), &resp1)
	if resp1.OptimizationRun.Status != "VERIFICATION_FAILED" {
		t.Fatalf("expected VERIFICATION_FAILED, got %s", resp1.OptimizationRun.Status)
	}

	propFail := doJSONReq(t, r, http.MethodPost, orgBase+"/proposals", analystTok, map[string]string{
		"optimization_run_id": resp1.OptimizationRun.ID,
	})
	if propFail.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 blocking proposal from VERIFICATION_FAILED run, got %d", propFail.Code)
	}

	// 2. Run 2 passes Go verification -> COMPLETED -> create & submit proposal
	rec2 := doJSONReq(t, r, http.MethodPost, optPath, analystTok, optPayload)
	var resp2 struct {
		OptimizationRun struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"optimization_run"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.OptimizationRun.Status != "COMPLETED" {
		t.Fatalf("expected COMPLETED, got %s", resp2.OptimizationRun.Status)
	}

	propRec := doJSONReq(t, r, http.MethodPost, orgBase+"/proposals", analystTok, map[string]string{
		"optimization_run_id": resp2.OptimizationRun.ID,
	})
	if propRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating proposal, got %d: %s", propRec.Code, propRec.Body.String())
	}
	var propBody struct {
		Proposal struct {
			ID string `json:"id"`
		} `json:"proposal"`
	}
	_ = json.Unmarshal(propRec.Body.Bytes(), &propBody)
	propID := propBody.Proposal.ID

	subRec := doJSONReq(t, r, http.MethodPost, orgBase+"/proposals/"+propID+"/submit", analystTok, map[string]int{"expires_in_minutes": 60})
	if subRec.Code != http.StatusOK {
		t.Fatalf("expected 200 submitting proposal, got %d", subRec.Code)
	}

	// 3. Analyst cannot approve own proposal (RBAC + Maker-Checker) -> 403
	selfApprove := doJSONReq(t, r, http.MethodPost, orgBase+"/proposals/"+propID+"/approve", analystTok, map[string]string{"rationale": "self"})
	if selfApprove.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on analyst self-approval, got %d", selfApprove.Code)
	}

	// 4. Simulate account version drift (stale proposal) -> 409 STALE_PROPOSAL_STATE
	acc := env.store.accounts[env.accountID]
	acc.Version++
	env.store.accounts[env.accountID] = acc

	staleRec := doJSONReq(t, r, http.MethodPost, orgBase+"/proposals/"+propID+"/approve", approverTok, map[string]string{"rationale": "approve stale"})
	if staleRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 STALE_PROPOSAL_STATE, got %d: %s", staleRec.Code, staleRec.Body.String())
	}

	// Restore account version so approval succeeds
	acc.Version--
	env.store.accounts[env.accountID] = acc

	okApprove := doJSONReq(t, r, http.MethodPost, orgBase+"/proposals/"+propID+"/approve", approverTok, map[string]string{"rationale": "verified live balances"})
	if okApprove.Code != http.StatusOK {
		t.Fatalf("expected 200 on approver approval, got %d: %s", okApprove.Code, okApprove.Body.String())
	}
}

func TestGovernance_PythonUnavailableDegradesGracefullyWithoutBreakingTreasuryReads(t *testing.T) {
	env, r := setupGovernanceTestEnv(t, "http://127.0.0.1:1")
	analystTok := loginForToken(t, r, "analyst@quanta.local", "AnalystPass!2026")
	orgBase := "/api/v1/orgs/" + env.orgAID.String()

	// Python optimize returns 503 UPSTREAM_UNAVAILABLE
	optRec := doJSONReq(t, r, http.MethodPost, orgBase+"/treasuries/"+env.entityAID.String()+"/optimize", analystTok, map[string]any{
		"investable_capital": "500.0000",
		"candidates":         []map[string]any{{"id": "mmf_a", "expected_return": "0.05", "max_allocation": "500.0000"}},
	})
	if optRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when Python intelligence is unreachable, got %d", optRec.Code)
	}

	// Core treasury account read still returns 200 OK
	readRec := doJSONReq(t, r, http.MethodGet, orgBase+"/accounts/"+env.accountID.String(), analystTok, nil)
	if readRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on treasury account read during Python outage, got %d", readRec.Code)
	}
}
