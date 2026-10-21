package vm

import "github.com/glade-sh/glade/internal/apexversion"

// Owned source-62/63/67 JSON observations reject null and the string "null"
// for a scalar Date root. Fields, collection entries and JSONParser retain
// their separate null behavior.
func (vm *VM) jsonRootDateNullError(typeName string, input jsonTypedInput) error {
	if !input.root || canonicalJSONScalarType(typeName) != "Date" {
		return nil
	}
	if input.value != nil {
		if text, ok := input.value.(string); !ok || text != "null" {
			return nil
		}
	}
	if class, registered := vm.lookupClass(typeName); registered && !class.PlatformType {
		return nil
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || (major != 62 && major != 63 && major != 67) {
		return nil
	}
	return jsonDeserializeException("Invalid format: \"null\"")
}
