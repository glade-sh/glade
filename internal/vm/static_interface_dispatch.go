package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Owned static_interface_dispatch_native and static_interface_result_native
// observations dispatch these public static signatures by the actual receiver.
// Other signatures and inherited static bodies retain their resolution paths.
func (vm *VM) observedStaticInterfaceImplementation(receiver Value, surface, candidate Method) bool {
	if vm == nil || !candidate.IsStatic || candidate.IsConstructor ||
		!strings.EqualFold(candidate.Access, "public") ||
		!methodHasModifier(surface.Modifiers, "abstract") || methodHasModifier(candidate.Modifiers, "abstract") ||
		methodHasModifier(candidate.Modifiers, "future") ||
		!vm.sameResolvedMethodParams(candidate, surface) || vm.currentCallerNamespace() != "" ||
		surface.Namespace != "" || candidate.Namespace != "" || vm.Org != nil && vm.Org.Namespace != "" {
		return false
	}
	contract, ok := vm.lookupClass(surface.ClassName)
	if !ok || !contract.IsInterface || contract.PlatformType || contract.Namespace != "" {
		return false
	}
	runtimeType := runtimeObjectType(receiver)
	implementation, ok := vm.lookupClass(runtimeType)
	if !ok || implementation.IsInterface || implementation.IsAbstract || implementation.PlatformType || implementation.Namespace != "" ||
		!strings.EqualFold(candidate.ClassName, runtimeType) || !vm.typeMatches(runtimeType, contract.Name, make(map[string]bool)) {
		return false
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known {
		return false
	}
	voidSignature := len(surface.Params) == 0 && len(candidate.Params) == 0 &&
		strings.EqualFold(surface.ReturnType, "void") && strings.EqualFold(candidate.ReturnType, "void") &&
		(major == 52 || major == 62 || major == 67)
	resultSignature := false
	if len(surface.Params) == 2 && len(candidate.Params) == 2 &&
		strings.EqualFold(surface.ReturnType, "Object") && strings.EqualFold(candidate.ReturnType, "Object") &&
		strings.EqualFold(surface.Params[0].Type, "Object") && (major == 62 || major == 67 || major == 68) {
		key, value, ok := mapTypeArgs(surface.Params[1].Type)
		resultSignature = ok && strings.EqualFold(key, "String") && strings.EqualFold(value, "Object")
	}
	if !voidSignature && !resultSignature {
		return false
	}
	for _, api := range []string{surface.APIVersion, candidate.APIVersion, contract.APIVersion, implementation.APIVersion} {
		version, known := apexversion.Major(api)
		if !known || version != major {
			return false
		}
	}
	return true
}
