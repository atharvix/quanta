package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// LedgerOperationInput defines parameters for crediting, debiting, or managing holds on an account.
type LedgerOperationInput struct {
	OrganizationID uuid.UUID
	AccountID      uuid.UUID
	ActorID        *uuid.UUID
	ActorRole      string
	Amount         money.Amount
	ReferenceType  string
	ReferenceID    uuid.UUID
	CorrelationID  string
}

// ReverseEntryInput defines parameters for appending an offsetting REVERSAL ledger entry.
type ReverseEntryInput struct {
	OrganizationID uuid.UUID
	EntryID        uuid.UUID
	ActorID        *uuid.UUID
	ActorRole      string
	Reason         string
	CorrelationID  string
}

// LedgerService enforces append-only ledger semantics and atomic account projection updates.
type LedgerService struct {
	txManager repository.TxManager
}

// NewLedgerService constructs a LedgerService backed by txManager.
func NewLedgerService(txManager repository.TxManager) *LedgerService {
	return &LedgerService{txManager: txManager}
}

// Credit appends a positive CREDIT entry and increases ledger_balance atomically.
func (s *LedgerService) Credit(ctx context.Context, in LedgerOperationInput) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	return s.applyMutation(ctx, domain.EntryCredit, in)
}

// Debit appends a negative DEBIT entry and decreases ledger_balance after verifying available funds.
func (s *LedgerService) Debit(ctx context.Context, in LedgerOperationInput) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	return s.applyMutation(ctx, domain.EntryDebit, in)
}

// PlaceHold appends HOLD_PLACED and increases encumbered_balance without reducing ledger_balance.
func (s *LedgerService) PlaceHold(ctx context.Context, in LedgerOperationInput) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	return s.applyMutation(ctx, domain.EntryHoldPlaced, in)
}

// ReleaseHold appends HOLD_RELEASED and reduces encumbered_balance.
func (s *LedgerService) ReleaseHold(ctx context.Context, in LedgerOperationInput) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	return s.applyMutation(ctx, domain.EntryHoldReleased, in)
}

// SettleHold appends HOLD_SETTLED and reduces both encumbered_balance and ledger_balance.
func (s *LedgerService) SettleHold(ctx context.Context, in LedgerOperationInput) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	return s.applyMutation(ctx, domain.EntryHoldSettled, in)
}

// ReverseEntry appends a REVERSAL entry offsetting an existing CREDIT or DEBIT entry.
func (s *LedgerService) ReverseEntry(ctx context.Context, in ReverseEntryInput) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	var createdEntry domain.LedgerEntry
	var updatedAccount domain.TreasuryAccount

	err := s.txManager.WithTx(ctx, func(ctx context.Context, repos repository.TxRepositories) error {
		original, err := repos.Ledger.GetEntry(ctx, in.OrganizationID, in.EntryID)
		if err != nil {
			return err
		}
		if original.EntryType != domain.EntryCredit && original.EntryType != domain.EntryDebit {
			return domain.ErrCannotReverseEntryType
		}

		alreadyReversed, err := repos.Ledger.HasReversalForEntry(ctx, in.OrganizationID, in.EntryID)
		if err != nil {
			return err
		}
		if alreadyReversed {
			return domain.ErrAlreadyReversed
		}

		account, err := repos.Accounts.GetAccountForUpdate(ctx, in.OrganizationID, original.AccountID)
		if err != nil {
			return err
		}

		delta := original.DeltaAmount.Neg()
		newLedger := account.LedgerBalance.Add(delta)
		if newLedger.IsNegative() || newLedger.Cmp(account.EncumberedBalance) < 0 {
			return domain.ErrInsufficientAvailableBalance
		}

		updatedAccount, err = repos.Accounts.UpdateAccountProjection(
			ctx,
			in.OrganizationID,
			account.ID,
			account.Version,
			newLedger,
			account.EncumberedBalance,
		)
		if err != nil {
			return err
		}

		reversedID := original.ID
		createdEntry = domain.LedgerEntry{
			ID:              uuid.New(),
			OrganizationID:  in.OrganizationID,
			AccountID:       account.ID,
			EntryType:       domain.EntryReversal,
			CurrencyCode:    account.CurrencyCode,
			DeltaAmount:     delta,
			BalanceAfter:    updatedAccount.LedgerBalance,
			EncumberedAfter: updatedAccount.EncumberedBalance,
			AccountVersion:  updatedAccount.Version,
			ReversedEntryID: &reversedID,
			ReferenceType:   "LEDGER_REVERSAL",
			ReferenceID:     original.ID,
			CorrelationID:   in.CorrelationID,
		}
		if err := repos.Ledger.AppendEntry(ctx, createdEntry); err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"reversed_entry_id": original.ID.String(),
			"reason":            in.Reason,
			"delta_amount":      delta.String(),
		})
		return repos.Audit.AppendAuditEvent(ctx, domain.AuditEvent{
			ID:             uuid.New(),
			OrganizationID: &in.OrganizationID,
			ActorID:        in.ActorID,
			ActorRole:      in.ActorRole,
			Action:         "LEDGER_ENTRY_REVERSED",
			EntityType:     "ledger_entry",
			EntityID:       &createdEntry.ID,
			CorrelationID:  in.CorrelationID,
			Metadata:       meta,
		})
	})

	return createdEntry, updatedAccount, err
}

func (s *LedgerService) applyMutation(
	ctx context.Context,
	entryType domain.LedgerEntryType,
	in LedgerOperationInput,
) (domain.LedgerEntry, domain.TreasuryAccount, error) {
	if !in.Amount.IsPositive() {
		return domain.LedgerEntry{}, domain.TreasuryAccount{}, domain.ErrInvalidAmount
	}

	var createdEntry domain.LedgerEntry
	var updatedAccount domain.TreasuryAccount

	err := s.txManager.WithTx(ctx, func(ctx context.Context, repos repository.TxRepositories) error {
		account, err := repos.Accounts.GetAccountForUpdate(ctx, in.OrganizationID, in.AccountID)
		if err != nil {
			return err
		}

		newLedger, newEncumbered, delta, err := computeProjectionDelta(account, entryType, in.Amount)
		if err != nil {
			return err
		}

		updatedAccount, err = repos.Accounts.UpdateAccountProjection(
			ctx,
			in.OrganizationID,
			account.ID,
			account.Version,
			newLedger,
			newEncumbered,
		)
		if err != nil {
			return err
		}

		createdEntry = domain.LedgerEntry{
			ID:              uuid.New(),
			OrganizationID:  in.OrganizationID,
			AccountID:       account.ID,
			EntryType:       entryType,
			CurrencyCode:    account.CurrencyCode,
			DeltaAmount:     delta,
			BalanceAfter:    updatedAccount.LedgerBalance,
			EncumberedAfter: updatedAccount.EncumberedBalance,
			AccountVersion:  updatedAccount.Version,
			ReferenceType:   in.ReferenceType,
			ReferenceID:     in.ReferenceID,
			CorrelationID:   in.CorrelationID,
		}
		if err := repos.Ledger.AppendEntry(ctx, createdEntry); err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"entry_type":      string(entryType),
			"delta_amount":    delta.String(),
			"balance_after":   updatedAccount.LedgerBalance.String(),
			"account_version": updatedAccount.Version,
		})
		return repos.Audit.AppendAuditEvent(ctx, domain.AuditEvent{
			ID:             uuid.New(),
			OrganizationID: &in.OrganizationID,
			ActorID:        in.ActorID,
			ActorRole:      in.ActorRole,
			Action:         fmt.Sprintf("LEDGER_%s", entryType),
			EntityType:     "treasury_account",
			EntityID:       &account.ID,
			CorrelationID:  in.CorrelationID,
			Metadata:       meta,
		})
	})

	return createdEntry, updatedAccount, err
}

func computeProjectionDelta(
	account domain.TreasuryAccount,
	entryType domain.LedgerEntryType,
	amount money.Amount,
) (newLedger money.Amount, newEncumbered money.Amount, delta money.Amount, err error) {
	newLedger = account.LedgerBalance
	newEncumbered = account.EncumberedBalance

	switch entryType {
	case domain.EntryCredit:
		delta = amount
		newLedger = account.LedgerBalance.Add(amount)
	case domain.EntryDebit:
		if amount.Cmp(account.AvailableBalance()) > 0 {
			return money.Zero(), money.Zero(), money.Zero(), domain.ErrInsufficientAvailableBalance
		}
		delta = amount.Neg()
		newLedger = account.LedgerBalance.Sub(amount)
	case domain.EntryHoldPlaced:
		if amount.Cmp(account.AvailableBalance()) > 0 {
			return money.Zero(), money.Zero(), money.Zero(), domain.ErrInsufficientAvailableBalance
		}
		delta = amount
		newEncumbered = account.EncumberedBalance.Add(amount)
	case domain.EntryHoldReleased:
		if amount.Cmp(account.EncumberedBalance) > 0 {
			return money.Zero(), money.Zero(), money.Zero(), domain.ErrInsufficientEncumbered
		}
		delta = amount.Neg()
		newEncumbered = account.EncumberedBalance.Sub(amount)
	case domain.EntryHoldSettled:
		if amount.Cmp(account.EncumberedBalance) > 0 {
			return money.Zero(), money.Zero(), money.Zero(), domain.ErrInsufficientEncumbered
		}
		if amount.Cmp(account.LedgerBalance) > 0 {
			return money.Zero(), money.Zero(), money.Zero(), domain.ErrInsufficientAvailableBalance
		}
		delta = amount.Neg()
		newEncumbered = account.EncumberedBalance.Sub(amount)
		newLedger = account.LedgerBalance.Sub(amount)
	default:
		return money.Zero(), money.Zero(), money.Zero(), fmt.Errorf("unsupported ledger entry type %q", entryType)
	}

	return newLedger, newEncumbered, delta, nil
}
