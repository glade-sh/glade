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
	case "Long->Integer", "Decimal->Integer", "Decimal->Long", "Integer->Boolean", "Boolean->String", "String->Integer",
		"List<Integer>->List<String>", "Set<Integer>->Set<String>", "Map<String,Integer>->Map<String,String>",
		// Area M1 S116 and blocker PB07 at API 62 and 67 (S116 on both routes).
		"Schema.SObjectType->Contact", "Schema.SObjectField->Schema.SObjectType":
		return "Illegal assignment from " + value + " to " + target
	}
	// Area M1 I01/I101/I102/I103 at API 62 and 67, both routes: native text
	// qualifies the Iterator type.
	switch strings.TrimPrefix(value, "System.") + "->" + strings.TrimPrefix(target, "System.") {
	case "Iterator<Integer>->Iterator<Object>", "Iterator<String>->Iterator<Integer>",
		"Iterator<Integer>->Iterator<Boolean>", "Iterator<Object>->Iterator<Boolean>",
		"Iterator<Object>->Iterator<Integer>", "Iterator<Integer>->Iterator<String>", "Iterator<SObject>->Iterator<Object>":
		return "Illegal assignment from System." + strings.TrimPrefix(value, "System.") + " to System." + strings.TrimPrefix(target, "System.")
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

// Assignment checking has already established the incompatible types. Keep
// specialized captured wording, then format ordinary rejected assignments.
func assignmentDiagnosticMessage(target, value string, model *semaTypeMemberView) string {
	if message := valueCollectionAssignmentMessage(target, value, model); message != "" {
		return message
	}
	if target == "" || value == "" || strings.EqualFold(value, "null") {
		return ""
	}
	display := func(name string) string {
		if strings.EqualFold(name, "Type") && !semaProjectTypeShadowsPlatform(model, name) {
			return "System.Type"
		}
		return name
	}
	return "Illegal assignment from " + display(value) + " to " + display(target)
}
