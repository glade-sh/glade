package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

const schemaCheckboxEqualityPrefix = "__glade_schema_checkbox_equality:"

// Native loaded-default rows preserve equality after a false Checkbox write
// on ordinary custom objects and hierarchy settings (named API 62/67).
// Number zero and unloaded-default controls do not acquire this equality
// presence; assignment and JSON markers stay intact. List settings stay outside
// the captured hierarchy-setting shape.
func rememberSchemaCheckboxEquality(receiver *Value, object storage.ObjectDefinition, field storage.Field, value storage.Value) {
	if receiver == nil || !hasSuffixFold(object.APIName, "__c") ||
		storage.IsCustomSettingDefinition(object) && !strings.EqualFold(object.Metadata["customSettingsType"], "Hierarchy") ||
		field.Type != storage.FieldBoolean || !strings.EqualFold(strings.TrimSpace(field.DefaultValue), "false") ||
		value.Kind != storage.ValueBoolean || value.Boolean {
		return
	}
	marker, ok := receiver.Fields[sobjectExplicitFieldsField]
	if !ok || marker.Kind != ValueMap {
		marker = Map()
		marker.Type = "Map<String,Boolean>"
	}
	fields := make(map[string]Value, len(marker.Fields)+1)
	for key, value := range marker.Fields {
		fields[key] = value
	}
	fields[schemaCheckboxEqualityPrefix+strings.ToLower(field.APIName)] = Bool(true)
	marker.Fields = fields
	receiver.Fields[sobjectExplicitFieldsField] = marker
}

func schemaLoadedCheckboxEqualityField(owner Value, field string, value Value) bool {
	return value.Kind == ValueBool && !value.Bool &&
		owner.Fields[sobjectExplicitFieldsField].Fields[schemaCheckboxEqualityPrefix+strings.ToLower(field)].Bool
}
