package vm

import "strings"

// LX038 captures a derived String literal hiding a base String literal. Keep
// the already-initialized derived slot without reordering initializer bodies.
// Distinct base-member access and nonliteral hiding need separate captures.
func (vm *VM) preserveSOQLBindShadowLiteral(class Class, object Value, name string, field Field) bool {
	if field.Static || field.Property || !strings.EqualFold(field.Type, "String") || field.InitializerLine == 0 || field.InitialValue.Kind != ValueString {
		return false
	}
	derived, owner, ok := vm.lookupField(runtimeObjectType(object), name)
	if !ok || strings.EqualFold(owner, runtimeClassName(class)) || derived.Static || derived.Property || !strings.EqualFold(derived.Type, "String") || derived.InitializerLine == 0 || derived.InitialValue.Kind != ValueString || derived.StorageName != name {
		return false
	}
	value, present := object.Fields[name]
	return present && value.Kind == ValueString && value.Text == derived.InitialValue.Text
}
