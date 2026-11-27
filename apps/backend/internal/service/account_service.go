package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// CreateAccountInput defines parameters for creating a single-currency treasury account.
type CreateAccountInput struct {
	OrganizationID   uuid.UUID
	TreasuryEntityID uuid.UUID
	AccountCode      string
	Name             string
	CurrencyCode     string
}

// AccountService manages treasury accounts and deterministic ledger reconciliation.
type AccountService struct {
	treasuryRepo repository.TreasuryRepository
	accountRepo  repository.AccountRepository
	ledgerRepo   repository.LedgerRepository
	auditRepo    repository.AuditRepository
}

// NewAccountService constructs an AccountService.
func NewAccountService(
	treasuryRepo repository.TreasuryRepository,
	accountRepo repository.AccountRepository,
	ledgerRepo repository.LedgerRepository,
	auditRepo repository.AuditRepository,
) *AccountService {
	return &AccountService{
		treasuryRepo: treasuryRepo,
		accountRepo:  accountRepo,
		ledgerRepo:   ledgerRepo,
		auditRepo:    auditRepo,
	}
}

// CreateAccount creates an organization-scoped account enforcing single-currency-per-treasury.
func (s *AccountService) CreateAccount(ctx context.Context, in CreateAccountInput) (domain.TreasuryAccount, error) {
	entity, err := s.treasuryRepo.GetTreasuryEntity(ctx, in.OrganizationID, in.TreasuryEntityID)
	if err != nil {
		return domain.TreasuryAccount{}, err
	}

	normalizedCurrency := strings.ToUpper(strings.TrimSpace(in.CurrencyCode))
	if normalizedCurrency == "" || normalizedCurrency != strings.ToUpper(strings.TrimSpace(entity.BaseCurrency)) {
		return domain.TreasuryAccount{}, domain.ErrCurrencyMismatch
	}

	account := domain.TreasuryAccount{
		ID:                uuid.New(),
		OrganizationID:    in.OrganizationID,
		TreasuryEntityID:  in.TreasuryEntityID,
		AccountCode:       strings.TrimSpace(in.AccountCode),
		Name:              strings.TrimSpace(in.Name),
		CurrencyCode:      entity.BaseCurrency,
		LedgerBalance:     money.Zero(),
		EncumberedBalance: money.Zero(),
		Version:           1,
	}
	if err := s.accountRepo.CreateAccount(ctx, account); err != nil {
		return domain.TreasuryAccount{}, err
	}
	return account, nil
}

// GetAccount fetches an organization-scoped treasury account.
func (s *AccountService) GetAccount(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	return s.accountRepo.GetAccount(ctx, orgID, accountID)
}

// ListAccountsByEntity lists all accounts for an organization-scoped treasury entity.
func (s *AccountService) ListAccountsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.TreasuryAccount, error) {
	return s.accountRepo.ListAccountsByEntity(ctx, orgID, entityID)
}

// ListTreasuryEntities lists all treasury entities belonging to orgID.
func (s *AccountService) ListTreasuryEntities(ctx context.Context, orgID uuid.UUID) ([]domain.TreasuryEntity, error) {
	return s.treasuryRepo.ListTreasuryEntities(ctx, orgID)
}

// ListEntriesByAccount returns the append-only ledger history for an organization-scoped account.
func (s *AccountService) ListEntriesByAccount(ctx context.Context, orgID, accountID uuid.UUID) ([]domain.LedgerEntry, error) {
	if _, err := s.accountRepo.GetAccount(ctx, orgID, accountID); err != nil {
		return nil, err
	}
	return s.ledgerRepo.ListEntriesByAccount(ctx, orgID, accountID)
}

// ReconcileAccount compares materialized account balances against summed ledger_entries.
// If a discrepancy is detected, a critical AuditEvent is recorded and ErrReconciliationMismatch is returned.
func (s *AccountService) ReconcileAccount(
	ctx context.Context,
	orgID, accountID uuid.UUID,
	actorID *uuid.UUID,
	actorRole, correlationID string,
) (domain.ReconciliationReport, error) {
	account, err := s.accountRepo.GetAccount(ctx, orgID, accountID)
	if err != nil {
		return domain.ReconciliationReport{}, err
	}

	summary, err := s.ledgerRepo.SumEntryEffectsByAccount(ctx, orgID, accountID)
	if err != nil {
		return domain.ReconciliationReport{}, err
	}

	matched := account.LedgerBalance.Equal(summary.ComputedLedgerBalance) &&
		account.EncumberedBalance.Equal(summary.ComputedEncumberedBalance)

	report := domain.ReconciliationReport{
		OrganizationID:            orgID,
		AccountID:                 accountID,
		StoredLedgerBalance:       account.LedgerBalance,
		ComputedLedgerBalance:     summary.ComputedLedgerBalance,
		StoredEncumberedBalance:   account.EncumberedBalance,
		ComputedEncumberedBalance: summary.ComputedEncumberedBalance,
		AccountVersion:            account.Version,
		Matched:                   matched,
	}

	if !matched {
		meta, _ := json.Marshal(map[string]any{
			"stored_ledger":       account.LedgerBalance.String(),
			"computed_ledger":     summary.ComputedLedgerBalance.String(),
			"stored_encumbered":   account.EncumberedBalance.String(),
			"computed_encumbered": summary.ComputedEncumberedBalance.String(),
			"account_version":     account.Version,
		})
		if s.auditRepo != nil {
			_ = s.auditRepo.AppendAuditEvent(ctx, domain.AuditEvent{
				ID:             uuid.New(),
				OrganizationID: &orgID,
				ActorID:        actorID,
				ActorRole:      actorRole,
				Action:         "RECONCILIATION_MISMATCH",
				EntityType:     "treasury_account",
				EntityID:       &accountID,
				CorrelationID:  correlationID,
				Metadata:       meta,
			})
		}
		return report, domain.ErrReconciliationMismatch
	}

	return report, nil
}
