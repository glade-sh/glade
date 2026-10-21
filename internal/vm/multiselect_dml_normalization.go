package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Normalize the copied DML input, preserving the caller and trigger order.
func (vm *VM) normalizeBeforeDMLMultiPicklistFields(records []storage.Record) {
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		definition := vm.describePreparedDefinition(objectName, vm.Org.Objects[objectName].Definition)
		for name, value := range records[i].Fields {
			field, known := definition.Fields[name]
			if !known || !emptyOptionalMultiPicklistSelection(field, value) {
				continue
			}
			// Keep a present null so update hydration cannot restore an old value.
			records[i].Fields[name] = storage.NullValue()
			if records[i].ExplicitNulls == nil {
				records[i].ExplicitNulls = make(map[string]bool)
			}
			records[i].ExplicitNulls[name] = true
		}
	}
}

func emptyOptionalMultiPicklistSelection(field storage.Field, value storage.Value) bool {
	return field.Type == storage.FieldMultiPicklist && !field.Required && !field.RestrictedPicklist &&
		value.Kind == storage.ValueString && value.String != "" && strings.Trim(value.String, ";") == ""
}
