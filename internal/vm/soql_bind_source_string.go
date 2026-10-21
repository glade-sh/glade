package vm

import "strings"

func isSOQLBindSourceStringArgument(tokens []token) bool {
	if len(tokens) < 4 {
		return false
	}
	call := tokens[len(tokens)-4:]
	if call[0].kind != tokenIdent || !strings.EqualFold(call[0].text, "Database") || call[1].text != "." || call[2].kind != tokenIdent || call[3].text != "(" {
		return false
	}
	switch strings.ToLower(call[2].text) {
	case "query", "countquery", "querywithbinds":
		return true
	}
	return false
}

// SOQLBindSourceStringLineBreak checks only the ordinary first-argument literal
// of a dynamic query call. The caller establishes that source context. Native
// LX067/071/075 capture a physical newline after a line-comment spelling;
// decoded runtime strings and unrelated payload literals keep their old path.
func SOQLBindSourceStringLineBreak(source string, start, end int) *ApexSyntaxError {
	if start < 0 || end > len(source) || end <= start {
		return nil
	}
	raw := source[start:end]
	newline := strings.IndexAny(raw, "\r\n")
	if newline < 0 || !strings.Contains(raw[:newline], "//") {
		return nil
	}
	const message = "Illegal string literal: Line breaks are not allowed in string literals"
	return &ApexSyntaxError{Message: message, NativeMessage: message, Offset: start + newline}
}
