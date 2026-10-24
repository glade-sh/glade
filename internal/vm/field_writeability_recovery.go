package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Native ordinary custom-object DML ignores retained audit values on JSON,
// queried and cloned records. Only the copied payload loses these values.
func (vm *VM) filterSObjectAuditDMLFields(records []storage.Record, targets []*Value) {
	for i, target := range targets {
		if i >= len(records) || target == nil || target.Kind != ValueObject || target.classInstance {
			continue
		}
		marker := target.Fields[sobjectExplicitFieldsField]
		json := marker.Kind == ValueMap && marker.Runtime == jsonDeserializedSObjectFieldOrder
		_, cloned := target.Fields[sobjectCloneMarkerField]
		_, queried := target.Fields[sobjectQueriedFieldsField]
		if !json && !cloned && !queried {
			continue
		}
		_, definition, ok := vm.describeObjectDefinition(target.Type)
		if !ok || !creationTimestampOrdinaryCustomObject(definition) {
			continue
		}
		for _, name := range []string{"CreatedById", "CreatedDate", "LastModifiedById", "LastModifiedDate", "SystemModstamp", "IsDeleted"} {
			definition, field, ok := vm.sObjectFieldDefinition(target.Type, name)
			if !ok || !strings.EqualFold(definition.APIName, records[i].Object) ||
				field.Createable == nil || *field.Createable || field.Updateable == nil || *field.Updateable ||
				isUserSetSObjectField(*target, name) {
				continue
			}
			if name == "IsDeleted" {
				if field.Type != storage.FieldBoolean {
					continue
				}
			} else if strings.HasSuffix(name, "Date") || name == "SystemModstamp" {
				if field.Type != storage.FieldDateTime {
					continue
				}
			} else if field.Type != storage.FieldReference && field.Type != storage.FieldID {
				continue
			}
			for key := range records[i].Fields {
				if strings.EqualFold(key, field.APIName) {
					delete(records[i].Fields, key)
				}
			}
			for key := range records[i].ExplicitNulls {
				if strings.EqualFold(key, field.APIName) {
					delete(records[i].ExplicitNulls, key)
				}
			}
		}
	}
}

func (vm *VM) filterJSONInsertCreationTimestamp(records []storage.Record, targets []*Value, resolvedMode string) {
	vm.filterJSONCreationTimestamp("insert", records, targets, resolvedMode)
}

func (vm *VM) filterJSONUpdateCreationTimestamp(records []storage.Record, targets []*Value, resolvedMode string) {
	vm.filterJSONCreationTimestamp("update", records, targets, resolvedMode)
}

// Keep the original unmodified JSON insert contract. The additional Account
// paths are backed by owned source/transport 53/65, 62/67 and 67/67 sequences.
// Ordinary custom objects additionally require the resolved SYSTEM_MODE; their
// API 67 evidence is explicitly system mode, separate from default user mode.
// Writable assignments preserve caller CreatedDate, while triggers and storage
// use the generated/stored timestamp. Only the copied DML payload is changed.
func (vm *VM) filterJSONCreationTimestamp(op string, records []storage.Record, targets []*Value, resolvedMode string) {
	if op != "insert" && op != "update" {
		return
	}
	for i, target := range targets {
		if i >= len(records) || target == nil || target.Kind != ValueObject || target.classInstance {
			continue
		}
		marker, ok := target.Fields[sobjectExplicitFieldsField]
		if !ok || marker.Kind != ValueMap || marker.Runtime != jsonDeserializedSObjectFieldOrder {
			continue
		}
		// Presence matters: an unsaved clone carries a false-valued marker.
		if _, cloned := target.Fields[sobjectCloneMarkerField]; cloned {
			continue
		}
		definition, field, ok := vm.sObjectFieldDefinition(target.Type, "CreatedDate")
		if !ok || !strings.EqualFold(definition.APIName, records[i].Object) ||
			!strings.EqualFold(field.APIName, "CreatedDate") || field.Type != storage.FieldDateTime ||
			field.Createable == nil || *field.Createable || field.Updateable == nil || *field.Updateable ||
			!isExplicitSObjectField(*target, field.APIName) {
			continue
		}
		_, queried := target.Fields[sobjectQueriedFieldsField]
		_, assigned := target.Fields[sobjectUserSetFieldsField]
		legacyInsert := op == "insert" && !queried && !assigned
		if !legacyInsert {
			account := strings.EqualFold(definition.APIName, "Account")
			customSystem := resolvedMode == "SYSTEM_MODE" && creationTimestampOrdinaryCustomObject(definition)
			if (!account && !customSystem) || !jsonCreationTimestampMatchesRecord(*target, records[i], field.APIName) {
				continue
			}
			if assigned && !vm.creationTimestampWritableAssignments(*target, op) {
				continue
			}
			if queried && (op != "update" || !creationTimestampDMLQuery(*target, definition.APIName)) {
				continue
			}
		}
		for name := range records[i].Fields {
			if strings.EqualFold(name, field.APIName) {
				delete(records[i].Fields, name)
			}
		}
		for name := range records[i].ExplicitNulls {
			if strings.EqualFold(name, field.APIName) {
				delete(records[i].ExplicitNulls, name)
			}
		}
	}
}

// Require positive imported object metadata before the ordinary custom-object
// API shape check. Special custom categories retain their existing behavior.
func creationTimestampOrdinaryCustomObject(definition storage.ObjectDefinition) bool {
	if storage.IsKnownStandardObject(definition.APIName) || storage.IsCustomSettingDefinition(definition) ||
		storage.IsCustomMetadataDefinition(definition) || strings.TrimSpace(definition.Metadata["publishBehavior"]) != "" {
		return false
	}
	kind := strings.TrimSpace(definition.Metadata["kind"])
	if kind != "" && !strings.EqualFold(kind, "customObject") {
		return false
	}
	if !strings.EqualFold(kind, "customObject") && strings.TrimSpace(definition.Metadata["nameFieldType"]) == "" {
		return false
	}
	return hasSuffixFold(definition.APIName, "__c")
}

func jsonCreationTimestampMatchesRecord(target Value, record storage.Record, field string) bool {
	_, supplied, ok := objectFieldValue(target, field)
	if !ok || recordHasExplicitNullAlias(record, field) {
		return false
	}
	text, err := platformScalarText(supplied, "Datetime")
	if err != nil {
		return false
	}
	if _, err := parseDatetimeText(text); err != nil {
		return false
	}
	found := false
	for name, value := range record.Fields {
		if strings.EqualFold(name, field) {
			if value.Kind != storage.ValueDateTime || value.String != text {
				return false
			}
			found = true
		}
	}
	return found
}

func (vm *VM) creationTimestampWritableAssignments(target Value, op string) bool {
	marker := target.Fields[sobjectUserSetFieldsField]
	if marker.Kind != ValueMap || len(marker.Map) == 0 {
		return false
	}
	for encoded, flag := range marker.Map {
		key, ok := marker.MapKeys[encoded]
		if !ok || key.Kind != ValueString || encoded != mapKey(key) || flag.Kind != ValueBool || !flag.Bool {
			return false
		}
		_, field, ok := vm.sObjectFieldDefinition(target.Type, key.Text)
		if !ok || strings.EqualFold(field.APIName, "CreatedDate") || strings.EqualFold(field.APIName, "Id") ||
			vmCalculatedOrSummaryField(field) {
			return false
		}
		writable := storage.FieldFlagValue(field.Createable, true)
		if op == "update" {
			writable = storage.FieldFlagValue(field.Updateable, true)
		}
		if !writable {
			return false
		}
	}
	return true
}

func creationTimestampDMLQuery(target Value, objectName string) bool {
	marker := target.Fields[sobjectQueriedFieldsField]
	if !dmlAccessibleSObject(target) || marker.Kind != ValueMap || len(marker.Map) < 2 {
		return false
	}
	objectKey := mapKey(String("object"))
	object := marker.Map[objectKey]
	id := marker.Map[mapKey(String("id"))]
	if object.Kind != ValueString || !strings.EqualFold(object.Text, objectName) || id.Kind != ValueBool || !id.Bool {
		return false
	}
	stringKeyPrefix := mapKey(String(""))
	for key, flag := range marker.Map {
		if key != objectKey && (!strings.HasPrefix(key, stringKeyPrefix) || key == stringKeyPrefix || flag.Kind != ValueBool || !flag.Bool) {
			return false
		}
	}
	return true
}

// Only expose the generated timestamp on the observed Trigger.new views. This
// changes the returned map, never explicit/user-set markers or a DML payload.
func (vm *VM) triggerCreationTimestampPopulated(receiver Value, field string, value Value) bool {
	if !strings.EqualFold(field, "CreatedDate") || !isTriggerSObject(receiver) || receiver.Ref == 0 ||
		value.Kind != ValueObject || !strings.EqualFold(value.Type, "Datetime") {
		return false
	}
	update := vm.triggerGlobals["Trigger.isUpdate"].Bool
	afterInsert := vm.triggerGlobals["Trigger.isInsert"].Bool && vm.triggerGlobals["Trigger.isAfter"].Bool
	if !update && !afterInsert {
		return false
	}
	definition, timestamp, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok || timestamp.Type != storage.FieldDateTime || !strings.EqualFold(timestamp.APIName, "CreatedDate") ||
		(!strings.EqualFold(definition.APIName, "Account") && !creationTimestampOrdinaryCustomObject(definition)) {
		return false
	}
	text, err := platformScalarText(value, "Datetime")
	if err != nil {
		return false
	}
	if _, err := parseDatetimeText(text); err != nil {
		return false
	}
	_, id, ok := objectFieldValue(receiver, "Id")
	currentNew := vm.triggerGlobals["Trigger.newMap"]
	if !ok || id.Kind == ValueNull || currentNew.Kind != ValueMap {
		return false
	}
	current, ok := currentNew.Map[mapKey(id)]
	return ok && current.Ref == receiver.Ref
}
