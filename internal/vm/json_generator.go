package vm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func newJSONGenerator(pretty bool) Value {
	gen := Object("JSONGenerator")
	gen.Fields["pretty"] = Bool(pretty)
	gen.Fields["closed"] = Bool(false)
	gen.Fields["rootWritten"] = Bool(false)
	gen.Fields["out"] = String("")
	gen.Fields["stack"] = List()
	return gen
}

func (vm *VM) callJSONGeneratorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if receiver.Type != "JSONGenerator" {
		return Null, receiver, false, false, nil
	}
	method = canonicalJSONGeneratorMethod(method)
	switch method {
	case "writeStartObject":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeStartObject expects 0 arguments")
		}
		return jsonGeneratorWriteContainer(receiver, "object")
	case "writeEndObject":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeEndObject expects 0 arguments")
		}
		return jsonGeneratorEndContainer(receiver, "object")
	case "writeStartArray":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeStartArray expects 0 arguments")
		}
		return jsonGeneratorWriteContainer(receiver, "array")
	case "writeEndArray":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeEndArray expects 0 arguments")
		}
		return jsonGeneratorEndContainer(receiver, "array")
	case "writeFieldName":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeFieldName expects String")
		}
		updated, err := jsonGeneratorWriteFieldName(receiver, args[0].Text)
		return Null, updated, true, true, err
	case "writeString":
		value, ok := jsonGeneratorStringArgument(args, 0)
		if len(args) != 1 || !ok {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeString expects String")
		}
		return jsonGeneratorWriteScalar(receiver, value)
	case "writeStringField":
		value, ok := jsonGeneratorStringArgument(args, 1)
		if len(args) != 2 || args[0].Kind != ValueString || !ok {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeStringField expects String field name and String value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, value)
	case "writeObject":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeObject expects Object")
		}
		return vm.jsonGeneratorWriteAny(receiver, args[0])
	case "writeObjectField":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeObjectField expects String field name and Object value")
		}
		return vm.jsonGeneratorWriteAnyField(receiver, args[0].Text, args[1])
	case "writeNumber":
		if len(args) != 1 || !jsonGeneratorIsNumber(args[0]) {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeNumber expects numeric value")
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeNumberField":
		if len(args) != 2 || args[0].Kind != ValueString || !jsonGeneratorIsNumber(args[1]) {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeNumberField expects String field name and numeric value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "writeBoolean":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeBoolean expects Boolean")
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeBooleanField":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeBooleanField expects String field name and Boolean value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "writeNull":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeNull expects 0 arguments")
		}
		return jsonGeneratorWriteScalar(receiver, Null)
	case "writeNullField":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeNullField expects String field name")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, Null)
	case "writeRaw":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeRaw expects String")
		}
		return jsonGeneratorWriteRawValue(receiver, args[0].Text, method)
	case "writeRawValue":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeRawValue expects String")
		}
		return jsonGeneratorWriteRawValue(receiver, args[0].Text, method)
	case "writeRawField":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeRawField expects String field name and String raw JSON value")
		}
		return jsonGeneratorWriteRawField(receiver, args[0].Text, args[1].Text)
	case "writeDate":
		if len(args) != 1 || !jsonGeneratorIsPlatformScalar(args[0], "Date") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeDate expects Date")
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeDateField":
		if len(args) != 2 || args[0].Kind != ValueString || !jsonGeneratorIsPlatformScalar(args[1], "Date") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeDateField expects String field name and Date value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "writeDateTime", "writeDatetime":
		if len(args) != 1 || !jsonGeneratorIsPlatformScalar(args[0], "Datetime") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.%s expects Datetime", method)
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeDateTimeField", "writeDatetimeField":
		if len(args) != 2 || args[0].Kind != ValueString || !jsonGeneratorIsPlatformScalar(args[1], "Datetime") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.%s expects String field name and Datetime value", method)
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "writeTime":
		if len(args) != 1 || !jsonGeneratorIsPlatformScalar(args[0], "Time") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeTime expects Time")
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeTimeField":
		if len(args) != 2 || args[0].Kind != ValueString || !jsonGeneratorIsPlatformScalar(args[1], "Time") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeTimeField expects String field name and Time value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "writeId":
		if len(args) != 1 || !jsonGeneratorIsID(args[0]) {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeId expects Id")
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeIdField":
		if len(args) != 2 || args[0].Kind != ValueString || !jsonGeneratorIsID(args[1]) {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeIdField expects String field name and Id value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "writeBlob":
		if len(args) != 1 || !jsonGeneratorIsPlatformScalar(args[0], "Blob") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeBlob expects Blob")
		}
		return jsonGeneratorWriteScalar(receiver, args[0])
	case "writeBlobField":
		if len(args) != 2 || args[0].Kind != ValueString || !jsonGeneratorIsPlatformScalar(args[1], "Blob") {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.writeBlobField expects String field name and Blob value")
		}
		return jsonGeneratorWriteField(receiver, args[0].Text, args[1])
	case "getAsString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.getAsString expects 0 arguments")
		}
		updated, err := jsonGeneratorClose(receiver)
		if err != nil {
			return Null, updated, true, true, err
		}
		return jsonGeneratorStringField(updated, "out"), updated, true, true, nil
	case "close":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.close expects 0 arguments")
		}
		updated, err := jsonGeneratorClose(receiver)
		return Null, updated, true, true, err
	case "isClosed":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONGenerator.isClosed expects 0 arguments")
		}
		return jsonGeneratorBoolField(receiver, "closed"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func canonicalJSONGeneratorMethod(method string) string {
	return canonicalStdlibMemberName(method,
		"writeStartObject", "writeEndObject", "writeStartArray", "writeEndArray", "writeFieldName",
		"writeString", "writeStringField", "writeObject", "writeObjectField",
		"writeNumber", "writeNumberField", "writeBoolean", "writeBooleanField",
		"writeNull", "writeNullField", "writeRaw", "writeRawValue", "writeRawField",
		"writeDate", "writeDateField", "writeDateTime", "writeDatetime", "writeDateTimeField", "writeDatetimeField",
		"writeTime", "writeTimeField", "writeId", "writeIdField", "writeBlob", "writeBlobField",
		"getAsString", "close", "isClosed",
	)
}

func jsonGeneratorWriteContainer(receiver Value, kind string) (Value, Value, bool, bool, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return Null, receiver, false, true, err
	}
	updated, err := jsonGeneratorBeforeValue(receiver)
	if err != nil {
		return Null, updated, true, true, err
	}
	if kind == "object" {
		jsonGeneratorAppend(&updated, "{")
	} else {
		jsonGeneratorAppend(&updated, "[")
	}
	stack := jsonGeneratorStack(updated)
	frame := Object("JSONGenerator.Frame")
	frame.Fields["kind"] = String(kind)
	frame.Fields["count"] = Int(0)
	frame.Fields["expectingField"] = Bool(kind == "object")
	frame.Fields["pendingField"] = String("")
	stack.List = append(stack.List, frame)
	updated.Fields["stack"] = stack
	return Null, updated, true, true, nil
}

func jsonGeneratorEndContainer(receiver Value, kind string) (Value, Value, bool, bool, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return Null, receiver, false, true, err
	}
	stack := jsonGeneratorStack(receiver)
	if len(stack.List) == 0 {
		return Null, receiver, false, true, jsonGeneratorException("JSONGenerator.writeEnd%s has no open %s", jsonGeneratorContainerName(kind), kind)
	}
	frame := stack.List[len(stack.List)-1]
	openKind := jsonGeneratorStringField(frame, "kind").Text
	if openKind != kind {
		if kind == "object" && openKind == "array" {
			return Null, receiver, false, true, newExceptionError("JSONException", "JSONGenerator.writeEndObject cannot be called inside an array")
		}
		if kind == "array" && openKind == "object" {
			return Null, receiver, false, true, newExceptionError("JSONException", "JSONGenerator.writeEndArray cannot be called inside an object")
		}
		return Null, receiver, false, true, jsonGeneratorException("JSONGenerator.writeEnd%s called while %s is open", jsonGeneratorContainerName(kind), openKind)
	}
	if kind == "object" && !jsonGeneratorBoolField(frame, "expectingField").Bool {
		return Null, receiver, false, true, jsonGeneratorException("JSONGenerator object field is missing a value")
	}
	updated := receiver
	if jsonGeneratorIntField(frame, "count").Int > 0 && jsonGeneratorPretty(receiver) {
		jsonGeneratorAppend(&updated, "\n"+strings.Repeat("  ", len(stack.List)-1))
	}
	if kind == "object" {
		jsonGeneratorAppend(&updated, "}")
	} else {
		jsonGeneratorAppend(&updated, "]")
	}
	stack.List = stack.List[:len(stack.List)-1]
	updated.Fields["stack"] = stack
	return Null, updated, true, true, nil
}

func jsonGeneratorContainerName(kind string) string {
	switch kind {
	case "object":
		return "Object"
	case "array":
		return "Array"
	default:
		return kind
	}
}

func jsonGeneratorWriteField(receiver Value, name string, value Value) (Value, Value, bool, bool, error) {
	updated, err := jsonGeneratorWriteFieldName(receiver, name)
	if err != nil {
		return Null, updated, true, true, err
	}
	return jsonGeneratorWriteScalar(updated, value)
}

func (vm *VM) jsonGeneratorWriteAnyField(receiver Value, name string, value Value) (Value, Value, bool, bool, error) {
	updated, err := jsonGeneratorWriteFieldName(receiver, name)
	if err != nil {
		return Null, updated, true, true, err
	}
	return vm.jsonGeneratorWriteAny(updated, value)
}

func jsonGeneratorWriteFieldName(receiver Value, name string) (Value, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return receiver, err
	}
	stack := jsonGeneratorStack(receiver)
	if len(stack.List) == 0 {
		return receiver, jsonGeneratorException("JSONGenerator.writeFieldName requires an open object")
	}
	frame := stack.List[len(stack.List)-1]
	if jsonGeneratorStringField(frame, "kind").Text != "object" {
		return receiver, newExceptionError("JSONException", "JSONGenerator.writeFieldName cannot be called inside an array")
	}
	if !jsonGeneratorBoolField(frame, "expectingField").Bool {
		return receiver, jsonGeneratorException("JSONGenerator field %q is missing a value", jsonGeneratorStringField(frame, "pendingField").Text)
	}
	updated := receiver
	count := jsonGeneratorIntField(frame, "count").Int
	if count > 0 {
		jsonGeneratorAppend(&updated, ",")
	}
	if jsonGeneratorPretty(updated) {
		jsonGeneratorAppend(&updated, "\n"+strings.Repeat("  ", len(stack.List)))
	}
	jsonGeneratorAppend(&updated, jsonGeneratorQuote(name))
	if jsonGeneratorPretty(updated) {
		jsonGeneratorAppend(&updated, ": ")
	} else {
		jsonGeneratorAppend(&updated, ":")
	}
	frame.Fields["expectingField"] = Bool(false)
	frame.Fields["pendingField"] = String(name)
	stack.List[len(stack.List)-1] = frame
	updated.Fields["stack"] = stack
	return updated, nil
}

func jsonGeneratorWriteScalar(receiver Value, value Value) (Value, Value, bool, bool, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return Null, receiver, false, true, err
	}
	updated, err := jsonGeneratorBeforeValue(receiver)
	if err != nil {
		return Null, updated, true, true, err
	}
	rendered, err := jsonGeneratorRenderScalar(value)
	if err != nil {
		return Null, updated, true, true, err
	}
	jsonGeneratorAppend(&updated, rendered)
	return Null, updated, true, true, nil
}

func (vm *VM) jsonGeneratorWriteAny(receiver Value, value Value) (Value, Value, bool, bool, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return Null, receiver, false, true, err
	}
	updated, err := jsonGeneratorBeforeValue(receiver)
	if err != nil {
		return Null, updated, true, true, err
	}
	rendered, err := vm.jsonGeneratorRenderAny(value)
	if err != nil {
		return Null, updated, true, true, err
	}
	jsonGeneratorAppend(&updated, rendered)
	return Null, updated, true, true, nil
}

func jsonGeneratorWriteRawField(receiver Value, name, raw string) (Value, Value, bool, bool, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return Null, receiver, false, true, err
	}
	normalized, err := jsonGeneratorValidateRawValue(raw, "writeRawField")
	if err != nil {
		return Null, receiver, false, true, err
	}
	updated, err := jsonGeneratorWriteFieldName(receiver, name)
	if err != nil {
		return Null, updated, true, true, err
	}
	return jsonGeneratorWriteRawValue(updated, normalized, "writeRawField")
}

func jsonGeneratorWriteRawValue(receiver Value, raw, method string) (Value, Value, bool, bool, error) {
	if err := jsonGeneratorEnsureOpen(receiver); err != nil {
		return Null, receiver, false, true, err
	}
	normalized, err := jsonGeneratorValidateRawValue(raw, method)
	if err != nil {
		return Null, receiver, false, true, err
	}
	updated, err := jsonGeneratorBeforeValue(receiver)
	if err != nil {
		return Null, updated, true, true, err
	}
	jsonGeneratorAppend(&updated, normalized)
	return Null, updated, true, true, nil
}

func jsonGeneratorValidateRawValue(raw, method string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", jsonGeneratorException("JSONGenerator.%s expects valid raw JSON value", method)
	}
	decoder := json.NewDecoder(strings.NewReader(normalized))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return "", jsonGeneratorException("JSONGenerator.%s expects valid raw JSON value: %v", method, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return "", jsonGeneratorException("JSONGenerator.%s expects one raw JSON value", method)
		}
		return "", jsonGeneratorException("JSONGenerator.%s expects one raw JSON value: %v", method, err)
	}
	return normalized, nil
}

func jsonGeneratorBeforeValue(receiver Value) (Value, error) {
	updated := receiver
	stack := jsonGeneratorStack(updated)
	if len(stack.List) == 0 {
		if jsonGeneratorBoolField(updated, "rootWritten").Bool {
			return updated, jsonGeneratorException("JSONGenerator root value already written")
		}
		updated.Fields["rootWritten"] = Bool(true)
		return updated, nil
	}
	frame := stack.List[len(stack.List)-1]
	switch jsonGeneratorStringField(frame, "kind").Text {
	case "array":
		count := jsonGeneratorIntField(frame, "count").Int
		if count > 0 {
			jsonGeneratorAppend(&updated, ",")
		}
		if jsonGeneratorPretty(updated) {
			jsonGeneratorAppend(&updated, "\n"+strings.Repeat("  ", len(stack.List)))
		}
		frame.Fields["count"] = Int(count + 1)
		stack.List[len(stack.List)-1] = frame
		updated.Fields["stack"] = stack
		return updated, nil
	case "object":
		if jsonGeneratorBoolField(frame, "expectingField").Bool {
			return updated, jsonGeneratorException("JSONGenerator object value requires writeFieldName first")
		}
		frame.Fields["count"] = Int(jsonGeneratorIntField(frame, "count").Int + 1)
		frame.Fields["expectingField"] = Bool(true)
		frame.Fields["pendingField"] = String("")
		stack.List[len(stack.List)-1] = frame
		updated.Fields["stack"] = stack
		return updated, nil
	default:
		return updated, jsonGeneratorException("JSONGenerator has invalid internal frame")
	}
}

func jsonGeneratorClose(receiver Value) (Value, error) {
	updated := receiver
	if jsonGeneratorBoolField(updated, "closed").Bool {
		return updated, nil
	}
	stack := jsonGeneratorStack(updated)
	for len(stack.List) > 0 {
		frame := stack.List[len(stack.List)-1]
		kind := jsonGeneratorStringField(frame, "kind").Text
		if kind == "object" {
			if !jsonGeneratorBoolField(frame, "expectingField").Bool {
				jsonGeneratorTrimPendingFieldSeparator(&updated)
			}
			jsonGeneratorAppend(&updated, "}")
		} else {
			jsonGeneratorAppend(&updated, "]")
		}
		stack.List = stack.List[:len(stack.List)-1]
	}
	updated.Fields["stack"] = stack
	updated.Fields["closed"] = Bool(true)
	return updated, nil
}

func jsonGeneratorTrimPendingFieldSeparator(receiver *Value) {
	current := jsonGeneratorStringField(*receiver, "out").Text
	current = strings.TrimSuffix(current, ": ")
	current = strings.TrimSuffix(current, ":")
	receiver.Fields["out"] = String(current)
}

func jsonGeneratorEnsureOpen(receiver Value) error {
	if jsonGeneratorBoolField(receiver, "closed").Bool {
		return newExceptionError("JSONException", "JSONGenerator is closed")
	}
	return nil
}

func jsonGeneratorRenderScalar(value Value) (string, error) {
	switch value.Kind {
	case ValueNull:
		return "null", nil
	case ValueString:
		return jsonGeneratorQuote(value.Text), nil
	case ValueBool:
		if value.Bool {
			return "true", nil
		}
		return "false", nil
	case ValueInt:
		return strconv.FormatInt(value.Int, 10), nil
	case ValueDecimal:
		if math.IsNaN(value.Decimal) || math.IsInf(value.Decimal, 0) {
			return "", jsonGeneratorException("JSONGenerator.writeNumber cannot write non-finite number")
		}
		if value.Text != "" {
			return value.Text, nil
		}
		return strconv.FormatFloat(value.Decimal, 'f', -1, 64), nil
	case ValueObject:
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			data, err := jsonMarshalNoEscape(scalar)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
		return "", jsonGeneratorException("JSONGenerator scalar writer does not support %s", value.Type)
	default:
		return "", jsonGeneratorException("JSONGenerator scalar writer does not support %s", value.Kind)
	}
}

func jsonGeneratorException(format string, args ...any) error {
	return newExceptionError("JSONException", fmt.Sprintf(format, args...))
}

func (vm *VM) jsonGeneratorRenderAny(value Value) (string, error) {
	data, err := jsonMarshalNoEscape(vm.jsonFromValueForSerialize(value, false))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func jsonPlatformScalarFromValue(value Value) (any, bool) {
	if value.Kind != ValueObject {
		return nil, false
	}
	if strings.TrimSpace(value.Text) != "" && len(value.Fields) == 0 {
		return value.Text, true
	}
	text, ok := value.Fields["value"]
	if !ok || text.Kind != ValueString {
		return nil, false
	}
	typeName := value.Type
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch typeName {
	case "Date", "Datetime", "URL":
		return text.Text, true
	case "Time":
		if clock, err := parseTimeText(text.Text); err == nil {
			return clock + "Z", true
		}
		return text.Text, true
	case "Blob":
		return base64.StdEncoding.EncodeToString([]byte(text.Text)), true
	case "Id":
		return text.Text, true
	default:
		return nil, false
	}
}

func jsonGeneratorQuote(text string) string {
	data, err := jsonMarshalNoEscape(text)
	if err != nil {
		return "\"\""
	}
	return string(data)
}

func jsonGeneratorAppend(receiver *Value, text string) {
	current := jsonGeneratorStringField(*receiver, "out").Text
	receiver.Fields["out"] = String(current + text)
}

func jsonGeneratorStack(receiver Value) Value {
	if stack, ok := receiver.Fields["stack"]; ok && stack.Kind == ValueList {
		return stack
	}
	return List()
}

func jsonGeneratorPretty(receiver Value) bool {
	return jsonGeneratorBoolField(receiver, "pretty").Bool
}

func jsonGeneratorStringField(receiver Value, field string) Value {
	if value, ok := receiver.Fields[field]; ok && value.Kind == ValueString {
		return value
	}
	return String("")
}

func jsonGeneratorBoolField(receiver Value, field string) Value {
	if value, ok := receiver.Fields[field]; ok && value.Kind == ValueBool {
		return value
	}
	return Bool(false)
}

func jsonGeneratorIntField(receiver Value, field string) Value {
	if value, ok := receiver.Fields[field]; ok && value.Kind == ValueInt {
		return value
	}
	return Int(0)
}

func jsonGeneratorIsNumber(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueDecimal
}

func jsonGeneratorStringArgument(args []Value, index int) (Value, bool) {
	if index < 0 || index >= len(args) {
		return Null, false
	}
	value := args[index]
	if value.Kind == ValueString {
		return value, true
	}
	if jsonGeneratorIsPlatformScalar(value, "Id") {
		raw, err := platformScalarText(value, "Id")
		if err != nil {
			return Null, false
		}
		return String(raw), true
	}
	return Null, false
}

func jsonGeneratorIsPlatformScalar(value Value, typeName string) bool {
	return value.Kind == ValueObject && value.Type == typeName
}

func jsonGeneratorIsID(value Value) bool {
	if value.Kind == ValueString {
		return validateApexID(value.Text) == nil
	}
	return jsonGeneratorIsPlatformScalar(value, "Id")
}
