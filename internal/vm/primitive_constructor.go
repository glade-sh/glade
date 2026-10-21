package vm

import "github.com/glade-sh/glade/internal/apexversion"

// Owned source62/63/67 Type-parameter observations distinguish an Integer class
// literal from Type.forName. Their existing token payloads retain that origin.
func (vm *VM) integerReflectionConstructorMessage(token Value) (string, bool) {
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 62 && major != 63 && major != 67 {
		return "", false
	}
	if token.Text == "Integer" {
		if _, scalarPayload := token.Fields["value"]; !scalarPayload {
			return "Integer does not have a no-arg constructor", true
		}
	}
	if token.Text == "" {
		if name, scalarPayload := platformScalarObjectText(token); scalarPayload && name == "Integer" {
			return "Integer cannot be constructed", true
		}
	}
	return "", false
}
