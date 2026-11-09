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
		isTriggerSObject(receiver) || isParentProjectionSObject(receiver) {
		return false
	}
	if _, queried := receiver.Fields[sobjectQueriedFieldsField]; queried && !dmlAccessibleSObject(receiver) {
		return false
	}
	// rows: an unsaved clone and an inserted caller keep the raw null.
	// A clone that carries an Id stays outside the observed set.
	if _, cloned := receiver.Fields[sobjectCloneMarkerField]; cloned && sObjectIDFromFields(receiver.Fields) != "" {
		return false
	}
	if marker := receiver.Fields[sobjectExplicitFieldsField]; marker.Runtime == jsonDeserializedSObjectFieldOrder {
		return false
	}
	return vm.observedStandardCheckboxField(receiver, field, !dmlAccessibleSObject(receiver))
}

// unpopulatedStandardCheckboxGetIsNull reports a dynamic read of a standard
// checkbox the caller never populated. Owned API62/67 rows (
// standard_checkbox_new_sobject_get) read null from Schema newSObject, an
// inserted caller and a clone, as from a constructed record; dotted reads stay
// false. Queried, trigger, parent-projection and JSON-origin records keep
// their separate read paths.
func (vm *VM) unpopulatedStandardCheckboxGetIsNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentCallerNamespace() != "" ||
		isTriggerSObject(receiver) || isParentProjectionSObject(receiver) {
		return false
	}
	if _, queried := receiver.Fields[sobjectQueriedFieldsField]; queried && !dmlAccessibleSObject(receiver) {
		return false
	}
	if marker := receiver.Fields[sobjectExplicitFieldsField]; marker.Runtime == jsonDeserializedSObjectFieldOrder {
		return false
	}
	return vm.observedStandardCheckboxField(receiver, field, false)
}

// observedStandardCheckboxField checks the API version and that field is a
// plain standard Boolean of a standard object. API 68 joins 62/67 on the
// case289_followups recapture: every 68 isTest and anonymous observation,
// including the OPV3 dynamic get, equals 67 (ORG-RUN.txt, 2026-11-03).
func (vm *VM) observedStandardCheckboxField(receiver Value, field string, checkStored bool) bool {
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 62 && major != 67 && major != 68 {
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
	if id := sObjectIDFromFields(receiver.Fields); checkStored && id != "" {
		if _, stored := vm.findOrgRecord(definition.APIName, id); stored {
			return false
		}
	}
	return true
}
