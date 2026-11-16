package vm

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// archIdentityInputs is a deterministic grid that exercises argument reduction,
// small and large magnitudes, and negative values.
func archIdentityInputs() []float64 {
	inputs := []float64{0.1, 0.25, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 7, 10, 33.3, 100.5, 700, -0.3, -1, -2.75, -9.9, -45.5}
	x := 0.0137
	for i := 0; i < 160; i++ {
		x = math.Mod(x*3.7+0.123456789, 1.9999)
		inputs = append(inputs, x-0.9)
		inputs = append(inputs, x*float64(i+1)*0.37)
	}
	return inputs
}

var archIdentityUnary = []string{
	"Math.cbrt", "Math.acos", "Math.asin", "Math.atan", "Math.cos", "Math.sin", "Math.tan",
	"Math.cosh", "Math.sinh", "Math.tanh", "Math.exp", "Math.log", "Math.log10", "Math.sqrt",
}

// archIdentityLines renders every case as "label hex-bits-of-result".
func archIdentityLines(t testing.TB) []string {
	var lines []string
	for _, callee := range archIdentityUnary {
		for _, in := range archIdentityInputs() {
			got, err := mathUnary(callee, []Value{decimalAsDouble(Decimal(in))})
			lines = append(lines, fmt.Sprintf("%s %016x %s", callee, math.Float64bits(in), archIdentityResult(got, err)))
		}
	}
	inputs := archIdentityInputs()
	for i := 0; i+1 < len(inputs); i += 3 {
		for _, callee := range []string{"Math.pow", "Math.atan2"} {
			a, b := inputs[i], inputs[i+1]
			if callee == "Math.pow" {
				a = math.Abs(a) + 0.1
				b = math.Mod(b, 9)
			}
			got, err := mathBinary(callee, []Value{decimalAsDouble(Decimal(a)), decimalAsDouble(Decimal(b))})
			lines = append(lines, fmt.Sprintf("%s %016x %016x %s", callee, math.Float64bits(a), math.Float64bits(b), archIdentityResult(got, err)))
		}
	}
	for i := 0; i+3 < len(inputs); i += 2 {
		lat1 := math.Mod(inputs[i], 90)
		lon1 := math.Mod(inputs[i+1]*7, 180)
		lat2 := math.Mod(inputs[i+2]*3, 90)
		lon2 := math.Mod(inputs[i+3]*11, 180)
		for _, unit := range []string{"km", "mi"} {
			left := newLocation(decimalAsDouble(Decimal(lat1)), decimalAsDouble(Decimal(lon1)))
			right := newLocation(decimalAsDouble(Decimal(lat2)), decimalAsDouble(Decimal(lon2)))
			got, err := locationDistance(left, right, unit)
			lines = append(lines, fmt.Sprintf("Location.getDistance.%s %016x %016x %016x %016x %s", unit,
				math.Float64bits(lat1), math.Float64bits(lon1), math.Float64bits(lat2), math.Float64bits(lon2), archIdentityResult(got, err)))
		}
	}
	return lines
}

func archIdentityResult(v Value, err error) string {
	if err != nil {
		return "err"
	}
	return fmt.Sprintf("%016x", math.Float64bits(numericFloat(v)))
}

func TestMathArchDump(t *testing.T) {
	if !testing.Verbose() {
		t.Skip("dump needs -v")
	}
	t.Log("\n" + strings.Join(archIdentityLines(t), "\n"))
}
