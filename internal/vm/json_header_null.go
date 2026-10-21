package vm

import "github.com/glade-sh/glade/internal/apexversion"

// Owned named and anonymous source-62/67/68 observations isolate these
// canonical header properties. Null recordIds rejects during JSON conversion;
// the other observed numeric/list properties retain their JSON defaults.
func (vm *VM) skipJSONHeaderNullField(typeName, field string, raw any) (bool, error) {
	if raw != nil || !vm.isPlatformJSONHeader(typeName) {
		return false, nil
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || (major != 62 && major != 67 && major != 68) {
		return false, nil
	}
	switch field {
	case "recordIds":
		return true, jsonDeserializeException("N/A")
	case "sequenceNumber", "commitTimestamp", "commitNumber", "changedFields", "nulledFields", "diffFields":
		return true, nil
	default:
		return false, nil
	}
}
