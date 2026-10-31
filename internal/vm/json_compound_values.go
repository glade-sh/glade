package vm

import (
	"maps"
	"strings"

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
	if !strings.EqualFold(field.DisplayType, "CURRENCY") && !strings.EqualFold(field.DisplayType, "DOUBLE") {
		return item
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
