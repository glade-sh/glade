//go:build cgo

package apexast

import (
	"strings"
	"testing"
)

func case323OldNormalizeApexSource(source string) string {
	return normalizeInlineQueryOffsets(case323OldNormalizeInlineSOQLFormula(case323OldNormalizeInlineSOSL(case323OldNormalizeExplicitConstructorInvocations(case323OldNormalizePackageQualifiedReferences(case323OldNormalizeTriggerContextReferences(case323OldNormalizeVoidIdentifiers(source)))))))
}

// The upstream tree grammar rejects SOSL forms that Apex accepts before runtime
// search validation. Normalize only those forms and preserve every byte offset;
// declarations, expressions and diagnostics continue reading the original text.

func case323OldNormalizeInlineSOSL(source string) string {
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
			if !hasWordAtFold(source, start, "FIND") {
				i++
				continue
			}
			end := inlineSOSLEnd(source, start)
			if end < 0 {
				i++
				continue
			}
			case323OldNormalizeInlineSOSLQuery(source, out, start, end)
			i = end + 1
		default:
			i++
		}
	}
	return string(out)
}

func case323OldNormalizeInlineSOSLQuery(source string, out []byte, start, end int) {
	term := nextSignificantIndex(source, start+len("FIND"))
	if term+1 < end && source[term:term+2] == "''" && skipApexString(source, term) == term+2 {
		copy(out[term:term+2], ":x") // C003: preserve the original empty term.
	}
	returning := false
	depth := 0
	for i := start + len("FIND"); i < end; {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case source[i] == '(':
			depth++
			i++
		case source[i] == ')':
			depth--
			i++
		case hasWordAtFold(source, i, "RETURNING") && depth == 0:
			returning = true
			i += len("RETURNING")
		case !returning && depth == 0 && hasWordAtFold(source, i, "IN"):
			scope := nextSignificantIndex(source, i+len("IN"))
			scopeEnd := scope
			for scopeEnd < end && isIdentifierByte(source[scopeEnd]) {
				scopeEnd++
			}
			fields := nextSignificantIndex(source, scopeEnd)
			if scopeEnd > scope && (source[scope] < '0' || source[scope] > '9') && hasWordAtFold(source, fields, "FIELDS") {
				switch strings.ToUpper(source[scope:scopeEnd]) {
				case "ALL", "NAME", "EMAIL", "PHONE", "SIDEBAR":
				default:
					case323OldBlankInlineSOSL(out, i, fields+len("FIELDS"))
				}
			}
			i += len("IN")
		case depth == 0 && hasWordAtFold(source, i, "WITH"):
			mode := nextSignificantIndex(source, i+len("WITH"))
			for _, word := range []string{"USER_MODE", "SYSTEM_MODE"} {
				if hasWordAtFold(source, mode, word) {
					case323OldBlankInlineSOSL(out, i, mode+len(word))
					break
				}
			}
			i += len("WITH")
		case returning && depth > 0 && hasWordAtFold(source, i, "OFFSET"):
			value := nextSignificantIndex(source, i+len("OFFSET"))
			if value+1 < end && source[value] == '-' && source[value+1] >= '0' && source[value+1] <= '9' {
				out[value] = ' '
			}
			i += len("OFFSET")
		default:
			i++
		}
	}
}

func case323OldBlankInlineSOSL(out []byte, start, end int) {
	for i := start; i < end && i < len(out); i++ {
		if out[i] != '\n' && out[i] != '\r' {
			out[i] = ' '
		}
	}
}

func case323OldNormalizePackageQualifiedReferences(source string) string {
	// The upstream grammar reserves Package for its version-expression rule, but
	// Package.Version is also a legal qualified type reference in Apex source.
	// Keep the source byte length unchanged so parser ranges still map directly
	// to the original source.
	out := []byte(source)
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case hasWordAtFold(source, i, "Package") && nextSignificantByte(source, i+len("Package")) == '.':
			copy(out[i:i+len(packageQualifiedTypeSentinel)], packageQualifiedTypeSentinel)
			i += len("Package")
		default:
			i++
		}
	}
	return string(out)
}

func case323OldNormalizeVoidIdentifiers(source string) string {
	// Apex allows methods named "void". The upstream grammar treats a bare
	// void(...) call as the void type token, so rewrite only that identifier
	// form with a same-length sentinel to preserve byte offsets.
	var out strings.Builder
	out.Grow(len(source))

	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			next := copyApexString(&out, source, i)
			i = next
		case hasPrefixAt(source, i, "//"):
			next := copyUntilNewline(&out, source, i)
			i = next
		case hasPrefixAt(source, i, "/*"):
			next := copyBlockComment(&out, source, i)
			i = next
		case hasWordAt(source, i, "void") && nextSignificantByte(source, i+len("void")) == '(':
			out.WriteString(voidIdentifierSentinel)
			i += len("void")
		default:
			out.WriteByte(source[i])
			i++
		}
	}
	return out.String()
}

func case323OldNormalizeTriggerContextReferences(source string) string {
	// The upstream grammar reserves trigger for trigger declarations. Apex also
	// exposes Trigger.* context variables inside bodies, which are not part of
	// this package's declaration API.
	out := []byte(source)
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case hasWordAtFold(source, i, "trigger") && nextSignificantByte(source, i+len("trigger")) == '.':
			copy(out[i:i+len(triggerContextSentinel)], triggerContextSentinel)
			i += len(triggerContextSentinel)
		default:
			i++
		}
	}
	return string(out)
}

func case323OldNormalizeExplicitConstructorInvocations(source string) string {
	// The upstream grammar treats this(...) and super(...) as Java-style
	// constructor chaining, which rejects Apex code that calls them elsewhere.
	// Body expression details are not part of this package's API, so rewrite
	// the call keyword and any leading cast prefix with same-length text.
	out := []byte(source)
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case hasWordAt(source, i, "this"):
			i = case323OldNormalizeExplicitConstructorInvocation(out, source, i, "thiz")
		case hasWordAt(source, i, "super"):
			i = case323OldNormalizeExplicitConstructorInvocation(out, source, i, "soper")
		default:
			i++
		}
	}
	return string(out)
}

func case323OldNormalizeExplicitConstructorInvocation(out []byte, source string, keywordStart int, replacement string) int {
	openParen := nextSignificantIndex(source, keywordStart+len(replacement))
	if openParen >= len(source) || source[openParen] != '(' {
		return keywordStart + len(replacement)
	}
	copy(out[keywordStart:keywordStart+len(replacement)], replacement)
	argStart := nextSignificantIndex(source, openParen+1)
	if argStart >= len(source) || source[argStart] != '(' {
		return openParen + 1
	}
	castEnd, ok := castTypeEnd(source, argStart)
	if !ok {
		return argStart + 1
	}
	for i := argStart; i <= castEnd; i++ {
		out[i] = ' '
	}
	return castEnd + 1
}

func case323OldNormalizeInlineSOQLFormula(source string) string {
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
			case323OldNormalizeSOQLFormulaPredicates(source, out, start, end)
			i = end + 1
		default:
			i++
		}
	}
	return string(out)
}

func case323OldNormalizeSOQLFormulaPredicates(source string, out []byte, start, end int) {
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
				case323OldBlankInlineSOSL(out, i, callEnd)
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

func TestCase323OldAndNewNormalization(t *testing.T) {
	for _, s := range []string{
		"", "class A {}", "é\xff\n// void() Trigger.new\n'Package.X'", "void(); Trigger.new; Package.Version; this(); super();",
		"[FIND '' IN BAD FIELDS RETURNING Account(Id OFFSET -1) WITH USER_MODE]",
		"[SELECT Id FROM Account WHERE FORMULA('true') = true]",
		"class A { String x = 'void()'; /* Trigger.new */ void f() {} }",
		"super((Integer) 2); this((String) x);", "[SELECT Id FROM Account WHERE Name = 'é']",
	} {
		want, got := case323OldNormalizeApexSource(s), normalizeApexSource(s)
		if got != want {
			t.Fatalf("input %q got %q want %q", s, got, want)
		}
	}
}
