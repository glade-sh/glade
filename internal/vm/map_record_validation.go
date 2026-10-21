package vm

import "strings"

// Record-valued Map reads and putAll validate the actual record, even when
// construction accepted a List with a different declared element view.
func (vm *VM) mapRecordValueMismatch(mapType string, value Value) (string, bool) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok || !strings.EqualFold(keyType, "Id") || value.Kind == ValueNull {
		return "", false
	}
	resolved := vm.resolveTypeNameInCurrentExecutionContext(valueType)
	if _, class := vm.lookupClass(resolved); class {
		return "", false
	}
	target, known := vm.resolveObjectName(resolved)
	if !known {
		return "", false
	}
	if value.Kind != ValueObject {
		return "", false
	}
	if value.classInstance {
		return target, true
	}
	actual, known := vm.resolveObjectName(runtimeObjectType(value))
	if !known {
		return "", false
	}
	return target, !strings.EqualFold(actual, target)
}
