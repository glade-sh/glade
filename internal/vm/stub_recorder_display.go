package vm

import "strings"

// The captured stateful provider is a top-level class without a superclass.
// Match declared field types and live state, never class or field names.
func (vm *VM) stubNamespaceRecorderProvider(provider Value, class Class) bool {
	if len(provider.Fields) != 5 || len(class.Fields) != 5 || class.SuperClass != "" {
		return false
	}
	types := map[string]int{}
	for name, field := range class.Fields {
		if field.Static || field.Property {
			return false
		}
		value, present := provider.Fields[name]
		if !present {
			return false
		}
		typ := strings.ReplaceAll(strings.ToLower(strings.ReplaceAll(field.Type, " ", "")), "system.", "")
		switch typ {
		case "integer":
			if value.Kind != ValueInt {
				return false
			}
		case "string":
			if value.Kind != ValueNull && value.Kind != ValueString {
				return false
			}
		case "type":
			if value.Kind != ValueObject || !strings.EqualFold(value.Type, "Type") {
				return false
			}
		case "list<type>":
			if value.Kind != ValueNull && value.Kind != ValueList {
				return false
			}
			for _, item := range value.List {
				if item.Kind != ValueObject || !strings.EqualFold(item.Type, "Type") {
					return false
				}
			}
		default:
			return false
		}
		types[typ]++
	}
	return types["integer"] == 2 && types["string"] == 1 && types["type"] == 1 && types["list<type>"] == 1
}

// Stateful proxy text is captured through explicit conversions, concatenation
// and object formatting. Empty-provider implicit display keeps its prior path.
func (vm *VM) stubNamespaceRecordedProxyText(value Value, seen map[uint64]bool) (string, bool) {
	if _, captured := vm.stubRecorderPlainProxyName(value); captured {
		return vm.stubProxyDisplayStringForOperation(value, seen, true), true
	}
	if _, captured := vm.stubNamespaceFieldlessProxyName(value); !captured || len(vm.currentStubProvider(value).Fields) == 0 {
		return "", false
	}
	return vm.stubProxyDisplayStringForOperation(value, seen, true), true
}

// The captured containers each hold one proxy directly: a list, a map with a
// String on the other side, or an ordinary Object-field holder in its context.
// Other container shapes retain their existing formatter.
func (vm *VM) stubNamespaceRecordedContainerText(value Value, seen map[uint64]bool) (string, bool) {
	if text, captured := vm.stubNamespaceRecordedProxyText(value, seen); captured {
		return text, true
	}
	switch value.Kind {
	case ValueList:
		if len(value.List) != 1 {
			return "", false
		}
		if text, captured := vm.stubNamespaceRecordedProxyText(value.List[0], seen); captured {
			return "(" + text + ")", true
		}
	case ValueMap:
		if len(value.Map) != 1 {
			return "", false
		}
		for key, item := range value.Map {
			storedKey := mapStoredKey(value, key)
			if item.Kind == ValueString {
				if text, captured := vm.stubNamespaceRecordedProxyText(storedKey, seen); captured {
					return "{" + text + "=" + item.Text + "}", true
				}
			}
			if storedKey.Kind == ValueString {
				if text, captured := vm.stubNamespaceRecordedProxyText(item, seen); captured {
					return "{" + storedKey.Text + "=" + text + "}", true
				}
			}
		}
	case ValueObject:
		if len(value.Fields) != 1 {
			return "", false
		}
		class, ok := vm.lookupClass(value.Type)
		if !ok || !strings.EqualFold(class.Namespace, vm.currentCallerNamespace()) || classIsInner(class) || class.Dependency || class.PlatformType || class.SuperClass != "" || len(class.Fields) != 1 {
			return "", false
		}
		// A user-defined toString must retain normal method dispatch.
		if _, found, ambiguous := vm.resolveInstanceMethodForArgs(value.Type, "toString", nil); found || ambiguous {
			return "", false
		}
		for name, field := range class.Fields {
			if field.Static || field.Property || !strings.EqualFold(field.Type, "Object") {
				return "", false
			}
			if text, captured := vm.stubNamespaceRecordedProxyText(value.Fields[name], seen); captured {
				return stubNamespaceLocalClassName(class) + ":[" + name + "=" + text + "]", true
			}
		}
	}
	return "", false
}

// Concatenation and String.format opt into their captured stateful object text.
// Debug and direct object assertion display keep the generic formatter.
func (vm *VM) stubNamespaceExpressionText(value Value, result *Result) (string, error) {
	if text, captured := vm.stubNamespaceRecordedProxyText(value, make(map[uint64]bool)); captured {
		return text, nil
	}
	return vm.displayString(value, result)
}
