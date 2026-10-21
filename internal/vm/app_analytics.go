package vm

import (
	"fmt"
	"strings"
)

func (vm *VM) appAnalyticsLogCustomInteraction(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("IsvPartners.AppAnalytics.logCustomInteraction expects interaction label[, id]")
	}
	invalidLabel := func(requirement string) (Value, error) {
		return Null, newExceptionError("System.InvalidParameterValueException", "We couldn’t log the custom interaction with App Analytics. The interaction label provided to IsvPartners.AppAnalytics.logCustomInteraction "+requirement)
	}
	label := args[0]
	switch label.Kind {
	case ValueNull, ValueString:
		return invalidLabel("must be an Apex enum.")
	case ValueInt, ValueDecimal:
		if len(args) == 1 {
			return invalidLabel("must be an Apex enum.")
		}
	}
	if label.Kind == ValueObject && label.Text != "" {
		platformType := label.Type
		if unqualified, ok := stripLeadingSystemNamespace(platformType); ok {
			platformType = unqualified
		}
		if enum, ok := vm.resolveEnumClass(label.Type); ok && !enum.PlatformType {
			// Captured calls use unpackaged enums in the caller's namespace.
			// Namespaced anonymous calls and foreign enum namespaces remain hosted.
			if !enum.Dependency && !vm.currentMethod.Dependency && strings.EqualFold(enum.Namespace, vm.currentCallerNamespace()) && (enum.Namespace == "" || vm.testContext != nil) {
				validArgument := len(args) == 1 || args[1].Kind == ValueNull
				if len(args) == 2 {
					if id, ok := idValueText(args[1]); ok && validateApexID(id) == nil {
						validArgument = true
					}
					if args[1].Kind == ValueObject && !args[1].classInstance && strings.EqualFold(args[1].Type, "UUID") {
						validArgument = true
					}
				}
				if validArgument {
					return Null, nil
				}
			}
		} else if len(args) == 1 && strings.EqualFold(platformType, "LoggingLevel") && label.Text == "INFO" {
			return invalidLabel("must have the same namespace as the Apex code that called the method.")
		}
	}
	return Null, unsupportedCallError("IsvPartners.AppAnalytics.logCustomInteraction hosted telemetry")
}
