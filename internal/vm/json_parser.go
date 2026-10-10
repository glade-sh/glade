package vm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type jsonParserFrame struct {
	kind          string
	expectingName bool
	currentName   string
	valueName     string
	line, column  int
}

func newJSONParser(text string) (Value, error) {
	tokens, err := jsonParserTokenize(text)
	if err != nil {
		return Null, jsonParserException("%s", jsonParserInputErrorMessage(err, text))
	}
	parser := Object("JSONParser")
	parser.Fields["tokens"] = List(tokens...)
	parser.Fields["index"] = Int(-1)
	parser.Fields["cleared"] = Bool(false)
	parser.Fields["lastClearedToken"] = Null
	parser.Fields["source"] = String(text)
	return parser, nil
}

func (vm *VM) callJSONParserMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if receiver.Type != "JSONParser" {
		return Null, receiver, false, false, nil
	}
	method = canonicalJSONParserMethod(method)
	switch method {
	case "nextToken":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.nextToken expects 0 arguments")
		}
		return jsonParserNextToken(receiver)
	case "nextValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.nextValue expects 0 arguments")
		}
		return jsonParserNextValue(receiver)
	case "getCurrentToken":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getCurrentToken expects 0 arguments")
		}
		token, ok := jsonParserCurrent(receiver)
		if !ok {
			return Null, receiver, false, true, nil
		}
		return jsonTokenValue(jsonParserTokenKind(token)), receiver, false, true, nil
	case "hasCurrentToken":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.hasCurrentToken expects 0 arguments")
		}
		_, ok := jsonParserCurrent(receiver)
		return Bool(ok), receiver, false, true, nil
	case "getLastClearedToken":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getLastClearedToken expects 0 arguments")
		}
		token, ok := receiver.Fields["lastClearedToken"]
		if !ok || token.Kind == ValueNull {
			return Null, receiver, false, true, nil
		}
		return jsonTokenValue(jsonParserTokenKind(token)), receiver, false, true, nil
	case "getText":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getText expects 0 arguments")
		}
		token, ok := jsonParserCurrent(receiver)
		if !ok {
			return Null, receiver, false, true, nil
		}
		return String(jsonParserTokenText(token)), receiver, false, true, nil
	case "getCurrentName":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getCurrentName expects 0 arguments")
		}
		token, ok := jsonParserCurrent(receiver)
		if !ok {
			if cleared, ok := jsonParserCurrentTokenEvenIfCleared(receiver); ok {
				if name := jsonParserTokenName(cleared); name != "" {
					return String(name), receiver, false, true, nil
				}
			}
			return Null, receiver, false, true, nil
		}
		if jsonParserTokenKind(token) == "FIELD_NAME" {
			return String(jsonParserTokenText(token)), receiver, false, true, nil
		}
		if name := jsonParserTokenName(token); name != "" {
			return String(name), receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getIntegerValue", "getLongValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.%s expects 0 arguments", method)
		}
		value, err := jsonParserIntegerValue(receiver, method)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return value, receiver, false, true, nil
	case "getDecimalValue", "getDoubleValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.%s expects 0 arguments", method)
		}
		value, err := jsonParserDecimalValue(receiver, method)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return value, receiver, false, true, nil
	case "getBooleanValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getBooleanValue expects 0 arguments")
		}
		value, err := jsonParserBooleanValue(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return value, receiver, false, true, nil
	case "getDateValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getDateValue expects 0 arguments")
		}
		text, err := jsonParserStringValue(receiver, "getDateValue")
		if err != nil {
			return Null, receiver, false, true, err
		}
		if _, err := time.Parse("2006-01-02", text); err != nil {
			parts := strings.Split(text, "-")
			if len(parts) == 3 {
				year, yearErr := strconv.Atoi(parts[0])
				month, monthErr := strconv.Atoi(parts[1])
				day, dayErr := strconv.Atoi(parts[2])
				if yearErr == nil && monthErr == nil && dayErr == nil && month >= 1 && month <= 12 {
					lastDay := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
					if day < 1 || day > lastDay {
						return Null, receiver, false, true, jsonParserException("Cannot parse %q: Value %d for dayOfMonth must be in the range [1,%d]", text, day, lastDay)
					}
				}
			}
			return Null, receiver, false, true, jsonParserException("JSONParser.getDateValue cannot parse Date %q", text)
		}
		return platformScalar("Date", text), receiver, false, true, nil
	case "getDatetimeValue", "getDateTimeValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.%s expects 0 arguments", method)
		}
		text, err := jsonParserStringValue(receiver, method)
		if err != nil {
			return Null, receiver, false, true, err
		}
		value, err := parseDatetimeTextAllowDateOnly(text)
		if err != nil {
			return Null, receiver, false, true, jsonParserException("JSONParser.%s cannot parse Datetime %q: %v", method, text, err)
		}
		return platformScalar("Datetime", value.UTC().Format(time.RFC3339Nano)), receiver, false, true, nil
	case "getTimeValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getTimeValue expects 0 arguments")
		}
		text, err := jsonParserStringValue(receiver, "getTimeValue")
		if err != nil {
			return Null, receiver, false, true, err
		}
		value, err := parseTimeText(text)
		if err != nil {
			return Null, receiver, false, true, jsonParserException("JSONParser.getTimeValue cannot parse Time %q: %v", text, err)
		}
		return platformScalar("Time", value), receiver, false, true, nil
	case "getIdValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getIdValue expects 0 arguments")
		}
		text, err := jsonParserStringValue(receiver, "getIdValue")
		if err != nil {
			return Null, receiver, false, true, err
		}
		if err := validateApexID(text); err != nil {
			return Null, receiver, false, true, newExceptionError("System.StringException", strings.TrimPrefix(err.Error(), "System.StringException: "))
		}
		return platformScalar("Id", text), receiver, false, true, nil
	case "getBlobValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.getBlobValue expects 0 arguments")
		}
		text, err := jsonParserStringValue(receiver, "getBlobValue")
		if err != nil {
			return Null, receiver, false, true, err
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return Null, receiver, false, true, jsonParserException("JSONParser.getBlobValue cannot decode base64: %v", err)
		}
		return platformScalar("Blob", string(decoded)), receiver, false, true, nil
	case "skipChildren":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.skipChildren expects 0 arguments")
		}
		updated, err := jsonParserSkipChildren(receiver)
		return Null, updated, true, true, err
	case "clearCurrentToken":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.clearCurrentToken expects 0 arguments")
		}
		updated := receiver
		if token, ok := jsonParserCurrentTokenEvenIfCleared(receiver); ok {
			updated.Fields["cleared"] = Bool(true)
			updated.Fields["lastClearedToken"] = token
		}
		return Null, updated, true, true, nil
	case "readValueAs", "readValueAsStrict":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("JSONParser.%s expects Type", method)
		}
		value, updated, err := vm.jsonParserReadValueAs(receiver, args[0], method == "readValueAsStrict")
		return value, updated, true, true, err
	default:
		return Null, receiver, false, false, nil
	}
}

func canonicalJSONParserMethod(method string) string {
	return canonicalStdlibMemberName(method,
		"nextToken", "nextValue", "getCurrentToken", "hasCurrentToken", "getLastClearedToken", "getText", "getCurrentName",
		"getIntegerValue", "getLongValue", "getDecimalValue", "getDoubleValue", "getBooleanValue",
		"getDateValue", "getDatetimeValue", "getDateTimeValue", "getTimeValue", "getIdValue", "getBlobValue",
		"skipChildren", "clearCurrentToken", "readValueAs", "readValueAsStrict",
	)
}

func (vm *VM) jsonParserReadValueAs(receiver Value, typeArg Value, strict bool) (Value, Value, error) {
	typeName := typeValueName(typeArg)
	_, recordType := vm.explicitSchemaRecordType(typeValueIdentityName(typeArg))
	if typeName == "" {
		return Null, receiver, fmt.Errorf("JSONParser.readValueAs expects Type")
	}
	source, ok := receiver.Fields["source"]
	if !ok || source.Kind != ValueString {
		return Null, receiver, jsonParserException("JSONParser.readValueAs missing parser source")
	}
	index := jsonParserIndex(receiver)
	var raw any
	var err error
	if index >= 0 {
		tokens := jsonParserTokens(receiver)
		var next int64
		raw, next, err = jsonParserRawValueAt(tokens.List, index, source.Text)
		if err != nil {
			return Null, receiver, jsonDeserializeException("JSONParser.readValueAs invalid JSON input: %v", err)
		}
		receiver.Fields["index"] = Int(next - 1)
	} else {
		raw, err = decodeJSONValueForDeserialize(source.Text, false)
		if err != nil {
			return Null, receiver, jsonDeserializeException("JSONParser.readValueAs invalid JSON input: %v", err)
		}
		tokens := jsonParserTokens(receiver)
		if len(tokens.List) > 0 {
			receiver.Fields["index"] = Int(int64(len(tokens.List) - 1))
		}
	}
	value, err := vm.typedValueFromJSON(typeName, jsonTypedInput{value: raw, sObjectRecord: recordType}, strict)
	if err == nil {
		receiver.Fields["cleared"] = Bool(true)
	}
	return value, receiver, err
}

func jsonParserRawValueAt(tokens []Value, index int64, source string) (any, int64, error) {
	if index < 0 || index >= int64(len(tokens)) {
		return nil, index, fmt.Errorf("JSONParser.readValueAs requires a current token")
	}
	token := tokens[index]
	switch kind := jsonParserTokenKind(token); kind {
	case "START_OBJECT":
		out := orderedJSONObject{}
		i := index + 1
		for i < int64(len(tokens)) {
			current := tokens[i]
			switch jsonParserTokenKind(current) {
			case "END_OBJECT":
				return out, i + 1, nil
			case "FIELD_NAME":
				name := jsonParserTokenText(current)
				value, next, err := jsonParserRawValueAt(tokens, i+1, source)
				if err != nil {
					return nil, index, err
				}
				out = append(out, orderedJSONField{name: name, value: value, source: source,
					start: int(current.Fields["start"].Int), end: int(tokens[next-1].Fields["end"].Int)})
				i = next
			default:
				return nil, index, fmt.Errorf("JSONParser.readValueAs expected object field name, got %s", jsonParserTokenKind(current))
			}
		}
		return nil, index, fmt.Errorf("JSONParser.readValueAs object has no end token")
	case "START_ARRAY":
		out := []any{}
		i := index + 1
		for i < int64(len(tokens)) {
			if jsonParserTokenKind(tokens[i]) == "END_ARRAY" {
				return out, i + 1, nil
			}
			value, next, err := jsonParserRawValueAt(tokens, i, source)
			if err != nil {
				return nil, index, err
			}
			out = append(out, value)
			i = next
		}
		return nil, index, fmt.Errorf("JSONParser.readValueAs array has no end token")
	case "VALUE_STRING":
		return jsonParserTokenText(token), index + 1, nil
	case "VALUE_NUMBER_INT", "VALUE_NUMBER_FLOAT":
		return json.Number(jsonParserTokenText(token)), index + 1, nil
	case "VALUE_TRUE":
		return true, index + 1, nil
	case "VALUE_FALSE":
		return false, index + 1, nil
	case "VALUE_NULL":
		return nil, index + 1, nil
	default:
		return nil, index, fmt.Errorf("JSONParser.readValueAs cannot read %s", kind)
	}
}

func jsonParserTokenize(text string) ([]Value, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var tokens []Value
	var stack []jsonParserFrame
	rootWritten := false
	for {
		start := jsonTokenInputStart(text, int(decoder.InputOffset()))
		raw, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, jsonSyntaxErrorAtInput(err, text, int(decoder.InputOffset()))
		}
		switch token := raw.(type) {
		case json.Delim:
			switch token {
			case '{':
				name, err := jsonParserBeforeValue(&stack, &rootWritten)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, jsonParserToken("START_OBJECT", "{", name, ""))
				line, column := jsonInputPosition(text, start)
				stack = append(stack, jsonParserFrame{kind: "object", expectingName: true, currentName: name, valueName: name, line: line, column: column - 1})
			case '[':
				name, err := jsonParserBeforeValue(&stack, &rootWritten)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, jsonParserToken("START_ARRAY", "[", name, ""))
				line, column := jsonInputPosition(text, start)
				stack = append(stack, jsonParserFrame{kind: "array", currentName: name, valueName: name, line: line, column: column - 1})
			case '}':
				if len(stack) == 0 || stack[len(stack)-1].kind != "object" {
					return nil, fmt.Errorf("JSONParser encountered unmatched object end")
				}
				name := stack[len(stack)-1].valueName
				stack = stack[:len(stack)-1]
				tokens = append(tokens, jsonParserToken("END_OBJECT", "}", name, ""))
			case ']':
				if len(stack) == 0 || stack[len(stack)-1].kind != "array" {
					return nil, fmt.Errorf("JSONParser encountered unmatched array end")
				}
				name := stack[len(stack)-1].valueName
				stack = stack[:len(stack)-1]
				tokens = append(tokens, jsonParserToken("END_ARRAY", "]", name, ""))
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1].kind == "object" && stack[len(stack)-1].expectingName {
				stack[len(stack)-1].expectingName = false
				stack[len(stack)-1].currentName = token
				tokens = append(tokens, jsonParserToken("FIELD_NAME", token, token, ""))
				jsonParserSetTokenLocation(&tokens[len(tokens)-1], start, int(decoder.InputOffset()))
				continue
			}
			name, err := jsonParserBeforeValue(&stack, &rootWritten)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, jsonParserToken("VALUE_STRING", token, name, ""))
		case json.Number:
			end := int(decoder.InputOffset())
			if strings.TrimPrefix(token.String(), "-") == "0" && end < len(text) && text[end] >= '0' && text[end] <= '9' {
				line, column := jsonInputPosition(text, end)
				return nil, jsonParserException("Invalid numeric value: Leading zeroes not allowed at input location [%d,%d]", line, column)
			}
			name, err := jsonParserBeforeValue(&stack, &rootWritten)
			if err != nil {
				return nil, err
			}
			kind := "VALUE_NUMBER_INT"
			if strings.ContainsAny(token.String(), ".eE") {
				kind = "VALUE_NUMBER_FLOAT"
			}
			value := jsonParserToken(kind, token.String(), name, "number")
			tokens = append(tokens, value)
		case bool:
			name, err := jsonParserBeforeValue(&stack, &rootWritten)
			if err != nil {
				return nil, err
			}
			if token {
				tokens = append(tokens, jsonParserToken("VALUE_TRUE", "true", name, ""))
			} else {
				tokens = append(tokens, jsonParserToken("VALUE_FALSE", "false", name, ""))
			}
		case nil:
			name, err := jsonParserBeforeValue(&stack, &rootWritten)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, jsonParserToken("VALUE_NULL", "null", name, ""))
		}
		jsonParserSetTokenLocation(&tokens[len(tokens)-1], start, int(decoder.InputOffset()))
		if rootWritten && len(stack) == 0 {
			break
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("JSONParser input is empty")
	}
	if len(stack) != 0 {
		frame := stack[len(stack)-1]
		line, column := jsonEOFPosition(text, false)
		return nil, jsonParserException("Unexpected end-of-input: expected close marker for %s (from [Source: java.io.StringReader@0; line: %d, column: %d]) at input location [%d,%d]", strings.ToUpper(frame.kind), frame.line, frame.column, line, column)
	}
	return tokens, nil
}

func jsonParserInputErrorMessage(err error, source string) string {
	if thrown, ok := err.(*apexThrowError); ok {
		return thrown.value.Fields["message"].Text
	}
	converted := jsonUnexpectedCharacterError(err, source)
	if character, ok := converted.(*jsonCharacterInputError); ok {
		return fmt.Sprintf("Unexpected character ('%c' (code %d)): %s at input location [%d,%d]", character.char, character.char, jsonExpectedValueDescription(character.char), character.line, character.column)
	}
	if quoted, ok := jsonStringEOFError(err, source).(*jsonStringInputError); ok {
		message := quoted.Error()
		at := strings.LastIndex(message, " at [line:")
		return message[:at] + fmt.Sprintf(" at input location [%d,%d]", quoted.line, quoted.column)
	}
	return fmt.Sprintf("JSONParser invalid JSON input: %v", err)
}

func jsonParserSetTokenLocation(token *Value, start, end int) {
	token.Fields["start"], token.Fields["end"] = Int(int64(start)), Int(int64(end))
}

func jsonParserInputLocation(receiver, token Value) (int, int) {
	source := receiver.Fields["source"].Text
	start, end := int(token.Fields["start"].Int), int(token.Fields["end"].Int)
	switch jsonParserTokenKind(token) {
	case "VALUE_NUMBER_INT", "VALUE_NUMBER_FLOAT", "VALUE_TRUE", "VALUE_FALSE", "VALUE_NULL":
		return jsonNumericInputPosition(source, end)
	case "VALUE_STRING":
		return jsonInputPosition(source, start+1)
	}
	return jsonInputPosition(source, start)
}

func jsonParserBeforeValue(stack *[]jsonParserFrame, rootWritten *bool) (string, error) {
	if len(*stack) == 0 {
		if *rootWritten {
			return "", fmt.Errorf("JSONParser input contains multiple root values")
		}
		*rootWritten = true
		return "", nil
	}
	top := &(*stack)[len(*stack)-1]
	switch top.kind {
	case "array":
		return top.currentName, nil
	case "object":
		if top.expectingName {
			return "", fmt.Errorf("JSONParser object value is missing a field name")
		}
		name := top.currentName
		top.expectingName = true
		return name, nil
	default:
		return "", fmt.Errorf("JSONParser has invalid internal frame")
	}
}

func jsonParserToken(kind, text, name, number string) Value {
	token := Object("JSONParser.Token")
	token.Fields["kind"] = String(kind)
	token.Fields["text"] = String(text)
	token.Fields["name"] = String(name)
	token.Fields["number"] = String(number)
	return token
}

func jsonParserNextToken(receiver Value) (Value, Value, bool, bool, error) {
	updated := receiver
	next := jsonParserIndex(updated) + 1
	tokens := jsonParserTokens(updated)
	if next >= int64(len(tokens.List)) {
		updated.Fields["index"] = Int(int64(len(tokens.List)))
		updated.Fields["cleared"] = Bool(false)
		return Null, updated, true, true, nil
	}
	updated.Fields["index"] = Int(next)
	updated.Fields["cleared"] = Bool(false)
	return jsonTokenValue(jsonParserTokenKind(tokens.List[next])), updated, true, true, nil
}

func jsonParserNextValue(receiver Value) (Value, Value, bool, bool, error) {
	value, updated, changed, handled, err := jsonParserNextToken(receiver)
	if err != nil || value.Kind == ValueNull {
		return value, updated, changed, handled, err
	}
	token, ok := jsonParserCurrent(updated)
	if ok && jsonParserTokenKind(token) == "FIELD_NAME" {
		return jsonParserNextToken(updated)
	}
	return value, updated, changed, handled, nil
}

func jsonParserSkipChildren(receiver Value) (Value, error) {
	token, err := jsonParserRequireCurrent(receiver, "skipChildren")
	if err != nil {
		return receiver, err
	}
	kind := jsonParserTokenKind(token)
	if kind != "START_OBJECT" && kind != "START_ARRAY" {
		return receiver, nil
	}
	index := jsonParserIndex(receiver)
	tokens := jsonParserTokens(receiver)
	depth := int64(0)
	for i := index; i < int64(len(tokens.List)); i++ {
		switch jsonParserTokenKind(tokens.List[i]) {
		case "START_OBJECT", "START_ARRAY":
			depth++
		case "END_OBJECT", "END_ARRAY":
			depth--
			if depth == 0 {
				updated := receiver
				updated.Fields["index"] = Int(i)
				return updated, nil
			}
		}
	}
	return receiver, jsonParserException("JSONParser.skipChildren could not find matching end token")
}

func jsonParserIntegerValue(receiver Value, method string) (Value, error) {
	token, err := jsonParserRequireNumeric(receiver)
	if err != nil {
		return Null, err
	}
	value, valid := jsonTruncatedIntegralNumber(json.Number(jsonParserTokenText(token)))
	if !valid {
		return Null, jsonParserException("JSONParser.%s cannot parse integer", method)
	}
	if strings.EqualFold(method, "getLongValue") {
		return longIntValue(value), nil
	}
	if value < -2147483648 || value > 2147483647 {
		line, column := jsonParserInputLocation(receiver, token)
		return Null, jsonParserException("Numeric value (%s) out of range of int at input location [%d,%d]", jsonParserTokenText(token), line, column)
	}
	return Int(value), nil
}

func jsonParserDecimalValue(receiver Value, method string) (Value, error) {
	token, err := jsonParserRequireNumeric(receiver)
	if err != nil {
		return Null, err
	}
	text := jsonParserTokenText(token)
	if strings.EqualFold(method, "getDoubleValue") {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return Null, jsonParserException("JSONParser.%s cannot parse decimal: %v", method, err)
		}
		return decimalAsDouble(Decimal(value)), nil
	}
	decimal, err := decimalFromText(text)
	if err != nil {
		return Null, jsonParserException("JSONParser.%s cannot parse decimal: %v", method, err)
	}
	return decimal, nil
}

func jsonParserBooleanValue(receiver Value) (Value, error) {
	token, err := jsonParserRequireCurrent(receiver, "getBooleanValue")
	if err != nil {
		return Null, err
	}
	switch jsonParserTokenKind(token) {
	case "VALUE_TRUE":
		return Bool(true), nil
	case "VALUE_FALSE":
		return Bool(false), nil
	default:
		line, column := jsonParserInputLocation(receiver, token)
		return Null, jsonParserException("Current token (%s) not of boolean type at input location [%d,%d]", jsonParserTokenKind(token), line, column)
	}
}

func jsonParserRequireNumeric(receiver Value) (Value, error) {
	token, current := jsonParserCurrent(receiver)
	if current {
		kind := jsonParserTokenKind(token)
		if kind == "VALUE_NUMBER_INT" || kind == "VALUE_NUMBER_FLOAT" {
			return token, nil
		}
	}
	kind, line, column := "null", 1, 1
	if current {
		kind = jsonParserTokenKind(token)
		line, column = jsonParserInputLocation(receiver, token)
	}
	return Null, jsonParserException("Current token (%s) not numeric, can not use numeric value accessors at input location [%d,%d]", kind, line, column)
}

func jsonParserStringValue(receiver Value, method string) (string, error) {
	token, err := jsonParserRequireCurrent(receiver, method)
	if err != nil {
		return "", err
	}
	if jsonParserTokenKind(token) != "VALUE_STRING" {
		return "", jsonParserException("JSONParser.%s requires VALUE_STRING", method)
	}
	return jsonParserTokenText(token), nil
}

func jsonParserRequireCurrent(receiver Value, method string) (Value, error) {
	token, ok := jsonParserCurrent(receiver)
	if !ok {
		return Null, jsonParserException("JSONParser.%s requires a current token", method)
	}
	return token, nil
}

func jsonParserException(format string, args ...any) error {
	return newExceptionError("JSONException", fmt.Sprintf(format, args...))
}

func jsonParserCurrent(receiver Value) (Value, bool) {
	if jsonParserBoolField(receiver, "cleared").Bool {
		return Null, false
	}
	return jsonParserCurrentTokenEvenIfCleared(receiver)
}

func jsonParserCurrentTokenEvenIfCleared(receiver Value) (Value, bool) {
	index := jsonParserIndex(receiver)
	tokens := jsonParserTokens(receiver)
	if index < 0 || index >= int64(len(tokens.List)) {
		return Null, false
	}
	return tokens.List[index], true
}

func jsonParserTokens(receiver Value) Value {
	if tokens, ok := receiver.Fields["tokens"]; ok && tokens.Kind == ValueList {
		return tokens
	}
	return List()
}

func jsonParserIndex(receiver Value) int64 {
	if index, ok := receiver.Fields["index"]; ok && index.Kind == ValueInt {
		return index.Int
	}
	return -1
}

func jsonParserBoolField(receiver Value, field string) Value {
	if value, ok := receiver.Fields[field]; ok && value.Kind == ValueBool {
		return value
	}
	return Bool(false)
}

func jsonParserTokenKind(token Value) string {
	return jsonParserTokenStringField(token, "kind")
}

func jsonParserTokenText(token Value) string {
	return jsonParserTokenStringField(token, "text")
}

func jsonParserTokenName(token Value) string {
	return jsonParserTokenStringField(token, "name")
}

func jsonParserTokenStringField(token Value, field string) string {
	if token.Kind != ValueObject {
		return ""
	}
	if value, ok := token.Fields[field]; ok && value.Kind == ValueString {
		return value.Text
	}
	return ""
}

func jsonTokenValue(name string) Value {
	return Value{Kind: ValueObject, Type: "JSONToken", Text: name}
}

func canonicalJSONTokenName(name string) (string, bool) {
	for _, candidate := range jsonTokenNames {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
}
