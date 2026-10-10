package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// Preserve the platform's receiver and argument spelling when
// a Test/Assert overload rejects a call. Project declarations still shadow it.
func semaTestCallDiagnostic(typ typesys.TypeSymbol, receiver, method string, args []string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if semaProjectTypeShadowsPlatform(model, receiver) {
		return diagnostic.Diagnostic{}, false
	}
	name := semaCanonicalPlatformAlias(receiver)
	// Statically typed comparisons reject these pairs;
	// J082-J084 keep Object-typed comparisons on the runtime path.
	if strings.EqualFold(name, "System") && strings.EqualFold(method, "assertEquals") && len(args) == 2 {
		left, right := semaCanonicalPlatformAlias(args[0]), semaCanonicalPlatformAlias(args[1])
		if strings.EqualFold(left, "String") && (strings.EqualFold(right, "Schema.DisplayType") || strings.EqualFold(right, "DisplayType")) {
			return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA023", Message: "Comparison arguments must be compatible types: String, Schema.DisplayType", File: typ.File, Range: semaRange(source, start, end)}, true
		}
		if strings.EqualFold(left, "Date") && strings.EqualFold(right, "String") {
			return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA023", Message: "Comparison arguments must be compatible types: Date, String", File: typ.File, Range: semaRange(source, start, end)}, true
		}
	}
	if !strings.EqualFold(name, "Test") && !strings.EqualFold(name, "Assert") {
		return diagnostic.Diagnostic{}, false
	}
	signature, ok := semaPlatformMethodSignatureFor(model, receiver, method)
	if !ok || semaArgsMatchAny(signature.params, args, model) {
		return diagnostic.Diagnostic{}, false
	}
	display := append([]string(nil), args...)
	for i, arg := range display {
		if strings.EqualFold(arg, "Type") {
			display[i] = "System.Type"
		} else if strings.EqualFold(arg, "null") {
			display[i] = "NULL"
		}
	}
	if strings.EqualFold(name, "Test") {
		name = "System.Test"
	} else {
		name = "System.Assert"
	}
	message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(display, ", "), name)
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA023", Message: message, File: typ.File, Range: semaRange(source, start, end)}, true
}

func semaTestRunAsDiagnostic(typ typesys.TypeSymbol, argType string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if semaRunAsArgumentAllowed(argType, model) {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA023", Message: "runAs requires a single argument of type 'User' or 'Version'", File: typ.File, Range: semaRange(source, start, end)}, true
}

// C019: StubProvider's native callback uses separators between parameters;
// do not change project lifecycle signatures or visibility diagnostics.
func semaTestRequiredMethodMessage(typ typesys.TypeSymbol, requirement methodRequirement, model *semaTypeMemberView) string {
	if !strings.EqualFold(semaCanonicalPlatformAlias(requirement.owner), "StubProvider") || semaProjectTypeShadowsPlatform(model, requirement.owner) {
		return ""
	}
	method := resolvedLifecycleMethod(model, requirement.owner, requirement.member)
	args := make([]string, len(method.Parameters))
	for i, param := range method.Parameters {
		args[i] = param.Type
	}
	return fmt.Sprintf("Class %s must implement the method: %s System.StubProvider.%s(%s)", typ.Name, method.Type, method.Name, strings.Join(args, ", "))
}
