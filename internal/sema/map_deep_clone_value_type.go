package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// semaMapDeepCloneValueTypeDiagnostic is the compile refusal of Map.deepClone
// on a map whose declared value type is not an SObject: review_round11
// dc_typed_self, dc_typed_put, dc_typed_remove, dc_typed_clear,
// dc_chained_self, dc_chained_put, dc_chained_remove, dc_chained_clear,
// dc_int, dc_object, dc_list_value, dc_nested_map,
// dc_nested_sobject_list_map and dc_fieldmap refuse at compile with the
// captured message at APIs 62.0 and 67.0, while dc_sobject and dc_account
// compile and run. Only a receiver with two declared type arguments is
// statically known; a raw Map or an erased receiver keeps the run-time
// refusal (vm.mapDeepCloneValueTypeError). The rows cover zero-argument
// calls only, so other arities keep the collection signature diagnostic.
func semaMapDeepCloneValueTypeDiagnostic(typ typesys.TypeSymbol, model *semaTypeMemberView, receiverType, method string, argc, start, end int, source string) (diagnostic.Diagnostic, bool) {
	if argc != 0 || !strings.EqualFold(strings.TrimSpace(method), "deepClone") {
		return diagnostic.Diagnostic{}, false
	}
	base, args := semaGenericBaseAndArgs(receiverType)
	if normalizeName(base) != "map" || len(args) != 2 || strings.TrimSpace(args[1]) == "" {
		return diagnostic.Diagnostic{}, false
	}
	if semaMapDeepCloneValueSObjectLike(args[1], model) {
		return diagnostic.Diagnostic{}, false
	}
	// The captured messages carry the canonical no-space map type, as the
	// run-time refusal renders it.
	mapType := strings.ReplaceAll(normalizeArrayType(semaStripTypeModifiers(receiverType)), " ", "")
	message := "Operation only applies to SObject value map types: " + mapType
	return diagnostic.Diagnostic{
		Severity:      diagnostic.Error,
		Code:          "GLADESEMA023",
		Message:       message,
		NativeMessage: message,
		File:          typ.File,
		Range:         semaRange(source, start, end),
	}, true
}

// semaMapDeepCloneValueSObjectLike reports whether a map value type passes the
// SObject-value gate. It extends isSemaSObjectLike with the name shapes the
// run-time gate (vm.isSObjectLikeType) accepts and no captured row refuses —
// big objects, shares and change events — so the compile refusal never
// rejects a value the run time would allow.
func semaMapDeepCloneValueSObjectLike(valueType string, model *semaTypeMemberView) bool {
	if isSemaSObjectLike(valueType, model) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(valueType))
	return strings.HasSuffix(name, "__b") || strings.HasSuffix(name, "__share") || strings.HasSuffix(name, "changeevent")
}
