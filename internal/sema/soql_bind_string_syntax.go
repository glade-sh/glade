package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var soqlBindSourceStringArgumentPattern = regexp.MustCompile(`(?i)\bDatabase\s*\.\s*(?:query|countQuery|queryWithBinds)\s*\(\s*$`)

// The captured rows reject physical line breaks inside ordinary quoted
// first-argument query strings with a line comment. JSON payload literals,
// escaped text and triple-quoted strings keep their existing behavior.
func singleQuotedStringLineBreakDiagnostics(typ typesys.TypeSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	for i := 0; i < len(body); i++ {
		if end, comment := skipSemaComment(body, i); comment {
			i = end
			continue
		}
		if body[i] != '\'' {
			continue
		}
		if strings.HasPrefix(body[i:], "'''") && i+3 < len(body) && (body[i+3] == '\n' || body[i+3] == '\r') {
			if close := strings.Index(body[i+3:], "'''"); close >= 0 {
				i += 3 + close + 2
				continue
			}
		}
		end := skipSemaString(body, i)
		if syntax := vm.SOQLBindSourceStringLineBreak(body, i, end+1); syntax != nil {
			if call := soqlBindSourceStringArgumentPattern.FindStringIndex(body[:i]); call != nil && newSourceFacts(body).containsCode(call[0]) {
				offset := bodyOffset + syntax.Offset
				return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA_SOURCE_SYNTAX", Message: syntax.Message, NativeMessage: syntax.NativeMessage, File: typ.File, Range: semaRange(source, offset, offset+1)}}
			}
		}
		i = end
	}
	return nil
}
