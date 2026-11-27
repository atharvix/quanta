package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/money"
)

var (
	ErrNotFound                     = errors.New("resource not found")
	ErrCrossTenantAccess            = errors.New("cross-tenant access denied")
	ErrCurrencyMismatch             = errors.New("currency does not match treasury base currency")
	ErrInsufficientAvailableBalance = errors.New("insufficient available balance")
	ErrInsufficientEncumbered       = errors.New("insufficient encumbered balance")
	ErrInvalidAmount                = errors.New("amount must be positive")
	ErrAlreadyReversed              = errors.New("ledger entry has already been reversed")
	ErrCannotReverseEntryType       = errors.New("only CREDIT or DEBIT entries can be reversed directly")
	ErrVersionConflict              = errors.New("optimistic concurrency version conflict")
	ErrReconciliationMismatch       = errors.New("account projection does not match ledger entries")
	ErrInvalidCredentials           = errors.New("invalid email or password")
	ErrInactiveUser                 = errors.New("user account is inactive")
	ErrForbidden                    = errors.New("access denied")
	ErrMakerCheckerViolation        = errors.New("maker-checker violation: submitter cannot approve own proposal")
	ErrPolicyViolation              = errors.New("allocation violates liquidity policy floors")
	ErrCounterpartyLimitExceeded    = errors.New("counterparty exposure limit exceeded")
	ErrInvalidConfiguration         = errors.New("invalid or conflicting configuration")
	ErrProposalNotCompletedRun      = errors.New("proposals may only be created from COMPLETED optimization runs")
	ErrInvalidProposalTransition    = errors.New("invalid proposal status transition")
	ErrProposalExpired              = errors.New("allocation proposal has expired")
	ErrStaleProposalState           = errors.New("underlying accounts or policies changed since proposal creation")
)

type OrganizationStatus string

const (
	OrgStatusActive    OrganizationStatus = "ACTIVE"
	OrgStatusSuspended OrganizationStatus = "SUSPENDED"
)

type MemberRole string

const (
	RoleAdmin            MemberRole = "ADMIN"
	RoleTreasuryApprover MemberRole = "TREASURY_APPROVER"
	RoleTreasuryAnalyst  MemberRole = "TREASURY_ANALYST"
	RoleViewer           MemberRole = "VIEWER"
)

type LedgerEntryType string

const (
	EntryCredit       LedgerEntryType = "CREDIT"
	EntryDebit        LedgerEntryType = "DEBIT"
	EntryReversal     LedgerEntryType = "REVERSAL"
	EntryHoldPlaced   LedgerEntryType = "HOLD_PLACED"
	EntryHoldReleased LedgerEntryType = "HOLD_RELEASED"
	EntryHoldSettled  LedgerEntryType = "HOLD_SETTLED"
)

type Organization struct {
	ID        uuid.UUID
	Name      string
	Status    OrganizationStatus
	CreatedAt time.Time
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	IsActive     bool
	CreatedAt    time.Time
}

type OrganizationMembership struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           MemberRole
	Status         OrganizationStatus
	CreatedAt      time.Time
}

type TreasuryEntity struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	BaseCurrency   string
	IsSynthetic    bool
	CreatedAt      time.Time
}

type TreasuryAccount struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	TreasuryEntityID  uuid.UUID
	AccountCode       string
	Name              string
	CurrencyCode      string
	LedgerBalance     money.Amount
	EncumberedBalance money.Amount
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AvailableBalance returns ledger_balance - encumbered_balance.
func (a TreasuryAccount) AvailableBalance() money.Amount {
	return a.LedgerBalance.Sub(a.EncumberedBalance)
}

type LedgerEntry struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	AccountID       uuid.UUID
	EntryType       LedgerEntryType
	CurrencyCode    string
	DeltaAmount     money.Amount
	BalanceAfter    money.Amount
	EncumberedAfter money.Amount
	AccountVersion  int64
	ReversedEntryID *uuid.UUID
	ReferenceType   string
	ReferenceID     uuid.UUID
	CorrelationID   string
	CreatedAt       time.Time
}

type LedgerEffectsSummary struct {
	ComputedLedgerBalance     money.Amount
	ComputedEncumberedBalance money.Amount
}

type ReconciliationReport struct {
	OrganizationID            uuid.UUID
	AccountID                 uuid.UUID
	StoredLedgerBalance       money.Amount
	ComputedLedgerBalance     money.Amount
	StoredEncumberedBalance   money.Amount
	ComputedEncumberedBalance money.Amount
	AccountVersion            int64
	Matched                   bool
}

type AuditEvent struct {
	ID             uuid.UUID
	OrganizationID *uuid.UUID
	ActorID        *uuid.UUID
	ActorRole      string
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	CorrelationID  string
	Metadata       json.RawMessage
	CreatedAt      time.Time
}

type LiquidityPolicy struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	TreasuryEntityID      uuid.UUID
	Version               int64
	MinOperatingLiquidity money.Amount
	MinEmergencyReserve   money.Amount
	IsActive              bool
	CreatedBy             uuid.UUID
	CreatedAt             time.Time
}

// TotalReserveFloor returns min_operating_liquidity + min_emergency_reserve.
func (p LiquidityPolicy) TotalReserveFloor() money.Amount {
	return p.MinOperatingLiquidity.Add(p.MinEmergencyReserve)
}

type Counterparty struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	TreasuryEntityID uuid.UUID
	Name             string
	CurrencyCode     string
	MaxExposureLimit money.Amount
	CurrentExposure  money.Amount
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RemainingHeadroom returns max_exposure_limit - current_exposure.
func (c Counterparty) RemainingHeadroom() money.Amount {
	return c.MaxExposureLimit.Sub(c.CurrentExposure)
}

type OptimizationStatus string

const (
	OptStatusCompleted          OptimizationStatus = "COMPLETED"
	OptStatusInfeasible         OptimizationStatus = "INFEASIBLE"
	OptStatusSolverFailed       OptimizationStatus = "SOLVER_FAILED"
	OptStatusVerificationFailed OptimizationStatus = "VERIFICATION_FAILED"
	OptStatusCancelled          OptimizationStatus = "CANCELLED"
)

type ProposalStatus string

const (
	ProposalStatusDraft           ProposalStatus = "DRAFT"
	ProposalStatusPendingApproval ProposalStatus = "PENDING_APPROVAL"
	ProposalStatusApproved        ProposalStatus = "APPROVED"
	ProposalStatusRejected        ProposalStatus = "REJECTED"
	ProposalStatusExpired         ProposalStatus = "EXPIRED"
)

type ApprovalDecisionType string

const (
	DecisionApprove ApprovalDecisionType = "APPROVE"
	DecisionReject  ApprovalDecisionType = "REJECT"
)

type OptimizationRun struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	TreasuryEntityID uuid.UUID
	RequestedBy      uuid.UUID
	CorrelationID    string
	SolverCategory   string
	Status           OptimizationStatus
	Feasible         bool
	ProblemSnapshot  json.RawMessage
	RawResponse      json.RawMessage
	FXSnapshotIDs    json.RawMessage
	RuntimeMs        int
	CreatedAt        time.Time
}

type AllocationProposal struct {
	ID                      uuid.UUID
	OrganizationID          uuid.UUID
	OptimizationRunID       uuid.UUID
	TreasuryEntityID        uuid.UUID
	SupersedesProposalID    *uuid.UUID
	Status                  ProposalStatus
	InvestableCapital       money.Amount
	TotalAllocated          money.Amount
	ResidualCash            money.Amount
	VerifiedAllocations     json.RawMessage
	SnapshotAccountVersions json.RawMessage
	SnapshotPolicyVersion   int64
	SubmittedBy             *uuid.UUID
	SubmittedAt             *time.Time
	ExpiresAt               *time.Time
	Version                 int64
	CreatedAt               time.Time
}

type ApprovalDecision struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	ProposalID          uuid.UUID
	ApproverID          uuid.UUID
	Decision            ApprovalDecisionType
	Rationale           string
	LiveBalanceSnapshot json.RawMessage
	DecidedAt           time.Time
}
