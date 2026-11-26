package domain

import (
	"context"
	"errors"
	"testing"
)

func TestCheckTreasuryAccess(t *testing.T) {
	ownerID := "user-123"
	otherID := "user-456"

	treasuryWithOwner := &Treasury{
		ID:      "treasury-abc",
		OwnerID: &ownerID,
	}

	treasuryWithoutOwner := &Treasury{
		ID:      "treasury-xyz",
		OwnerID: nil,
	}

	t.Run("Nil treasury returns ErrNotFound", func(t *testing.T) {
		err := CheckTreasuryAccess(context.Background(), nil)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Unauthenticated context on unassigned treasury is permitted", func(t *testing.T) {
		err := CheckTreasuryAccess(context.Background(), treasuryWithoutOwner)
		if err != nil {
			t.Fatalf("expected nil error for unassigned treasury in unauthenticated context, got %v", err)
		}
	})

	t.Run("Unauthenticated context cannot access owned treasury", func(t *testing.T) {
		err := CheckTreasuryAccess(context.Background(), treasuryWithOwner)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for unauthenticated context on owned treasury, got %v", err)
		}
	})

	t.Run("Admin has access to any treasury", func(t *testing.T) {
		ctx := WithUserContext(context.Background(), "admin-1", "admin@quanta.local", RoleAdmin)
		if err := CheckTreasuryAccess(ctx, treasuryWithOwner); err != nil {
			t.Errorf("admin should have access to owned treasury: %v", err)
		}
		if err := CheckTreasuryAccess(ctx, treasuryWithoutOwner); err != nil {
			t.Errorf("admin should have access to unowned treasury: %v", err)
		}
	})

	t.Run("Analyst owner has access to own treasury", func(t *testing.T) {
		ctx := WithUserContext(context.Background(), ownerID, "analyst@quanta.local", RoleAnalyst)
		if err := CheckTreasuryAccess(ctx, treasuryWithOwner); err != nil {
			t.Fatalf("expected owner analyst to have access, got %v", err)
		}
	})

	t.Run("Analyst non-owner is forbidden", func(t *testing.T) {
		ctx := WithUserContext(context.Background(), otherID, "other@quanta.local", RoleAnalyst)
		err := CheckTreasuryAccess(ctx, treasuryWithOwner)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for non-owner analyst, got %v", err)
		}
	})

	t.Run("Analyst accessing unassigned treasury is forbidden", func(t *testing.T) {
		ctx := WithUserContext(context.Background(), ownerID, "analyst@quanta.local", RoleAnalyst)
		err := CheckTreasuryAccess(ctx, treasuryWithoutOwner)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for unassigned treasury, got %v", err)
		}
	})
}
