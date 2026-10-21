package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Captured sibling constructors reject a write after base initialization.
// Like final-local rebinding, the raw error escapes Apex catch(Exception).
func (vm *VM) inheritedFinalConstructorWrite(object Value, field Field, owner string) error {
	if !vm.currentMethod.IsConstructor ||
		field.Property || field.Static || !methodHasModifier(field.Modifiers, "final") ||
		(!strings.EqualFold(field.Access, "private") && !strings.EqualFold(field.Access, "protected")) {
		return nil
	}
	caller, ok := vm.lookupClass(vm.currentClass)
	base, baseOK := vm.lookupClass(owner)
	if !ok || !baseOK || caller.Dependency || base.Dependency ||
		caller.Namespace != "" || base.Namespace != "" || caller.IsAbstract ||
		methodHasModifier(caller.Modifiers, "virtual") || !strings.EqualFold(object.Type, runtimeClassName(caller)) ||
		!apexversion.AtLeast(vm.currentMethod.APIVersion, 62) || !methodHasModifier(base.Modifiers, "virtual") ||
		methodHasModifier(caller.Modifiers, AnonymousClassModifier) ||
		!sameLexicalTopLevel(caller.Name, base.Name) ||
		!strings.EqualFold(vm.resolvedSuperClassName(caller), runtimeClassName(base)) {
		return nil
	}
	this, thisOK := vm.Globals["this"]
	if !thisOK || object.Kind != ValueObject || object.Ref == 0 || object.Ref != this.Ref {
		return nil
	}
	// Preserve the earlier private Integer declaration-initializer capture.
	if len(vm.currentMethod.Params) == 0 && strings.EqualFold(field.Access, "private") &&
		strings.EqualFold(field.Type, "Integer") && field.InitializerLine != 0 && field.InitialValue.Kind == ValueInt {
		return &RuntimeError{Type: "System.FinalException", Message: "Final variable has already been initialized"}
	}
	// CD-076: named sibling constructors, absent declaration initializer,
	// zero args or one matching field-type arg. Every measured base write is
	// non-null; null initialization is not inferred from these rows.
	if field.InitializerLine != 0 || field.InitialValue.Kind != "" && field.InitialValue.Kind != ValueNull ||
		len(vm.currentMethod.Params) > 1 || strings.Count(caller.Name, ".") != 1 || strings.Count(base.Name, ".") != 1 ||
		!methodHasModifier(caller.Modifiers, "public") || !vm.capturedInheritedFinalType(owner, caller.Name, field.Type) {
		return nil
	}
	if len(vm.currentMethod.Params) == 1 && !strings.EqualFold(
		vm.resolveTypeNameInClass(caller.Name, vm.currentMethod.Params[0].Type), vm.resolveTypeNameInClass(owner, field.Type)) {
		return nil
	}
	name := field.StorageName
	if name == "" {
		name = field.Name
	}
	_, stored, found := objectFieldValue(object, name)
	if !found || stored.Kind == ValueNull || stored.Kind == "" {
		return nil
	}
	return &RuntimeError{Type: "System.FinalException", Message: "Final variable has already been initialized"}
}

func (vm *VM) capturedInheritedFinalType(scope, caller, raw string) bool {
	resolved := vm.resolveTypeNameInClass(scope, raw)
	switch strings.ToLower(strings.ReplaceAll(canonicalRuntimePlatformType(resolved), " ", "")) {
	case "integer", "string", "account", "list<string>", "map<string,object>":
		return true
	}
	user, ok := vm.lookupClass(resolved)
	return ok && !user.Dependency && user.Namespace == "" && !user.IsAbstract && !user.IsInterface && !user.PlatformType &&
		methodHasModifier(user.Modifiers, "public") && strings.Count(user.Name, ".") == 1 && sameLexicalTopLevel(caller, user.Name)
}
