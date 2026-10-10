package vm

import (
	"fmt"
	"strings"
)

func managedFeatureValueKey(kind, apiName string) string {
	return strings.ToLower(strings.TrimSpace(kind)) + ":" + strings.ToLower(strings.TrimSpace(apiName))
}

func (vm *VM) managedFeatureValue(kind string, args []Value) (Value, error) {
	if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) {
		return Null, fmt.Errorf("FeatureManagement.checkPackage%sValue expects String", kind)
	}
	if args[0].Kind == ValueNull {
		if kind == "Boolean" {
			return Bool(false), nil
		}
		return Null, nil
	}
	if value, ok := vm.managedFeatureValues[managedFeatureValueKey(kind, args[0].Text)]; ok {
		return cloneValue(value), nil
	}
	return Null, newExceptionError("System.NoDataFoundException", "Unable to retrieve feature parameter "+args[0].Text+". No results found.")
}
