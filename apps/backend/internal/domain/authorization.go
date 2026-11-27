package domain

import "context"

// CheckTreasuryAccess enforces resource-level authorization:
// - Platform Admin has unrestricted access to all corporate treasuries.
// - Analyst and Viewer can access a treasury they own (OwnerID matches authenticated caller ID).
// - Unassigned/legacy treasuries (OwnerID == nil or empty) are accessible to Admin and Analyst roles,
//   while Viewer access is denied.
// - Unauthenticated internal/unit-test contexts on unassigned treasuries are permitted.
func CheckTreasuryAccess(ctx context.Context, treasury *Treasury) error {
	if treasury == nil {
		return ErrNotFound
	}

	callerRole := RoleFromContext(ctx)
	if callerRole == RoleAdmin {
		return nil
	}

	callerID := UserIDFromContext(ctx)

	// If the treasury has an explicit owner assigned:
	if treasury.OwnerID != nil && *treasury.OwnerID != "" {
		if callerID == "" || *treasury.OwnerID != callerID {
			return ErrForbidden
		}
		return nil
	}

	// Treasury is unassigned (OwnerID == nil or empty):
	// Allow Analysts (and unauthenticated internal test contexts) while blocking Viewers.
	if callerRole == RoleViewer {
		return ErrForbidden
	}

	return nil
}
