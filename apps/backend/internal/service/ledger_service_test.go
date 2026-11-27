package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type memStore struct {
	entities     map[uuid.UUID]domain.TreasuryEntity
	accounts     map[uuid.UUID]domain.TreasuryAccount
	entries      []domain.LedgerEntry
	audits       []domain.AuditEvent
	failOnAppend bool
	failOnAudit  bool
}

func newMemStore() *memStore {
	return &memStore{
		entities: make(map[uuid.UUID]domain.TreasuryEntity),
		accounts: make(map[uuid.UUID]domain.TreasuryAccount),
	}
}

func (s *memStore) clone() *memStore {
	c := &memStore{
		entities:     make(map[uuid.UUID]domain.TreasuryEntity, len(s.entities)),
		accounts:     make(map[uuid.UUID]domain.TreasuryAccount, len(s.accounts)),
		entries:      append([]domain.LedgerEntry(nil), s.entries...),
		audits:       append([]domain.AuditEvent(nil), s.audits...),
		failOnAppend: s.failOnAppend,
		failOnAudit:  s.failOnAudit,
	}
	for k, v := range s.entities {
		c.entities[k] = v
	}
	for k, v := range s.accounts {
		c.accounts[k] = v
	}
	return c
}

type memTxManager struct {
	store *memStore
}

func (m *memTxManager) WithTx(ctx context.Context, fn func(ctx context.Context, repos repository.TxRepositories) error) error {
	snapshot := m.store.clone()
	repos := repository.TxRepositories{
		Accounts: snapshot,
		Ledger:   snapshot,
		Audit:    snapshot,
	}
	if err := fn(ctx, repos); err != nil {
		// Rollback by discarding snapshot.
		return err
	}
	m.store.entities = snapshot.entities
	m.store.accounts = snapshot.accounts
	m.store.entries = snapshot.entries
	m.store.audits = snapshot.audits
	return nil
}

func (s *memStore) CreateTreasuryEntity(_ context.Context, entity domain.TreasuryEntity) error {
	s.entities[entity.ID] = entity
	return nil
}

func (s *memStore) GetTreasuryEntity(_ context.Context, orgID, entityID uuid.UUID) (domain.TreasuryEntity, error) {
	e, ok := s.entities[entityID]
	if !ok || e.OrganizationID != orgID {
		return domain.TreasuryEntity{}, domain.ErrNotFound
	}
	return e, nil
}

func (s *memStore) ListTreasuryEntities(_ context.Context, orgID uuid.UUID) ([]domain.TreasuryEntity, error) {
	var out []domain.TreasuryEntity
	for _, e := range s.entities {
		if e.OrganizationID == orgID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *memStore) CreateAccount(_ context.Context, account domain.TreasuryAccount) error {
	s.accounts[account.ID] = account
	return nil
}

func (s *memStore) GetAccount(_ context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	acc, ok := s.accounts[accountID]
	if !ok || acc.OrganizationID != orgID {
		return domain.TreasuryAccount{}, domain.ErrNotFound
	}
	return acc, nil
}

func (s *memStore) GetAccountForUpdate(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	return s.GetAccount(ctx, orgID, accountID)
}

func (s *memStore) ListAccountsByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]domain.TreasuryAccount, error) {
	var out []domain.TreasuryAccount
	for _, acc := range s.accounts {
		if acc.OrganizationID == orgID && acc.TreasuryEntityID == entityID {
			out = append(out, acc)
		}
	}
	return out, nil
}

func (s *memStore) UpdateAccountProjection(
	_ context.Context,
	orgID, accountID uuid.UUID,
	expectedVersion int64,
	newLedgerBalance, newEncumberedBalance money.Amount,
) (domain.TreasuryAccount, error) {
	acc, ok := s.accounts[accountID]
	if !ok || acc.OrganizationID != orgID {
		return domain.TreasuryAccount{}, domain.ErrNotFound
	}
	if acc.Version != expectedVersion {
		return domain.TreasuryAccount{}, domain.ErrVersionConflict
	}
	acc.LedgerBalance = newLedgerBalance
	acc.EncumberedBalance = newEncumberedBalance
	acc.Version++
	s.accounts[accountID] = acc
	return acc, nil
}

func (s *memStore) AppendEntry(_ context.Context, entry domain.LedgerEntry) error {
	if s.failOnAppend {
		return errors.New("simulated ledger insert failure")
	}
	s.entries = append(s.entries, entry)
	return nil
}

func (s *memStore) GetEntry(_ context.Context, orgID, entryID uuid.UUID) (domain.LedgerEntry, error) {
	for _, e := range s.entries {
		if e.OrganizationID == orgID && e.ID == entryID {
			return e, nil
		}
	}
	return domain.LedgerEntry{}, domain.ErrNotFound
}

func (s *memStore) HasReversalForEntry(_ context.Context, orgID, entryID uuid.UUID) (bool, error) {
	for _, e := range s.entries {
		if e.OrganizationID == orgID && e.ReversedEntryID != nil && *e.ReversedEntryID == entryID {
			return true, nil
		}
	}
	return false, nil
}

func (s *memStore) ListEntriesByAccount(_ context.Context, orgID, accountID uuid.UUID) ([]domain.LedgerEntry, error) {
	var out []domain.LedgerEntry
	for _, e := range s.entries {
		if e.OrganizationID == orgID && e.AccountID == accountID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *memStore) SumEntryEffectsByAccount(_ context.Context, orgID, accountID uuid.UUID) (domain.LedgerEffectsSummary, error) {
	ledger := money.Zero()
	encumbered := money.Zero()
	for _, e := range s.entries {
		if e.OrganizationID != orgID || e.AccountID != accountID {
			continue
		}
		switch e.EntryType {
		case domain.EntryCredit, domain.EntryDebit, domain.EntryReversal:
			ledger = ledger.Add(e.DeltaAmount)
		case domain.EntryHoldPlaced, domain.EntryHoldReleased:
			encumbered = encumbered.Add(e.DeltaAmount)
		case domain.EntryHoldSettled:
			ledger = ledger.Add(e.DeltaAmount)
			encumbered = encumbered.Add(e.DeltaAmount)
		}
	}
	return domain.LedgerEffectsSummary{
		ComputedLedgerBalance:     ledger,
		ComputedEncumberedBalance: encumbered,
	}, nil
}

func (s *memStore) AppendAuditEvent(_ context.Context, event domain.AuditEvent) error {
	if s.failOnAudit {
		return errors.New("simulated audit insert failure")
	}
	s.audits = append(s.audits, event)
	return nil
}

func (s *memStore) ListAuditEvents(_ context.Context, orgID uuid.UUID, _ int) ([]domain.AuditEvent, error) {
	var out []domain.AuditEvent
	for _, ev := range s.audits {
		if ev.OrganizationID != nil && *ev.OrganizationID == orgID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func setupAccountFixture(t *testing.T) (*memStore, *service.AccountService, *service.LedgerService, uuid.UUID, domain.TreasuryAccount) {
	t.Helper()
	store := newMemStore()
	orgID := uuid.New()
	entityID := uuid.New()
	store.entities[entityID] = domain.TreasuryEntity{
		ID:             entityID,
		OrganizationID: orgID,
		Name:           "Primary USD Treasury",
		BaseCurrency:   "USD",
		IsSynthetic:    true,
	}

	acctSvc := service.NewAccountService(store, store, store, store)
	ledgerSvc := service.NewLedgerService(&memTxManager{store: store})

	acc, err := acctSvc.CreateAccount(context.Background(), service.CreateAccountInput{
		OrganizationID:   orgID,
		TreasuryEntityID: entityID,
		AccountCode:      "OP-USD-01",
		Name:             "Operating Cash",
		CurrencyCode:     "USD",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	return store, acctSvc, ledgerSvc, orgID, acc
}

func TestAccountService_EnforcesSingleCurrencyAndTenantIsolation(t *testing.T) {
	store, acctSvc, _, orgID, acc := setupAccountFixture(t)
	otherOrgID := uuid.New()

	_, err := acctSvc.CreateAccount(context.Background(), service.CreateAccountInput{
		OrganizationID:   orgID,
		TreasuryEntityID: acc.TreasuryEntityID,
		AccountCode:      "EUR-01",
		Name:             "Euro Account",
		CurrencyCode:     "EUR",
	})
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}

	if _, err := acctSvc.GetAccount(context.Background(), otherOrgID, acc.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected cross-tenant GetAccount to return ErrNotFound, got %v", err)
	}
	if len(store.accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(store.accounts))
	}
}

func TestLedgerService_HoldLifecycleAndReconciliation(t *testing.T) {
	ctx := context.Background()
	store, acctSvc, ledgerSvc, orgID, acc := setupAccountFixture(t)
	actorID := uuid.New()

	// 1. Credit 1000.0000
	creditEntry, acc, err := ledgerSvc.Credit(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorID:        &actorID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("1000.0000"),
		ReferenceType:  "SEED",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-1",
	})
	if err != nil {
		t.Fatalf("Credit failed: %v", err)
	}
	if !acc.LedgerBalance.Equal(money.MustFromString("1000.0000")) || acc.Version != 2 {
		t.Fatalf("unexpected account after credit: %+v", acc)
	}

	// 2. Place hold of 400.0000
	_, acc, err = ledgerSvc.PlaceHold(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorID:        &actorID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("400.0000"),
		ReferenceType:  "PROPOSAL",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-2",
	})
	if err != nil {
		t.Fatalf("PlaceHold failed: %v", err)
	}
	if !acc.AvailableBalance().Equal(money.MustFromString("600.0000")) {
		t.Fatalf("expected available balance 600.0000, got %s", acc.AvailableBalance())
	}

	// 3. Reject debit exceeding available balance (600.0000)
	_, _, err = ledgerSvc.Debit(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("650.0000"),
		ReferenceType:  "PAYMENT",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-3",
	})
	if !errors.Is(err, domain.ErrInsufficientAvailableBalance) {
		t.Fatalf("expected ErrInsufficientAvailableBalance, got %v", err)
	}

	// 4. Release 150.0000 of hold, then settle remaining 250.0000
	_, acc, err = ledgerSvc.ReleaseHold(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorRole:      string(domain.RoleTreasuryApprover),
		Amount:         money.MustFromString("150.0000"),
		ReferenceType:  "PROPOSAL_RELEASE",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-4",
	})
	if err != nil {
		t.Fatalf("ReleaseHold failed: %v", err)
	}
	_, acc, err = ledgerSvc.SettleHold(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorRole:      string(domain.RoleTreasuryApprover),
		Amount:         money.MustFromString("250.0000"),
		ReferenceType:  "PROPOSAL_SETTLE",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-5",
	})
	if err != nil {
		t.Fatalf("SettleHold failed: %v", err)
	}
	if !acc.LedgerBalance.Equal(money.MustFromString("750.0000")) || !acc.EncumberedBalance.IsZero() {
		t.Fatalf("expected ledger 750.0000 and encumbered 0.0000, got ledger=%s enc=%s", acc.LedgerBalance, acc.EncumberedBalance)
	}

	// 5. Reconcile matches ledger sum
	report, err := acctSvc.ReconcileAccount(ctx, orgID, acc.ID, &actorID, string(domain.RoleAdmin), "corr-6")
	if err != nil || !report.Matched {
		t.Fatalf("expected clean reconciliation, got err=%v report=%+v", err, report)
	}

	// 6. Reverse original credit (1000.0000) should fail because ledger only has 750.0000 after settlement
	_, _, err = ledgerSvc.ReverseEntry(ctx, service.ReverseEntryInput{
		OrganizationID: orgID,
		EntryID:        creditEntry.ID,
		ActorRole:      string(domain.RoleAdmin),
		Reason:         "test insufficient balance reversal",
		CorrelationID:  "corr-7",
	})
	if !errors.Is(err, domain.ErrInsufficientAvailableBalance) {
		t.Fatalf("expected ErrInsufficientAvailableBalance on full credit reversal, got %v", err)
	}

	if len(store.entries) != 4 {
		t.Fatalf("expected 4 append-only ledger entries, got %d", len(store.entries))
	}
}

func TestLedgerService_ReversalAndAtomicRollback(t *testing.T) {
	ctx := context.Background()
	store, acctSvc, ledgerSvc, orgID, acc := setupAccountFixture(t)
	actorID := uuid.New()

	creditEntry, acc, err := ledgerSvc.Credit(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorID:        &actorID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("500.0000"),
		ReferenceType:  "SEED",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-rev-1",
	})
	if err != nil {
		t.Fatalf("Credit failed: %v", err)
	}

	// Reverse once succeeds
	revEntry, acc, err := ledgerSvc.ReverseEntry(ctx, service.ReverseEntryInput{
		OrganizationID: orgID,
		EntryID:        creditEntry.ID,
		ActorID:        &actorID,
		ActorRole:      string(domain.RoleAdmin),
		Reason:         "duplicate deposit",
		CorrelationID:  "corr-rev-2",
	})
	if err != nil {
		t.Fatalf("ReverseEntry failed: %v", err)
	}
	if !acc.LedgerBalance.IsZero() || revEntry.ReversedEntryID == nil || *revEntry.ReversedEntryID != creditEntry.ID {
		t.Fatalf("unexpected reversal outcome: acc=%+v entry=%+v", acc, revEntry)
	}

	// Double reversal is rejected
	_, _, err = ledgerSvc.ReverseEntry(ctx, service.ReverseEntryInput{
		OrganizationID: orgID,
		EntryID:        creditEntry.ID,
		ActorRole:      string(domain.RoleAdmin),
		Reason:         "second attempt",
		CorrelationID:  "corr-rev-3",
	})
	if !errors.Is(err, domain.ErrAlreadyReversed) {
		t.Fatalf("expected ErrAlreadyReversed, got %v", err)
	}

	// Simulate ledger append failure -> account projection update MUST roll back atomically
	store.failOnAppend = true
	_, _, err = ledgerSvc.Credit(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("250.0000"),
		ReferenceType:  "SEED",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-rollback",
	})
	if err == nil {
		t.Fatal("expected error when AppendEntry fails")
	}
	persisted, _ := acctSvc.GetAccount(ctx, orgID, acc.ID)
	if !persisted.LedgerBalance.IsZero() || persisted.Version != acc.Version {
		t.Fatalf("expected atomic rollback of account projection, got %+v", persisted)
	}
}

func TestAccountService_DetectsReconciliationMismatch(t *testing.T) {
	ctx := context.Background()
	store, acctSvc, ledgerSvc, orgID, acc := setupAccountFixture(t)
	actorID := uuid.New()

	_, acc, err := ledgerSvc.Credit(ctx, service.LedgerOperationInput{
		OrganizationID: orgID,
		AccountID:      acc.ID,
		ActorRole:      string(domain.RoleTreasuryAnalyst),
		Amount:         money.MustFromString("100.0000"),
		ReferenceType:  "SEED",
		ReferenceID:    uuid.New(),
		CorrelationID:  "corr-rec-1",
	})
	if err != nil {
		t.Fatalf("Credit failed: %v", err)
	}

	// Corrupt materialized balance directly in store to verify reconciliation catches it
	corrupted := store.accounts[acc.ID]
	corrupted.LedgerBalance = money.MustFromString("999.0000")
	store.accounts[acc.ID] = corrupted

	auditCountBefore := len(store.audits)
	report, err := acctSvc.ReconcileAccount(ctx, orgID, acc.ID, &actorID, string(domain.RoleAdmin), "corr-rec-2")
	if !errors.Is(err, domain.ErrReconciliationMismatch) {
		t.Fatalf("expected ErrReconciliationMismatch, got %v", err)
	}
	if report.Matched {
		t.Fatal("expected report.Matched to be false")
	}
	if len(store.audits) != auditCountBefore+1 {
		t.Fatal("expected critical RECONCILIATION_MISMATCH audit event to be appended")
	}
}
