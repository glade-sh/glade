package vm

import (
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
)

// decimalDivideBigDecimalBySearch is the earlier form of decimalDivideBigDecimal:
// it tries every k from 0 to 33 until a*10^k divides by b.
func decimalDivideBigDecimalBySearch(left, right Value, leftRat, rightRat *big.Rat) (Value, bool) {
	s1, s2 := decimalScale(left), decimalScale(right)
	unscaled := func(rat *big.Rat, scale int) (*big.Int, bool) {
		scaled := new(big.Rat).Set(rat)
		factor := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absInt(scale))), nil))
		if scale >= 0 {
			scaled.Mul(scaled, factor)
		} else {
			scaled.Quo(scaled, factor)
		}
		return scaled.Num(), scaled.IsInt()
	}
	a, okA := unscaled(leftRat, s1)
	b, okB := unscaled(rightRat, s2)
	if okA && okB && b.Sign() != 0 {
		ten := big.NewInt(10)
		for k := 0; k <= 33; k++ {
			num := new(big.Int).Mul(a, new(big.Int).Exp(ten, big.NewInt(int64(k)), nil))
			quotient, remainder := new(big.Int).QuoRem(num, b, new(big.Int))
			if remainder.Sign() != 0 {
				continue
			}
			if len(new(big.Int).Abs(quotient).String()) > 33 {
				break
			}
			exponent := s2 - s1 - k
			value := new(big.Rat).SetInt(quotient)
			factor := new(big.Rat).SetInt(new(big.Int).Exp(ten, big.NewInt(int64(absInt(exponent))), nil))
			if exponent >= 0 {
				value.Mul(value, factor)
			} else {
				value.Quo(value, factor)
			}
			out := decimalFromRat(value, int64(s1-s2+k))
			if math.IsInf(out.Decimal, 0) || math.IsNaN(out.Decimal) {
				return Null, false
			}
			return out, true
		}
	}
	return decimalRound33(new(big.Rat).Quo(leftRat, rightRat))
}

// The direct shift must give the same Value as the search for every quotient:
// terminating, repeating, zero, negative, wide and over-33-digit ones.
func TestDecimalDivideShiftMatchesSearch(t *testing.T) {
	rng := rand.New(rand.NewSource(312))
	ten := big.NewInt(10)
	randomInt := func() *big.Int {
		switch rng.Intn(6) {
		case 0:
			// powers of 2 and 5 times a small factor: terminating quotients
			n := new(big.Int).Lsh(big.NewInt(int64(1+rng.Intn(9))), uint(rng.Intn(40)))
			return n.Mul(n, new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(rng.Intn(40))), nil))
		case 1:
			return big.NewInt(int64(rng.Intn(1000)))
		case 2:
			return new(big.Int).Exp(ten, big.NewInt(int64(rng.Intn(36))), nil)
		default:
			n := new(big.Int).Rand(rng, new(big.Int).Exp(ten, big.NewInt(int64(1+rng.Intn(34))), nil))
			return n
		}
	}
	checked := 0
	for i := 0; i < 20000; i++ {
		a, b := randomInt(), randomInt()
		if rng.Intn(4) == 0 {
			a.Neg(a)
		}
		if rng.Intn(4) == 0 {
			b.Neg(b)
		}
		if b.Sign() == 0 {
			continue
		}
		s1, s2 := int64(rng.Intn(12)), int64(rng.Intn(12))
		leftRat := new(big.Rat).SetFrac(a, new(big.Int).Exp(ten, big.NewInt(s1), nil))
		rightRat := new(big.Rat).SetFrac(b, new(big.Int).Exp(ten, big.NewInt(s2), nil))
		left, right := decimalFromRat(leftRat, s1), decimalFromRat(rightRat, s2)
		lr, _ := valueDecimalRat(left)
		rr, _ := valueDecimalRat(right)
		got, gotOK := decimalDivideBigDecimal(left, right, new(big.Rat).Set(lr), new(big.Rat).Set(rr))
		want, wantOK := decimalDivideBigDecimalBySearch(left, right, new(big.Rat).Set(lr), new(big.Rat).Set(rr))
		if gotOK != wantOK || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s/%s (scales %d,%d): got %#v %v, want %#v %v", left.Text, right.Text, s1, s2, got, gotOK, want, wantOK)
		}
		checked++
	}
	if checked < 15000 {
		t.Fatalf("only %d cases checked", checked)
	}
}

func TestDecimalTerminatingShift(t *testing.T) {
	for _, tc := range []struct {
		a, b int64
		k    int
		ok   bool
	}{
		{1, 3, 0, false}, {1, 4, 2, true}, {1, 8, 3, true}, {1, 5, 1, true}, {1, 125, 3, true},
		{3, 6, 1, true}, {6, 3, 0, true}, {0, 7, 0, true}, {-1, 20, 2, true}, {7, -14, 1, true}, {1, 1600, 6, true}, {2, 7, 0, false},
	} {
		k, ok := decimalTerminatingShift(big.NewInt(tc.a), big.NewInt(tc.b))
		if k != tc.k || ok != tc.ok {
			t.Errorf("shift(%d, %d) = %d %v, want %d %v", tc.a, tc.b, k, ok, tc.k, tc.ok)
		}
	}
}
