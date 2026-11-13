package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// splitReceiverTrivia inspects the final separator of a flattened call name.
// The IR loses trivia, so source supplies it after the receiver is resolved.
func splitReceiverTrivia(text, callee string) (before, after string, ok bool) {
	parts := strings.Split(callee, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	i := 0
	skip := func() {
		for i < len(text) {
			switch {
			case strings.ContainsRune(" \t\r\n\f", rune(text[i])):
				i++
			case strings.HasPrefix(text[i:], "/*"):
				n := strings.Index(text[i+2:], "*/")
				if n < 0 {
					i = len(text)
					return
				}
				i += n + 4
			case strings.HasPrefix(text[i:], "//"):
				n := strings.IndexByte(text[i:], '\n')
				if n < 0 {
					i = len(text)
					return
				}
				i += n + 1
			default:
				return
			}
		}
	}
	for n, part := range parts {
		if !strings.HasPrefix(text[i:], part) {
			return "", "", false
		}
		i += len(part)
		if n == len(parts)-1 {
			return before, after, true
		}
		start := i
		skip()
		if i >= len(text) || text[i] != '.' {
			return "", "", false
		}
		b := text[start:i]
		i++
		start = i
		skip()
		a := text[start:i]
		if n == len(parts)-2 {
			before, after = b, a
		}
	}
	return "", "", false
}

var splitReceiverAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*\s+([A-Za-z_][A-Za-z_0-9]*)\s*=\s*$`)

// Before-dot captures cover standalone calls and calls in a two-argument
// assertion-shaped expression, including qualified receivers and repeats.
var splitReceiverBeforeShape = func() *regexp.Regexp {
	id := `[A-Za-z_][A-Za-z_0-9]*`
	path := id + `(?:\.` + id + `)*`
	trivia := `(?:\s|/\*[^*]*\*+(?:[^/*][^*]*\*+)*/)*`
	literal := `'(?:[^'\\]|\\.)*'`
	call := path + trivia + `\.\s*` + id + `\s*\(\s*(?:` + literal + `|[0-9]+)?\s*\)`
	args := `(?:` + literal + `\s*,\s*` + call + `|` + call + `\s*,\s*` + call + `)`
	return regexp.MustCompile(`(?s)^\s*(?:` + call + `|` + path + `\s*\(\s*` + args + `\s*\))\s*;`)
}()

// Restrict after-dot handling to the statement, assignment and argument
// shapes observed in the anonymous captures. In particular, a matching
// prefix alone does not establish the rest of the expression's shape.
var splitReceiverAfterShape = func() *regexp.Regexp {
	id := `[A-Za-z_][A-Za-z_0-9]*`
	path := id + `(?:\.` + id + `)*`
	trivia := `(?:\s*|/\*[^*]*\*+(?:[^/*][^*]*\*+)*/\s*)`
	call := id + `\.` + trivia + id + `\s*\(\s*\)`
	literal := `'(?:[^'\\]|\\.)*'`
	args := `(?:` + literal + `\s*,\s*` + call + `|` + call + `\s*,\s*` + call + `|` + literal + `\s*,\s*` + path + `\s*\(\s*` + call + `\s*\))`
	return regexp.MustCompile(`(?s)^\s*(?:` + call + `|` + id + `\s+` + id + `\s*=\s*` + call + `|` + path + `\s*\(\s*` + args + `\s*\))\s*;`)
}()

func semaSplitStaticReceiver(typ typesys.TypeSymbol, expr ir.Expr, scope irSemaScope, pos int, source string, model *semaTypeMemberView) *diagnostic.Diagnostic {
	major, _ := apexversion.Major(typ.EffectiveAPIVersion)
	if !scope.anonymous || expr.Left != nil || (major != 62 && major != 67) {
		return nil
	}
	receiver, _, ok := splitSemaMethodPath(expr.Callee)
	if !ok {
		return nil
	}
	start, end, ok := semaIRNameSpan(source, pos, expr.Callee)
	if !ok {
		return nil
	}
	if splitReceiverBeforeShape.MatchString(source[pos:]) && semaReceiverExprLooksLikeType(receiver, scope, model) {
		for at, last, found := start, end, true; found; at, last, found = semaIRNameSpan(source, last, expr.Callee) {
			before, after, valid := splitReceiverTrivia(source[at:last], expr.Callee)
			if valid && before != "" && !strings.ContainsAny(after, "\r\n") && strings.HasPrefix(strings.TrimSpace(source[last:]), "(") {
				root, _, _ := strings.Cut(receiver, ".")
				message := "Variable does not exist: " + root
				return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_RECEIVER_SPLIT", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, at, at+len(receiver))}
			}
		}
	}
	if strings.Contains(receiver, ".") || !splitReceiverAfterShape.MatchString(source[pos:]) {
		return nil
	}
	for at, last, found := start, end, true; found; at, last, found = semaIRNameSpan(source, last, expr.Callee) {
		before, after, valid := splitReceiverTrivia(source[at:last], expr.Callee)
		if !valid || before != "" || !strings.ContainsAny(after, "\r\n") {
			continue
		}
		prefix := strings.TrimSpace(source[pos:at])
		var message string
		switch {
		case prefix == "":
			message = "Invalid type: " + receiver + "."
		case splitReceiverAssignment.MatchString(prefix):
			message = "Unexpected token '" + splitReceiverAssignment.FindStringSubmatch(prefix)[1] + "'."
		default:
			message = "Unexpected token '('."
		}
		return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_RECEIVER_SPLIT", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, at, at+len(receiver))}
	}
	return nil
}

// Repeated IR calls may identify the same source occurrence. Keep one report
// for that location without merging diagnostics at distinct locations.
func dedupeSplitReceiverDiagnostics(items []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	seen := make(map[diagnostic.Position]map[string]bool)
	out := make([]diagnostic.Diagnostic, 0, len(items))
	for _, d := range items {
		if d.Code == "GLADESEMA_RECEIVER_SPLIT" && d.Range != nil {
			d.Code = "GLADESEMA013"
			pos := d.Range.Start
			if seen[pos] == nil {
				seen[pos] = make(map[string]bool)
			}
			if seen[pos][d.NativeMessage] {
				continue
			}
			seen[pos][d.NativeMessage] = true
		}
		out = append(out, d)
	}
	return out
}
