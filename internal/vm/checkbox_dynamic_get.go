package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// Owned named and anonymous API62/67 observations distinguish a fresh standard
// checkbox's raw dynamic value from its dotted false default. Only the
// constructor-default provenance is hidden; assigned and materialized values
// retain their separate read paths.
func (vm *VM) freshDefaultedCheckboxGetIsNull(receiver Value, field string, value Value) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentCallerNamespace() != "" || value.Kind != ValueBool || value.Bool ||
		!isDefaultedSObjectField(receiver, field) || isExplicitSObjectField(receiver, field) ||
		dmlAccessibleSObject(receiver) || isTriggerSObject(receiver) || isParentProjectionSObject(receiver) {
		return false
	}
	if _, queried := receiver.Fields[sobjectQueriedFieldsField]; queried {
		return false
	}
	if _, cloned := receiver.Fields[sobjectCloneMarkerField]; cloned {
		return false
	}
	if marker := receiver.Fields[sobjectExplicitFieldsField]; marker.Runtime == jsonDeserializedSObjectFieldOrder {
		return false
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 62 && major != 67 {
		return false
	}
	definition, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok || fieldDef.Type != storage.FieldBoolean || strings.TrimSpace(fieldDef.Formula) != "" ||
		storage.IsCustomMetadataDefinition(definition) || storage.IsCustomSettingDefinition(definition) {
		return false
	}
	standard, ok := storage.StandardObjectDefinition(definition.APIName)
	if !ok {
		return false
	}
	standardName, ok := storage.ResolveFieldName(standard, "", fieldDef.APIName)
	if !ok || standard.Fields[standardName].Type != storage.FieldBoolean {
		return false
	}
	if id := sObjectIDFromFields(receiver.Fields); id != "" {
		if _, stored := vm.findOrgRecord(definition.APIName, id); stored {
			return false
		}
	}
	return true
}
