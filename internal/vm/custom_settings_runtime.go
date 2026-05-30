package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func customDataArgsCacheKey(args []Value) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, strings.ToLower(arg.String()))
	}
	return strings.Join(parts, "|")
}
func (vm *VM) customDataOrgDefaultRecord(objectName string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	object := vm.Org.Objects[objectName]
	records := make([]storage.Record, 0, len(object.Records))
	for _, record := range object.Records {
		if !record.System.IsDeleted {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return storage.Record{}, false
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records[0], true
}
func unsupportedHierarchyCustomSettingStatic(definition storage.ObjectDefinition, typeName, method string) error {
	if !storage.IsCustomSettingDefinition(definition) {
		return nil
	}
	if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		return unsupportedCallError(typeName + "." + method + " hierarchy custom setting merge behavior")
	}
	return nil
}
func (vm *VM) hierarchyCustomSettingOrgDefaults(objectName, kind string) Value {
	if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, vm.orgID()); found {
		return vm.readOnlyCustomDataValue(record, kind)
	}
	return vm.readOnlyCustomDataDefaultValue(objectName, kind)
}
func (vm *VM) hierarchyCustomSettingRecordForOwner(objectName, ownerID string) (storage.Record, bool) {
	if vm.Org == nil || ownerID == "" {
		return storage.Record{}, false
	}
	object := vm.Org.Objects[objectName]
	for _, record := range sortedCustomDataRecords(object.Records, object.Definition, "custom setting", vm.Org.Namespace) {
		if record.System.IsDeleted {
			continue
		}
		value, ok := record.GetField("SetupOwnerId")
		if ok && value.Kind == storage.ValueString && value.String == ownerID {
			return record, true
		}
		if !ok {
			name, hasName := record.GetField("Name")
			if hasName && name.Kind == storage.ValueString && name.String == ownerID {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}
func (vm *VM) customDataObject(typeName string) (string, storage.ObjectDefinition, string, bool) {
	if vm.Org == nil {
		return "", storage.ObjectDefinition{}, "", false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		if definition, syntheticOK := syntheticListCustomSettingDefinition(typeName); syntheticOK {
			return definition.APIName, definition, "custom setting", true
		}
		return "", storage.ObjectDefinition{}, "", false
	}
	definition := vm.Org.Objects[objectName].Definition
	switch {
	case storage.IsCustomMetadataDefinition(definition):
		return objectName, definition, "custom metadata", true
	case storage.IsCustomSettingDefinition(definition):
		return objectName, definition, "custom setting", true
	default:
		return "", storage.ObjectDefinition{}, "", false
	}
}
func syntheticListCustomSettingDefinition(typeName string) (storage.ObjectDefinition, bool) {
	if !hasSuffixFold(typeName, "__c") {
		return storage.ObjectDefinition{}, false
	}
	return storage.ObjectDefinition{
		APIName: typeName,
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Name", Type: storage.FieldString, DisplayType: "STRING"},
		},
		Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"},
	}, true
}
func (vm *VM) customDataGetInstance(objectName string, definition storage.ObjectDefinition, kind string, args []Value) (storage.Record, bool, error) {
	object := vm.Org.Objects[objectName]
	if len(args) == 0 {
		if kind != "custom setting" {
			return storage.Record{}, false, fmt.Errorf("%s.getInstance expects record name", objectName)
		}
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, vm.orgID()); found {
				return record, true, nil
			}
			for _, record := range sortedCustomDataRecords(object.Records, definition, kind, vm.Org.Namespace) {
				if !record.System.IsDeleted && customSettingRecordHasNoSetupOwner(record) {
					return record, true, nil
				}
			}
			return storage.Record{}, false, nil
		}
		for _, record := range sortedCustomDataRecords(object.Records, definition, kind, vm.Org.Namespace) {
			if record.System.IsDeleted {
				continue
			}
			return record, true, nil
		}
		return storage.Record{}, false, nil
	}
	if len(args) != 1 || (args[0].Kind != ValueString && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
		return storage.Record{}, false, fmt.Errorf("%s.getInstance expects optional String name", objectName)
	}
	wanted := args[0].Text
	if args[0].Kind == ValueObject {
		text, err := platformScalarText(args[0], "Id")
		if err != nil {
			return storage.Record{}, false, err
		}
		wanted = text
	}
	if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, wanted); found {
			return record, true, nil
		}
		if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, vm.orgID()); found {
			return record, true, nil
		}
		return storage.Record{}, false, nil
	}
	for _, record := range sortedCustomDataRecords(object.Records, definition, kind, vm.Org.Namespace) {
		if record.System.IsDeleted {
			continue
		}
		if customDataRecordMatches(definition, kind, record, wanted, vm.Org.Namespace) {
			return record, true, nil
		}
	}
	return storage.Record{}, false, nil
}
func customSettingRecordHasNoSetupOwner(record storage.Record) bool {
	value, ok := record.GetField("SetupOwnerId")
	if !ok || value.Kind == storage.ValueNull {
		return true
	}
	return value.Kind == storage.ValueString && strings.TrimSpace(value.String) == ""
}
func sortedCustomDataRecords(records map[storage.ID]storage.Record, definition storage.ObjectDefinition, kind, namespace string) []storage.Record {
	out := make([]storage.Record, 0, len(records))
	for _, record := range records {
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		return customDataRecordLess(definition, kind, out[i], out[j], namespace)
	})
	return out
}
func customDataRecordLess(definition storage.ObjectDefinition, kind string, left, right storage.Record, namespace string) bool {
	leftKey := customDataRecordKey(definition, kind, left, namespace)
	rightKey := customDataRecordKey(definition, kind, right, namespace)
	if leftKey != rightKey {
		return leftKey < rightKey
	}
	return string(left.ID) < string(right.ID)
}
func customDataRecordMatches(definition storage.ObjectDefinition, kind string, record storage.Record, wanted, namespace string) bool {
	if string(record.ID) == wanted {
		return true
	}
	for _, candidate := range customDataRecordNames(definition, kind, record, namespace) {
		if strings.EqualFold(candidate, wanted) {
			return true
		}
	}
	return false
}
func customDataRecordKey(definition storage.ObjectDefinition, kind string, record storage.Record, namespace string) string {
	names := customDataRecordNames(definition, kind, record, namespace)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
func customDataRecordNames(definition storage.ObjectDefinition, kind string, record storage.Record, namespace string) []string {
	fieldOrder := []string{"Name"}
	if kind == "custom metadata" {
		fieldOrder = []string{"DeveloperName", "QualifiedApiName", "Name"}
	}
	var out []string
	for _, field := range fieldOrder {
		if value, ok := record.GetField(field); ok && value.Kind == storage.ValueString && value.String != "" {
			out = append(out, value.String)
		}
	}
	if kind == "custom metadata" {
		developerName := firstStringField(record, "DeveloperName", "Name")
		prefix := firstStringField(record, "NamespacePrefix")
		if developerName != "" && prefix != "" {
			out = append(out, prefix+"__"+developerName)
		}
		if developerName != "" && prefix == "" && namespace != "" && strings.HasPrefix(definition.APIName, namespace+"__") {
			out = append(out, namespace+"__"+developerName)
		}
	}
	return out
}
func (vm *VM) readOnlyCustomDataValue(record storage.Record, kind string) Value {
	value := vm.vmValueFromRecord(record)
	if kind == "custom metadata" {
		value.Fields[sobjectReadOnlyField] = String(kind + " records returned by getAll/getInstance are read-only")
	}
	return value
}
func (vm *VM) readOnlyCustomDataDefaultValue(objectName, kind string) Value {
	value := Object(objectName)
	if vm.Org != nil {
		if object, ok := vm.Org.Objects[objectName]; ok {
			for name, field := range object.Definition.Fields {
				if defaultValue, ok := storage.DefaultValueForField(field); ok {
					putVMFieldPath(value, name, vmValueFromStorage(defaultValue))
				}
			}
		}
	}
	if kind == "custom metadata" {
		value.Fields[sobjectReadOnlyField] = String(kind + " records returned by getAll/getInstance are read-only")
	}
	return value
}
