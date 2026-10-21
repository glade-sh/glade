package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/dml"
)

// Anonymous Formula contexts are transient declarations, not named global Apex
// classes. Use request provenance rather than a class-name convention.
func (vm *VM) formulaApexTypeContextValidation(contextType Value) string {
	typeName := formulaContextTypeName(contextType)
	class, ok := vm.lookupClass(typeName)
	// String is the captured standard-library rejection. Other platform
	// context types keep their existing behavior until separately observed.
	if strings.EqualFold(typeName, "String") && (!ok || class.PlatformType) {
		return "Unsupported standard library type: String"
	}
	if ok && !class.PlatformType {
		if methodHasModifier(class.Modifiers, AnonymousClassModifier) {
			return "Unsupported standard library type: " + typeName
		}
		if !strings.EqualFold(class.Access, "global") {
			return "context type must be global"
		}
	}
	return ""
}

func formulaApexTypeFieldVisible(class Class, field Field) bool {
	return class.PlatformType || !strings.EqualFold(class.Access, "global") || strings.EqualFold(field.Access, "global")
}

// Formula's member visibility is independent of the caller's Apex scope. The
// captured public member of a global context is absent from its formula fields.
func (vm *VM) formulaApexTypeMemberValidation(typeName, formula string, template bool) string {
	class, ok := vm.lookupClass(typeName)
	if !ok || class.PlatformType || !strings.EqualFold(class.Access, "global") {
		return ""
	}
	for _, referenced := range dml.FormulaReferencedFields(formula, template) {
		root := strings.SplitN(referenced, ".", 2)[0]
		if field, _, found := vm.lookupReceiverField(typeName, root); found && !field.Static && !formulaApexTypeFieldVisible(class, field) {
			return "Field " + root + " does not exist. Check spelling."
		}
	}
	return ""
}

// The captured global property computes an SObject from inherited state. Its
// initialized backing slot is not the getter's value. Automatic properties and
// plain fields keep their existing stored-value path.
func (vm *VM) formulaApexTypeGetter(receiver Value, name string) (Value, bool, error) {
	if vm.isSObjectLikeType(receiver.Type) {
		return Null, false, nil
	}
	class, ok := vm.lookupClass(receiver.Type)
	if !ok || class.PlatformType || !strings.EqualFold(class.Access, "global") {
		return Null, false, nil
	}
	field, owner, ok := vm.lookupReceiverField(receiver.Type, name)
	if !ok || field.Static || !field.Property || field.Getter == nil || !strings.EqualFold(field.Access, "global") || !vm.isSObjectLikeType(vm.resolveTypeNameInClass(owner, field.Type)) {
		return Null, false, nil
	}
	value, err := vm.callGetter(owner, field, receiver)
	return value, true, err
}
