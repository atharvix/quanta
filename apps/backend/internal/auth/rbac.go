package auth

import (
	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

// Permission represents an organization-scoped capability.
type Permission string

const (
	PermTreasuryRead      Permission = "treasury:read"
	PermLedgerMutate      Permission = "ledger:mutate"
	PermPolicyRead        Permission = "policy:read"
	PermPolicyWrite       Permission = "policy:write"
	PermCounterpartyRead  Permission = "counterparty:read"
	PermCounterpartyWrite Permission = "counterparty:write"
	PermProposalSubmit    Permission = "proposal:submit"
	PermProposalApprove   Permission = "proposal:approve"
	PermAuditRead         Permission = "audit:read"
	PermOrgAdmin          Permission = "org:admin"
)

var rolePermissions = map[domain.MemberRole]map[Permission]bool{
	domain.RoleViewer: {
		PermTreasuryRead:     true,
		PermPolicyRead:       true,
		PermCounterpartyRead: true,
		PermAuditRead:        true,
	},
	domain.RoleTreasuryAnalyst: {
		PermTreasuryRead:      true,
		PermLedgerMutate:      true,
		PermPolicyRead:        true,
		PermPolicyWrite:       true,
		PermCounterpartyRead:  true,
		PermCounterpartyWrite: true,
		PermProposalSubmit:    true,
	},
	domain.RoleTreasuryApprover: {
		PermTreasuryRead:     true,
		PermPolicyRead:       true,
		PermPolicyWrite:      true,
		PermCounterpartyRead: true,
		PermProposalApprove:  true,
		PermAuditRead:        true,
	},
	domain.RoleAdmin: {
		PermTreasuryRead:      true,
		PermLedgerMutate:      true,
		PermPolicyRead:        true,
		PermPolicyWrite:       true,
		PermCounterpartyRead:  true,
		PermCounterpartyWrite: true,
		PermAuditRead:         true,
		PermOrgAdmin:          true,
	},
}

// HasPermission returns true only if role explicitly grants perm. Defaults to deny.
func HasPermission(role domain.MemberRole, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	return perms[perm]
}

// EnforceMakerChecker ensures the submitter (maker) and approver (checker) are distinct users.
func EnforceMakerChecker(submittedBy, approverID uuid.UUID) error {
	if submittedBy == uuid.Nil || approverID == uuid.Nil {
		return domain.ErrMakerCheckerViolation
	}
	if submittedBy == approverID {
		return domain.ErrMakerCheckerViolation
	}
	return nil
}
