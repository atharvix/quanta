package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/atharvix/quanta/apps/backend/internal/router"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type apiMemStore struct {
	users          map[uuid.UUID]domain.User
	orgs           map[uuid.UUID]domain.Organization
	memberships    []domain.OrganizationMembership
	entities       map[uuid.UUID]domain.TreasuryEntity
	accounts       map[uuid.UUID]domain.TreasuryAccount
	entries        []domain.LedgerEntry
	policies       map[uuid.UUID]domain.LiquidityPolicy
	counterparties map[uuid.UUID]domain.Counterparty
	runs           map[uuid.UUID]domain.OptimizationRun
	proposals      map[uuid.UUID]domain.AllocationProposal
	decisions      []domain.ApprovalDecision
	audits         []domain.AuditEvent
}

func newAPIMemStore() *apiMemStore {
	return &apiMemStore{
		users:          make(map[uuid.UUID]domain.User),
		orgs:           make(map[uuid.UUID]domain.Organization),
		entities:       make(map[uuid.UUID]domain.TreasuryEntity),
		accounts:       make(map[uuid.UUID]domain.TreasuryAccount),
		policies:       make(map[uuid.UUID]domain.LiquidityPolicy),
		counterparties: make(map[uuid.UUID]domain.Counterparty),
		runs:           make(map[uuid.UUID]domain.OptimizationRun),
		proposals:      make(map[uuid.UUID]domain.AllocationProposal),
	}
}

func (s *apiMemStore) WithTx(ctx context.Context, fn func(ctx context.Context, repos repository.TxRepositories) error) error {
	return fn(ctx, repository.TxRepositories{Accounts: s, Ledger: s, Audit: s, Proposals: s, Decisions: s})
}

func (s *apiMemStore) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	for _, u := range s.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *apiMemStore) GetUserByID(_ context.Context, userID uuid.UUID) (domain.User, error) {
	u, ok := s.users[userID]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (s *apiMemStore) ListMembershipsByUser(_ context.Context, userID uuid.UUID) ([]domain.OrganizationMembership, error) {
	var out []domain.OrganizationMembership
	for _, m := range s.memberships {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *apiMemStore) CreateOrganization(_ context.Context, org domain.Organization) error {
	s.orgs[org.ID] = org
	return nil
}

func (s *apiMemStore) GetOrganization(_ context.Context, orgID uuid.UUID) (domain.Organization, error) {
	org, ok := s.orgs[orgID]
	if !ok {
		return domain.Organization{}, domain.ErrNotFound
	}
	return org, nil
}

func (s *apiMemStore) GetMembership(_ context.Context, orgID, userID uuid.UUID) (domain.OrganizationMembership, error) {
	for _, m := range s.memberships {
		if m.OrganizationID == orgID && m.UserID == userID {
			return m, nil
		}
	}
	return domain.OrganizationMembership{}, domain.ErrNotFound
}

func (s *apiMemStore) CreateTreasuryEntity(_ context.Context, entity domain.TreasuryEntity) error {
	s.entities[entity.ID] = entity
	return nil
}

func (s *apiMemStore) GetTreasuryEntity(_ context.Context, orgID, entityID uuid.UUID) (domain.TreasuryEntity, error) {
	e, ok := s.entities[entityID]
	if !ok || e.OrganizationID != orgID {
		return domain.TreasuryEntity{}, domain.ErrNotFound
	}
	return e, nil
}

func (s *apiMemStore) ListTreasuryEntities(_ context.Context, orgID uuid.UUID) ([]domain.TreasuryEntity, error) {
	var out []domain.TreasuryEntity
	for _, e := range s.entities {
		if e.OrganizationID == orgID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *apiMemStore) CreateAccount(_ context.Context, account domain.TreasuryAccount) error {
	s.accounts[account.ID] = account
	return nil
}

func (s *apiMemStore) GetAccount(_ context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	acc, ok := s.accounts[accountID]
	if !ok || acc.OrganizationID != orgID {
		return domain.TreasuryAccount{}, domain.ErrNotFound
	}
	return acc, nil
}

func (s *apiMemStore) GetAccountForUpdate(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	return s.GetAccount(ctx, orgID, accountID)
}

func (s *apiMemStore) ListAccountsByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]domain.TreasuryAccount, error) {
	var out []domain.TreasuryAccount
	for _, acc := range s.accounts {
		if acc.OrganizationID == orgID && acc.TreasuryEntityID == entityID {
			out = append(out, acc)
		}
	}
	return out, nil
}

func (s *apiMemStore) UpdateAccountProjection(_ context.Context, orgID, accountID uuid.UUID, expectedVersion int64, newLedger, newEnc money.Amount) (domain.TreasuryAccount, error) {
	acc, ok := s.accounts[accountID]
	if !ok || acc.OrganizationID != orgID {
		return domain.TreasuryAccount{}, domain.ErrNotFound
	}
	if acc.Version != expectedVersion {
		return domain.TreasuryAccount{}, domain.ErrVersionConflict
	}
	acc.LedgerBalance = newLedger
	acc.EncumberedBalance = newEnc
	acc.Version++
	s.accounts[accountID] = acc
	return acc, nil
}

func (s *apiMemStore) AppendEntry(_ context.Context, entry domain.LedgerEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *apiMemStore) GetEntry(_ context.Context, orgID, entryID uuid.UUID) (domain.LedgerEntry, error) {
	for _, e := range s.entries {
		if e.OrganizationID == orgID && e.ID == entryID {
			return e, nil
		}
	}
	return domain.LedgerEntry{}, domain.ErrNotFound
}

func (s *apiMemStore) HasReversalForEntry(_ context.Context, orgID, entryID uuid.UUID) (bool, error) {
	for _, e := range s.entries {
		if e.OrganizationID == orgID && e.ReversedEntryID != nil && *e.ReversedEntryID == entryID {
			return true, nil
		}
	}
	return false, nil
}

func (s *apiMemStore) ListEntriesByAccount(_ context.Context, orgID, accountID uuid.UUID) ([]domain.LedgerEntry, error) {
	var out []domain.LedgerEntry
	for _, e := range s.entries {
		if e.OrganizationID == orgID && e.AccountID == accountID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *apiMemStore) SumEntryEffectsByAccount(_ context.Context, orgID, accountID uuid.UUID) (domain.LedgerEffectsSummary, error) {
	ledger := money.Zero()
	enc := money.Zero()
	for _, e := range s.entries {
		if e.OrganizationID != orgID || e.AccountID != accountID {
			continue
		}
		switch e.EntryType {
		case domain.EntryCredit, domain.EntryDebit, domain.EntryReversal:
			ledger = ledger.Add(e.DeltaAmount)
		case domain.EntryHoldPlaced, domain.EntryHoldReleased:
			enc = enc.Add(e.DeltaAmount)
		case domain.EntryHoldSettled:
			ledger = ledger.Add(e.DeltaAmount)
			enc = enc.Add(e.DeltaAmount)
		}
	}
	return domain.LedgerEffectsSummary{ComputedLedgerBalance: ledger, ComputedEncumberedBalance: enc}, nil
}

func (s *apiMemStore) CreatePolicy(_ context.Context, policy domain.LiquidityPolicy) error {
	if policy.IsActive {
		for id, p := range s.policies {
			if p.OrganizationID == policy.OrganizationID && p.TreasuryEntityID == policy.TreasuryEntityID {
				p.IsActive = false
				s.policies[id] = p
			}
		}
	}
	s.policies[policy.ID] = policy
	return nil
}

func (s *apiMemStore) GetActivePolicy(_ context.Context, orgID, entityID uuid.UUID) (domain.LiquidityPolicy, error) {
	for _, p := range s.policies {
		if p.OrganizationID == orgID && p.TreasuryEntityID == entityID && p.IsActive {
			return p, nil
		}
	}
	return domain.LiquidityPolicy{}, domain.ErrNotFound
}

func (s *apiMemStore) GetPolicyByID(_ context.Context, orgID, policyID uuid.UUID) (domain.LiquidityPolicy, error) {
	p, ok := s.policies[policyID]
	if !ok || p.OrganizationID != orgID {
		return domain.LiquidityPolicy{}, domain.ErrNotFound
	}
	return p, nil
}

func (s *apiMemStore) ListPoliciesByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]domain.LiquidityPolicy, error) {
	var out []domain.LiquidityPolicy
	for _, p := range s.policies {
		if p.OrganizationID == orgID && p.TreasuryEntityID == entityID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *apiMemStore) UpdatePolicy(_ context.Context, policy domain.LiquidityPolicy) (domain.LiquidityPolicy, error) {
	s.policies[policy.ID] = policy
	return policy, nil
}

func (s *apiMemStore) CreateCounterparty(_ context.Context, cp domain.Counterparty) error {
	s.counterparties[cp.ID] = cp
	return nil
}

func (s *apiMemStore) GetCounterparty(_ context.Context, orgID, counterpartyID uuid.UUID) (domain.Counterparty, error) {
	cp, ok := s.counterparties[counterpartyID]
	if !ok || cp.OrganizationID != orgID {
		return domain.Counterparty{}, domain.ErrNotFound
	}
	return cp, nil
}

func (s *apiMemStore) ListCounterpartiesByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]domain.Counterparty, error) {
	var out []domain.Counterparty
	for _, cp := range s.counterparties {
		if cp.OrganizationID == orgID && cp.TreasuryEntityID == entityID {
			out = append(out, cp)
		}
	}
	return out, nil
}

func (s *apiMemStore) UpdateCounterparty(_ context.Context, cp domain.Counterparty, expectedVersion int64) (domain.Counterparty, error) {
	existing, ok := s.counterparties[cp.ID]
	if !ok || existing.OrganizationID != cp.OrganizationID {
		return domain.Counterparty{}, domain.ErrNotFound
	}
	if existing.Version != expectedVersion {
		return domain.Counterparty{}, domain.ErrVersionConflict
	}
	cp.Version++
	s.counterparties[cp.ID] = cp
	return cp, nil
}

func (s *apiMemStore) AppendAuditEvent(_ context.Context, event domain.AuditEvent) error {
	event.CreatedAt = time.Now().UTC()
	s.audits = append(s.audits, event)
	return nil
}

func (s *apiMemStore) ListAuditEvents(_ context.Context, orgID uuid.UUID, _ int) ([]domain.AuditEvent, error) {
	var out []domain.AuditEvent
	for _, ev := range s.audits {
		if ev.OrganizationID != nil && *ev.OrganizationID == orgID {
			out = append(out, ev)
		}
	}
	return out, nil
}

type apiTestEnv struct {
	store      *apiMemStore
	router     http.Handler
	orgAID     uuid.UUID
	orgBID     uuid.UUID
	entityAID  uuid.UUID
	accountID  uuid.UUID
	analystID  uuid.UUID
	approverID uuid.UUID
}

func setupAPITestEnv(t *testing.T) *apiTestEnv {
	t.Helper()
	store := newAPIMemStore()
	tokenSvc, err := auth.NewTokenService("test-jwt-secret-32-characters-minimum!", "quanta-backend", "quanta-api", time.Hour)
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}

	analystHash, _ := auth.HashPassword("AnalystPass!2026")
	approverHash, _ := auth.HashPassword("ApproverPass!2026")

	orgA := uuid.New()
	orgB := uuid.New()
	analystID := uuid.New()
	approverID := uuid.New()
	entityA := uuid.New()
	accountA := uuid.New()

	store.orgs[orgA] = domain.Organization{ID: orgA, Name: "Org Alpha", Status: domain.OrgStatusActive}
	store.orgs[orgB] = domain.Organization{ID: orgB, Name: "Org Beta", Status: domain.OrgStatusActive}
	store.users[analystID] = domain.User{ID: analystID, Email: "analyst@quanta.local", PasswordHash: analystHash, FullName: "Alex Analyst", IsActive: true}
	store.users[approverID] = domain.User{ID: approverID, Email: "approver@quanta.local", PasswordHash: approverHash, FullName: "Morgan Approver", IsActive: true}
	store.memberships = []domain.OrganizationMembership{
		{ID: uuid.New(), OrganizationID: orgA, UserID: analystID, Role: domain.RoleTreasuryAnalyst, Status: domain.OrgStatusActive},
		{ID: uuid.New(), OrganizationID: orgA, UserID: approverID, Role: domain.RoleTreasuryApprover, Status: domain.OrgStatusActive},
		{ID: uuid.New(), OrganizationID: orgB, UserID: approverID, Role: domain.RoleViewer, Status: domain.OrgStatusActive},
	}
	store.entities[entityA] = domain.TreasuryEntity{ID: entityA, OrganizationID: orgA, Name: "Alpha USD Treasury", BaseCurrency: "USD", IsSynthetic: true}
	store.accounts[accountA] = domain.TreasuryAccount{
		ID: accountA, OrganizationID: orgA, TreasuryEntityID: entityA,
		AccountCode: "OP-USD-01", Name: "Operating USD", CurrencyCode: "USD",
		LedgerBalance: money.Zero(), EncumberedBalance: money.Zero(), Version: 1,
	}

	authSvc := service.NewAuthService(store, store, tokenSvc)
	accountSvc := service.NewAccountService(store, store, store, store)
	ledgerSvc := service.NewLedgerService(store)
	policySvc := service.NewPolicyService(store, store, store)
	cpSvc := service.NewCounterpartyService(store, store, store)

	r := router.NewRouter(&config.Config{}, router.Dependencies{
		Health:       handlers.NewHealthHandler(stubPinger{}, stubIntelligence{}),
		Auth:         handlers.NewAuthHandler(authSvc),
		Treasury:     handlers.NewTreasuryHandler(accountSvc, ledgerSvc),
		Policy:       handlers.NewPolicyHandler(policySvc),
		Counterparty: handlers.NewCounterpartyHandler(cpSvc),
		Audit:        handlers.NewAuditHandler(store),
		Tokens:       tokenSvc,
		OrgRepo:      store,
	})

	return &apiTestEnv{
		store:      store,
		router:     r,
		orgAID:     orgA,
		orgBID:     orgB,
		entityAID:  entityA,
		accountID:  accountA,
		analystID:  analystID,
		approverID: approverID,
	}
}

func doJSONReq(t *testing.T, r http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func loginForToken(t *testing.T, r http.Handler, email, password string) string {
	t.Helper()
	rec := doJSONReq(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email":    email,
		"password": password,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed (%d): %s", rec.Code, rec.Body.String())
	}
	var res map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	tok, _ := res["access_token"].(string)
	if tok == "" {
		t.Fatal("missing access_token in login response")
	}
	return tok
}

func TestAPI_AuthTenantIsolationAndRoleSeparation(t *testing.T) {
	env := setupAPITestEnv(t)

	// Unauthenticated access denied
	if rec := doJSONReq(t, env.router, http.MethodGet, "/api/v1/auth/me", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	analystToken := loginForToken(t, env.router, "analyst@quanta.local", "AnalystPass!2026")
	approverToken := loginForToken(t, env.router, "approver@quanta.local", "ApproverPass!2026")

	// Verify audit logs never contain plaintext passwords or JWTs
	for _, ev := range env.store.audits {
		raw := string(ev.Metadata)
		if strings.Contains(raw, "AnalystPass!2026") || strings.Contains(raw, analystToken) {
			t.Fatal("audit metadata leaked secret password or JWT")
		}
	}

	// Approver belongs to OrgA & OrgB (multi-org user)
	meRec := doJSONReq(t, env.router, http.MethodGet, "/api/v1/auth/me", approverToken, nil)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /auth/me, got %d", meRec.Code)
	}

	// Analyst has NO membership in OrgB -> cross-org access denied (403)
	crossRec := doJSONReq(t, env.router, http.MethodGet, "/api/v1/orgs/"+env.orgBID.String()+"/treasuries", analystToken, nil)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on cross-org access, got %d", crossRec.Code)
	}

	// Approver CANNOT mutate ledger in OrgA (role separation) -> 403
	approverCredit := doJSONReq(t, env.router, http.MethodPost,
		"/api/v1/orgs/"+env.orgAID.String()+"/accounts/"+env.accountID.String()+"/credit",
		approverToken,
		map[string]string{"amount": "100.0000"},
	)
	if approverCredit.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when approver attempts ledger credit, got %d", approverCredit.Code)
	}

	// Analyst CAN credit account -> 201
	analystCredit := doJSONReq(t, env.router, http.MethodPost,
		"/api/v1/orgs/"+env.orgAID.String()+"/accounts/"+env.accountID.String()+"/credit",
		analystToken,
		map[string]string{"amount": "1000.0000", "reference_type": "DEMO_SEED"},
	)
	if analystCredit.Code != http.StatusCreated {
		t.Fatalf("expected 201 on analyst credit, got %d: %s", analystCredit.Code, analystCredit.Body.String())
	}
}

func TestAPI_LiquidityPolicyAndCounterpartyEnforcement(t *testing.T) {
	env := setupAPITestEnv(t)
	analystToken := loginForToken(t, env.router, "analyst@quanta.local", "AnalystPass!2026")
	baseURL := "/api/v1/orgs/" + env.orgAID.String() + "/treasuries/" + env.entityAID.String()

	// Create liquidity policy: operating 200.0000 + emergency 100.0000 = 300.0000 floor
	polRec := doJSONReq(t, env.router, http.MethodPost, baseURL+"/liquidity-policies", analystToken, map[string]any{
		"min_operating_liquidity": "200.0000",
		"min_emergency_reserve":   "100.0000",
		"is_active":               true,
	})
	if polRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating policy, got %d: %s", polRec.Code, polRec.Body.String())
	}

	// Validate allocation breaching floor: 1000 available, proposing 750 leaves 250 < 300 -> 422
	breachRec := doJSONReq(t, env.router, http.MethodPost, baseURL+"/liquidity-policies/validate", analystToken, map[string]string{
		"total_available":     "1000.0000",
		"proposed_allocation": "750.0000",
	})
	if breachRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on liquidity floor breach, got %d: %s", breachRec.Code, breachRec.Body.String())
	}

	// Create counterparty in EUR on USD entity -> 400 CURRENCY_MISMATCH
	badCurrRec := doJSONReq(t, env.router, http.MethodPost, baseURL+"/counterparties", analystToken, map[string]string{
		"name":               "Euro Bank",
		"currency_code":      "EUR",
		"max_exposure_limit": "500.0000",
	})
	if badCurrRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on currency mismatch, got %d: %s", badCurrRec.Code, badCurrRec.Body.String())
	}

	// Create counterparty in USD -> 201
	cpRec := doJSONReq(t, env.router, http.MethodPost, baseURL+"/counterparties", analystToken, map[string]string{
		"name":               "Synthetic Treasury MMF",
		"currency_code":      "USD",
		"max_exposure_limit": "500.0000",
		"current_exposure":   "300.0000",
	})
	if cpRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating counterparty, got %d: %s", cpRec.Code, cpRec.Body.String())
	}
	var cpResp struct {
		Counterparty struct {
			ID string `json:"id"`
		} `json:"counterparty"`
	}
	_ = json.Unmarshal(cpRec.Body.Bytes(), &cpResp)

	// Validate exposure exceeding cap (300 + 250 = 550 > 500) -> 422
	capRec := doJSONReq(t, env.router, http.MethodPost,
		baseURL+"/counterparties/"+cpResp.Counterparty.ID+"/validate-exposure",
		analystToken,
		map[string]string{"currency_code": "USD", "proposed_increment": "250.0000"},
	)
	if capRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on counterparty limit exceeded, got %d: %s", capRec.Code, capRec.Body.String())
	}
}
