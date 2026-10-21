package vm

import "strings"

// The the captured rows null Set<UserClass> arguments cannot widen to the
// captured Set<String>/Set<Object>/Set<Id> overload triplet. Keep general
// collection assignment and non-null runtime conversion outside this rule.
func (vm *VM) capturedUserClassSetOverloadRejection(candidates []Method, args []Value) bool {
	if len(candidates) != 3 || len(args) != 1 || args[0].Kind != ValueNull {
		return false
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.IsConstructor || len(candidate.Params) != 1 {
			return false
		}
		param := vm.resolveTypeNameInClass(vm.methodTypeResolutionClass(candidate), candidate.Params[0].Type)
		element, ok := collectionElementType(param)
		if collectionBase(param) != "Set" || !ok {
			return false
		}
		seen[strings.ToLower(canonicalRuntimePlatformType(element))] = true
	}
	if len(seen) != 3 || !seen["string"] || !seen["object"] || !seen["id"] {
		return false
	}
	for _, scalar := range []string{"String", "Object", "Id"} {
		if class, known := vm.lookupClass(scalar); known && !class.PlatformType {
			return false
		}
	}
	view := args[0].Static
	if view == "" {
		view = args[0].Type
	}
	view = vm.resolveTypeNameInClass(vm.currentClass, view)
	element, ok := collectionElementType(view)
	if collectionBase(view) != "Set" || !ok {
		return false
	}
	class, known := vm.lookupClass(element)
	return known && !class.PlatformType && !class.IsInterface && len(class.EnumValues) == 0 && !vm.isSObjectLikeType(element)
}
