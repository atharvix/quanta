package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
)

type LedgerRepository struct {
	db DBTX
}

func NewLedgerRepository(db DBTX) *LedgerRepository {
	return &LedgerRepository{db: db}
}

func (r *LedgerRepository) AppendEntry(ctx context.Context, entry domain.LedgerEntry) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ledger_entries (
			id, organization_id, account_id, entry_type, currency_code,
			delta_amount, balance_after, encumbered_after, account_version,
			reversed_entry_id, reference_type, reference_id, correlation_id
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6::NUMERIC(20,4), $7::NUMERIC(20,4), $8::NUMERIC(20,4), $9,
			$10, $11, $12, $13
		)
	`,
		entry.ID,
		entry.OrganizationID,
		entry.AccountID,
		string(entry.EntryType),
		entry.CurrencyCode,
		entry.DeltaAmount.String(),
		entry.BalanceAfter.String(),
		entry.EncumberedAfter.String(),
		entry.AccountVersion,
		entry.ReversedEntryID,
		entry.ReferenceType,
		entry.ReferenceID,
		entry.CorrelationID,
	)
	return err
}

func (r *LedgerRepository) GetEntry(ctx context.Context, orgID, entryID uuid.UUID) (domain.LedgerEntry, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, organization_id, account_id, entry_type, currency_code,
		       delta_amount::TEXT, balance_after::TEXT, encumbered_after::TEXT,
		       account_version, reversed_entry_id, reference_type, reference_id,
		       correlation_id, created_at
		FROM ledger_entries
		WHERE organization_id = $1 AND id = $2
	`, orgID, entryID)
	entry, err := scanLedgerEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LedgerEntry{}, domain.ErrNotFound
	}
	return entry, err
}

func (r *LedgerRepository) HasReversalForEntry(ctx context.Context, orgID, entryID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM ledger_entries
			WHERE organization_id = $1 AND reversed_entry_id = $2
		)
	`, orgID, entryID).Scan(&exists)
	return exists, err
}

func (r *LedgerRepository) ListEntriesByAccount(ctx context.Context, orgID, accountID uuid.UUID) ([]domain.LedgerEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, account_id, entry_type, currency_code,
		       delta_amount::TEXT, balance_after::TEXT, encumbered_after::TEXT,
		       account_version, reversed_entry_id, reference_type, reference_id,
		       correlation_id, created_at
		FROM ledger_entries
		WHERE organization_id = $1 AND account_id = $2
		ORDER BY account_version ASC, created_at ASC
	`, orgID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		entry, err := scanLedgerEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *LedgerRepository) SumEntryEffectsByAccount(ctx context.Context, orgID, accountID uuid.UUID) (domain.LedgerEffectsSummary, error) {
	var ledgerRaw, encumberedRaw string
	err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE
				WHEN entry_type IN ('CREDIT', 'DEBIT', 'REVERSAL', 'HOLD_SETTLED') THEN delta_amount
				ELSE 0
			END), 0)::NUMERIC(20,4)::TEXT AS computed_ledger,
			COALESCE(SUM(CASE
				WHEN entry_type IN ('HOLD_PLACED', 'HOLD_RELEASED', 'HOLD_SETTLED') THEN delta_amount
				ELSE 0
			END), 0)::NUMERIC(20,4)::TEXT AS computed_encumbered
		FROM ledger_entries
		WHERE organization_id = $1 AND account_id = $2
	`, orgID, accountID).Scan(&ledgerRaw, &encumberedRaw)
	if err != nil {
		return domain.LedgerEffectsSummary{}, err
	}

	computedLedger, err := money.FromString(ledgerRaw)
	if err != nil {
		return domain.LedgerEffectsSummary{}, fmt.Errorf("parse computed_ledger %q: %w", ledgerRaw, err)
	}
	computedEncumbered, err := money.FromString(encumberedRaw)
	if err != nil {
		return domain.LedgerEffectsSummary{}, fmt.Errorf("parse computed_encumbered %q: %w", encumberedRaw, err)
	}

	return domain.LedgerEffectsSummary{
		ComputedLedgerBalance:     computedLedger,
		ComputedEncumberedBalance: computedEncumbered,
	}, nil
}

func scanLedgerEntry(row pgx.Row) (domain.LedgerEntry, error) {
	var e domain.LedgerEntry
	var entryType, deltaRaw, balanceAfterRaw, encumberedAfterRaw string
	err := row.Scan(
		&e.ID,
		&e.OrganizationID,
		&e.AccountID,
		&entryType,
		&e.CurrencyCode,
		&deltaRaw,
		&balanceAfterRaw,
		&encumberedAfterRaw,
		&e.AccountVersion,
		&e.ReversedEntryID,
		&e.ReferenceType,
		&e.ReferenceID,
		&e.CorrelationID,
		&e.CreatedAt,
	)
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	delta, err := money.FromString(deltaRaw)
	if err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("parse delta_amount %q: %w", deltaRaw, err)
	}
	balAfter, err := money.FromString(balanceAfterRaw)
	if err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("parse balance_after %q: %w", balanceAfterRaw, err)
	}
	encAfter, err := money.FromString(encumberedAfterRaw)
	if err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("parse encumbered_after %q: %w", encumberedAfterRaw, err)
	}

	e.EntryType = domain.LedgerEntryType(entryType)
	e.DeltaAmount = delta
	e.BalanceAfter = balAfter
	e.EncumberedAfter = encAfter
	return e, nil
}
