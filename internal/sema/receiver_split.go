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

var splitReceiverAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9.]*\s+([A-Za-z_][A-Za-z_0-9]*)\s*=\s*$`)
var splitReceiverArgument = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9.]*\s*\(`)

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
	if _, _, repeated := semaIRNameSpan(source, end, expr.Callee); repeated {
		return nil
	}
	before, after, ok := splitReceiverTrivia(source[start:end], expr.Callee)
	if !ok {
		return nil
	}
	var message string
	if strings.ContainsAny(after, "\r\n") {
		prefix := strings.TrimSpace(source[pos:start])
		switch {
		case prefix == "":
			message = "Invalid type: " + receiver + "."
		case splitReceiverAssignment.MatchString(prefix):
			message = "Unexpected token '" + splitReceiverAssignment.FindStringSubmatch(prefix)[1] + "'."
		case splitReceiverArgument.MatchString(prefix):
			message = "Unexpected token '('."
		default:
			return nil
		}
	} else {
		if before == "" || !semaReceiverExprLooksLikeType(receiver, scope, model) {
			return nil
		}
		root, _, _ := strings.Cut(receiver, ".")
		message = "Variable does not exist: " + root
	}
	return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+len(receiver))}
}
