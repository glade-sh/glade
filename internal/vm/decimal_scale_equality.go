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
		// O(n): bucket the right side's Decimals by value and keep their scales; a
		// Set without a Decimal (Integer, String, Id ...) answers at once.
		var scales map[float64]map[int]bool
		for _, r := range right.Set {
			if r.Kind == ValueDecimal && !isFloatBackedDecimal(r) {
				if scales == nil {
					scales = make(map[float64]map[int]bool)
				}
				if scales[r.Decimal] == nil {
					scales[r.Decimal] = make(map[int]bool)
				}
				// An empty Text matches any scale, as decimalScalesDiffer treats it.
				if r.Text == "" {
					scales[r.Decimal][-1] = true
				} else {
					scales[r.Decimal][decimalTextScale(r.Text)] = true
				}
			}
		}
		if scales == nil {
			return false
		}
		for _, l := range left.Set {
			if l.Kind != ValueDecimal || isFloatBackedDecimal(l) {
				continue
			}
			if l.Text == "" {
				continue
			}
			if bucket, ok := scales[l.Decimal]; ok && !bucket[-1] && !bucket[decimalTextScale(l.Text)] {
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
