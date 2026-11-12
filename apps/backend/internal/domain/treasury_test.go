package domain

import (
	"strings"
	"testing"
)

func TestCreateTreasuryInput_Validate(t *testing.T) {
	tests := []struct {
		name          string
		input         CreateTreasuryInput
		expectValid   bool
		expectedError string
	}{
		{
			name: "Valid input",
			input: CreateTreasuryInput{
				Name:              "Acme Treasury Corp",
				BaseCurrency:      "INR",
				TotalCapital:      10000000.0,
				RiskTolerance:     0.45,
				MinLiquidityRatio: 0.25,
			},
			expectValid: true,
		},
		{
			name: "Missing name",
			input: CreateTreasuryInput{
				Name:         "",
				TotalCapital: 500000.0,
			},
			expectValid:   false,
			expectedError: "name is required",
		},
		{
			name: "Non-positive capital",
			input: CreateTreasuryInput{
				Name:         "Negative Capital Corp",
				TotalCapital: 0,
			},
			expectValid:   false,
			expectedError: "total_capital must be greater than 0",
		},
		{
			name: "Risk tolerance out of bounds",
			input: CreateTreasuryInput{
				Name:          "Risk Corp",
				TotalCapital:  1000.0,
				RiskTolerance: 1.5,
			},
			expectValid:   false,
			expectedError: "risk_tolerance must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.input.Validate()
			if tt.expectValid && len(errs) != 0 {
				t.Fatalf("expected valid input, got errors: %v", errs)
			}
			if !tt.expectValid {
				if len(errs) == 0 {
					t.Fatalf("expected error containing '%s', got 0 errors", tt.expectedError)
				}
				found := false
				for _, e := range errs {
					if strings.Contains(e, tt.expectedError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing '%s', got: %v", tt.expectedError, errs)
				}
			}
		})
	}
}

func TestCreateTreasuryInput_ApplyDefaults(t *testing.T) {
	input := CreateTreasuryInput{
		Name:         "Default Corp",
		TotalCapital: 100000.0,
	}
	input.ApplyDefaults()

	if input.BaseCurrency != "INR" {
		t.Errorf("expected default BaseCurrency 'INR', got '%s'", input.BaseCurrency)
	}
	if input.RiskTolerance != 0.50 {
		t.Errorf("expected default RiskTolerance 0.50, got %f", input.RiskTolerance)
	}
	if input.MinLiquidityRatio != 0.20 {
		t.Errorf("expected default MinLiquidityRatio 0.20, got %f", input.MinLiquidityRatio)
	}
}
