package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
)

// TxRepositories provides repository instances bound to a single ACID transaction.
type TxRepositories struct {
	Accounts  AccountRepository
	Ledger    LedgerRepository
	Audit     AuditRepository
	Proposals ProposalRepository
	Decisions ApprovalDecisionRepository
}

// TxManager executes financial operations inside an ACID transaction boundary.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context, repos TxRepositories) error) error
}

// UserRepository manages global user identities and cross-organization memberships.
type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error)
	ListMembershipsByUser(ctx context.Context, userID uuid.UUID) ([]domain.OrganizationMembership, error)
}

// OrganizationRepository manages tenant organizations and memberships.
type OrganizationRepository interface {
	CreateOrganization(ctx context.Context, org domain.Organization) error
	GetOrganization(ctx context.Context, orgID uuid.UUID) (domain.Organization, error)
	GetMembership(ctx context.Context, orgID, userID uuid.UUID) (domain.OrganizationMembership, error)
}

// TreasuryRepository manages organization-scoped treasury entities.
type TreasuryRepository interface {
	CreateTreasuryEntity(ctx context.Context, entity domain.TreasuryEntity) error
	GetTreasuryEntity(ctx context.Context, orgID, entityID uuid.UUID) (domain.TreasuryEntity, error)
	ListTreasuryEntities(ctx context.Context, orgID uuid.UUID) ([]domain.TreasuryEntity, error)
}

// AccountRepository manages organization-scoped treasury account projections.
type AccountRepository interface {
	CreateAccount(ctx context.Context, account domain.TreasuryAccount) error
	GetAccount(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error)
	GetAccountForUpdate(ctx context.Context, orgID, accountID uuid.UUID) (domain.TreasuryAccount, error)
	ListAccountsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.TreasuryAccount, error)
	UpdateAccountProjection(
		ctx context.Context,
		orgID, accountID uuid.UUID,
		expectedVersion int64,
		newLedgerBalance, newEncumberedBalance money.Amount,
	) (domain.TreasuryAccount, error)
}

// LedgerRepository manages append-only ledger_entries.
type LedgerRepository interface {
	AppendEntry(ctx context.Context, entry domain.LedgerEntry) error
	GetEntry(ctx context.Context, orgID, entryID uuid.UUID) (domain.LedgerEntry, error)
	HasReversalForEntry(ctx context.Context, orgID, entryID uuid.UUID) (bool, error)
	ListEntriesByAccount(ctx context.Context, orgID, accountID uuid.UUID) ([]domain.LedgerEntry, error)
	SumEntryEffectsByAccount(ctx context.Context, orgID, accountID uuid.UUID) (domain.LedgerEffectsSummary, error)
}

// LiquidityPolicyRepository manages organization-scoped liquidity policies.
type LiquidityPolicyRepository interface {
	CreatePolicy(ctx context.Context, policy domain.LiquidityPolicy) error
	GetActivePolicy(ctx context.Context, orgID, entityID uuid.UUID) (domain.LiquidityPolicy, error)
	GetPolicyByID(ctx context.Context, orgID, policyID uuid.UUID) (domain.LiquidityPolicy, error)
	ListPoliciesByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.LiquidityPolicy, error)
	UpdatePolicy(ctx context.Context, policy domain.LiquidityPolicy) (domain.LiquidityPolicy, error)
}

// CounterpartyRepository manages organization-scoped counterparties and exposure limits.
type CounterpartyRepository interface {
	CreateCounterparty(ctx context.Context, cp domain.Counterparty) error
	GetCounterparty(ctx context.Context, orgID, counterpartyID uuid.UUID) (domain.Counterparty, error)
	ListCounterpartiesByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.Counterparty, error)
	UpdateCounterparty(ctx context.Context, cp domain.Counterparty, expectedVersion int64) (domain.Counterparty, error)
}

// AuditRepository manages append-only audit_events.
type AuditRepository interface {
	AppendAuditEvent(ctx context.Context, event domain.AuditEvent) error
	ListAuditEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.AuditEvent, error)
}

// OptimizationRunRepository manages persisted optimization runs.
type OptimizationRunRepository interface {
	CreateRun(ctx context.Context, run domain.OptimizationRun) error
	GetRun(ctx context.Context, orgID, runID uuid.UUID) (domain.OptimizationRun, error)
	ListRunsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.OptimizationRun, error)
}

// ProposalRepository manages allocation proposals and optimistic status transitions.
type ProposalRepository interface {
	CreateProposal(ctx context.Context, proposal domain.AllocationProposal) error
	GetProposal(ctx context.Context, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error)
	GetProposalForUpdate(ctx context.Context, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error)
	ListProposalsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.AllocationProposal, error)
	UpdateProposalStatus(
		ctx context.Context,
		orgID, proposalID uuid.UUID,
		expectedVersion int64,
		newStatus domain.ProposalStatus,
		submittedBy *uuid.UUID,
		submittedAt, expiresAt *int64,
	) (domain.AllocationProposal, error)
}

// ApprovalDecisionRepository manages append-only approval_decisions.
type ApprovalDecisionRepository interface {
	AppendDecision(ctx context.Context, decision domain.ApprovalDecision) error
	ListDecisionsByProposal(ctx context.Context, orgID, proposalID uuid.UUID) ([]domain.ApprovalDecision, error)
}
