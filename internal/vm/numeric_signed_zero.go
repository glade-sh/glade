package vm

import (
	"math"
	"strings"
)

// signedDoubleZeroEqual covers the captured Double zero comparisons only.
func signedDoubleZeroEqual(left, right Value) (bool, bool) {
	if !isFloatBackedDecimal(left) || !isFloatBackedDecimal(right) || left.Decimal != 0 || right.Decimal != 0 {
		return false, false
	}
	return math.Signbit(left.Decimal) == math.Signbit(right.Decimal), true
}

func signedDoubleIntegerZeroEqual(left, right Value) (bool, bool) {
	if left.Kind == ValueInt {
		left, right = right, left
	}
	if !isFloatBackedDecimal(left) || left.Decimal != 0 || right.Kind != ValueInt || right.Int != 0 || strings.EqualFold(right.Type, "Long") {
		return false, false
	}
	return !math.Signbit(left.Decimal), true
}

func isDoubleList(value Value) bool {
	element, ok := collectionElementType(value.Type)
	return ok && strings.EqualFold(element, "Double")
}
