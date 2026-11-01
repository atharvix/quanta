// Package domain provides core enterprise domain models and validation rules
// mapped directly to the shared QUANTA canonical JSON schemas.
package domain

import (
	"fmt"
	"strings"
)

// Default financial parameters defined in shared/schemas/financial_problem.json.
const (
	DefaultStepSize         = 5.0
	DefaultRiskTolerance    = 0.50
	DefaultMinimumLiquidity = 0.20
	DefaultObjective        = "maximize_risk_adjusted_return"
	DefaultMarketState      = "normal"
)

// Allowed objectives enumerated in schema.
var AllowedObjectives = map[string]bool{
	"maximize_risk_adjusted_return": true,
	"minimize_risk":                 true,
	"maximize_liquidity":            true,
}

// Allowed market states enumerated in schema.
var AllowedMarketStates = map[string]bool{
	"normal":           true,
	"bull":             true,
	"bear":             true,
	"high_volatility":  true,
	"liquidity_crunch": true,
}

// Asset models a capital allocation opportunity (e.g. government bond, commercial paper).
type Asset struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	AssetType      string                 `json:"asset_type"`
	Cost           float64                `json:"cost"`
	ExpectedReturn float64                `json:"expected_return"`
	Risk           float64                `json:"risk"`
	Liquidity      float64                `json:"liquidity"`
	Volatility     float64                `json:"volatility"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Liability represents debt or payment obligation that requires cash reserves.
type Liability struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Principal          float64 `json:"principal"`
	InterestRate       float64 `json:"interest_rate"`
	MaturityMonths     int     `json:"maturity_months,omitempty"`
	Priority           int     `json:"priority"`
	IsMandatoryReserve bool    `json:"is_mandatory_reserve"`
}

// Constraint represents a business or regulatory restriction imposed on the allocation.
type Constraint struct {
	ID             string                 `json:"id"`
	ConstraintType string                 `json:"constraint_type"`
	Description    string                 `json:"description,omitempty"`
	Parameters     map[string]interface{} `json:"parameters"`
}

// FinancialProblem is the canonical payload sent to AI & Optimization solvers.
type FinancialProblem struct {
	Capital          float64      `json:"capital"`
	StepSize         float64      `json:"step_size"`
	Assets           []Asset      `json:"assets"`
	Liabilities      []Liability  `json:"liabilities"`
	Constraints      []Constraint `json:"constraints"`
	RiskTolerance    float64      `json:"risk_tolerance"`
	MinimumLiquidity float64      `json:"minimum_liquidity"`
	Objective        string       `json:"objective"`
	MarketState      string       `json:"market_state"`
}

// ApplyDefaults populates default values from the schema for any optional or empty fields.
func (f *FinancialProblem) ApplyDefaults() {
	if f.StepSize <= 0 {
		f.StepSize = DefaultStepSize
	}
	if f.RiskTolerance == 0 {
		f.RiskTolerance = DefaultRiskTolerance
	}
	if f.MinimumLiquidity == 0 {
		f.MinimumLiquidity = DefaultMinimumLiquidity
	}
	if strings.TrimSpace(f.Objective) == "" {
		f.Objective = DefaultObjective
	}
	if strings.TrimSpace(f.MarketState) == "" {
		f.MarketState = DefaultMarketState
	}
}

// Validate executes strict business validation matching shared/schemas/financial_problem.json.
// Returns a list of human-readable error descriptions.
func (f *FinancialProblem) Validate() []string {
	var errs []string

	// 1. Capital must be positive
	if f.Capital <= 0 {
		errs = append(errs, "capital must be greater than 0")
	}

	// 2. Step size cannot be negative
	if f.StepSize < 0 {
		errs = append(errs, "step_size cannot be negative")
	}

	// 3. Assets validation: at least 1 asset, unique IDs, correct bounds
	if len(f.Assets) == 0 {
		errs = append(errs, "assets must contain at least 1 asset")
	} else {
		seenIDs := make(map[string]bool)
		for i, a := range f.Assets {
			idx := fmt.Sprintf("assets[%d]", i)

			if strings.TrimSpace(a.ID) == "" {
				errs = append(errs, fmt.Sprintf("%s.id is required", idx))
			} else {
				if seenIDs[a.ID] {
					errs = append(errs, fmt.Sprintf("duplicate asset id '%s' detected", a.ID))
				}
				seenIDs[a.ID] = true
			}

			if strings.TrimSpace(a.Name) == "" {
				errs = append(errs, fmt.Sprintf("%s.name is required", idx))
			}
			if strings.TrimSpace(a.AssetType) == "" {
				errs = append(errs, fmt.Sprintf("%s.asset_type is required", idx))
			}
			if a.ExpectedReturn < -1.0 || a.ExpectedReturn > 5.0 {
				errs = append(errs, fmt.Sprintf("%s.expected_return must be between -1.0 and 5.0", idx))
			}
			if a.Risk < 0.0 || a.Risk > 1.0 {
				errs = append(errs, fmt.Sprintf("%s.risk must be between 0.0 and 1.0", idx))
			}
			if a.Liquidity < 0.0 || a.Liquidity > 1.0 {
				errs = append(errs, fmt.Sprintf("%s.liquidity must be between 0.0 and 1.0", idx))
			}
			if a.Cost < 0.0 {
				errs = append(errs, fmt.Sprintf("%s.cost cannot be negative", idx))
			}
			if a.Volatility < 0.0 {
				errs = append(errs, fmt.Sprintf("%s.volatility cannot be negative", idx))
			}
		}
	}

	// 4. Liabilities validation
	if f.Liabilities == nil {
		errs = append(errs, "liabilities array is required")
	} else {
		for i, l := range f.Liabilities {
			idx := fmt.Sprintf("liabilities[%d]", i)
			if strings.TrimSpace(l.ID) == "" {
				errs = append(errs, fmt.Sprintf("%s.id is required", idx))
			}
			if strings.TrimSpace(l.Name) == "" {
				errs = append(errs, fmt.Sprintf("%s.name is required", idx))
			}
			if l.Principal < 0.0 {
				errs = append(errs, fmt.Sprintf("%s.principal cannot be negative", idx))
			}
			if l.InterestRate < 0.0 {
				errs = append(errs, fmt.Sprintf("%s.interest_rate cannot be negative", idx))
			}
			if l.Priority < 0 {
				errs = append(errs, fmt.Sprintf("%s.priority cannot be negative", idx))
			}
		}
	}

	// 5. Constraints validation
	if f.Constraints == nil {
		errs = append(errs, "constraints array is required")
	} else {
		for i, c := range f.Constraints {
			idx := fmt.Sprintf("constraints[%d]", i)
			if strings.TrimSpace(c.ID) == "" {
				errs = append(errs, fmt.Sprintf("%s.id is required", idx))
			}
			if strings.TrimSpace(c.ConstraintType) == "" {
				errs = append(errs, fmt.Sprintf("%s.constraint_type is required", idx))
			}
			if c.Parameters == nil {
				errs = append(errs, fmt.Sprintf("%s.parameters cannot be nil", idx))
			}
		}
	}

	// 6. Risk tolerance bounds [0.0, 1.0]
	if f.RiskTolerance < 0.0 || f.RiskTolerance > 1.0 {
		errs = append(errs, "risk_tolerance must be between 0.0 and 1.0")
	}

	// 7. Minimum liquidity bounds [0.0, 1.0]
	if f.MinimumLiquidity < 0.0 || f.MinimumLiquidity > 1.0 {
		errs = append(errs, "minimum_liquidity must be between 0.0 and 1.0")
	}

	// 8. Objective enum verification
	if !AllowedObjectives[f.Objective] {
		errs = append(errs, fmt.Sprintf("objective '%s' is invalid, allowed values: [maximize_risk_adjusted_return, minimize_risk, maximize_liquidity]", f.Objective))
	}

	// 9. Market state enum verification
	if !AllowedMarketStates[f.MarketState] {
		errs = append(errs, fmt.Sprintf("market_state '%s' is invalid, allowed values: [normal, bull, bear, high_volatility, liquidity_crunch]", f.MarketState))
	}

	return errs
}
