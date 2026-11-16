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
