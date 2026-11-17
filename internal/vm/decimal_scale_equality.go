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
