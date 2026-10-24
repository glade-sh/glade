//go:build cgo

package apexast

import "strings"

// The tree grammar predates FORMULA predicates. Give it a field operand with
// the same byte range; query validation continues to use the original source.
func normalizeInlineSOQLFormula(source string) string {
	out := []byte(source)
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case source[i] == '[':
			start := nextSignificantIndex(source, i+1)
			if !hasWordAtFold(source, start, "SELECT") {
				i++
				continue
			}
			end := inlineSOSLEnd(source, start)
			if end < 0 {
				i++
				continue
			}
			normalizeSOQLFormulaPredicates(source, out, start, end)
			i = end + 1
		default:
			i++
		}
	}
	return string(out)
}

func normalizeSOQLFormulaPredicates(source string, out []byte, start, end int) {
	depth, opaqueDepth := 0, -1
	fromSeen, where, operand, qualified := false, false, false, false
	for i := start + len("SELECT"); i < end; {
		next := nextSignificantIndex(source, i)
		if next >= end {
			return
		}
		i = next
		if source[i] == '\'' {
			i = skipApexString(source, i)
			operand, qualified = false, false
			continue
		}
		if source[i] == '(' {
			if where && !operand && opaqueDepth < 0 {
				opaqueDepth = depth
			}
			depth++
			qualified = false
			i++
			continue
		}
		if source[i] == ')' {
			depth--
			if depth == opaqueDepth {
				opaqueDepth = -1
			}
			qualified = false
			i++
			continue
		}
		if !isIdentifierByte(source[i]) {
			operand = false
			qualified = source[i] == '.' || source[i] == ':'
			i++
			continue
		}
		wordEnd := i + 1
		for wordEnd < end && isIdentifierByte(source[wordEnd]) {
			wordEnd++
		}
		word := strings.ToUpper(source[i:wordEnd])
		switch {
		case depth == 0 && word == "FROM":
			fromSeen = true
		case depth == 0 && fromSeen && word == "WHERE":
			where, operand = true, true
		case depth == 0 && where && (word == "GROUP" || word == "HAVING" || word == "ORDER" || word == "LIMIT" || word == "OFFSET" || word == "WITH" || word == "FOR"):
			where, operand = false, false
		case where && opaqueDepth < 0 && !qualified && (word == "AND" || word == "OR"):
			operand = true
		case where && opaqueDepth < 0 && operand && word == "NOT":
		case where && opaqueDepth < 0 && operand && word == "FORMULA":
			if callEnd := inlineSOQLFormulaCallEnd(source, wordEnd, end); callEnd > wordEnd {
				blankInlineSOSL(out, i, callEnd)
				copy(out[i:callEnd], "Id")
				wordEnd = callEnd
			}
			operand = false
		default:
			operand = false
		}
		qualified = false
		i = wordEnd
	}
}

func inlineSOQLFormulaCallEnd(source string, afterName, end int) int {
	open := nextSignificantIndex(source, afterName)
	if open >= end || source[open] != '(' {
		return -1
	}
	argument := nextSignificantIndex(source, open+1)
	if argument >= end || source[argument] != '\'' {
		return -1
	}
	close := nextSignificantIndex(source, skipApexString(source, argument))
	if close >= end || source[close] != ')' {
		return -1
	}
	return close + 1
}
