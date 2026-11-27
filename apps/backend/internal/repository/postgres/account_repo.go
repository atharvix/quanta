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

type AccountRepository struct {
	db DBTX
}

func NewAccountRepository(db DBTX) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) CreateAccount(ctx context.Context, account domain.TreasuryAccount) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO treasury_accounts (
			id, organization_id, treasury_entity_id, account_code, name,
			currency_code, ledger_balance, encumbered_balance, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::NUMERIC(20,4), $8::NUMERIC(20,4), $9)
	`,
		account.ID,
		account.OrganizationID,
		account.TreasuryEntityID,
		account.AccountCode,
		account.Name,
		account.CurrencyCode,
		account.LedgerBalance.String(),
		account.EncumberedBalance.String(),
		account.Version,
	)
	return err
}

func (r *AccountRepository) GetAccount(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	return r.scanSingleAccount(ctx, `
		SELECT id, organization_id, treasury_entity_id, account_code, name, currency_code,
		       ledger_balance::TEXT, encumbered_balance::TEXT, version, created_at, updated_at
		FROM treasury_accounts
		WHERE organization_id = $1 AND id = $2
	`, orgID, accountID)
}

func (r *AccountRepository) GetAccountForUpdate(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	return r.scanSingleAccount(ctx, `
		SELECT id, organization_id, treasury_entity_id, account_code, name, currency_code,
		       ledger_balance::TEXT, encumbered_balance::TEXT, version, created_at, updated_at
		FROM treasury_accounts
		WHERE organization_id = $1 AND id = $2
		FOR UPDATE
	`, orgID, accountID)
}

func (r *AccountRepository) ListAccountsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.TreasuryAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, treasury_entity_id, account_code, name, currency_code,
		       ledger_balance::TEXT, encumbered_balance::TEXT, version, created_at, updated_at
		FROM treasury_accounts
		WHERE organization_id = $1 AND treasury_entity_id = $2
		ORDER BY account_code ASC
	`, orgID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.TreasuryAccount
	for rows.Next() {
		acc, err := scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

func (r *AccountRepository) UpdateAccountProjection(
	ctx context.Context,
	orgID, accountID uuid.UUID,
	expectedVersion int64,
	newLedgerBalance, newEncumberedBalance money.Amount,
) (domain.TreasuryAccount, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE treasury_accounts
		SET ledger_balance = $4::NUMERIC(20,4),
		    encumbered_balance = $5::NUMERIC(20,4),
		    version = version + 1,
		    updated_at = NOW()
		WHERE organization_id = $1 AND id = $2 AND version = $3
		RETURNING id, organization_id, treasury_entity_id, account_code, name, currency_code,
		          ledger_balance::TEXT, encumbered_balance::TEXT, version, created_at, updated_at
	`, orgID, accountID, expectedVersion, newLedgerBalance.String(), newEncumberedBalance.String())

	acc, err := scanAccountRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TreasuryAccount{}, domain.ErrVersionConflict
	}
	return acc, err
}

func (r *AccountRepository) scanSingleAccount(ctx context.Context, query string, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error) {
	row := r.db.QueryRow(ctx, query, orgID, accountID)
	acc, err := scanAccountRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TreasuryAccount{}, domain.ErrNotFound
	}
	return acc, err
}

func scanAccountRow(row pgx.Row) (domain.TreasuryAccount, error) {
	var acc domain.TreasuryAccount
	var ledgerRaw, encumberedRaw string
	err := row.Scan(
		&acc.ID,
		&acc.OrganizationID,
		&acc.TreasuryEntityID,
		&acc.AccountCode,
		&acc.Name,
		&acc.CurrencyCode,
		&ledgerRaw,
		&encumberedRaw,
		&acc.Version,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		return domain.TreasuryAccount{}, err
	}
	ledgerBal, err := money.FromString(ledgerRaw)
	if err != nil {
		return domain.TreasuryAccount{}, fmt.Errorf("parse ledger_balance %q: %w", ledgerRaw, err)
	}
	encumberedBal, err := money.FromString(encumberedRaw)
	if err != nil {
		return domain.TreasuryAccount{}, fmt.Errorf("parse encumbered_balance %q: %w", encumberedRaw, err)
	}
	acc.LedgerBalance = ledgerBal
	acc.EncumberedBalance = encumberedBal
	return acc, nil
}
