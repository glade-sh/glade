package vm

import "strings"

// Numeric Set.contains uses the element's boxed representation for its lookup
// hash. E23N001-006/029 preserve Decimal scale, E23N008-013/031 use Double
// boxing, and E23N022-027 widen Integer arguments to Long. Null and nonnumeric
// arguments keep the existing lookup path, including equals-only Object sets.
func setNullNumericContainsNeedle(set Value, needle Value) (Value, error) {
	element, typed := collectionElementType(set.Type)
	if !typed {
		return needle, nil
	}
	// The captured integral arguments are Integer, not boxed Long values.
	if needle.Kind == ValueInt && needle.Type != "" && !strings.EqualFold(needle.Type, "Integer") {
		return needle, nil
	}
	switch canonicalApexScalarType(element) {
	case "Decimal", "Long":
		if needle.Kind == ValueInt {
			return coerceAssignable(element, needle)
		}
	case "Double":
		if needle.Kind == ValueInt || needle.Kind == ValueDecimal {
			return coerceAssignable(element, needle)
		}
	}
	return needle, nil
}
