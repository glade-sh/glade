package vm

import "strings"

func scaleZeroNumberBoxedCastSourceName(value Value) string {
	if value.scaleZeroNumberTokenDouble {
		return "Double"
	}
	return runtimeValueTypeName(value)
}

// Boxed snapshots reject Decimal-to-Long, while direct statically
// Decimal field casts, Integer/Long widening and null remain valid.
func scaleZeroNumberBoxedLongCastRejected(target string, value Value) bool {
	return strings.EqualFold(target, "Long") && strings.EqualFold(value.Static, "Object") &&
		value.Kind == ValueDecimal && (!isFloatBackedDecimal(value) || value.scaleZeroNumberTokenDouble)
}
