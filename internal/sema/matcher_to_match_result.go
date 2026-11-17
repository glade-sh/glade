package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// semaMatcherToMatchResultDiagnostic reports Matcher.toMatchResult(). Apex has
// no MatchResult type, so native rejects the call at compile time at API 62.0
// and 67.0 (regex_java_divergence u6_toMatchResult, anonymous route):
// "Method does not exist or incorrect signature: void toMatchResult() from the
// type System.Matcher". Only the captured zero-argument spelling is claimed.
func semaMatcherToMatchResultDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, platformType, receiverMode, method string, args []ir.Expr, pos int, source string) (diagnostic.Diagnostic, bool) {
	if receiverMode != "instance" || len(args) != 0 || !strings.EqualFold(platformType, "Matcher") || !strings.EqualFold(method, "toMatchResult") {
		return diagnostic.Diagnostic{}, false
	}
	d := unknownCallDiagnostic(typ, member, platformType+"."+method, pos, pos+max(1, len(method)), source)
	d.NativeMessage = "Method does not exist or incorrect signature: void toMatchResult() from the type System.Matcher"
	d.Message = d.NativeMessage
	return d, true
}
