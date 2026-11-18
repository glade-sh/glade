package stablemath

import (
	"math"
	"testing"
)

// The unfused ports must stay within a few ulps of the standard library.
func TestPortsMatchStandardLibraryClosely(t *testing.T) {
	type fn struct {
		name   string
		stable func(float64) float64
		std    func(float64) float64
		domain func(float64) bool
	}
	any := func(float64) bool { return true }
	unit := func(x float64) bool { return x >= -1 && x <= 1 }
	positive := func(x float64) bool { return x > 0 }
	small := func(x float64) bool { return math.Abs(x) < 700 }
	fns := []fn{
		{"Sin", SinStable, math.Sin, any}, {"Cos", CosStable, math.Cos, any}, {"Tan", TanStable, math.Tan, any},
		{"Asin", AsinStable, math.Asin, unit}, {"Acos", AcosStable, math.Acos, unit}, {"Atan", AtanStable, math.Atan, any},
		{"Sinh", SinhStable, math.Sinh, small}, {"Cosh", CoshStable, math.Cosh, small}, {"Tanh", TanhStable, math.Tanh, any},
		{"Exp", ExpStable, math.Exp, small}, {"Expm1", Expm1Stable, math.Expm1, small}, {"Log", LogStable, math.Log, positive},
		{"Log10", Log10Stable, math.Log10, positive}, {"Cbrt", CbrtStable, math.Cbrt, any},
	}
	x := 0.0137
	for i := 0; i < 400; i++ {
		x = math.Mod(float64(x*3.7)+0.123456789, 1.9999)
		for _, in := range []float64{x - 0.9, float64(x*float64(i+1)) * 0.37, -x * 5} {
			for _, f := range fns {
				if !f.domain(in) {
					continue
				}
				got, want := f.stable(in), f.std(in)
				if math.Abs(got-want) > 1e-13*math.Max(1, math.Abs(want)) {
					t.Fatalf("%s(%v) = %v, standard library %v", f.name, in, got, want)
				}
			}
			if in > 0 {
				for _, y := range []float64{0.3, 2, 3.5, -1.5} {
					got, want := PowStable(in, y), math.Pow(in, y)
					if math.Abs(got-want) > 1e-13*math.Max(1, math.Abs(want)) {
						t.Fatalf("Pow(%v, %v) = %v, standard library %v", in, y, got, want)
					}
				}
			}
			if got, want := Atan2Stable(in, x+1), math.Atan2(in, x+1); math.Abs(got-want) > 1e-13 {
				t.Fatalf("Atan2(%v, %v) = %v, standard library %v", in, x+1, got, want)
			}
		}
	}
}

func sameFloat(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// Special values must match IEEE and the Go documentation exactly, including
// the sign of zero.
func TestSpecialValuesExact(t *testing.T) {
	nan, inf, negZero := math.NaN(), math.Inf(1), math.Copysign(0, -1)
	pi := math.Pi
	cases := []struct {
		name string
		fn   func(float64) float64
		in   float64
		want float64
	}{
		{"Sin", SinStable, nan, nan}, {"Sin", SinStable, inf, nan}, {"Sin", SinStable, -inf, nan}, {"Sin", SinStable, 0, 0}, {"Sin", SinStable, negZero, negZero},
		{"Cos", CosStable, nan, nan}, {"Cos", CosStable, inf, nan}, {"Cos", CosStable, -inf, nan}, {"Cos", CosStable, 0, 1}, {"Cos", CosStable, negZero, 1},
		{"Tan", TanStable, nan, nan}, {"Tan", TanStable, inf, nan}, {"Tan", TanStable, -inf, nan}, {"Tan", TanStable, 0, 0}, {"Tan", TanStable, negZero, negZero},
		{"Asin", AsinStable, nan, nan}, {"Asin", AsinStable, 2, nan}, {"Asin", AsinStable, -2, nan}, {"Asin", AsinStable, 0, 0}, {"Asin", AsinStable, negZero, negZero},
		{"Acos", AcosStable, nan, nan}, {"Acos", AcosStable, 2, nan}, {"Acos", AcosStable, 1, 0}, {"Acos", AcosStable, -1, pi},
		{"Atan", AtanStable, nan, nan}, {"Atan", AtanStable, inf, pi / 2}, {"Atan", AtanStable, -inf, -pi / 2}, {"Atan", AtanStable, 0, 0}, {"Atan", AtanStable, negZero, negZero},
		{"Sinh", SinhStable, nan, nan}, {"Sinh", SinhStable, inf, inf}, {"Sinh", SinhStable, -inf, -inf}, {"Sinh", SinhStable, 0, 0}, {"Sinh", SinhStable, negZero, negZero},
		{"Cosh", CoshStable, nan, nan}, {"Cosh", CoshStable, inf, inf}, {"Cosh", CoshStable, -inf, inf}, {"Cosh", CoshStable, 0, 1}, {"Cosh", CoshStable, negZero, 1},
		{"Tanh", TanhStable, nan, nan}, {"Tanh", TanhStable, inf, 1}, {"Tanh", TanhStable, -inf, -1}, {"Tanh", TanhStable, 0, 0}, {"Tanh", TanhStable, negZero, negZero},
		{"Exp", ExpStable, nan, nan}, {"Exp", ExpStable, inf, inf}, {"Exp", ExpStable, -inf, 0}, {"Exp", ExpStable, 0, 1}, {"Exp", ExpStable, negZero, 1}, {"Exp", ExpStable, 710, inf}, {"Exp", ExpStable, -746, 0},
		{"Exp", ExpStable, -745, 5e-324},
		{"Expm1", Expm1Stable, nan, nan}, {"Expm1", Expm1Stable, inf, inf}, {"Expm1", Expm1Stable, -inf, -1}, {"Expm1", Expm1Stable, 0, 0}, {"Expm1", Expm1Stable, negZero, negZero},
		{"Log", LogStable, nan, nan}, {"Log", LogStable, inf, inf}, {"Log", LogStable, -inf, nan}, {"Log", LogStable, -1, nan}, {"Log", LogStable, 0, -inf}, {"Log", LogStable, negZero, -inf}, {"Log", LogStable, 1, 0},
		{"Log10", Log10Stable, nan, nan}, {"Log10", Log10Stable, inf, inf}, {"Log10", Log10Stable, -1, nan}, {"Log10", Log10Stable, 0, -inf}, {"Log10", Log10Stable, 1, 0}, {"Log10", Log10Stable, 100, 2}, {"Log10", Log10Stable, 1000, 3},
		{"Cbrt", CbrtStable, nan, nan}, {"Cbrt", CbrtStable, inf, inf}, {"Cbrt", CbrtStable, -inf, -inf}, {"Cbrt", CbrtStable, 0, 0}, {"Cbrt", CbrtStable, negZero, negZero}, {"Cbrt", CbrtStable, 8, 2}, {"Cbrt", CbrtStable, -27, -3},
	}
	for _, c := range cases {
		if got := c.fn(c.in); !sameFloat(got, c.want) {
			t.Errorf("%s(%v) = %v (%016x), want %v (%016x)", c.name, c.in, got, math.Float64bits(got), c.want, math.Float64bits(c.want))
		}
	}
	pow := []struct{ x, y, want float64 }{
		{2, 10, 1024}, {-2, 3, -8}, {nan, 0, 1}, {1, nan, 1}, {nan, 1, nan}, {0, -1, inf}, {negZero, -1, -inf}, {0, 1, 0}, {negZero, 3, negZero},
		{-1, inf, 1}, {2, inf, inf}, {0.5, inf, 0}, {2, -inf, 0}, {inf, 2, inf}, {inf, -2, 0}, {-inf, 3, -inf}, {-2, 0.5, nan}, {4, 0.5, 2}, {4, -0.5, 0.5}, {5e-324, 0, 1},
	}
	for _, c := range pow {
		if got := PowStable(c.x, c.y); !sameFloat(got, c.want) {
			t.Errorf("Pow(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	atan2 := []struct{ y, x, want float64 }{
		{0, 1, 0}, {negZero, 1, negZero}, {0, -1, pi}, {negZero, -1, -pi}, {1, 0, pi / 2}, {-1, 0, -pi / 2}, {inf, inf, pi / 4}, {inf, -inf, 3 * pi / 4}, {nan, 1, nan}, {1, nan, nan},
	}
	for _, c := range atan2 {
		if got := Atan2Stable(c.y, c.x); !sameFloat(got, c.want) {
			t.Errorf("Atan2(%v, %v) = %v, want %v", c.y, c.x, got, c.want)
		}
	}
}

// Values near overflow, underflow and the subnormal range, where the standard
// library's amd64 assembly is wrong or the architectures disagree.
func TestEdgeRangeValues(t *testing.T) {
	if got := ExpStable(709.78); math.IsInf(got, 0) || got < 1.79e308 {
		t.Errorf("Exp(709.78) = %v, want finite near 1.793e308", got)
	}
	near := func(name string, got, want, tol float64) {
		t.Helper()
		if math.Abs(got-want) > tol {
			t.Errorf("%s = %.17g, want %.17g within %g", name, got, want, tol)
		}
	}
	near("Log(5e-324)", LogStable(5e-324), -744.4400719213812, 1e-12)
	near("Log10(5e-324)", Log10Stable(5e-324), -323.30621534311577, 1e-12)
	near("Pow(5e-324, 1e-10)", PowStable(5e-324, 1e-10), 0.99999992555599558, 1e-13)
	near("Sin(1e300)", SinStable(1e300), -0.8178819121159085, 1e-15)
}
