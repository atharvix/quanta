package domain

import (
	"strings"
	"testing"
)

// Helper to construct a base valid FinancialProblem matching shared/schemas/financial_problem.json.
func createValidProblem() FinancialProblem {
	return FinancialProblem{
		Capital:  100.0,
		StepSize: 5.0,
		Assets: []Asset{
			{
				ID:             "gov_bond",
				Name:           "Government Bonds",
				AssetType:      "gov_bond",
				Cost:           0.001,
				ExpectedReturn: 0.07,
				Risk:           0.05,
				Liquidity:      0.95,
				Volatility:     0.03,
			},
			{
				ID:             "corp_debt",
				Name:           "Corporate Debt",
				AssetType:      "corp_bond",
				Cost:           0.002,
				ExpectedReturn: 0.09,
				Risk:           0.15,
				Liquidity:      0.80,
				Volatility:     0.06,
			},
		},
		Liabilities: []Liability{
			{
				ID:                 "loan_1",
				Name:               "Senior Term Debt",
				Principal:          15.0,
				InterestRate:       0.085,
				MaturityMonths:     12,
				Priority:           1,
				IsMandatoryReserve: true,
			},
		},
		Constraints: []Constraint{
			{
				ID:             "c_min_liq",
				ConstraintType: "minimum_liquidity",
				Description:    "Maintain minimum 20% liquidity",
				Parameters:     map[string]interface{}{"threshold": 0.20},
			},
		},
		RiskTolerance:    0.50,
		MinimumLiquidity: 0.20,
		Objective:        "maximize_risk_adjusted_return",
		MarketState:      "normal",
	}
}

func TestFinancialProblem_Validate_ValidCase(t *testing.T) {
	prob := createValidProblem()
	errs := prob.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected 0 validation errors for valid problem, got %d: %v", len(errs), errs)
	}
}

func TestFinancialProblem_Validate_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(p *FinancialProblem)
		expectedError string
	}{
		{
			name: "Zero capital",
			mutate: func(p *FinancialProblem) {
				p.Capital = 0
			},
			expectedError: "capital must be greater than 0",
		},
		{
			name: "Negative capital",
			mutate: func(p *FinancialProblem) {
				p.Capital = -50
			},
			expectedError: "capital must be greater than 0",
		},
		{
			name: "Empty assets slice",
			mutate: func(p *FinancialProblem) {
				p.Assets = []Asset{}
			},
			expectedError: "assets must contain at least 1 asset",
		},
		{
			name: "Duplicate asset id",
			mutate: func(p *FinancialProblem) {
				p.Assets = append(p.Assets, Asset{
					ID:             "gov_bond", // Duplicate of first asset
					Name:           "Another Gov Bond",
					AssetType:      "gov_bond",
					ExpectedReturn: 0.05,
					Risk:           0.02,
					Liquidity:      0.90,
				})
			},
			expectedError: "duplicate asset id 'gov_bond' detected",
		},
		{
			name: "Asset risk out of bounds (> 1.0)",
			mutate: func(p *FinancialProblem) {
				p.Assets[0].Risk = 1.25
			},
			expectedError: "assets[0].risk must be between 0.0 and 1.0",
		},
		{
			name: "Asset risk negative (< 0.0)",
			mutate: func(p *FinancialProblem) {
				p.Assets[0].Risk = -0.1
			},
			expectedError: "assets[0].risk must be between 0.0 and 1.0",
		},
		{
			name: "Asset liquidity out of bounds (> 1.0)",
			mutate: func(p *FinancialProblem) {
				p.Assets[0].Liquidity = 1.5
			},
			expectedError: "assets[0].liquidity must be between 0.0 and 1.0",
		},
		{
			name: "Asset expected return out of bounds (< -1.0)",
			mutate: func(p *FinancialProblem) {
				p.Assets[0].ExpectedReturn = -1.5
			},
			expectedError: "assets[0].expected_return must be between -1.0 and 5.0",
		},
		{
			name: "Asset expected return out of bounds (> 5.0)",
			mutate: func(p *FinancialProblem) {
				p.Assets[0].ExpectedReturn = 6.0
			},
			expectedError: "assets[0].expected_return must be between -1.0 and 5.0",
		},
		{
			name: "Risk tolerance out of bounds (> 1.0)",
			mutate: func(p *FinancialProblem) {
				p.RiskTolerance = 1.5
			},
			expectedError: "risk_tolerance must be between 0.0 and 1.0",
		},
		{
			name: "Minimum liquidity out of bounds (< 0.0)",
			mutate: func(p *FinancialProblem) {
				p.MinimumLiquidity = -0.05
			},
			expectedError: "minimum_liquidity must be between 0.0 and 1.0",
		},
		{
			name: "Invalid objective",
			mutate: func(p *FinancialProblem) {
				p.Objective = "gamble_all_on_crypto"
			},
			expectedError: "objective 'gamble_all_on_crypto' is invalid",
		},
		{
			name: "Invalid market state",
			mutate: func(p *FinancialProblem) {
				p.MarketState = "apocalypse"
			},
			expectedError: "market_state 'apocalypse' is invalid",
		},
		{
			name: "Missing asset id",
			mutate: func(p *FinancialProblem) {
				p.Assets[0].ID = ""
			},
			expectedError: "assets[0].id is required",
		},
		{
			name: "Missing constraint parameters",
			mutate: func(p *FinancialProblem) {
				p.Constraints[0].Parameters = nil
			},
			expectedError: "constraints[0].parameters cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prob := createValidProblem()
			tt.mutate(&prob)

			errs := prob.Validate()
			if len(errs) == 0 {
				t.Fatalf("expected validation error containing '%s', got 0 errors", tt.expectedError)
			}

			found := false
			for _, e := range errs {
				if strings.Contains(e, tt.expectedError) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error containing '%s', but got errors: %v", tt.expectedError, errs)
			}
		})
	}
}

func TestFinancialProblem_ApplyDefaults(t *testing.T) {
	prob := FinancialProblem{
		Capital: 50.0,
	}
	prob.ApplyDefaults()

	if prob.StepSize != DefaultStepSize {
		t.Errorf("expected StepSize %f, got %f", DefaultStepSize, prob.StepSize)
	}
	if prob.RiskTolerance != DefaultRiskTolerance {
		t.Errorf("expected RiskTolerance %f, got %f", DefaultRiskTolerance, prob.RiskTolerance)
	}
	if prob.MinimumLiquidity != DefaultMinimumLiquidity {
		t.Errorf("expected MinimumLiquidity %f, got %f", DefaultMinimumLiquidity, prob.MinimumLiquidity)
	}
	if prob.Objective != DefaultObjective {
		t.Errorf("expected Objective '%s', got '%s'", DefaultObjective, prob.Objective)
	}
	if prob.MarketState != DefaultMarketState {
		t.Errorf("expected MarketState '%s', got '%s'", DefaultMarketState, prob.MarketState)
	}
}

func TestValidateAsset_CanonicalEnums(t *testing.T) {
	canonicalTypes := []string{
		"cash",
		"gov_bond",
		"corp_bond",
		"equity",
		"real_estate",
		"commodity",
		"other",
	}

	for _, assetType := range canonicalTypes {
		t.Run("Valid asset type: "+assetType, func(t *testing.T) {
			asset := Asset{
				ID:             "asset_" + assetType,
				Name:           "Asset " + assetType,
				AssetType:      assetType,
				Cost:           0.001,
				ExpectedReturn: 0.08,
				Risk:           0.1,
				Liquidity:      0.8,
				Volatility:     0.05,
			}
			errs := ValidateAsset(asset)
			if len(errs) != 0 {
				t.Fatalf("expected 0 errors for canonical type '%s', got: %v", assetType, errs)
			}
		})
	}
}

func TestValidateAsset_InvalidEnums(t *testing.T) {
	invalidCases := []struct {
		name          string
		assetType     string
		expectedError string
	}{
		{
			name:          "Legacy bond",
			assetType:     "bond",
			expectedError: "asset_type 'bond' is invalid, allowed values: [cash, gov_bond, corp_bond, equity, real_estate, commodity, other]",
		},
		{
			name:          "Government bonds plural",
			assetType:     "government_bonds",
			expectedError: "asset_type 'government_bonds' is invalid",
		},
		{
			name:          "Corporate bonds plural",
			assetType:     "corporate_bonds",
			expectedError: "asset_type 'corporate_bonds' is invalid",
		},
		{
			name:          "Empty string",
			assetType:     "",
			expectedError: "asset_type is required",
		},
		{
			name:          "Whitespace string",
			assetType:     "   ",
			expectedError: "asset_type is required",
		},
		{
			name:          "Unsupported crypto",
			assetType:     "crypto",
			expectedError: "asset_type 'crypto' is invalid",
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			asset := Asset{
				ID:             "asset_test",
				Name:           "Test Asset",
				AssetType:      tc.assetType,
				Cost:           0.001,
				ExpectedReturn: 0.08,
				Risk:           0.1,
				Liquidity:      0.8,
				Volatility:     0.05,
			}
			errs := ValidateAsset(asset)
			if len(errs) == 0 {
				t.Fatalf("expected validation error for invalid asset type '%s', got 0 errors", tc.assetType)
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e, tc.expectedError) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error containing '%s', got errors: %v", tc.expectedError, errs)
			}
		})
	}
}

func TestValidateAsset_Bounds(t *testing.T) {
	baseAsset := Asset{
		ID:             "valid_id",
		Name:           "Valid Name",
		AssetType:      "equity",
		Cost:           0.001,
		ExpectedReturn: 0.08,
		Risk:           0.1,
		Liquidity:      0.8,
		Volatility:     0.05,
	}

	t.Run("Missing ID", func(t *testing.T) {
		a := baseAsset
		a.ID = ""
		errs := ValidateAsset(a)
		if len(errs) == 0 || !strings.Contains(errs[0], "id is required") {
			t.Errorf("expected 'id is required', got: %v", errs)
		}
	})

	t.Run("Missing Name", func(t *testing.T) {
		a := baseAsset
		a.Name = ""
		errs := ValidateAsset(a)
		if len(errs) == 0 || !strings.Contains(errs[0], "name is required") {
			t.Errorf("expected 'name is required', got: %v", errs)
		}
	})

	t.Run("Negative Cost", func(t *testing.T) {
		a := baseAsset
		a.Cost = -1.0
		errs := ValidateAsset(a)
		if len(errs) == 0 || !strings.Contains(errs[0], "cost cannot be negative") {
			t.Errorf("expected 'cost cannot be negative', got: %v", errs)
		}
	})
}

func TestValidateLiability_Validation(t *testing.T) {
	t.Run("Valid Liability", func(t *testing.T) {
		l := Liability{
			ID:                 "debt_01",
			Name:               "Revolving Credit Facility",
			Principal:          1000000,
			InterestRate:       0.045,
			MaturityMonths:     36,
			Priority:           1,
			IsMandatoryReserve: true,
		}
		errs := ValidateLiability(l)
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors, got: %v", errs)
		}
	})

	t.Run("Invalid Liability Missing ID and Name", func(t *testing.T) {
		l := Liability{
			ID:           "",
			Name:         "",
			Principal:    -500,
			InterestRate: -0.02,
			Priority:     -1,
		}
		errs := ValidateLiability(l)
		if len(errs) != 5 {
			t.Errorf("expected 5 errors, got %d: %v", len(errs), errs)
		}
	})
}
