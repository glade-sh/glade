package vm

import (
	"fmt"
	"strings"
)

// LX030/LX032 capture reserved String locals, distinct from the reserved type
// names that retain the existing declaration diagnostic.
func soqlBindReservedLocalError(typeName string, name token) error {
	if strings.EqualFold(typeName, "String") && (name.text == "select" || name.text == "class") {
		return &ApexSyntaxError{Message: fmt.Sprintf("reserved local identifier %s at byte %d", name.text, name.pos), NativeMessage: "Unexpected token '" + name.text + "'.", Offset: name.pos}
	}
	return nil
}
