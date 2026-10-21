package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

const scaleZeroNumberDynamicInputPrefix = "__glade_scale_zero_number_input:"

// Dynamic input identity differs from the Decimal
// string/typed view. Keep it inside the already hidden explicit-field marker;
// it is not an SObject field, JSON property, equality key or stored column.
func scaleZeroNumberCapturedField(definition storage.ObjectDefinition, field storage.Field) bool {
	return hasSuffixFold(definition.APIName, "__c") &&
		!storage.IsCustomSettingDefinition(definition) &&
		field.Type == storage.FieldDecimal && strings.EqualFold(field.DisplayType, "DOUBLE") &&
		field.Precision == 18 && field.ScaleSpecified && field.Scale == 0 &&
		strings.TrimSpace(field.Formula) == ""
}

func scaleZeroNumberDynamicInput(receiver Value, field string) (Value, bool) {
	marker := receiver.Fields[sobjectExplicitFieldsField]
	input, ok := marker.Fields[scaleZeroNumberDynamicInputPrefix+strings.ToLower(field)]
	return input, ok
}

func setScaleZeroNumberDynamicInput(receiver *Value, field string, input Value) {
	if receiver == nil || receiver.Kind != ValueObject {
		return
	}
	marker, ok := receiver.Fields[sobjectExplicitFieldsField]
	if !ok || marker.Kind != ValueMap {
		return
	}
	// Do not mutate an alias snapshot's previous provenance map.
	fields := make(map[string]Value, len(marker.Fields)+1)
	for key, value := range marker.Fields {
		fields[key] = value
	}
	key := scaleZeroNumberDynamicInputPrefix + strings.ToLower(field)
	if input.Kind == ValueNull {
		delete(fields, key)
	} else {
		fields[key] = input
	}
	marker.Fields = fields
	receiver.Fields[sobjectExplicitFieldsField] = marker
}

func clearScaleZeroNumberDynamicInput(receiver *Value, field string) {
	if receiver == nil {
		return
	}
	if _, exists := scaleZeroNumberDynamicInput(*receiver, field); exists {
		setScaleZeroNumberDynamicInput(receiver, field, Null)
	}
}

func (vm *VM) rememberScaleZeroNumberDynamicInput(receiver *Value, field string, fieldArg, input Value) {
	if receiver == nil || receiver.classInstance || (input.Kind != ValueInt && input.Kind != ValueDecimal) {
		return
	}
	definition, column, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok || !scaleZeroNumberCapturedField(definition, column) {
		return
	}
	entry := Value{Kind: ValueMap, Fields: map[string]Value{
		"value":     input,
		"stringPut": Bool(fieldArg.Kind == ValueString),
	}}
	setScaleZeroNumberDynamicInput(receiver, column.APIName, entry)
}

func (vm *VM) scaleZeroNumberTokenInput(receiver Value, field string, fieldArg Value) (Value, bool) {
	if fieldArg.Kind != ValueObject || !isSObjectFieldTokenType(fieldArg.Type) ||
		receiver.classInstance || isTriggerSObject(receiver) {
		return Null, false
	}
	definition, column, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok || !scaleZeroNumberCapturedField(definition, column) {
		return Null, false
	}
	entry, ok := scaleZeroNumberDynamicInput(receiver, column.APIName)
	input := entry.Fields["value"]
	if !ok || (input.Kind != ValueInt && input.Kind != ValueDecimal) {
		return Null, false
	}
	// number_token_provenance C01-C04/C17-C24 and PF_A/C/D confirm
	// explicit Integer token inputs across Object, Id, DML and query rewrites.
	// Untouched query values have no input marker. String and typed reads
	// continue using the normalized payload, not this dynamic input.
	_, materialized := receiver.Fields[sobjectQueriedFieldsField]
	withID := sObjectIDFromFields(receiver.Fields) != ""
	boxed := strings.EqualFold(input.Static, "Object")
	stringPut := entry.Fields["stringPut"].Bool
	queried := materialized && !dmlAccessibleSObject(receiver)
	if input.Kind == ValueDecimal {
		// PF_C_DECIMAL preserves a fresh Object Decimal token input's scale
		// after query. C10 preserves direct Double on a new Id-less record.
		if !stringPut && queried && boxed && !isFloatBackedDecimal(input) {
			return input, true
		}
		if !stringPut && !materialized && !withID && !boxed && isFloatBackedDecimal(input) {
			// Object parameter binding replaces Static. Retain the observed
			// runtime identity and public integral .0 rendering across it.
			input.Runtime = "Double"
			input.Text = doubleDisplayText(input.Decimal)
			input.scaleZeroNumberTokenDouble = true
			return input, true
		}
		return Null, false
	}
	// C12-C16 include boxed Integer String puts on new Id-less records.
	// String-origin token reads on Id-bearing/materialized records are unprobed.
	if stringPut && (withID || materialized || (boxed && isLongIntValue(input))) {
		return Null, false
	}
	// C07/the captured row cover direct Id-less Long; PF_C_LONG covers queried
	// Object Long token rewrites. Other boxed/Id/DML Long shapes are unprobed.
	if isLongIntValue(input) && (boxed || withID || materialized) && !(queried && boxed && !stringPut) {
		return Null, false
	}
	return input, true
}
