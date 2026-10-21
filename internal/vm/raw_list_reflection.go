package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// The owned helper61/caller61 named rows resolve this exact Date-cast label.
// Class lookup takes precedence; no other erased label or overload is inferred.
func (vm *VM) rawListReflectionAlias(name string) (string, bool) {
	version := vm.currentMethod.APIVersion
	if name != "List<ANY>" || !apexversion.AtLeast(version, 61) || apexversion.AtLeast(version, 62) {
		return "", false
	}
	return "List<Object>", true
}

// Source61 named rows observe raw List reflection for Object and String.
// Date and the other versions retain the pre-existing pending boundary.
// Assignment and instanceof continue to use their existing generic rules.
func (vm *VM) rawListReflectionCandidate(source, target string) bool {
	version := vm.currentMethod.APIVersion
	if version == "" && vm.Org != nil {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 61) || apexversion.AtLeast(version, 69) ||
		!strings.EqualFold(target, "List") || !strings.EqualFold(collectionBase(source), "List") {
		return false
	}
	argument, known := collectionElementType(source)
	capturedString := strings.EqualFold(argument, "String") && !apexversion.AtLeast(version, 62)
	if !known || (!strings.EqualFold(argument, "Object") && !strings.EqualFold(argument, "Date") && !capturedString) {
		return false
	}
	for _, name := range []string{target, vm.resolveTypeNameInClass(vm.currentClass, argument)} {
		if class, found := vm.lookupClass(name); found && !class.PlatformType {
			return false
		}
	}
	return true
}
