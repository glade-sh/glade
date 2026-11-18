package vm

import (
	"math"
	"math/rand"
	"testing"
)

// setDecimalScalesDifferPairwise is the pairwise Set walk that
// setDecimalScalesDiffer replaces.
func setDecimalScalesDifferPairwise(left, right []Value) bool {
	for _, l := range left {
		matched, sameValue := false, false
		for _, r := range right {
			if l.Kind == ValueDecimal && r.Kind == ValueDecimal && !isFloatBackedDecimal(l) && !isFloatBackedDecimal(r) && l.Decimal == r.Decimal {
				sameValue = true
				if !decimalScalesDiffer(l, r) {
					matched = true
				}
			}
		}
		if sameValue && !matched {
			return true
		}
	}
	return false
}

func TestSetDecimalScalesDifferMatchesPairwise(t *testing.T) {
	rng := rand.New(rand.NewSource(312))
	texts := []string{"0", "0.0", "-0.0", "1", "1.0", "1.00", "2.5", "2.50", "3", "3.0", "1E+1", "1E1", "1e1", "10", "1E+2", "100", "0.1", "0.10", "1E-1"}
	pick := func() Value {
		switch rng.Intn(6) {
		case 5:
			// A Decimal with no text matches any scale.
			blank := Decimal([]float64{0, 1, 10, 2.5}[rng.Intn(4)])
			blank.Text = ""
			return blank
		case 0:
			return Int(int64(rng.Intn(4)))
		case 1:
			return decimalAsDouble(Decimal([]float64{0, math.Copysign(0, -1), 1, 2.5, math.NaN()}[rng.Intn(5)]))
		case 2:
			return String("x")
		default:
			value, err := decimalFromText(texts[rng.Intn(len(texts))])
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
	}
	for round := 0; round < 20000; round++ {
		left, right := make([]Value, rng.Intn(8)), make([]Value, rng.Intn(8))
		for i := range left {
			left[i] = pick()
		}
		for i := range right {
			right[i] = pick()
		}
		if got, want := setDecimalScalesDiffer(left, right), setDecimalScalesDifferPairwise(left, right); got != want {
			t.Fatalf("round %d: grouped %v, pairwise %v\nleft %v\nright %v", round, got, want, left, right)
		}
	}
}
