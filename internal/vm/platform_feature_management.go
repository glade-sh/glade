package vm

import (
	"fmt"
	"strconv"
	"strings"
)

func managedFeatureValueKey(kind, apiName string) string {
	return strings.ToLower(kind) + ":" + strings.ToLower(apiName)
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
	key := managedFeatureValueKey(kind, args[0].Text)
	parameter, ok := vm.managedFeatureParameters[key]
	if !ok {
		return Null, managedFeatureParameterNotFound(args[0].Text)
	}
	if value, ok := vm.managedFeatureValues[key]; ok {
		return cloneValue(value), nil
	}
	return featureParameterValue(parameter)
}

func (vm *VM) setManagedFeatureValue(kind string, args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("FeatureManagement.setPackage%sValue expects String and %s", kind, kind)
	}
	if args[0].Kind == ValueNull {
		return Null, &RuntimeError{Type: "System.UnexpectedException", Message: "Unable to set a null feature parameter name"}
	}
	validValue := false
	switch kind {
	case "Boolean":
		validValue = args[1].Kind == ValueBool
	case "Integer":
		validValue = args[1].Kind == ValueInt
	case "Date":
		validValue = args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "Date")
	}
	if args[0].Kind != ValueString || !validValue {
		return Null, fmt.Errorf("FeatureManagement.setPackage%sValue expects String and %s", kind, kind)
	}
	key := managedFeatureValueKey(kind, args[0].Text)
	parameter, ok := vm.managedFeatureParameters[key]
	if !ok {
		return Null, managedFeatureParameterNotFound(args[0].Text)
	}
	if vm.testContext == nil && !strings.EqualFold(parameter.DataflowDirection, "SubscriberToLmo") {
		return Null, &RuntimeError{Type: "System.UnexpectedException", Message: "Feature parameter is not writable from the subscriber org"}
	}
	value := cloneValue(args[1])
	storedValue, err := featureParameterStoredValue(kind, value)
	if err != nil {
		return Null, err
	}
	if vm.managedFeatureValues == nil {
		vm.managedFeatureValues = make(map[string]Value)
	}
	vm.managedFeatureValues[key] = value
	if vm.Org != nil && vm.testContext == nil {
		for i := range vm.Org.Metadata.FeatureParameters {
			if managedFeatureValueKey(vm.Org.Metadata.FeatureParameters[i].Type, vm.Org.Metadata.FeatureParameters[i].Name) == key {
				vm.Org.Metadata.FeatureParameters[i].Value = storedValue
				break
			}
		}
	}
	return Null, nil
}

func featureParameterStoredValue(kind string, value Value) (string, error) {
	switch kind {
	case "Boolean":
		return strconv.FormatBool(value.Bool), nil
	case "Integer":
		return strconv.FormatInt(value.Int, 10), nil
	case "Date":
		return platformScalarText(value, "Date")
	default:
		return "", fmt.Errorf("unsupported feature parameter type")
	}
}

func managedFeatureParameterNotFound(name string) error {
	return newExceptionError("System.NoDataFoundException", "Unable to retrieve feature parameter "+name+". No results found.")
}
