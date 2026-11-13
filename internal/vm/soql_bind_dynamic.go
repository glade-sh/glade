package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Reject captured dynamic bind syntax before lexical lookup or bind-map
// substitution can turn it into a value. Uncaptured expression shapes keep
// their existing expansion behavior.
func (vm *VM) validateDynamicSOQLBindSyntax(raw string, countQuery bool) error {
	return vm.validateDynamicSOQLBindSyntaxMode(raw, countQuery, false)
}

// validateDynamicSOQLBindSyntaxMode adds the scope-bind rules of query,
// countQuery and getQueryLocator when scopeBinds is set. queryWithBinds
// resolves a dotted name as a bind-map key, so it leaves them off.
func (vm *VM) validateDynamicSOQLBindSyntaxMode(raw string, countQuery, scopeBinds bool) error {
	previousWord := ""
	orderBy := false
	depth, orderDepth := 0, 0
	for i := 0; i < len(raw); {
		if raw[i] == '\'' {
			i = soqlBindQuotedEnd(raw, i)
			continue
		}
		if i+1 < len(raw) && raw[i] == '/' && raw[i+1] == '*' {
			// Inline SOQL has its own comment rules. A dynamic query comment is
			// a syntax error, as captured on all four source APIs and routes.
			return dynamicSOQLUnexpectedToken("/")
		}
		if isIdentStart(raw[i]) {
			start := i
			for i < len(raw) && isIdentPart(raw[i]) {
				i++
			}
			word := strings.ToUpper(raw[start:i])
			if word == "BY" && previousWord == "ORDER" {
				orderBy = true
				orderDepth = depth
			} else if word == "FROM" || word == "WHERE" || word == "GROUP" || word == "HAVING" || word == "LIMIT" || word == "OFFSET" || word == "FOR" || word == "WITH" || word == "ALL" || word == "USING" {
				orderBy = false
			}
			previousWord = word
			continue
		}
		if raw[i] != ':' || isSOQLDateLiteralBind(raw, i) {
			if raw[i] == '(' {
				depth++
			} else if raw[i] == ')' {
				if depth > 0 {
					depth--
				}
				if orderBy && orderDepth > depth {
					orderBy = false
				}
			}
			i++
			continue
		}
		if orderBy {
			return dynamicSOQLUnexpectedToken(":")
		}
		i = skipDynamicSOQLWhitespace(raw, i+1)
		if i < len(raw) && raw[i] == '(' {
			// Named countQuery first changed at API 67 on the same release;
			// anonymous execution and API 59/62 retain the token diagnostic.
			if countQuery && vm.currentClass != "" && apexversion.AtLeast(vm.currentMethod.APIVersion, 67) {
				return newExceptionError("QueryException", "Only variable references are allowed in dynamic SOQL/SOSL.")
			}
			return dynamicSOQLUnexpectedToken("(")
		}
		start := i
		for i < len(raw) && isIdentPart(raw[i]) {
			i++
		}
		if strings.EqualFold(raw[start:i], "select") {
			return dynamicSOQLUnexpectedToken(raw[start:i])
		}
		member := ""
		path := raw[start:i]
		dotted, called := false, false
		for {
			i = skipDynamicSOQLWhitespace(raw, i)
			if i >= len(raw) {
				break
			}
			if raw[i] == '[' {
				return dynamicSOQLUnexpectedToken(raw[i : i+1])
			}
			if raw[i] == '(' {
				// BP113/136/159 capture only the empty toUpperCase() call.
				// In particular, retain the existing Map.values() path.
				close := skipDynamicSOQLWhitespace(raw, i+1)
				if scopeBinds && close < len(raw) && raw[close] != ')' {
					// A call that takes an argument, such as Map.get('k').
					return dynamicSOQLUnexpectedToken("(")
				}
				called = true
				if strings.EqualFold(member, "toUpperCase") && close < len(raw) && raw[close] == ')' {
					return dynamicSOQLUnexpectedToken("(")
				}
				break
			}
			if raw[i] != '.' {
				break
			}
			i = skipDynamicSOQLWhitespace(raw, i+1)
			if i >= len(raw) || !isIdentStart(raw[i]) {
				break
			}
			start := i
			for i < len(raw) && isIdentPart(raw[i]) {
				i++
			}
			member = raw[start:i]
			path += "." + member
			dotted = true
		}
		if next := skipDynamicSOQLWhitespace(raw, i); scopeBinds && dotted && !called && path != "" && (path[0] < '0' || path[0] > '9') && !(next < len(raw) && strings.ContainsRune("+-*/", rune(raw[next]))) {
			// Dynamic SOQL resolves a bind by its whole spelled name: a dotted
			// path never names a variable, declared or not. A leading digit is
			// not a bind at all: it is the colon of a datetime literal and falls
			// through to the query grammar. An arithmetic
			// continuation has no captured row and keeps its prior handling.
			return newExceptionError("QueryException", "Variable does not exist: "+path)
		}
		if i < len(raw) && raw[i] == '?' {
			return dynamicSOQLUnexpectedToken("?")
		}
	}
	return nil
}

func dynamicSOQLUnexpectedToken(token string) error {
	return newExceptionError("QueryException", "unexpected token: '"+token+"'")
}

func skipDynamicSOQLWhitespace(raw string, i int) int {
	for i < len(raw) && strings.ContainsRune(" \t\n\r", rune(raw[i])) {
		i++
	}
	return i
}

// Preserve both SOQL backslash escapes and the doubled-quote spelling already
// accepted by the query lexer. Colons inside either form are literal text.
func soqlBindQuotedEnd(raw string, start int) int {
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			i++
			continue
		}
		if raw[i] == '\'' {
			if i+1 < len(raw) && raw[i+1] == '\'' {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(raw)
}
