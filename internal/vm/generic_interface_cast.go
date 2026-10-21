package vm

import (
	"fmt"
	"strings"
)

// These class-instance casts use the implemented interface's identity for
// specific argument pairs. Assignment and collection casts use their own rules.
func (vm *VM) genericInterfaceCastParts(typeName string) (string, string, bool) {
	typeName = vm.resolveAssignableTargetType(typeName)
	args, ok := genericTypeArgs(typeName)
	open := strings.IndexByte(typeName, '<')
	if !ok || len(args) != 1 || open < 0 {
		return "", "", false
	}
	base := strings.TrimSpace(typeName[:open])
	if class, found := vm.lookupClass(base); found && !class.PlatformType {
		return "", "", false
	}
	if rest, qualified := stripLeadingSystemNamespace(base); qualified {
		base = rest
	}
	switch strings.ToLower(base) {
	case "comparator", "database.batchable", "iterable":
		return strings.ToLower(base), canonicalRuntimePlatformType(args[0]), true
	}
	return "", "", false
}

func genericInterfaceCapturedCastPair(base, from, to string) bool {
	pair := strings.ToLower(from + ":" + to)
	switch base {
	case "comparator":
		return pair == "string:object" || pair == "string:integer" || pair == "object:string"
	case "database.batchable":
		return pair == "sobject:object" || pair == "sobject:integer" || pair == "object:sobject" || pair == "account:object"
	case "iterable":
		// Native captures at API 62.0, 67.0 and 68.0: a user class
		// implementing Iterable<Object> or Iterable<Account> casts to Iterable of
		// SObject, String, Integer, Contact, Account and Object alike.
		return true
	}
	return false
}

func (vm *VM) genericInterfaceImplementationCast(typeName string, value Value) bool {
	if value.Kind != ValueObject || !value.classInstance {
		return false
	}
	base, to, ok := vm.genericInterfaceCastParts(typeName)
	if !ok {
		return false
	}
	class, ok := vm.lookupClass(runtimeObjectType(value))
	if !ok || class.PlatformType {
		return false
	}
	for _, iface := range vm.resolvedInterfaceNames(class) {
		fromBase, from, known := vm.genericInterfaceCastParts(iface)
		if known && base == fromBase && (genericInterfaceCapturedCastPair(base, from, to) ||
			base == "comparator" && vm.comparatorClassCastPair(class, from, to)) {
			return true
		}
	}
	return false
}

// String keeps its existing boundary; native also accepts the captured
// concrete Datetime/Object view at APIs 61..68.
func (vm *VM) genericInterfaceComparatorInstanceOf(value Value, target string) bool {
	base, arg, ok := vm.genericInterfaceCastParts(target)
	if !ok || base != "comparator" || !strings.EqualFold(arg, "Object") {
		return false
	}
	if vm.genericInterfaceStringComparator(value) {
		return true
	}
	class, found := vm.genericInterfaceComparatorPrimitiveClass(value, "Datetime")
	return found && vm.comparatorClassCastPair(class, "Datetime", "Object")
}

func (vm *VM) genericInterfaceStringComparator(value Value) bool {
	_, found := vm.genericInterfaceComparatorClass(value, "String")
	return found
}

func (vm *VM) genericInterfaceComparatorClass(value Value, argument string) (Class, bool) {
	if value.Kind != ValueObject || !value.classInstance {
		return Class{}, false
	}
	class, ok := vm.lookupClass(runtimeObjectType(value))
	if !ok || class.PlatformType {
		return Class{}, false
	}
	for _, iface := range vm.resolvedInterfaceNames(class) {
		base, arg, known := vm.genericInterfaceCastParts(iface)
		if known && base == "comparator" && strings.EqualFold(arg, argument) {
			return class, true
		}
	}
	return Class{}, false
}

func (vm *VM) genericInterfaceComparatorPrimitiveClass(value Value, argument string) (Class, bool) {
	class, found := vm.genericInterfaceComparatorClass(value, argument)
	if found {
		if subject, known := vm.lookupClass(vm.resolveTypeNameInClass(class.Name, argument)); known && !subject.PlatformType {
			return Class{}, false
		}
	}
	return class, found
}

// rows bind Datetime's declared compare signature through either captured
// interface view; null arguments must not select a different overload.
func (vm *VM) genericInterfaceComparatorBoundMethod(view string, receiver Value, method string, args []Value) (Method, bool) {
	base, argument, known := vm.genericInterfaceCastParts(view)
	if !known || base != "comparator" || !strings.EqualFold(method, "compare") || len(args) != 2 ||
		(!strings.EqualFold(argument, "Object") && !strings.EqualFold(argument, "Datetime")) {
		return Method{}, false
	}
	class, found := vm.genericInterfaceComparatorPrimitiveClass(receiver, "Datetime")
	if !found || !vm.comparatorClassCastPair(class, "Datetime", "Object") {
		return Method{}, false
	}
	surface := Method{ClassName: class.Name, Params: []Param{{Type: "Datetime"}, {Type: "Datetime"}}}
	for _, candidate := range vm.registeredMethodCandidates(class.Name + "." + method) {
		if !candidate.IsStatic && !methodHasModifier(candidate.Modifiers, "abstract") && vm.sameResolvedMethodParams(candidate, surface) {
			return candidate, true
		}
	}
	return Method{}, false
}

// Failed Iterable casts name the raw interface, as do the captured
// Object-argument casts of plain user instances to other interfaces.
func (vm *VM) genericInterfaceCastErrorTarget(typeName string, value Value) string {
	base, arg, ok := vm.genericInterfaceCastParts(typeName)
	if !ok {
		return ""
	}
	if base == "iterable" {
		return "System.Iterable"
	}
	if value.Kind != ValueObject || !value.classInstance {
		return ""
	}
	class, known := vm.lookupClass(runtimeObjectType(value))
	if !known || class.PlatformType || len(class.Interfaces) != 0 || vm.resolvedSuperClassName(class) != "" {
		return ""
	}
	if !strings.EqualFold(arg, "Object") {
		return ""
	}
	switch base {
	case "comparator":
		return "System.Comparator"
	case "database.batchable":
		return "Database.Batchable"
	}
	return ""
}

// Captured Object views retain concrete parameter rejection even when ordinary
// coercion or a different overload could accept the supplied argument types.
func (vm *VM) genericInterfaceComparatorArgumentError(receiverName string, receiver Value, method string, args []Value) error {
	if !strings.EqualFold(method, "compare") || len(args) != 2 {
		return nil
	}
	view := vm.declaredReceiverType(receiverName)
	if view == "" {
		view = receiver.Static
	}
	base, arg, ok := vm.genericInterfaceCastParts(view)
	if !ok || base != "comparator" || !strings.EqualFold(arg, "Object") {
		return nil
	}
	from := runtimeValueTypeName(args[0])
	if !strings.EqualFold(from, runtimeValueTypeName(args[1])) {
		return nil
	}
	if strings.EqualFold(from, "Integer") && vm.genericInterfaceStringComparator(receiver) {
		return newExceptionError("System.TypeException", "Invalid conversion from runtime type Integer to String")
	}
	for _, target := range []string{"Datetime", "Long"} {
		captured := target == "Datetime" && (strings.EqualFold(from, "Integer") || strings.EqualFold(from, "String")) ||
			target == "Long" && strings.EqualFold(from, "Decimal")
		class, found := vm.genericInterfaceComparatorPrimitiveClass(receiver, target)
		if !captured || !found || !vm.comparatorClassCastPair(class, target, "Object") {
			continue
		}
		for _, concrete := range class.Methods {
			if strings.EqualFold(apexMethodMemberName(concrete.Name), "compare") && len(concrete.Params) == 2 &&
				strings.EqualFold(canonicalRuntimePlatformType(concrete.Params[0].Type), target) &&
				strings.EqualFold(canonicalRuntimePlatformType(concrete.Params[1].Type), target) {
				return newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", from, target))
			}
		}
	}
	return nil
}
