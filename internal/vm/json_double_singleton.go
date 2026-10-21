package vm

import (
	"encoding/json"
	"math"
	"strings"
)

// S06_DOUBLE captures a boxed integral Double in a singleton Object list.
// Preserve its fractional marker without changing other collection or field
// serialization shapes, or the existing non-finite-number failure path.
func capturedDoubleSingletonJSON(value Value) (any, bool) {
	if value.Kind != ValueList || len(value.List) != 1 || !strings.EqualFold(value.Type, "List<Object>") {
		return nil, false
	}
	item := value.List[0]
	if !isFloatBackedDecimal(item) || math.IsNaN(item.Decimal) || math.IsInf(item.Decimal, 0) || math.Trunc(item.Decimal) != item.Decimal {
		return nil, false
	}
	return []any{json.Number(doubleDisplayText(item.Decimal))}, true
}
