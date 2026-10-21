package vm

import "github.com/glade-sh/glade/internal/apexversion"

// Owned API62/63/67 N1-N4/N14 reject JSON null and the exact string "null"
// at genuine scalar Datetime roots. Recursive nulls keep their separate path.
func (vm *VM) jsonRootDatetimeNullError(typeName string, input jsonTypedInput) error {
	if !input.root || canonicalJSONScalarType(typeName) != "Datetime" {
		return nil
	}
	if input.value != nil {
		if text, ok := input.value.(string); !ok || text != "null" {
			return nil
		}
	}
	if vm.Org != nil && vm.Org.Namespace != "" {
		return nil
	}
	if source, known := vm.lookupClass(vm.currentMethod.ClassName); known && source.Namespace != "" {
		return nil
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
