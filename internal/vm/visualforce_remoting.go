package vm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DecodeRemotingScalarArgument converts the wire value to the declared Apex
// parameter type before method selection. Remoting has its own date and Id
// contract; its other scalar conversions reuse the JSON runtime.
// The boolean reports whether this entry point handles the parameter type.
func DecodeRemotingScalarArgument(typeName string, raw json.RawMessage) (Value, bool, error) {
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	typeName = canonicalJSONScalarType(typeName)
	switch typeName {
	case "String", "Boolean", "Integer", "Long", "Decimal", "Double", "Date", "Datetime", "Id":
	default:
		return Null, false, nil
	}
	decoded, err := decodeJSONValue(string(raw))
	if err != nil {
		return Null, true, err
	}
	if decoded == nil {
		return Null, true, nil
	}
	if typeName == "Date" || typeName == "Datetime" {
		if text, ok := decoded.(string); ok {
			return Null, true, fmt.Errorf("Unable to convert date '%s' to Apex type %s.", text, typeName)
		}
		return Null, false, nil
	}
	if typeName == "Id" {
		text, ok := decoded.(string)
		if !ok {
			return Null, false, nil
		}
		if text != "" {
			if err := validateApexIDShape(text); err != nil {
				return Null, true, fmt.Errorf("Value '%s' cannot be converted from String to Id.", text)
			}
		}
		value := String(text)
		value.Type = "Id"
		return value, true, nil
	}
	// Composite values keep the caller's object/collection conversion path.
	switch decoded.(type) {
	case bool:
		if typeName != "String" && typeName != "Boolean" {
			return Null, false, nil
		}
	case string, json.Number:
	default:
		return Null, false, nil
	}
	value, _, err := typedScalarFromJSON(typeName, decoded)
	if typeName == "Integer" && (err != nil || value.Kind == ValueInt && (value.Int < -2147483648 || value.Int > 2147483647)) {
		return Null, true, fmt.Errorf("Unable to convert number %v to Apex type %s.", decoded, typeName)
	}
	if err != nil {
		return Null, false, nil
	}
	return value, true, nil
}

// DecodeRemotingArgument resolves inner DTO names in the action's declaring
// class and reuses typed JSON conversion for collections and registered types.
func (vm *VM) DecodeRemotingArgument(className, typeName string, raw json.RawMessage) (Value, bool, error) {
	if value, handled, err := DecodeRemotingScalarArgument(typeName, raw); handled || err != nil {
		return value, handled, err
	}
	typeName = vm.resolveTypeNameInClass(className, typeName)
	_, registeredClass := vm.lookupClass(typeName)
	if collectionBase(typeName) == "" && !isMapType(typeName) && !registeredClass && !vm.isSObjectLikeType(typeName) {
		return Null, false, nil
	}
	decoded, err := decodeJSONValue(string(raw))
	if err != nil {
		return Null, true, err
	}
	if decoded == nil {
		return Null, true, nil
	}
	value, err := vm.typedValueFromJSON(typeName, decoded, false)
	if err != nil {
		// Preserve the existing dispatch path for unsupported mappings. This
		// boundary adds successful declared conversions, not JSON diagnostics.
		return Null, false, nil
	}
	return value, true, nil
}

// RemotingJSONResult projects the captured collection, SObject and Apex DTO
// shapes. Ordinary JSON.serialize and REST serialization keep their contracts.
// The boolean lets the server retain its existing scalar/other-value mapping.
func (vm *VM) RemotingJSONResult(value Value) (any, bool) {
	typeName := value.Type
	if value.Static != "" {
		typeName = value.Static
	}
	switch value.Kind {
	case ValueList:
		elementType, ok := collectionElementType(typeName)
		if !ok || !strings.EqualFold(elementType, "String") {
			return nil, false
		}
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			if item.Kind != ValueNull {
				out = append(out, vm.jsonFromValueForSerialize(item, true))
			}
		}
		return out, true
	case ValueMap:
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok || !strings.EqualFold(keyType, "String") || !strings.EqualFold(valueType, "String") {
			return nil, false
		}
		out := map[string]any{}
		for key, item := range StringValueMapEntries(value) {
			if item.Kind != ValueNull {
				out[key] = vm.jsonFromValueForSerialize(item, true)
			}
		}
		return out, true
	case ValueObject:
		if vm.isSObjectLikeType(value.Type) {
			out := map[string]any{}
			for _, name := range jsonSObjectFieldNames(value) {
				out[name] = vm.jsonFromValueForSerialize(value.Fields[name], true)
			}
			return out, true
		}
		if _, registered := vm.lookupClass(value.Type); registered {
			return vm.jsonFromValueForSerialize(value, true), true
		}
	}
	return nil, false
}
