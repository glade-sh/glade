package vm

import "strings"

// Implicit method and constructor arguments accept a List<SObject> view of an
// unrelated concrete record list. The list keeps its concrete runtime identity;
// local assignments and explicit casts still check that identity.
func (vm *VM) coerceMethodArgument(typeName string, value Value) (Value, error) {
	if element, ok := collectionElementType(value.Type); ok && collectionBase(value.Type) == "List" && strings.EqualFold(element, "SObject") {
		if narrowed, ok := vm.returnListNarrowing(typeName, value); ok {
			return narrowed, nil
		}
	}
	return vm.coerceAssignable(typeName, value)
}
