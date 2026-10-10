package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

func customDataArgsCacheKey(args []Value) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, string(arg.Kind)+":"+arg.Type+":"+arg.String())
	}
	return strings.Join(parts, "|")
}

func customSettingOwnerIDArg(value Value) (string, bool, error) {
	switch {
	case value.Kind == ValueNull:
		return "", true, nil
	case value.Kind == ValueString:
		return value.Text, true, nil
	case value.Kind == ValueObject && strings.EqualFold(value.Type, "Id"):
		text, err := platformScalarText(value, "Id")
		if err != nil {
			return "", true, err
		}
		return text, true, nil
	default:
		return "", false, nil
	}
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

// getInstance merges non-null fields from organization, profile and user rows;
// getValues deliberately bypasses this merge (R226-R242 at APIs 62 and 67).
func (vm *VM) hierarchyCustomSettingInstance(objectName, kind string, args []Value) (Value, error) {
	ownerID := vm.currentUserID()
	if ownerID == "" && vm.Org != nil {
		// R227: use the same offline identity exposed by UserInfo.getUserId
		// when anonymous execution has no explicit execution user.
		ownerID = vm.currentUserInfoField("Id", "005000000000001")
	}
	if len(args) == 1 && args[0].Kind != ValueNull {
		var ok bool
		var err error
		ownerID, ok, err = customSettingOwnerIDArg(args[0])
		if err != nil {
			return Null, err
		}
		if !ok {
			return Null, fmt.Errorf("%s.getInstance expects optional setup owner Id", objectName)
		}
		if err := validateCustomSettingOwnerID(ownerID); err != nil {
			return Null, err
		}
	}
	owners := []string{vm.orgID()}
	if strings.HasPrefix(ownerID, "005") {
		profileID := ""
		if storage.IDsEqual(storage.ID(ownerID), storage.ID(vm.currentUserID())) {
			profileID = vm.currentUserInfoField("ProfileId", "")
		} else if vm.Org != nil {
			for id, user := range vm.Org.Objects["User"].Records {
				if storage.IDsEqual(id, storage.ID(ownerID)) {
					profileID = firstStringField(user, "ProfileId")
					break
				}
			}
		}
		if profileID != "" {
			owners = append(owners, profileID)
		}
	}
	if !storage.IDsEqual(storage.ID(ownerID), storage.ID(vm.orgID())) {
		owners = append(owners, ownerID)
	}
	merged := storage.Record{Object: objectName, Fields: make(map[string]storage.Value)}
	for _, owner := range owners {
		if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, owner); found {
			for name, value := range record.Fields {
				if value.Kind != storage.ValueNull {
					merged.Fields[name] = value
				}
			}
			if storage.IDsEqual(storage.ID(owner), storage.ID(ownerID)) {
				merged.ID, merged.System = record.ID, record.System
			}
		}
	}
	merged.Fields["SetupOwnerId"] = storage.IDValue(storage.ID(ownerID))
	return vm.readOnlyCustomDataValue(merged, kind), nil
}

func validateCustomSettingOwnerID(ownerID string) error {
	if validateApexIDShape(ownerID) != nil ||
		!(strings.HasPrefix(ownerID, "005") || strings.HasPrefix(ownerID, "00e") || strings.HasPrefix(ownerID, "00D")) {
		return newExceptionError("InvalidParameterValueException", "Invalid SetupOwner for Custom Settings: "+ownerID)
	}
	return nil
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
		if ok {
			var storedOwnerID string
			switch value.Kind {
			case storage.ValueID:
				storedOwnerID = string(value.ID)
			case storage.ValueString:
				storedOwnerID = value.String
			}
			if storage.IDsEqual(storage.ID(storedOwnerID), storage.ID(ownerID)) {
				return record, true
			}
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

// hierarchyCustomSettingRecordForCurrentContext follows Salesforce hierarchy
// custom-setting precedence for the no-argument getInstance accessor. A
// current-user row wins over a profile row, which wins over the organization
// row; an ownerless row is the final fallback for sparse local fixtures.
func (vm *VM) hierarchyCustomSettingRecordForCurrentContext(objectName string) (storage.Record, bool) {
	ownerIDs := make([]string, 0, 3)
	if userID := vm.currentUserID(); userID != "" && userID != "__run_as_user_without_id__" {
		ownerIDs = append(ownerIDs, userID)
	}
	if profileID := vm.currentUserInfoField("ProfileId", ""); profileID != "" {
		ownerIDs = append(ownerIDs, profileID)
	}
	if organizationID := vm.orgID(); organizationID != "" {
		ownerIDs = append(ownerIDs, organizationID)
	}
	for _, ownerID := range ownerIDs {
		if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, ownerID); found {
			return record, true
		}
	}
	return storage.Record{}, false
}
func (vm *VM) customDataObject(typeName string) (string, storage.ObjectDefinition, string, bool) {
	if vm == nil {
		return "", storage.ObjectDefinition{}, "", false
	}
	if vm.Org == nil {
		if definition, syntheticOK := syntheticListCustomSettingDefinition(typeName); syntheticOK {
			return definition.APIName, definition, "custom setting", true
		}
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

func customDataStaticAccessor(methodKey string) bool {
	switch strings.ToLower(strings.TrimSpace(methodKey)) {
	case "getall", "getinstance", "getorgdefaults", "getvalues":
		return true
	default:
		return false
	}
}

func (vm *VM) metadataLightCustomSettingObject(typeName, methodKey string) (string, storage.ObjectDefinition, string, bool) {
	if vm == nil || vm.Org == nil || !customDataStaticAccessor(methodKey) {
		return "", storage.ObjectDefinition{}, "", false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok || !hasSuffixFold(objectName, "__c") {
		return "", storage.ObjectDefinition{}, "", false
	}
	object := vm.Org.Objects[objectName]
	if storage.IsCustomMetadataDefinition(object.Definition) || storage.IsCustomSettingDefinition(object.Definition) {
		return "", storage.ObjectDefinition{}, "", false
	}
	definition, ok := syntheticListCustomSettingDefinition(objectName)
	if !ok {
		return "", storage.ObjectDefinition{}, "", false
	}
	if methodKey != "getall" {
		definition.Metadata["customSettingsType"] = "Hierarchy"
	}
	for fieldName, field := range object.Definition.Fields {
		definition.Fields[fieldName] = field
	}
	return objectName, definition, "custom setting", true
}

func (vm *VM) customDataGetInstance(objectName string, definition storage.ObjectDefinition, kind string, args []Value) (storage.Record, bool, error) {
	if vm.Org == nil {
		return storage.Record{}, false, nil
	}
	object := vm.Org.Objects[objectName]
	if len(args) == 0 {
		if kind != "custom setting" {
			return storage.Record{}, false, fmt.Errorf("%s.getInstance expects record name", objectName)
		}
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			if record, found := vm.hierarchyCustomSettingRecordForCurrentContext(objectName); found {
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
	if len(args) == 1 && args[0].Kind == ValueNull {
		if kind == "custom setting" && strings.EqualFold(definition.Metadata["customSettingsType"], "List") {
			// R263 returns raw null at the supported API floor and ceiling.
			// Retain the accepted API-37 first-inserted behavior below that floor.
			major, known := apexversion.Major(vm.currentMethod.APIVersion)
			if !known || major >= 62 {
				return storage.Record{}, false, nil
			}
			// Generated IDs retain the per-object insertion sequence. Names and
			// CreatedDate can change after insertion and cannot order this lookup.
			var first storage.Record
			found := false
			for _, record := range object.Records {
				if !record.System.IsDeleted && (!found || record.ID < first.ID) {
					first, found = record, true
				}
			}
			return first, found, nil
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
	// R253: getInstance(String.valueOf(queried.Id)) accepts the display ID,
	// while installed source-backed records can store its 15-character form.
	if record.ID != "" && validateApexIDShape(wanted) == nil && storage.IDsEqual(record.ID, storage.ID(wanted)) {
		return true
	}
	for _, candidate := range customDataRecordNames(definition, kind, record, namespace) {
		if candidate == wanted {
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
	// Cached accessors discard declared Number scale; SOQL retains it
	// (R216/R224, R245/R254). An integral cached Decimal still has scale one.
	for name, fieldValue := range value.Fields {
		if fieldValue.Kind != ValueDecimal {
			continue
		}
		if rational, ok := valueDecimalRat(fieldValue); ok {
			text := rational.FloatString(decimalScale(fieldValue))
			if strings.Contains(text, ".") {
				text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
			}
			if !strings.Contains(text, ".") {
				text += ".0"
			}
			if normalized, err := decimalFromText(text); err == nil {
				value.Fields[name] = normalized
			}
		}
	}
	if kind == "custom setting" && vm.Org != nil {
		if object, ok := vm.Org.Objects[record.Object]; ok {
			for name, field := range object.Definition.Fields {
				if _, exists := value.Fields[name]; !exists {
					if field.Type == storage.FieldBoolean {
						if defaultValue, ok := storage.DefaultValueForField(field); ok {
							value.Fields[name] = vmValueFromStorage(defaultValue)
							continue
						}
					}
					value.Fields[name] = Value{Kind: ValueNull, Type: string(object.Definition.Fields[name].Type)}
				}
			}
		}
	}
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
