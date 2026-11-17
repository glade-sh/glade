package vm

import "strings"

// decimalScalesDiffer reports two Decimals of equal value but different scale.
// Native dec_DecHashColl_S09 and dec_a03 (surf_collections_strings, API 62.0
// and 67.0): List<Decimal>{1.5} == List<Decimal>{1.50} is false, because a list
// compares its Decimals as java.math.BigDecimal.equals does. A Double and a
// scalar Decimal comparison keep value equality.
func decimalScalesDiffer(left, right Value) bool {
	if left.Kind != ValueDecimal || right.Kind != ValueDecimal || isFloatBackedDecimal(left) || isFloatBackedDecimal(right) {
		return false
	}
	if left.Text == "" || right.Text == "" || left.Decimal != right.Decimal {
		return false
	}
	return decimalTextScale(left.Text) != decimalTextScale(right.Text)
}

// valueScaleDiffers reports a scale difference between two Decimals anywhere
// inside two otherwise equal collections: nested lists, sets, map values and the
// Decimal fields of the records a list holds. Native dec rows (DecEq, API 62.0
// and 67.0): List ==, equals, contains and indexOf, nested lists, a List<Object>
// and a Set of Decimals all compare scale (1.5 versus 1.50 and 3 versus 3.0).
func valueScaleDiffers(left, right Value) bool {
	if left.Kind == ValueDecimal && right.Kind == ValueDecimal {
		return decimalScalesDiffer(left, right)
	}
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if valueScaleDiffers(left.List[i], right.List[i]) {
				return true
			}
		}
	case ValueSet:
		for _, l := range left.Set {
			matched, sameValue := false, false
			for _, r := range right.Set {
				if l.Kind == ValueDecimal && r.Kind == ValueDecimal && !isFloatBackedDecimal(l) && !isFloatBackedDecimal(r) && l.Decimal == r.Decimal {
					sameValue = true
					if !decimalScalesDiffer(l, r) {
						matched = true
					}
				}
			}
			if sameValue && !matched {
				return true
			}
		}
	case ValueMap:
		for key, l := range left.Map {
			if r, ok := right.Map[key]; ok && valueScaleDiffers(l, r) {
				return true
			}
		}
	case ValueObject:
		// Only a record's Decimal fields: an instance of a class with its own
		// equals() goes through that equals(), never through this walk.
		if left.classInstance || left.projectClass || right.classInstance || right.projectClass || !sObjectValueType(left.Type) {
			return false
		}
		if len(left.Fields) != len(right.Fields) || len(left.Fields) == 0 {
			return false
		}
		for key, l := range left.Fields {
			if r, ok := right.Fields[key]; ok && valueScaleDiffers(l, r) {
				return true
			}
		}
	}
	return false
}

func decimalTextScale(text string) int {
	text = strings.TrimSpace(text)
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(text), "e")
	scale := 0
	if _, fraction, ok := strings.Cut(mantissa, "."); ok {
		scale = len(fraction)
	}
	if hasExponent {
		value := 0
		sign := 1
		for i, r := range exponent {
			switch {
			case i == 0 && r == '-':
				sign = -1
			case i == 0 && r == '+':
			case r >= '0' && r <= '9':
				value = value*10 + int(r-'0')
			}
		}
		scale -= sign * value
	}
	return scale
}
