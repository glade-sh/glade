package vm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

type Value struct {
	Kind     ValueKind        `json:"kind"`
	Int      int64            `json:"int,omitempty"`
	Decimal  float64          `json:"decimal,omitempty"`
	Bool     bool             `json:"bool,omitempty"`
	Text     string           `json:"text,omitempty"`
	Type     string           `json:"type,omitempty"`
	Static   string           `json:"-"`
	Runtime  string           `json:"-"`
	Ref      uint64           `json:"-"`
	Fields   map[string]Value `json:"fields,omitempty"`
	List     []Value          `json:"list,omitempty"`
	Set      []Value          `json:"set,omitempty"`
	Map      map[string]Value `json:"map,omitempty"`
	MapKeys  map[string]Value `json:"-"`
	MapOrder []string         `json:"-"`
}

type ValueKind string

const (
	ValueNull    ValueKind = "null"
	ValueInt     ValueKind = "integer"
	ValueDecimal ValueKind = "decimal"
	ValueBool    ValueKind = "boolean"
	ValueString  ValueKind = "string"
	ValueList    ValueKind = "list"
	ValueSet     ValueKind = "set"
	ValueMap     ValueKind = "map"
	ValueObject  ValueKind = "object"
)

var Null = Value{Kind: ValueNull}

var nextValueRef atomic.Uint64

func newValueRef() uint64 {
	return nextValueRef.Add(1)
}

func Int(v int64) Value {
	return Value{Kind: ValueInt, Int: v}
}

func Decimal(v float64) Value {
	return Value{Kind: ValueDecimal, Decimal: v}
}

func Bool(v bool) Value {
	return Value{Kind: ValueBool, Bool: v}
}

func String(v string) Value {
	return Value{Kind: ValueString, Text: v}
}

func List(values ...Value) Value {
	return Value{Kind: ValueList, List: values, Ref: newValueRef()}
}

func Set(values ...Value) Value {
	out := Value{Kind: ValueSet, Ref: newValueRef()}
	for _, value := range values {
		if !containsValue(out.Set, value) {
			out.Set = append(out.Set, value)
		}
	}
	return out
}

func Map() Value {
	return Value{Kind: ValueMap, Map: make(map[string]Value), MapKeys: make(map[string]Value), Ref: newValueRef()}
}

func Object(typeName string) Value {
	return Value{Kind: ValueObject, Type: typeName, Fields: make(map[string]Value), Ref: newValueRef()}
}

func (v Value) String() string {
	switch v.Kind {
	case ValueNull:
		return "null"
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueDecimal:
		if v.Text != "" {
			return v.Text
		}
		return strconv.FormatFloat(v.Decimal, 'f', -1, 64)
	case ValueBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case ValueString:
		return v.Text
	case ValueList:
		return "List" + valuesString(v.List)
	case ValueSet:
		return "Set" + valuesString(v.Set)
	case ValueMap:
		return mapString(v.Map)
	case ValueObject:
		if stubbedType, ok := stubProxyTypeName(v); ok {
			return fmt.Sprintf("%s__sfdc_ApexStub:%d", stubbedType, v.Ref)
		}
		if strings.EqualFold(v.Type, "PageReference") {
			if rawURL, ok := v.Fields["url"]; ok && rawURL.Kind == ValueString {
				return rawURL.Text
			}
			return ""
		}
		if strings.EqualFold(v.Type, "Schema.SObjectType") {
			if objectName, ok := v.Fields["object"]; ok && objectName.Kind == ValueString {
				return objectName.Text
			}
		}
		if strings.EqualFold(v.Type, "Schema.SObjectField") {
			objectName, hasObject := v.Fields["object"]
			fieldName, hasField := v.Fields["field"]
			if hasObject && hasField && objectName.Kind == ValueString && fieldName.Kind == ValueString {
				return fieldName.Text
			}
		}
		if raw, ok := v.Fields["value"]; ok && raw.Kind == ValueString {
			if strings.EqualFold(v.Type, "Id") {
				return displayIDText(raw.Text)
			}
			return raw.Text
		}
		if message, ok := v.Fields["message"]; ok && message.Kind == ValueString {
			return message.Text
		}
		if text, ok := objectFieldsString(v, make(map[uint64]bool)); ok {
			return text
		}
		return fmt.Sprintf("%s:{}", v.Type)
	default:
		return fmt.Sprintf("<%s>", v.Kind)
	}
}

func objectFieldsString(v Value, seen map[uint64]bool) (string, bool) {
	if v.Kind != ValueObject || len(v.Fields) == 0 {
		return "", false
	}
	if v.Ref != 0 {
		if seen[v.Ref] {
			return fmt.Sprintf("%s:{...}", v.Type), true
		}
		seen[v.Ref] = true
		defer delete(seen, v.Ref)
	}
	keys := make([]string, 0, len(v.Fields))
	for key := range v.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+valueStringWithSeen(v.Fields[key], seen))
	}
	return fmt.Sprintf("%s:{%s}", v.Type, strings.Join(parts, ", ")), true
}

func valueStringWithSeen(v Value, seen map[uint64]bool) string {
	switch v.Kind {
	case ValueList:
		parts := make([]string, 0, len(v.List))
		for _, value := range v.List {
			parts = append(parts, valueStringWithSeen(value, seen))
		}
		return "List[" + strings.Join(parts, ", ") + "]"
	case ValueSet:
		parts := make([]string, 0, len(v.Set))
		for _, value := range v.Set {
			parts = append(parts, valueStringWithSeen(value, seen))
		}
		return "Set[" + strings.Join(parts, ", ") + "]"
	case ValueMap:
		if len(v.Map) == 0 {
			return "{}"
		}
		keys := sortedMapKeys(v.Map)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, valueFromMapKey(key).String()+"="+valueStringWithSeen(v.Map[key], seen))
		}
		return "Map{" + strings.Join(parts, ", ") + "}"
	case ValueObject:
		if text, ok := objectFieldsString(v, seen); ok {
			return text
		}
	}
	return v.String()
}

func decimalDisplayText(value Value) string {
	text := value.Text
	if text == "" {
		text = strconv.FormatFloat(value.Decimal, 'f', -1, 64)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if strings.ContainsAny(text, "eE") {
		if parsed, err := strconv.ParseFloat(text, 64); err == nil {
			text = strconv.FormatFloat(parsed, 'f', -1, 64)
		}
	}
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "-0" {
		return "0"
	}
	return text
}

func stubProxyTypeName(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	raw, ok := value.Fields["__gladeStubbedType"]
	if !ok || raw.Kind != ValueString || raw.Text == "" {
		return "", false
	}
	return raw.Text, true
}

func (v Value) Equal(other Value) bool {
	return v.equal(other, make(map[[2]uint64]bool))
}

func (v Value) equal(other Value, seen map[[2]uint64]bool) bool {
	if v.Ref != 0 && other.Ref != 0 {
		key := [2]uint64{v.Ref, other.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	if v.Kind != other.Kind {
		if v.Kind == ValueInt && other.Kind == ValueDecimal {
			return float64(v.Int) == other.Decimal
		}
		if v.Kind == ValueDecimal && other.Kind == ValueInt {
			return v.Decimal == float64(other.Int)
		}
		if v.Kind == ValueString && other.Kind == ValueObject && strings.EqualFold(other.Type, "Id") {
			if text, ok := platformScalarObjectText(other); ok {
				return apexIDTextEqual(v.Text, text)
			}
		}
		if v.Kind == ValueObject && strings.EqualFold(v.Type, "Id") && other.Kind == ValueString {
			if text, ok := platformScalarObjectText(v); ok {
				return apexIDTextEqual(text, other.Text)
			}
		}
		if v.Kind == ValueString && other.Kind == ValueObject && isStringComparableEnum(other.Type) {
			return v.Text == other.Text
		}
		if v.Kind == ValueObject && isStringComparableEnum(v.Type) && other.Kind == ValueString {
			return v.Text == other.Text
		}
		return false
	}
	switch v.Kind {
	case ValueNull:
		return true
	case ValueInt:
		return v.Int == other.Int
	case ValueDecimal:
		return v.Decimal == other.Decimal
	case ValueBool:
		return v.Bool == other.Bool
	case ValueString:
		if shouldCompareTextAsID(v.Text, other.Text) {
			return apexIDTextEqual(v.Text, other.Text)
		}
		return v.Text == other.Text
	case ValueList:
		if len(v.List) != len(other.List) {
			return false
		}
		for i := range v.List {
			if !v.List[i].equal(other.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(v.Set) != len(other.Set) {
			return false
		}
		for _, value := range v.Set {
			if !containsValue(other.Set, value) {
				return false
			}
		}
		return true
	case ValueMap:
		if len(v.Map) != len(other.Map) {
			return false
		}
		for key, value := range v.Map {
			otherValue, ok := other.Map[key]
			if !ok || !value.equal(otherValue, seen) {
				return false
			}
		}
		return true
	case ValueObject:
		if strings.EqualFold(v.Type, "Type") && strings.EqualFold(other.Type, "Type") {
			leftType := typeValueText(v)
			rightType := typeValueText(other)
			if leftType != "" || rightType != "" {
				return canonicalTypeValueText(leftType) == canonicalTypeValueText(rightType)
			}
		}
		if strings.EqualFold(v.Type, "Schema.SObjectType") && strings.EqualFold(other.Type, "Schema.SObjectType") {
			if sObjectTypeTokenEqual(v, other) {
				return true
			}
			return mapKey(v) == mapKey(other)
		}
		if strings.EqualFold(v.Type, "Schema.SObjectField") && strings.EqualFold(other.Type, "Schema.SObjectField") {
			return mapKey(v) == mapKey(other)
		}
		if platformScalarObject(v.Type) {
			value, ok := v.Fields["value"]
			otherValue, otherOK := other.Fields["value"]
			if !ok || !otherOK {
				return false
			}
			if strings.EqualFold(v.Type, "Id") && strings.EqualFold(other.Type, "Id") && value.Kind == ValueString && otherValue.Kind == ValueString {
				return apexIDTextEqual(value.Text, otherValue.Text)
			}
			if strings.EqualFold(v.Type, "Date") && strings.EqualFold(other.Type, "Date") {
				leftDate, leftErr := parsePlatformDate(v)
				rightDate, rightErr := parsePlatformDate(other)
				if leftErr == nil && rightErr == nil {
					return leftDate.Year() == rightDate.Year() && leftDate.Month() == rightDate.Month() && leftDate.Day() == rightDate.Day()
				}
			}
			if sameDateAndMidnightDatetime(v, other) || sameDateAndMidnightDatetime(other, v) {
				return true
			}
			return strings.EqualFold(v.Type, other.Type) && value.equal(otherValue, seen)
		}
		if v.Text != "" || other.Text != "" {
			return (strings.EqualFold(v.Type, other.Type) || namespaceQualifiedTypeEquivalent(v.Type, other.Type)) && v.Text == other.Text
		}
		if sObjectValueType(v.Type) && sObjectValueType(other.Type) {
			return sObjectValuesEqual(v, other, seen)
		}
		return strings.EqualFold(v.Type, other.Type) && fmt.Sprintf("%p", v.Fields) == fmt.Sprintf("%p", other.Fields)
	default:
		return false
	}
}

func sameDateAndMidnightDatetime(dateValue, datetimeValue Value) bool {
	if !strings.EqualFold(dateValue.Type, "Date") ||
		(!strings.EqualFold(datetimeValue.Type, "Datetime") && !strings.EqualFold(datetimeValue.Type, "DateTime")) {
		return false
	}
	date, err := parsePlatformDate(dateValue)
	if err != nil {
		return false
	}
	datetime, err := parsePlatformDatetime(datetimeValue)
	if err != nil {
		return false
	}
	return datetime.Hour() == 0 && datetime.Minute() == 0 && datetime.Second() == 0 && datetime.Nanosecond() == 0 &&
		date.Year() == datetime.Year() && date.Month() == datetime.Month() && date.Day() == datetime.Day()
}

func sObjectValueType(typeName string) bool {
	key := strings.ToLower(typeName)
	return strings.EqualFold(typeName, "sObject") || strings.EqualFold(typeName, "AggregateResult") ||
		isCommonSObjectTypeName(typeName) || strings.HasSuffix(key, "__c") ||
		strings.HasSuffix(key, "__e") || strings.HasSuffix(key, "__mdt") ||
		strings.HasSuffix(key, "__r")
}

func sObjectValuesEqual(left, right Value, seen map[[2]uint64]bool) bool {
	if !sObjectValueTypesEqual(left.Type, right.Type) {
		return false
	}
	for key, leftValue := range left.Fields {
		if isInternalSObjectField(key) {
			continue
		}
		_, rightValue, ok := objectFieldValue(right, key)
		if !ok {
			if sObjectMissingFieldEqualsImplicitDefault(left, key, leftValue) {
				continue
			}
			return false
		}
		if !leftValue.equal(rightValue, seen) {
			return false
		}
	}
	for key, rightValue := range right.Fields {
		if isInternalSObjectField(key) {
			continue
		}
		if _, _, ok := objectFieldValue(left, key); !ok {
			if sObjectMissingFieldEqualsImplicitDefault(right, key, rightValue) {
				continue
			}
			return false
		}
	}
	return true
}

func sObjectValueTypesEqual(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	leftBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(left))
	rightBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(right))
	if !strings.EqualFold(leftBase, rightBase) {
		return false
	}
	leftNamespaced := !strings.EqualFold(leftBase, canonicalRuntimePlatformType(left))
	rightNamespaced := !strings.EqualFold(rightBase, canonicalRuntimePlatformType(right))
	return leftNamespaced != rightNamespaced
}

func sObjectMissingFieldEqualsImplicitDefault(owner Value, field string, value Value) bool {
	if isExplicitSObjectField(owner, field) {
		return false
	}
	switch value.Kind {
	case ValueNull:
		return true
	case ValueBool:
		return !value.Bool
	case ValueInt:
		return value.Int == 0
	case ValueDecimal:
		return value.Decimal == 0
	case ValueString:
		return value.Text == ""
	default:
		return false
	}
}

func isStringComparableEnum(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Schema.DisplayType"), strings.EqualFold(typeName, "Schema.SOAPType"):
		return true
	default:
		return false
	}
}

func apexIDTextEqual(left, right string) bool {
	if len(left) >= 15 && len(right) >= 15 {
		return left[:15] == right[:15]
	}
	return left == right
}

func canonicalIDMapKey(value string) string {
	if len(value) >= 15 {
		return value[:15]
	}
	return value
}

func looksLikeID(value string) bool {
	if len(value) != 15 && len(value) != 18 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func looksLikeComparableIDText(value string) bool {
	if !looksLikeID(value) || len(value) < 3 {
		return false
	}
	_, ok := standardSObjectPrefixes[value[:3]]
	return ok
}

func shouldCompareTextAsID(left, right string) bool {
	if !looksLikeID(left) || !looksLikeID(right) {
		return false
	}
	if looksLikeComparableIDText(left) || looksLikeComparableIDText(right) {
		return true
	}
	return len(left) == 18 || len(right) == 18
}

func valueIdentityEqual(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueList, ValueSet, ValueMap, ValueObject:
		if left.Kind == ValueObject && schemaTokenIdentityEqual(left, right) {
			return true
		}
		if left.Kind == ValueObject && left.Text != "" && right.Text != "" &&
			(strings.EqualFold(left.Type, right.Type) || namespaceQualifiedTypeEquivalent(left.Type, right.Type)) {
			return strings.EqualFold(left.Text, right.Text)
		}
		return left.Ref != 0 && left.Ref == right.Ref
	default:
		return left.Equal(right)
	}
}

func schemaTokenIdentityEqual(left, right Value) bool {
	if !strings.EqualFold(left.Type, right.Type) {
		return false
	}
	switch {
	case strings.EqualFold(left.Type, "Schema.SObjectType"):
		leftObject, leftOK := left.Fields["object"]
		rightObject, rightOK := right.Fields["object"]
		return leftOK && rightOK && leftObject.Kind == ValueString && rightObject.Kind == ValueString && strings.EqualFold(leftObject.Text, rightObject.Text)
	case strings.EqualFold(left.Type, "Schema.SObjectField"):
		leftObject, leftObjectOK := left.Fields["object"]
		rightObject, rightObjectOK := right.Fields["object"]
		leftField, leftFieldOK := left.Fields["field"]
		rightField, rightFieldOK := right.Fields["field"]
		return leftObjectOK && rightObjectOK && leftFieldOK && rightFieldOK &&
			leftObject.Kind == ValueString && rightObject.Kind == ValueString &&
			leftField.Kind == ValueString && rightField.Kind == ValueString &&
			strings.EqualFold(leftObject.Text, rightObject.Text) &&
			strings.EqualFold(leftField.Text, rightField.Text)
	default:
		return false
	}
}

func queryResultRecordsList(value Value) (Value, bool) {
	if value.Kind != ValueMap {
		return Null, false
	}
	records, ok := value.Map[mapKey(String("records"))]
	if !ok {
		return Null, false
	}
	if records.Kind != ValueList {
		return Null, false
	}
	return records, true
}

func platformScalarObjectText(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	raw, ok := value.Fields["value"]
	if !ok || raw.Kind != ValueString {
		return "", false
	}
	return raw.Text, true
}

func typeValueText(value Value) string {
	if value.Text != "" {
		return value.Text
	}
	if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
		return raw.Text
	}
	return ""
}

func canonicalTypeValueText(text string) string {
	normalized := strings.TrimPrefix(text, "System.")
	switch strings.ToLower(normalized) {
	case "blob":
		return "Blob"
	case "boolean":
		return "Boolean"
	case "date":
		return "Date"
	case "datetime":
		return "Datetime"
	case "decimal":
		return "Decimal"
	case "double":
		return "Double"
	case "id":
		return "Id"
	case "integer":
		return "Integer"
	case "long":
		return "Long"
	case "object":
		return "Object"
	case "string":
		return "String"
	case "time":
		return "Time"
	case "type":
		return "Type"
	case "url":
		return "URL"
	case "void":
		return "Void"
	default:
		return text
	}
}

func platformScalarObject(typeName string) bool {
	switch typeName {
	case "Blob", "Date", "Datetime", "Id", "Time", "Type", "URL", "UUID":
		return true
	default:
		return false
	}
}

func mapKey(v Value) string {
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Schema.SObjectType") {
		if objectName, ok := v.Fields["object"]; ok && objectName.Kind == ValueString {
			return string(v.Kind) + ":" + v.Type + ":" + schemaTokenObjectKey(objectName.Text)
		}
	}
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Schema.SObjectField") {
		objectName, hasObject := v.Fields["object"]
		fieldName, hasField := v.Fields["field"]
		if hasObject && hasField && objectName.Kind == ValueString && fieldName.Kind == ValueString {
			return string(v.Kind) + ":" + v.Type + ":" + schemaTokenObjectKey(objectName.Text) + "." + schemaTokenFieldKey(fieldName.Text)
		}
	}
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Schema.ChildRelationship") {
		relationshipName, hasRelationship := v.Fields["relationshipName"]
		childSObject, hasChild := v.Fields["childSObject"]
		field, hasField := v.Fields["field"]
		if hasRelationship && hasChild && hasField && relationshipName.Kind == ValueString && childSObject.Kind == ValueObject && strings.EqualFold(childSObject.Type, "Schema.SObjectType") && field.Kind == ValueObject && strings.EqualFold(field.Type, "Schema.SObjectField") {
			childName, childOK := childSObject.Fields["object"]
			fieldName, fieldOK := field.Fields["field"]
			if childOK && fieldOK && childName.Kind == ValueString && fieldName.Kind == ValueString {
				return string(v.Kind) + ":" + v.Type + ":" + relationshipName.Text + "|" + childName.Text + "|" + fieldName.Text
			}
		}
	}
	if v.Kind == ValueObject && v.Type == "Type" && v.Text != "" {
		return string(v.Kind) + ":" + v.Type + ":" + v.Text
	}
	if v.Kind == ValueObject && platformScalarObject(v.Type) {
		if raw, ok := v.Fields["value"]; ok && raw.Kind == ValueString {
			if strings.EqualFold(v.Type, "Id") {
				return string(v.Kind) + ":" + v.Type + ":" + canonicalIDMapKey(raw.Text)
			}
			return string(v.Kind) + ":" + v.Type + ":" + raw.Text
		}
	}
	if v.Kind == ValueObject {
		if isStubProxy(v) {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
		if sObjectValueType(v.Type) {
			if key, ok := objectIDFieldMapKey(v); ok {
				return key
			}
			if key, ok := stableSObjectFieldMapKey(v, make(map[uint64]bool)); ok {
				return key
			}
		}
		if key, ok := stableObjectFieldMapKey(v, make(map[uint64]bool)); ok {
			return key
		}
		if v.Type != "" && !sObjectValueType(v.Type) && v.Ref != 0 {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
	}
	if v.Kind == ValueObject && v.Type != "" {
		if key, ok := objectIDFieldMapKey(v); ok {
			return key
		}
	}
	if v.Kind == ValueString && strings.EqualFold(v.Type, "Id") && looksLikeID(v.Text) {
		return string(ValueObject) + ":Id:" + canonicalIDMapKey(v.Text)
	}
	if v.Kind == ValueObject && v.Type != "" && v.Text != "" {
		return string(v.Kind) + ":" + v.Type + ":" + v.Text
	}
	return string(v.Kind) + ":" + v.String()
}

func objectIDFieldMapKey(v Value) (string, bool) {
	if v.Kind != ValueObject || v.Type == "" {
		return "", false
	}
	id, ok := v.Fields["Id"]
	if !ok {
		return "", false
	}
	text, ok := idValueText(id)
	if !ok || text == "" {
		return "", false
	}
	return string(v.Kind) + ":" + v.Type + ":" + canonicalIDMapKey(text), true
}

func stableObjectFieldMapKey(v Value, seen map[uint64]bool) (string, bool) {
	return stableFieldMapKey(v, seen, false)
}

func stableSObjectFieldMapKey(v Value, seen map[uint64]bool) (string, bool) {
	return stableFieldMapKey(v, seen, true)
}

func stableFieldMapKey(v Value, seen map[uint64]bool, allowSObject bool) (string, bool) {
	if v.Type == "" || len(v.Fields) == 0 || (!allowSObject && sObjectValueType(v.Type)) {
		return "", false
	}
	if v.Ref != 0 {
		if seen[v.Ref] {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10), true
		}
		seen[v.Ref] = true
		defer delete(seen, v.Ref)
	}
	names := make([]string, 0, len(v.Fields))
	for name := range v.Fields {
		if strings.HasPrefix(name, "__glade") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names)+2)
	parts = append(parts, string(v.Kind), v.Type)
	for _, name := range names {
		if !stableObjectMapKeyField(v.Fields[name]) {
			continue
		}
		parts = append(parts, name+"="+mapKeyWithSeen(v.Fields[name], seen))
	}
	if len(parts) == 2 {
		return "", false
	}
	return strings.Join(parts, ":"), true
}

func stableObjectMapKeyField(value Value) bool {
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString, ValueList, ValueSet, ValueMap:
		return true
	case ValueObject:
		return strings.EqualFold(value.Type, "Type") ||
			hasTypePrefixFold(value.Type, "Schema") ||
			platformScalarObject(value.Type) ||
			sObjectValueType(value.Type)
	default:
		return false
	}
}

func mapKeyWithSeen(v Value, seen map[uint64]bool) string {
	if v.Kind == ValueObject {
		if isStubProxy(v) {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
		if sObjectValueType(v.Type) {
			if key, ok := objectIDFieldMapKey(v); ok {
				return key
			}
			if key, ok := stableSObjectFieldMapKey(v, seen); ok {
				return key
			}
		}
		if key, ok := stableObjectFieldMapKey(v, seen); ok {
			return key
		}
		if v.Type != "" && !sObjectValueType(v.Type) && v.Ref != 0 {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
	}
	return mapKey(v)
}

func schemaTokenObjectKey(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot < len(name)-1 {
		name = name[dot+1:]
	}
	return strings.ToLower(stripSchemaNamespaceToken(name))
}

func schemaTokenFieldKey(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot < len(name)-1 {
		name = name[dot+1:]
	}
	return strings.ToLower(stripSchemaNamespaceToken(name))
}

func stripSchemaNamespaceToken(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	rest := name[first+2:]
	if strings.Contains(rest, "__") {
		return rest
	}
	return name
}

func sObjectTypeTokenEqual(left, right Value) bool {
	leftObject, leftOK := left.Fields["object"]
	rightObject, rightOK := right.Fields["object"]
	if !leftOK || !rightOK || leftObject.Kind != ValueString || rightObject.Kind != ValueString {
		return false
	}
	return strings.EqualFold(schemaTokenObjectKey(leftObject.Text), schemaTokenObjectKey(rightObject.Text))
}

func containsValue(values []Value, needle Value) bool {
	for _, value := range values {
		if value.Equal(needle) {
			return true
		}
	}
	return false
}

func valuesString(values []Value) string {
	out := "["
	for i, value := range values {
		if i > 0 {
			out += ", "
		}
		out += value.String()
	}
	return out + "]"
}

func mapString(values map[string]Value) string {
	if len(values) == 0 {
		return "{}"
	}
	keys := sortedMapKeys(values)
	out := "Map{"
	for i, key := range keys {
		if i > 0 {
			out += ", "
		}
		out += valueFromMapKey(key).String() + "=" + values[key].String()
	}
	return out + "}"
}

func sortedMapKeys(values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func orderedValueMapKeys(value Value) []string {
	if len(value.MapOrder) == 0 {
		return sortedMapKeys(value.Map)
	}
	keys := make([]string, 0, len(value.Map))
	seen := make(map[string]bool, len(value.Map))
	for _, key := range value.MapOrder {
		if _, ok := value.Map[key]; !ok || seen[key] {
			continue
		}
		keys = append(keys, key)
		seen[key] = true
	}
	for _, key := range sortedMapKeys(value.Map) {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}
