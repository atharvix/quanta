package domain

import (
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"alice.bob+tag@sub.domain.co.in", true},
		{"ADMIN123@QUANTA.LOCAL", true},
		{"", false},
		{"plainaddress", false},
		{"@missingusername.com", false},
		{"missingat.com", false},
		{"missingdomain@.com", false},
		{strings.Repeat("a", 250) + "@example.com", false}, // > 254 chars
	}

	for _, tc := range tests {
		actual := ValidateEmail(tc.email)
		if actual != tc.valid {
			t.Errorf("ValidateEmail(%q) = %v; expected %v", tc.email, actual, tc.valid)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"short", false},
		{"1234567", false},
		{"12345678", true},
		{"a-very-strong-secure-password", true},
		{strings.Repeat("a", 72), true},
		{strings.Repeat("a", 73), false}, // exceeds 72-byte bcrypt limit
		{strings.Repeat("a", 200), false},
	}

	for _, tc := range tests {
		actual := ValidatePassword(tc.password)
		if actual != tc.valid {
			t.Errorf("ValidatePassword(len=%d) = %v; expected %v", len(tc.password), actual, tc.valid)
		}
	}
}

func TestRole_IsValid(t *testing.T) {
	if !RoleAdmin.IsValid() {
		t.Errorf("RoleAdmin should be valid")
	}
	if !RoleAnalyst.IsValid() {
		t.Errorf("RoleAnalyst should be valid")
	}
	if !RoleViewer.IsValid() {
		t.Errorf("RoleViewer should be valid")
	}
	if Role("superadmin").IsValid() {
		t.Errorf("Role 'superadmin' should be invalid")
	}
	if Role("").IsValid() {
		t.Errorf("Empty role should be invalid")
	}
}
