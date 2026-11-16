package vm

import (
	"crypto/sha256"
	"encoding/hex"
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
		x = math.Mod(float64(x*3.7)+0.123456789, 1.9999)
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

// archIdentitySHA256 is the digest of archIdentityLines on amd64. The same
// digest on arm64 proves every result is bit-identical across architectures.
const archIdentitySHA256 = "f6299b5cf927399c6ea91f6cb8669a8a65b5954f84890949c812f23063345099"

// archIdentitySpot lists readable rows from the same amd64 run.
var archIdentitySpot = map[string]string{
	"Math.cbrt 3fb999999999999a":                   "3fddb4c7760bcff3",
	"Math.cbrt 3fd0000000000000":                   "3fe428a2f98d728b",
	"Math.acos 3fb999999999999a":                   "3ff787b22ce3f590",
	"Math.acos 3fd0000000000000":                   "3ff51700e0c14b25",
	"Math.asin 3fb999999999999a":                   "3fb9a49276037884",
	"Math.asin 3fd0000000000000":                   "3fd02be9ce0b87cd",
	"Math.atan 3fb999999999999a":                   "3fb983e282e2cc4d",
	"Math.atan 3fd0000000000000":                   "3fcf5b75f92c80dd",
	"Math.cos 3fb999999999999a":                    "3fefd712f9a817c0",
	"Math.cos 3fd0000000000000":                    "3fef01549f7deea1",
	"Math.sin 3fb999999999999a":                    "3fb98eaecb8bcb2c",
	"Math.sin 3fd0000000000000":                    "3fcfaaeed4f31577",
	"Math.tan 3fb999999999999a":                    "3fb9af8877430b80",
	"Math.tan 3fd0000000000000":                    "3fd05785a43c4c56",
	"Math.cosh 3fb999999999999a":                   "3ff0147f40224b38",
	"Math.cosh 3fd0000000000000":                   "3ff080ab05ca6146",
	"Math.sinh 3fb999999999999a":                   "3fb9a487337b59b4",
	"Math.sinh 3fd0000000000000":                   "3fd02accd9d08102",
	"Math.tanh 3fb999999999999a":                   "3fb983d7795f413a",
	"Math.tanh 3fd0000000000000":                   "3fcf597ea69a1c86",
	"Math.exp 3fb999999999999a":                    "3ff1aec7b35a00d4",
	"Math.exp 3fd0000000000000":                    "3ff48b5e3c3e8186",
	"Math.log 3fb999999999999a":                    "c0026bb1bbb55515",
	"Math.log 3fd0000000000000":                    "bff62e42fefa39ef",
	"Math.log10 3fb999999999999a":                  "bfefffffffffffff",
	"Math.log10 3fd0000000000000":                  "bfe34413509f79ff",
	"Math.sqrt 3fb999999999999a":                   "3fd43d136248490f",
	"Math.sqrt 3fd0000000000000":                   "3fe0000000000000",
	"Math.pow 3fc999999999999a 3fd0000000000000":   "3fe56652116c816c",
	"Math.atan2 3fb999999999999a 3fd0000000000000": "3fd85a376b677dc0",
	"Math.pow 3feb333333333333 3ff0000000000000":   "3feb333333333333",
	"Math.atan2 3fe8000000000000 3ff0000000000000": "3fe4978fa3269ee1",
	"Location.getDistance.km 3fb999999999999a 3ffc000000000000 3ff8000000000000 4020800000000000": "40871a0ccfe184ce",
	"Location.getDistance.mi 3fb999999999999a 3ffc000000000000 3ff8000000000000 4020800000000000": "407cb5a3ca6204f5",
	"Location.getDistance.km 3fe0000000000000 4015000000000000 4008000000000000 4030800000000000": "4094032a770fd569",
	"Location.getDistance.mi 3fe0000000000000 4015000000000000 4008000000000000 4030800000000000": "4088dec64f136121",
}

func TestMathArchIdentity(t *testing.T) {
	lines := archIdentityLines(t)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	got := hex.EncodeToString(sum[:])
	for _, line := range lines {
		cut := strings.LastIndex(line, " ")
		if want, ok := archIdentitySpot[line[:cut]]; ok && line[cut+1:] != want {
			t.Errorf("%s = %s, want %s", line[:cut], line[cut+1:], want)
		}
	}
	if got != archIdentitySHA256 {
		t.Fatalf("math results differ from the amd64 reference: sha256 %s, want %s (%d rows)", got, archIdentitySHA256, len(lines))
	}
}
