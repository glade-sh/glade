package vm

// A concrete sObject action parameter represents the input record itself.
// Request-wrapper inputs instead map parameter names onto wrapper fields.
func (vm *VM) invocableRecordInput(elementType, parameterName, inputName string, value Value) (Value, bool) {
	if inputName != parameterName || value.Kind != ValueObject || value.classInstance {
		return Null, false
	}
	if _, class := vm.lookupClass(elementType); class {
		return Null, false
	}
	declared, ok := vm.resolveObjectName(elementType)
	if !ok {
		return Null, false
	}
	actual, ok := vm.resolveObjectName(value.Type)
	if !ok || actual != declared {
		return Null, false
	}
	return value, true
}
