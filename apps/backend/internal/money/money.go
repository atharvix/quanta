// Package money implements exact fixed-scale decimal monetary arithmetic.
// Authoritative monetary values must never use binary floating-point types.
package money

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// DefaultScale matches PostgreSQL NUMERIC(20, 4) monetary precision.
const DefaultScale int32 = 4

// Amount represents an immutable, fixed-scale monetary value.
type Amount struct {
	val decimal.Decimal
}

// Zero returns an Amount equal to 0.0000.
func Zero() Amount {
	return Amount{val: decimal.Zero.RoundBank(DefaultScale)}
}

// FromString parses a decimal string (e.g. "1000.2500") into an Amount rounded half-to-even at DefaultScale.
func FromString(raw string) (Amount, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Amount{}, fmt.Errorf("monetary amount string must not be empty")
	}
	d, err := decimal.NewFromString(trimmed)
	if err != nil {
		return Amount{}, fmt.Errorf("invalid monetary amount %q: %w", raw, err)
	}
	return Amount{val: d.RoundBank(DefaultScale)}, nil
}

// MustFromString parses a decimal string and panics on error (intended for tests and constants).
func MustFromString(raw string) Amount {
	a, err := FromString(raw)
	if err != nil {
		panic(err)
	}
	return a
}

// Add returns a + b at DefaultScale.
func (a Amount) Add(b Amount) Amount {
	return Amount{val: a.val.Add(b.val).RoundBank(DefaultScale)}
}

// Sub returns a - b at DefaultScale.
func (a Amount) Sub(b Amount) Amount {
	return Amount{val: a.val.Sub(b.val).RoundBank(DefaultScale)}
}

// MulDecimal multiplies a by a decimal factor and rounds half-to-even at DefaultScale.
func (a Amount) MulDecimal(factor decimal.Decimal) Amount {
	return Amount{val: a.val.Mul(factor).RoundBank(DefaultScale)}
}

// RoundHalfEven rounds an arbitrary decimal.Decimal to DefaultScale using Banker's Rounding.
func RoundHalfEven(d decimal.Decimal) Amount {
	return Amount{val: d.RoundBank(DefaultScale)}
}

// IsNegative reports whether the amount is strictly less than zero.
func (a Amount) IsNegative() bool {
	return a.val.IsNegative()
}

// IsPositive reports whether the amount is strictly greater than zero.
func (a Amount) IsPositive() bool {
	return a.val.IsPositive()
}

// IsZero reports whether the amount equals zero.
func (a Amount) IsZero() bool {
	return a.val.IsZero()
}

// Neg returns -a at DefaultScale.
func (a Amount) Neg() Amount {
	return Amount{val: a.val.Neg().RoundBank(DefaultScale)}
}

// Cmp compares a and b, returning -1 if a < b, 0 if a == b, and +1 if a > b.
func (a Amount) Cmp(b Amount) int {
	return a.val.Cmp(b.val)
}

// Equal reports whether a and b represent the exact same monetary value.
func (a Amount) Equal(b Amount) bool {
	return a.val.Equal(b.val)
}

// Decimal returns the underlying shopspring/decimal value.
func (a Amount) Decimal() decimal.Decimal {
	return a.val
}

// String formats the amount with exactly DefaultScale fractional digits.
func (a Amount) String() string {
	return a.val.StringFixed(DefaultScale)
}

// AllocateByWeights distributes total across weights using half-to-even rounding and assigns
// any sub-unit quantization residual to unallocatedCash so total capital is conserved exactly.
func AllocateByWeights(total Amount, weights []decimal.Decimal) ([]Amount, Amount, error) {
	if total.IsNegative() {
		return nil, Zero(), fmt.Errorf("total capital must not be negative: %s", total.String())
	}

	weightSum := decimal.Zero
	for i, w := range weights {
		if w.IsNegative() {
			return nil, Zero(), fmt.Errorf("weight[%d] must not be negative: %s", i, w.String())
		}
		weightSum = weightSum.Add(w)
	}
	if weightSum.GreaterThan(decimal.NewFromInt(1)) {
		return nil, Zero(), fmt.Errorf("sum of weights (%s) must not exceed 1", weightSum.String())
	}

	allocations := make([]Amount, len(weights))
	allocatedSum := Zero()
	for i, w := range weights {
		alloc := total.MulDecimal(w)
		allocations[i] = alloc
		allocatedSum = allocatedSum.Add(alloc)
	}

	residual := total.Sub(allocatedSum)
	if residual.IsNegative() {
		// Defensive guard against edge-case half-even round-up on full-weight sums.
		if len(allocations) > 0 {
			allocations[len(allocations)-1] = allocations[len(allocations)-1].Add(residual)
			residual = Zero()
		}
	}

	return allocations, residual, nil
}
