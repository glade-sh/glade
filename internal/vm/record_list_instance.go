package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Captured generic SObject list views admit the Lead and Case type checks,
// both empty and after addAll. Concrete runtime lists keep their own type.
// Other record and collection pairs remain outside these observations.
func (vm *VM) genericRecordListObservedInstance(value Value, target string) bool {
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 62 && major != 67 && major != 68 {
		return false
	}
	if value.Kind != ValueList || collectionBase(target) != "List" {
		return false
	}
	element, ok := collectionElementType(target)
	if record, explicit := vm.explicitSchemaRecordType(element); explicit {
		element = record
	} else if _, class := vm.lookupClass(element); class {
		return false
	}
	if !ok || !(strings.EqualFold(element, "Lead") || strings.EqualFold(element, "Case")) {
		return false
	}
	runtimeType := value.Runtime
	if runtimeType == "" {
		runtimeType = value.Type
	}
	runtimeElement, ok := collectionElementType(runtimeType)
	return ok && collectionBase(runtimeType) == "List" && strings.EqualFold(runtimeElement, "SObject")
}

// Lead views are captured. Account retains the narrowly required compatibility
// shape pending equivalent native observations; other record types stay on the
// previous assertion path.
func (vm *VM) recordListAssertionRuntimeTypeSupported(element string) bool {
	if record, explicit := vm.explicitSchemaRecordType(element); explicit {
		element = record
	}
	// Preserve the previously supported Account source versions while their
	// native equivalents are pending; this does not expand that path.
	if strings.EqualFold(element, "Account") {
		return true
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	return known && (major == 62 || major == 67 || major == 68) && strings.EqualFold(element, "Lead")
}
