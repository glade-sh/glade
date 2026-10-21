package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Native source-62/67 JSON inserts through DEFAULT, USER_MODE, SYSTEM_MODE,
// keyword and DMLOptions routes retain the caller's Date, but expose
// null/unpopulated LastActivityDate in the owned insert observer and store
// null. Normalize only the copied Account insert payload.
func (vm *VM) filterJSONInsertLastActivityDate(records []storage.Record, targets []*Value) {
	for i, target := range targets {
		if i >= len(records) || target == nil || target.Kind != ValueObject || target.classInstance {
			continue
		}
		marker, ok := target.Fields[sobjectExplicitFieldsField]
		if !ok || marker.Kind != ValueMap || marker.Runtime != jsonDeserializedSObjectFieldOrder {
			continue
		}
		if _, cloned := target.Fields[sobjectCloneMarkerField]; cloned {
			continue
		}
		if _, queried := target.Fields[sobjectQueriedFieldsField]; queried {
			continue
		}
		if _, assigned := target.Fields[sobjectUserSetFieldsField]; assigned {
			continue
		}
		definition, field, ok := vm.sObjectFieldDefinition(target.Type, "LastActivityDate")
		if !ok || !strings.EqualFold(definition.APIName, "Account") ||
			!strings.EqualFold(records[i].Object, definition.APIName) ||
			!strings.EqualFold(field.APIName, "LastActivityDate") || field.Type != storage.FieldDate ||
			field.Createable == nil || *field.Createable || field.Updateable == nil || *field.Updateable ||
			field.Permissionable == nil || *field.Permissionable || !isExplicitSObjectField(*target, field.APIName) {
			continue
		}
		_, supplied, ok := objectFieldValue(*target, field.APIName)
		if !ok || supplied.Kind != ValueObject || supplied.Type != "Date" {
			continue
		}
		text, ok := platformScalarObjectText(supplied)
		if !ok || text == "" || recordHasExplicitNullAlias(records[i], field.APIName) {
			continue
		}
		if date, err := parseDateText(text); err != nil || date.Format("2006-01-02") != text {
			continue
		}
		for name, value := range records[i].Fields {
			if strings.EqualFold(name, field.APIName) && value.Kind == storage.ValueDate && value.String == text {
				delete(records[i].Fields, name)
			}
		}
	}
}
