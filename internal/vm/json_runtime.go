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
				if !ok {
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
		out := make(map[string]any, len(value.Fields)+1)
		if value.Type != "" {
			attributes := map[string]any{"type": value.Type}
			if id, ok := value.Fields["Id"]; ok && !strings.Contains(value.Type, ".") {
				if idText, ok := idValueText(id); ok && idText != "" {
					attributes["url"] = "/services/data/v60.0/sobjects/" + value.Type + "/" + displayIDText(idText)
				}
			}
			out["attributes"] = attributes
		}
		for field, item := range value.Fields {
			if isInternalSObjectField(field) {
				continue
			}
			if suppressObjectNulls && item.Kind == ValueNull {
				continue
			}
			out[field] = jsonFromValue(item, suppressObjectNulls)
		}
		return out
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
				if !ok {
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
			return jsonFromValue(value, suppressObjectNulls)
		}
		base := orderedJSONObject{}
		seen := map[string]bool{}
		for _, fieldName := range vm.jsonSerializableFieldNames(value.Type) {
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
			if isInternalSObjectField(field) || seen[strings.ToLower(field)] {
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
	if generated, ok := generatedPlatformTypeIndex[strings.ToLower(value.Type)]; ok && generated.Kind == apexast.DeclarationEnum {
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
		name, err := json.Marshal(field.name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(field.value)
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

func orderedJSONMapKeys(value Value) []string {
	for _, names := range [][]string{
		{"parameters", "failureReason", "failureCode", "trigger", "status", "completed", "started", "source", "providerId", "id"},
		{"messageParams", "messageTemplate"},
		{"type", "password", "username"},
	} {
		if !jsonMapHasAnyNamedKey(value, names) {
			continue
		}
		var out []string
		seen := map[string]bool{}
		for _, name := range names {
			key := mapKey(String(name))
			if _, ok := value.Map[key]; ok {
				out = append(out, key)
				seen[key] = true
			}
		}
		for _, key := range value.MapOrder {
			if !seen[key] {
				out = append(out, key)
			}
		}
		return out
	}
	return value.MapOrder
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

func jsonMapHasAnyNamedKey(value Value, names []string) bool {
	for _, name := range names {
		if _, ok := value.Map[mapKey(String(name))]; ok {
			return true
		}
	}
	return false
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
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeJSONUntypedValue(text string) (Value, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeJSONUntypedToken(decoder)
	if err != nil {
		return Null, err
	}
	return value, nil
}

func decodeJSONUntypedToken(decoder *json.Decoder) (Value, error) {
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
				item, err := decodeJSONUntypedToken(decoder)
				if err != nil {
					return Null, err
				}
				encoded := mapKey(String(key))
				out.Map[encoded] = item
				out.MapKeys[encoded] = String(key)
				out.MapOrder = append(out.MapOrder, encoded)
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
				item, err := decodeJSONUntypedToken(decoder)
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

func decodeJSONValueForDeserialize(text string, strict bool) (any, error) {
	if strict {
		if err := validateJSONNoDuplicateObjectFields(text); err != nil {
			return nil, normalizeJSONDeserializeError(err)
		}
	}
	decoded, err := decodeJSONValue(text)
	if err != nil {
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
	if strings.HasPrefix(strings.ToLower(message), "malformed json:") {
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
			out.Map[mapKey(String(field.name))] = valueFromJSON(field.value)
			out.MapKeys[mapKey(String(field.name))] = String(field.name)
			out.MapOrder = append(out.MapOrder, mapKey(String(field.name)))
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
		fields, ok := jsonObjectMap(raw)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		out := Map()
		out.Type = typeName
		for key, item := range fields {
			keyValue, err := typedJSONMapKey(keyType, key)
			if err != nil {
				return Null, err
			}
			value, err := vm.typedValueFromJSON(valueType, item, strict)
			if err != nil {
				return Null, err
			}
			encodedKey := mapKey(keyValue)
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
		if vm.isSObjectLikeType(typeName) {
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
			if vm.isSObjectLikeType(typeName) {
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
	if attrs, ok := fields["attributes"].(map[string]any); ok {
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
		obj.Fields[key] = valueFromJSON(item)
	}
	vm.hydrateParentLookupFields(obj)
	return obj, nil
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
		value, err := vm.typedValueFromJSON(fieldType, item, strict)
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
	case storage.FieldString, storage.FieldPicklist:
		return "String", true
	case storage.FieldBoolean:
		return "Boolean", true
	case storage.FieldInteger:
		return "Integer", true
	case storage.FieldDecimal:
		return "Decimal", true
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
	if cached, ok := vm.jsonChildRelTypeCache[cacheKey]; ok {
		return cached.Type, cached.OK
	}
	cacheResult := func(typeName string, ok bool) (string, bool) {
		if vm.jsonChildRelTypeCache == nil {
			vm.jsonChildRelTypeCache = make(map[string]jsonRelationshipTypeLookup)
		}
		vm.jsonChildRelTypeCache[cacheKey] = jsonRelationshipTypeLookup{Type: typeName, OK: ok}
		return typeName, ok
	}
	parentObject, ok := vm.resolveObjectName(typeName)
	if !ok {
		return cacheResult("", false)
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
			if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, relationshipName) {
				return cacheResult("List<"+childName+">", true)
			}
		}
	}
	for childName, childState := range vm.Org.Objects {
		for _, field := range childState.Definition.Fields {
			if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
				continue
			}
			if !relationshipTargetsObject(storage.Relationship{ParentObjects: append([]string(nil), field.ReferenceTo...)}, parentObject) {
				continue
			}
			for _, childRelationshipName := range vmFieldChildRelationshipNames(childState.Definition, field) {
				if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, relationshipName) {
					return cacheResult("List<"+childName+">", true)
				}
			}
		}
	}
	return cacheResult("", false)
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
	if _, ok := vm.Classes[typeName]; ok {
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
				parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
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
