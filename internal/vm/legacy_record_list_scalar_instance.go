package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Owned source37 predicates accept Object-held List<SObject> and List<Account>
// as SObject, including empty and null-placeholder lists. The value remains a
// whole list; no element matching, stub-provider or assignment behavior changes.
func (vm *VM) source37RecordListScalarInstance(value Value, target string) bool {
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 37 || value.Kind != ValueList || !strings.EqualFold(value.Static, "Object") || !strings.EqualFold(target, "SObject") {
		return false
	}
	if class, exists := vm.lookupClass(target); exists && !class.PlatformType {
		return false
	}
	runtimeType := value.Runtime
	if runtimeType == "" {
		runtimeType = value.Type
	}
	if strings.EqualFold(runtimeType, "List<SObject>") {
		return true
	}
	if !strings.EqualFold(runtimeType, "List<Account>") {
		return false
	}
	// A user class with this spelling does not denote the captured record.
	class, exists := vm.lookupClass("Account")
	return !exists || class.PlatformType
}
