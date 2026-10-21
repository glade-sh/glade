package vm

import "strings"

// PENDING-ROWS: only String property storage is a candidate. The developer
// guide's String description conflicts with the reference object signature.
// Keep object/null coercion and actual DML option handling on their paths.
func (vm *VM) coerceDMLLocaleProperty(receiver Value, field, declaredType string, input Value) (Value, error) {
	if strings.EqualFold(receiver.Type, "Database.DMLOptions") && strings.EqualFold(field, "LocaleOptions") && input.Kind == ValueString {
		definition, registered := vm.lookupClass(receiver.Type)
		if !receiver.classInstance || (registered && definition.PlatformType) {
			return input, nil
		}
	}
	return vm.coerceAssignable(declaredType, input)
}
