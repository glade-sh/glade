package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// These value-layer rejection forms are identical in the owned Integer/Long,
// Double/Decimal, String and collection API 62 and 67 org rows. Keep the
// editor explanation and attach native text at the shared semantic rejection,
// rather than changing availability or typing.
func valueCollectionCallDiagnostic(d diagnostic.Diagnostic, receiver, method string, args []string, model *semaTypeMemberView) diagnostic.Diagnostic {
	if d.NativeMessage != "" || semaProjectTypeShadowsPlatform(model, receiver) || semaArgTypesContainUnknown(args) {
		return d
	}
	base, _ := semaGenericBaseAndArgs(receiver)
	switch base {
	case "Integer", "Long", "Double", "Decimal", "String", "List", "Set", "Map":
	default:
		return d
	}
	displayArgs := append([]string(nil), args...)
	for i, arg := range displayArgs {
		if strings.EqualFold(arg, "null") {
			displayArgs[i] = "NULL"
		}
	}
	signature := fmt.Sprintf("void %s(%s)", method, strings.Join(displayArgs, ", "))
	d.NativeMessage = "Method does not exist or incorrect signature: " + signature + " from the type " + receiver
	// The Double/Decimal compareTo and null valueOf rows distinguish visibility
	// and null overload ambiguity.
	if receiver == "Decimal" && method == "compareTo" && len(args) == 1 && args[0] == "Decimal" {
		d.NativeMessage = "Method is not visible: Integer Decimal.compareTo(Decimal)"
	}
	if (receiver == "Decimal" || receiver == "Double") && method == "valueOf" && len(args) == 1 && strings.EqualFold(args[0], "null") {
		d.NativeMessage = "Ambiguous method signature: void valueOf(NULL)"
	}
	return d
}

func valueCollectionAssignmentMessage(target, value string, model *semaTypeMemberView) string {
	if semaProjectTypeShadowsPlatform(model, target) || semaProjectTypeShadowsPlatform(model, value) {
		return ""
	}
	switch value + "->" + target {
	case "Long->Integer", "Decimal->Integer", "Decimal->Long", "Integer->Boolean", "Boolean->String",
		"List<Integer>->List<String>", "Set<Integer>->Set<String>", "Map<String,Integer>->Map<String,String>":
		return "Illegal assignment from " + value + " to " + target
	}
	return ""
}

func valueCollectionAssignmentNativeText(message string) bool {
	pair, ok := strings.CutPrefix(message, "Illegal assignment from ")
	if !ok {
		return false
	}
	value, target, ok := strings.Cut(pair, " to ")
	return ok && valueCollectionAssignmentMessage(target, value, nil) != ""
}
