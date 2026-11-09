package vm

import (
	"maps"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// The compound payload is a value object, including inside whole-SObject JSON.
// Native sobject_json2 rows address/mailingAddress/location at source62/67 keep
// Address null properties under both serialize overloads.
func jsonCompoundFromValue(value Value, suppress bool, convert func(Value, bool) any) (any, bool) {
	typ := value.Type
	if value.Runtime == "System.Address" || value.Runtime == "System.Location" {
		typ = value.Runtime
	}
	if value.Kind != ValueObject || value.classInstance || value.projectClass {
		return nil, false
	}
	var names []string
	childSuppress := false
	switch typ {
	case "System.Address":
		names = []string{"city", "country", "countryCode", "geocodeAccuracy", "latitude", "longitude", "postalCode", "state", "stateCode", "street"}
	case "System.Location":
		names = []string{"latitude", "longitude"}
	default:
		if !strings.EqualFold(value.Type, "System.Address") {
			return nil, false
		}
		// The legacy case-insensitive Address path followed platform scalars
		// and propagated suppression to children. Preserve that ordering.
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar, true
		}
		names = []string{"city", "country", "countryCode", "geocodeAccuracy", "latitude", "longitude", "postalCode", "state", "stateCode", "street"}
		childSuppress = suppress
	}
	out := orderedJSONObject{}
	for _, name := range names {
		item, present := value.Fields[name]
		if !present && (name == "countryCode" || name == "stateCode") {
			continue
		}
		if !present {
			item = Null
		}
		out = append(out, orderedJSONField{name: name, value: convert(item, childSuppress)})
	}
	return out, true
}

// Deserialized compound fields use the same representation as query values.
func (vm *VM) jsonCompoundSObjectValue(value Value) Value {
	if value.Kind != ValueObject {
		return value
	}

	copied := false
	setField := func(name string, item Value) {
		if !copied {
			value.Fields = maps.Clone(value.Fields)
			copied = true
		}
		value.Fields[name] = item
	}
	for name, item := range value.Fields {
		if item.Kind == ValueDecimal {
			scaled := vm.jsonQueryNumericValue(value, name, item)
			if scaled.Text != item.Text {
				setField(name, scaled)
			}
		}
		if item.Kind != ValueMap {
			continue
		}
		_, field, ok := vm.sObjectFieldDefinition(value.Type, name)
		if !ok {
			continue
		}
		if field.Type == storage.FieldAddress {
			setField(name, coerceReadSObjectFieldRuntimeValue(value, item, field))
		}
		if field.Type == storage.FieldLocation {
			compound := Object("System.Location")
			for _, key := range []string{"latitude", "longitude"} {
				if v, ok := item.Map[mapKey(String(key))]; ok {
					compound.Fields[key] = v
				}
			}
			setField(name, compound)
		}
	}
	return value
}

func (vm *VM) materializeJSONQueryLocations(value *Value, record storage.Record, fields map[string]bool) {
	for name := range fields {
		_, field, ok := vm.sObjectFieldDefinition(record.Object, name)
		if !ok || field.Type != storage.FieldLocation {
			continue
		}
		full, ok := vm.findOrgRecord(record.Object, record.ID)
		if !ok {
			continue
		}
		location := Object("System.Location")
		base := strings.TrimSuffix(field.APIName, "__c")
		for _, component := range []struct{ suffix, key string }{{"__Latitude__s", "latitude"}, {"__Longitude__s", "longitude"}} {
			if stored, found := full.GetField(base + component.suffix); found && stored.Kind != storage.ValueNull {
				location.Fields[component.key] = vmValueFromStorage(stored)
			}
		}
		if len(location.Fields) > 0 {
			value.Fields[field.APIName] = location
		}
	}
}

func (vm *VM) jsonQueryNumericValue(value Value, name string, item Value) Value {
	if value.Fields[jsonSObjectQueryProjectionField].Kind != ValueString || isExplicitSObjectField(value, name) || item.Kind != ValueDecimal {
		return item
	}
	definition, field, ok := vm.jsonActiveSObjectField(value.Type, name)
	if !ok || field.Type != storage.FieldDecimal {
		return item
	}
	// Only schema-backed currency and coordinate display scales are captured.
	// Bare Decimal definitions keep their existing arbitrary precision.
	if !strings.EqualFold(field.DisplayType, "CURRENCY") && !strings.EqualFold(field.DisplayType, "DOUBLE") &&
		!strings.EqualFold(field.DisplayType, "PERCENT") {
		return item
	}
	if field.Scale == 0 && !strings.ContainsAny(item.Text, "eE") && strings.Contains(item.Text, ".") {
		// A queried scale-zero Currency, Number or Percent drops fractional
		// trailing zeroes (5.0 reads 5, 5.50 reads 5.5) without rounding.
		if normalized, err := decimalFromText(normalizeComputedDecimalText(item.Text)); err == nil {
			item = normalized
		}
	}
	scale := field.Scale
	if base, ok := definition.Fields[field.CompoundFieldName]; ok && base.Type == storage.FieldLocation {
		scale = base.Scale
	}
	// Native rows establish a minimum display scale, not a rounding rule.
	// Preserve all existing precision and only append required zeroes.
	if scale <= decimalScale(item) {
		return item
	}
	padded, err := roundDecimalValueToScale("JSON.serialize", item, int64(scale), "UNNECESSARY")
	if err != nil {
		return item
	}
	return padded
}

// Query serialization follows the active schema. Describe enrichment must not
// turn an explicitly supplied bare Decimal definition into a currency field.
func (vm *VM) jsonActiveSObjectField(typeName, name string) (storage.ObjectDefinition, storage.Field, bool) {
	_, definition, ok := vm.sObjectDefinitionForRead(typeName)
	if !ok {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	_, field, ok := storage.ResolveFieldDefinition(definition, vm.Org.Namespace, name)
	return definition, field, ok
}

// systemAddressJSONFieldTypes lists the System.Address members JSON.deserialize
// accepts. Native rows: ../apextest/testdata/conformance/json_system_address.
var systemAddressJSONFieldTypes = map[string]string{
	"city": "String", "country": "String", "countryCode": "String", "geocodeAccuracy": "String",
	"latitude": "Double", "longitude": "Double", "postalCode": "String", "state": "String",
	"stateCode": "String", "street": "String",
}

// systemAddressFromJSON deserializes into System.Address. A project class named
// Address keeps its own deserialization; only the System type is handled here.
func (vm *VM) systemAddressFromJSON(typeName string, raw any, input jsonTypedInput) (Value, bool, error) {
	if !strings.EqualFold(typeName, "System.Address") {
		if !strings.EqualFold(typeName, "Address") {
			return Null, false, nil
		}
		if class, ok := vm.lookupClass(typeName); ok && !class.PlatformType {
			return Null, false, nil
		}
	}
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return Null, true, jsonTypeMappingError("System.Address", raw)
	}
	address := newSystemAddress()
	// Absent components read null. Native serialize omits the code components
	// when they were not supplied, so those two stay absent.
	for name := range systemAddressJSONFieldTypes {
		if name != "stateCode" && name != "countryCode" {
			address.Fields[name] = Null
		}
	}
	for _, field := range fields {
		fieldType, known := systemAddressJSONFieldTypes[field.name]
		if !known {
			line, column := jsonInputPosition(field.source, systemAddressUnrecognizedOffset(field))
			return Null, true, jsonDeserializeException("Unrecognized field %q at [line:%d, column:%d]", field.name, line, column)
		}
		value, err := vm.typedValueFromJSON(fieldType, jsonTypedInput{value: field.value, source: field.source, start: field.enumStart}, false)
		if err != nil {
			return Null, true, err
		}
		address.Fields[field.name] = value
	}
	return address, true, nil
}

// systemAddressUnrecognizedOffset reports where native parsing stops after an
// unknown member: just inside a string, object or array value, and after a
// scalar value. Rows: json_system_address/native/unrecognized.tsv.
func systemAddressUnrecognizedOffset(field orderedJSONField) int {
	source := field.source
	i := field.start
	if i < 0 || i >= len(source) || source[i] != '"' {
		return field.end
	}
	for i++; i < len(source) && source[i] != '"'; i++ {
		if source[i] == '\\' {
			i++
		}
	}
	for i++; i < len(source) && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n' || source[i] == '\r' || source[i] == ':'); i++ {
	}
	if i < len(source) && (source[i] == '"' || source[i] == '{' || source[i] == '[') {
		return i + 1
	}
	return field.end
}

// jsonTriggerFormulaSObjectValue materializes the computed formula values of a
// Trigger.new or Trigger.old record for JSON.serialize (org capture
// blank_text_formula: the key is present for a computed value and absent for a
// blank one, on the trigger read surface). Other records serialize as they are.
func (vm *VM) jsonTriggerFormulaSObjectValue(value Value) Value {
	if value.Kind != ValueObject || vm == nil || vm.Org == nil || !isTriggerSObject(value) {
		return value
	}
	_, definition, ok := vm.sObjectDefinitionForRead(value.Type)
	if !ok {
		return value
	}
	record, haveRecord := storage.Record{}, false
	copied := false
	for name, field := range definition.Fields {
		if field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
			continue
		}
		if present, ok := value.Fields[name]; ok {
			// A materialized null is absent from the serialized record.
			if present.Kind == ValueNull {
				if !copied {
					value.Fields = maps.Clone(value.Fields)
					copied = true
				}
				delete(value.Fields, name)
			}
			continue
		}
		if !haveRecord {
			record, ok = vm.formulaRecordFromSObject(value)
			if !ok {
				return value
			}
			haveRecord = true
		}
		evaluated, isNull, ok := dml.EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, vm.Org, definition, record, vm.sObjectFormulaEvaluationOptions(value))
		if !ok {
			continue
		}
		if !copied {
			value.Fields = maps.Clone(value.Fields)
			copied = true
		}
		if isNull {
			delete(value.Fields, name)
			continue
		}
		formulaValue := vmValueFromStorage(evaluated)
		if calculatedDateFormulaBlankValue(field, formulaValue) {
			delete(value.Fields, name)
			continue
		}
		value.Fields[name] = formulaValue
	}
	return value
}
