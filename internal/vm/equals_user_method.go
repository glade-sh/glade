package vm

import "strings"

// E23E013-020: only a directly declared instance equals(Object) on a
// source class returning Boolean or another source class participates here.
// Inherited, platform and overloaded surfaces keep the existing resolver
// until their collection behavior is captured.
func (vm *VM) equalsUserObjectDeclaration(value Value) (Method, bool) {
	if value.Kind != ValueObject || isStubProxy(value) {
		return Method{}, false
	}
	typeName := runtimeObjectType(value)
	class, known := vm.lookupClass(typeName)
	if !known || class.PlatformType || class.Dependency || class.IsInterface {
		return Method{}, false
	}
	var target Method
	found := false
	for _, candidate := range vm.registeredMethodCandidates(typeName + ".equals") {
		if candidate.IsStatic || !strings.EqualFold(candidate.ClassName, typeName) ||
			len(candidate.Params) != 1 || !strings.EqualFold(candidate.Params[0].Type, "Object") {
			return Method{}, false
		}
		if !found || vm.preferMethodCandidate(candidate, target) {
			target, found = candidate, true
		}
	}
	if !found || strings.TrimSpace(target.ReturnType) == "" {
		return Method{}, false
	}
	if strings.EqualFold(target.ReturnType, "Boolean") {
		return target, true
	}
	resultType := vm.resolveTypeNameInClass(typeName, target.ReturnType)
	resultClass, known := vm.lookupClass(resultType)
	return target, known && !resultClass.PlatformType && !resultClass.Dependency && !resultClass.IsInterface
}

// A non-Boolean declaration is ignored without being called. The incoming or
// lookup key's Boolean declaration is called in a custom hash bucket.
// Equals-only Boolean keys retain the identity-hash path.
func (vm *VM) equalsUserCollectionDeclaration(lookup, stored Value) (Method, bool) {
	if stored.Kind != ValueObject || !strings.EqualFold(runtimeObjectType(lookup), runtimeObjectType(stored)) {
		return Method{}, false
	}
	method, declared := vm.equalsUserObjectDeclaration(lookup)
	if !declared {
		return Method{}, false
	}
	if !strings.EqualFold(method.ReturnType, "Boolean") {
		return method, true
	}
	hash, found, ambiguous := vm.resolveInstanceMethodForArgs(runtimeObjectType(lookup), "hashCode", nil)
	if !found || ambiguous || hash.IsStatic || !strings.EqualFold(hash.ClassName, method.ClassName) || !strings.EqualFold(hash.ReturnType, "Integer") {
		return Method{}, false
	}
	return method, true
}

func (vm *VM) equalsUserSetKey(stored, lookup Value, result *Result) (bool, error) {
	if method, captured := vm.equalsUserCollectionDeclaration(lookup, stored); captured {
		if !strings.EqualFold(method.ReturnType, "Boolean") {
			return valueIdentityEqual(lookup, stored), nil
		}
		return vm.apexCollectionElementEquals(lookup, stored, result)
	}
	return vm.apexCollectionElementEquals(stored, lookup, result)
}

// E23E020 observes the same instance through Object: a non-Boolean user
// declaration does not replace Object.equals. Distinct-instance and Boolean
// Object-view dispatch are outside this capture and keep their existing path.
func (vm *VM) equalsUserObjectView(receiverName string, receiver Value, method string, args []Value) bool {
	if !strings.EqualFold(method, "equals") || len(args) != 1 || !valueIdentityEqual(receiver, args[0]) {
		return false
	}
	view := vm.declaredReceiverType(receiverName)
	if view == "" {
		view = receiver.Static
	}
	if !strings.EqualFold(view, "Object") {
		return false
	}
	declaration, found := vm.equalsUserObjectDeclaration(receiver)
	return found && !strings.EqualFold(declaration.ReturnType, "Boolean")
}

// E23E019 captures a pair: equals(Object) returns a user class and
// equals(the declaring class) returns String. An Object argument selects
// the first declaration even when its runtime value is the declaring class.
// Keep global conversion scoring, nulls and every other overload unchanged.
func (vm *VM) equalsUserObjectArgumentOverload(candidates []Method, args []Value) (Method, bool) {
	if len(candidates) < 2 || len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Static, "Object") {
		return Method{}, false
	}
	owner := candidates[0].ClassName
	class, known := vm.lookupClass(owner)
	if !known || class.PlatformType || class.Dependency || class.IsInterface || !strings.EqualFold(runtimeObjectType(args[0]), owner) {
		return Method{}, false
	}
	var objectMethod Method
	foundObject, foundTyped := false, false
	for _, candidate := range candidates {
		if candidate.IsStatic || !strings.EqualFold(candidate.ClassName, owner) ||
			!strings.EqualFold(apexMethodMemberName(candidate.Name), "equals") || len(candidate.Params) != 1 {
			return Method{}, false
		}
		parameter := vm.resolveTypeNameInClass(owner, candidate.Params[0].Type)
		switch {
		case strings.EqualFold(parameter, "Object"):
			resultType := vm.resolveTypeNameInClass(owner, candidate.ReturnType)
			resultClass, known := vm.lookupClass(resultType)
			if !known || resultClass.PlatformType || resultClass.Dependency || resultClass.IsInterface {
				return Method{}, false
			}
			if !foundObject || vm.preferMethodCandidate(candidate, objectMethod) {
				objectMethod, foundObject = candidate, true
			}
		case strings.EqualFold(parameter, owner) && strings.EqualFold(candidate.ReturnType, "String"):
			foundTyped = true
		default:
			return Method{}, false
		}
	}
	return objectMethod, foundObject && foundTyped
}
