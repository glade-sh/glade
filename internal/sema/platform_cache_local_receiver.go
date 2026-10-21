package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// A local Object or Integer parameter named Cache makes Session a value-path
// field reference. Other bindings, calls and namespaced scopes stay unchanged.
func semaCapturedCacheLocalReceiverDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, start int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if expr.Left != nil || !strings.EqualFold(expr.Callee, "Cache.Session.isAvailable") ||
		len(expr.Args) != 0 || len(expr.NamedArgs) != 0 || typ.Namespace != "" {
		return diagnostic.Diagnostic{}, false
	}
	switch typ.EffectiveAPIVersion {
	case "62.0", "67.0", "68.0":
	default:
		return diagnostic.Diagnostic{}, false
	}
	binding, bound := scope.binding("Cache")
	if !bound || semaProjectTypeShadowsPlatform(model, binding.typ) {
		return diagnostic.Diagnostic{}, false
	}
	localObject := binding.origin == irSemaOriginLocal && strings.EqualFold(binding.typ, "Object")
	integerParameter := binding.origin == irSemaOriginParam && strings.EqualFold(binding.typ, "Integer")
	if !localObject && !integerParameter {
		return diagnostic.Diagnostic{}, false
	}
	if _, platform := semaPlatformCacheTypeName("Cache.Session", model); !platform {
		return diagnostic.Diagnostic{}, false
	}
	receiver := "Cache.Session"
	if at, _, found := semaIRNameSpan(source, start, receiver); found {
		start = at
	}
	// Match the ordinary unknown-field diagnostic category and Glade wording.
	// Native text stays in the conformance fixture, not in NativeMessage.
	message := fmt.Sprintf("%s %q references unknown field %q on %s", member.Kind, member.Name, "Session", binding.typ)
	return semaPlatformCacheDiagnostic(typ, "GLADESEMA021", message, start, start+len(receiver), source), true
}
