package vm

import "strings"

// An absent custom field on a new metadata-backed Number record
// is undeclared, not a synthetic null column. Keep the existing unknown-field
// error adapter; only disable its permissive schema fallback for this shape.
func (vm *VM) scaleZeroNumberGetRequiresDeclaredField(receiver Value, fieldArg Value) bool {
	if vm == nil || vm.Org == nil || receiver.classInstance || isTriggerSObject(receiver) ||
		fieldArg.Kind != ValueString || !hasSuffixFold(strings.TrimSpace(fieldArg.Text), "__c") ||
		sObjectIDFromFields(receiver.Fields) != "" {
		return false
	}
	if _, queried := receiver.Fields[sobjectQueriedFieldsField]; queried {
		return false
	}
	objectName, known := vm.resolveObjectName(receiver.Type)
	if !known {
		return false
	}
	definition := vm.describePreparedDefinition(objectName, vm.Org.Objects[objectName].Definition)
	for _, field := range definition.Fields {
		if scaleZeroNumberCapturedField(definition, field) {
			return true
		}
	}
	return false
}
