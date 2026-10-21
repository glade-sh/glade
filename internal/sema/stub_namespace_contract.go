package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// Captured at named APIs 62/67/68: getNamespace() is unavailable on a
// namespaced top-level class literal's System.Type. Do not change variable
// receivers, other reflection methods, interfaces, inner literals or source
// methods named getNamespace. This packet has no anonymous capture.
func semaStubNamespaceLiteralDiagnostic(typ typesys.TypeSymbol, callee string, arity, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	method, literal := semaClassLiteralMethod(callee)
	if !literal || !strings.EqualFold(method, "getNamespace") || arity != 0 || typ.Namespace == "" || !typ.IsTest {
		return diagnostic.Diagnostic{}, false
	}
	cut := strings.Index(strings.ToLower(callee), ".class.")
	typeName := resolveNestedTypeReference(model, typ.Name, callee[:cut])
	owner, _, found := semaLookupTypeMembers(model, typeName)
	if !found || owner.platform || owner.sobject || owner.kind != apexast.DeclarationClass || owner.nestingDepth != 0 || !strings.EqualFold(owner.namespace, typ.Namespace) {
		return diagnostic.Diagnostic{}, false
	}
	message := "Method does not exist or incorrect signature: void getNamespace() from the type System.Type"
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA009", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, end)}, true
}
