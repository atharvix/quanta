package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFromString_HalfEvenRounding(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "exact 4 decimals", in: "1250.5000", want: "1250.5000"},
		{name: "half to even down (even 4th digit)", in: "10.00025", want: "10.0002"},
		{name: "half to even up (odd 4th digit)", in: "10.00035", want: "10.0004"},
		{name: "above half rounds up", in: "10.00026", want: "10.0003"},
		{name: "below half rounds down", in: "10.00024", want: "10.0002"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromString(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("FromString(%q) = %s, want %s", tc.in, got.String(), tc.want)
			}
		})
	}

	if _, err := FromString(""); err == nil {
		t.Errorf("expected error for empty string")
	}
	if _, err := FromString("not-a-number"); err == nil {
		t.Errorf("expected error for invalid decimal string")
	}
}

func TestAllocateByWeights_CapitalConservation(t *testing.T) {
	t.Run("conserves capital exactly across repeating thirds", func(t *testing.T) {
		total := MustFromString("100.0000")
		oneThird := decimal.NewFromInt(1).Div(decimal.NewFromInt(3))
		weights := []decimal.Decimal{oneThird, oneThird, oneThird}

		allocs, residual, err := AllocateByWeights(total, weights)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sum := residual
		for _, a := range allocs {
			if a.IsNegative() {
				t.Fatalf("allocation must not be negative: %s", a.String())
			}
			sum = sum.Add(a)
		}

		if !sum.Equal(total) {
			t.Fatalf("capital conservation violated: sum=%s != total=%s", sum.String(), total.String())
		}
		if residual.String() != "0.0001" {
			t.Errorf("expected 0.0001 residual cash from 3 x 33.3333, got %s", residual.String())
		}
	})

	t.Run("partial weights route remainder to unallocated cash", func(t *testing.T) {
		total := MustFromString("1000.0000")
		weights := []decimal.Decimal{
			decimal.RequireFromString("0.25"),
			decimal.RequireFromString("0.35"),
		}

		allocs, residual, err := AllocateByWeights(total, weights)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allocs[0].String() != "250.0000" || allocs[1].String() != "350.0000" {
			t.Errorf("unexpected allocations: %v", allocs)
		}
		if residual.String() != "400.0000" {
			t.Errorf("expected residual 400.0000, got %s", residual.String())
		}
	})

	t.Run("rejects negative total or weights exceeding 1", func(t *testing.T) {
		if _, _, err := AllocateByWeights(MustFromString("-1.0000"), nil); err == nil {
			t.Errorf("expected error for negative total")
		}
		if _, _, err := AllocateByWeights(MustFromString("100"), []decimal.Decimal{decimal.RequireFromString("0.6"), decimal.RequireFromString("0.5")}); err == nil {
			t.Errorf("expected error when weights sum > 1")
		}
	})
}
