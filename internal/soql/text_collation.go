package soql

import (
	"strings"
	"unicode/utf8"
)

//go:generate go run collation_weights_generate.go

// Text collation for English locales. Native rows in testdata/collation show
// that, for a user whose LocaleSidKey is en_*, SOQL compares text character by
// character: ASCII characters use a fixed weight (asciiCollationWeight, derived
// from the captured single-character order, letters case-insensitive), every
// character at or above U+0080 sorts after z in code point order without case
// folding or expansion, and a shorter prefix sorts first. ORDER BY, WHERE
// ranges, MIN and MAX share this order, and GROUP BY merges text values that
// differ only in ASCII case. Other locales keep compareSOQLText until they are
// captured.

// englishTextLocale reports whether locale is an en_* LocaleSidKey. An empty
// locale keeps the folded comparator.
func englishTextLocale(locale string) bool {
	return len(locale) > 3 && locale[2] == '_' && (locale[0] == 'e' || locale[0] == 'E') && (locale[1] == 'n' || locale[1] == 'N')
}

// textCollationNonASCIIBase keeps every non-ASCII code point above all ASCII
// weights while preserving code point order.
const textCollationNonASCIIBase = 256

func textCollationWeight(r rune) int32 {
	if r >= 0 && r < 128 {
		return int32(asciiCollationWeight[r])
	}
	return textCollationNonASCIIBase + int32(r)
}

// compareSOQLTextCollated compares two strings by the en_* text collation.
// Strings that differ only in ASCII case tie.
func compareSOQLTextCollated(left, right string) int {
	for len(left) > 0 && len(right) > 0 {
		var lw, rw int32
		if left[0] < utf8.RuneSelf {
			lw = int32(asciiCollationWeight[left[0]])
			left = left[1:]
		} else {
			r, size := utf8.DecodeRuneInString(left)
			lw = textCollationNonASCIIBase + int32(r)
			left = left[size:]
		}
		if right[0] < utf8.RuneSelf {
			rw = int32(asciiCollationWeight[right[0]])
			right = right[1:]
		} else {
			r, size := utf8.DecodeRuneInString(right)
			rw = textCollationNonASCIIBase + int32(r)
			right = right[size:]
		}
		if lw != rw {
			if lw < rw {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(left) == len(right):
		return 0
	case len(left) == 0:
		return -1
	}
	return 1
}

// collateOrder returns order with the collation flag set. The caller's slice
// is shared with the cached query and is never modified.
func collateOrder(order []OrderSpec, collate bool) []OrderSpec {
	if !collate || len(order) == 0 {
		return order
	}
	stamped := make([]OrderSpec, len(order))
	copy(stamped, order)
	for i := range stamped {
		stamped[i].textCollation = true
	}
	return stamped
}

func collateAggregates(aggregates []Aggregate) []Aggregate {
	if len(aggregates) == 0 {
		return aggregates
	}
	stamped := make([]Aggregate, len(aggregates))
	copy(stamped, aggregates)
	for i := range stamped {
		stamped[i].textCollation = true
	}
	return stamped
}

// withTextCollation returns a query copy that carries the execution-time
// collation to aggregate evaluation.
func withTextCollation(query Query, locale string) Query {
	query.textLocale = locale
	if !englishTextLocale(locale) {
		return query
	}
	query.textCollation = true
	query.Aggregates = collateAggregates(query.Aggregates)
	query.HavingAggregates = collateAggregates(query.HavingAggregates)
	query.orderAggregates = collateAggregates(query.orderAggregates)
	return query
}

// textGroupKey merges text values that differ only in ASCII case.
func textGroupKey(text string) string {
	for i := 0; i < len(text); i++ {
		if c := text[i]; c >= 'A' && c <= 'Z' {
			var b strings.Builder
			b.Grow(len(text))
			for j := 0; j < len(text); j++ {
				c := text[j]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				b.WriteByte(c)
			}
			return b.String()
		}
	}
	return text
}
