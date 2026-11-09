package vm

import (
	"math"
	"strconv"
	"strings"
)

// Numeric rendering follows the API67 black-box family oracle. Double uses
// shortest round-trip digits with scientific notation outside [1e-3, 1e7).
// Decimal preserves scale and uses scientific notation below exponent -6.
func numericDisplayText(value Value) string {
	if isFloatBackedDecimal(value) {
		return doubleDisplayText(value.Decimal)
	}
	text := decimalPlainText(value)
	if strings.ContainsAny(text, "eE") {
		return text
	}
	unsigned := strings.TrimPrefix(text, "-")
	digits := strings.ReplaceAll(unsigned, ".", "")
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return text
	}
	exponent := len(digits) - decimalScale(value) - 1
	if exponent >= -6 {
		return text
	}
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign = "-"
	}
	mantissa := digits[:1]
	if len(digits) > 1 {
		mantissa += "." + digits[1:]
	}
	return sign + mantissa + "E" + strconv.Itoa(exponent)
}

func doubleDisplayText(value float64) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	abs := math.Abs(value)
	if abs == math.SmallestNonzeroFloat64 {
		// Captured: the platform writes the smallest subnormal as 4.9E-324,
		// not the shortest round-trip 5.0E-324.
		if value < 0 {
			return "-4.9E-324"
		}
		return "4.9E-324"
	}
	if abs == 0 || (abs >= 0.001 && abs < 10000000) {
		text := strconv.FormatFloat(value, 'f', -1, 64)
		if !strings.Contains(text, ".") {
			text += ".0"
		}
		return text
	}
	text := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exp, _ := strings.Cut(text, "e")
	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	exponent, _ := strconv.Atoi(exp)
	return mantissa + "E" + strconv.Itoa(exponent)
}

func doubleToInteger(value float64) int32 {
	if math.IsNaN(value) {
		return 0
	}
	if value >= math.MaxInt32 {
		return math.MaxInt32
	}
	if value <= math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}

func doubleToLong(value float64) int64 {
	if math.IsNaN(value) {
		return 0
	}
	if value >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	if value <= float64(math.MinInt64) {
		return math.MinInt64
	}
	return int64(value)
}

// decimalFromDouble converts a Double to a Decimal through its public text. The
// platform rejects NaN and the infinities.
func decimalFromDouble(value float64) (Value, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		// The platform names both infinities 'Infinity' here.
		name := "NaN"
		if math.IsInf(value, 0) {
			name = "Infinity"
		}
		return Null, newExceptionError("System.MathException", "Cannot represent Double '"+name+"' as a Decimal")
	}
	return decimalFromText(doubleDisplayText(value))
}
