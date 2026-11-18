package dml

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Native dec_DecHashProbe_F17 and F20 (surf_collections_strings, API 62.0 and
// 67.0): inserting 123456.789 into Percent(3,2) or Number(10,8) fails with
// NUMBER_OUTSIDE_VALID_RANGE. API 62.0 words it "<label>: value outside of
// valid range on numeric field: <value>"; API 67.0 adds "the number <value> is
// too large; please enter a value with no more than N digits and S decimal
// places". The check covers Percent and Number fields that declare a nonzero
// scale, and rejects a value whose integer part has more than precision-scale
// digits. Currency and values that only round past the limit keep their earlier
// behavior; the capture does not observe them.
const (
	scaledNumberOverflowOff = iota
	scaledNumberOverflowShort
	scaledNumberOverflowDetailed
)

func (e *Engine) validateScaledNumberOverflow(definition storage.ObjectDefinition, namespace string, record storage.Record) error {
	mode := e.Options.ScaledNumberOverflow
	if mode == scaledNumberOverflowOff {
		return nil
	}
	for name, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if !ok {
			continue
		}
		field := definition.Fields[canonical]
		display := strings.ToUpper(strings.TrimSpace(field.DisplayType))
		if field.Type != storage.FieldDecimal || (display != "DOUBLE" && display != "PERCENT") ||
			!field.ScaleSpecified || field.Scale < 1 || field.Precision < 1 || field.Precision > 18 || field.Scale >= field.Precision {
			continue
		}
		var text string
		switch value.Kind {
		case storage.ValueInteger:
			text = strconv.FormatInt(value.Integer, 10)
		case storage.ValueDecimal:
			text = value.Decimal
		default:
			continue
		}
		number, ok := new(big.Rat).SetString(text)
		if !ok {
			continue
		}
		digits := field.Precision - field.Scale
		limit := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil))
		if new(big.Rat).Abs(number).Cmp(limit) < 0 {
			continue
		}
		label := strings.TrimSpace(field.Label)
		if label == "" {
			label = canonical
		}
		if mode == scaledNumberOverflowShort {
			return dmlErrorf("NUMBER_OUTSIDE_VALID_RANGE", []string{canonical}, "%s: value outside of valid range on numeric field: %s", label, text)
		}
		return dmlErrorf("NUMBER_OUTSIDE_VALID_RANGE", []string{canonical}, "%s: value outside of valid range on numeric field: the number %s is too large; please enter a value with no more than %d digits and %d decimal places", label, text, digits, field.Scale)
	}
	return nil
}
