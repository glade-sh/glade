package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/storage"
)

func jsonFromValue(value Value, suppressObjectNulls bool) any {
	switch value.Kind {
	case ValueNull:
		return nil
	case ValueInt:
		return value.Int
	case ValueDecimal:
		if strings.TrimSpace(value.Text) != "" {
			return json.Number(value.Text)
		}
		return value.Decimal
	case ValueBool:
		return value.Bool
	case ValueString:
		return value.Text
	case ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, jsonFromValue(item, suppressObjectNulls))
		}
		return out
	case ValueSet:
		out := make([]any, 0, len(value.Set))
		for _, item := range value.Set {
			out = append(out, jsonFromValue(item, suppressObjectNulls))
		}
		return out
	case ValueMap:
		if len(value.MapOrder) > 0 {
			out := orderedJSONObject{}
			seen := map[string]bool{}
			for _, key := range orderedJSONMapKeys(value) {
				item, ok := value.Map[key]
				if !ok || seen[key] {
					continue
				}
				out = append(out, orderedJSONField{name: mapStoredKey(value, key).String(), value: jsonFromValue(item, suppressObjectNulls)})
				seen[key] = true
			}
			for _, key := range sortedMapKeys(value.Map) {
				if seen[key] {
					continue
				}
				out = append(out, orderedJSONField{name: mapStoredKey(value, key).String(), value: jsonFromValue(value.Map[key], suppressObjectNulls)})
			}
			return out
		}
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			out[mapStoredKey(value, key).String()] = jsonFromValue(item, suppressObjectNulls)
		}
		return out
	case ValueObject:
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar
		}
		return jsonSObjectFromValue(value, suppressObjectNulls, jsonFromValue, storage.DefaultRESTAPIVersion)
	default:
		return nil
	}
}

func (vm *VM) jsonFromValueForSerialize(value Value, suppressObjectNulls bool) any {
	switch value.Kind {
	case ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, vm.jsonFromValueForSerialize(item, suppressObjectNulls))
		}
		return out
	case ValueSet:
		out := make([]any, 0, len(value.Set))
		for _, item := range value.Set {
			out = append(out, vm.jsonFromValueForSerialize(item, suppressObjectNulls))
		}
		return out
	case ValueMap:
		if len(value.MapOrder) > 0 {
			out := orderedJSONObject{}
			seen := map[string]bool{}
			for _, key := range orderedJSONMapKeys(value) {
				item, ok := value.Map[key]
				if !ok || seen[key] {
					continue
				}
				name := mapStoredKey(value, key).String()
				out = append(out, orderedJSONField{name: name, value: vm.jsonFromValueForSerialize(item, suppressObjectNulls)})
				seen[key] = true
			}
			for _, key := range sortedMapKeys(value.Map) {
				if seen[key] {
					continue
				}
				out = append(out, orderedJSONField{name: mapStoredKey(value, key).String(), value: vm.jsonFromValueForSerialize(value.Map[key], suppressObjectNulls)})
			}
			return out
		}
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			out[mapStoredKey(value, key).String()] = vm.jsonFromValueForSerialize(item, suppressObjectNulls)
		}
		return out
	case ValueObject:
		if vm.isEnumObjectValue(value) {
			return value.Text
		}
		if strings.EqualFold(value.Type, "Datetime") || strings.EqualFold(value.Type, "DateTime") {
			if t, err := parsePlatformDatetime(value); err == nil {
				return t.UTC().Format("2006-01-02T15:04:05.000Z")
			}
		}
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar
		}
		if value.Type == "" || sObjectValueType(value.Type) {
			version := storage.DefaultRESTAPIVersion
			if vm != nil && vm.Org != nil {
				version = storage.EffectiveRESTAPIVersion(vm.Org.APIVersion)
			}
			return jsonSObjectFromValue(value, suppressObjectNulls, vm.jsonFromValueForSerialize, version)
		}
		base := orderedJSONObject{}
		seen := map[string]bool{}
		getterNames := vm.jsonSerializableGetterNameSet(value.Type)
		for _, fieldName := range vm.jsonSerializableFieldNames(value.Type) {
			fieldKey := strings.ToLower(fieldName)
			field, owner, fieldOK := vm.lookupField(value.Type, fieldName)
			if fieldOK {
				fieldKey = strings.ToLower(field.Name)
				if jsonFieldIsTransient(field) {
					seen[fieldKey] = true
					continue
				}
				if field.Getter != nil && !field.Static {
					getterValue, err := vm.callGetter(vm.getterOwner(owner, field), field, value)
					if err == nil && !(suppressObjectNulls && getterValue.Kind == ValueNull) {
						base = append(base, orderedJSONField{name: field.Name, value: vm.jsonFromValueForSerialize(getterValue, suppressObjectNulls)})
						seen[fieldKey] = true
					}
					continue
				}
				if _, shadowed := getterNames[fieldKey]; shadowed {
					continue
				}
			}
			actualName, item, ok := objectFieldValue(value, fieldName)
			if !ok {
				continue
			}
			if isInternalSObjectField(actualName) || (suppressObjectNulls && item.Kind == ValueNull) {
				continue
			}
			base = append(base, orderedJSONField{name: actualName, value: vm.jsonFromValueForSerialize(item, suppressObjectNulls)})
			seen[strings.ToLower(actualName)] = true
		}
		var extras []string
		for field := range value.Fields {
			fieldKey := strings.ToLower(field)
			if isInternalSObjectField(field) || seen[fieldKey] {
				continue
			}
			if classField, _, ok := vm.lookupField(value.Type, field); ok && jsonFieldIsTransient(classField) {
				continue
			}
			extras = append(extras, field)
		}
		sort.Strings(extras)
		for _, field := range extras {
			item := value.Fields[field]
			if suppressObjectNulls && item.Kind == ValueNull {
				continue
			}
			base = append(base, orderedJSONField{name: field, value: vm.jsonFromValueForSerialize(item, suppressObjectNulls)})
			seen[strings.ToLower(field)] = true
		}
		for _, field := range vm.jsonSerializableGetterFields(value.Type) {
			if field.Getter == nil || field.Static {
				continue
			}
			name := field.Name
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			getterValue, err := vm.callGetter(vm.getterOwner(value.Type, field), field, value)
			if err != nil || (suppressObjectNulls && getterValue.Kind == ValueNull) {
				continue
			}
			base = append(base, orderedJSONField{name: name, value: vm.jsonFromValueForSerialize(getterValue, suppressObjectNulls)})
			seen[strings.ToLower(name)] = true
		}
		return base
	default:
		return jsonFromValue(value, suppressObjectNulls)
	}
}

func jsonFieldIsTransient(field Field) bool {
	for _, modifier := range field.Modifiers {
		if strings.EqualFold(modifier, "transient") {
			return true
		}
	}
	return false
}

func (vm *VM) isEnumObjectValue(value Value) bool {
	if value.Kind != ValueObject || strings.TrimSpace(value.Text) == "" {
		return false
	}
	switch {
	case strings.EqualFold(value.Type, "Schema.DisplayType"),
		strings.EqualFold(value.Type, "DisplayType"),
		strings.EqualFold(value.Type, "Schema.SOAPType"),
		strings.EqualFold(value.Type, "SOAPType"),
		strings.EqualFold(value.Type, "LoggingLevel"),
		strings.EqualFold(value.Type, "RoundingMode"),
		strings.EqualFold(value.Type, "AccessType"),
		strings.EqualFold(value.Type, "TriggerOperation"),
		strings.EqualFold(value.Type, "StatusCode"),
		strings.EqualFold(value.Type, "Metadata.DeployStatus"),
		strings.EqualFold(value.Type, "Metadata.MetadataType"):
		return true
	}
	if generated, ok := generatedPlatformTypes()[strings.ToLower(value.Type)]; ok && generated.Kind == apexast.DeclarationEnum {
		return true
	}
	if _, ok := vm.resolveEnumClass(value.Type); ok {
		return true
	}
	return false
}

func formatSalesforcePrettyJSON(data []byte) string {
	var out bytes.Buffer
	out.Grow(len(data))
	inString := false
	escaped := false
	for _, b := range data {
		if inString {
			out.WriteByte(b)
			if escaped {
				escaped = false
				continue
			}
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
			out.WriteByte(b)
		case ':':
			if out.Len() > 0 && out.Bytes()[out.Len()-1] != ' ' {
				out.WriteByte(' ')
			}
			out.WriteByte(b)
		default:
			out.WriteByte(b)
		}
	}
	return collapseSalesforcePrettyPrimitiveArrays(out.String())
}

func collapseSalesforcePrettyPrimitiveArrays(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if !strings.HasSuffix(trimmed, "[") {
			out = append(out, line)
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		items := make([]string, 0)
		j := i + 1
		for ; j < len(lines); j++ {
			itemLine := strings.TrimSpace(lines[j])
			if itemLine == "]" || itemLine == "]," {
				trailingComma := ""
				if strings.HasSuffix(itemLine, ",") {
					trailingComma = ","
				}
				if len(items) == 0 {
					out = append(out, indent+strings.TrimSuffix(trimmed, "[")+"[]"+trailingComma)
				} else {
					out = append(out, indent+strings.TrimSuffix(trimmed, "[")+"[ "+strings.Join(items, ", ")+" ]"+trailingComma)
				}
				i = j
				break
			}
			item := strings.TrimSuffix(itemLine, ",")
			if !salesforcePrettyPrimitiveArrayItem(item) {
				break
			}
			items = append(items, item)
		}
		if j >= len(lines) || (j < len(lines) && strings.TrimSpace(lines[j]) != "]" && strings.TrimSpace(lines[j]) != "],") {
			out = append(out, line)
			continue
		}
	}
	return strings.Join(out, "\n")
}

func salesforcePrettyPrimitiveArrayItem(item string) bool {
	if item == "null" || item == "true" || item == "false" {
		return true
	}
	if item == "" || strings.ContainsAny(item, "{}[]") {
		return false
	}
	if item[0] == '"' {
		return len(item) >= 2 && item[len(item)-1] == '"'
	}
	if _, err := strconv.ParseFloat(item, 64); err == nil {
		return true
	}
	return false
}

type orderedJSONField struct {
	name  string
	value any
}

type orderedJSONObject []orderedJSONField

func (object orderedJSONObject) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for i, field := range object {
		if i > 0 {
			out.WriteByte(',')
		}
		name, err := jsonMarshalNoEscape(field.name)
		if err != nil {
			return nil, err
		}
		value, err := jsonMarshalNoEscape(field.value)
		if err != nil {
			return nil, err
		}
		out.Write(name)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

func jsonMarshalNoEscape(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	data := out.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return data, nil
}

func jsonMarshalNoEscapeIndent(value any, prefix, indent string) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	data := out.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return data, nil
}

func jsonSObjectFromValue(value Value, suppressObjectNulls bool, convert func(Value, bool) any, apiVersion string) any {
	out := orderedJSONObject{}
	if value.Type != "" {
		attributes := map[string]any{"type": value.Type}
		if id, ok := value.Fields["Id"]; ok && !strings.Contains(value.Type, ".") {
			if idText, ok := idValueText(id); ok && idText != "" {
				attributes["url"] = "/services/data/v" + storage.EffectiveRESTAPIVersion(apiVersion) + "/sobjects/" + value.Type + "/" + displayIDText(idText)
			}
		}
		out = append(out, orderedJSONField{name: "attributes", value: attributes})
	}
	for _, field := range jsonSObjectFieldNames(value) {
		item := value.Fields[field]
		if suppressObjectNulls && item.Kind == ValueNull {
			continue
		}
		out = append(out, orderedJSONField{name: field, value: convert(item, suppressObjectNulls)})
	}
	return out
}

func jsonSObjectFieldNames(value Value) []string {
	regular := make([]string, 0, len(value.Fields))
	system := make([]string, 0, 8)
	for field, item := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		if isImplicitFalseIsDeleted(value, field, item) {
			continue
		}
		if isImplicitGeneratedSystemField(value, field) {
			continue
		}
		if jsonGeneratedSystemField(field) {
			system = append(system, field)
			continue
		}
		regular = append(regular, field)
	}
	sort.Strings(regular)
	sort.Strings(system)
	return append(regular, system...)
}

func jsonGeneratedSystemField(field string) bool {
	switch field {
	case "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SetupOwnerId", "SystemModstamp":
		return true
	default:
		return false
	}
}

func isImplicitGeneratedSystemField(value Value, field string) bool {
	if !jsonGeneratedSystemField(field) {
		return false
	}
	for _, explicit := range explicitSObjectFieldNames(value) {
		if explicit == field {
			return false
		}
	}
	return true
}

func isImplicitFalseIsDeleted(value Value, field string, item Value) bool {
	if field != "IsDeleted" || item.Kind != ValueBool || item.Bool {
		return false
	}
	for _, explicit := range explicitSObjectFieldNames(value) {
		if explicit == "IsDeleted" {
			return false
		}
	}
	return true
}

func orderedJSONMapKeys(value Value) []string {
	return reverseMapOrder(value.MapOrder)
}

func jsonObjectMap(raw any) (map[string]any, bool) {
	if fields, ok := raw.(map[string]any); ok {
		return fields, true
	}
	object, ok := raw.(orderedJSONObject)
	if !ok {
		return nil, false
	}
	fields := make(map[string]any, len(object))
	for _, field := range object {
		fields[field.name] = field.value
	}
	return fields, true
}

func jsonObjectFields(raw any) ([]orderedJSONField, bool) {
	if object, ok := raw.(orderedJSONObject); ok {
		out := make([]orderedJSONField, 0, len(object))
		positions := make(map[string]int, len(object))
		for _, field := range object {
			if index, ok := positions[field.name]; ok {
				out[index].value = field.value
				continue
			}
			positions[field.name] = len(out)
			out = append(out, field)
		}
		return out, true
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]orderedJSONField, 0, len(keys))
	for _, key := range keys {
		out = append(out, orderedJSONField{name: key, value: fields[key]})
	}
	return out, true
}

func reverseMapOrder(order []string) []string {
	reversed := make([]string, len(order))
	for i, key := range order {
		reversed[len(reversed)-1-i] = key
	}
	return reversed
}

func (vm *VM) jsonSerializableFieldNames(typeName string) []string {
	var fields []string
	var visit func(string)
	visit = func(name string) {
		class, ok := vm.lookupClass(name)
		if !ok {
			return
		}
		if class.SuperClass != "" {
			visit(class.SuperClass)
		}
		fields = append(fields, class.FieldOrder...)
	}
	visit(typeName)
	return fields
}

func (vm *VM) jsonSerializableGetterNameSet(typeName string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, field := range vm.jsonSerializableGetterFields(typeName) {
		if field.Getter == nil || field.Static || field.Name == "" {
			continue
		}
		out[strings.ToLower(field.Name)] = struct{}{}
	}
	return out
}

func (vm *VM) jsonSerializableGetterFields(typeName string) []Field {
	var fields []Field
	var visit func(string)
	visit = func(name string) {
		class, ok := vm.lookupClass(name)
		if !ok {
			return
		}
		if class.SuperClass != "" {
			visit(class.SuperClass)
		}
		for _, fieldName := range class.FieldOrder {
			field, ok := class.Fields[fieldName]
			if ok && field.Getter != nil {
				fields = append(fields, field)
			}
		}
	}
	visit(typeName)
	return fields
}

func (vm *VM) getterOwner(typeName string, field Field) string {
	if field.Getter == nil {
		return typeName
	}
	if dot := strings.LastIndex(field.Getter.Name, "."); dot > 0 {
		return field.Getter.Name[:dot]
	}
	return typeName
}

func decodeJSONValue(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	return decodeJSONToken(decoder, text)
}

func decodeJSONToken(decoder *json.Decoder, source string) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			out := orderedJSONObject{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, fmt.Errorf("expected object field name")
				}
				item, err := decodeJSONToken(decoder, source)
				if err != nil {
					return nil, err
				}
				out = append(out, orderedJSONField{name: key, value: item})
			}
			if end, err := decoder.Token(); err != nil {
				return nil, err
			} else if end != json.Delim('}') {
				return nil, fmt.Errorf("expected object end")
			}
			return out, nil
		case '[':
			out := []any{}
			for decoder.More() {
				item, err := decodeJSONToken(decoder, source)
				if err != nil {
					return nil, err
				}
				out = append(out, item)
			}
			if end, err := decoder.Token(); err != nil {
				return nil, err
			} else if end != json.Delim(']') {
				return nil, fmt.Errorf("expected array end")
			}
			return out, nil
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	default:
		return value, nil
	}
}

func decodeJSONUntypedValue(text string) (Value, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeJSONUntypedToken(decoder, text)
	if err != nil {
		return Null, err
	}
	return value, nil
}

func decodeJSONUntypedToken(decoder *json.Decoder, source string) (Value, error) {
	token, err := decoder.Token()
	if err != nil {
		return Null, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			out := Map()
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return Null, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return Null, fmt.Errorf("expected object field name")
				}
				item, err := decodeJSONUntypedToken(decoder, source)
				if err != nil {
					return Null, err
				}
				encoded := mapKey(String(key))
				if _, exists := out.Map[encoded]; !exists {
					out.MapOrder = append(out.MapOrder, encoded)
				}
				out.Map[encoded] = item
				out.MapKeys[encoded] = String(key)
			}
			if end, err := decoder.Token(); err != nil {
				return Null, err
			} else if end != json.Delim('}') {
				return Null, fmt.Errorf("expected object end")
			}
			return out, nil
		case '[':
			out := List()
			for decoder.More() {
				item, err := decodeJSONUntypedToken(decoder, source)
				if err != nil {
					return Null, err
				}
				out.List = append(out.List, item)
			}
			if end, err := decoder.Token(); err != nil {
				return Null, err
			} else if end != json.Delim(']') {
				return Null, fmt.Errorf("expected array end")
			}
			return out, nil
		default:
			return Null, fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	case nil:
		return Null, nil
	case string:
		return String(value), nil
	case bool:
		return Bool(value), nil
	case json.Number:
		if integer, err := strconv.ParseInt(value.String(), 10, 64); err == nil {
			return Int(integer), nil
		}
		if !strings.ContainsAny(value.String(), ".eE") {
			line, column := jsonNumberStartLineColumn(source, decoder.InputOffset(), value.String())
			return Null, &jsonNumberInputError{text: value.String(), line: line, column: column}
		}
		decimal, err := strconv.ParseFloat(value.String(), 64)
		if err != nil {
			return Null, err
		}
		out := Decimal(decimal)
		out.Text = value.String()
		return out, nil
	default:
		return valueFromJSON(value), nil
	}
}

type jsonNumberInputError struct {
	text   string
	line   int
	column int
}

func (err *jsonNumberInputError) Error() string {
	return fmt.Sprintf("For input string: %q at [line:%d, column:%d]", err.text, err.line, err.column)
}

func jsonNumberStartLineColumn(source string, endOffset int64, number string) (int, int) {
	start := int(endOffset) - len(number)
	if start < 0 {
		start = 0
	}
	line := 1
	column := 1
	for i, r := range source {
		if i >= start {
			break
		}
		if r == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func decodeJSONValueForDeserialize(text string, strict bool) (any, error) {
	decoded, err := decodeJSONValue(text)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected EOF") && strings.HasPrefix(strings.TrimSpace(text), `"`) {
			return nil, fmt.Errorf("malformed JSON: %s", err.Error())
		}
		if strings.Contains(err.Error(), "invalid character '\\\\'") && strings.Contains(text, `\"`) {
			decoded, retryErr := decodeJSONValue(strings.ReplaceAll(text, `\"`, `"`))
			if retryErr == nil {
				return decoded, nil
			}
			return nil, normalizeJSONDeserializeError(retryErr)
		}
		return nil, normalizeJSONDeserializeError(err)
	}
	return decoded, nil
}

func normalizeJSONDeserializeError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.Contains(message, "unexpected EOF") || message == "EOF" {
		return fmt.Errorf("Unexpected end-of-input: %s", message)
	}
	if strings.HasPrefix(message, "JSON.deserializeStrict") {
		return err
	}
	if hasPrefixFold(message, "malformed json:") {
		return err
	}
	return fmt.Errorf("malformed JSON: %s", message)
}

func validateJSONNoDuplicateObjectFields(text string) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	return validateJSONValueNoDuplicateObjectFields(decoder)
}

func validateJSONValueNoDuplicateObjectFields(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON.deserializeStrict expected object field name")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON.deserializeStrict found duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := validateJSONValueNoDuplicateObjectFields(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("JSON.deserializeStrict expected object end")
		}
	case '[':
		for decoder.More() {
			if err := validateJSONValueNoDuplicateObjectFields(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("JSON.deserializeStrict expected array end")
		}
	default:
		return fmt.Errorf("JSON.deserializeStrict found unexpected delimiter %q", delim)
	}
	return nil
}

func valueFromJSON(raw any) Value {
	switch v := raw.(type) {
	case nil:
		return Null
	case bool:
		return Bool(v)
	case float64:
		if math.Trunc(v) == v {
			if converted, err := int64FromFloat("JSON number", v); err == nil {
				return Int(converted)
			}
		}
		return Decimal(v)
	case json.Number:
		text := v.String()
		if !strings.ContainsAny(text, ".eE") {
			if converted, err := strconv.ParseInt(text, 10, 64); err == nil {
				return Int(converted)
			}
		}
		if converted, err := strconv.ParseFloat(text, 64); err == nil {
			decimal := Decimal(converted)
			decimal.Text = text
			return decimal
		}
		return String(text)
	case string:
		return String(v)
	case []any:
		out := make([]Value, 0, len(v))
		for _, item := range v {
			out = append(out, valueFromJSON(item))
		}
		return List(out...)
	case map[string]any:
		out := Map()
		for key, item := range v {
			out.Map[mapKey(String(key))] = valueFromJSON(item)
		}
		return out
	case orderedJSONObject:
		out := Map()
		for _, field := range v {
			encoded := mapKey(String(field.name))
			if _, exists := out.Map[encoded]; !exists {
				out.MapOrder = append(out.MapOrder, encoded)
			}
			out.Map[encoded] = valueFromJSON(field.value)
			out.MapKeys[encoded] = String(field.name)
		}
		return out
	default:
		return Null
	}
}

func (vm *VM) typedValueFromJSON(typeName string, raw any, strict bool) (Value, error) {
	if value, ok, err := typedScalarFromJSON(typeName, raw); ok || err != nil {
		return value, err
	}
	typeName = vm.resolveJSONTypeName(typeName)
	if collectionBase(typeName) == "List" {
		items, ok := raw.([]any)
		if !ok {
			if records, recordsOK := jsonQueryResultRecords(raw); recordsOK {
				items = records
			} else {
				return Null, jsonTypeMappingError(typeName, raw)
			}
		}
		elementType, _ := collectionElementType(typeName)
		out := List()
		out.Type = typeName
		for _, item := range items {
			value, err := vm.typedValueFromJSON(elementType, item, strict)
			if err != nil {
				return Null, err
			}
			out.List = append(out.List, value)
		}
		return out, nil
	}
	if collectionBase(typeName) == "Set" {
		items, ok := raw.([]any)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		elementType, _ := collectionElementType(typeName)
		out := Set()
		out.Type = typeName
		for _, item := range items {
			value, err := vm.typedValueFromJSON(elementType, item, strict)
			if err != nil {
				return Null, err
			}
			if !containsValue(out.Set, value) {
				out.Set = append(out.Set, value)
			}
		}
		return out, nil
	}
	if isMapType(typeName) {
		_, ok := jsonObjectMap(raw)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		out := Map()
		out.Type = typeName
		orderedFields, _ := jsonObjectFields(raw)
		for _, field := range orderedFields {
			keyValue, err := vm.typedJSONMapKey(keyType, field.name)
			if err != nil {
				return Null, err
			}
			value, err := vm.typedValueFromJSON(valueType, field.value, strict)
			if err != nil {
				return Null, err
			}
			encodedKey := vm.mapKey(keyValue)
			if _, exists := out.Map[encodedKey]; !exists {
				out.MapOrder = append(out.MapOrder, encodedKey)
			}
			out.Map[encodedKey] = value
			out.MapKeys[encodedKey] = keyValue
		}
		return out, nil
	}
	if strings.EqualFold(typeName, "Object") {
		return valueFromJSON(raw), nil
	}
	if enumValue, ok, err := vm.typedEnumValueFromJSON(typeName, raw); ok || err != nil {
		return enumValue, err
	}
	if strings.EqualFold(typeName, "sObject") {
		return vm.sObjectValueFromJSON(raw, strict)
	}
	if !vm.isJSONTypedObjectTarget(typeName) {
		return Null, unsupportedCallError("JSON.deserialize local class/SObject mapping for " + typeName)
	}
	obj, err := vm.jsonObjectBaseValue(typeName)
	if err != nil {
		return Null, err
	}
	if vm.isSObjectLikeType(typeName) {
		vm.markJSONDeserializedSObjectFields(&obj, typeName)
	}
	typedObjectIsSObject := vm.isSObjectLikeType(typeName)
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return Null, jsonTypeMappingError(typeName, raw)
	}
	if strict {
		if !vm.allowOpenSObjectJSONFields(typeName) {
			allowed := vm.jsonAllowedFields(typeName)
			for key := range fields {
				if key == "attributes" {
					continue
				}
				if !jsonAllowedFieldContains(allowed, key) && !vm.jsonStrictAllowsRelationshipPayload(typeName, key, fields[key]) {
					if typedObjectIsSObject && len(fields) == 1 {
						return Null, newExceptionError("JSONException", fmt.Sprintf("No such column '%s' on sobject of type %s", key, typeName))
					}
					return Null, newExceptionError("JSONException", fmt.Sprintf("JSON.deserializeStrict found unknown field %q for %s", key, typeName))
				}
			}
		}
	}
	for _, key := range vm.sortedJSONTypedObjectFields(typeName, fields) {
		item := fields[key]
		if key == "attributes" {
			continue
		}
		if jsonSObjectEmptyCanonicalIDShadowedByLowercase(fields, key) {
			continue
		}
		if jsonSObjectLowercaseIDShadowedByCanonical(fields, key) {
			continue
		}
		if typedObjectIsSObject {
			if handled, err := vm.applyDottedSObjectJSONField(&obj, typeName, key, item, strict); handled || err != nil {
				if err != nil {
					return Null, err
				}
				continue
			}
		}
		if typedObjectIsSObject {
			if relationshipType, ok := vm.jsonSObjectChildRelationshipType(typeName, key); ok {
				if _, hasRecords := jsonQueryResultRecords(item); hasRecords {
					value, err := vm.typedValueFromJSON(relationshipType, item, strict)
					if err != nil {
						return Null, err
					}
					obj.Fields[key] = value
					continue
				}
				if _, isArray := item.([]any); isArray {
					value, err := vm.typedValueFromJSON(relationshipType, item, strict)
					if err != nil {
						return Null, err
					}
					obj.Fields[key] = value
					continue
				}
			}
		}
		if typedObjectIsSObject {
			if relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, key); ok {
				value, err := vm.typedValueFromJSON(relationshipType, item, strict)
				if err != nil {
					return Null, err
				}
				vm.setSObjectParentRelationshipValue(&obj, typeName, key, value)
				continue
			}
		}
		if typedObjectIsSObject {
			if fieldType, ok := vm.jsonSObjectFieldType(typeName, key); ok {
				value, err := vm.typedSObjectFieldValueFromJSON(fieldType, item, strict)
				if err != nil {
					return Null, err
				}
				if vm.jsonSObjectFieldIsIDLike(typeName, key) {
					value = markJSONSObjectIDValue(value)
				}
				fieldName := vm.resolveSObjectFieldName(typeName, key)
				obj.Fields[fieldName] = value
				markExplicitSObjectField(&obj, fieldName)
				continue
			}
		}
		if typedObjectIsSObject {
			fieldName := vm.resolveSObjectFieldName(typeName, key)
			if !strings.EqualFold(fieldName, key) {
				if fieldType, ok := vm.jsonSObjectFieldType(typeName, fieldName); ok {
					value, err := vm.typedValueFromJSON(fieldType, item, strict)
					if err != nil {
						return Null, err
					}
					if vm.jsonSObjectFieldIsIDLike(typeName, fieldName) {
						value = markJSONSObjectIDValue(value)
					}
					obj.Fields[fieldName] = value
					markExplicitSObjectField(&obj, fieldName)
					continue
				}
			}
		}
		if field, owner, ok := vm.lookupField(typeName, key); ok && field.Type != "" {
			fieldType := vm.resolveTypeNameInClass(owner, field.Type)
			value, err := vm.typedValueFromJSON(fieldType, item, strict)
			if err != nil {
				if strict {
					return Null, err
				}
				value = valueFromJSON(item)
			}
			if field.Setter != nil {
				updated, err := vm.callReceiverSetterReturningReceiver(obj, *field.Setter, value)
				if err != nil {
					return Null, err
				}
				obj = updated
				continue
			}
			fieldName := field.Name
			if fieldName == "" {
				fieldName = key
			}
			if typedObjectIsSObject {
				fieldName = vm.resolveSObjectFieldName(typeName, fieldName)
				obj.Fields[fieldName] = value
				markExplicitSObjectField(&obj, fieldName)
				continue
			}
			obj.Fields[fieldName] = value
			continue
		}
		if fieldType, ok := platformJSONDTOFieldType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(fieldType, item, strict)
			if err != nil {
				return Null, err
			}
			obj.Fields[platformJSONDTOFieldName(key)] = value
			continue
		}
		actualKey := key
		if existingKey, _, ok := objectFieldValue(obj, key); ok {
			actualKey = existingKey
		}
		obj.Fields[actualKey] = valueFromJSON(item)
	}
	vm.hydrateParentLookupFields(obj)
	return obj, nil
}

func (vm *VM) callReceiverSetterReturningReceiver(receiver Value, setter Method, value Value) (Value, error) {
	if vm.Globals == nil {
		vm.Globals = make(map[string]Value)
	}
	key := "__glade_json_receiver"
	for {
		if _, exists := vm.Globals[key]; !exists {
			break
		}
		key += "_"
	}
	vm.Globals[key] = receiver
	_, err := vm.callMethodWithReceiver(setter, receiver, []Value{value}, resultForLookup())
	updated := vm.Globals[key]
	delete(vm.Globals, key)
	return updated, err
}

func (vm *VM) sortedJSONTypedObjectFields(typeName string, fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		leftPriority := vm.jsonTypedObjectFieldPriority(typeName, keys[i])
		rightPriority := vm.jsonTypedObjectFieldPriority(typeName, keys[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return keys[i] < keys[j]
	})
	return keys
}

func (vm *VM) jsonTypedObjectFieldPriority(typeName, key string) int {
	if key == "attributes" {
		return -1
	}
	if field, _, ok := vm.lookupField(typeName, key); ok && field.Setter != nil {
		return 1
	}
	return 0
}

func (vm *VM) jsonObjectBaseValue(typeName string) (Value, error) {
	if class, ok := vm.lookupClass(typeName); ok && classHasZeroArgConstructor(class) {
		value, err := vm.constructValue(typeName, nil, nil, resultForLookup())
		if err != nil {
			return Null, err
		}
		if value.Kind == ValueObject {
			return value, nil
		}
	}
	obj := Object(typeName)
	vm.initializeFields(&obj, typeName)
	return obj, nil
}

func (vm *VM) applySObjectMetadataDefaults(record *Value) {
	if vm == nil || vm.Org == nil || record == nil || record.Kind != ValueObject {
		return
	}
	_, definition, ok := vm.describeObjectDefinition(record.Type)
	if !ok || definition.APIName == "" || len(definition.Fields) == 0 {
		return
	}
	for name, field := range definition.Fields {
		if _, _, exists := objectFieldValue(*record, name); exists {
			continue
		}
		defaultValue, ok := vm.defaultValueForNewSObjectField(definition, *record, field)
		if !ok {
			continue
		}
		putVMFieldPath(*record, name, vmValueFromStorage(defaultValue))
	}
}

func (vm *VM) markJSONDeserializedSObjectFields(record *Value, typeName string) {
	if vm == nil || vm.Org == nil || record == nil || record.Kind != ValueObject {
		return
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return
	}
	definition := vm.describePreparedDefinition(objectName, object.Definition)
	for name := range definition.Fields {
		markSetSObjectField(record, name)
	}
	for _, name := range []string{"Id", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp", "OwnerId", "IsDeleted"} {
		markSetSObjectField(record, name)
	}
}

func classHasZeroArgConstructor(class Class) bool {
	for _, ctor := range class.Constructors {
		if len(ctor.Params) == 0 {
			return true
		}
	}
	return false
}

func (vm *VM) sObjectValueFromJSON(raw any, strict bool) (Value, error) {
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return Null, jsonTypeMappingError("sObject", raw)
	}
	typeName := "sObject"
	if attrs, ok := jsonObjectMap(fields["attributes"]); ok {
		if rawType, ok := attrs["type"].(string); ok && strings.TrimSpace(rawType) != "" {
			typeName = strings.TrimSpace(rawType)
		}
	}
	obj := Object(typeName)
	vm.initializeFields(&obj, typeName)
	vm.markJSONDeserializedSObjectFields(&obj, typeName)
	for key, item := range fields {
		if key == "attributes" {
			continue
		}
		if jsonSObjectEmptyCanonicalIDShadowedByLowercase(fields, key) {
			continue
		}
		if jsonSObjectLowercaseIDShadowedByCanonical(fields, key) {
			continue
		}
		if handled, err := vm.applyDottedSObjectJSONField(&obj, typeName, key, item, strict); handled || err != nil {
			if err != nil {
				return Null, err
			}
			continue
		}
		if relationshipType, ok := vm.jsonSObjectChildRelationshipType(typeName, key); ok {
			if _, hasRecords := jsonQueryResultRecords(item); hasRecords {
				value, err := vm.typedValueFromJSON(relationshipType, item, strict)
				if err != nil {
					return Null, err
				}
				obj.Fields[key] = value
				continue
			}
			if _, isArray := item.([]any); isArray {
				value, err := vm.typedValueFromJSON(relationshipType, item, strict)
				if err != nil {
					return Null, err
				}
				obj.Fields[key] = value
				continue
			}
		}
		if relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(relationshipType, item, strict)
			if err != nil {
				return Null, err
			}
			vm.setSObjectParentRelationshipValue(&obj, typeName, key, value)
			continue
		}
		if fieldType, ok := vm.jsonSObjectFieldType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(fieldType, item, strict)
			if err != nil {
				return Null, err
			}
			if vm.jsonSObjectFieldIsIDLike(typeName, key) {
				value = markJSONSObjectIDValue(value)
			}
			fieldName := vm.resolveSObjectFieldName(typeName, key)
			obj.Fields[fieldName] = value
			markExplicitSObjectField(&obj, fieldName)
			continue
		}
		fieldName := vm.resolveSObjectFieldName(typeName, key)
		if !strings.EqualFold(fieldName, key) {
			if fieldType, ok := vm.jsonSObjectFieldType(typeName, fieldName); ok {
				value, err := vm.typedSObjectFieldValueFromJSON(fieldType, item, strict)
				if err != nil {
					return Null, err
				}
				if vm.jsonSObjectFieldIsIDLike(typeName, fieldName) {
					value = markJSONSObjectIDValue(value)
				}
				obj.Fields[fieldName] = value
				markExplicitSObjectField(&obj, fieldName)
				continue
			}
		}
		obj.Fields[key] = valueFromJSON(item)
	}
	vm.hydrateParentLookupFields(obj)
	return obj, nil
}

func (vm *VM) typedSObjectFieldValueFromJSON(fieldType string, raw any, strict bool) (Value, error) {
	value, err := vm.typedValueFromJSON(fieldType, raw, strict)
	if err == nil || !strings.EqualFold(fieldType, "Blob") {
		return value, err
	}
	text, ok := raw.(string)
	if !ok {
		return value, err
	}
	return platformScalar("Blob", text), nil
}

func jsonSObjectLowercaseIDShadowedByCanonical(fields map[string]any, key string) bool {
	if key == "Id" || !strings.EqualFold(key, "Id") {
		return false
	}
	canonical, ok := fields["Id"]
	if !ok {
		return false
	}
	if text, ok := canonical.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	return canonical != nil
}

func jsonSObjectEmptyCanonicalIDShadowedByLowercase(fields map[string]any, key string) bool {
	if key != "Id" {
		return false
	}
	canonical, hasCanonical := fields["Id"]
	lower, hasLower := fields["id"]
	if !hasCanonical || !hasLower {
		return false
	}
	if text, ok := canonical.(string); ok && strings.TrimSpace(text) != "" {
		return false
	}
	if text, ok := lower.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	return lower != nil
}

func (vm *VM) setSObjectParentRelationshipValue(obj *Value, typeName, relationshipName string, relationship Value) {
	if obj == nil || obj.Kind != ValueObject {
		return
	}
	if obj.Fields == nil {
		obj.Fields = make(map[string]Value)
	}
	obj.Fields[relationshipName] = relationship
	for _, alias := range vm.parentRelationshipValueAliases(typeName, relationshipName) {
		obj.Fields[alias] = relationship
	}
}

func (vm *VM) parentRelationshipValueAliases(typeName, relationshipName string) []string {
	aliases := []string{relationshipName}
	if vm != nil && vm.Org != nil && vm.Org.Namespace != "" {
		aliases = append(aliases, storage.StripNamespaceToken(vm.Org.Namespace, relationshipName))
		aliases = append(aliases, storage.NamespaceTokenName(vm.Org.Namespace, relationshipName))
	}
	if vm == nil || vm.Org == nil {
		return uniqueNonEmptyStrings(aliases)
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return uniqueNonEmptyStrings(aliases)
	}
	object := vm.Org.Objects[objectName]
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		aliases = append(aliases, relation.ParentRelationship)
		if vm.Org.Namespace != "" {
			aliases = append(aliases, storage.StripNamespaceToken(vm.Org.Namespace, relation.ParentRelationship))
			aliases = append(aliases, storage.NamespaceTokenName(vm.Org.Namespace, relation.ParentRelationship))
		}
	}
	return uniqueNonEmptyStrings(aliases)
}

func uniqueNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func (vm *VM) applyDottedSObjectJSONField(obj *Value, typeName, key string, item any, strict bool) (bool, error) {
	relationshipName, childPath, ok := strings.Cut(key, ".")
	if !ok || strings.TrimSpace(relationshipName) == "" || strings.TrimSpace(childPath) == "" {
		return false, nil
	}
	relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, relationshipName)
	if !ok {
		return false, nil
	}
	actualRelationshipName := relationshipName
	relationship, exists := Null, false
	if actual, value, ok := objectFieldValue(*obj, relationshipName); ok {
		actualRelationshipName = actual
		relationship = value
		exists = true
	}
	if !exists || relationship.Kind == ValueNull {
		relationship = Object(relationshipType)
		vm.initializeFields(&relationship, relationshipType)
	}
	if relationship.Kind != ValueObject {
		return true, fmt.Errorf("JSON dotted relationship %s on %s is not an SObject", relationshipName, typeName)
	}
	if nested, err := vm.applyDottedSObjectJSONField(&relationship, relationshipType, childPath, item, strict); nested || err != nil {
		if err != nil {
			return true, err
		}
		obj.Fields[actualRelationshipName] = relationship
		return true, nil
	}
	if fieldType, ok := vm.jsonSObjectFieldType(relationshipType, childPath); ok {
		value, err := vm.typedSObjectFieldValueFromJSON(fieldType, item, strict)
		if err != nil {
			return true, err
		}
		relationship.Fields[vm.resolveSObjectFieldName(relationshipType, childPath)] = value
		obj.Fields[actualRelationshipName] = relationship
		return true, nil
	}
	if strict {
		return true, newExceptionError("JSONException", fmt.Sprintf("JSON.deserializeStrict found unknown field %q for %s", childPath, relationshipType))
	}
	relationship.Fields[childPath] = valueFromJSON(item)
	obj.Fields[actualRelationshipName] = relationship
	return true, nil
}

func jsonQueryResultRecords(raw any) ([]any, bool) {
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return nil, false
	}
	records, ok := fields["records"].([]any)
	return records, ok
}

func (vm *VM) jsonSObjectFieldType(typeName, fieldName string) (string, bool) {
	if strings.EqualFold(fieldName, "Id") {
		return "String", true
	}
	if fieldType, ok := jsonSObjectSystemFieldType(fieldName); ok {
		return fieldType, true
	}
	if vm.Org == nil {
		return "", false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return "", false
	}
	fieldName = vm.resolveSObjectFieldName(typeName, fieldName)
	field, ok := vm.Org.Objects[objectName].Definition.Fields[fieldName]
	if !ok {
		return "", false
	}
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return "String", true
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		return "String", true
	case storage.FieldBoolean:
		return "Boolean", true
	case storage.FieldInteger:
		return "Integer", true
	case storage.FieldDecimal:
		return "Decimal", true
	case storage.FieldBlob:
		return "Blob", true
	case storage.FieldDate:
		return "Date", true
	case storage.FieldDateTime:
		return "Datetime", true
	case storage.FieldCalculated, storage.FieldSummary:
		switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
		case "INTEGER":
			return "Integer", true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return "Decimal", true
		case "BOOLEAN":
			return "Boolean", true
		case "DATE":
			return "Date", true
		case "DATETIME":
			return "Datetime", true
		case "ID", "REFERENCE":
			return "String", true
		case "STRING", "TEXTAREA", "PICKLIST":
			return "String", true
		}
		return "String", true
	default:
		return "", false
	}
}

func (vm *VM) jsonSObjectFieldIsIDLike(typeName, fieldName string) bool {
	if strings.EqualFold(fieldName, "Id") {
		return true
	}
	_, field, ok := vm.sObjectFieldDefinition(typeName, fieldName)
	if !ok {
		return false
	}
	return field.Type == storage.FieldID || field.Type == storage.FieldReference
}

func markJSONSObjectIDValue(value Value) Value {
	if value.Kind == ValueString {
		value.Type = "Id"
	}
	return value
}

func (vm *VM) typedEnumValueFromJSON(typeName string, raw any) (Value, bool, error) {
	text, ok := raw.(string)
	if !ok {
		return Null, false, nil
	}
	if value, ok := schemaDisplayTypeStaticValue("Schema.DisplayType." + text); ok &&
		(strings.EqualFold(typeName, "Schema.DisplayType") || strings.EqualFold(typeName, "DisplayType")) {
		return value, true, nil
	}
	if value, ok := schemaSOAPTypeStaticValue("Schema.SOAPType." + text); ok &&
		(strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "SOAPType")) {
		return value, true, nil
	}
	if class, ok := vm.resolveEnumClass(typeName); ok {
		for i, candidate := range class.EnumValues {
			if strings.EqualFold(candidate, text) {
				value := Value{Kind: ValueObject, Type: class.Name, Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}
				return value, true, nil
			}
		}
		return Null, true, newExceptionError("System.NoSuchElementException", fmt.Sprintf("No enum value found called %s", text))
	}
	return Null, false, nil
}

func jsonSObjectSystemFieldType(fieldName string) (string, bool) {
	switch {
	case strings.EqualFold(fieldName, "CreatedDate"),
		strings.EqualFold(fieldName, "LastModifiedDate"),
		strings.EqualFold(fieldName, "SystemModstamp"):
		return "Datetime", true
	case strings.EqualFold(fieldName, "CreatedById"),
		strings.EqualFold(fieldName, "LastModifiedById"),
		strings.EqualFold(fieldName, "OwnerId"):
		return "String", true
	case strings.EqualFold(fieldName, "IsDeleted"):
		return "Boolean", true
	default:
		return "", false
	}
}

func (vm *VM) jsonSObjectParentRelationshipType(typeName, relationshipName string) (string, bool) {
	if vm.Org == nil {
		return "", false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return "", false
	}
	object := vm.Org.Objects[objectName]
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		if len(relation.ParentObjects) == 0 {
			continue
		}
		return relation.ParentObjects[0], true
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		if len(relation.ParentObjects) == 0 {
			return "", false
		}
		return relation.ParentObjects[0], true
	}
	return "", false
}

func (vm *VM) jsonSObjectChildRelationshipType(typeName, relationshipName string) (string, bool) {
	if vm.Org == nil {
		return "", false
	}
	cacheKey := strings.ToLower(strings.TrimSpace(typeName)) + "\x00" + strings.ToLower(strings.TrimSpace(relationshipName))
	if cached, ok := vm.jsonChildRelTypeCache.load(cacheKey); ok {
		return cached.Type, cached.OK
	}
	cacheResult := func(typeName string, ok bool) (string, bool) {
		if vm.jsonChildRelTypeCache == nil {
			vm.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
		}
		vm.jsonChildRelTypeCache.store(cacheKey, jsonRelationshipTypeLookup{Type: typeName, OK: ok})
		return typeName, ok
	}
	parentObject, ok := vm.resolveObjectName(typeName)
	if !ok {
		return cacheResult("", false)
	}
	parentKey := strings.ToLower(strings.TrimSpace(parentObject))
	relationshipKey := strings.ToLower(strings.TrimSpace(relationshipName))
	if index, ok := vm.jsonChildRelTypeCache.loadParent(parentKey); ok {
		if cached, ok := index[relationshipKey]; ok {
			return cacheResult(cached.Type, cached.OK)
		}
		return cacheResult("", false)
	}
	index := vm.buildJSONSObjectChildRelationshipTypeIndex(parentObject)
	vm.jsonChildRelTypeCache.storeParent(parentKey, index)
	if cached, ok := index[relationshipKey]; ok {
		return cacheResult(cached.Type, cached.OK)
	}
	return cacheResult("", false)
}

func (vm *VM) buildJSONSObjectChildRelationshipTypeIndex(parentObject string) map[string]jsonRelationshipTypeLookup {
	matchesByName := make(map[string][]string)
	addMatch := func(childRelationshipName, childName string) {
		if strings.TrimSpace(childRelationshipName) == "" || strings.TrimSpace(childName) == "" {
			return
		}
		for _, alias := range jsonRelationshipNameLookupKeys(vm.Org.Namespace, childRelationshipName) {
			matchesByName[alias] = appendUniqueStringFold(matchesByName[alias], childName)
		}
	}
	for childName, childState := range vm.Org.Objects {
		childRelationshipName := ""
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) {
				continue
			}
			childRelationshipName = relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			addMatch(childRelationshipName, childName)
		}
	}
	for childName, childState := range vm.Org.Objects {
		for _, field := range childState.Definition.Fields {
			if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
				continue
			}
			if !relationshipTargetsObject(storage.Relationship{ParentObjects: field.ReferenceTo}, parentObject) {
				continue
			}
			for _, childRelationshipName := range vmFieldChildRelationshipNames(childState.Definition, field) {
				addMatch(childRelationshipName, childName)
			}
		}
	}
	index := make(map[string]jsonRelationshipTypeLookup, len(matchesByName))
	for relationshipKey, matches := range matchesByName {
		if childName := vm.bestChildRelationshipObject(matches); childName != "" {
			index[relationshipKey] = jsonRelationshipTypeLookup{Type: "List<" + childName + ">", OK: true}
		}
	}
	return index
}

func jsonRelationshipNameLookupKeys(namespace, name string) []string {
	seen := make(map[string]bool, 6)
	keys := make([]string, 0, 6)
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		keys = append(keys, value)
	}
	addVariant := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		add(value)
		if hasSuffixFold(value, "__r") {
			add(value[:len(value)-3])
		} else {
			add(value + "__r")
		}
	}
	addVariant(name)
	addVariant(stripAnyNamespaceToken(name))
	if strings.TrimSpace(namespace) != "" {
		addVariant(storage.StripNamespaceToken(namespace, name))
		addVariant(storage.NamespaceTokenName(namespace, name))
		addVariant(jsonNamespacedRelationshipLookupName(namespace, name))
	}
	return keys
}

func jsonNamespacedRelationshipLookupName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" || strings.Contains(name, "__") {
		return name
	}
	return namespace + "__" + name
}

func (vm *VM) bestChildRelationshipObject(matches []string) string {
	if len(matches) == 0 {
		return ""
	}
	best := matches[0]
	for _, candidate := range matches[1:] {
		if vm.childRelationshipObjectLess(candidate, best) {
			best = candidate
		}
	}
	return best
}

func (vm *VM) childRelationshipObjectLess(left, right string) bool {
	leftPriority := vm.childRelationshipObjectPriority(left)
	rightPriority := vm.childRelationshipObjectPriority(right)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if vm != nil && vm.Org != nil {
		leftState, leftOK := vm.Org.Objects[left]
		rightState, rightOK := vm.Org.Objects[right]
		if leftOK && rightOK {
			leftScore := len(leftState.Definition.Fields) + len(leftState.Definition.Relations)
			rightScore := len(rightState.Definition.Fields) + len(rightState.Definition.Relations)
			if leftScore != rightScore {
				return leftScore > rightScore
			}
		}
	}
	return left < right
}

func (vm *VM) childRelationshipObjectPriority(objectName string) int {
	if vm != nil && vm.Org != nil && vm.Org.Namespace != "" {
		if hasPrefixFold(objectName, strings.ToLower(vm.Org.Namespace)+"__") && isCustomObjectLikeName(objectName) {
			return 0
		}
	}
	if isCustomObjectLikeName(objectName) {
		return 1
	}
	if isCommonSObjectTypeName(objectName) {
		return 2
	}
	return 3
}

func vmFieldChildRelationshipNames(definition storage.ObjectDefinition, field storage.Field) []string {
	names := []string(nil)
	if field.ChildRelationshipName != "" {
		names = appendUniqueStringFold(names, field.ChildRelationshipName)
	}
	if childRelationshipName := storage.ChildRelationshipName(field); childRelationshipName != "" {
		names = appendUniqueStringFold(names, childRelationshipName)
	}
	if derived := derivedVMChildRelationshipName(definition); derived != "" {
		names = appendUniqueStringFold(names, derived)
	}
	return names
}

func (vm *VM) isJSONTypedObjectTarget(typeName string) bool {
	typeName = vm.resolveJSONTypeName(typeName)
	if _, ok := vm.lookupClass(typeName); ok {
		return true
	}
	if vm.isSObjectLikeType(typeName) {
		return true
	}
	if strings.EqualFold(typeName, "Schema.FieldSetMember") {
		return true
	}
	if _, ok := platformJSONDTOFields(typeName); ok {
		return true
	}
	if vm.Org != nil {
		if _, ok := vm.Org.Objects[typeName]; ok {
			return true
		}
	}
	return false
}

func (vm *VM) resolveJSONTypeName(typeName string) string {
	if resolved, ok := vm.resolveClassName(typeName); ok {
		return resolved
	}
	if alias, ok := platformShortTypeAlias(typeName); ok {
		return alias
	}
	if canonical := canonicalRuntimeTypeName(typeName); !strings.EqualFold(canonical, typeName) {
		return canonical
	}
	return typeName
}

func platformJSONDTOFields(typeName string) (map[string]string, bool) {
	resultFields := map[string]string{
		"success":           "Boolean",
		"id":                "Id",
		"errors":            "List<Database.Error>",
		"created":           "Boolean",
		"mergedRecordIds":   "List<Id>",
		"updatedRelatedIds": "List<Id>",
	}
	switch {
	case strings.EqualFold(typeName, "Database.SaveResult"),
		strings.EqualFold(typeName, "Database.DeleteResult"),
		strings.EqualFold(typeName, "Database.UndeleteResult"),
		strings.EqualFold(typeName, "Database.EmptyRecycleBinResult"),
		strings.EqualFold(typeName, "Database.LockResult"),
		strings.EqualFold(typeName, "Database.UnlockResult"),
		strings.EqualFold(typeName, "Approval.LockResult"),
		strings.EqualFold(typeName, "Approval.UnlockResult"),
		strings.EqualFold(typeName, "Database.UpsertResult"),
		strings.EqualFold(typeName, "Database.MergeResult"):
		return resultFields, true
	case strings.EqualFold(typeName, "Database.Error"):
		return map[string]string{
			"message":              "String",
			"statusCode":           "String",
			"fields":               "List<String>",
			"extendedErrorDetails": "List<Object>",
		}, true
	default:
		return nil, false
	}
}

func platformJSONDTOFieldType(typeName, field string) (string, bool) {
	fields, ok := platformJSONDTOFields(typeName)
	if !ok {
		return "", false
	}
	for candidate, fieldType := range fields {
		if strings.EqualFold(candidate, field) {
			return fieldType, true
		}
	}
	return "", false
}

func platformJSONDTOFieldName(field string) string {
	switch {
	case strings.EqualFold(field, "statusCode"):
		return "statusCode"
	case strings.EqualFold(field, "mergedRecordIds"):
		return "mergedRecordIds"
	case strings.EqualFold(field, "updatedRelatedIds"):
		return "updatedRelatedIds"
	case field == "":
		return field
	default:
		return strings.ToLower(field[:1]) + field[1:]
	}
}

func typedScalarFromJSON(typeName string, raw any) (Value, bool, error) {
	canonical := canonicalJSONScalarType(typeName)
	if raw == nil {
		return Null, true, nil
	}
	switch canonical {
	case "String":
		switch value := raw.(type) {
		case string:
			return String(value), true, nil
		case json.Number:
			return String(value.String()), true, nil
		case float64:
			return String(strconv.FormatFloat(value, 'f', -1, 64)), true, nil
		case bool:
			if value {
				return String("true"), true, nil
			}
			return String("false"), true, nil
		default:
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
	case "Boolean":
		value, ok := raw.(bool)
		if !ok {
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(text)))
				if err == nil {
					return Bool(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Bool(value), true, nil
	case "Integer", "Long":
		value, ok := jsonIntegralNumber(raw)
		if !ok {
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
				if err == nil {
					return Int(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Int(value), true, nil
	case "Decimal", "Double":
		if number, ok := raw.(json.Number); ok {
			parsed, err := strconv.ParseFloat(number.String(), 64)
			if err != nil {
				return Null, true, jsonTypeMappingError(canonical, raw)
			}
			decimal := Decimal(parsed)
			decimal.Text = number.String()
			return decimal, true, nil
		}
		value, ok := jsonDecimalNumber(raw)
		if !ok {
			if text, textOK := raw.(string); textOK {
				trimmed := strings.TrimSpace(text)
				if trimmed == "" {
					return Null, true, nil
				}
				parsed, err := strconv.ParseFloat(trimmed, 64)
				if err == nil {
					return Decimal(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Decimal(value), true, nil
	case "Date":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseDateText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("JSON.deserialize cannot parse Date %q", text)
		}
		return platformScalar("Date", value.Format("2006-01-02")), true, nil
	case "Datetime":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseDatetimeTextAllowDateOnly(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Datetime", value.UTC().Format(time.RFC3339)), true, nil
	case "Time":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseTimeText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Time", value), true, nil
	case "Id":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		if err := validateApexID(text); err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Id", text), true, nil
	case "Blob":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return Null, true, jsonDeserializeException("JSON.deserialize cannot decode Blob base64: %v", err)
		}
		return platformScalar("Blob", string(decoded)), true, nil
	case "UUID":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		parsed, err := parseUUIDText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return uuidValue(parsed), true, nil
	}
	return Null, false, nil
}

// jsonChildRelTypeLookupCache memoizes (typeName, relationshipName) -> child
// type lookups inside one runtime clone.
type jsonChildRelTypeLookupCache struct {
	mu            sync.RWMutex
	entries       map[string]jsonRelationshipTypeLookup
	parentIndexes map[string]map[string]jsonRelationshipTypeLookup
}

func newJSONChildRelTypeLookupCache() *jsonChildRelTypeLookupCache {
	return &jsonChildRelTypeLookupCache{
		entries:       make(map[string]jsonRelationshipTypeLookup),
		parentIndexes: make(map[string]map[string]jsonRelationshipTypeLookup),
	}
}

func (c *jsonChildRelTypeLookupCache) load(key string) (jsonRelationshipTypeLookup, bool) {
	if c == nil {
		return jsonRelationshipTypeLookup{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *jsonChildRelTypeLookupCache) store(key string, value jsonRelationshipTypeLookup) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
}

func (c *jsonChildRelTypeLookupCache) loadParent(key string) (map[string]jsonRelationshipTypeLookup, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.parentIndexes[key]
	return value, ok
}

func (c *jsonChildRelTypeLookupCache) storeParent(key string, value map[string]jsonRelationshipTypeLookup) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.parentIndexes[key] = value
	c.mu.Unlock()
}

func canonicalJSONScalarType(typeName string) string {
	switch {
	case strings.EqualFold(typeName, "String"):
		return "String"
	case strings.EqualFold(typeName, "Boolean"):
		return "Boolean"
	case strings.EqualFold(typeName, "Integer"):
		return "Integer"
	case strings.EqualFold(typeName, "Long"):
		return "Long"
	case strings.EqualFold(typeName, "Decimal"):
		return "Decimal"
	case strings.EqualFold(typeName, "Double"):
		return "Double"
	case strings.EqualFold(typeName, "Date"):
		return "Date"
	case strings.EqualFold(typeName, "Datetime") || strings.EqualFold(typeName, "DateTime"):
		return "Datetime"
	case strings.EqualFold(typeName, "Time"):
		return "Time"
	case strings.EqualFold(typeName, "Id"):
		return "Id"
	case strings.EqualFold(typeName, "Blob"):
		return "Blob"
	case strings.EqualFold(typeName, "UUID"):
		return "UUID"
	default:
		return typeName
	}
}
func jsonIntegralNumber(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case json.Number:
		text := value.String()
		if strings.ContainsAny(text, ".eE") {
			decimal, err := strconv.ParseFloat(text, 64)
			if err != nil || math.Trunc(decimal) != decimal {
				return 0, false
			}
			converted, err := int64FromFloat("JSON number", decimal)
			return converted, err == nil
		}
		converted, err := strconv.ParseInt(text, 10, 64)
		return converted, err == nil
	case float64:
		if math.Trunc(value) != value {
			return 0, false
		}
		converted, err := int64FromFloat("JSON number", value)
		return converted, err == nil
	default:
		return 0, false
	}
}
func jsonDecimalNumber(raw any) (float64, bool) {
	switch value := raw.(type) {
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		converted, err := strconv.ParseFloat(value.String(), 64)
		return converted, err == nil
	case float64:
		return value, true
	default:
		return 0, false
	}
}
func jsonTypeMappingError(typeName string, raw any) error {
	return jsonDeserializeException("JSON.deserialize cannot map JSON %s to %s", jsonRawKind(raw), typeName)
}
func jsonDeserializeException(format string, args ...any) error {
	if format == "JSON.deserializeUntyped invalid JSON input: %v" && len(args) == 1 {
		if err, ok := args[0].(*jsonNumberInputError); ok {
			return newExceptionError("JSONException", err.Error())
		}
	}
	return newExceptionError("JSONException", fmt.Sprintf(format, args...))
}
func jsonRawKind(raw any) string {
	switch raw.(type) {
	case nil:
		return "null"
	case bool:
		return "Boolean"
	case json.Number, float64:
		return "number"
	case string:
		return "String"
	case []any:
		return "array"
	case map[string]any, orderedJSONObject:
		return "object"
	default:
		return fmt.Sprintf("%T", raw)
	}
}
func (vm *VM) jsonAllowedFields(typeName string) map[string]struct{} {
	allowed := map[string]struct{}{
		"Id":               {},
		"CreatedDate":      {},
		"CreatedById":      {},
		"LastModifiedDate": {},
		"LastModifiedById": {},
		"SystemModstamp":   {},
		"OwnerId":          {},
		"IsDeleted":        {},
	}
	if vm.Org != nil {
		if objectName, ok := vm.resolveObjectName(typeName); ok {
			object := vm.Org.Objects[objectName]
			for name := range object.Definition.Fields {
				allowed[name] = struct{}{}
				if vm.Org.Namespace != "" {
					allowed[storage.StripNamespaceToken(vm.Org.Namespace, name)] = struct{}{}
					allowed[storage.NamespaceTokenName(vm.Org.Namespace, name)] = struct{}{}
				}
			}
			for _, relation := range object.Definition.Relations {
				for _, name := range []string{relation.ParentRelationship, relation.ChildRelationship} {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					allowed[name] = struct{}{}
					if vm.Org.Namespace != "" {
						allowed[storage.StripNamespaceToken(vm.Org.Namespace, name)] = struct{}{}
						allowed[storage.NamespaceTokenName(vm.Org.Namespace, name)] = struct{}{}
					}
				}
			}
		}
	}
	for className := typeName; className != ""; {
		class, ok := vm.Classes[className]
		if !ok {
			break
		}
		for name := range class.Fields {
			allowed[name] = struct{}{}
		}
		className = class.SuperClass
	}
	return allowed
}
func jsonAllowedFieldContains(allowed map[string]struct{}, key string) bool {
	if _, ok := allowed[key]; ok {
		return true
	}
	for candidate := range allowed {
		if strings.EqualFold(candidate, key) {
			return true
		}
	}
	return false
}
func (vm *VM) jsonStrictAllowsRelationshipPayload(typeName, key string, item any) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if hasSuffixFold(key, "__r") {
		return true
	}
	if _, ok := jsonQueryResultRecords(item); ok {
		return hasSuffixFold(key, "s")
	}
	return false
}
func (vm *VM) allowOpenSObjectJSONFields(typeName string) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if _, ok := vm.Classes[typeName]; ok {
		return false
	}
	if vm.Org == nil {
		return true
	}
	_, ok := vm.resolveObjectName(typeName)
	return !ok
}
