package vm

import "strings"

// coerceMethodReturn converts a returned value to the method's declared type.
// Native rows at API 62/67/68, both routes: a method declared as one concrete
// record list may return a List<SObject> view of another concrete record list,
// empty or not, whether a StubProvider or plain code produced it, and in both
// custom object and custom metadata directions. The returned list keeps its
// runtime type. Assignment, explicit casts and arguments keep their paths.
func (vm *VM) coerceMethodReturn(typeName string, value Value) (Value, error) {
	// Returning through a typed interface view preserves a user iterator's
	// concrete identity even when its implemented element argument differs.
	if collectionBase(value.Static) == "Iterator" && vm.iteratorObjectImplementationCast(typeName, value) {
		return value, nil
	}
	if narrowed, ok := vm.returnListNarrowing(typeName, value); ok {
		return narrowed, nil
	}
	return vm.coerceAssignable(typeName, value)
}

func (vm *VM) returnListNarrowing(typeName string, value Value) (Value, bool) {
	if value.Kind != ValueList {
		return Null, false
	}
	target := vm.resolveAssignableTargetType(typeName)
	targetElement, ok := collectionElementType(target)
	if !ok || collectionBase(target) != "List" || !vm.concreteRecordElement(targetElement) {
		return Null, false
	}
	source := value.Runtime
	if collectionBase(source) != "List" {
		source = value.Type
	}
	sourceElement, ok := collectionElementType(source)
	if !ok || collectionBase(source) != "List" || !vm.concreteRecordElement(sourceElement) ||
		vm.typeAssignableTo(sourceElement, targetElement) {
		return Null, false
	}
	// Query results keep their separately captured conversion path.
	if inlineSOQLQueryText(value) != "" {
		return Null, false
	}
	for _, row := range value.List {
		if _, queried := row.Fields[sobjectQueriedFieldsField]; queried {
			return Null, false
		}
	}
	value.Runtime = source
	value.Type = target
	value.Static = target
	return value, true
}

func (vm *VM) concreteRecordElement(typeName string) bool {
	return !strings.EqualFold(typeName, "SObject") && !strings.EqualFold(typeName, "Object") &&
		!strings.EqualFold(typeName, "AggregateResult") && vm.isSObjectLikeType(typeName)
}
