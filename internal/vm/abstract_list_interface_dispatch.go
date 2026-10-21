package vm

import "strings"

// Native observations are pending for a concrete record-list implementation
// behind an abstract SObject-list interface. Keep the candidate on the actual
// receiver and leave general collection assignment and overload selection alone.
func (vm *VM) abstractRecordListImplementation(receiver Value, surface Method, member string, args []Value) (Method, bool, bool) {
	owner, known := vm.lookupClass(surface.ClassName)
	if !known || !owner.IsInterface || !methodHasModifier(surface.Modifiers, "abstract") {
		return Method{}, false, false
	}
	runtimeType := runtimeObjectType(receiver)
	if !vm.typeMatches(runtimeType, surface.ClassName, make(map[string]bool)) {
		return Method{}, false, false
	}
	seen := make(map[string]bool)
	for runtimeType != "" && !seen[strings.ToLower(runtimeType)] {
		seen[strings.ToLower(runtimeType)] = true
		class, known := vm.lookupClass(runtimeType)
		if !known || class.IsInterface {
			break
		}
		var target Method
		found := false
		for _, candidate := range vm.registeredMethodCandidates(runtimeType + "." + member) {
			if !vm.abstractRecordListParamsMatch(candidate, surface, args) {
				continue
			}
			if found && !vm.sameResolvedMethodParams(target, candidate) {
				return Method{}, false, true
			}
			if !found || vm.preferMethodCandidate(candidate, target) {
				target, found = candidate, true
			}
		}
		if found {
			return target, true, false
		}
		runtimeType = vm.resolvedSuperClassName(class)
	}
	return Method{}, false, false
}

func (vm *VM) abstractRecordListParamsMatch(candidate, surface Method, args []Value) bool {
	if candidate.IsStatic || candidate.IsConstructor || methodHasModifier(candidate.Modifiers, "abstract") || len(candidate.Params) != len(surface.Params) || len(args) != len(surface.Params) {
		return false
	}
	selectedReturn := vm.resolveTypeNameInClass(vm.methodTypeResolutionClass(surface), surface.ReturnType)
	implementationReturn := vm.resolveTypeNameInClass(vm.methodTypeResolutionClass(candidate), candidate.ReturnType)
	if !strings.EqualFold(selectedReturn, "void") || !strings.EqualFold(selectedReturn, implementationReturn) {
		return false
	}
	narrowed := false
	for i := range surface.Params {
		selected := vm.resolveTypeNameInClass(vm.methodTypeResolutionClass(surface), surface.Params[i].Type)
		implementation := vm.resolveTypeNameInClass(vm.methodTypeResolutionClass(candidate), candidate.Params[i].Type)
		if strings.EqualFold(canonicalRuntimePlatformType(selected), canonicalRuntimePlatformType(implementation)) {
			continue
		}
		selectedElement, selectedList := collectionElementType(selected)
		element, implementationList := collectionElementType(implementation)
		if narrowed || collectionBase(selected) != "List" || !selectedList || !strings.EqualFold(selectedElement, "SObject") || collectionBase(implementation) != "List" || !implementationList || strings.EqualFold(element, "SObject") || strings.EqualFold(element, "AggregateResult") || !vm.isSObjectLikeType(element) || args[i].Kind != ValueList {
			return false
		}
		value := args[i]
		for _, declared := range []string{value.Type, value.Static} {
			if declaredElement, typed := collectionElementType(declared); typed && collectionBase(declared) == "List" && !strings.EqualFold(declaredElement, element) && !strings.EqualFold(declaredElement, "SObject") {
				return false
			}
		}
		view := value.Runtime
		if collectionBase(view) != "List" {
			view = value.Type
		}
		viewElement, typed := collectionElementType(view)
		if collectionBase(view) != "List" || !typed || (!strings.EqualFold(viewElement, element) && !strings.EqualFold(viewElement, "SObject")) || (len(value.List) == 0 && !strings.EqualFold(viewElement, element)) {
			return false
		}
		for _, row := range value.List {
			if row.Kind != ValueObject || row.classInstance || !strings.EqualFold(runtimeObjectType(row), element) {
				return false
			}
		}
		// Coercion can retype list entries. Test a detached slice so checking a
		// candidate cannot change the caller's list or another overload's input.
		value.List = append([]Value(nil), value.List...)
		if _, err := vm.coerceAssignable(implementation, value); err != nil {
			return false
		}
		narrowed = true
	}
	return narrowed
}
