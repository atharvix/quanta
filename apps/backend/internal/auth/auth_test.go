package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("QuantaDemo#2026!")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if err := VerifyPassword(hash, "QuantaDemo#2026!"); err != nil {
		t.Fatalf("VerifyPassword failed for valid password: %v", err)
	}
	if err := VerifyPassword(hash, "wrong-password"); err == nil {
		t.Fatal("expected VerifyPassword to fail for wrong password")
	}
}

func TestTokenService_IssueVerifyExpiryAndTamper(t *testing.T) {
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	svc, err := NewTokenService("0123456789abcdef0123456789abcdef", "quanta-backend", "quanta-api", 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenService failed: %v", err)
	}
	svc.nowFn = func() time.Time { return now }

	userID := uuid.New()
	token, _, err := svc.IssueAccessToken(userID, "analyst@quanta.local")
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	claims, err := svc.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken failed: %v", err)
	}
	if claims.Subject != userID || claims.Email != "analyst@quanta.local" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// Tamper signature
	tampered := token[:len(token)-2] + "aa"
	if _, err := svc.VerifyAccessToken(tampered); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}

	// Issuer/audience mismatch
	otherSvc, _ := NewTokenService("0123456789abcdef0123456789abcdef", "other-issuer", "quanta-api", 15*time.Minute)
	otherSvc.nowFn = func() time.Time { return now }
	if _, err := otherSvc.VerifyAccessToken(token); err == nil {
		t.Fatal("expected issuer mismatch to be rejected")
	}

	// Expiry check
	svc.nowFn = func() time.Time { return now.Add(16 * time.Minute) }
	if _, err := svc.VerifyAccessToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestRBAC_PermissionsAndMakerChecker(t *testing.T) {
	if HasPermission(domain.MemberRole("UNKNOWN"), PermTreasuryRead) {
		t.Fatal("expected unknown role to be denied by default")
	}
	if !HasPermission(domain.RoleTreasuryAnalyst, PermLedgerMutate) {
		t.Fatal("expected TreasuryAnalyst to have PermLedgerMutate")
	}
	if HasPermission(domain.RoleTreasuryAnalyst, PermProposalApprove) {
		t.Fatal("expected TreasuryAnalyst NOT to have PermProposalApprove")
	}
	if !HasPermission(domain.RoleTreasuryApprover, PermProposalApprove) {
		t.Fatal("expected TreasuryApprover to have PermProposalApprove")
	}
	if HasPermission(domain.RoleTreasuryApprover, PermLedgerMutate) {
		t.Fatal("expected TreasuryApprover NOT to have PermLedgerMutate")
	}

	userA := uuid.New()
	userB := uuid.New()
	if err := EnforceMakerChecker(userA, userB); err != nil {
		t.Fatalf("expected distinct maker and checker to pass, got %v", err)
	}
	if err := EnforceMakerChecker(userA, userA); !errors.Is(err, domain.ErrMakerCheckerViolation) {
		t.Fatalf("expected ErrMakerCheckerViolation when maker == checker, got %v", err)
	}
}
