package vm

import "strings"

// The captured rows, both routes at API 62/67/68: a populated Object-valued
// parameter map rejects typed Result deserialization. The empty Result object
// succeeds with five null fields. Other Result shapes retain the existing path.
func (vm *VM) invocableFlowResultFromJSON(typeName string, raw any, input jsonTypedInput, strict bool) (Value, bool, error) {
	if !strings.EqualFold(typeName, "Invocable.Action.Result") {
		return Null, false, nil
	}
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return Null, false, nil
	}
	for _, field := range fields {
		if field.name != "outputParameters" && field.name != "invocationParameters" {
			continue
		}
		parameters, object := jsonObjectFields(field.value)
		if !object || len(parameters) == 0 {
			continue
		}
		source := field.source
		if source == "" {
			source = input.source
		}
		_, err := vm.typedValueFromJSON("Map<String,Object>", jsonTypedInput{
			value: field.value, source: source, start: field.start,
		}, strict)
		if err != nil {
			return Null, true, err
		}
	}
	if len(fields) != 0 {
		return Null, false, nil
	}
	result := Object("Invocable.Action.Result")
	for _, field := range []string{"success", "outputParameters", "invocationParameters", "errors", "action"} {
		result.Fields[field] = Null
	}
	if input.root {
		vm.rememberLocalOnlyObject(result)
	}
	return result, true, nil
}

// The captured row, both routes, and 041-044, anonymous, at API 62/67/68 expose the
// Result field order and its custom Flow Action shape. Keep other Action kinds
// on their existing serializer path; their serialized shape was not captured.
func (vm *VM) invocableFlowActionJSONValue(value Value, suppressObjectNulls bool, active map[jsonSerializationIdentity]bool) (any, bool) {
	var fields []string
	switch {
	case strings.EqualFold(value.Type, "Invocable.Action.Result"):
		fields = []string{"success", "outputParameters", "invocationParameters", "errors", "action"}
		if !invocableFlowActionJSONShape(value.Fields["action"]) {
			if len(value.Fields) != len(fields) {
				return nil, false
			}
			for _, field := range fields {
				item, present := value.Fields[field]
				if !present || item.Kind != ValueNull {
					return nil, false
				}
			}
		}
	case invocableFlowActionJSONShape(value):
		fields = []string{"version", "type", "namespace", "name", "invokeFromLightningComponent", "invocations", "apiVersion"}
	default:
		return nil, false
	}
	serialized := orderedJSONObject{}
	for _, field := range fields {
		_, item, present := objectFieldValue(value, field)
		if !present {
			item = Null
		}
		if suppressObjectNulls && item.Kind == ValueNull {
			continue
		}
		serialized = append(serialized, orderedJSONField{
			name: field, value: vm.jsonFromValueForSerializeWithActive(item, suppressObjectNulls, active),
		})
	}
	return serialized, true
}

func invocableFlowActionJSONShape(value Value) bool {
	standard := value.Fields["standard"]
	return value.Kind == ValueObject && strings.EqualFold(value.Type, "Invocable.Action") &&
		strings.EqualFold(invocableActionType(value), "flow") && standard.Kind == ValueBool && !standard.Bool
}
