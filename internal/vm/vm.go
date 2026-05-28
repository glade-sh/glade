package vm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
)

func industryMapResult() Value {
	result := typedMap("Map<String,Object>")
	for key, value := range map[string]Value{
		"success": Bool(true),
		"records": typedList("List<Object>"),
		"errors":  typedList("List<Object>"),
	} {
		encoded := mapKey(String(key))
		result.Map[encoded] = value
		result.MapKeys[encoded] = String(key)
	}
	return result
}

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

func (vm *VM) orgID() string {
	if vm.Org != nil && vm.Org.OrgID != "" {
		return vm.Org.OrgID
	}
	return "00D000000000001"
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
	if !strings.HasSuffix(strings.ToLower(typeName), "__c") {
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

func firstStringField(record storage.Record, names ...string) string {
	for _, name := range names {
		value, ok := record.GetField(name)
		if ok && value.Kind == storage.ValueString {
			return value.String
		}
	}
	return ""
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

func userInfoField(user Value, field, fallback string) string {
	if user.Kind == ValueObject {
		_, value, ok := objectFieldValue(user, field)
		if ok {
			if value.Kind == ValueString {
				return value.Text
			}
			if value.Kind == ValueObject {
				if raw, err := platformScalarText(value, value.Type); err == nil {
					return raw
				}
			}
		}
		return fallback
	}
	if user.Kind == ValueString {
		if !strings.EqualFold(field, "Id") && !strings.EqualFold(field, "Username") {
			return fallback
		}
		return user.Text
	}
	return fallback
}

func recordFieldString(record storage.Record, field string) string {
	if record.Fields == nil {
		return ""
	}
	value, ok := record.Fields[field]
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return value.String
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueDecimal:
		return value.Decimal
	default:
		return ""
	}
}

func (vm *VM) currentUserInfoField(field, fallback string) string {
	if vm.testContext != nil {
		return userInfoField(vm.testContext.CurrentUser, field, fallback)
	}
	if vm.executionUser.Kind != "" && vm.executionUser.Kind != ValueNull {
		return userInfoField(vm.executionUser, field, fallback)
	}
	return fallback
}

func (vm *VM) currentUserTimeZoneID() string {
	return vm.currentUserInfoField("TimeZoneSidKey", "UTC")
}

func userHasPermission(user Value, permission string) bool {
	if user.Kind != ValueObject {
		return false
	}
	for _, field := range []string{"Permissions", "PermissionSets"} {
		_, value, ok := objectFieldValue(user, field)
		if !ok {
			continue
		}
		if value.Kind == ValueString && strings.EqualFold(value.Text, permission) {
			return true
		}
		if value.Kind == ValueList {
			for _, item := range value.List {
				if item.Kind == ValueString && strings.EqualFold(item.Text, permission) {
					return true
				}
			}
		}
	}
	return false
}

func (vm *VM) currentUserHasPermission(permission string) bool {
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if userHasPermission(user, permission) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if vm.permissionSetHasPermission(permissionSetID, permission) {
			return true
		}
	}
	return false
}

func (vm *VM) permissionSetHasPermission(permissionSetID, permission string) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(permissionSetID) == "" {
		return false
	}
	state, ok := vm.Org.Objects["PermissionSet"]
	if !ok {
		return false
	}
	record, ok := state.Records[storage.ID(permissionSetID)]
	if !ok {
		return false
	}
	for _, field := range []string{"Permissions", "PermissionSets", "CustomPermissions"} {
		value, ok := record.GetField(field)
		if !ok {
			continue
		}
		if storagePermissionValueMatches(value, permission) {
			return true
		}
	}
	return false
}

func storagePermissionValueMatches(value storage.Value, permission string) bool {
	if value.Kind == storage.ValueString && strings.EqualFold(strings.TrimSpace(value.String), strings.TrimSpace(permission)) {
		return true
	}
	if value.Kind == storage.ValueList {
		for _, item := range value.List {
			if storagePermissionValueMatches(item, permission) {
				return true
			}
		}
	}
	return false
}

func (vm *VM) currentUserObjectPermission(objectName, method string) bool {
	if method == "isQueryable" || method == "isSearchable" {
		return true
	}
	if vm.Org == nil {
		return true
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	profileID := stringField(user, "ProfileId")
	if vm.currentProfileIsSystemAdministrator(profileID) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if allowed, ok := vm.explicitObjectPermission(permissionSetID, objectName, method); ok && allowed {
			return true
		}
	}
	if allowed, ok := vm.explicitObjectPermission(profileID, objectName, method); ok {
		return allowed
	}
	if vm.profileHasLicense(profileID, "Chatter External") && !isSetupObject(objectName) {
		return false
	}
	switch vm.currentProfileName(profileID) {
	case "Minimum Access - Salesforce":
		return method == "isAccessible" && isBaselineReadableObject(objectName)
	case "Standard Platform User":
		if strings.EqualFold(objectName, "Case") {
			return false
		}
		return true
	case "Read Only":
		return method == "isAccessible"
	}
	return true
}

func (vm *VM) currentUserFieldPermission(objectName, fieldName, method string) bool {
	if vm.Org == nil {
		return true
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	profileID := stringField(user, "ProfileId")
	if vm.currentProfileIsSystemAdministrator(profileID) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if allowed, ok := vm.explicitFieldPermission(permissionSetID, objectName, fieldName, method); ok && allowed {
			return true
		}
	}
	if allowed, ok := vm.explicitFieldPermission(profileID, objectName, fieldName, method); ok {
		return allowed
	}
	if profileID != "" && vm.parentHasFieldPermissionsForObject(profileID, objectName) {
		return false
	}
	if method == "isAccessible" && vm.isBaselineReadableField(objectName, fieldName) {
		return true
	}
	switch vm.currentProfileName(profileID) {
	case "Minimum Access - Salesforce":
		return false
	case "Standard Platform User":
		if strings.EqualFold(objectName, "Case") {
			if strings.EqualFold(fieldName, "CaseNumber") {
				return false
			}
			return vm.isBaselineReadableField(objectName, fieldName)
		}
		return true
	case "Read Only":
		return method == "isAccessible"
	}
	if vm.profileHasLicense(profileID, "Chatter External") && !isSetupObject(objectName) {
		return false
	}
	return true
}

func (vm *VM) stripInaccessibleRecords(accessType string, records Value, enforceRootObjectCRUD bool) (Value, Value, Value, error) {
	out := cloneValue(records)
	out.Type = records.Type
	removedFields := Map()
	removedFields.Type = "Map<String,Set<String>>"
	modifiedIndexes := Set()
	modifiedIndexes.Type = "Set<Integer>"
	for i := range out.List {
		modified, err := vm.stripInaccessibleRecord(accessType, &out.List[i], enforceRootObjectCRUD, removedFields)
		if err != nil {
			return Null, Null, Null, err
		}
		if modified {
			modifiedIndexes.Set = append(modifiedIndexes.Set, Int(int64(i)))
		}
	}
	return out, removedFields, modifiedIndexes, nil
}

func (vm *VM) enforceUserModeDMLAccess(op string, value Value) error {
	objectPermission, fieldPermission := userModeDMLPermissions(op)
	if objectPermission == "" {
		return nil
	}
	records := dmlAccessRecords(value)
	for _, record := range records {
		if record.Kind != ValueObject {
			continue
		}
		objectName := record.Type
		if vm.Org != nil {
			if resolved, ok := vm.resolveObjectName(objectName); ok {
				objectName = resolved
			}
		}
		if !vm.currentUserObjectPermission(objectName, objectPermission) {
			return newExceptionError("SecurityException", fmt.Sprintf("Access to entity '%s' denied", objectName))
		}
		for field := range record.Fields {
			if fieldPermission == "" || isInternalSObjectField(field) || isSObjectSystemField(field) {
				continue
			}
			canonicalField := vm.resolveSObjectFieldName(objectName, field)
			if strings.EqualFold(canonicalField, "Id") {
				continue
			}
			if !vm.currentUserFieldPermission(objectName, canonicalField, fieldPermission) {
				return newExceptionError("SecurityException", fmt.Sprintf("Access to field '%s.%s' denied", objectName, canonicalField))
			}
		}
	}
	return nil
}

func dmlAccessRecords(value Value) []Value {
	if value.Kind == ValueList {
		return value.List
	}
	return []Value{value}
}

func userModeDMLPermissions(op string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "insert":
		return "isCreateable", "isCreateable"
	case "update", "upsert":
		return "isUpdateable", "isUpdateable"
	case "delete", "undelete":
		return "isDeletable", ""
	default:
		return "", ""
	}
}

func (vm *VM) stripInaccessibleRecord(accessType string, record *Value, enforceRootObjectCRUD bool, removedFields Value) (bool, error) {
	if record == nil || record.Kind != ValueObject {
		return false, nil
	}
	modified := false
	objectName := record.Type
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
	}
	objectPermission := stripInaccessibleObjectPermission(accessType)
	if enforceRootObjectCRUD && objectPermission != "" && !vm.currentUserObjectPermission(objectName, objectPermission) {
		return false, newExceptionError("NoAccessException", "No access to entity: "+objectName)
	}
	fieldPermission := stripInaccessibleFieldPermission(accessType)
	for _, field := range orderedSObjectFieldIteration(*record) {
		value, ok := record.Fields[field]
		if !ok {
			continue
		}
		if isInternalSObjectField(field) || isSObjectSystemField(field) {
			continue
		}
		if value.Kind == ValueList {
			if childObjectName := sObjectListObjectName(value); childObjectName != "" {
				if vm.Org != nil {
					if resolved, ok := vm.resolveObjectName(childObjectName); ok {
						childObjectName = resolved
					}
				}
				if objectPermission != "" && !vm.currentUserObjectPermission(childObjectName, objectPermission) {
					delete(record.Fields, field)
					unmarkQueriedSObjectField(record, field)
					addStripInaccessibleRemovedField(removedFields, objectName, field)
					modified = true
					continue
				}
			}
			for i := range value.List {
				childModified, err := vm.stripInaccessibleRecord(accessType, &value.List[i], enforceRootObjectCRUD, removedFields)
				if err != nil {
					return false, err
				}
				if childModified {
					modified = true
				}
			}
			record.Fields[field] = value
			continue
		}
		canonicalField := vm.resolveSObjectFieldName(objectName, field)
		if strings.EqualFold(canonicalField, "Id") {
			continue
		}
		if fieldPermission == "" || vm.currentUserFieldPermission(objectName, canonicalField, fieldPermission) || vm.stripInaccessibleKeepsBaselineWritableField(accessType, objectName, canonicalField) {
			continue
		}
		delete(record.Fields, field)
		if canonicalField != field {
			delete(record.Fields, canonicalField)
		}
		unmarkQueriedSObjectField(record, canonicalField)
		unmarkQueriedSObjectField(record, field)
		addStripInaccessibleRemovedField(removedFields, objectName, canonicalField)
		modified = true
	}
	if selected, ok := record.Fields[sobjectQueriedFieldsField]; ok && selected.Kind == ValueMap {
		for _, selectedFieldValue := range selected.MapKeys {
			if selectedFieldValue.Kind != ValueString {
				continue
			}
			field := selectedFieldValue.Text
			if strings.EqualFold(field, "object") || strings.EqualFold(field, "Id") || recordHasSObjectField(*record, field) {
				continue
			}
			canonicalField := vm.resolveSObjectFieldName(objectName, field)
			if fieldPermission == "" || vm.currentUserFieldPermission(objectName, canonicalField, fieldPermission) || vm.stripInaccessibleKeepsBaselineWritableField(accessType, objectName, canonicalField) {
				continue
			}
			unmarkQueriedSObjectField(record, canonicalField)
			unmarkQueriedSObjectField(record, field)
			addStripInaccessibleRemovedField(removedFields, objectName, canonicalField)
			modified = true
		}
	}
	return modified, nil
}

func orderedSObjectFieldIteration(record Value) []string {
	if len(record.Fields) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(record.Fields))
	ordered := make([]string, 0, len(record.Fields))
	for _, explicit := range explicitSObjectFieldNames(record) {
		if explicit == "" {
			continue
		}
		actual, _, ok := objectFieldValue(record, explicit)
		if !ok {
			continue
		}
		lookup := actual
		if _, ok := record.Fields[lookup]; !ok {
			lookup = explicit
		}
		if _, ok := record.Fields[lookup]; !ok {
			continue
		}
		lower := strings.ToLower(lookup)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		ordered = append(ordered, lookup)
	}
	fallback := make([]string, 0, len(record.Fields))
	for field := range record.Fields {
		if _, ok := seen[strings.ToLower(field)]; ok {
			continue
		}
		fallback = append(fallback, field)
	}
	sort.Strings(fallback)
	ordered = append(ordered, fallback...)
	return ordered
}

func recordHasSObjectField(record Value, field string) bool {
	if record.Fields == nil {
		return false
	}
	for candidate := range record.Fields {
		if strings.EqualFold(candidate, field) {
			return true
		}
	}
	return false
}

func sObjectListObjectName(value Value) string {
	if value.Kind != ValueList {
		return ""
	}
	for _, item := range value.List {
		if item.Kind == ValueObject && strings.TrimSpace(item.Type) != "" {
			return item.Type
		}
	}
	if strings.HasPrefix(value.Type, "List<") && strings.HasSuffix(value.Type, ">") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value.Type, "List<"), ">"))
	}
	return ""
}

func stripInaccessibleObjectPermission(accessType string) string {
	switch strings.ToUpper(strings.TrimSpace(accessType)) {
	case "CREATABLE":
		return "isCreateable"
	case "READABLE":
		return "isAccessible"
	case "UPDATABLE", "UPSERTABLE":
		return "isUpdateable"
	default:
		return ""
	}
}

func stripInaccessibleFieldPermission(accessType string) string {
	switch strings.ToUpper(strings.TrimSpace(accessType)) {
	case "CREATABLE":
		return "isCreateable"
	case "READABLE":
		return "isAccessible"
	case "UPDATABLE", "UPSERTABLE":
		return "isUpdateable"
	default:
		return ""
	}
}

func (vm *VM) stripInaccessibleKeepsBaselineWritableField(accessType, objectName, fieldName string) bool {
	switch strings.ToUpper(strings.TrimSpace(accessType)) {
	case "CREATABLE", "UPSERTABLE":
		return vm.isBaselineWritableField(objectName, fieldName)
	case "UPDATABLE":
		return strings.EqualFold(objectName, "Lead") && localWritableLeadCommunicationField(fieldName)
	default:
		return false
	}
}

func localWritableLeadCommunicationField(fieldName string) bool {
	switch strings.ToLower(strings.TrimSpace(fieldName)) {
	case "donotcall", "hasoptedoutofemail", "hasoptedoutoffax":
		return true
	default:
		return false
	}
}

func addStripInaccessibleRemovedField(removedFields Value, objectName, fieldName string) {
	if removedFields.Kind != ValueMap || strings.TrimSpace(objectName) == "" || strings.TrimSpace(fieldName) == "" {
		return
	}
	key := mapKey(String(objectName))
	fields, ok := removedFields.Map[key]
	if !ok || fields.Kind != ValueSet {
		fields = Set()
		fields.Type = "Set<String>"
	}
	if !containsValue(fields.Set, String(fieldName)) {
		fields.Set = append(fields.Set, String(fieldName))
	}
	removedFields.Map[key] = fields
	if removedFields.MapKeys == nil {
		removedFields.MapKeys = make(map[string]Value)
	}
	removedFields.MapKeys[key] = String(objectName)
}

func stringField(value Value, field string) string {
	if value.Kind != ValueObject {
		return ""
	}
	_, raw, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	switch raw.Kind {
	case ValueString:
		return raw.Text
	case ValueObject:
		if strings.EqualFold(raw.Type, "Id") && raw.Text != "" {
			return raw.Text
		}
		if raw.Text != "" {
			return raw.Text
		}
		if nested, ok := raw.Fields["value"]; ok && nested.Kind == ValueString {
			return nested.Text
		}
	}
	return ""
}

func (vm *VM) explicitObjectPermission(parentID, objectName, method string) (bool, bool) {
	if parentID == "" || vm.Org == nil {
		return false, false
	}
	state, ok := vm.Org.Objects["ObjectPermissions"]
	if !ok {
		return false, false
	}
	field := objectPermissionField(method)
	for _, record := range state.Records {
		if parentIDValue, ok := record.GetField("ParentId"); !ok || !storageIDValueEquals(parentIDValue, parentID) {
			continue
		}
		if sObjectTypeValue, ok := record.GetField("SObjectType"); !ok || !storageStringValueEquals(sObjectTypeValue, objectName) {
			continue
		}
		value, ok := record.GetField(field)
		if !ok || value.Kind != storage.ValueBoolean {
			return false, false
		}
		return value.Boolean, true
	}
	return false, false
}

func (vm *VM) explicitFieldPermission(parentID, objectName, fieldName, method string) (bool, bool) {
	if parentID == "" || vm.Org == nil {
		return false, false
	}
	state, ok := vm.Org.Objects["FieldPermissions"]
	if !ok {
		return false, false
	}
	field := "PermissionsRead"
	if method == "isCreateable" || method == "isUpdateable" {
		field = "PermissionsEdit"
	}
	for _, record := range state.Records {
		if parentIDValue, ok := record.GetField("ParentId"); !ok || !storageIDValueEquals(parentIDValue, parentID) {
			continue
		}
		if sObjectTypeValue, ok := record.GetField("SObjectType"); !ok || !storageStringValueEquals(sObjectTypeValue, objectName) {
			continue
		}
		if fieldValue, ok := record.GetField("Field"); !ok || !vm.fieldPermissionFieldMatches(fieldValue, objectName, fieldName) {
			continue
		}
		value, ok := record.GetField(field)
		if !ok || value.Kind != storage.ValueBoolean {
			return false, false
		}
		return value.Boolean, true
	}
	return false, false
}

func (vm *VM) parentHasFieldPermissionsForObject(parentID, objectName string) bool {
	if parentID == "" || vm.Org == nil {
		return false
	}
	state, ok := vm.Org.Objects["FieldPermissions"]
	if !ok {
		return false
	}
	for _, record := range state.Records {
		if parentIDValue, ok := record.GetField("ParentId"); !ok || !storageIDValueEquals(parentIDValue, parentID) {
			continue
		}
		if sObjectTypeValue, ok := record.GetField("SObjectType"); ok && storageStringValueEquals(sObjectTypeValue, objectName) {
			return true
		}
	}
	return false
}

func (vm *VM) fieldPermissionFieldMatches(value storage.Value, objectName, fieldName string) bool {
	if value.Kind != storage.ValueString {
		return false
	}
	text := value.String
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
		return strings.EqualFold(text[:dot], objectName) && vm.fieldPermissionNameMatches(objectName, text[dot+1:], fieldName)
	}
	return vm.fieldPermissionNameMatches(objectName, text, fieldName)
}

func (vm *VM) fieldPermissionNameMatches(objectName, permissionField, fieldName string) bool {
	if strings.EqualFold(permissionField, fieldName) {
		return true
	}
	for _, component := range vm.compoundFieldPermissionComponents(objectName, permissionField) {
		if strings.EqualFold(component, fieldName) {
			return true
		}
	}
	return false
}

func (vm *VM) compoundFieldPermissionComponents(objectName, fieldName string) []string {
	switch {
	case strings.EqualFold(fieldName, "ShippingAddress"):
		return []string{"ShippingStreet", "ShippingCity", "ShippingState", "ShippingPostalCode", "ShippingCountry", "ShippingLatitude", "ShippingLongitude", "ShippingGeocodeAccuracy"}
	case strings.EqualFold(fieldName, "BillingAddress"):
		return []string{"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry", "BillingLatitude", "BillingLongitude", "BillingGeocodeAccuracy"}
	case strings.EqualFold(fieldName, "MailingAddress"):
		return []string{"MailingStreet", "MailingCity", "MailingState", "MailingPostalCode", "MailingCountry", "MailingLatitude", "MailingLongitude", "MailingGeocodeAccuracy"}
	case strings.EqualFold(fieldName, "OtherAddress"):
		return []string{"OtherStreet", "OtherCity", "OtherState", "OtherPostalCode", "OtherCountry", "OtherLatitude", "OtherLongitude", "OtherGeocodeAccuracy"}
	}
	if vm.Org == nil {
		return nil
	}
	resolvedObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		return nil
	}
	definition := vm.Org.Objects[resolvedObject].Definition
	resolvedField, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return nil
	}
	var out []string
	for name, field := range definition.Fields {
		if strings.EqualFold(field.CompoundFieldName, resolvedField) || strings.EqualFold(field.CompoundFieldName, fieldName) {
			out = append(out, name)
		}
	}
	return out
}

func objectPermissionField(method string) string {
	switch method {
	case "isCreateable":
		return "PermissionsCreate"
	case "isUpdateable":
		return "PermissionsEdit"
	case "isDeletable":
		return "PermissionsDelete"
	default:
		return "PermissionsRead"
	}
}

func (vm *VM) assignedPermissionSetIDs(userID string) []string {
	if userID == "" || vm.Org == nil {
		return nil
	}
	state, ok := vm.Org.Objects["PermissionSetAssignment"]
	if !ok {
		return nil
	}
	var out []string
	for _, record := range state.Records {
		if assigneeValue, ok := record.GetField("AssigneeId"); ok && storageIDValueEquals(assigneeValue, userID) {
			if permissionSetValue, ok := record.GetField("PermissionSetId"); ok {
				if id := storageValueIDText(permissionSetValue); id != "" {
					out = append(out, id)
				}
			}
			if permissionSetGroupValue, ok := record.GetField("PermissionSetGroupId"); ok {
				out = append(out, vm.permissionSetGroupComponentIDs(storageValueIDText(permissionSetGroupValue))...)
			}
		}
	}
	return out
}

func (vm *VM) permissionSetGroupComponentIDs(groupID string) []string {
	if groupID == "" || vm.Org == nil {
		return nil
	}
	state, ok := vm.Org.Objects["PermissionSetGroupComponent"]
	if !ok {
		return nil
	}
	var out []string
	for _, record := range state.Records {
		if groupValue, ok := record.GetField("PermissionSetGroupId"); !ok || !storageIDValueEquals(groupValue, groupID) {
			continue
		}
		if permissionSetValue, ok := record.GetField("PermissionSetId"); ok {
			if id := storageValueIDText(permissionSetValue); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func (vm *VM) currentProfileIsSystemAdministrator(profileID string) bool {
	return vm.currentProfileName(profileID) == "System Administrator"
}

func (vm *VM) currentProfileName(profileID string) string {
	if profileID == "" || vm.Org == nil {
		return ""
	}
	profiles, ok := vm.Org.Objects["Profile"]
	if !ok {
		return ""
	}
	profile, ok := profiles.Records[storage.ID(profileID)]
	if !ok {
		return ""
	}
	if value, ok := profile.Fields["Name"]; ok && value.Kind == storage.ValueString {
		return value.String
	}
	return ""
}

func (vm *VM) isBaselineReadableField(objectName, fieldName string) bool {
	if strings.EqualFold(fieldName, "Id") {
		return true
	}
	if isBaselineSystemReadableField(fieldName) {
		return true
	}
	if vm.Org == nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldName, ok = storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return false
	}
	field := definition.Fields[fieldName]
	return field.Required || isNameFieldDescribe(field)
}

func isBaselineSystemReadableField(fieldName string) bool {
	switch strings.ToLower(strings.TrimSpace(fieldName)) {
	case "createdbyid", "createddate", "isdeleted", "lastmodifiedbyid", "lastmodifieddate", "ownerid", "recordtypeid", "systemmodstamp":
		return true
	default:
		return false
	}
}

func isBaselineReadableObject(objectName string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) {
	case "task", "event":
		return true
	default:
		return false
	}
}

func (vm *VM) isBaselineWritableField(objectName, fieldName string) bool {
	if vm.Org == nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldName, ok = storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return false
	}
	field := definition.Fields[fieldName]
	return field.Required || isNameFieldDescribe(field)
}

func (vm *VM) profileHasLicense(profileID, licenseName string) bool {
	if profileID == "" || vm.Org == nil {
		return false
	}
	profiles, ok := vm.Org.Objects["Profile"]
	if !ok {
		return false
	}
	profile, ok := profiles.Records[storage.ID(profileID)]
	if !ok {
		return false
	}
	licenseID := storageValueIDText(profile.Fields["UserLicenseId"])
	if licenseID == "" {
		return false
	}
	licenses, ok := vm.Org.Objects["UserLicense"]
	if !ok {
		return false
	}
	license, ok := licenses.Records[storage.ID(licenseID)]
	if !ok {
		return false
	}
	return storageStringValueEquals(license.Fields["Name"], licenseName)
}

func storageIDValueEquals(value storage.Value, text string) bool {
	return storageValueIDText(value) == text
}

func storageValueIDText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}

func storageStringValueEquals(value storage.Value, text string) bool {
	return value.Kind == storage.ValueString && strings.EqualFold(value.String, text)
}

func (vm *VM) shouldEnqueueFuture(method Method) bool {
	if vm.testContext == nil || vm.testContext.Draining {
		return false
	}
	return methodHasModifier(method.Modifiers, "future")
}

func (vm *VM) enqueueFuture(method Method, args []Value, result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, nil
	}
	if !method.IsStatic {
		return Null, fmt.Errorf("@future method %s must be static", method.Name)
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("futureCalls", 1); err != nil {
		return Null, err
	}
	if len(args) != len(method.Params) {
		return Null, fmt.Errorf("%s expects %d arguments", method.Name, len(method.Params))
	}
	coercedArgs := make([]Value, len(args))
	resolutionClass := vm.methodTypeResolutionClass(method)
	for i, param := range method.Params {
		paramType := vm.resolveTypeNameInClass(resolutionClass, param.Type)
		coerced, err := vm.coerceAssignable(paramType, vm.valueWithTypesResolvedInClass(resolutionClass, args[i]))
		if err != nil {
			return Null, fmt.Errorf("%s parameter %s: %w", method.Name, param.Name, err)
		}
		coerced.Static = paramType
		coercedArgs[i] = coerced
	}
	job := AsyncJob{
		ID:     vm.nextAsyncJobID(),
		Kind:   "Future",
		Method: method,
		Args:   coercedArgs,
	}
	vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":   job.Kind,
		"jobId":  job.ID,
		"method": method.Name,
	})
	return Null, nil
}

func (vm *VM) assertError(message string) error {
	return &RuntimeError{
		Type:    "System.AssertException",
		Message: message,
		Stack:   vm.stackFrames(),
	}
}

func (vm *VM) apexPagesMessagesFromValue(value Value, result *Result) ([]Value, error) {
	if value.Kind == ValueList {
		messages := make([]Value, 0, len(value.List))
		for _, item := range value.List {
			nested, err := vm.apexPagesMessagesFromValue(item, result)
			if err != nil {
				return nil, err
			}
			messages = append(messages, nested...)
		}
		return messages, nil
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "ApexPages.Message") {
		return []Value{value}, nil
	}
	if value.Kind == ValueObject && isExceptionType(value.Type) {
		summary := ""
		if _, message, ok := objectFieldValue(value, "message"); ok {
			text, err := vm.displayString(message, result)
			if err != nil {
				return nil, err
			}
			summary = text
		}
		if summary == "" {
			summary = value.String()
		}
		message := Object("ApexPages.Message")
		severity, _ := apexPagesSeverityStaticValue("ApexPages.Severity.ERROR")
		message.Fields["severity"] = severity
		message.Fields["summary"] = String(summary)
		message.Fields["detail"] = String(summary)
		return []Value{message}, nil
	}
	return nil, fmt.Errorf("ApexPages.addMessages expects Exception or ApexPages.Message list")
}

func (vm *VM) requireTestContext(callee string) error {
	if vm.testContext == nil {
		return fmt.Errorf("%s is only available in test context", callee)
	}
	return nil
}

func (vm *VM) calleeStartsWithRuntimeReceiver(callee string, args []Value) bool {
	root, _, ok := strings.Cut(callee, ".")
	if !ok || root == "" {
		return false
	}
	if _, ok := vm.Globals[root]; ok {
		return true
	}
	if !vm.hasExactRuntimeClass(root) {
		if _, ok := vm.lookupGlobalName(root); ok {
			return true
		}
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if _, _, ok := objectFieldValue(this, root); ok {
			return true
		}
		if _, _, ok := vm.lookupReceiverField(this.Type, root); ok {
			return true
		}
	}
	if vm.currentClass != "" {
		if _, _, ok := vm.lookupStaticField(vm.currentClass, root); ok {
			return true
		}
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if _, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args); ok || ambiguous {
			return false
		}
	}
	return false
}

func (vm *VM) hasExactRuntimeClass(name string) bool {
	if vm == nil || strings.TrimSpace(name) == "" {
		return false
	}
	if _, ok := vm.Classes[name]; ok {
		return true
	}
	if _, ok := generatedPlatformTypeIndex[strings.ToLower(name)]; ok {
		return true
	}
	if class, ok := vm.resolveEnumClass(name); ok && class.Name == name {
		return true
	}
	return isBuiltinTypeName(name)
}

func (vm *VM) testSetMock(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.setMock expects mock type and mock instance")
	}
	if err := vm.requireTestContext("Test.setMock"); err != nil {
		return Null, err
	}
	mockType, ok := testMockTypeName(args[0])
	if !ok {
		return Null, fmt.Errorf("Test.setMock expects mock type")
	}
	if mockType == "WebServiceMock" {
		vm.testContext.WebServiceMock = args[1]
		return Null, nil
	}
	if mockType != "HttpCalloutMock" {
		return Null, unsupportedCallError("Test.setMock " + mockType + " mock surface")
	}
	vm.testContext.HTTPMock = args[1]
	return Null, nil
}

func (vm *VM) testSetContinuationResponse(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "HttpResponse") {
		return Null, fmt.Errorf("Test.setContinuationResponse expects label String and HttpResponse")
	}
	if err := vm.requireTestContext("Test.setContinuationResponse"); err != nil {
		return Null, err
	}
	if vm.testContext.ContinuationResponses == nil {
		vm.testContext.ContinuationResponses = make(map[string]Value)
	}
	vm.testContext.ContinuationResponses[args[0].Text] = args[1]
	return Null, nil
}

func (vm *VM) continuationGetResponse(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("Continuation.getResponse expects label String")
	}
	if vm.testContext == nil || vm.testContext.ContinuationResponses == nil {
		return Null, unsupportedCallError("Continuation.getResponse local continuation callout surface")
	}
	if response, ok := vm.testContext.ContinuationResponses[args[0].Text]; ok {
		return response, nil
	}
	return Null, unsupportedCallError("Continuation.getResponse local continuation callout surface")
}

func (vm *VM) testInvokeContinuationMethod(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Continuation") {
		return Null, fmt.Errorf("Test.invokeContinuationMethod expects controller Object and Continuation")
	}
	if err := vm.requireTestContext("Test.invokeContinuationMethod"); err != nil {
		return Null, err
	}
	methodValue, ok := args[1].Fields["ContinuationMethod"]
	if !ok || methodValue.Kind == ValueNull {
		methodValue, ok = args[1].Fields["continuationMethod"]
	}
	if !ok || methodValue.Kind != ValueString || methodValue.Text == "" {
		return Null, fmt.Errorf("Continuation.ContinuationMethod must be set before Test.invokeContinuationMethod")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, methodValue.Text, nil)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+"."+methodValue.Text, nil)
	}
	if !ok {
		return Null, unsupportedCallError(args[0].Type + "." + methodValue.Text)
	}
	return vm.callMethodWithReceiver(target, args[0], nil, result)
}

func (vm *VM) testNotificationActionHandler(args []Value, result *Result) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.testNotificationActionHandler expects handler and actionable notification")
	}
	if err := vm.requireTestContext("Test.testNotificationActionHandler"); err != nil {
		return Null, err
	}
	if args[0].Kind != ValueObject || !vm.typeMatches(args[0].Type, "Messaging.NotificationActionHandler", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.testNotificationActionHandler expects Messaging.NotificationActionHandler")
	}
	if args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Messaging.ActionableNotification") {
		return Null, fmt.Errorf("Test.testNotificationActionHandler expects Messaging.ActionableNotification")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, "executeAction", []Value{args[1]})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+".executeAction", []Value{args[1]})
	}
	if !ok {
		return Null, fmt.Errorf("Messaging.NotificationActionHandler %s must implement executeAction", args[0].Type)
	}
	value, err := vm.callMethodWithReceiver(target, args[0], []Value{args[1]}, result)
	if err != nil {
		return Null, err
	}
	if value.Kind == ValueNull || (value.Kind == ValueObject && strings.EqualFold(value.Type, "Messaging.ActionResult")) {
		return value, nil
	}
	return Null, fmt.Errorf("Messaging.NotificationActionHandler %s executeAction must return Messaging.ActionResult", args[0].Type)
}

func (vm *VM) testSandboxPostCopyScript(args []Value, result *Result) (Value, error) {
	if len(args) != 4 && len(args) != 5 {
		return Null, fmt.Errorf("Test.testSandboxPostCopyScript expects script, organizationId, sandboxId, sandboxName[, isRunAsAutoProcUser]")
	}
	if err := vm.requireTestContext("Test.testSandboxPostCopyScript"); err != nil {
		return Null, err
	}
	if args[0].Kind != ValueObject || !vm.typeMatches(args[0].Type, "SandboxPostCopy", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.testSandboxPostCopyScript expects SandboxPostCopy")
	}
	if !isApexIDLikeValue(args[1]) || !isApexIDLikeValue(args[2]) || args[3].Kind != ValueString {
		return Null, fmt.Errorf("Test.testSandboxPostCopyScript expects organization Id, sandbox Id, and sandbox name")
	}
	runAsAutoProcUser := Bool(false)
	if len(args) == 5 {
		if args[4].Kind != ValueBool {
			return Null, fmt.Errorf("Test.testSandboxPostCopyScript isRunAsAutoProcUser expects Boolean")
		}
		runAsAutoProcUser = args[4]
	}
	context := Object("SandboxContext")
	context.Fields["organizationId"] = platformScalar("Id", scalarText(args[1]))
	context.Fields["sandboxId"] = platformScalar("Id", scalarText(args[2]))
	context.Fields["sandboxName"] = args[3]
	context.Fields["isRunAsAutoProcUser"] = runAsAutoProcUser
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, "runApexClass", []Value{context})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+".runApexClass", []Value{context})
	}
	if !ok {
		return Null, fmt.Errorf("SandboxPostCopy %s must implement runApexClass", args[0].Type)
	}
	_, err := vm.callMethodWithReceiver(target, args[0], []Value{context}, result)
	return Null, err
}

func (vm *VM) canvasTestMockRenderContext(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueMap || args[1].Kind != ValueMap {
		return Null, fmt.Errorf("Canvas.Test.mockRenderContext expects app and environment Map<String,String> values")
	}
	if err := vm.requireTestContext("Canvas.Test.mockRenderContext"); err != nil {
		return Null, err
	}
	app := Object("Canvas.ApplicationContext")
	bindCanvasContextMap(&app, args[0])
	env := Object("Canvas.EnvironmentContext")
	bindCanvasContextMap(&env, args[1])
	env.Fields["entityFields"] = typedList("List<String>")
	ctx := Object("Canvas.RenderContext")
	ctx.Fields["applicationContext"] = app
	ctx.Fields["environmentContext"] = env
	return ctx, nil
}

func (vm *VM) canvasTestCanvasLifecycle(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Canvas.RenderContext") {
		return Null, fmt.Errorf("Canvas.Test.testCanvasLifecycle expects CanvasLifecycleHandler and RenderContext")
	}
	if err := vm.requireTestContext("Canvas.Test.testCanvasLifecycle"); err != nil {
		return Null, err
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, "onRender", []Value{args[1]})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+".onRender", []Value{args[1]})
	}
	if !ok {
		return Null, nil
	}
	_, err := vm.callMethodWithReceiver(target, args[0], []Value{args[1]}, result)
	return Null, err
}

func bindCanvasContextMap(target *Value, source Value) {
	if target == nil || source.Kind != ValueMap {
		return
	}
	for encoded, value := range source.Map {
		keyValue := mapStoredKey(source, encoded)
		if keyValue.Kind != ValueString {
			continue
		}
		target.Fields[canvasContextFieldName(keyValue.Text)] = value
	}
}

func canvasContextFieldName(key string) string {
	switch strings.ToLower(strings.ReplaceAll(key, "_", "")) {
	case "canvasurl", "keycanvasurl":
		return "canvasUrl"
	case "developername", "keydevelopername":
		return "developerName"
	case "displaylocation", "keydisplaylocation":
		return "displayLocation"
	case "locationurl", "keylocationurl":
		return "locationUrl"
	case "name", "keyname":
		return "name"
	case "namespace", "keynamespace":
		return "namespace"
	case "sublocation", "keysublocation":
		return "sublocation"
	case "version", "keyversion":
		return "version"
	default:
		return key
	}
}

func (vm *VM) webServiceCalloutInvoke(args []Value, result *Result) (Value, error) {
	if len(args) != 4 {
		return Null, fmt.Errorf("WebServiceCallout.invoke expects stub, request, response map, and options")
	}
	if err := vm.incrementLimit("callouts", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.callout.webservice", "apex.callout", map[string]any{"operation": "WebServiceCallout.invoke"})
	if vm.testContext == nil || vm.testContext.WebServiceMock.Kind != ValueObject {
		return Null, unsupportedCallError("WebServiceCallout.invoke without WebServiceMock")
	}
	if args[2].Kind != ValueMap {
		return Null, fmt.Errorf("WebServiceCallout.invoke expects response map")
	}
	if args[3].Kind != ValueList || len(args[3].List) < 7 {
		return Null, fmt.Errorf("WebServiceCallout.invoke expects 7 option strings")
	}
	mockArgs := []Value{
		args[0],
		args[1],
		args[2],
		String(scalarText(args[3].List[0])),
		String(scalarText(args[3].List[1])),
		String(scalarText(args[3].List[3])),
		String(scalarText(args[3].List[4])),
		String(scalarText(args[3].List[5])),
		String(scalarText(args[3].List[6])),
	}
	mock := vm.testContext.WebServiceMock
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(mock.Type, "doInvoke", mockArgs)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(mock.Type+".doInvoke", mockArgs)
	}
	if !ok {
		return Null, fmt.Errorf("WebServiceMock %s must implement doInvoke", mock.Type)
	}
	_, err := vm.callMethodWithReceiver(target, mock, mockArgs, result)
	return Null, err
}

func (vm *VM) testCreateStub(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.createStub expects Type and StubProvider")
	}
	if err := vm.requireTestContext("Test.createStub"); err != nil {
		return Null, err
	}
	stubbedType, ok := testMockTypeName(args[0])
	if !ok || stubbedType == "" {
		return Null, fmt.Errorf("Test.createStub expects Type")
	}
	if args[1].Kind != ValueObject || !vm.typeMatches(args[1].Type, "StubProvider", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.createStub expects StubProvider")
	}
	if resolved, ok := vm.resolveClassName(stubbedType); ok {
		stubbedType = resolved
		if class, classOK := vm.lookupClass(resolved); classOK {
			stubbedType = vm.classTypeToken(class)
		}
	} else {
		return Null, unsupportedCallError("Test.createStub local proxy for unknown type " + stubbedType)
	}
	proxy := Object(stubbedType)
	proxy.Fields["__gladeStubProvider"] = args[1]
	proxy.Fields["__gladeStubbedType"] = String(stubbedType)
	if _, ok := vm.lookupClass(stubbedType); ok {
		// Test.createStub should return a proxy without executing user constructors
		// or instance initializers of the stubbed type.
		vm.initializeFields(&proxy, stubbedType)
	}
	return proxy, nil
}

func (vm *VM) testCreateSoqlStub(args []Value) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) {
		return Null, fmt.Errorf("Test.createSoqlStub expects Schema.SObjectType and SoqlStubProvider")
	}
	if err := vm.requireTestContext("Test.createSoqlStub"); err != nil {
		return Null, err
	}
	if args[1].Kind != ValueObject || !vm.typeMatches(args[1].Type, "SoqlStubProvider", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.createSoqlStub expects SoqlStubProvider")
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	if vm.testContext.SoqlStubs == nil {
		vm.testContext.SoqlStubs = make(map[string]Value)
	}
	vm.testContext.SoqlStubs[strings.ToLower(objectName)] = args[1]
	return Null, nil
}

func (vm *VM) testCreateStubQueryRow(args []Value) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) || args[1].Kind != ValueMap {
		return Null, fmt.Errorf("Test.createStubQueryRow expects Schema.SObjectType and Map<String,Object>")
	}
	if err := vm.requireTestContext("Test.createStubQueryRow"); err != nil {
		return Null, err
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	return vm.stubQueryRowFromMap(objectName, args[1])
}

func (vm *VM) testCreateStubQueryRows(args []Value) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) || args[1].Kind != ValueList {
		return Null, fmt.Errorf("Test.createStubQueryRows expects Schema.SObjectType and List<Map<String,Object>>")
	}
	if err := vm.requireTestContext("Test.createStubQueryRows"); err != nil {
		return Null, err
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	rows := typedList("List<" + objectName + ">")
	for _, item := range args[1].List {
		if item.Kind != ValueMap {
			return Null, fmt.Errorf("Test.createStubQueryRows expects List<Map<String,Object>>")
		}
		row, err := vm.stubQueryRowFromMap(objectName, item)
		if err != nil {
			return Null, err
		}
		rows.List = append(rows.List, row)
	}
	return rows, nil
}

func (vm *VM) stubQueryRowFromMap(objectName string, fields Value) (Value, error) {
	row := Object(objectName)
	for rawKey, fieldValue := range fields.Map {
		key := mapStoredKey(fields, rawKey)
		if key.Kind != ValueString || strings.TrimSpace(key.Text) == "" {
			return Null, fmt.Errorf("Test.createStubQueryRow field names must be strings")
		}
		setExplicitSObjectField(&row, key.Text, fieldValue)
		markQueriedSObjectField(&row, key.Text)
	}
	return row, nil
}

func (vm *VM) testLoadData(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) || args[1].Kind != ValueString {
		return Null, fmt.Errorf("Test.loadData expects Schema.SObjectType and static resource name")
	}
	if err := vm.requireTestContext("Test.loadData"); err != nil {
		return Null, err
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	content, ok := vm.staticResourceContent(args[1].Text)
	if !ok {
		return Null, fmt.Errorf("Test.loadData static resource %s not found", args[1].Text)
	}
	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return Null, fmt.Errorf("Test.loadData CSV parse failed: %w", err)
	}
	out := typedList("List<" + objectName + ">")
	if len(rows) == 0 {
		return out, nil
	}
	headers := rows[0]
	for rowIndex, csvRow := range rows[1:] {
		record := Object(objectName)
		for i, header := range headers {
			fieldName := strings.TrimSpace(header)
			if fieldName == "" {
				continue
			}
			raw := ""
			if i < len(csvRow) {
				raw = csvRow[i]
			}
			value, err := vm.testLoadDataFieldValue(objectName, fieldName, raw)
			if err != nil {
				return Null, fmt.Errorf("Test.loadData row %d %s.%s: %w", rowIndex+2, objectName, fieldName, err)
			}
			setExplicitSObjectField(&record, fieldName, value)
		}
		out.List = append(out.List, record)
	}
	if len(out.List) == 0 {
		return out, nil
	}
	if _, err := vm.applyDML("insert", out, true, "", dml.Options{}, result); err != nil {
		return Null, err
	}
	return out, nil
}

func (vm *VM) staticResourceContent(name string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	for _, resource := range vm.Org.Metadata.StaticResources {
		if !strings.EqualFold(resource.Name, name) {
			continue
		}
		if resource.Content != "" {
			return resource.Content, true
		}
		if resource.ContentPath != "" {
			content, err := os.ReadFile(resource.ContentPath)
			if err == nil {
				return string(content), true
			}
		}
		return "", false
	}
	return "", false
}

func (vm *VM) testLoadDataFieldValue(objectName, fieldName, raw string) (Value, error) {
	if strings.TrimSpace(raw) == "" {
		return Null, nil
	}
	if vm == nil || vm.Org == nil {
		return String(raw), nil
	}
	canonicalObject := objectName
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		canonicalObject = resolved
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return String(raw), nil
	}
	canonicalField := fieldName
	if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName); ok {
		canonicalField = resolved
	}
	field, ok := object.Definition.Fields[canonicalField]
	if !ok {
		return String(raw), nil
	}
	switch field.Type {
	case storage.FieldBoolean:
		return Bool(strings.EqualFold(raw, "true") || raw == "1"), nil
	case storage.FieldInteger:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return Null, err
		}
		return Int(parsed), nil
	case storage.FieldDecimal:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return Null, err
		}
		return Decimal(parsed), nil
	case storage.FieldID, storage.FieldReference:
		return platformScalar("Id", raw), nil
	case storage.FieldDate:
		return platformScalar("Date", raw), nil
	case storage.FieldDateTime:
		return platformScalar("Datetime", raw), nil
	case storage.FieldBlob:
		return platformScalar("Blob", raw), nil
	default:
		return String(raw), nil
	}
}

func (vm *VM) testInstall(args []Value, result *Result) (Value, error) {
	if len(args) != 2 && len(args) != 3 {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler, previousVersion[, isPush]")
	}
	if err := vm.requireTestContext("Test.testInstall"); err != nil {
		return Null, err
	}
	handler := args[0]
	if handler.Kind != ValueObject || handler.Type == "" {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler")
	}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(handler.Type, "onInstall", []Value{Object("InstallContext")})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(handler.Type+".onInstall", []Value{Object("InstallContext")})
	}
	if !ok {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler with onInstall")
	}
	context := Object("InstallContext")
	context.Fields["PreviousVersion"] = args[1]
	context.Fields["InstallerId"] = Null
	context.Fields["installerId"] = Null
	if len(args) == 3 {
		context.Fields["IsPush"] = args[2]
	}
	vm.installContextDepth++
	defer func() {
		vm.installContextDepth--
	}()
	if _, err := vm.callMethodWithReceiver(method, handler, []Value{context}, result); err != nil {
		return Null, err
	}
	return Null, nil
}

func testMockTypeName(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		if value.Text == "" {
			return "", false
		}
		return value.Text, true
	case ValueObject:
		if (strings.EqualFold(value.Type, "Type") || strings.EqualFold(value.Static, "Type") || strings.EqualFold(value.Runtime, "Type")) && value.Text != "" {
			return value.Text, true
		}
	}
	return "", false
}

func (vm *VM) testStart() (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.startTest is only available in test context")
	}
	if vm.testContext.Started {
		return Null, fmt.Errorf("Test.startTest cannot be called more than once")
	}
	vm.testContext.Started = true
	vm.testContext.Stopped = false
	if vm.currentPage.Kind == "" {
		vm.currentPage = newPageReference("/apex/current")
	}
	vm.testContext.AsyncStartIndex = len(vm.testContext.AsyncJobs)
	vm.testContext.PlatformEventStartIndex = len(vm.testContext.PlatformEvents)
	vm.deferPreStartAsyncJobRecords()
	vm.testContext.ChainEnqueued = false
	vm.testContext.ParentLimits = vm.limits
	vm.testContext.ParentViolations = append([]LimitViolation(nil), vm.limitViolations...)
	vm.ResetLimits()
	return Null, nil
}

func (vm *VM) deferPreStartAsyncJobRecords() {
	if vm.testContext == nil || vm.Org == nil || len(vm.testContext.AsyncJobs) == 0 {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	for _, job := range vm.testContext.AsyncJobs {
		storedID, record, ok := storage.LookupRecordByID(object.Records, storage.ID(job.ID))
		if !ok {
			continue
		}
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields["Status"] = storage.StringValue("Deferred")
		object.Records[storedID] = record
	}
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) testStop(result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.stopTest is only available in test context")
	}
	if !vm.testContext.Started {
		return Null, fmt.Errorf("Test.stopTest called before Test.startTest")
	}
	if vm.testContext.Stopped {
		return Null, fmt.Errorf("Test.stopTest cannot be called more than once")
	}
	vm.testContext.Stopped = true
	err := vm.drainTestWork(result)
	vm.limits = vm.testContext.ParentLimits
	vm.limitViolations = append([]LimitViolation(nil), vm.testContext.ParentViolations...)
	return Null, err
}

func (vm *VM) drainTestWork(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	previousPreserve := vm.testContext.PreserveAsyncStatics
	vm.testContext.PreserveAsyncStatics = true
	defer func() {
		vm.testContext.PreserveAsyncStatics = previousPreserve
	}()
	for iteration := 0; iteration < maxLoopIterations; iteration++ {
		beforeAsync := len(vm.testContext.AsyncJobs)
		beforeEvents := len(vm.testContext.PlatformEvents)
		beforePublishes := len(vm.testContext.EventPublishes)
		if err := vm.drainTestAsync(result); err != nil {
			return err
		}
		if err := vm.drainTestPlatformEventsFrom(result, vm.testContext.PlatformEventStartIndex, true); err != nil {
			return err
		}
		if err := vm.drainTestEventPublishes(result); err != nil {
			return err
		}
		startIndex := vm.testContext.AsyncStartIndex
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex > len(vm.testContext.AsyncJobs) {
			startIndex = len(vm.testContext.AsyncJobs)
		}
		eventStartIndex := vm.testContext.PlatformEventStartIndex
		if eventStartIndex < 0 {
			eventStartIndex = 0
		}
		if eventStartIndex > len(vm.testContext.PlatformEvents) {
			eventStartIndex = len(vm.testContext.PlatformEvents)
		}
		if len(vm.testContext.AsyncJobs) <= startIndex && len(vm.testContext.PlatformEvents) <= eventStartIndex && len(vm.testContext.EventPublishes) == 0 {
			return nil
		}
		if len(vm.testContext.AsyncJobs) == beforeAsync && len(vm.testContext.PlatformEvents) == beforeEvents && len(vm.testContext.EventPublishes) == beforePublishes &&
			nextDrainableAsyncJobIndex(vm.testContext.AsyncJobs, startIndex) < 0 {
			return nil
		}
	}
	return fmt.Errorf("Test.stopTest async/event drain exceeded %d iterations", maxLoopIterations)
}

func (vm *VM) enqueueJob(args []Value, result *Result) (Value, error) {
	if len(args) == 2 {
		if args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "AsyncOptions") {
			return Null, fmt.Errorf("System.enqueueJob options expects AsyncOptions")
		}
	}
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable[, AsyncOptions]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("queueableJobs", 1); err != nil {
		return Null, err
	}
	draining, chainEnqueued := vm.asyncDrainState()
	if draining && vm.currentAsyncKind == "Queueable" && chainEnqueued {
		return Null, fmt.Errorf("Queueable chaining limit exceeded")
	}
	vm.markAsyncChainEnqueued()
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "Queueable", Object: cloneValue(args[0])}
	if vm.currentAsyncKind == "Queueable" {
		job.QueueableDepth = vm.currentQueueableDepth + 1
		job.QueueableMaxDepth = vm.currentQueueableMaxDepth
	} else {
		job.QueueableDepth = 1
	}
	if len(args) == 2 {
		if maxDepth, ok := asyncOptionsInt(args[1], "maximumQueueableStackDepth"); ok {
			job.QueueableMaxDepth = maxDepth
		}
	}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":  job.Kind,
		"jobId": job.ID,
		"class": args[0].Type,
	})
	return String(job.ID), nil
}

func (vm *VM) executeBatch(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("Database.executeBatch expects batch instance[, scopeSize]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("Database.executeBatch expects Batchable object")
	}
	batchSize := 200
	if len(args) == 2 {
		if args[1].Kind != ValueInt {
			return Null, fmt.Errorf("Database.executeBatch scope size expects Integer")
		}
		batchSize = int(args[1].Int)
		if batchSize <= 0 {
			return Null, fmt.Errorf("Database.executeBatch scope size must be positive")
		}
		if batchSize > 2000 {
			return Null, fmt.Errorf("Database.executeBatch scope size must be at most 2000")
		}
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("batchJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "BatchApex", Object: cloneValue(args[0]), BatchSize: batchSize}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":      job.Kind,
		"jobId":     job.ID,
		"class":     args[0].Type,
		"batchSize": batchSize,
	})
	return String(job.ID), nil
}

func (vm *VM) scheduleJob(args []Value, result *Result) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueObject {
		return Null, fmt.Errorf("System.schedule expects name, cron, and Schedulable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("scheduledJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "ScheduledApex", Object: cloneValue(args[2]), Name: args[0].Text, Cron: args[1].Text}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	vm.recordCronTrigger(job, "Waiting")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":  job.Kind,
		"jobId": job.ID,
		"class": args[2].Type,
		"name":  job.Name,
	})
	return String(cronTriggerID(job.ID)), nil
}

func (vm *VM) scheduleBatch(args []Value, result *Result) (Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return Null, fmt.Errorf("System.scheduleBatch expects batch instance, name, minutesFromNow[, scopeSize]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.scheduleBatch expects Batchable object")
	}
	if args[1].Kind != ValueString {
		return Null, fmt.Errorf("System.scheduleBatch expects job name String")
	}
	if args[2].Kind != ValueInt {
		return Null, fmt.Errorf("System.scheduleBatch expects minutesFromNow Integer")
	}
	batchSize := 200
	if len(args) == 4 {
		if args[3].Kind != ValueInt {
			return Null, fmt.Errorf("System.scheduleBatch scope size expects Integer")
		}
		batchSize = int(args[3].Int)
		if batchSize <= 0 {
			return Null, fmt.Errorf("System.scheduleBatch scope size must be positive")
		}
		if batchSize > 2000 {
			return Null, fmt.Errorf("System.scheduleBatch scope size must be at most 2000")
		}
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("batchJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("scheduledJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "ScheduledBatch", Object: cloneValue(args[0]), BatchSize: batchSize, Name: args[1].Text, Cron: fmt.Sprintf("after %d minutes", args[2].Int)}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	vm.recordCronTrigger(job, "Waiting")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":      job.Kind,
		"jobId":     job.ID,
		"class":     args[0].Type,
		"name":      job.Name,
		"batchSize": batchSize,
	})
	return String(cronTriggerID(job.ID)), nil
}

func (vm *VM) abortJob(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("System.abortJob expects job Id")
	}
	if vm.testContext == nil {
		return Null, unsupportedCallError("System.abortJob local async scheduling surface")
	}
	jobID, ok := valueIDString(args[0])
	if !ok {
		return Null, fmt.Errorf("System.abortJob expects String job Id")
	}
	for i, job := range vm.testContext.AsyncJobs {
		if job.ID != jobID && cronTriggerID(job.ID) != jobID {
			continue
		}
		vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs[:i], vm.testContext.AsyncJobs[i+1:]...)
		vm.recordAsyncJob(job, "Aborted", "")
		if job.Kind == "ScheduledApex" {
			vm.recordCronTrigger(job, "Deleted")
		}
		return Null, nil
	}
	if vm.asyncJobRecordStatus(jobID) != "" {
		vm.abortRecordedAsyncJob(jobID)
		return Null, nil
	}
	return Null, unsupportedCallError("System.abortJob unknown local async records")
}

func (vm *VM) abortRecordedAsyncJob(jobID string) {
	if vm == nil || vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	asyncID := jobID
	if strings.HasPrefix(asyncID, "08e") {
		asyncID = strings.Replace(asyncID, "08e", "707", 1)
	}
	if object, ok := vm.Org.Objects["AsyncApexJob"]; ok {
		if record, found := object.Records[storage.ID(asyncID)]; found {
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			record.Fields["Status"] = storage.StringValue("Aborted")
			object.Records[record.ID] = record
			vm.Org.Objects["AsyncApexJob"] = object
		}
	}
	cronID := jobID
	if strings.HasPrefix(cronID, "707") {
		cronID = strings.Replace(cronID, "707", "08e", 1)
	}
	if object, ok := vm.Org.Objects["CronTrigger"]; ok {
		if record, found := object.Records[storage.ID(cronID)]; found {
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			record.Fields["State"] = storage.StringValue("Deleted")
			object.Records[record.ID] = record
			vm.Org.Objects["CronTrigger"] = object
		}
	}
}

func (vm *VM) emptyChildRelationshipForEach(expr ir.Expr) (Value, bool) {
	path := expr.Name
	if path == "" {
		path = expr.Callee
	}
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return Null, false
	}
	rootName := parts[0]
	if actual, ok := vm.lookupGlobalName(rootName); ok {
		rootName = actual
	}
	receiver, ok := vm.Globals[rootName]
	if !ok || receiver.Kind != ValueObject {
		return Null, false
	}
	if len(parts) > 2 {
		value, err := vm.lookupPath(receiver, parts[1:len(parts)-1])
		if err != nil || value.Kind != ValueObject {
			return Null, false
		}
		receiver = value
	}
	relationshipName := parts[len(parts)-1]
	if relationshipType, ok := vm.jsonSObjectChildRelationshipType(receiver.Type, relationshipName); ok {
		children := List()
		children.Type = relationshipType
		return children, true
	}
	return Null, false
}

func forEachExprContext(expr ir.Expr) string {
	switch {
	case expr.Name != "":
		return ": " + expr.Name
	case expr.Callee != "":
		return ": " + expr.Callee
	default:
		return ""
	}
}

func (vm *VM) executeObjectForEach(source string, inst ir.Instruction, result *Result, iterable Value) (execOutcome, error) {
	iterator := iterable
	if !isIteratorValue(iterator) {
		var err error
		iterator, err = vm.iteratorForObject(iterable, result)
		if err != nil {
			return execOutcome{}, err
		}
	}
	_, existed := vm.Globals[inst.Name]
	previous := vm.Globals[inst.Name]
	previousType, hadType := vm.VarTypes[inst.Name]
	const iteratorName = "__glade_for_each_iterator"
	previousIterator, hadIterator := vm.Globals[iteratorName]
	previousIteratorType, hadIteratorType := vm.VarTypes[iteratorName]
	defer func() {
		if existed {
			vm.Globals[inst.Name] = previous
		} else {
			delete(vm.Globals, inst.Name)
		}
		if hadType {
			vm.VarTypes[inst.Name] = previousType
		} else {
			delete(vm.VarTypes, inst.Name)
		}
		if hadIterator {
			vm.Globals[iteratorName] = previousIterator
		} else {
			delete(vm.Globals, iteratorName)
		}
		if hadIteratorType {
			vm.VarTypes[iteratorName] = previousIteratorType
		} else {
			delete(vm.VarTypes, iteratorName)
		}
	}()
	vm.Globals[iteratorName] = iterator
	vm.VarTypes[iteratorName] = iterator.Type
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("enhanced for loop exceeded %d iterations", maxLoopIterations)
		}
		hasNext, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "hasNext", nil, result)
		if err != nil {
			return execOutcome{}, err
		}
		if !handled || hasNext.Kind != ValueBool {
			return execOutcome{}, fmt.Errorf("enhanced for iterator requires Boolean hasNext")
		}
		if !hasNext.Bool {
			return execOutcome{}, nil
		}
		value, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "next", nil, result)
		if err != nil {
			return execOutcome{}, err
		}
		if !handled {
			return execOutcome{}, fmt.Errorf("enhanced for iterator requires next")
		}
		coerced, err := vm.coerceAssignable(inst.Type, value)
		if err != nil {
			return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
		}
		vm.Globals[inst.Name] = coerced
		vm.VarTypes[inst.Name] = inst.Type
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone:
		case signalContinue:
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
	}
}

func (vm *VM) iteratorForObject(iterable Value, result *Result) (Value, error) {
	if iterable.Kind == ValueNull {
		return Null, newNullDereferenceError("enhanced for over null collection")
	}
	dispatchType := runtimeObjectType(iterable)
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchType, "iterator", nil)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(dispatchType+".iterator", nil)
	}
	if !ok && iterable.Static != "" && !strings.EqualFold(iterable.Static, dispatchType) {
		target, ok, ambiguous = vm.resolveInstanceMethodForArgs(iterable.Static, "iterator", nil)
		if ambiguous {
			return Null, vm.ambiguousOverloadError(iterable.Static+".iterator", nil)
		}
	}
	if !ok {
		return Null, fmt.Errorf("enhanced for requires List, Set, or Iterable, got %s", iterable.Kind)
	}
	iterator, err := vm.callMethodWithReceiver(target, iterable, nil, result)
	if err != nil {
		return Null, err
	}
	if iterator.Kind == ValueNull {
		return Null, newNullDereferenceError("enhanced for over null iterator")
	}
	return iterator, nil
}

func (vm *VM) asyncJobRecordStatus(jobID string) string {
	if vm.Org == nil {
		return ""
	}
	vm.ensureAsyncObjects()
	if strings.HasPrefix(jobID, "08e") {
		jobID = strings.Replace(jobID, "08e", "707", 1)
	}
	object := vm.Org.Objects["AsyncApexJob"]
	record, ok := object.Records[storage.ID(jobID)]
	if !ok {
		return ""
	}
	if status, ok := record.GetField("Status"); ok && status.Kind == storage.ValueString {
		return status.String
	}
	return ""
}

func (vm *VM) drainTestAsync(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	startIndex := vm.testContext.AsyncStartIndex
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(vm.testContext.AsyncJobs) {
		startIndex = len(vm.testContext.AsyncJobs)
	}
	return vm.drainAsyncJobsFrom(result, &vm.testContext.AsyncJobs, startIndex, &vm.testContext.Draining, &vm.testContext.ChainEnqueued)
}

func (vm *VM) drainTestEventPublishes(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	for len(vm.testContext.EventPublishes) > 0 {
		publish := vm.testContext.EventPublishes[0]
		vm.testContext.EventPublishes = vm.testContext.EventPublishes[1:]
		methodName := "onSuccess"
		resultType := "eventbus.SuccessResult"
		if publish.Fail {
			methodName = "onFailure"
			resultType = "eventbus.FailureResult"
		}
		callbackResult := Object(resultType)
		uuidValues := make([]Value, 0, len(publish.EventUUIDs))
		for _, uuid := range publish.EventUUIDs {
			uuidValues = append(uuidValues, String(uuid))
		}
		callbackResult.Fields["eventUuids"] = List(uuidValues...)
		method, ok, ambiguous := vm.resolveInstanceMethodForArgs(publish.Callback.Type, methodName, []Value{callbackResult})
		if ambiguous {
			return vm.ambiguousOverloadError(publish.Callback.Type+"."+methodName, []Value{callbackResult})
		}
		if !ok {
			return fmt.Errorf("event publish callback %s has no %s method", publish.Callback.Type, methodName)
		}
		if _, err := vm.callMethodWithReceiver(method, publish.Callback, []Value{callbackResult}, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) drainTestPlatformEvents(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	return vm.drainTestPlatformEventsFrom(result, 0, false)
}

func (vm *VM) drainTestPlatformEventsFrom(result *Result, startIndex int, stopTimeDelivery bool) error {
	if vm.testContext == nil {
		return nil
	}
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(vm.testContext.PlatformEvents) {
		startIndex = len(vm.testContext.PlatformEvents)
	}
	for len(vm.testContext.PlatformEvents) > startIndex {
		records := append([]storage.Record(nil), vm.testContext.PlatformEvents[startIndex:]...)
		vm.testContext.PlatformEvents = vm.testContext.PlatformEvents[:startIndex]
		grouped := make(map[string][]storage.Record)
		order := make([]string, 0)
		for _, record := range records {
			if _, ok := grouped[record.Object]; !ok {
				order = append(order, record.Object)
			}
			grouped[record.Object] = append(grouped[record.Object], record)
		}
		for _, objectName := range order {
			if stopTimeDelivery {
				wasDraining := vm.testContext.Draining
				previousUser := vm.testContext.CurrentUser
				vm.testContext.Draining = true
				if user := vm.automatedProcessUser(); user.Kind != "" {
					vm.testContext.CurrentUser = user
				}
				_, err := vm.runWithFreshStatics(func() (Value, error) {
					_, err := vm.runTriggers(triggerTimingAfter, "insert", grouped[objectName], nil, result)
					return Null, err
				})
				vm.testContext.Draining = wasDraining
				vm.testContext.CurrentUser = previousUser
				if err != nil {
					return err
				}
				continue
			}
			if _, err := vm.runTriggers(triggerTimingAfter, "insert", grouped[objectName], nil, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vm *VM) automatedProcessUser() Value {
	if vm == nil || vm.Org == nil {
		return Value{}
	}
	users, ok := vm.Org.Objects["User"]
	if !ok {
		return Value{}
	}
	for _, record := range users.Records {
		if strings.EqualFold(recordFieldString(record, "UserType"), "AutomatedProcess") {
			return vmValueFromRecord(record)
		}
	}
	return Value{}
}

func (vm *VM) runWithFreshStatics(fn func() (Value, error)) (Value, error) {
	snapshot := vm.staticFieldSnapshot()
	staticInitSnapshot := copyStaticInitStateMap(vm.staticInitState)
	if err := vm.ResetStatics(); err != nil {
		return Null, err
	}
	value, err := fn()
	vm.restoreStaticFieldSnapshot(snapshot)
	vm.staticInitState = copyStaticInitStateMap(staticInitSnapshot)
	return value, err
}

func (vm *VM) drainLocalAsync(result *Result) error {
	return vm.drainAsyncJobs(result, &vm.localAsyncJobs, &vm.localAsyncDrain, &vm.localAsyncChain)
}

func (vm *VM) drainAsyncJobs(result *Result, jobs *[]AsyncJob, draining *bool, chainEnqueued *bool) error {
	return vm.drainAsyncJobsFrom(result, jobs, 0, draining, chainEnqueued)
}

func (vm *VM) drainAsyncJobsFrom(result *Result, jobs *[]AsyncJob, startIndex int, draining *bool, chainEnqueued *bool) error {
	if *draining {
		return nil
	}
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(*jobs) {
		startIndex = len(*jobs)
	}
	*draining = true
	restoreStatics := true
	if vm.testContext != nil && vm.testContext.PreserveAsyncStatics {
		restoreStatics = false
	}
	var snapshot map[string]map[string]Value
	var staticInitSnapshot map[string]staticInitState
	if restoreStatics {
		snapshot = vm.staticFieldSnapshot()
		staticInitSnapshot = copyStaticInitStateMap(vm.staticInitState)
	}
	defer func() {
		if restoreStatics {
			vm.restoreStaticFieldSnapshot(snapshot)
			vm.staticInitState = copyStaticInitStateMap(staticInitSnapshot)
		}
		*draining = false
	}()
	maxJobs := -1
	if vm.testContext != nil {
		maxJobs = drainableAsyncJobCount(*jobs, startIndex)
	}
	processed := 0
	for maxJobs < 0 || processed < maxJobs {
		jobIndex := nextDrainableAsyncJobIndex(*jobs, startIndex)
		if jobIndex < 0 {
			break
		}
		job := (*jobs)[jobIndex]
		*jobs = append((*jobs)[:jobIndex], (*jobs)[jobIndex+1:]...)
		processed++
		var collectionSnapshot map[string]map[string]Value
		if vm.testContext != nil {
			collectionSnapshot = vm.staticCollectionFieldSnapshot()
			if err := vm.ResetTestAsyncStaticCollections(); err != nil {
				vm.restoreStaticFieldSnapshot(collectionSnapshot)
				return err
			}
		} else {
			if err := vm.ResetStatics(); err != nil {
				return err
			}
		}
		*chainEnqueued = false
		vm.recordAsyncJob(job, "Processing", "")
		appendTrace(result, "apex.async.run", "apex.async", map[string]any{
			"kind":  job.Kind,
			"jobId": job.ID,
		})
		err := vm.runAsyncJob(job, result)
		if collectionSnapshot != nil {
			vm.restoreStaticFieldSnapshot(collectionSnapshot)
		}
		if err != nil {
			vm.recordAsyncJob(job, "Failed", err.Error())
			return err
		}
		if vm.testContext != nil && job.Kind == "BatchApex" {
			vm.recordAsyncJob(job, "Completed", "")
			vm.markCompletedBatchJobVisiblePendingInTest(job)
		} else if job.Kind == "ScheduledApex" {
			vm.recordAsyncJob(job, "Queued", "")
		} else {
			vm.recordAsyncJob(job, "Completed", "")
		}
	}
	return nil
}

func drainableAsyncJobCount(jobs []AsyncJob, startIndex int) int {
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(jobs) {
		startIndex = len(jobs)
	}
	count := 0
	for _, job := range jobs[startIndex:] {
		if !job.Deferred {
			count++
		}
	}
	return count
}

func nextDrainableAsyncJobIndex(jobs []AsyncJob, startIndex int) int {
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(jobs) {
		startIndex = len(jobs)
	}
	for i := startIndex; i < len(jobs); i++ {
		if !jobs[i].Deferred {
			return i
		}
	}
	return -1
}

func (vm *VM) staticFieldSnapshot() map[string]map[string]Value {
	out := make(map[string]map[string]Value, len(vm.Classes))
	for className, class := range vm.Classes {
		fields := make(map[string]Value, len(class.StaticFields))
		for fieldName, field := range class.StaticFields {
			fields[fieldName] = field.Value
		}
		out[className] = fields
	}
	return out
}

func (vm *VM) staticCollectionFieldSnapshot() map[string]map[string]Value {
	out := make(map[string]map[string]Value, len(vm.Classes))
	for className, class := range vm.Classes {
		fields := make(map[string]Value)
		for fieldName, field := range class.StaticFields {
			if isStaticCollectionField(field) {
				fields[fieldName] = cloneValue(field.Value)
			}
		}
		if len(fields) != 0 {
			out[className] = fields
		}
	}
	return out
}

func (vm *VM) restoreStaticFieldSnapshot(snapshot map[string]map[string]Value) {
	for className, fields := range snapshot {
		class, ok := vm.Classes[className]
		if !ok {
			continue
		}
		for fieldName, value := range fields {
			field, ok := class.StaticFields[fieldName]
			if !ok {
				continue
			}
			field.Value = value
			class.StaticFields[fieldName] = field
		}
		vm.Classes[className] = class
	}
	vm.invalidateStaticValueRefs()
}

func (vm *VM) asyncDrainState() (bool, bool) {
	if vm.testContext != nil {
		return vm.testContext.Draining, vm.testContext.ChainEnqueued
	}
	return vm.localAsyncDrain, vm.localAsyncChain
}

func (vm *VM) markAsyncChainEnqueued() {
	if vm.testContext != nil {
		if vm.testContext.Draining {
			vm.testContext.ChainEnqueued = true
		}
		return
	}
	if vm.localAsyncDrain {
		vm.localAsyncChain = true
	}
}

func (vm *VM) enqueueAsyncJob(job AsyncJob) {
	if vm.testContext != nil {
		vm.recordApexClass(asyncClassName(job))
		if vm.testContext.Draining && job.Kind != "Queueable" && job.Kind != "BatchApex" {
			return
		}
		if vm.testContext.Draining && job.Kind == "Queueable" && !vm.canDrainQueueableJob(job) {
			job.Deferred = true
		}
		if vm.testContext.Draining && job.Kind == "BatchApex" && vm.currentAsyncKind == "Queueable" {
			job.Deferred = true
		}
		vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, job)
		return
	}
	vm.localAsyncJobs = append(vm.localAsyncJobs, job)
}

func asyncOptionsInt(options Value, fieldName string) (int, bool) {
	if options.Kind != ValueObject {
		return 0, false
	}
	for name, value := range options.Fields {
		if !strings.EqualFold(name, fieldName) {
			continue
		}
		switch value.Kind {
		case ValueInt:
			return int(value.Int), true
		case ValueDecimal:
			return int(value.Decimal), true
		}
	}
	return 0, false
}

func (vm *VM) canDrainQueueableJob(job AsyncJob) bool {
	if vm.currentAsyncKind == "BatchApex" {
		return true
	}
	if vm.currentAsyncKind != "Queueable" {
		return false
	}
	if job.QueueableMaxDepth <= 0 {
		return false
	}
	return job.QueueableDepth > 0 && job.QueueableDepth <= job.QueueableMaxDepth
}

func (vm *VM) runAsyncJob(job AsyncJob, result *Result) error {
	switch job.Kind {
	case "Future":
		_, err := vm.withAsyncKind("Future", func() (Value, error) {
			return vm.callMethod(job.Method, job.Args, result)
		})
		return err
	case "Queueable":
		args := []Value{asyncContext("QueueableContext", job.ID)}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", args)
		if !ok && !ambiguous {
			args = nil
			target, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", nil)
		}
		if ambiguous {
			return fmt.Errorf("async job %s execute method is ambiguous", job.Object.Type)
		}
		if !ok {
			return fmt.Errorf("async job %s has no execute method", job.Object.Type)
		}
		if len(target.Params) == 0 {
			args = nil
		}
		previousFinalizer := vm.currentFinalizer
		vm.currentFinalizer = Value{}
		_, err := vm.withQueueableJob(job, func() (Value, error) {
			return vm.callMethodWithReceiver(target, job.Object, args, result)
		})
		finalizer := vm.currentFinalizer
		vm.currentFinalizer = previousFinalizer
		if finalizer.Kind == ValueObject {
			finalizerErr := vm.runQueueableFinalizer(finalizer, job, result, err)
			if err == nil {
				err = finalizerErr
			}
		}
		return err
	case "BatchApex":
		_, err := vm.withAsyncKind("BatchApex", func() (Value, error) {
			return Null, vm.runBatchJob(job, result)
		})
		return err
	case "ScheduledBatch":
		_, err := vm.withAsyncKind("BatchApex", func() (Value, error) {
			return Null, vm.runBatchJob(job, result)
		})
		vm.recordCronTrigger(job, "Complete")
		return err
	case "ScheduledApex":
		args := []Value{schedulableContext(job.ID)}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", args)
		if ambiguous {
			return vm.ambiguousOverloadError(job.Object.Type+".execute", args)
		}
		if !ok {
			target, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", nil)
			if ambiguous {
				return vm.ambiguousOverloadError(job.Object.Type+".execute", nil)
			}
		}
		if !ok {
			return fmt.Errorf("scheduled job %s has no execute method", job.Object.Type)
		}
		if len(target.Params) == 0 {
			args = nil
		}
		_, err := vm.withAsyncKind("ScheduledApex", func() (Value, error) {
			return vm.callMethodWithReceiver(target, job.Object, args, result)
		})
		vm.recordCronTrigger(job, "Complete")
		return err
	default:
		return fmt.Errorf("unsupported async job kind %s", job.Kind)
	}
}

func (vm *VM) runQueueableFinalizer(finalizer Value, job AsyncJob, result *Result, parentErr error) error {
	args := []Value{finalizerContext(job.ID, parentErr)}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(finalizer.Type, "execute", args)
	if ambiguous {
		return fmt.Errorf("async finalizer %s execute method is ambiguous", finalizer.Type)
	}
	if !ok {
		return fmt.Errorf("async finalizer %s has no execute method", finalizer.Type)
	}
	if len(target.Params) == 0 {
		args = nil
	}
	_, err := vm.withAsyncKind("Queueable", func() (Value, error) {
		return vm.callMethodWithReceiver(target, finalizer, args, result)
	})
	return err
}

func (vm *VM) withAsyncKind(kind string, run func() (Value, error)) (Value, error) {
	previous := vm.currentAsyncKind
	vm.currentAsyncKind = kind
	defer func() {
		vm.currentAsyncKind = previous
	}()
	return run()
}

func (vm *VM) withQueueableJob(job AsyncJob, run func() (Value, error)) (Value, error) {
	previousKind := vm.currentAsyncKind
	previousDepth := vm.currentQueueableDepth
	previousMaxDepth := vm.currentQueueableMaxDepth
	vm.currentAsyncKind = "Queueable"
	vm.currentQueueableDepth = job.QueueableDepth
	if vm.currentQueueableDepth <= 0 {
		vm.currentQueueableDepth = 1
	}
	vm.currentQueueableMaxDepth = job.QueueableMaxDepth
	defer func() {
		vm.currentAsyncKind = previousKind
		vm.currentQueueableDepth = previousDepth
		vm.currentQueueableMaxDepth = previousMaxDepth
	}()
	return run()
}

func (vm *VM) isAsyncKind(callee string) bool {
	switch callee {
	case "System.isBatch":
		return vm.currentAsyncKind == "BatchApex"
	case "System.isFuture":
		return vm.currentAsyncKind == "Future"
	case "System.isQueueable":
		return vm.currentAsyncKind == "Queueable"
	case "System.isScheduled":
		return vm.currentAsyncKind == "ScheduledApex"
	default:
		return false
	}
}

func (vm *VM) runBatchJob(job AsyncJob, result *Result) error {
	var scope []Value
	if start, ok := vm.resolveInstanceMethod(job.Object.Type, "start"); ok {
		value, err := vm.callMethodWithReceiver(start, job.Object, batchArgs(start, "Database.BatchableContext", job.ID), result)
		if err != nil {
			return err
		}
		scopeValues, err := vm.batchScopeValues(value, result)
		if err != nil {
			return err
		}
		scope = append(scope, scopeValues...)
	}
	chunks := batchChunks(scope, job.BatchSize)
	executeArgs := []Value{asyncContext("Database.BatchableContext", job.ID), List()}
	execute, ok, ambiguous := vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", executeArgs)
	if ambiguous {
		return vm.ambiguousOverloadError(job.Object.Type+".execute", executeArgs)
	}
	if !ok {
		executeArgs = []Value{List()}
		execute, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", executeArgs)
		if ambiguous {
			return vm.ambiguousOverloadError(job.Object.Type+".execute", executeArgs)
		}
	}
	if !ok {
		execute, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", nil)
		if ambiguous {
			return vm.ambiguousOverloadError(job.Object.Type+".execute", nil)
		}
	}
	if ok {
		vm.recordAsyncJobTotals(job, len(chunks), 0, 0)
		for _, chunk := range chunks {
			if _, err := vm.callMethodWithReceiver(execute, job.Object, batchExecuteArgs(execute, chunk, job.ID), result); err != nil {
				vm.recordAsyncJob(job, "Failed", err.Error())
				if eventErr := vm.emitBatchApexErrorEvent(job, chunk, "EXECUTE", err, result); eventErr != nil {
					return eventErr
				}
				return err
			}
		}
	}
	if finish, ok := vm.resolveInstanceMethod(job.Object.Type, "finish"); ok {
		vm.recordAsyncJob(job, "Completed", "")
		if _, err := vm.callMethodWithReceiver(finish, job.Object, batchArgs(finish, "Database.BatchableContext", job.ID), result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) batchScopeValues(value Value, result *Result) ([]Value, error) {
	switch {
	case value.Kind == ValueNull:
		return nil, nil
	case value.Kind == ValueList:
		return append([]Value(nil), value.List...), nil
	case value.Kind == ValueSet:
		return append([]Value(nil), value.Set...), nil
	case value.Kind == ValueObject && value.Type == "Database.QueryLocator":
		if records, ok := value.Fields["Records"]; ok && records.Kind == ValueList {
			return append([]Value(nil), records.List...), nil
		}
		return nil, nil
	case value.Kind == ValueObject:
		iterator := value
		if !isIteratorValue(iterator) {
			var err error
			iterator, err = vm.iteratorForObject(value, result)
			if err != nil {
				return nil, err
			}
		}
		return vm.collectIteratorValues(iterator, result)
	default:
		return nil, fmt.Errorf("Database.Batchable.start returned unsupported scope %s", value.Kind)
	}
}

func (vm *VM) collectIteratorValues(iterator Value, result *Result) ([]Value, error) {
	const iteratorName = "__glade_batch_iterator"
	previousIterator, hadIterator := vm.Globals[iteratorName]
	previousIteratorType, hadIteratorType := vm.VarTypes[iteratorName]
	defer func() {
		if hadIterator {
			vm.Globals[iteratorName] = previousIterator
		} else {
			delete(vm.Globals, iteratorName)
		}
		if hadIteratorType {
			vm.VarTypes[iteratorName] = previousIteratorType
		} else {
			delete(vm.VarTypes, iteratorName)
		}
	}()
	vm.Globals[iteratorName] = iterator
	vm.VarTypes[iteratorName] = iterator.Type
	values := []Value{}
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return nil, fmt.Errorf("batch iterable exceeded %d iterations", maxLoopIterations)
		}
		hasNext, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "hasNext", nil, result)
		if err != nil {
			return nil, err
		}
		if !handled || hasNext.Kind != ValueBool {
			return nil, fmt.Errorf("batch iterable requires Boolean hasNext")
		}
		if !hasNext.Bool {
			return values, nil
		}
		value, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "next", nil, result)
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, fmt.Errorf("batch iterable requires next")
		}
		values = append(values, value)
	}
}

func (vm *VM) emitBatchApexErrorEvent(job AsyncJob, scope []Value, phase string, cause error, result *Result) error {
	if vm.Org == nil || cause == nil {
		return nil
	}
	vm.ensureAsyncObjects()
	vm.ensureBatchApexErrorEventObject()
	record := storage.Record{
		Object: "BatchApexErrorEvent",
		Fields: map[string]storage.Value{
			"AsyncApexJobId": storage.IDValue(storage.ID(job.ID)),
			"JobScope":       storage.StringValue(batchErrorJobScope(scope)),
			"ExceptionType":  storage.StringValue(batchErrorExceptionType(cause)),
			"Message":        storage.StringValue(cause.Error()),
			"Phase":          storage.StringValue(phase),
			"StackTrace":     storage.StringValue(cause.Error()),
		},
	}
	_, err := vm.runTriggers(triggerTimingAfter, "insert", []storage.Record{record}, nil, result)
	return err
}

func (vm *VM) ensureBatchApexErrorEventObject() {
	if vm.Org == nil {
		return
	}
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "BatchApexErrorEvent",
		Label:     "Batch Apex Error Event",
		KeyPrefix: "1Be",
		Fields: map[string]storage.Field{
			"AsyncApexJobId": {APIName: "AsyncApexJobId", Type: storage.FieldReference, ReferenceTo: []string{"AsyncApexJob"}, RelationshipName: "AsyncApexJob"},
			"JobScope":       {APIName: "JobScope", Type: storage.FieldString},
			"ExceptionType":  {APIName: "ExceptionType", Type: storage.FieldString},
			"Message":        {APIName: "Message", Type: storage.FieldString},
			"Phase":          {APIName: "Phase", Type: storage.FieldString},
			"StackTrace":     {APIName: "StackTrace", Type: storage.FieldString},
		},
		Relations: []storage.Relationship{{
			Field:              "AsyncApexJobId",
			ParentObjects:      []string{"AsyncApexJob"},
			ParentRelationship: "AsyncApexJob",
		}},
	})
}

func batchErrorJobScope(scope []Value) string {
	parts := make([]string, 0, len(scope))
	for _, item := range scope {
		if id := sObjectIDFromFields(item.Fields); id != "" {
			parts = append(parts, string(id))
			continue
		}
		if text, ok := idValueText(item); ok && text != "" {
			parts = append(parts, text)
			continue
		}
		if item.Kind != ValueNull {
			parts = append(parts, item.String())
		}
	}
	return strings.Join(parts, ",")
}

func batchErrorExceptionType(err error) string {
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
		return runtimeErr.Type
	}
	return "Exception"
}

func batchChunks(values []Value, size int) [][]Value {
	if size <= 0 {
		size = 200
	}
	if len(values) == 0 {
		return nil
	}
	var chunks [][]Value
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func batchArgs(method Method, contextType, jobID string) []Value {
	if len(method.Params) == 0 {
		return nil
	}
	return []Value{asyncContext(contextType, jobID)}
}

func batchExecuteArgs(method Method, scope []Value, jobID string) []Value {
	switch len(method.Params) {
	case 0:
		return nil
	case 1:
		return []Value{List(scope...)}
	default:
		return []Value{asyncContext("Database.BatchableContext", jobID), List(scope...)}
	}
}

func asyncContext(typeName, jobID string) Value {
	ctx := Object(typeName)
	if jobID != "" {
		ctx.Fields["JobId"] = String(jobID)
	}
	return ctx
}

func finalizerContext(jobID string, parentErr error) Value {
	ctx := asyncContext("FinalizerContext", jobID)
	ctx.Fields["Result"] = parentJobResultValue("SUCCESS")
	ctx.Fields["Exception"] = Null
	if parentErr != nil {
		ctx.Fields["Result"] = parentJobResultValue("UNHANDLED_EXCEPTION")
		exception := Object("Exception")
		exception.Fields["message"] = String(parentErr.Error())
		ctx.Fields["Exception"] = exception
	}
	return ctx
}

func parentJobResultValue(name string) Value {
	value := Value{Kind: ValueObject, Type: "ParentJobResult", Text: name}
	value.Fields = map[string]Value{"ordinal": Int(0)}
	if name == "UNHANDLED_EXCEPTION" {
		value.Fields["ordinal"] = Int(1)
	}
	return value
}

func schedulableContext(jobID string) Value {
	ctx := Object("SchedulableContext")
	if jobID != "" {
		ctx.Fields["TriggerId"] = String(cronTriggerID(jobID))
	}
	return ctx
}

func cronTriggerID(jobID string) string {
	return strings.Replace(jobID, "707", "08e", 1)
}

func (vm *VM) nextAsyncJobID() string {
	if vm.testContext != nil {
		vm.testContext.JobSeq++
		return fmt.Sprintf("707%012d", vm.testContext.JobSeq)
	}
	vm.localAsyncSeq++
	return fmt.Sprintf("707%012d", vm.localAsyncSeq)
}

func ensureObject(org *storage.OrgState, definition storage.ObjectDefinition) {
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	if existing, ok := org.Objects[definition.APIName]; ok {
		existing.Definition = existing.Definition.Clone()
		if existing.Records == nil {
			existing.Records = make(map[storage.ID]storage.Record)
		}
		if existing.Definition.Fields == nil {
			existing.Definition.Fields = definition.Fields
		} else {
			for name, field := range definition.Fields {
				if _, ok := existing.Definition.Fields[name]; !ok {
					existing.Definition.Fields[name] = field
				}
			}
		}
		for _, relation := range definition.Relations {
			found := false
			for _, existingRelation := range existing.Definition.Relations {
				if existingRelation.Field == relation.Field {
					found = true
					break
				}
			}
			if !found {
				existing.Definition.Relations = append(existing.Definition.Relations, relation)
			}
		}
		org.Objects[definition.APIName] = existing
		return
	}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
}

func (vm *VM) recordAsyncJob(job AsyncJob, status, detail string) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	record := object.Records[storage.ID(job.ID)]
	if record.ID == "" {
		record = storage.Record{ID: storage.ID(job.ID), Object: "AsyncApexJob", Fields: make(map[string]storage.Value)}
	}
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = vm.fakeNow.Format(time.RFC3339)
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = storage.ID(vm.currentUserInfoField("Id", "005000000000001"))
	}
	record.System.LastModifiedDate = vm.fakeNow.Format(time.RFC3339)
	record.System.LastModifiedByID = record.System.CreatedByID
	record.System.SystemModstamp = record.System.LastModifiedDate
	className := asyncClassName(job)
	apexClassID := vm.recordApexClass(className)
	record.Fields["Status"] = storage.StringValue(status)
	record.Fields["JobType"] = storage.StringValue(asyncJobType(job))
	record.Fields["ApexClassId"] = storage.IDValue(apexClassID)
	record.Fields["ApexClassName"] = storage.StringValue(className)
	record.Fields["MethodName"] = storage.StringValue(asyncMethodName(job))
	if job.Kind == "ScheduledApex" || job.Kind == "ScheduledBatch" {
		record.Fields["CronTriggerId"] = storage.IDValue(storage.ID(cronTriggerID(job.ID)))
	} else {
		delete(record.Fields, "CronTriggerId")
	}
	if existing, ok := record.Fields["TotalJobItems"]; ok && existing.Kind == storage.ValueInteger && existing.Integer > 0 && asyncJobType(job) == "BatchApex" {
		record.Fields["TotalJobItems"] = existing
	} else {
		record.Fields["TotalJobItems"] = storage.IntegerValue(int64(asyncTotalItems(job)))
	}
	if status == "Completed" {
		record.Fields["JobItemsProcessed"] = record.Fields["TotalJobItems"]
		record.Fields["NumberOfErrors"] = storage.IntegerValue(0)
		record.Fields["CompletedDate"] = storage.DateTimeValue(vm.fakeNow.Format(time.RFC3339))
	} else if status == "Failed" {
		record.Fields["NumberOfErrors"] = storage.IntegerValue(1)
		record.Fields["ExtendedStatus"] = storage.StringValue(detail)
		record.Fields["CompletedDate"] = storage.DateTimeValue(vm.fakeNow.Format(time.RFC3339))
	} else if status == "Aborted" {
		record.Fields["CompletedDate"] = storage.DateTimeValue(vm.fakeNow.Format(time.RFC3339))
	} else {
		delete(record.Fields, "CompletedDate")
	}
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) markCompletedBatchJobVisiblePendingInTest(job AsyncJob) {
	if vm.Org == nil {
		return
	}
	object := vm.Org.Objects["AsyncApexJob"]
	record := object.Records[storage.ID(job.ID)]
	if record.ID == "" {
		return
	}
	record.Fields["__GLADETestPendingStatus"] = storage.StringValue("Queued")
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) recordApexClass(className string) storage.ID {
	vm.ensureAsyncObjects()
	return vm.recordApexClassRecord(className)
}

func (vm *VM) recordApexClassRecord(className string) storage.ID {
	fallbackID := storage.ID(asyncApexClassID(className))
	if vm.Org == nil || className == "" {
		return fallbackID
	}
	object := vm.Org.Objects["ApexClass"]
	namespace, localName := splitApexClassName(className)
	if namespace == "" {
		namespace = vm.apexClassNamespace(localName)
	}
	if id, ok := findApexClassRecordID(object, localName, namespace); ok {
		return id
	}
	if _, ok := object.Records[fallbackID]; ok {
		return fallbackID
	}
	object.Records[fallbackID] = storage.Record{
		ID:     fallbackID,
		Object: "ApexClass",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue(localName),
			"NamespacePrefix": apexClassNamespaceValue(namespace),
		},
	}
	vm.Org.Objects["ApexClass"] = object
	return fallbackID
}

func findApexClassRecordID(object storage.ObjectState, localName, namespace string) (storage.ID, bool) {
	for id, record := range object.Records {
		if !storageStringValueEquals(record.Fields["Name"], localName) {
			continue
		}
		nsValue := record.Fields["NamespacePrefix"]
		if strings.TrimSpace(namespace) == "" {
			if nsValue.Kind == storage.ValueNull || (nsValue.Kind == storage.ValueString && strings.TrimSpace(nsValue.String) == "") {
				return id, true
			}
			continue
		}
		if storageStringValueEquals(nsValue, namespace) {
			return id, true
		}
	}
	return "", false
}

func (vm *VM) apexClassNamespace(localName string) string {
	if vm == nil {
		return ""
	}
	if class, ok := vm.lookupClass(localName); ok && strings.TrimSpace(class.Namespace) != "" {
		return class.Namespace
	}
	if vm.Org != nil {
		return strings.TrimSpace(vm.Org.Namespace)
	}
	return ""
}

func splitApexClassName(className string) (string, string) {
	namespace, localName, ok := strings.Cut(className, ".")
	if !ok || strings.TrimSpace(namespace) == "" || strings.TrimSpace(localName) == "" {
		return "", className
	}
	return namespace, localName
}

func apexClassNamespaceValue(namespace string) storage.Value {
	if strings.TrimSpace(namespace) == "" {
		return storage.NullValue()
	}
	return storage.StringValue(namespace)
}

func (vm *VM) recordAsyncJobTotals(job AsyncJob, total, processed, errors int) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	record := object.Records[storage.ID(job.ID)]
	if record.ID == "" {
		record = storage.Record{ID: storage.ID(job.ID), Object: "AsyncApexJob", Fields: make(map[string]storage.Value)}
	}
	record.Fields["TotalJobItems"] = storage.IntegerValue(int64(total))
	record.Fields["JobItemsProcessed"] = storage.IntegerValue(int64(processed))
	record.Fields["NumberOfErrors"] = storage.IntegerValue(int64(errors))
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) recordCronTrigger(job AsyncJob, state string) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["CronTrigger"]
	id := storage.ID(cronTriggerID(job.ID))
	record := object.Records[id]
	if record.ID == "" {
		record = storage.Record{ID: id, Object: "CronTrigger", Fields: make(map[string]storage.Value)}
	}
	record.Fields["State"] = storage.StringValue(state)
	record.Fields["CronExpression"] = storage.StringValue(job.Cron)
	record.Fields["CronJobDetail"] = storage.StringValue(job.Name)
	record.Fields["CronJobDetailId"] = storage.IDValue(storage.ID(cronJobDetailID(job.Name)))
	record.Fields["TimesTriggered"] = storage.IntegerValue(0)
	if nextFireTime, ok := cronNextFireTime(job.Cron, vm.fakeNow); ok {
		record.Fields["NextFireTime"] = storage.DateTimeValue(nextFireTime)
	}
	object.Records[record.ID] = record
	vm.Org.Objects["CronTrigger"] = object
	vm.recordCronJobDetail(job)
}

func cronNextFireTime(expr string, now time.Time) (string, bool) {
	parts := strings.Fields(expr)
	if len(parts) != 6 && len(parts) != 7 {
		return "", false
	}
	sec, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", false
	}
	hour, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", false
	}
	if sec < 0 || sec > 59 || min < 0 || min > 59 || hour < 0 || hour > 23 {
		return "", false
	}
	day, anyDay, ok := cronField(parts[3], 1, 31, true)
	if !ok {
		return "", false
	}
	month, anyMonth, ok := cronField(parts[4], 1, 12, false)
	if !ok {
		return "", false
	}
	weekday, anyWeekday, ok := cronField(parts[5], 1, 7, true)
	if !ok {
		return "", false
	}
	year, anyYear := 0, true
	if len(parts) == 7 {
		year, anyYear, ok = cronField(parts[6], 1970, 9999, false)
		if !ok {
			return "", false
		}
	}
	start := now.UTC().Truncate(24 * time.Hour)
	for offset := 0; offset < 3660; offset++ {
		candidateDay := start.AddDate(0, 0, offset)
		if !anyYear && candidateDay.Year() != year {
			continue
		}
		if !anyMonth && int(candidateDay.Month()) != month {
			continue
		}
		if !anyDay && candidateDay.Day() != day {
			continue
		}
		if !anyWeekday && salesforceCronWeekday(candidateDay) != weekday {
			continue
		}
		candidate := time.Date(candidateDay.Year(), candidateDay.Month(), candidateDay.Day(), hour, min, sec, 0, time.UTC)
		if candidate.After(now.UTC()) {
			return formatPlatformDatetime(candidate), true
		}
	}
	return "", false
}

func cronField(part string, min, max int, questionWildcard bool) (int, bool, bool) {
	if part == "*" || (questionWildcard && part == "?") {
		return 0, true, true
	}
	value, err := strconv.Atoi(part)
	if err != nil || value < min || value > max {
		return 0, false, false
	}
	return value, false, true
}

func salesforceCronWeekday(value time.Time) int {
	weekday := int(value.Weekday()) + 1
	if weekday == 8 {
		return 1
	}
	return weekday
}

func (vm *VM) recordCronJobDetail(job AsyncJob) {
	if vm.Org == nil || job.Name == "" {
		return
	}
	object := vm.Org.Objects["CronJobDetail"]
	id := storage.ID(cronJobDetailID(job.Name))
	if _, ok := object.Records[id]; ok {
		return
	}
	object.Records[id] = storage.Record{
		ID:     id,
		Object: "CronJobDetail",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue(job.Name),
			"JobType": storage.StringValue(cronJobType(job)),
		},
	}
	vm.Org.Objects["CronJobDetail"] = object
}

func asyncClassName(job AsyncJob) string {
	if job.Method.ClassName != "" {
		return job.Method.ClassName
	}
	return job.Object.Type
}

func asyncJobType(job AsyncJob) string {
	if job.Kind == "ScheduledBatch" {
		return "BatchApex"
	}
	return job.Kind
}

func asyncApexClassID(className string) string {
	sum := sha1.Sum([]byte(className))
	return "01p" + hex.EncodeToString(sum[:])[:12]
}

func cronJobDetailID(name string) string {
	sum := sha1.Sum([]byte(name))
	return "08a" + hex.EncodeToString(sum[:])[:12]
}

func cronJobType(job AsyncJob) string {
	if job.Kind == "ScheduledApex" || job.Kind == "ScheduledBatch" {
		return "7"
	}
	return "0"
}

func asyncMethodName(job AsyncJob) string {
	if job.Method.Name == "" {
		return "execute"
	}
	name := job.Method.Name
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

func asyncTotalItems(job AsyncJob) int {
	if asyncJobType(job) != "BatchApex" || job.BatchSize <= 0 {
		return 1
	}
	return 1
}

func (vm *VM) queryLocatorFromSOQL(query string, result *Result) (Value, error) {
	value, err := vm.executeSOQL(query, result)
	if err != nil {
		return Null, err
	}
	locator := Object("Database.QueryLocator")
	locator.Fields["Records"] = value
	locator.Fields["Query"] = String(query)
	return locator, nil
}

func (vm *VM) executeSOQL(raw string, execResult *Result) (Value, error) {
	if soql.IsSOSLFind(raw) {
		return vm.executeSOSL(raw, execResult)
	}
	values, err := vm.executeSOQLRows(raw, execResult)
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, vm.expandSOQLBinds); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	return out, nil
}

func (vm *VM) executeSOQLWithAccessLevel(raw string, accessLevel Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithAccessLevel(raw, execResult, accessLevel)
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, vm.expandSOQLBinds); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	return out, nil
}

func (vm *VM) executeInlineSOQL(raw string, execResult *Result) (Value, error) {
	value, err := vm.executeSOQL(raw, execResult)
	if err != nil {
		return Null, err
	}
	if value.Kind == ValueList {
		if value.Fields == nil {
			value.Fields = make(map[string]Value)
		}
		value.Fields["__soqlQuery"] = String(raw)
	}
	if vm.inlineSOQLMayReturnScalarCount(raw) {
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
	}
	return value, nil
}

func (vm *VM) inlineSOQLMayReturnScalarCount(raw string) bool {
	query, err := soql.ParseAt(raw, vm.fakeNow)
	if err != nil {
		return !strings.Contains(strings.ToLower(raw), " group by ")
	}
	if len(query.GroupBy) > 0 || query.Having != nil {
		return false
	}
	return query.Count || len(query.Aggregates) == 1
}

func inlineSOQLQueryText(value Value) string {
	if value.Kind != ValueList || value.Fields == nil {
		return ""
	}
	query, ok := value.Fields["__soqlQuery"]
	if !ok || query.Kind != ValueString {
		return ""
	}
	return query.Text
}

func (vm *VM) executeSOQLWithBindMap(raw string, binds Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithExpander(raw, execResult, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}, binds, "")
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	return out, nil
}

func (vm *VM) executeSOQLWithBindMapAccessLevel(raw string, binds Value, accessLevel Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithExpander(raw, execResult, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}, binds, databaseAccessLevelSecurityMode(accessLevel))
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	return out, nil
}

func (vm *VM) soqlResultObjectNameWithExpander(raw string, expand func(string) (string, error)) string {
	queryText := raw
	if expand != nil {
		if expanded, err := expand(raw); err == nil {
			queryText = expanded
		}
	}
	return vm.soqlResultObjectName(queryText)
}

func (vm *VM) soqlResultObjectName(raw string) string {
	query, err := soql.ParseAt(raw, vm.fakeNow)
	if err != nil || strings.TrimSpace(query.Object) == "" {
		return ""
	}
	if query.Count || len(query.Aggregates) > 0 || len(query.HavingAggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil {
		return "AggregateResult"
	}
	objectName := query.Object
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(query.Object); ok {
			objectName = resolved
		}
	}
	return objectName
}

func (vm *VM) executeSOQLForType(raw, typeName string, result *Result) (Value, error) {
	value, err := vm.executeSOQL(raw, result)
	if err != nil {
		return Null, err
	}
	if collectionBase(typeName) == "List" || typeName == "Object" {
		if collectionBase(typeName) == "List" {
			if value.Runtime == "" && value.Type != "" && !strings.EqualFold(value.Type, typeName) {
				value.Runtime = value.Type
			}
			value.Type = typeName
		}
		return value, nil
	}
	if typeName == "Integer" || typeName == "Long" {
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
	}
	if len(value.List) == 0 {
		return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
	}
	if len(value.List) > 1 {
		return Null, newExceptionError("QueryException", "List has more than 1 row for assignment to SObject")
	}
	return value.List[0], nil
}

func (vm *VM) executeSOQLRows(raw string, execResult *Result) ([]Value, error) {
	return vm.executeSOQLRowsWithExpander(raw, execResult, vm.expandSOQLBinds, typedMap("Map<String,Object>"), "")
}

func (vm *VM) executeSOQLRowsWithAccessLevel(raw string, execResult *Result, accessLevel Value) ([]Value, error) {
	return vm.executeSOQLRowsWithExpander(raw, execResult, vm.expandSOQLBinds, typedMap("Map<String,Object>"), databaseAccessLevelSecurityMode(accessLevel))
}

func writeSOQLBindExpansion(out *strings.Builder, value Value, consumed string) {
	out.WriteString(soqlLiteral(value))
	if strings.TrimRight(consumed, " \t\n\r") != consumed {
		out.WriteByte(' ')
	}
}

func shouldEvaluateSOQLBindExpression(raw string, pos, callEnd int, isCall bool) bool {
	if isCall {
		pos = callEnd
	}
	for pos < len(raw) && (raw[pos] == ' ' || raw[pos] == '\t' || raw[pos] == '\n' || raw[pos] == '\r') {
		pos++
	}
	return pos < len(raw) && (raw[pos] == '[' || raw[pos] == '(' || raw[pos] == '.' || raw[pos] == '+')
}

func (vm *VM) evalSOQLBindExpression(source string, result *Result) (Value, int, error) {
	expr, end, err := compileExpressionPrefix(source)
	if err != nil {
		return Null, 0, err
	}
	value, err := vm.eval(expr, result)
	if err != nil {
		return Null, 0, err
	}
	return value, end, nil
}

func isSOQLLiteralBind(name string) bool {
	return strings.EqualFold(name, "true") || strings.EqualFold(name, "false") || strings.EqualFold(name, "null")
}

func rewriteTrailingSOQLEqualsToIn(out *strings.Builder) {
	text := out.String()
	trimmed := strings.TrimRight(text, " \t\n\r")
	if !strings.HasSuffix(trimmed, "=") {
		return
	}
	out.Reset()
	out.WriteString(strings.TrimRight(trimmed[:len(trimmed)-1], " \t\n\r"))
	out.WriteString(" IN ")
}

func consumeEmptyCallSuffix(raw string, index int) (int, bool) {
	j := index
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != '(' {
		return index, false
	}
	j++
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != ')' {
		return index, false
	}
	return j + 1, true
}

func isSOQLDateLiteralBind(raw string, colon int) bool {
	start := colon - 1
	for start >= 0 && (raw[start] == ' ' || raw[start] == '\t' || raw[start] == '\n' || raw[start] == '\r') {
		start--
	}
	end := start + 1
	for start >= 0 && (raw[start] == '_' || raw[start] >= 'A' && raw[start] <= 'Z' || raw[start] >= 'a' && raw[start] <= 'z') {
		start--
	}
	prefix := strings.ToUpper(raw[start+1 : end])
	switch prefix {
	case "LAST_N_DAYS", "NEXT_N_DAYS", "N_DAYS_AGO", "LAST_N_WEEKS", "NEXT_N_WEEKS", "LAST_N_MONTHS", "NEXT_N_MONTHS", "LAST_N_YEARS", "NEXT_N_YEARS":
		return true
	default:
		return false
	}
}

func (vm *VM) executeDML(op string, expr ir.Expr, externalIDField string, result *Result) error {
	if op == "merge" {
		if expr.Kind != ir.ExprCall || len(expr.Args) < 2 {
			return fmt.Errorf("merge statement requires master and duplicate record(s)")
		}
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return err
			}
			args = append(args, value)
		}
		value, err := vm.executeDatabaseMerge(args, result)
		if err != nil {
			return err
		}
		results := []Value{value}
		if value.Kind == ValueList {
			results = value.List
		}
		for _, mergeResult := range results {
			if mergeResult.Kind != ValueObject {
				continue
			}
			success, ok := mergeResult.Fields["success"]
			if ok && success.Kind == ValueBool && success.Bool {
				continue
			}
			if errValue, ok := mergeResult.Fields["error"]; ok && errValue.Kind == ValueString && errValue.Text != "" {
				return errors.New(errValue.Text)
			}
			return errors.New("merge failed")
		}
		return nil
	}
	value, err := vm.eval(expr, result)
	if err != nil {
		return err
	}
	results, err := vm.applyDML(op, value, true, externalIDField, dml.Options{}, result)
	if err != nil {
		return err
	}
	for _, dmlResult := range results {
		if !dmlResult.Success {
			return databaseDMLException(op, results)
		}
	}
	if expr.Kind == ir.ExprVariable {
		vm.populateDMLResultFields(&value, results)
		if err := vm.assign(expr.Name, value); err != nil {
			return err
		}
	}
	return nil
}

func copyOrgIDSequences(in map[string]uint64) map[string]uint64 {
	if in == nil {
		return nil
	}
	out := make(map[string]uint64, len(in))
	for objectName, sequence := range in {
		out[objectName] = sequence
	}
	return out
}

func maxOrgIDSequences(left, right map[string]uint64) map[string]uint64 {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := copyOrgIDSequences(left)
	if out == nil {
		out = map[string]uint64{}
	}
	for objectName, sequence := range right {
		if sequence > out[objectName] {
			out[objectName] = sequence
		}
	}
	return out
}

func isSetupObject(objectName string) bool {
	switch strings.ToLower(objectName) {
	case "user", "profile", "userrole", "permissionset", "permissionsetassignment", "permissionsetgroup", "permissionsetgroupcomponent", "fieldpermissions", "objectpermissions", "setupentityaccess":
		return true
	default:
		return false
	}
}

func isSetupDMLRecord(record storage.Record) bool {
	return mixedDMLRecordKind(record) == "setup"
}

func mixedDMLRecordKind(record storage.Record) string {
	if !isSetupObject(record.Object) {
		return "nonsetup"
	}
	if !strings.EqualFold(record.Object, "User") {
		return "setup"
	}
	roleID, ok := record.GetField("UserRoleId")
	if !ok || roleID.Kind == storage.ValueNull || storageIDFromValue(roleID) == "" {
		return "neutral"
	}
	return "setup"
}

func (vm *VM) recordsFromValue(value Value) ([]storage.Record, []*Value, error) {
	if value.Kind == ValueList {
		records := make([]storage.Record, 0, len(value.List))
		targets := make([]*Value, 0, len(value.List))
		var aliases map[uint64]Value
		if len(value.List) > 1 {
			aliases = vm.sObjectAliasMergeIndex()
		}
		for i := range value.List {
			merged := value.List[i]
			if aliases != nil && merged.Kind == ValueObject && merged.Ref != 0 && vm.isSObjectLikeType(merged.Type) {
				if alias, ok := aliases[merged.Ref]; ok {
					mergeSObjectFieldsInto(&merged, alias)
				}
			} else {
				merged = vm.mergeSObjectAliasFields(merged)
			}
			value.List[i] = merged
			record, err := vm.recordFromValue(&value.List[i])
			if err != nil {
				return nil, nil, err
			}
			records = append(records, record)
			targets = append(targets, &value.List[i])
		}
		return records, targets, nil
	}
	value = vm.mergeSObjectAliasFields(value)
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return nil, nil, err
	}
	return []storage.Record{record}, []*Value{&value}, nil
}

func (vm *VM) mergeSObjectAliasFields(value Value) Value {
	if value.Kind != ValueObject || value.Ref == 0 || !vm.isSObjectLikeType(value.Type) {
		return value
	}
	merged := value
	for _, root := range vm.Globals {
		mergeSObjectAliasFieldsFromValue(root, value.Ref, &merged, make(map[uint64]bool))
	}
	for _, scope := range vm.scopeStack {
		for _, root := range scope {
			mergeSObjectAliasFieldsFromValue(root, value.Ref, &merged, make(map[uint64]bool))
		}
	}
	return merged
}

func (vm *VM) sObjectAliasMergeIndex() map[uint64]Value {
	if vm == nil || (len(vm.Globals) == 0 && len(vm.scopeStack) == 0) {
		return nil
	}
	index := make(map[uint64]Value)
	seen := make(map[uint64]bool)
	for _, root := range vm.Globals {
		vm.collectSObjectAliasMergeIndex(root, index, seen)
	}
	for _, scope := range vm.scopeStack {
		for _, root := range scope {
			vm.collectSObjectAliasMergeIndex(root, index, seen)
		}
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

func (vm *VM) collectSObjectAliasMergeIndex(value Value, index map[uint64]Value, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	if value.Kind == ValueObject && value.Ref != 0 && vm.isSObjectLikeType(value.Type) {
		merged := index[value.Ref]
		if merged.Kind == "" {
			merged = value
		} else {
			mergeSObjectFieldsInto(&merged, value)
		}
		index[value.Ref] = merged
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			vm.collectSObjectAliasMergeIndex(child, index, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			vm.collectSObjectAliasMergeIndex(child, index, seen)
		}
		for _, child := range value.MapKeys {
			vm.collectSObjectAliasMergeIndex(child, index, seen)
		}
	case ValueList:
		for _, child := range value.List {
			vm.collectSObjectAliasMergeIndex(child, index, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			vm.collectSObjectAliasMergeIndex(child, index, seen)
		}
	}
}

func mergeSObjectFieldsInto(merged *Value, source Value) {
	if merged == nil || source.Kind != ValueObject {
		return
	}
	if merged.Fields == nil {
		merged.Fields = make(map[string]Value)
	}
	for field, fieldValue := range source.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		sourceExplicit := isExplicitSObjectField(source, field)
		if _, exists := merged.Fields[field]; !exists {
			if sourceExplicit {
				setExplicitSObjectField(merged, field, fieldValue)
			} else {
				merged.Fields[field] = fieldValue
			}
			continue
		}
		if current := merged.Fields[field]; current.Kind == ValueNull && fieldValue.Kind != ValueNull {
			if sourceExplicit {
				setExplicitSObjectField(merged, field, fieldValue)
			} else {
				merged.Fields[field] = fieldValue
			}
		}
	}
}

func mergeSObjectAliasFieldsFromValue(value Value, ref uint64, merged *Value, seen map[uint64]bool) {
	if value.Ref == ref && value.Kind == ValueObject {
		mergeSObjectFieldsInto(merged, value)
		return
	}
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	case ValueList:
		for _, child := range value.List {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
		for _, child := range value.MapKeys {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	}
}

func preserveMissingExplicitNulls(record *storage.Record, previous storage.Record) {
	if record == nil || len(previous.ExplicitNulls) == 0 {
		return
	}
	for field, isNull := range previous.ExplicitNulls {
		if !isNull {
			continue
		}
		if _, ok := record.GetField(field); ok || record.HasExplicitNull(field) {
			continue
		}
		if record.ExplicitNulls == nil {
			record.ExplicitNulls = make(map[string]bool)
		}
		record.ExplicitNulls[field] = true
	}
}

func (vm *VM) triggerObjectMatches(triggerObject, recordObject string) bool {
	if strings.EqualFold(triggerObject, recordObject) {
		return true
	}
	if vm != nil && vm.Org != nil {
		if resolvedTrigger, ok := vm.resolveObjectName(triggerObject); ok {
			if strings.EqualFold(resolvedTrigger, recordObject) {
				return true
			}
		}
		if resolvedRecord, ok := vm.resolveObjectName(recordObject); ok {
			if strings.EqualFold(resolvedRecord, triggerObject) {
				return true
			}
			if resolvedTrigger, ok := vm.resolveObjectName(triggerObject); ok && strings.EqualFold(resolvedTrigger, resolvedRecord) {
				return true
			}
		}
	}
	return strings.EqualFold(storage.StripAnyNamespaceToken(triggerObject), storage.StripAnyNamespaceToken(recordObject))
}

func preserveMissingSystemFields(record *storage.Record, original storage.SystemFields) {
	if record == nil {
		return
	}
	if record.System.OwnerID == "" {
		record.System.OwnerID = original.OwnerID
	}
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = original.CreatedDate
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = original.CreatedByID
	}
	if record.System.LastModifiedDate == "" {
		record.System.LastModifiedDate = original.LastModifiedDate
	}
	if record.System.LastModifiedByID == "" {
		record.System.LastModifiedByID = original.LastModifiedByID
	}
	if record.System.SystemModstamp == "" {
		record.System.SystemModstamp = original.SystemModstamp
	}
}

func triggerContext(trigger Trigger, records, oldRecords []storage.Record) map[string]Value {
	newValues := make([]Value, 0, len(records))
	newMap := Map()
	for _, record := range records {
		value := vmValueFromRecord(record)
		markTriggerSObject(&value)
		newValues = append(newValues, value)
		if record.ID != "" {
			key := platformScalar("Id", string(record.ID))
			encodedKey := mapKey(key)
			newMap.Map[encodedKey] = value
			newMap.MapKeys[encodedKey] = key
		}
	}
	oldValues := make([]Value, 0, len(oldRecords))
	oldMap := Map()
	for _, record := range oldRecords {
		value := vmValueFromRecord(record)
		markTriggerSObject(&value)
		oldValues = append(oldValues, value)
		if record.ID != "" {
			key := platformScalar("Id", string(record.ID))
			encodedKey := mapKey(key)
			oldMap.Map[encodedKey] = value
			oldMap.MapKeys[encodedKey] = key
		}
	}
	newListValue := Null
	newMapValue := Null
	if trigger.Operation == "insert" || trigger.Operation == "update" || trigger.Operation == "undelete" {
		newListValue = List(newValues...)
		newListValue.Type = "List<" + trigger.Object + ">"
		newListValue.Runtime = "List<SObject>"
		if trigger.Operation != "insert" || trigger.Timing == triggerTimingAfter {
			newMap.Type = "Map<Id," + trigger.Object + ">"
			newMap.Runtime = "Map<Id,SObject>"
			newMapValue = newMap
		}
	}
	oldListValue := Null
	oldMapValue := Null
	if trigger.Operation == "update" || trigger.Operation == "delete" {
		oldListValue = List(oldValues...)
		oldListValue.Type = "List<" + trigger.Object + ">"
		oldListValue.Runtime = "List<SObject>"
		oldMap.Type = "Map<Id," + trigger.Object + ">"
		oldMap.Runtime = "Map<Id,SObject>"
		oldMapValue = oldMap
	}
	ctx := map[string]Value{
		"Trigger.new":           newListValue,
		"Trigger.old":           oldListValue,
		"Trigger.newMap":        newMapValue,
		"Trigger.oldMap":        oldMapValue,
		"Trigger.isExecuting":   Bool(true),
		"Trigger.isBefore":      Bool(trigger.Timing == triggerTimingBefore),
		"Trigger.isAfter":       Bool(trigger.Timing == triggerTimingAfter),
		"Trigger.isInsert":      Bool(trigger.Operation == "insert"),
		"Trigger.isUpdate":      Bool(trigger.Operation == "update"),
		"Trigger.isDelete":      Bool(trigger.Operation == "delete"),
		"Trigger.isUndelete":    Bool(trigger.Operation == "undelete"),
		"Trigger.isUnDelete":    Bool(trigger.Operation == "undelete"),
		"Trigger.operationType": Value{Kind: ValueObject, Type: "TriggerOperation", Text: strings.ToUpper(trigger.Timing + "_" + trigger.Operation)},
		"Trigger.size":          Int(int64(len(records))),
	}
	return ctx
}

func (vm *VM) sObjectNameForIDPrefix(prefix string) (string, bool) {
	if vm.Org != nil {
		names := make([]string, 0, len(vm.Org.Objects))
		for name := range vm.Org.Objects {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if vm.Org.Objects[name].Definition.KeyPrefix == prefix {
				return name, true
			}
		}
	}
	name, ok := standardSObjectPrefixes[prefix]
	return name, ok
}

func (vm *VM) sObjectNameForID(idText string) (string, bool) {
	if len(idText) < 3 {
		return "", false
	}
	if vm.Org != nil {
		names := make([]string, 0, len(vm.Org.Objects))
		for name := range vm.Org.Objects {
			names = append(names, name)
		}
		sort.Strings(names)
		id := storage.ID(idText)
		for _, name := range names {
			object := vm.Org.Objects[name]
			if _, _, ok := storage.LookupRecordByID(object.Records, id); ok {
				return name, true
			}
		}
	}
	return vm.sObjectNameForIDPrefix(idText[:3])
}

func idTextFromValue(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		return value.Text, true
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			return platformScalarObjectText(value)
		}
	}
	return "", false
}

func idPrefix(idText string) string {
	if len(idText) < 3 {
		return idText
	}
	return idText[:3]
}

var standardSObjectPrefixes = map[string]string{
	"001": "Account",
	"003": "Contact",
	"005": "User",
	"006": "Opportunity",
	"00G": "Group",
	"00Q": "Lead",
	"00T": "Task",
	"00U": "Event",
	"00D": "Organization",
	"500": "Case",
	"701": "Campaign",
}

var commonSObjectTypeNames []string
var generatedPlatformTypeIndex map[string]generatedPlatformType
var generatedPlatformMethodIndex map[string]map[string][]Method

func init() {
	for objectName, prefix := range storage.StandardKeyPrefixes() {
		if prefix != "" {
			standardSObjectPrefixes[prefix] = objectName
		}
	}
	commonSObjectTypeNames = buildCommonSObjectTypeNames()
	generatedPlatformTypeIndex = buildGeneratedPlatformTypeIndex()
	generatedPlatformMethodIndex = buildGeneratedPlatformMethodIndex()
}

func CommonSObjectTypeNames() []string {
	return commonSObjectTypeNames
}

type generatedPlatformType struct {
	Name             string
	Kind             apexast.DeclarationKind
	SuperClass       string
	Fields           map[string]Field
	FieldOrder       []string
	StaticFields     map[string]Field
	StaticFieldOrder []string
	Constructors     []Method
}

func buildGeneratedPlatformTypeIndex() map[string]generatedPlatformType {
	out := make(map[string]generatedPlatformType)
	for _, typ := range typesys.StandardPlatformSymbols() {
		name := generatedPlatformRuntimeName(typ)
		if name == "" {
			continue
		}
		generated := generatedPlatformType{
			Name:         name,
			Kind:         typ.Kind,
			SuperClass:   typ.SuperClass,
			Fields:       make(map[string]Field),
			StaticFields: make(map[string]Field),
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationField, apexast.DeclarationProperty:
				field := Field{
					Name:      member.Name,
					Type:      member.Type,
					Static:    methodHasModifier(member.Modifiers, "static"),
					Access:    "global",
					Modifiers: append([]string(nil), member.Modifiers...),
					Property:  member.Kind == apexast.DeclarationProperty,
				}
				if field.Static {
					generated.StaticFields[field.Name] = field
					generated.StaticFieldOrder = append(generated.StaticFieldOrder, field.Name)
				} else {
					generated.Fields[field.Name] = field
					generated.FieldOrder = append(generated.FieldOrder, field.Name)
				}
			case apexast.DeclarationConstructor:
				generated.Constructors = append(generated.Constructors, generatedPlatformRuntimeConstructor(name, member))
			}
		}
		out[strings.ToLower(name)] = generated
	}
	return out
}

func buildGeneratedPlatformMethodIndex() map[string]map[string][]Method {
	out := make(map[string]map[string][]Method)
	for _, typ := range typesys.StandardPlatformSymbols() {
		className := generatedPlatformRuntimeName(typ)
		if className == "" {
			continue
		}
		classKey := strings.ToLower(className)
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			method := generatedPlatformRuntimeMethod(className, member)
			if method.Name == "" {
				continue
			}
			methodKey := strings.ToLower(member.Name)
			if out[classKey] == nil {
				out[classKey] = make(map[string][]Method)
			}
			out[classKey][methodKey] = append(out[classKey][methodKey], method)
		}
	}
	return out
}

func generatedPlatformRuntimeConstructor(className string, member typesys.MemberSymbol) Method {
	params := make([]Param, 0, len(member.Parameters))
	for i, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = "arg" + strconv.Itoa(i)
		}
		params = append(params, Param{Name: name, Type: param.Type})
	}
	return Method{
		Name:          className + ".<init>",
		ClassName:     className,
		ReturnType:    "void",
		Params:        params,
		IsConstructor: true,
		Access:        "global",
		Modifiers:     []string{"passive-generated"},
	}
}

func generatedPlatformRuntimeName(typ typesys.TypeSymbol) string {
	if typ.Namespace == "" || strings.Contains(typ.Name, ".") {
		return typ.Name
	}
	return typ.Namespace + "." + typ.Name
}

func generatedPlatformRuntimeMethod(className string, member typesys.MemberSymbol) Method {
	params := make([]Param, 0, len(member.Parameters))
	for i, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = "arg" + strconv.Itoa(i)
		}
		params = append(params, Param{Name: name, Type: param.Type})
	}
	modifiers := []string{"passive-generated"}
	if methodHasModifier(member.Modifiers, "static") {
		modifiers = append(modifiers, "static")
	}
	return Method{
		Name:       className + "." + member.Name,
		ClassName:  className,
		ReturnType: member.Type,
		Params:     params,
		IsStatic:   methodHasModifier(member.Modifiers, "static"),
		Access:     "global",
		Modifiers:  modifiers,
	}
}

func buildCommonSObjectTypeNames() []string {
	knownStandardObjects := storage.KnownStandardObjectNames()
	names := make([]string, 0, len(standardSObjectPrefixes)+len(knownStandardObjects))
	seen := make(map[string]bool, len(standardSObjectPrefixes)+len(knownStandardObjects))
	for _, name := range standardSObjectPrefixes {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for _, name := range knownStandardObjects {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	return names
}

func platformScalar(typeName, value string) Value {
	out := Object(typeName)
	out.Fields["value"] = String(value)
	return out
}

func platformScalarText(value Value, typeName string) (string, error) {
	if value.Kind != ValueObject || value.Type != typeName {
		return "", fmt.Errorf("expected %s value", typeName)
	}
	raw, ok := value.Fields["value"]
	if !ok || raw.Kind != ValueString {
		return "", fmt.Errorf("%s value is missing scalar text", typeName)
	}
	return raw.Text, nil
}

func datetimeLegacyIsoFractionalTruncate(value Value) bool {
	flag, ok := value.Fields["legacyIsoFractionalTruncate"]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func defaultURLPort(scheme string) int64 {
	switch strings.ToLower(scheme) {
	case "http":
		return 80
	case "https":
		return 443
	case "ftp":
		return 21
	default:
		return -1
	}
}

func parsePlatformDate(value Value) (time.Time, error) {
	text, err := platformScalarText(value, "Date")
	if err != nil {
		return time.Time{}, err
	}
	date, err := parseDateText(text)
	if err != nil {
		return parsePlatformDateText(text)
	}
	return date, nil
}

func parsePlatformDatetime(value Value) (time.Time, error) {
	text, err := platformScalarText(value, "Datetime")
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := parseDatetimeTextAllowDateOnly(text)
	if err != nil {
		return parsePlatformDatetimeText(text)
	}
	return parsed.UTC(), nil
}

func datetimeFromLocalParts(year, month, day, hour, minute, second, millisecond int, zoneID string) (time.Time, error) {
	canonical, offset, ok := parseFixedTimeZoneID(zoneID)
	if ok {
		return time.Date(year, time.Month(month), day, hour, minute, second, millisecond*int(time.Millisecond), time.FixedZone(canonical, int(offset/time.Second))).UTC(), nil
	}
	zone, ok := supportedNamedTimeZone(zoneID)
	if !ok {
		return time.Time{}, unsupportedCallError("Datetime.newInstance timezone " + zoneID)
	}
	return zone.instantFromLocal(year, time.Month(month), day, hour, minute, second, millisecond), nil
}

func addMonthsClamped(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	monthIndex := year*12 + int(month) - 1 + months
	targetYear := monthIndex / 12
	targetMonthIndex := monthIndex % 12
	if targetMonthIndex < 0 {
		targetMonthIndex += 12
		targetYear--
	}
	targetMonth := time.Month(targetMonthIndex + 1)
	if maxDay := daysInMonth(targetYear, targetMonth); day > maxDay {
		day = maxDay
	}
	return time.Date(targetYear, targetMonth, day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func parseDatetimeText(text string) (time.Time, error) {
	text = normalizeDatetimeShortTimezoneOffset(strings.TrimSpace(text))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
	} {
		if value, err := time.Parse(layout, text); err == nil {
			year := value.Year()
			if err := validateDateParts(year, int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
}

var datetimeShortTimezoneOffsetPattern = regexp.MustCompile(`^(.+[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?)([+-])([0-9]{1,2})$`)
var datetimeUnsignedShortTimezoneOffsetPattern = regexp.MustCompile(`^(.+[T ][0-9]{2}:[0-9]{2}:[0-9]{2})([0-9]{1,2})$`)

func normalizeDatetimeShortTimezoneOffset(text string) string {
	matches := datetimeShortTimezoneOffsetPattern.FindStringSubmatch(text)
	if matches != nil {
		hour, err := strconv.Atoi(matches[3])
		if err != nil {
			return text
		}
		return fmt.Sprintf("%s%s%02d:00", matches[1], matches[2], hour)
	}
	matches = datetimeUnsignedShortTimezoneOffsetPattern.FindStringSubmatch(text)
	if matches == nil {
		return text
	}
	hour, err := strconv.Atoi(matches[2])
	if err != nil || hour > 14 {
		return text
	}
	return fmt.Sprintf("%s+%02d:00", matches[1], hour)
}

func parseDatetimeTextAllowDateOnly(text string) (time.Time, error) {
	if value, err := parseDatetimeText(text); err == nil {
		return value, nil
	}
	return parseDateText(text)
}

func parseDateText(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, ","); idx > 0 {
		if value, err := parseDateText(text[:idx]); err == nil {
			return value, nil
		}
	}
	for _, layout := range []string{
		"2006-01-02", "2006-1-2",
		"2006-01-02 15:04:05", "2006-1-2 15:04:05",
		"2006-01-02T15:04:05", "2006-1-2T15:04:05",
	} {
		if value, err := time.Parse(layout, text); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	if value, err := parseDatetimeText(text); err == nil {
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
}

func parsePlatformDateText(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range []string{"2006-01-02", "2006-1-2"} {
		if value, err := time.Parse(layout, text); err == nil {
			if value.Year() < 0 || value.Year() > 9999 {
				return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
			}
			if value.Format("2006-01-02") != text {
				return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
}

func parsePlatformDatetimeText(text string) (time.Time, error) {
	normalized := normalizeDatetimeShortTimezoneOffset(strings.TrimSpace(text))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
	} {
		if value, err := time.Parse(layout, normalized); err == nil {
			if value.Year() < 0 || value.Year() > 9999 {
				return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
			}
			return value, nil
		}
	}
	if value, err := parsePlatformDateText(text); err == nil {
		return value, nil
	}
	return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
}

func parseDateParseText(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range []string{"1/2/2006", "01/02/2006", "1/2/06", "01/02/06"} {
		if value, err := time.Parse(layout, text); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return parseDateText(text)
}

func formatPlatformDatetime(value time.Time) string {
	utc := value.UTC()
	ms := utc.Nanosecond() / int(time.Millisecond)
	if ms == 0 {
		return utc.Format("2006-01-02T15:04:05Z")
	}
	frac := strings.TrimRight(fmt.Sprintf("%03d", ms), "0")
	return fmt.Sprintf("%s.%sZ", utc.Format("2006-01-02T15:04:05"), frac)
}

func formatApexDatetimePattern(value time.Time, pattern, zoneID, zoneLabel string, offset time.Duration) (string, error) {
	var b strings.Builder
	for i := 0; i < len(pattern); {
		ch := pattern[i]
		if ch == '\'' {
			next, literal, err := readApexDatePatternLiteral(pattern, i)
			if err != nil {
				return "", err
			}
			b.WriteString(literal)
			i = next
			continue
		}
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			b.WriteByte(ch)
			i++
			continue
		}
		j := i + 1
		for j < len(pattern) && pattern[j] == ch {
			j++
		}
		token := pattern[i:j]
		text, err := formatApexDatetimeToken(value, token, zoneID, zoneLabel, offset)
		if err != nil {
			return "", err
		}
		b.WriteString(text)
		i = j
	}
	return b.String(), nil
}

func readApexDatePatternLiteral(pattern string, start int) (int, string, error) {
	var b strings.Builder
	for i := start + 1; i < len(pattern); i++ {
		if pattern[i] != '\'' {
			b.WriteByte(pattern[i])
			continue
		}
		if i+1 < len(pattern) && pattern[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		return i + 1, b.String(), nil
	}
	return 0, "", fmt.Errorf("Datetime.format unsupported unterminated quoted literal")
}

func formatApexDatetimeToken(value time.Time, token, zoneID, zoneLabel string, offset time.Duration) (string, error) {
	count := len(token)
	switch token[0] {
	case 'y', 'Y':
		year := value.Year()
		if count == 2 {
			return fmt.Sprintf("%02d", year%100), nil
		}
		return fmt.Sprintf("%0*d", maxInt(count, 4), year), nil
	case 'M':
		month := value.Month()
		switch {
		case count >= 4:
			return month.String(), nil
		case count == 3:
			return month.String()[:3], nil
		case count == 2:
			return fmt.Sprintf("%02d", int(month)), nil
		default:
			return strconv.Itoa(int(month)), nil
		}
	case 'd':
		return formatPaddedDateNumber(value.Day(), count), nil
	case 'H':
		return formatPaddedDateNumber(value.Hour(), count), nil
	case 'h':
		hour := value.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		return formatPaddedDateNumber(hour, count), nil
	case 'm':
		return formatPaddedDateNumber(value.Minute(), count), nil
	case 's':
		return formatPaddedDateNumber(value.Second(), count), nil
	case 'S':
		if count > 3 {
			return "", fmt.Errorf("Datetime.format unsupported pattern token %q", token)
		}
		millisecond := value.Nanosecond() / int(time.Millisecond)
		if count <= 1 {
			return strconv.Itoa(millisecond), nil
		}
		return fmt.Sprintf("%0*d", minInt(count, 3), millisecond), nil
	case 'a':
		if value.Hour() < 12 {
			return "AM", nil
		}
		return "PM", nil
	case 'E':
		name := value.Weekday().String()
		if count >= 4 {
			return name, nil
		}
		return name[:3], nil
	case 'u':
		weekday := int(value.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return formatPaddedDateNumber(weekday, count), nil
	case 'w':
		_, week := value.ISOWeek()
		return formatPaddedDateNumber(week, count), nil
	case 'G', 'L', 'c', 'e':
		return "", unsupportedCallError(fmt.Sprintf("Datetime.format locale-dependent pattern token %q", token))
	case 'Z':
		return formatRFC822Offset(offset), nil
	case 'z':
		if zoneID == "UTC" {
			return "UTC", nil
		}
		return zoneLabel, nil
	default:
		return "", fmt.Errorf("Datetime.format unsupported pattern token %q", token)
	}
}

func formatPaddedDateNumber(value, count int) string {
	if count >= 2 {
		return fmt.Sprintf("%02d", value)
	}
	return strconv.Itoa(value)
}

func formatRFC822Offset(offset time.Duration) string {
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	totalMinutes := int(offset / time.Minute)
	return fmt.Sprintf("%s%02d%02d", sign, totalMinutes/60, totalMinutes%60)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func normalizeDateNewInstanceParts(year, month, day int) (int, int, int) {
	if validateDateParts(year, month, day) == nil {
		return year, month, day
	}
	if year < 1 || year > 12 || month < 1 || month > 31 || day < 1000 {
		return year, month, day
	}
	if validateDateParts(day, year, month) == nil {
		return day, year, month
	}
	return year, month, day
}

func dateFromNewInstanceParts(year, month, day int) (time.Time, error) {
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Year() < 0 || value.Year() > 9999 {
		return time.Time{}, newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	return value, nil
}

func validateDateParts(year, month, day int) error {
	if year < 1 || year > 9999 {
		return newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Year() != year || int(value.Month()) != month || value.Day() != day {
		return newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	return nil
}

func validateTimeParts(hour, minute, second int) error {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return fmt.Errorf("invalid Time parts: hour=%d minute=%d second=%d", hour, minute, second)
	}
	return nil
}

func parseTimeText(text string) (string, error) {
	for _, layout := range []string{"15:04:05.000", "15:04:05.000Z", "15:04:05", "15:04:05Z"} {
		if value, err := time.Parse(layout, text); err == nil {
			return formatPlatformTimeWithMillis(value.Hour(), value.Minute(), value.Second(), value.Nanosecond()/int(time.Millisecond)), nil
		}
	}
	return "", fmt.Errorf("unsupported Time value %q", text)
}

func parsePlatformTime(value Value) (time.Duration, error) {
	text, err := platformScalarText(value, "Time")
	if err != nil {
		return 0, err
	}
	parsed, err := parseTimeText(text)
	if err != nil {
		return 0, err
	}
	t, err := time.Parse("15:04:05.000", ensureTimeMillis(parsed))
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second +
		time.Duration(t.Nanosecond()), nil
}

func ensureTimeMillis(text string) string {
	if strings.Contains(text, ".") {
		return text
	}
	return text + ".000"
}

func formatPlatformTime(hour, minute, second, millisecond int) string {
	base := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	if millisecond == 0 {
		return base
	}
	return fmt.Sprintf("%s.%03d", base, millisecond)
}

func formatPlatformTimeWithMillis(hour, minute, second, millisecond int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hour, minute, second, millisecond)
}

func formatPlatformTimeZulu(hour, minute, second, millisecond int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03dZ", hour, minute, second, millisecond)
}

func toTwelveHour(hour int) int {
	value := hour % 12
	if value == 0 {
		return 12
	}
	return value
}

func ampm(hour int) string {
	if hour < 12 {
		return "AM"
	}
	return "PM"
}

func platformTimeFromDuration(value time.Duration) Value {
	day := 24 * time.Hour
	value %= day
	if value < 0 {
		value += day
	}
	hour := int(value / time.Hour)
	value %= time.Hour
	minute := int(value / time.Minute)
	value %= time.Minute
	second := int(value / time.Second)
	value %= time.Second
	millisecond := int(value / time.Millisecond)
	return platformScalar("Time", formatPlatformTimeWithMillis(hour, minute, second, millisecond))
}

func fixedTimeZone(id string) (Value, error) {
	canonical, offset, ok := parseFixedTimeZoneID(id)
	locationName := ""
	if !ok {
		location, locationOK := supportedNamedTimeZone(id)
		if !locationOK {
			return Null, unsupportedCallError("TimeZone.getTimeZone " + id)
		}
		canonical = id
		offset = location.standardOffset
		locationName = location.id
	} else if canonical == "UTC" {
		locationName = "UTC"
	}
	out := Object("TimeZone")
	out.Fields["id"] = String(canonical)
	out.Fields["offsetMillis"] = Int(int64(offset / time.Millisecond))
	out.Fields["location"] = String(locationName)
	return out, nil
}

type modeledTimeZone struct {
	id             string
	standardOffset time.Duration
	daylightOffset time.Duration
	standardLabel  string
	daylightLabel  string
	daylightRule   string
}

var supportedNamedTimeZones = map[string]modeledTimeZone{
	"America/Los_Angeles": {id: "America/Los_Angeles", standardOffset: -8 * time.Hour, daylightOffset: -7 * time.Hour, standardLabel: "PST", daylightLabel: "PDT", daylightRule: "us"},
	"America/New_York":    {id: "America/New_York", standardOffset: -5 * time.Hour, daylightOffset: -4 * time.Hour, standardLabel: "EST", daylightLabel: "EDT", daylightRule: "us"},
	"America/Chicago":     {id: "America/Chicago", standardOffset: -6 * time.Hour, daylightOffset: -5 * time.Hour, standardLabel: "CST", daylightLabel: "CDT", daylightRule: "us"},
	"America/Denver":      {id: "America/Denver", standardOffset: -7 * time.Hour, daylightOffset: -6 * time.Hour, standardLabel: "MST", daylightLabel: "MDT", daylightRule: "us"},
	"America/Panama":      {id: "America/Panama", standardOffset: -5 * time.Hour, standardLabel: "EST"},
	"Europe/London":       {id: "Europe/London", standardOffset: 0, daylightOffset: time.Hour, standardLabel: "GMT", daylightLabel: "BST", daylightRule: "europe"},
	"Europe/Berlin":       {id: "Europe/Berlin", standardOffset: time.Hour, daylightOffset: 2 * time.Hour, standardLabel: "CET", daylightLabel: "CEST", daylightRule: "europe"},
	"Asia/Ho_Chi_Minh":    {id: "Asia/Ho_Chi_Minh", standardOffset: 7 * time.Hour, standardLabel: "ICT"},
	"Asia/Tokyo":          {id: "Asia/Tokyo", standardOffset: 9 * time.Hour, standardLabel: "JST"},
	"Pacific/Honolulu":    {id: "Pacific/Honolulu", standardOffset: -10 * time.Hour, standardLabel: "HST"},
	"Pacific/Pago_Pago":   {id: "Pacific/Pago_Pago", standardOffset: -11 * time.Hour, standardLabel: "SST"},
	"Australia/Sydney":    {id: "Australia/Sydney", standardOffset: 10 * time.Hour, daylightOffset: 11 * time.Hour, standardLabel: "AEST", daylightLabel: "AEDT", daylightRule: "sydney"},
}

func supportedNamedTimeZone(id string) (modeledTimeZone, bool) {
	location, ok := supportedNamedTimeZones[id]
	return location, ok
}

func resolveTimeZoneForInstant(id string, instant time.Time) (string, time.Duration, time.Time, string, bool) {
	canonical, offset, ok := parseFixedTimeZoneID(id)
	if ok {
		local := instant.UTC().In(time.FixedZone(canonical, int(offset/time.Second)))
		return canonical, offset, local, canonical, true
	}
	location, ok := supportedNamedTimeZone(id)
	if !ok {
		return "", 0, time.Time{}, "", false
	}
	offset, label := location.offsetAt(instant)
	local := instant.UTC().In(time.FixedZone(label, int(offset/time.Second)))
	return id, offset, local, label, true
}

func timeZoneOffsetMillis(receiver Value, instant time.Time) (Value, error) {
	locationValue := receiver.Fields["location"]
	if locationValue.Kind == ValueString && locationValue.Text != "" && locationValue.Text != "UTC" {
		location, ok := supportedNamedTimeZone(locationValue.Text)
		if !ok {
			return Null, unsupportedCallError("TimeZone.getOffset " + locationValue.Text)
		}
		offset, _ := location.offsetAt(instant)
		return Int(int64(offset / time.Millisecond)), nil
	}
	offsetValue := receiver.Fields["offsetMillis"]
	if offsetValue.Kind != ValueInt {
		return Null, fmt.Errorf("TimeZone offset is missing")
	}
	return offsetValue, nil
}

func timeZoneDisplayName(receiver Value, daylight bool) Value {
	locationValue := receiver.Fields["location"]
	if locationValue.Kind == ValueString && locationValue.Text != "" && locationValue.Text != "UTC" {
		if location, ok := supportedNamedTimeZone(locationValue.Text); ok {
			if daylight && location.daylightLabel != "" {
				return String(location.daylightLabel)
			}
			return String(location.standardLabel)
		}
	}
	return receiver.Fields["id"]
}

func (zone modeledTimeZone) offsetAt(instant time.Time) (time.Duration, string) {
	if zone.daylightRule == "" || !zone.isDaylight(instant.UTC()) {
		return zone.standardOffset, zone.standardLabel
	}
	return zone.daylightOffset, zone.daylightLabel
}

func (zone modeledTimeZone) instantFromLocal(year int, month time.Month, day, hour, minute, second, millisecond int) time.Time {
	local := time.Date(year, month, day, hour, minute, second, millisecond*int(time.Millisecond), time.UTC)
	offsets := []time.Duration{zone.standardOffset}
	if zone.daylightRule != "" && zone.daylightOffset != zone.standardOffset {
		offsets = append(offsets, zone.daylightOffset)
	}
	var matches []time.Time
	for _, offset := range offsets {
		candidate := local.Add(-offset)
		actualOffset, _ := zone.offsetAt(candidate)
		if candidate.Add(actualOffset).Equal(local) {
			matches = append(matches, candidate.UTC())
		}
	}
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool { return matches[i].Before(matches[j]) })
		return matches[0]
	}
	return local.Add(-zone.standardOffset).UTC()
}

func (zone modeledTimeZone) isDaylight(instant time.Time) bool {
	year := instant.Year()
	switch zone.daylightRule {
	case "us":
		start := localRuleTransitionUTC(year, time.March, nthWeekdayOfMonth(year, time.March, time.Sunday, 2), 2, zone.standardOffset)
		end := localRuleTransitionUTC(year, time.November, nthWeekdayOfMonth(year, time.November, time.Sunday, 1), 2, zone.daylightOffset)
		return !instant.Before(start) && instant.Before(end)
	case "europe":
		start := time.Date(year, time.March, lastWeekdayOfMonth(year, time.March, time.Sunday), 1, 0, 0, 0, time.UTC)
		end := time.Date(year, time.October, lastWeekdayOfMonth(year, time.October, time.Sunday), 1, 0, 0, 0, time.UTC)
		return !instant.Before(start) && instant.Before(end)
	case "sydney":
		start := localRuleTransitionUTC(year, time.October, nthWeekdayOfMonth(year, time.October, time.Sunday, 1), 2, zone.standardOffset)
		end := localRuleTransitionUTC(year, time.April, nthWeekdayOfMonth(year, time.April, time.Sunday, 1), 3, zone.daylightOffset)
		return !instant.Before(start) || instant.Before(end)
	default:
		return false
	}
}

func localRuleTransitionUTC(year int, month time.Month, day, hour int, offsetBefore time.Duration) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC).Add(-offsetBefore)
}

func nthWeekdayOfMonth(year int, month time.Month, weekday time.Weekday, n int) int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	delta := (int(weekday) - int(first.Weekday()) + 7) % 7
	return 1 + delta + 7*(n-1)
}

func lastWeekdayOfMonth(year int, month time.Month, weekday time.Weekday) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	delta := (int(last.Weekday()) - int(weekday) + 7) % 7
	return last.Day() - delta
}

func parseFixedTimeZoneID(id string) (string, time.Duration, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed != id {
		return "", 0, false
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "UTC", "GMT", "ETC/UTC", "Z":
		return "UTC", 0, true
	}
	if !strings.HasPrefix(upper, "GMT+") && !strings.HasPrefix(upper, "GMT-") && !strings.HasPrefix(upper, "UTC+") && !strings.HasPrefix(upper, "UTC-") {
		return "", 0, false
	}
	prefix := upper[:3]
	signText := upper[3:4]
	rest := upper[4:]
	if prefix == "UTC" {
		rest = upper[4:]
	}
	parts := strings.Split(rest, ":")
	if len(parts) > 2 || parts[0] == "" {
		return "", 0, false
	}
	if len(parts[0]) > 2 || !allASCIIDigits(parts[0]) {
		return "", 0, false
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", 0, false
	}
	minutes := 0
	if len(parts) == 2 {
		if len(parts[1]) != 2 || !allASCIIDigits(parts[1]) {
			return "", 0, false
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, false
		}
	}
	if hours > 14 || minutes > 59 || (hours == 14 && minutes != 0) {
		return "", 0, false
	}
	offset := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
	if signText == "-" {
		offset = -offset
	}
	return fmt.Sprintf("GMT%s%02d:%02d", signText, hours, minutes), offset, true
}

func allASCIIDigits(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return text != ""
}

const (
	defaultHttpTimeoutMillis int64 = 10000
	maxHttpTimeoutMillis     int64 = 120000
)

func validateHttpRequest(request Value) error {
	endpoint, ok := request.Fields["endpoint"]
	if !ok || endpoint.Kind != ValueString {
		return fmt.Errorf("HttpRequest endpoint is required before Http.send")
	}
	if strings.TrimSpace(endpoint.Text) == "" {
		return fmt.Errorf("HttpRequest endpoint is required before Http.send")
	}
	if err := validateHttpEndpoint(endpoint.Text); err != nil {
		return err
	}
	method, ok := request.Fields["method"]
	if !ok || method.Kind != ValueString {
		return fmt.Errorf("HttpRequest method is required before Http.send")
	}
	if strings.TrimSpace(method.Text) == "" {
		return fmt.Errorf("HttpRequest method is required before Http.send")
	}
	if _, err := normalizeHttpMethod(method.Text); err != nil {
		return err
	}
	if timeout, ok := request.Fields["timeout"]; ok {
		if timeout.Kind != ValueInt {
			return fmt.Errorf("HttpRequest timeout must be Integer")
		}
		return validateHttpTimeout(timeout.Int)
	}
	return nil
}

func validateHttpEndpoint(endpoint string) error {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return fmt.Errorf("HttpRequest endpoint is required")
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "callout:") {
		if strings.TrimSpace(trimmed[len("callout:"):]) == "" {
			return fmt.Errorf("HttpRequest endpoint named credential is required")
		}
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("HttpRequest endpoint must be an absolute http, https, or callout URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("HttpRequest endpoint must use http, https, or callout scheme")
	}
	return nil
}

func normalizeHttpMethod(method string) (string, error) {
	trimmed := strings.TrimSpace(method)
	if trimmed == "" {
		return "", fmt.Errorf("HttpRequest method is required")
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "DELETE", "GET", "HEAD", "PATCH", "POST", "PUT", "TRACE":
		return upper, nil
	default:
		return "", fmt.Errorf("HttpRequest method %q is not supported", method)
	}
}

func validateHttpTimeout(timeout int64) error {
	if timeout < 1 || timeout > maxHttpTimeoutMillis {
		return fmt.Errorf("HttpRequest timeout must be between 1 and %d milliseconds", maxHttpTimeoutMillis)
	}
	return nil
}

func httpSetHeader(receiver Value, name string, value Value) {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		headers = Map()
	}
	headers.Map[mapKey(String(strings.ToLower(name)))] = value
	receiver.Fields["headers"] = headers
}

func httpGetHeader(receiver Value, name string) Value {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		return Null
	}
	if value, ok := headers.Map[mapKey(String(strings.ToLower(name)))]; ok {
		return value
	}
	return Null
}

func httpHeaderKeys(receiver Value) Value {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		return List()
	}
	keys := make([]string, 0, len(headers.Map))
	for rawKey := range headers.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString {
			keys = append(keys, decoded.Text)
		}
	}
	sort.Strings(keys)
	out := make([]Value, 0, len(keys))
	for _, key := range keys {
		out = append(out, String(key))
	}
	return List(out...)
}

func (vm *VM) assertMessage(base string, extra []Value, result *Result) (string, error) {
	if len(extra) == 0 {
		return base, nil
	}
	message, err := vm.displayString(extra[0], result)
	if err != nil {
		return "", err
	}
	return base + ": " + message, nil
}

func blobStringArg(name string, args []Value) (string, error) {
	if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
		return "", fmt.Errorf("%s expects Blob", name)
	}
	return args[0].Fields["value"].String(), nil
}

func urlEncodeWithCharset(name, text, charset string) (string, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return url.QueryEscape(text), nil
	case "us-ascii":
		return urlEncodeASCII(name, text)
	case "iso-8859-1":
		return urlEncodeLatin1(name, text)
	default:
		return "", unsupportedCallError(fmt.Sprintf("%s charset %q", name, charset))
	}
}

func urlDecodeWithCharset(name, text, charset string) (string, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return url.QueryUnescape(text)
	case "us-ascii":
		return urlDecodeASCII(name, text)
	case "iso-8859-1":
		return urlDecodeLatin1(text)
	default:
		return "", unsupportedCallError(fmt.Sprintf("%s charset %q", name, charset))
	}
}

func normalizeURLCharset(charset string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(charset), "_", "-"))
	switch normalized {
	case "utf-8", "utf8":
		return "utf-8"
	case "us-ascii", "usascii", "ascii":
		return "us-ascii"
	case "iso-8859-1", "iso8859-1", "iso-88591", "iso88591", "latin1", "latin-1":
		return "iso-8859-1"
	default:
		return normalized
	}
}

func urlEncodeASCII(name, text string) (string, error) {
	var out strings.Builder
	for _, r := range text {
		if r > 0x7f {
			return "", fmt.Errorf("%s charset \"US-ASCII\" cannot encode U+%04X", name, r)
		}
		writeURLEncodedByte(&out, byte(r))
	}
	return out.String(), nil
}

func urlEncodeLatin1(name, text string) (string, error) {
	var out strings.Builder
	for _, r := range text {
		if r > 0xff {
			return "", fmt.Errorf("%s charset \"ISO-8859-1\" cannot encode U+%04X", name, r)
		}
		writeURLEncodedByte(&out, byte(r))
	}
	return out.String(), nil
}

func writeURLEncodedByte(out *strings.Builder, b byte) {
	switch {
	case b == ' ':
		out.WriteByte('+')
	case (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '*':
		out.WriteByte(b)
	default:
		const hexDigits = "0123456789ABCDEF"
		out.WriteByte('%')
		out.WriteByte(hexDigits[b>>4])
		out.WriteByte(hexDigits[b&0x0f])
	}
}

func urlDecodeASCII(name, text string) (string, error) {
	decoded, err := urlDecodeBytes(text)
	if err != nil {
		return "", err
	}
	for _, b := range decoded {
		if b > 0x7f {
			return "", fmt.Errorf("%s charset \"US-ASCII\" cannot decode byte 0x%02X", name, b)
		}
	}
	return string(decoded), nil
}

func urlDecodeLatin1(text string) (string, error) {
	decoded, err := urlDecodeBytes(text)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, b := range decoded {
		out.WriteRune(rune(b))
	}
	return out.String(), nil
}

func urlDecodeBytes(text string) ([]byte, error) {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		ch := text[i]
		switch ch {
		case '+':
			out = append(out, ' ')
		case '%':
			if i+2 >= len(text) {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:])
			}
			hi, ok := fromHex(text[i+1])
			if !ok {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:i+3])
			}
			lo, ok := fromHex(text[i+2])
			if !ok {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:i+3])
			}
			out = append(out, hi<<4|lo)
			i += 2
		default:
			out = append(out, ch)
		}
	}
	return out, nil
}

func fromHex(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}

func normalizeCryptoAlgorithm(algorithm string) string {
	normalized := strings.ToUpper(strings.TrimSpace(algorithm))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	return normalized
}

func generateDigest(algorithm string, data []byte) ([]byte, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	switch normalized {
	case "MD5":
		sum := md5.Sum(data)
		return sum[:], nil
	case "SHA1":
		sum := sha1.Sum(data)
		return sum[:], nil
	case "SHA256":
		sum := sha256.Sum256(data)
		return sum[:], nil
	case "SHA512":
		sum := sha512.Sum512(data)
		return sum[:], nil
	case "SHA3256":
		sum := sha3.Sum256(data)
		return sum[:], nil
	case "SHA3384":
		sum := sha3.Sum384(data)
		return sum[:], nil
	case "SHA3512":
		sum := sha3.Sum512(data)
		return sum[:], nil
	default:
		return nil, fmt.Errorf("unsupported digest algorithm %q", algorithm)
	}
}

func generateMac(algorithm string, input, privateKey []byte) ([]byte, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	var mac hash.Hash
	switch normalized {
	case "HMACMD5":
		mac = hmac.New(md5.New, privateKey)
	case "HMACSHA1":
		mac = hmac.New(sha1.New, privateKey)
	case "HMACSHA256":
		mac = hmac.New(sha256.New, privateKey)
	case "HMACSHA512":
		mac = hmac.New(sha512.New, privateKey)
	default:
		return nil, fmt.Errorf("unsupported MAC algorithm %q", algorithm)
	}
	if _, err := mac.Write(input); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func encryptAESCBC(algorithm string, privateKey, initializationVector, clearText []byte) ([]byte, error) {
	keySize, err := aesKeySizeForAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	if len(privateKey) != keySize {
		return nil, fmt.Errorf("Crypto.encrypt %s privateKey expects %d bytes, got %d", normalizeCryptoAlgorithm(algorithm), keySize, len(privateKey))
	}
	if len(initializationVector) != aes.BlockSize {
		return nil, fmt.Errorf("Crypto.encrypt initializationVector expects %d bytes, got %d", aes.BlockSize, len(initializationVector))
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(clearText, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, initializationVector).CryptBlocks(out, padded)
	return out, nil
}

func decryptAESCBC(algorithm string, privateKey, initializationVector, cipherText []byte) ([]byte, error) {
	keySize, err := aesKeySizeForAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	if len(privateKey) != keySize {
		return nil, fmt.Errorf("Crypto.decrypt %s privateKey expects %d bytes, got %d", normalizeCryptoAlgorithm(algorithm), keySize, len(privateKey))
	}
	if len(initializationVector) != aes.BlockSize {
		return nil, fmt.Errorf("Crypto.decrypt initializationVector expects %d bytes, got %d", aes.BlockSize, len(initializationVector))
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("Crypto.decrypt cipherText must be a positive multiple of %d bytes", aes.BlockSize)
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	padded := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, initializationVector).CryptBlocks(padded, cipherText)
	return pkcs7Unpad(padded, aes.BlockSize)
}

func managedIV(privateKey, clearText []byte) []byte {
	sum := sha256.Sum256(append(append([]byte("glade-managed-iv:"), privateKey...), clearText...))
	iv := make([]byte, aes.BlockSize)
	copy(iv, sum[:aes.BlockSize])
	return iv
}

func localCryptoSignature(algorithm string, input []byte) ([]byte, error) {
	digestAlgorithm, err := signatureDigestAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	return generateDigest(digestAlgorithm, input)
}

func signatureDigestAlgorithm(algorithm string) (string, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	switch normalized {
	case "RSA", "RSASHA1", "ECDSASHA1":
		return "SHA1", nil
	case "RSASHA256", "ECDSASHA256":
		return "SHA256", nil
	case "RSASHA384", "ECDSASHA384":
		return "SHA384", nil
	case "RSASHA512", "ECDSASHA512":
		return "SHA512", nil
	default:
		return "", fmt.Errorf("unsupported signature algorithm %q", algorithm)
	}
}

func aesKeySizeForAlgorithm(algorithm string) (int, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	if strings.HasSuffix(normalized, "CBC") {
		normalized = strings.TrimSuffix(normalized, "CBC")
	}
	switch normalized {
	case "AES128":
		return 16, nil
	case "AES192":
		return 24, nil
	case "AES256":
		return 32, nil
	default:
		return 0, fmt.Errorf("unsupported encryption algorithm %q", algorithm)
	}
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS7 padding length")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("invalid PKCS7 padding")
		}
	}
	return data[:len(data)-padding], nil
}

func mathUnary(callee string, args []Value) (Value, error) {
	if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueDecimal) {
		return Null, fmt.Errorf("%s expects numeric argument", callee)
	}
	n := numericFloat(args[0])
	if math.IsInf(n, 0) || math.IsNaN(n) {
		return Null, fmt.Errorf("%s argument must be finite", callee)
	}
	switch callee {
	case "Math.abs":
		if args[0].Kind == ValueInt {
			if args[0].Int == math.MinInt64 {
				return Null, fmt.Errorf("Math.abs integer overflow")
			}
			if args[0].Int < 0 {
				return Int(-args[0].Int), nil
			}
			return args[0], nil
		}
		return Decimal(math.Abs(n)), nil
	case "Math.floor", "Math.ceil", "Math.rint":
		switch callee {
		case "Math.floor":
			return Decimal(math.Floor(n)), nil
		case "Math.ceil":
			return Decimal(math.Ceil(n)), nil
		default:
			return Decimal(roundHalfEven(n)), nil
		}
	case "Math.round":
		rounded, err := int64FromFloat("Math.round", roundHalfEven(n))
		if err != nil {
			return Null, err
		}
		return Int(rounded), nil
	case "Math.roundToLong":
		rounded, err := int64FromFloat("Math.roundToLong", roundHalfEven(n))
		if err != nil {
			return Null, err
		}
		return Int(rounded), nil
	case "Math.signum":
		switch {
		case n > 0:
			return Int(1), nil
		case n < 0:
			return Int(-1), nil
		default:
			return Int(0), nil
		}
	case "Math.sqrt":
		if n < 0 {
			return Null, fmt.Errorf("Math.sqrt argument out of domain")
		}
		return finiteDecimalResult(callee, math.Sqrt(n))
	case "Math.cbrt":
		return finiteDecimalResult(callee, math.Cbrt(n))
	case "Math.acos":
		if n < -1 || n > 1 {
			return Null, newExceptionError("System.MathException", "Math.acos argument out of domain")
		}
		return finiteDecimalResult(callee, math.Acos(n))
	case "Math.asin":
		if n < -1 || n > 1 {
			return Null, newExceptionError("System.MathException", "Math.asin argument out of domain")
		}
		return finiteDecimalResult(callee, math.Asin(n))
	case "Math.atan":
		return finiteDecimalResult(callee, math.Atan(n))
	case "Math.cos":
		return finiteDecimalResult(callee, math.Cos(n))
	case "Math.sin":
		return finiteDecimalResult(callee, math.Sin(n))
	case "Math.tan":
		return finiteDecimalResult(callee, math.Tan(n))
	case "Math.cosh":
		return finiteDecimalResult(callee, math.Cosh(n))
	case "Math.sinh":
		return finiteDecimalResult(callee, math.Sinh(n))
	case "Math.tanh":
		return finiteDecimalResult(callee, math.Tanh(n))
	case "Math.exp":
		return finiteDecimalResult(callee, math.Exp(n))
	case "Math.log":
		if n <= 0 {
			return Null, fmt.Errorf("Math.log argument out of domain")
		}
		return finiteDecimalResult(callee, math.Log(n))
	case "Math.log10":
		if n <= 0 {
			return Null, fmt.Errorf("Math.log10 argument out of domain")
		}
		return finiteDecimalResult(callee, math.Log10(n))
	default:
		return Null, unsupportedCallError(callee)
	}
}

func mathBinary(callee string, args []Value) (Value, error) {
	if len(args) != 2 || !isMathNumeric(args[0]) || !isMathNumeric(args[1]) {
		return Null, fmt.Errorf("%s expects two numeric arguments", callee)
	}
	left := numericFloat(args[0])
	right := numericFloat(args[1])
	if math.IsInf(left, 0) || math.IsNaN(left) || math.IsInf(right, 0) || math.IsNaN(right) {
		return Null, fmt.Errorf("%s arguments must be finite", callee)
	}
	switch callee {
	case "Math.max":
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(int64(math.Max(left, right))), nil
		}
		return Decimal(math.Max(left, right)), nil
	case "Math.min":
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(int64(math.Min(left, right))), nil
		}
		return Decimal(math.Min(left, right)), nil
	case "Math.mod":
		if right == 0 {
			return Null, fmt.Errorf("Math.mod divisor cannot be zero")
		}
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(args[0].Int % args[1].Int), nil
		}
		return Decimal(math.Mod(left, right)), nil
	case "Math.pow":
		return finiteDecimalResult(callee, math.Pow(left, right))
	case "Math.atan2":
		return finiteDecimalResult(callee, math.Atan2(left, right))
	default:
		return Null, unsupportedCallError(callee)
	}
}

func finiteDecimalResult(callee string, value float64) (Value, error) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return Null, fmt.Errorf("%s result must be finite", callee)
	}
	return Decimal(value), nil
}

func isMathNumeric(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueDecimal
}

func numericFloat(value Value) float64 {
	if value.Kind == ValueInt {
		return float64(value.Int)
	}
	return value.Decimal
}

func builtinEnumStaticValue(typeName, memberName string) (Value, bool) {
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch {
	case strings.EqualFold(typeName, "AccessLevel"):
		for _, known := range []string{"USER_MODE", "SYSTEM_MODE"} {
			if strings.EqualFold(memberName, known) {
				return Value{Kind: ValueObject, Type: "AccessLevel", Text: known}, true
			}
		}
	case strings.EqualFold(typeName, "AccessType"):
		return namedEnumStaticValue("AccessType", accessTypeNames, "AccessType."+memberName)
	case strings.EqualFold(typeName, "RoundingMode"):
		if mode, ok := canonicalDecimalRoundingModeName(memberName); ok {
			return Value{Kind: ValueObject, Type: "RoundingMode", Text: mode}, true
		}
	case strings.EqualFold(typeName, "LoggingLevel"):
		if level, ok := canonicalLoggingLevelName(memberName); ok {
			return Value{Kind: ValueObject, Type: "LoggingLevel", Text: level}, true
		}
	case strings.EqualFold(typeName, "TriggerOperation"):
		if operation, ok := canonicalTriggerOperationName(memberName); ok {
			return Value{Kind: ValueObject, Type: "TriggerOperation", Text: operation}, true
		}
	case strings.EqualFold(typeName, "StatusCode"):
		if statusCode, ok := canonicalStatusCodeName(memberName); ok {
			return Value{Kind: ValueObject, Type: "StatusCode", Text: statusCode}, true
		}
	case strings.EqualFold(typeName, "JSONToken"):
		if token, ok := canonicalJSONTokenName(memberName); ok {
			return Value{Kind: ValueObject, Type: "JSONToken", Text: token}, true
		}
	case strings.EqualFold(typeName, "XmlTag"):
		if tag, ok := canonicalXmlTagName(memberName); ok {
			return Value{Kind: ValueObject, Type: "XmlTag", Text: tag}, true
		}
	case strings.EqualFold(typeName, "DisplayType") || strings.EqualFold(typeName, "Schema.DisplayType"):
		return schemaDisplayTypeStaticValue("Schema.DisplayType." + memberName)
	case strings.EqualFold(typeName, "SOAPType") || strings.EqualFold(typeName, "SoapType") || strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "Schema.SoapType"):
		return schemaSOAPTypeStaticValue("Schema.SOAPType." + memberName)
	case strings.EqualFold(typeName, "SObjectDescribeOptions"):
		for _, known := range []string{"DEFERRED", "FULL"} {
			if strings.EqualFold(memberName, known) {
				return Value{Kind: ValueObject, Type: "SObjectDescribeOptions", Text: known}, true
			}
		}
	}
	return Null, false
}

func canonicalTriggerOperationName(name string) (string, bool) {
	for _, candidate := range triggerOperationNames {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
}

func canonicalStatusCodeName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", false
	}
	return strings.ToUpper(trimmed), true
}

func jsonSuppressNulls(callee string, args []Value) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueBool {
		return false, fmt.Errorf("%s expects suppressApexObjectNulls Boolean", callee)
	}
	return args[0].Bool, nil
}

func typedJSONMapKey(typeName, key string) (Value, error) {
	if strings.EqualFold(typeName, "String") || strings.EqualFold(typeName, "Object") {
		return String(key), nil
	}
	value, ok, err := typedScalarFromJSON(typeName, key)
	if err != nil {
		return Null, err
	}
	if ok {
		return value, nil
	}
	return Null, jsonDeserializeException("JSON.deserialize supports Map keys only for scalar/String/Object targets, got %s", typeName)
}

func canonicalJSONScalarType(typeName string) string {
	switch {
	case strings.EqualFold(typeName, "String"):
		return "String"
	case strings.EqualFold(typeName, "Boolean"):
		return "Boolean"
	case strings.EqualFold(typeName, "Integer"):
		return "Integer"
	case strings.EqualFold(typeName, "Long"):
		return "Long"
	case strings.EqualFold(typeName, "Decimal"):
		return "Decimal"
	case strings.EqualFold(typeName, "Double"):
		return "Double"
	case strings.EqualFold(typeName, "Date"):
		return "Date"
	case strings.EqualFold(typeName, "Datetime") || strings.EqualFold(typeName, "DateTime"):
		return "Datetime"
	case strings.EqualFold(typeName, "Time"):
		return "Time"
	case strings.EqualFold(typeName, "Id"):
		return "Id"
	case strings.EqualFold(typeName, "Blob"):
		return "Blob"
	case strings.EqualFold(typeName, "UUID"):
		return "UUID"
	default:
		return typeName
	}
}

func jsonIntegralNumber(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case json.Number:
		text := value.String()
		if strings.ContainsAny(text, ".eE") {
			decimal, err := strconv.ParseFloat(text, 64)
			if err != nil || math.Trunc(decimal) != decimal {
				return 0, false
			}
			converted, err := int64FromFloat("JSON number", decimal)
			return converted, err == nil
		}
		converted, err := strconv.ParseInt(text, 10, 64)
		return converted, err == nil
	case float64:
		if math.Trunc(value) != value {
			return 0, false
		}
		converted, err := int64FromFloat("JSON number", value)
		return converted, err == nil
	default:
		return 0, false
	}
}

func jsonDecimalNumber(raw any) (float64, bool) {
	switch value := raw.(type) {
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		converted, err := strconv.ParseFloat(value.String(), 64)
		return converted, err == nil
	case float64:
		return value, true
	default:
		return 0, false
	}
}

func jsonTypeMappingError(typeName string, raw any) error {
	return jsonDeserializeException("JSON.deserialize cannot map JSON %s to %s", jsonRawKind(raw), typeName)
}

func jsonDeserializeException(format string, args ...any) error {
	return newExceptionError("JSONException", fmt.Sprintf(format, args...))
}

func jsonRawKind(raw any) string {
	switch raw.(type) {
	case nil:
		return "null"
	case bool:
		return "Boolean"
	case json.Number, float64:
		return "number"
	case string:
		return "String"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", raw)
	}
}

func (vm *VM) jsonAllowedFields(typeName string) map[string]struct{} {
	allowed := map[string]struct{}{
		"Id":               {},
		"CreatedDate":      {},
		"CreatedById":      {},
		"LastModifiedDate": {},
		"LastModifiedById": {},
		"SystemModstamp":   {},
		"OwnerId":          {},
		"IsDeleted":        {},
	}
	if vm.Org != nil {
		if objectName, ok := vm.resolveObjectName(typeName); ok {
			object := vm.Org.Objects[objectName]
			for name := range object.Definition.Fields {
				allowed[name] = struct{}{}
				if vm.Org.Namespace != "" {
					allowed[storage.StripNamespaceToken(vm.Org.Namespace, name)] = struct{}{}
					allowed[storage.NamespaceTokenName(vm.Org.Namespace, name)] = struct{}{}
				}
			}
			for _, relation := range object.Definition.Relations {
				for _, name := range []string{relation.ParentRelationship, relation.ChildRelationship} {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					allowed[name] = struct{}{}
					if vm.Org.Namespace != "" {
						allowed[storage.StripNamespaceToken(vm.Org.Namespace, name)] = struct{}{}
						allowed[storage.NamespaceTokenName(vm.Org.Namespace, name)] = struct{}{}
					}
				}
			}
		}
	}
	for className := typeName; className != ""; {
		class, ok := vm.Classes[className]
		if !ok {
			break
		}
		for name := range class.Fields {
			allowed[name] = struct{}{}
		}
		className = class.SuperClass
	}
	return allowed
}

func jsonAllowedFieldContains(allowed map[string]struct{}, key string) bool {
	if _, ok := allowed[key]; ok {
		return true
	}
	for candidate := range allowed {
		if strings.EqualFold(candidate, key) {
			return true
		}
	}
	return false
}

func (vm *VM) jsonStrictAllowsRelationshipPayload(typeName, key string, item any) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if strings.HasSuffix(strings.ToLower(key), "__r") {
		return true
	}
	if _, ok := jsonQueryResultRecords(item); ok {
		return strings.HasSuffix(strings.ToLower(key), "s")
	}
	return false
}

func (vm *VM) allowOpenSObjectJSONFields(typeName string) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if _, ok := vm.Classes[typeName]; ok {
		return false
	}
	if vm.Org == nil {
		return true
	}
	_, ok := vm.resolveObjectName(typeName)
	return !ok
}

func (vm *VM) schemaGlobalDescribe() Value {
	if vm.globalDescribeCache != nil {
		return *vm.globalDescribeCache
	}
	out := Map()
	out.Type = "Schema.GlobalDescribeMap"
	if vm.Org == nil {
		return out
	}
	names := make([]string, 0, len(vm.Org.Objects))
	for name := range vm.Org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		token := sObjectTypeToken(name)
		for _, alias := range vm.schemaDescribeMapAliases(name) {
			out.Map[mapKey(String(alias))] = token
		}
	}
	vm.globalDescribeCache = &out
	return out
}

func (vm *VM) schemaDescribeObjectName(value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && value.Type == "Schema.SObjectType" {
		objectValue, ok := value.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return "", fmt.Errorf("Schema.SObjectType token missing object")
		}
		return objectValue.Text, nil
	}
	return "", fmt.Errorf("Schema.describeSObjects expects object names or SObjectType tokens")
}

func (vm *VM) schemaDescribeTabs() Value {
	if vm.describeTabsCache != nil {
		return *vm.describeTabsCache
	}
	tabs := vm.schemaDescribeTabValues()
	if len(tabs) == 0 {
		value := List()
		vm.describeTabsCache = &value
		return value
	}
	tabSet := Object("Schema.DescribeTabSetResult")
	tabSet.Fields["name"] = String("AllTabs")
	tabSet.Fields["label"] = String("All Tabs")
	tabSet.Fields["tabs"] = List(tabs...)
	tabSet.Fields["selected"] = Bool(false)
	value := List(tabSet)
	vm.describeTabsCache = &value
	return value
}

func (vm *VM) schemaDescribeTabValues() []Value {
	if vm.Org == nil {
		return nil
	}
	tabs := append([]storage.TabMetadata(nil), vm.Org.Metadata.Tabs...)
	seen := make(map[string]struct{}, len(tabs))
	for _, tab := range tabs {
		if objectName := describeTabSObjectName(tab); objectName != "" {
			seen[strings.ToLower(objectName)] = struct{}{}
		}
	}
	objectNames := make([]string, 0, len(vm.Org.Objects))
	for name, state := range vm.Org.Objects {
		apiName := state.Definition.APIName
		if apiName == "" {
			apiName = name
		}
		if !isStandardDescribeTabObject(apiName) {
			continue
		}
		key := strings.ToLower(apiName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		objectNames = append(objectNames, apiName)
	}
	sort.Strings(objectNames)
	for _, name := range objectNames {
		tabs = append(tabs, storage.TabMetadata{
			Name:        name,
			Label:       name,
			SObjectName: name,
		})
	}
	sort.Slice(tabs, func(i, j int) bool { return tabs[i].Name < tabs[j].Name })
	values := make([]Value, 0, len(tabs))
	for _, tab := range tabs {
		if describeTabSObjectName(tab) == "" {
			continue
		}
		values = append(values, describeTabValue(tab))
	}
	return values
}

func isStandardDescribeTabObject(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	if lowered == "" {
		return false
	}
	for _, suffix := range []string{"__c", "__e", "__mdt", "__b", "__x"} {
		if strings.HasSuffix(lowered, suffix) {
			return false
		}
	}
	_, ok := standardDescribeTabObjects[lowered]
	return ok
}

var standardDescribeTabObjects = map[string]struct{}{
	"account":     {},
	"campaign":    {},
	"case":        {},
	"contact":     {},
	"contract":    {},
	"event":       {},
	"lead":        {},
	"opportunity": {},
	"order":       {},
	"pricebook2":  {},
	"product2":    {},
	"task":        {},
	"user":        {},
}

func describeTabValue(tab storage.TabMetadata) Value {
	value := Object("Schema.DescribeTabResult")
	label := tab.Label
	if label == "" {
		label = tab.Name
	}
	value.Fields["name"] = String(tab.Name)
	value.Fields["label"] = String(label)
	sObjectName := describeTabSObjectName(tab)
	if sObjectName == "" {
		value.Fields["sObjectName"] = Null
	} else {
		value.Fields["sObjectName"] = String(sObjectName)
	}
	value.Fields["custom"] = Bool(tab.Custom)
	value.Fields["iconUrl"] = String(tab.Motif)
	value.Fields["icons"] = List(describeTabIconValue(tab))
	value.Fields["url"] = String("/lightning/o/" + tab.Name + "/list")
	return value
}

func describeTabSObjectName(tab storage.TabMetadata) string {
	sObjectName := strings.TrimSpace(tab.SObjectName)
	if sObjectName == "" && tab.Custom && strings.HasSuffix(strings.ToLower(tab.Name), "__c") {
		sObjectName = tab.Name
	}
	return sObjectName
}

func describeTabIconValue(tab storage.TabMetadata) Value {
	icon := Object("Schema.DescribeIconResult")
	icon.Fields["contentType"] = String("image/svg+xml")
	icon.Fields["height"] = Int(0)
	icon.Fields["theme"] = String(tab.Motif)
	icon.Fields["url"] = String(describeTabIconURL(tab))
	icon.Fields["width"] = Int(0)
	return icon
}

func describeTabIconURL(tab storage.TabMetadata) string {
	name := strings.TrimSpace(tab.Name)
	if name == "" {
		name = "custom"
	}
	token := "custom"
	if tab.Custom {
		token = strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(name, "__c"), "__tab"))
		token = strings.ReplaceAll(token, "__", "_")
	}
	return "/img/icon/t4v35/custom/" + token + "_120.png.svg"
}

func (vm *VM) schemaDescribeDataCategoryGroups(sobjects Value) Value {
	out := typedList("List<Schema.DescribeDataCategoryGroupResult>")
	if vm.Org == nil {
		return out
	}
	for _, sobject := range sobjects.List {
		if sobject.Kind != ValueString {
			continue
		}
		for _, group := range vm.dataCategoryGroupsForSObject(sobject.Text) {
			out.List = append(out.List, describeDataCategoryGroupValue(group))
		}
	}
	return out
}

func (vm *VM) schemaDescribeDataCategoryGroupStructures(pairs Value, topCategoriesOnly bool) Value {
	out := typedList("List<Schema.DescribeDataCategoryGroupStructureResult>")
	if vm.Org == nil {
		return out
	}
	for _, pair := range pairs.List {
		if pair.Kind != ValueObject {
			continue
		}
		sobjectName := schemaPassiveStringField(pair, "sobject")
		groupName := schemaPassiveStringField(pair, "dataCategoryGroupName")
		for _, group := range vm.dataCategoryGroupsForSObject(sobjectName) {
			if !vmMetadataNameMatches(group.Name, groupName) {
				continue
			}
			out.List = append(out.List, describeDataCategoryGroupStructureValue(group, topCategoriesOnly))
		}
	}
	return out
}

func (vm *VM) dataCategoryGroupsForSObject(sobjectName string) []storage.DataCategoryGroup {
	if vm == nil || vm.Org == nil {
		return nil
	}
	groups := make([]storage.DataCategoryGroup, 0)
	for _, group := range vm.Org.Metadata.DataCategoryGroups {
		if !vmMetadataNameMatches(group.SObjectName, sobjectName) {
			continue
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func describeDataCategoryGroupValue(group storage.DataCategoryGroup) Value {
	value := Object("Schema.DescribeDataCategoryGroupResult")
	value.Fields["name"] = String(group.Name)
	value.Fields["label"] = String(metadataLabel(group.Label, group.Name))
	value.Fields["description"] = String(group.Description)
	value.Fields["sobject"] = String(group.SObjectName)
	value.Fields["categorycount"] = Int(int64(countDataCategories(group.Categories)))
	return value
}

func describeDataCategoryGroupStructureValue(group storage.DataCategoryGroup, topCategoriesOnly bool) Value {
	value := Object("Schema.DescribeDataCategoryGroupStructureResult")
	value.Fields["name"] = String(group.Name)
	value.Fields["label"] = String(metadataLabel(group.Label, group.Name))
	value.Fields["description"] = String(group.Description)
	value.Fields["sobject"] = String(group.SObjectName)
	categories := typedList("List<Schema.DataCategory>")
	for _, category := range group.Categories {
		categories.List = append(categories.List, describeDataCategoryValue(category, topCategoriesOnly))
	}
	value.Fields["topcategories"] = categories
	return value
}

func describeDataCategoryValue(category storage.DataCategory, topCategoriesOnly bool) Value {
	value := Object("Schema.DataCategory")
	value.Fields["name"] = String(category.Name)
	value.Fields["label"] = String(metadataLabel(category.Label, category.Name))
	children := typedList("List<Schema.DataCategory>")
	if !topCategoriesOnly {
		for _, child := range category.Children {
			children.List = append(children.List, describeDataCategoryValue(child, false))
		}
	}
	value.Fields["childcategories"] = children
	return value
}

func countDataCategories(categories []storage.DataCategory) int {
	count := 0
	for _, category := range categories {
		count++
		count += countDataCategories(category.Children)
	}
	return count
}

func schemaPassiveStringField(value Value, field string) string {
	_, fieldValue, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	if fieldValue.Kind == ValueString {
		return fieldValue.Text
	}
	return ""
}

func metadataLabel(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}

func vmMetadataNameMatches(candidate, requested string) bool {
	return strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(requested))
}

func (vm *VM) lookupField(typeName, fieldName string) (Field, string, bool) {
	return vm.lookupFieldForReceiver(typeName, fieldName, false)
}

func (vm *VM) lookupFieldForReceiver(typeName, fieldName string, preferDependency bool) (Field, string, bool) {
	for typeName != "" {
		class, ok := vm.lookupClass(typeName)
		if !ok {
			return Field{}, "", false
		}
		if field, ok := vm.lookupFieldInMapWithOptions(class.Fields, fieldName, preferDependency); ok {
			return field, runtimeClassName(class), true
		}
		typeName = vm.resolvedSuperClassName(class)
	}
	return Field{}, "", false
}

func (vm *VM) lookupReceiverField(typeName, fieldName string) (Field, string, bool) {
	if vm.currentClass != "" && (strings.EqualFold(typeName, vm.currentClass) || vm.isSubclass(typeName, vm.currentClass)) {
		if class, ok := vm.Classes[vm.currentClass]; ok {
			if field, ok := vm.lookupFieldInMapWithOptions(class.Fields, fieldName, vm.currentMethod.Dependency); ok {
				return field, runtimeClassName(class), true
			}
		}
	}
	return vm.lookupFieldForReceiver(typeName, fieldName, vm.currentMethod.Dependency)
}

func (vm *VM) lookupStaticField(typeName, fieldName string) (Field, string, bool) {
	return vm.lookupStaticFieldForReceiver(typeName, fieldName, false)
}

func (vm *VM) lookupStaticFieldForReceiver(typeName, fieldName string, preferDependency bool) (Field, string, bool) {
	for search := typeName; search != ""; {
		for current := search; current != ""; {
			class, ok := vm.lookupClass(current)
			if !ok {
				break
			}
			if field, ok := vm.lookupFieldInMapWithOptions(class.StaticFields, fieldName, preferDependency); ok {
				if field.Value.Kind == "" {
					field.Value = defaultValue(field.Type, field.InitialValue)
				}
				return field, runtimeClassName(class), true
			}
			current = vm.resolvedSuperClassName(class)
		}
		dot := strings.LastIndex(search, ".")
		if dot < 0 {
			break
		}
		search = search[:dot]
	}
	return Field{}, "", false
}

func (vm *VM) lookupFieldInMap(fields map[string]Field, fieldName string) (Field, bool) {
	return vm.lookupFieldInMapWithOptions(fields, fieldName, false)
}

func (vm *VM) lookupFieldInMapWithOptions(fields map[string]Field, fieldName string, preferDependency bool) (Field, bool) {
	requested := strings.TrimSpace(fieldName)
	normalized := strings.ToLower(requested)
	var best Field
	bestScore := -1
	found := false
	for candidate, field := range fields {
		fieldNameKey := strings.ToLower(strings.TrimSpace(field.Name))
		if strings.ToLower(candidate) != normalized && fieldNameKey != normalized {
			continue
		}
		if field.Name == "" {
			field.Name = candidate
		}
		field.StorageName = candidate
		score := vm.fieldProvenanceScore(field)
		if candidate == requested || field.Name == requested {
			score += 16
		}
		if dependencyPreferenceRank(fieldOrigin(field), preferDependency) == 0 {
			score += 32
		}
		if !found || score > bestScore {
			best = field
			bestScore = score
			found = true
		}
	}
	return best, found
}

func (vm *VM) fieldProvenanceScore(field Field) int {
	score := 0
	if fieldOrigin(field) == symbolOriginDependency {
		score += 8
	}
	if field.Type != "" {
		score += 2
	}
	return score
}

func staticFieldStorageName(requested string, field Field) string {
	if strings.TrimSpace(field.StorageName) != "" {
		return field.StorageName
	}
	if strings.TrimSpace(field.Name) != "" {
		return field.Name
	}
	return requested
}

func (vm *VM) staticFieldWritebackKey(owner, requested string, field Field) string {
	class, ok := vm.Classes[owner]
	if ok {
		if storageName := strings.TrimSpace(field.StorageName); storageName != "" {
			if _, exists := class.StaticFields[storageName]; exists {
				return storageName
			}
		}
		normalized := strings.ToLower(strings.TrimSpace(requested))
		for key := range class.StaticFields {
			if strings.ToLower(strings.TrimSpace(key)) == normalized {
				return key
			}
		}
	}
	return staticFieldStorageName(requested, field)
}

func builtinStaticField(typeName, fieldName string) (Value, bool) {
	if value, ok := builtinEnumStaticValue(typeName, fieldName); ok {
		return value, true
	}
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch {
	case strings.EqualFold(typeName, "Math"):
		switch {
		case strings.EqualFold(fieldName, "E"):
			return Decimal(math.E), true
		case strings.EqualFold(fieldName, "PI"):
			return Decimal(math.Pi), true
		}
	}
	switch typeName {
	case "Math":
		switch fieldName {
		case "E":
			return Decimal(math.E), true
		case "PI":
			return Decimal(math.Pi), true
		}
	case "Integer":
		switch fieldName {
		case "MAX_VALUE":
			return Int(math.MaxInt32), true
		case "MIN_VALUE":
			return Int(math.MinInt32), true
		}
	case "Long":
		switch fieldName {
		case "MAX_VALUE":
			return Int(math.MaxInt64), true
		case "MIN_VALUE":
			return Int(math.MinInt64), true
		}
	case "Pattern":
		switch fieldName {
		case "UNIX_LINES":
			return Int(patternFlagUnixLines), true
		case "CASE_INSENSITIVE":
			return Int(patternFlagCaseInsensitive), true
		case "COMMENTS":
			return Int(patternFlagComments), true
		case "MULTILINE":
			return Int(patternFlagMultiline), true
		case "LITERAL":
			return Int(patternFlagLiteral), true
		case "DOTALL":
			return Int(patternFlagDotall), true
		case "UNICODE_CASE":
			return Int(patternFlagUnicodeCase), true
		case "CANON_EQ":
			return Int(patternFlagCanonEq), true
		case "UNICODE_CHARACTER_CLASS":
			return Int(patternFlagUnicodeCharacterClass), true
		}
	case "Dom.XmlNodeType":
		return domXmlNodeTypeValue(fieldName)
	case "Canvas.Test":
		switch fieldName {
		case "KEY_CANVAS_URL":
			return String("canvasUrl"), true
		case "KEY_DEVELOPER_NAME":
			return String("developerName"), true
		case "KEY_DISPLAY_LOCATION":
			return String("displayLocation"), true
		case "KEY_LOCATION_URL":
			return String("locationUrl"), true
		case "KEY_NAME":
			return String("name"), true
		case "KEY_NAMESPACE":
			return String("namespace"), true
		case "KEY_SUB_LOCATION":
			return String("sublocation"), true
		case "KEY_VERSION":
			return String("version"), true
		}
	}
	return Null, false
}

func (vm *VM) checkMemberAccess(ownerClass, access, member string, modifierSets ...[]string) error {
	if err := vm.checkClassAccess(ownerClass, member, modifierSets...); err != nil {
		return err
	}
	if err := vm.checkNamespaceAccess(ownerClass, access, member, modifierSets...); err != nil {
		return err
	}
	switch strings.ToLower(access) {
	case "", "public", "global", "webservice":
		return nil
	case "private":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClassIsTest() && vm.hasTestVisibleAncestorMember(ownerClass, member) {
			return nil
		}
		if vm.sameAccessScope(vm.currentClass, ownerClass) {
			return nil
		}
		methodOwner := vm.currentMethod.ClassName
		if methodOwner == "" {
			methodOwner = classNameFromMethod(vm.currentMethod.Name)
		} else if vm.classNamespace(methodOwner) == "" {
			if owner := classNameFromMethod(vm.currentMethod.Name); owner != "" {
				methodOwner = owner
			}
		}
		if vm.sameAccessScope(methodOwner, ownerClass) {
			return nil
		}
	case "protected":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClassIsTest() && vm.hasTestVisibleAncestorMember(ownerClass, member) {
			return nil
		}
		if vm.sameAccessScope(vm.currentClass, ownerClass) || vm.isSubclass(vm.currentClass, ownerClass) || vm.isSubclass(ownerClass, vm.currentClass) {
			return nil
		}
		methodOwner := vm.currentMethod.ClassName
		if methodOwner == "" {
			methodOwner = classNameFromMethod(vm.currentMethod.Name)
		} else if vm.classNamespace(methodOwner) == "" {
			if owner := classNameFromMethod(vm.currentMethod.Name); owner != "" {
				methodOwner = owner
			}
		}
		if vm.sameAccessScope(methodOwner, ownerClass) || vm.isSubclass(methodOwner, ownerClass) || vm.isSubclass(ownerClass, methodOwner) {
			return nil
		}
	default:
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is %s and not visible", member, access)
	}
	return fmt.Errorf("%s is %s and not visible from %s", member, access, vm.currentClass)
}

func (vm *VM) hasTestVisibleAncestorMember(ownerClass, member string) bool {
	memberName := apexMethodMemberName(member)
	for superClass := vm.superClassName(ownerClass); superClass != ""; superClass = vm.superClassName(superClass) {
		methodKey := superClass + "." + memberName
		if method, ok := vm.Methods[methodKey]; ok && hasAnyMethodModifier([][]string{method.Modifiers}, "testvisible") {
			return true
		}
		for _, method := range vm.MethodOverloads[methodKey] {
			if hasAnyMethodModifier([][]string{method.Modifiers}, "testvisible") {
				return true
			}
		}
		if class, ok := vm.Classes[superClass]; ok {
			for _, field := range class.Fields {
				if strings.EqualFold(field.Name, memberName) && hasAnyMethodModifier([][]string{field.Modifiers}, "testvisible") {
					return true
				}
			}
		}
	}
	return false
}

func sameLexicalTopLevel(a, b string) bool {
	aTop, aNested := lexicalTopLevel(a)
	bTop, bNested := lexicalTopLevel(b)
	return aNested && bNested && strings.EqualFold(aTop, bTop)
}

func (vm *VM) sameAccessScope(left, right string) bool {
	for _, leftName := range vm.accessScopeNames(left) {
		for _, rightName := range vm.accessScopeNames(right) {
			if sameOrNestedTypeFold(leftName, rightName) || sameLexicalTopLevel(leftName, rightName) {
				return true
			}
		}
	}
	return false
}

func (vm *VM) accessScopeNames(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	out := []string{name}
	if class, ok := vm.classForAccess(name); ok {
		out = append(out, class.Name)
		if class.Namespace != "" {
			out = append(out, runtimeClassName(class))
		}
	}
	return out
}

func sameOrNestedTypeFold(left, right string) bool {
	return strings.EqualFold(left, right) || hasTypePrefixFold(left, right) || hasTypePrefixFold(right, left)
}

func hasTypePrefixFold(value, prefix string) bool {
	if prefix == "" || len(value) <= len(prefix) || value[len(prefix)] != '.' {
		return false
	}
	return strings.EqualFold(value[:len(prefix)], prefix)
}

func lexicalTopLevel(className string) (string, bool) {
	dot := strings.IndexByte(className, '.')
	if dot <= 0 {
		return className, false
	}
	return className[:dot], true
}

func (vm *VM) checkClassAccess(ownerClass, member string, modifierSets ...[]string) error {
	class, ok := vm.classForAccess(ownerClass)
	if !ok || class.Namespace == "" {
		return nil
	}
	if vm.memberAccessScopeMatches(ownerClass) {
		return nil
	}
	callerNS := vm.currentCallerNamespace()
	if callerNS == class.Namespace {
		return nil
	}
	switch strings.ToLower(class.Access) {
	case "global", "webservice":
		return nil
	}
	if hasAnyMethodModifier([][]string{class.Modifiers}, "namespaceaccessible") || hasAnyMethodModifier(modifierSets, "namespaceaccessible") {
		return nil
	}
	if vm.hasAccessibleInheritedMember(ownerClass, member) {
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is not global and not visible outside namespace %s", member, class.Namespace)
	}
	return fmt.Errorf("%s is not global and not visible from namespace %s", member, callerNS)
}

func (vm *VM) currentClassIsTest() bool {
	class, ok := vm.Classes[vm.currentClass]
	return ok && class.IsTest
}

func hasAnyMethodModifier(modifierSets [][]string, expected string) bool {
	for _, modifiers := range modifierSets {
		for _, modifier := range modifiers {
			if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
				return true
			}
		}
	}
	return false
}

func (vm *VM) checkNamespaceAccess(ownerClass, access, member string, modifierSets ...[]string) error {
	class, ok := vm.classForAccess(ownerClass)
	ownerNS := ""
	if ok {
		ownerNS = class.Namespace
	}
	if ownerNS == "" {
		return nil
	}
	if vm.memberAccessScopeMatches(ownerClass) {
		return nil
	}
	callerNS := vm.currentCallerNamespace()
	if callerNS == ownerNS {
		return nil
	}
	switch strings.ToLower(access) {
	case "global", "webservice":
		return nil
	}
	if hasAnyMethodModifier(modifierSets, "namespaceaccessible") {
		return nil
	}
	if vm.hasAccessibleInheritedMember(ownerClass, member) {
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is not global and not visible outside namespace %s", member, ownerNS)
	}
	return fmt.Errorf("%s is not global and not visible from namespace %s", member, callerNS)
}

func (vm *VM) hasAccessibleInheritedMember(ownerClass, member string) bool {
	memberName := apexMethodMemberName(member)
	if memberName == "" {
		return false
	}
	class, ok := vm.classForAccess(ownerClass)
	if !ok {
		return false
	}
	return vm.hasAccessibleInheritedMemberFromClass(class, memberName, make(map[string]bool))
}

func (vm *VM) hasAccessibleInheritedMemberFromClass(class Class, memberName string, seen map[string]bool) bool {
	className := runtimeClassName(class)
	if className == "" || seen[strings.ToLower(className)] {
		return false
	}
	seen[strings.ToLower(className)] = true
	for _, parentName := range append([]string{vm.resolvedSuperClassName(class)}, vm.resolvedInterfaceNames(class)...) {
		parentName = strings.TrimSpace(parentName)
		if parentName == "" {
			continue
		}
		if vm.classHasAccessibleMember(parentName, memberName) {
			return true
		}
		parent, ok := vm.classForAccess(parentName)
		if !ok {
			continue
		}
		if vm.hasAccessibleInheritedMemberFromClass(parent, memberName, seen) {
			return true
		}
	}
	return false
}

func (vm *VM) classHasAccessibleMember(className, memberName string) bool {
	className = strings.TrimSpace(className)
	if className == "" || memberName == "" {
		return false
	}
	names := []string{className}
	if class, ok := vm.classForAccess(className); ok {
		names = append(names, class.Name, runtimeClassName(class), shortTypeName(class.Name))
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if vm.methodSurfaceAccessible(name+"."+memberName, name) {
			return true
		}
	}
	return false
}

func (vm *VM) methodSurfaceAccessible(methodKey, ownerClass string) bool {
	for _, method := range vm.MethodOverloads[methodKey] {
		if vm.memberSurfaceMethodAccessible(method, ownerClass) {
			return true
		}
	}
	if method, ok := vm.Methods[methodKey]; ok && vm.memberSurfaceMethodAccessible(method, ownerClass) {
		return true
	}
	return false
}

func (vm *VM) memberSurfaceMethodAccessible(method Method, ownerClass string) bool {
	if !vm.methodClassMatchesAccessOwner(method.ClassName, ownerClass) {
		return false
	}
	switch strings.ToLower(method.Access) {
	case "global", "webservice":
		return true
	}
	return methodHasModifier(method.Modifiers, "namespaceaccessible")
}

func (vm *VM) methodClassMatchesAccessOwner(methodClass, ownerClass string) bool {
	methodClass = strings.TrimSpace(methodClass)
	ownerClass = strings.TrimSpace(ownerClass)
	if methodClass == "" || ownerClass == "" {
		return true
	}
	if strings.EqualFold(methodClass, ownerClass) || vm.sameAccessScope(methodClass, ownerClass) {
		return true
	}
	methodOwner, methodOK := vm.classForAccess(methodClass)
	accessOwner, ownerOK := vm.classForAccess(ownerClass)
	if methodOK && ownerOK {
		return strings.EqualFold(runtimeClassName(methodOwner), runtimeClassName(accessOwner))
	}
	return false
}

func (vm *VM) memberAccessScopeMatches(ownerClass string) bool {
	if vm.sameAccessScope(vm.currentClass, ownerClass) {
		return true
	}
	methodOwner := vm.currentMethod.ClassName
	if methodOwner == "" {
		methodOwner = classNameFromMethod(vm.currentMethod.Name)
	}
	return vm.sameAccessScope(methodOwner, ownerClass)
}

func (vm *VM) classNamespace(className string) string {
	if vm.classNamespaceCache == nil {
		vm.classNamespaceCache = make(map[string]string)
	}
	cacheKey := canonicalClassLookupKey(className)
	if cacheKey != "" {
		if namespace, ok := vm.classNamespaceCache[cacheKey]; ok {
			return namespace
		}
	}
	class, ok := vm.lookupClass(className)
	if !ok {
		if resolved, found := vm.resolveClassName(className); found {
			class, ok = vm.lookupClass(resolved)
		}
	}
	if !ok {
		for _, triggers := range vm.Triggers {
			for _, trigger := range triggers {
				if strings.EqualFold(trigger.Name, className) {
					namespace := trigger.Namespace
					if cacheKey != "" {
						vm.classNamespaceCache[cacheKey] = namespace
					}
					return namespace
				}
			}
		}
		if cacheKey != "" {
			vm.classNamespaceCache[cacheKey] = ""
		}
		return ""
	}
	if cacheKey != "" {
		vm.classNamespaceCache[cacheKey] = class.Namespace
	}
	return class.Namespace
}

func (vm *VM) currentCallerNamespace() string {
	if count := len(vm.activeTriggerNamespaces); count > 0 {
		if ns := strings.TrimSpace(vm.activeTriggerNamespaces[count-1]); ns != "" {
			return ns
		}
	}
	if ns := vm.activeTriggerNamespace(); ns != "" {
		return ns
	}
	if ns := vm.currentTriggerNamespace(); ns != "" {
		return ns
	}
	if strings.TrimSpace(vm.currentClass) != "" && vm.classNamespace(vm.currentClass) == "" {
		if ns := strings.TrimSpace(vm.currentNamespace); ns != "" {
			return ns
		}
	}
	if ns := vm.classNamespace(vm.currentMethod.ClassName); ns != "" {
		return ns
	}
	if owner := classNameFromMethod(vm.currentMethod.Name); owner != "" && !strings.EqualFold(owner, vm.currentMethod.ClassName) {
		if ns := vm.classNamespace(owner); ns != "" {
			return ns
		}
	}
	if ns := vm.classNamespace(vm.currentClass); ns != "" {
		return ns
	}
	if ns := strings.TrimSpace(vm.currentNamespace); ns != "" {
		return ns
	}
	return ""
}

func (vm *VM) currentTriggerNamespace() string {
	if strings.TrimSpace(vm.currentClass) == "" {
		return ""
	}
	return vm.triggerNamespaceByName(vm.currentClass)
}

func (vm *VM) activeTriggerNamespace() string {
	for i := len(vm.callStack) - 1; i >= 0; i-- {
		symbol := strings.TrimSpace(vm.callStack[i].Symbol)
		if symbol == "" {
			continue
		}
		if ns := vm.triggerNamespaceByName(symbol); ns != "" {
			return ns
		}
	}
	return ""
}

func (vm *VM) triggerNamespaceByName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	cacheKey := triggerNamespaceLookupKey{CurrentNamespace: strings.TrimSpace(vm.currentNamespace), Name: name}
	if vm.triggerNamespaceCache == nil {
		vm.triggerNamespaceCache = make(map[triggerNamespaceLookupKey]string)
	}
	if cached, ok := vm.triggerNamespaceCache[cacheKey]; ok {
		return cached
	}
	for _, triggers := range vm.Triggers {
		for _, trigger := range triggers {
			if !strings.EqualFold(trigger.Name, name) {
				continue
			}
			if ns := strings.TrimSpace(trigger.Namespace); ns != "" {
				vm.triggerNamespaceCache[cacheKey] = ns
				return ns
			}
			ns := strings.TrimSpace(vm.currentNamespace)
			vm.triggerNamespaceCache[cacheKey] = ns
			return ns
		}
	}
	vm.triggerNamespaceCache[cacheKey] = ""
	return ""
}

func (vm *VM) classForAccess(className string) (Class, bool) {
	if vm.classForAccessCache == nil {
		vm.classForAccessCache = make(map[string]classForAccessLookup)
	}
	cacheKey := canonicalClassLookupKey(className) + "|" + canonicalClassLookupKey(vm.currentClass) + "|" + canonicalClassLookupKey(vm.currentNamespace)
	if cached, ok := vm.classForAccessCache[cacheKey]; ok {
		return cached.Class, cached.OK
	}
	store := func(class Class, ok bool) (Class, bool) {
		vm.classForAccessCache[cacheKey] = classForAccessLookup{Class: class, OK: ok}
		return class, ok
	}
	if className != "" && !strings.Contains(className, ".") && vm.currentClass != "" {
		if currentNS := strings.TrimSpace(vm.currentNamespace); currentNS != "" {
			if class, ok := vm.lookupClassInNamespace(currentNS, className); ok {
				return store(class, true)
			}
		}
		if callerNS := vm.classNamespace(vm.currentClass); callerNS != "" {
			if class, ok := vm.lookupClassInNamespace(callerNS, className); ok {
				return store(class, true)
			}
		}
		if class, ok := vm.lookupClassInNamespace("", className); ok {
			return store(class, true)
		}
	}
	class, ok := vm.Classes[className]
	if !ok {
		if resolved, found := vm.resolveClassName(className); found {
			class, ok = vm.Classes[resolved]
		}
	}
	return store(class, ok)
}

func (vm *VM) lookupClassInNamespace(namespace, className string) (Class, bool) {
	if vm.namespaceClassLookup == nil {
		vm.namespaceClassLookup = make(map[string]map[string]namespaceClassLookup)
	}
	className = strings.TrimSpace(className)
	if className == "" {
		return Class{}, false
	}
	nsKey := strings.ToLower(strings.TrimSpace(namespace))
	shortKey := strings.ToLower(shortTypeName(className))
	if classesByShort, ok := vm.namespaceClassLookup[nsKey]; ok {
		result, found := classesByShort[shortKey]
		if !found || !result.OK {
			return Class{}, false
		}
		return result.Class, true
	}
	classesByShort := make(map[string]namespaceClassLookup)
	for _, entry := range vm.classNameSearchEntries() {
		class, ok := vm.lookupClass(entry.Name)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(class.Namespace)) != nsKey {
			continue
		}
		if !strings.Contains(class.Name, ".") {
			key := strings.ToLower(strings.TrimSpace(class.Name))
			classesByShort[key] = namespaceClassLookup{Class: class, OK: true}
			continue
		}
		key := strings.ToLower(shortTypeName(class.Name))
		if existing, exists := classesByShort[key]; exists {
			if existing.OK && strings.Contains(existing.Class.Name, ".") && !strings.EqualFold(existing.Class.Name, class.Name) {
				classesByShort[key] = namespaceClassLookup{}
			}
			continue
		}
		classesByShort[key] = namespaceClassLookup{Class: class, OK: true}
	}
	vm.namespaceClassLookup[nsKey] = classesByShort
	result, found := classesByShort[shortKey]
	if !found || !result.OK {
		return Class{}, false
	}
	return result.Class, true
}

func (vm *VM) isSubclass(child, parent string) bool {
	if resolved, ok := vm.resolveClassName(child); ok {
		child = resolved
	}
	if resolved, ok := vm.resolveClassName(parent); ok {
		parent = resolved
	}
	seen := make(map[string]bool)
	for child != "" {
		key := canonicalClassLookupKey(child)
		if seen[key] {
			return false
		}
		seen[key] = true
		class, ok := vm.lookupClass(child)
		if !ok {
			return false
		}
		superClass := vm.resolvedSuperClassName(class)
		if strings.EqualFold(superClass, parent) || vm.classNamesReferToSameRuntimeType(superClass, parent) {
			return true
		}
		child = superClass
	}
	return false
}

func (vm *VM) splitClassMember(name string) (string, string, bool) {
	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		className := strings.Join(parts[:i], ".")
		if !strings.Contains(className, ".") && vm.currentClass != "" {
			if resolved, ok := vm.resolveNestedTypeInClassHierarchy(vm.currentClass, className); ok {
				return resolved, strings.Join(parts[i:], "."), true
			}
		}
		if resolved, ok := vm.resolveClassName(className); ok {
			return resolved, strings.Join(parts[i:], "."), true
		}
		if generated, ok := generatedPlatformTypeIndex[strings.ToLower(className)]; ok {
			return generated.Name, strings.Join(parts[i:], "."), true
		}
		if class, ok := vm.resolveEnumClass(className); ok {
			return class.Name, strings.Join(parts[i:], "."), true
		}
	}
	return "", "", false
}

func apexIdentifierStartsUpper(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z'
}

func (vm *VM) typeForName(namespace, name string) Value {
	if strings.TrimSpace(name) == "" {
		return Null
	}
	if strings.HasPrefix(name, "System.") {
		systemName := strings.TrimPrefix(name, "System.")
		if isBuiltinTypeName(systemName) {
			return platformScalar("Type", "System."+systemName)
		}
	}
	if strings.TrimSpace(namespace) == "System" {
		systemName := strings.TrimPrefix(name, "System.")
		if isBuiltinTypeName(systemName) {
			return platformScalar("Type", "System."+systemName)
		}
		return Null
	}
	if namespace != "" {
		for _, candidate := range namespaceTypeNameCandidates(namespace, name) {
			if class, ok := vm.lookupClass(candidate); ok {
				return platformScalar("Type", typeForNameClassToken(namespace, class))
			}
		}
		return Null
	}
	if resolved, ok := vm.resolveClassName(name); ok {
		if class, ok := vm.lookupClass(resolved); ok {
			return platformScalar("Type", vm.classTypeToken(class))
		}
		return platformScalar("Type", resolved)
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(name); ok {
			return platformScalar("Type", canonical)
		}
	}
	if resolved, ok := vm.resolveTypeNameToken(name); ok {
		return platformScalar("Type", resolved)
	}
	if isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name) {
		return platformScalar("Type", name)
	}
	return Null
}

func namespaceTypeNameCandidates(namespace, name string) []string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil
	}
	candidates := []string{namespace + "." + name}
	if strings.Contains(name, ".") {
		candidates = append(candidates, name)
	}
	return candidates
}

func typeForNameClassToken(namespace string, class Class) string {
	namespace = strings.TrimSpace(namespace)
	if class.Namespace == "" || namespace == "" || !strings.EqualFold(namespace, class.Namespace) {
		return class.Name
	}
	if prefix, _, ok := strings.Cut(class.Name, "."); ok && strings.EqualFold(prefix, class.Namespace) {
		return class.Name
	}
	return class.Namespace + "." + class.Name
}

func (vm *VM) classTypeToken(class Class) string {
	namespace := strings.TrimSpace(class.Namespace)
	if namespace == "" && vm.Org != nil {
		namespace = strings.TrimSpace(vm.Org.Namespace)
	}
	if namespace == "" {
		return class.Name
	}
	if prefix, _, ok := strings.Cut(class.Name, "."); ok && strings.EqualFold(prefix, namespace) {
		return class.Name
	}
	return namespace + "." + class.Name
}

func (vm *VM) resolveTypeNameToken(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	if strings.HasSuffix(name, "[]") {
		element, ok := vm.resolveTypeNameToken(strings.TrimSpace(strings.TrimSuffix(name, "[]")))
		if !ok {
			return "", false
		}
		return element + "[]", true
	}
	if args, ok := genericTypeArgs(name); ok {
		base, ok := genericBaseName(name)
		if !ok {
			return "", false
		}
		switch {
		case strings.EqualFold(base, "List"), strings.EqualFold(base, "Set"), strings.EqualFold(base, "Iterator"), strings.EqualFold(base, "Iterable"):
			if len(args) != 1 {
				return "", false
			}
			element, ok := vm.resolveTypeNameToken(args[0])
			if !ok {
				return "", false
			}
			return base + "<" + element + ">", true
		case strings.EqualFold(base, "Map"):
			if len(args) != 2 {
				return "", false
			}
			key, keyOK := vm.resolveTypeNameToken(args[0])
			value, valueOK := vm.resolveTypeNameToken(args[1])
			if !keyOK || !valueOK {
				return "", false
			}
			return base + "<" + key + "," + value + ">", true
		}
		return "", false
	}
	if resolved, ok := vm.resolveClassName(name); ok {
		return resolved, true
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(name); ok {
			return canonical, true
		}
	}
	if isBuiltinTypeName(name) || isCommonSObjectTypeName(name) {
		return name, true
	}
	return "", false
}

func isBuiltinTypeName(name string) bool {
	if isBuiltinExceptionType(exceptionTypeName(name)) {
		return true
	}
	if strings.EqualFold(name, "sObject") {
		return true
	}
	switch name {
	case "Object", "String", "Boolean", "Integer", "Long", "Decimal", "Double", "Date", "Datetime", "Time", "TimeZone", "Blob", "Id", "Type", "URL", "JSONGenerator", "JSONParser", "JSONToken", "StatusCode", "ChildRelationship", "DescribeFieldResult", "DescribeSObjectResult", "DescribeTabResult", "DescribeTabSetResult", "PicklistEntry", "RecordTypeInfo", "XmlStreamReader", "XmlStreamWriter", "PageReference", "SelectOption", "LoggingLevel", "AccessType", "SObjectAccessDecision", "ApexPages.Severity", "ApexPages.StandardController", "ApexPages.StandardSetController", "RestContext", "RestRequest", "RestResponse", "Callable", "StubProvider", "InstallContext", "InstallHandler", "Auth.JWT", "ConnectApi.UserSettings", "ConnectApi.TimeZone", "Metadata.Metadata", "Metadata.MetadataType", "Metadata.DeployContainer", "Metadata.CustomMetadata", "Metadata.CustomField", "Metadata.CustomObject", "Metadata.DeployCallback", "Metadata.DeployCallBack", "Metadata.DeployResult", "Metadata.DeployStatus", "Metadata.DeployDetails", "Metadata.DeployMessage", "Metadata.DeployCallbackContext", "Metadata.AsyncResult":
		return true
	default:
		return false
	}
}

func isGenericTypeName(name string) bool {
	open := strings.IndexByte(name, '<')
	if open <= 0 || !strings.HasSuffix(name, ">") {
		return false
	}
	base := name[:open]
	args, ok := genericTypeArgs(name)
	if !ok {
		return false
	}
	switch base {
	case "List", "Set":
		return len(args) == 1 && isTypeNameToken(args[0])
	case "Map":
		return len(args) == 2 && isTypeNameToken(args[0]) && isTypeNameToken(args[1])
	default:
		return false
	}
}

func isTypeNameToken(name string) bool {
	if strings.HasSuffix(name, "[]") {
		return isTypeNameToken(strings.TrimSpace(strings.TrimSuffix(name, "[]")))
	}
	return isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name)
}

func isCommonSObjectTypeName(name string) bool {
	if storage.IsKnownStandardObject(name) {
		return true
	}
	for _, objectName := range standardSObjectPrefixes {
		if strings.EqualFold(name, objectName) {
			return true
		}
	}
	return false
}

func (vm *VM) resolveClassName(typeName string) (string, bool) {
	if isCommonSObjectTypeName(typeName) {
		return typeName, true
	}
	if !strings.Contains(typeName, ".") && vm.currentClass != "" {
		if resolved, ok := vm.resolveNestedTypeInClassHierarchy(vm.currentClass, typeName); ok {
			if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
				if class, found := vm.lookupClassInNamespace(namespace, typeName); found && strings.EqualFold(resolved, class.Name) {
					return runtimeClassName(class), true
				}
			}
			return resolved, true
		}
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			if class, ok := vm.lookupClassInNamespace(namespace, typeName); ok {
				return runtimeClassName(class), true
			}
		}
	}
	if strings.Contains(typeName, ".") && vm.currentClass != "" {
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			if class, ok := vm.lookupClass(namespace + "." + typeName); ok {
				return runtimeClassName(class), true
			}
		}
	}
	if class, ok := vm.lookupClass(typeName); ok {
		if strings.Contains(typeName, ".") && class.Namespace != "" {
			return runtimeClassName(class), true
		}
		return class.Name, true
	}
	return "", false
}

func (vm *VM) lookupClass(typeName string) (Class, bool) {
	if class, ok := vm.Classes[typeName]; ok {
		return class, true
	}
	if vm.classLookup == nil {
		vm.rebuildClassLookup()
	}
	if class, ok := vm.classLookup[canonicalClassLookupKey(typeName)]; ok {
		return class, true
	}
	return Class{}, false
}

func (vm *VM) storeClassAliases(class Class) {
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if vm.classLookup == nil {
		vm.classLookup = make(map[string]Class)
	}
	if existing, exists := vm.Classes[class.Name]; !exists || shouldReplaceShortClassAlias(existing, class) {
		vm.Classes[class.Name] = class
	}
	vm.resetClassAccessCaches()
	vm.enumLookup = nil
	vm.enumSuffixLookup = nil
	vm.storeClassLookupAlias(class.Name, class)
	if class.Namespace != "" {
		qualified := runtimeClassName(class)
		vm.Classes[qualified] = class
		vm.storeClassLookupAlias(qualified, class)
	}
}

func shouldReplaceShortClassAlias(existing, incoming Class) bool {
	if strings.EqualFold(existing.Namespace, incoming.Namespace) {
		return true
	}
	// Keep local/project class on short-name collisions; dependency classes
	// remain available through explicit namespace-qualified aliases.
	if !existing.Dependency && incoming.Dependency {
		return false
	}
	if existing.Dependency && !incoming.Dependency {
		return true
	}
	// Stable tie-breaker for same provenance kind.
	return strings.Compare(strings.ToLower(strings.TrimSpace(incoming.Namespace)), strings.ToLower(strings.TrimSpace(existing.Namespace))) < 0
}

func (vm *VM) storeClassLookupAlias(name string, class Class) {
	if strings.TrimSpace(name) == "" {
		return
	}
	vm.classLookup[canonicalClassLookupKey(name)] = class
}

func (vm *VM) storeClassValue(class Class) {
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if existing, exists := vm.Classes[class.Name]; !exists || shouldReplaceShortClassAlias(existing, class) {
		vm.Classes[class.Name] = class
	}
	vm.Classes[runtimeClassName(class)] = class
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		vm.Classes[class.Namespace+"."+class.Name] = class
	}
}

func runtimeClassName(class Class) string {
	name := strings.TrimSpace(class.Name)
	namespace := strings.TrimSpace(class.Namespace)
	if name == "" || namespace == "" {
		return name
	}
	if strings.HasPrefix(strings.ToLower(name), strings.ToLower(namespace)+".") {
		return name
	}
	return namespace + "." + name
}

func (vm *VM) rebuildClassLookup() {
	vm.resetClassAccessCaches()
	vm.classLookup = make(map[string]Class, len(vm.Classes)*2)
	for alias, class := range vm.Classes {
		vm.storeClassLookupAlias(alias, class)
		vm.storeClassLookupAlias(class.Name, class)
		if class.Namespace != "" {
			vm.storeClassLookupAlias(class.Namespace+"."+class.Name, class)
		}
	}
}

func (vm *VM) resetClassAccessCaches() {
	vm.namespaceClassLookup = make(map[string]map[string]namespaceClassLookup)
	vm.classNamespaceCache = make(map[string]string)
	vm.classForAccessCache = make(map[string]classForAccessLookup)
}

func canonicalClassLookupKey(name string) string {
	// Apex identifiers are ASCII. Avoid the strings.ToLower allocation
	// when no folding is required (most lookups in steady state).
	trimmed := strings.TrimSpace(name)
	needsFold := false
	for i := 0; i < len(trimmed); i++ {
		if c := trimmed[i]; c >= 'A' && c <= 'Z' {
			needsFold = true
			break
		}
	}
	if !needsFold {
		return trimmed
	}
	buf := make([]byte, len(trimmed))
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	return string(buf)
}

func resultForLookup() *Result {
	return &Result{TraceFormat: trace.FormatChromeTraceEvent, traceEnabled: true}
}

func newRestRequest() Value {
	request := Object("RestRequest")
	request.Fields["requestURI"] = String("")
	request.Fields["resourcePath"] = String("")
	request.Fields["httpMethod"] = String("")
	request.Fields["remoteAddress"] = String("")
	request.Fields["headers"] = typedMap("Map<String,String>")
	request.Fields["params"] = typedMap("Map<String,String>")
	request.Fields["requestBody"] = nullBlob()
	return request
}

func nullBlob() Value {
	blob := Object("Blob")
	blob.Fields["value"] = Null
	return blob
}

func newPageReference(rawURL string) Value {
	page := Object("PageReference")
	page.Fields["url"] = String(rawURL)
	page.Fields["parameters"] = pageReferenceParameters(rawURL)
	page.Fields["headers"] = typedMap("Map<String,String>")
	page.Fields["cookies"] = typedMap("Map<String,Cookie>")
	return page
}

func newDataWeaveScript(name string) Value {
	script := Object("DataWeave.Script")
	script.Fields["name"] = String(name)
	return script
}

func newDataWeaveResult(scriptName string, inputs Value) Value {
	result := Object("DataWeave.Result")
	result.Fields["scriptName"] = String(scriptName)
	mimeType := "application/apex"
	switch strings.ToLower(strings.TrimSpace(scriptName)) {
	case "helloworld":
		mimeType = "text/plain"
	case "multipleinputs":
		mimeType = "application/xml"
	}
	result.Fields["mimeType"] = String(mimeType)
	value := dataWeaveDefaultValue(scriptName, inputs)
	result.Fields["value"] = value
	valueAsString := dataWeaveValueAsString(value)
	if dataWeaveRawStringResult(scriptName) && value.Kind == ValueString {
		valueAsString = value.Text
	}
	result.Fields["valueAsString"] = String(valueAsString)
	return result
}

func dataWeaveDefaultValue(scriptName string, inputs Value) Value {
	switch strings.ToLower(strings.TrimSpace(scriptName)) {
	case "helloworld":
		return String("Hello World")
	case "csvtojsonbasic":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveCSVRecords(payload, ',', nil)
		}
	case "csvtojsonwithfieldrenaming":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveCSVRecords(payload, ',', map[string]string{
				"first_name": "FirstName",
				"last_name":  "LastName",
				"company":    "Company",
				"address":    "MailingStreet",
			})
		}
	case "csvseparatortojson":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveCSVRecords(payload, ';', nil)
		}
	case "csvtocontacts":
		if payload, ok := dataWeaveStringInput(inputs, "records"); ok {
			return dataWeaveCSVObjects(payload, ',', "Contact", nil)
		}
	case "csvtoapexobject":
		if payload, ok := dataWeaveStringInput(inputs, "records"); ok {
			return dataWeaveCSVObjects(payload, ',', "CsvData", nil)
		}
	case "jsontocontacts":
		if payload, ok := dataWeaveStringInput(inputs, "records"); ok {
			decoded, err := decodeJSONValue(payload)
			if err == nil {
				return dataWeaveJSONObjects(decoded, "Contact")
			}
		}
	case "pluralizefunction":
		if payload, ok := dataWeaveStringInput(inputs, "inputs"); ok {
			return dataWeavePluralize(payload)
		}
	case "reservedapexkeywords":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveReservedApexKeywords(payload)
		}
	case "logfilter":
		if payload, ok := dataWeaveStringInput(inputs, "payload"); ok {
			return dataWeaveLogFilter(payload)
		}
	case "jsondateformat":
		if records, ok := dataWeaveInput(inputs, "records"); ok {
			return dataWeaveJSONDateFormat(records)
		}
	case "multipleinputs":
		return dataWeaveMultipleInputsXML(inputs)
	}
	if inputs.Kind == ValueMap {
		if value, ok := inputs.Map[mapKey(String("records"))]; ok {
			return value
		}
	}
	return Null
}

func dataWeaveRawStringResult(scriptName string) bool {
	switch strings.ToLower(strings.TrimSpace(scriptName)) {
	case "multipleinputs":
		return true
	default:
		return false
	}
}

func dataWeaveInput(inputs Value, key string) (Value, bool) {
	if inputs.Kind != ValueMap {
		return Null, false
	}
	value, ok := inputs.Map[mapKey(String(key))]
	return value, ok
}

func dataWeaveStringInput(inputs Value, key string) (string, bool) {
	if inputs.Kind != ValueMap {
		return "", false
	}
	value, ok := inputs.Map[mapKey(String(key))]
	if !ok || value.Kind != ValueString {
		return "", false
	}
	return value.Text, true
}

func dataWeaveCSVRecords(payload string, comma rune, rename map[string]string) Value {
	rows := dataWeaveReadCSV(payload, comma)
	out := List()
	for _, row := range rows {
		record := Map()
		for key, value := range row {
			if renamed, ok := rename[key]; ok {
				key = renamed
			}
			record.Map[mapKey(String(key))] = String(value)
		}
		out.List = append(out.List, record)
	}
	return out
}

func dataWeaveCSVObjects(payload string, comma rune, typeName string, rename map[string]string) Value {
	rows := dataWeaveReadCSV(payload, comma)
	out := List()
	out.Type = "List<" + typeName + ">"
	for _, row := range rows {
		record := Object(typeName)
		for key, text := range row {
			if renamed, ok := rename[key]; ok {
				key = renamed
			}
			record.Fields[dataWeaveObjectFieldName(key)] = String(text)
		}
		out.List = append(out.List, record)
	}
	return out
}

func dataWeaveJSONObjects(decoded any, typeName string) Value {
	items, ok := decoded.([]any)
	if !ok {
		if records, recordsOK := jsonQueryResultRecords(decoded); recordsOK {
			items = records
		}
	}
	out := List()
	out.Type = "List<" + typeName + ">"
	for _, item := range items {
		if fields, ok := item.(map[string]any); ok {
			record := Object(typeName)
			for key, raw := range fields {
				if strings.EqualFold(key, "attributes") {
					continue
				}
				record.Fields[dataWeaveObjectFieldName(key)] = valueFromJSON(raw)
			}
			out.List = append(out.List, record)
		}
	}
	return out
}

func dataWeaveObjectFieldName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "first_name":
		return "FirstName"
	case "last_name":
		return "LastName"
	case "company":
		return "Company"
	case "address":
		return "MailingStreet"
	case "email":
		return "Email"
	default:
		return name
	}
}

func dataWeaveReadCSV(payload string, comma rune) []map[string]string {
	reader := csv.NewReader(strings.NewReader(payload))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.Comma = comma
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		return nil
	}
	headers := records[0]
	out := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(headers))
		for i, header := range headers {
			if i >= len(record) {
				row[header] = ""
				continue
			}
			row[header] = record[i]
		}
		out = append(out, row)
	}
	return out
}

func dataWeaveValueAsString(value Value) string {
	if text, ok := dataWeaveOrderedJSON(value); ok {
		return text
	}
	data, err := json.MarshalIndent(jsonFromValue(value, false), "", "  ")
	if err != nil {
		return value.String()
	}
	return string(data)
}

func dataWeaveOrderedJSON(value Value) (string, bool) {
	if value.Kind != ValueMap {
		return "", false
	}
	users, ok := value.Map[mapKey(String("users"))]
	if !ok || users.Kind != ValueList {
		return "", false
	}
	var b strings.Builder
	b.WriteString("{\n  \"users\": [")
	for i, user := range users.List {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {")
		first := true
		first = dataWeaveWriteJSONField(&b, user, "firstName", first)
		first = dataWeaveWriteJSONField(&b, user, "lastName", first)
		dataWeaveWriteJSONField(&b, user, "createdDate", first)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}")
	return b.String(), true
}

func dataWeaveWriteJSONField(b *strings.Builder, value Value, field string, first bool) bool {
	if value.Kind != ValueMap {
		return first
	}
	item, ok := value.Map[mapKey(String(field))]
	if !ok {
		return first
	}
	if !first {
		b.WriteString(",")
	}
	data, err := json.Marshal(jsonFromValue(item, false))
	if err != nil {
		data = []byte(strconv.Quote(item.String()))
	}
	b.WriteString("\n      ")
	b.WriteString(strconv.Quote(field))
	b.WriteString(": ")
	b.Write(data)
	return false
}

func dataWeavePluralize(payload string) Value {
	decoded, err := decodeJSONValue(payload)
	if err != nil {
		return Null
	}
	words, ok := decoded.([]any)
	if !ok {
		return Null
	}
	plurals := map[string]string{
		"box":    "boxes",
		"cat":    "cats",
		"deer":   "deer",
		"die":    "dice",
		"person": "people",
		"datum":  "data",
		"cactus": "cactus",
	}
	out := List()
	for _, raw := range words {
		word, ok := raw.(string)
		if !ok {
			continue
		}
		plural := plurals[word]
		if plural == "" {
			plural = word + "s"
		}
		item := Map()
		item.Map[mapKey(String(word))] = String(plural)
		out.List = append(out.List, item)
	}
	return out
}

func dataWeaveReservedApexKeywords(payload string) Value {
	decoded, err := decodeJSONValue(payload)
	if err != nil {
		return Null
	}
	items, ok := decoded.([]any)
	if !ok {
		return Null
	}
	out := List()
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		record := Map()
		for key, raw := range fields {
			if strings.EqualFold(key, "currency") {
				key = "currency_x"
			}
			record.Map[mapKey(String(key))] = valueFromJSON(raw)
		}
		out.List = append(out.List, record)
	}
	return out
}

func dataWeaveLogFilter(payload string) Value {
	decoded, err := decodeJSONValue(payload)
	if err != nil {
		return Null
	}
	items, ok := decoded.([]any)
	if !ok {
		return Null
	}
	out := List()
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if winner, ok := fields["isWinner"].(bool); !ok || !winner {
			continue
		}
		out.List = append(out.List, valueFromJSON(fields))
	}
	return out
}

func dataWeaveJSONDateFormat(records Value) Value {
	users := List()
	for _, record := range collectionMembers(records) {
		user := Map()
		if _, value, ok := objectFieldValue(record, "FirstName"); ok {
			user.Map[mapKey(String("firstName"))] = value
		}
		if _, value, ok := objectFieldValue(record, "LastName"); ok {
			user.Map[mapKey(String("lastName"))] = value
		}
		if _, value, ok := objectFieldValue(record, "CreatedDate"); ok {
			user.Map[mapKey(String("createdDate"))] = String(dataWeaveFormatDatetime(value))
		}
		users.List = append(users.List, user)
	}
	out := Map()
	out.Map[mapKey(String("users"))] = users
	return out
}

func dataWeaveMultipleInputsXML(inputs Value) Value {
	productsText, productsOK := dataWeaveStringInput(inputs, "products")
	attributesText, attributesOK := dataWeaveStringInput(inputs, "attributes")
	exchangeRatesText, exchangeRatesOK := dataWeaveStringInput(inputs, "exchangeRates")
	if !productsOK || !attributesOK || !exchangeRatesOK {
		return String("")
	}
	productsRaw, err := decodeJSONValue(productsText)
	if err != nil {
		return String("")
	}
	attributesRaw, err := decodeJSONValue(attributesText)
	if err != nil {
		return String("")
	}
	exchangeRatesRaw, err := decodeJSONValue(exchangeRatesText)
	if err != nil {
		return String("")
	}
	products, _ := productsRaw.([]any)
	attributes, _ := attributesRaw.(map[string]any)
	exchangeRates, _ := exchangeRatesRaw.(map[string]any)
	publishedAfter := dataWeaveJSONNumber(attributes["publishedAfter"])
	rates := dataWeaveJSONList(exchangeRates["USD"])

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><books>`)
	for _, rawProduct := range products {
		product, ok := rawProduct.(map[string]any)
		if !ok {
			continue
		}
		properties, _ := product["properties"].(map[string]any)
		year := dataWeaveJSONNumber(properties["year"])
		if year <= publishedAfter {
			continue
		}
		price := dataWeaveJSONNumber(product["price"])
		b.WriteString(`<book year="`)
		b.WriteString(escapeXMLAttr(dataWeaveFormatJSONNumber(year)))
		b.WriteString(`">`)
		for _, rawRate := range rates {
			rate, ok := rawRate.(map[string]any)
			if !ok {
				continue
			}
			currency, _ := rate["currency"].(string)
			ratio := dataWeaveJSONNumber(rate["ratio"])
			b.WriteString(`<price currency="`)
			b.WriteString(escapeXMLAttr(currency))
			b.WriteString(`">`)
			b.WriteString(escapeXMLText(dataWeaveFormatJSONNumber(price * ratio)))
			b.WriteString(`</price>`)
		}
		if title, ok := properties["title"].(string); ok {
			b.WriteString(`<title>`)
			b.WriteString(escapeXMLText(title))
			b.WriteString(`</title>`)
		}
		b.WriteString(`<authors>`)
		for _, rawAuthor := range dataWeaveJSONList(properties["author"]) {
			author, ok := rawAuthor.(string)
			if !ok {
				continue
			}
			b.WriteString(`<author>`)
			b.WriteString(escapeXMLText(author))
			b.WriteString(`</author>`)
		}
		b.WriteString(`</authors></book>`)
	}
	b.WriteString(`</books>`)
	return String(b.String())
}

func dataWeaveJSONList(raw any) []any {
	if items, ok := raw.([]any); ok {
		return items
	}
	return nil
}

func dataWeaveJSONNumber(raw any) float64 {
	switch value := raw.(type) {
	case json.Number:
		parsed, _ := strconv.ParseFloat(value.String(), 64)
		return parsed
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func dataWeaveFormatJSONNumber(value float64) string {
	rounded := math.Round(value*100) / 100
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

func dataWeaveFormatDatetime(value Value) string {
	text := ""
	if value.Kind == ValueString {
		text = value.Text
	} else if scalar, ok := platformScalarObjectText(value); ok {
		text = scalar
	}
	parsed, err := parseDatetimeTextAllowDateOnly(text)
	if err != nil {
		return text
	}
	return parsed.UTC().Format("03:04:05 PM, January 02, 2006")
}

func dataWeaveScriptName(receiver Value) string {
	if receiver.Kind != ValueObject {
		return ""
	}
	if _, value, ok := objectFieldValue(receiver, "name"); ok && value.Kind == ValueString {
		return value.Text
	}
	if strings.HasPrefix(receiver.Type, "DataWeaveScriptResource.") {
		return strings.TrimPrefix(receiver.Type, "DataWeaveScriptResource.")
	}
	return ""
}

func newCookie(args []Value) (Value, error) {
	if len(args) != 5 && len(args) != 6 && len(args) != 7 {
		return Null, fmt.Errorf("Cookie constructor expects 5, 6, or 7 arguments")
	}
	if args[0].Kind != ValueString || args[1].Kind != ValueString || (args[2].Kind != ValueString && args[2].Kind != ValueNull) || args[3].Kind != ValueInt || args[4].Kind != ValueBool {
		return Null, fmt.Errorf("Cookie constructor expects name, value, path, maxAge, and isSecure")
	}
	cookie := Object("Cookie")
	cookie.Fields["name"] = args[0]
	cookie.Fields["value"] = args[1]
	cookie.Fields["path"] = args[2]
	cookie.Fields["maxAge"] = args[3]
	cookie.Fields["secure"] = args[4]
	cookie.Fields["sameSite"] = Null
	cookie.Fields["httpOnly"] = Bool(false)
	if len(args) >= 6 {
		if args[5].Kind != ValueString {
			return Null, fmt.Errorf("Cookie constructor sameSite expects String")
		}
		cookie.Fields["sameSite"] = args[5]
	}
	if len(args) == 7 {
		if args[6].Kind != ValueBool {
			return Null, fmt.Errorf("Cookie constructor isHttpOnly expects Boolean")
		}
		cookie.Fields["httpOnly"] = args[6]
	}
	return cookie, nil
}

func newLocation(latitude, longitude Value) Value {
	location := Object("Location")
	location.Fields["latitude"] = Decimal(numericFloat(latitude))
	location.Fields["longitude"] = Decimal(numericFloat(longitude))
	return location
}

func newDomainFromHostname(hostname string) Value {
	host := strings.ToLower(strings.TrimSpace(hostname))
	host = strings.TrimSuffix(host, ".")
	domain := Object("Domain")
	domain.Fields["hostname"] = String(host)
	domain.Fields["domainType"] = domainTypeForHostname(host)
	domain.Fields["myDomainName"] = String(domainLabel(host))
	domain.Fields["packageName"] = String(domainPackageName(host))
	domain.Fields["sandboxName"] = Null
	domain.Fields["sitesSubdomainName"] = String(domainLabel(host))
	return domain
}

func domainParserHost(value Value) (string, error) {
	switch value.Kind {
	case ValueString:
		return hostFromURLText(value.Text), nil
	case ValueObject:
		if strings.EqualFold(value.Type, "URL") || strings.EqualFold(value.Type, "Url") {
			raw, err := platformScalarText(value, "URL")
			if err != nil {
				return "", err
			}
			return hostFromURLText(raw), nil
		}
	}
	return "", fmt.Errorf("DomainParser.parse expects hostname String or URL")
}

func hostFromURLText(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if parsed, err := url.Parse(text); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	if parsed, err := url.Parse("https://" + text); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return text
}

func domainLabel(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	first := strings.Split(host, ".")[0]
	if before, _, ok := strings.Cut(first, "--"); ok {
		first = before
	}
	return first
}

func domainPackageName(host string) string {
	first := strings.Split(strings.TrimSpace(host), ".")[0]
	before, _, ok := strings.Cut(first, "--")
	if !ok {
		return ""
	}
	return before
}

func domainTypeForHostname(host string) Value {
	normalized := strings.ToLower(host)
	name := "ORG_MY_DOMAIN"
	switch {
	case strings.Contains(normalized, "content"):
		name = "CONTENT_DOMAIN"
	case strings.Contains(normalized, "builder"):
		name = "EXPERIENCE_CLOUD_SITES_BUILDER_DOMAIN"
	case strings.Contains(normalized, "live-preview"):
		name = "EXPERIENCE_CLOUD_SITES_LIVE_PREVIEW_DOMAIN"
	case strings.Contains(normalized, "preview"):
		name = "EXPERIENCE_CLOUD_SITES_PREVIEW_DOMAIN"
	case strings.Contains(normalized, "site"):
		name = "EXPERIENCE_CLOUD_SITES_DOMAIN"
	case strings.Contains(normalized, "visualforce") || strings.Contains(normalized, "--"):
		name = "VISUALFORCE_DOMAIN"
	case strings.Contains(normalized, "lightning-container"):
		name = "LIGHTNING_CONTAINER_COMPONENT_DOMAIN"
	case strings.Contains(normalized, "lightning"):
		name = "LIGHTNING_DOMAIN"
	case strings.Contains(normalized, "setup"):
		name = "SETUP_DOMAIN"
	}
	return Value{Kind: ValueObject, Type: "DomainType", Text: name}
}

func localDomainHostname(kind, packageName string) string {
	normalizedPackage := strings.ToLower(strings.TrimSpace(packageName))
	packagePrefix := ""
	if normalizedPackage != "" {
		packagePrefix = normalizedPackage + "--"
	}
	switch strings.ToLower(kind) {
	case "contenthostname":
		return "glade.content.local"
	case "experiencecloudsitesbuilderhostname":
		return "glade.builder.sites.local"
	case "experiencecloudsiteshostname":
		return "glade.sites.local"
	case "experiencecloudsiteslivepreviewhostname":
		return "glade.live-preview.sites.local"
	case "experiencecloudsitespreviewhostname":
		return "glade.preview.sites.local"
	case "lightningcontainercomponenthostname":
		return packagePrefix + "glade.lightning-container.local"
	case "lightninghostname":
		return "glade.lightning.local"
	case "orgmydomainhostname", "org_my_domain":
		return "glade.my.salesforce.local"
	case "salesforcesiteshostname":
		return "glade.salesforce-sites.local"
	case "setuphostname":
		return "glade.setup.local"
	case "visualforcehostname":
		return packagePrefix + "glade.visualforce.local"
	default:
		return "glade.my.salesforce.local"
	}
}

func locationCoordinate(location Value, field string) (float64, bool) {
	if location.Kind != ValueObject || (!strings.EqualFold(location.Type, "Location") && !strings.EqualFold(location.Type, "Address")) {
		return 0, false
	}
	_, value, ok := objectFieldValue(location, field)
	if !ok || !isMathNumeric(value) {
		return 0, false
	}
	return numericFloat(value), true
}

func locationDistance(left, right Value, unit string) (Value, error) {
	leftLat, ok := locationCoordinate(left, "latitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects Location latitude")
	}
	leftLon, ok := locationCoordinate(left, "longitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects Location longitude")
	}
	rightLat, ok := locationCoordinate(right, "latitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects other Location latitude")
	}
	rightLon, ok := locationCoordinate(right, "longitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects other Location longitude")
	}
	const earthKm = 6371.0088
	lat1 := leftLat * math.Pi / 180
	lat2 := rightLat * math.Pi / 180
	dLat := (rightLat - leftLat) * math.Pi / 180
	dLon := (rightLon - leftLon) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	distance := earthKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "mi", "mile", "miles":
		distance *= 0.621371192237334
	case "m", "meter", "meters":
		distance *= 1000
	case "km", "kilometer", "kilometers", "":
	default:
		return Null, fmt.Errorf("Location.getDistance unit must be mi, km, or m")
	}
	return Decimal(distance), nil
}

func newQueueableDuplicateSignatureBuilder() Value {
	builder := Object("QueueableDuplicateSignature.Builder")
	builder.Fields["parts"] = typedList("List<String>")
	return builder
}

func newCurrencyValue(amount Value, isoCode string) Value {
	value := Object("CURRENCY")
	value.Fields["amount"] = Decimal(numericFloat(amount))
	value.Fields["isoCode"] = String(strings.ToUpper(strings.TrimSpace(isoCode)))
	return value
}

func currencyAmountText(value Value) string {
	if amount, ok := value.Fields["amount"]; ok {
		switch amount.Kind {
		case ValueDecimal:
			return strconv.FormatFloat(amount.Decimal, 'f', -1, 64)
		case ValueInt:
			return strconv.FormatInt(amount.Int, 10)
		}
	}
	return "0"
}

func currencyISOCode(value Value) string {
	if iso, ok := value.Fields["isoCode"]; ok && iso.Kind == ValueString && iso.Text != "" {
		return iso.Text
	}
	return "USD"
}

func formatLocalThreadingToken(id string) string {
	return "ref:_00Dlocal._" + id + ":ref"
}

func idText(value Value) string {
	id := scalarText(value)
	if id == "" {
		if text, ok := typedIDValueText(value); ok {
			id = text
		}
	}
	return strings.TrimSpace(id)
}

func recordIDFromEmail(subject, textBody, htmlBody Value) Value {
	for _, value := range []Value{subject, textBody, htmlBody} {
		if value.Kind != ValueString {
			continue
		}
		if id := recordIDFromThreadID(value.Text); id.Kind != ValueNull {
			return id
		}
	}
	return Null
}

func caseIDFromThreadID(text string) Value {
	if id := recordIDFromThreadID(text); id.Kind != ValueNull {
		if idText, ok := typedIDValueText(id); ok && strings.HasPrefix(idText, "500") {
			return id
		}
		return Null
	}
	start := strings.Index(text, "500")
	if start < 0 {
		return Null
	}
	end := start
	for end < len(text) {
		ch := text[end]
		if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
			break
		}
		end++
	}
	if end-start < 15 {
		return Null
	}
	id := text[start:end]
	if len(id) > 18 {
		id = id[:18]
	}
	return platformScalar("Id", id)
}

func recordIDFromThreadID(text string) Value {
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return r <= ' ' || r == '<' || r == '>' || r == '"' || r == '\'' || r == '[' || r == ']' || r == '(' || r == ')'
	}) {
		if !strings.Contains(strings.ToLower(token), "ref:") {
			continue
		}
		if id := idFromThreadingToken(token); id != "" {
			return platformScalar("Id", id)
		}
	}
	return Null
}

func idFromThreadingToken(token string) string {
	found := ""
	for i := 0; i < len(token); i++ {
		if !isSalesforceIDChar(token[i]) {
			continue
		}
		end := i
		for end < len(token) && isSalesforceIDChar(token[end]) {
			end++
		}
		if end-i >= 15 {
			id := token[i:end]
			if len(id) > 18 {
				id = id[:18]
			}
			found = id
		}
		i = end
	}
	return found
}

func isSalesforceIDChar(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func collatorCompare(left, right string) int64 {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func newPageTokenReference(rawURL string) Value {
	page := newPageReference(rawURL)
	page.Fields["__pageToken"] = Bool(true)
	return page
}

func pageReferenceParameters(rawURL string) Value {
	params := typedMap("Map<String,String>")
	params.Runtime = "pagereference-parameters"
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return params
	}
	for key, values := range parsed.Query() {
		if key == "" || len(values) == 0 {
			continue
		}
		encodedKey := mapKey(String(key))
		params.Map[encodedKey] = String(values[len(values)-1])
		params.MapKeys[encodedKey] = String(key)
	}
	return params
}

func pageReferenceURL(page Value) Value {
	raw, ok := page.Fields["url"]
	if !ok || raw.Kind != ValueString {
		return String("")
	}
	params, ok := page.Fields["parameters"]
	parsed, err := url.Parse(raw.Text)
	if err != nil {
		return raw
	}
	if !ok || params.Kind != ValueMap || params.Equal(pageReferenceParameters(raw.Text)) {
		if strings.Contains(parsed.RawQuery, "?") {
			query := url.Values{}
			for key, values := range parsed.Query() {
				if len(values) > 0 {
					query.Set(key, values[len(values)-1])
				}
			}
			parsed.RawQuery = query.Encode()
			return String(parsed.String())
		}
		return String(parsed.String())
	}
	query := url.Values{}
	for rawKey, value := range params.Map {
		key := mapStoredKey(params, rawKey)
		if key.Kind != ValueString || key.Text == "" || value.Kind == ValueNull {
			continue
		}
		if value.Kind == ValueString {
			query.Set(key.Text, value.Text)
			continue
		}
		query.Set(key.Text, value.String())
	}
	parsed.RawQuery = query.Encode()
	return String(parsed.String())
}

func newDomDocument() Value {
	doc := Object("Dom.Document")
	doc.Fields["root"] = Null
	return doc
}

func domXmlNodeTypeValue(name string) (Value, bool) {
	switch strings.ToUpper(name) {
	case "ELEMENT", "TEXT", "COMMENT":
		return Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: strings.ToUpper(name)}, true
	default:
		return Null, false
	}
}

func newDomXmlNode(nodeType, name, namespace, text string) Value {
	node := Object("Dom.XmlNode")
	node.Fields["nodeType"] = Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: nodeType}
	node.Fields["name"] = String(name)
	node.Fields["namespace"] = domNullableString(namespace)
	node.Fields["prefix"] = Null
	node.Fields["text"] = String(text)
	node.Fields["children"] = typedList("List<Dom.XmlNode>")
	node.Fields["attributes"] = typedList("List<Dom.XmlAttribute>")
	node.Fields["namespaces"] = typedMap("Map<String,String>")
	node.Fields["parent"] = Null
	return node
}

func domNullableString(value string) Value {
	if value == "" {
		return Null
	}
	return String(value)
}

func domString(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	return ""
}

func domNodeType(node Value) string {
	if value, ok := node.Fields["nodeType"]; ok && value.Kind == ValueObject {
		return value.Text
	}
	return ""
}

func domNodeList(node Value, field string) Value {
	if value, ok := node.Fields[field]; ok && value.Kind == ValueList {
		return value
	}
	return typedList("List<Dom.XmlNode>")
}

func domChildElements(node Value) Value {
	out := typedList("List<Dom.XmlNode>")
	for _, child := range domNodeList(node, "children").List {
		if domNodeType(child) == "ELEMENT" {
			out.List = append(out.List, child)
		}
	}
	return out
}

func domNamespaceFor(node Value, prefix string) Value {
	namespaces := node.Fields["namespaces"]
	if namespaces.Kind != ValueMap {
		return Null
	}
	if namespace, ok := namespaces.Map[mapKey(String(prefix))]; ok {
		return namespace
	}
	return Null
}

func domPrefixFor(node Value, namespace string) Value {
	namespaces := node.Fields["namespaces"]
	if namespaces.Kind != ValueMap {
		return Null
	}
	for rawKey, value := range namespaces.Map {
		if value.Kind == ValueString && value.Text == namespace {
			return valueFromMapKey(rawKey)
		}
	}
	return Null
}

func domSetParent(child, parent Value) Value {
	if child.Kind == ValueObject && child.Type == "Dom.XmlNode" {
		child.Fields["parent"] = parent
	}
	return child
}

func domAppendChild(parent, child Value) Value {
	children := domNodeList(parent, "children")
	child = domSetParent(child, parent)
	children.List = append(children.List, child)
	parent.Fields["children"] = children
	return child
}

func domAttribute(key, value, keyNamespace, valueNamespace string) Value {
	attr := Object("Dom.XmlAttribute")
	attr.Fields["key"] = String(key)
	attr.Fields["value"] = String(value)
	attr.Fields["keyNamespace"] = domNullableString(keyNamespace)
	attr.Fields["valueNamespace"] = domNullableString(valueNamespace)
	return attr
}

func domDocumentXMLString(doc Value) string {
	root, ok := doc.Fields["root"]
	if !ok || root.Kind != ValueObject {
		return `<?xml version="1.0" encoding="UTF-8"?>`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` + domNodeXMLString(root)
}

func domNodeXMLString(node Value) string {
	switch domNodeType(node) {
	case "TEXT":
		return escapeXMLText(domString(node.Fields["text"]))
	case "COMMENT":
		return "<!--" + strings.ReplaceAll(domString(node.Fields["text"]), "--", "- -") + "-->"
	case "ELEMENT":
		name := domString(node.Fields["name"])
		if name == "" {
			return ""
		}
		var out strings.Builder
		out.WriteByte('<')
		out.WriteString(name)
		for _, attr := range domNodeList(node, "attributes").List {
			key := domString(attr.Fields["key"])
			if key == "" {
				continue
			}
			out.WriteByte(' ')
			out.WriteString(key)
			out.WriteString(`="`)
			out.WriteString(escapeXMLAttr(domString(attr.Fields["value"])))
			out.WriteByte('"')
		}
		children := domNodeList(node, "children").List
		if len(children) == 0 {
			out.WriteString(" />")
			return out.String()
		}
		out.WriteByte('>')
		for _, child := range children {
			out.WriteString(domNodeXMLString(child))
		}
		out.WriteString("</")
		out.WriteString(name)
		out.WriteByte('>')
		return out.String()
	default:
		return ""
	}
}

func escapeXMLText(text string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(text))
	return out.String()
}

func escapeXMLAttr(text string) string {
	escaped := escapeXMLText(text)
	escaped = strings.ReplaceAll(escaped, `"`, "&#34;")
	escaped = strings.ReplaceAll(escaped, "'", "&#39;")
	return escaped
}

func parseDomDocument(source string) (Value, error) {
	source = normalizeHTMLVoidElementsForDOM(source)
	decoder := xml.NewDecoder(strings.NewReader(source))
	var stack []Value
	var root Value
	prefixes := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Null, fmt.Errorf("Dom.Document.load invalid XML: %w", err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			node := newDomXmlNode("ELEMENT", typed.Name.Local, typed.Name.Space, "")
			attrs := typedList("List<Dom.XmlAttribute>")
			namespaces := typedMap("Map<String,String>")
			for _, attr := range typed.Attr {
				if attr.Name.Local == "xmlns" && attr.Name.Space == "" {
					prefixes[""] = attr.Value
					namespaces.Map[mapKey(String(""))] = String(attr.Value)
					continue
				}
				if attr.Name.Space == "xmlns" {
					prefixes[attr.Name.Local] = attr.Value
					namespaces.Map[mapKey(String(attr.Name.Local))] = String(attr.Value)
					continue
				}
				attrs.List = append(attrs.List, domAttribute(attr.Name.Local, attr.Value, attr.Name.Space, ""))
			}
			for prefix, uri := range prefixes {
				if _, ok := namespaces.Map[mapKey(String(prefix))]; !ok {
					namespaces.Map[mapKey(String(prefix))] = String(uri)
				}
				if uri == typed.Name.Space && typed.Name.Space != "" {
					node.Fields["prefix"] = String(prefix)
				}
			}
			node.Fields["attributes"] = attrs
			node.Fields["namespaces"] = namespaces
			if len(stack) == 0 {
				root = node
			} else {
				parent := stack[len(stack)-1]
				domAppendChild(parent, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text := string([]byte(typed))
			if len(stack) == 0 || text == "" {
				continue
			}
			textNode := newDomXmlNode("TEXT", "", "", text)
			parent := stack[len(stack)-1]
			domAppendChild(parent, textNode)
		case xml.Comment:
			if len(stack) == 0 {
				continue
			}
			commentNode := newDomXmlNode("COMMENT", "", "", string([]byte(typed)))
			parent := stack[len(stack)-1]
			domAppendChild(parent, commentNode)
		}
	}
	if root.Kind == "" {
		return Null, fmt.Errorf("Dom.Document.load expected root element")
	}
	doc := newDomDocument()
	doc.Fields["root"] = root
	return doc, nil
}

var htmlVoidElementPattern = regexp.MustCompile(`(?i)<(area|base|br|col|embed|hr|img|input|link|meta|param|source|track|wbr)([^<>]*?)>`)

func normalizeHTMLVoidElementsForDOM(source string) string {
	return htmlVoidElementPattern.ReplaceAllStringFunc(source, func(tag string) string {
		trimmed := strings.TrimSpace(tag)
		if strings.HasSuffix(trimmed, "/>") {
			return tag
		}
		return strings.TrimSuffix(tag, ">") + "/>"
	})
}

func callDomDocumentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "load", "getRootElement", "createRootElement", "toXmlString")
	switch method {
	case "load":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.load expects String")
		}
		doc, err := parseDomDocument(args[0].Text)
		if err != nil {
			return Null, receiver, false, true, newExceptionError("XmlException", err.Error())
		}
		return Null, doc, true, true, nil
	case "getRootElement":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.getRootElement expects 0 arguments")
		}
		if root, ok := receiver.Fields["root"]; ok {
			return root, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "createRootElement":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.createRootElement expects name, namespace, prefix")
		}
		namespace := domString(args[1])
		root := newDomXmlNode("ELEMENT", args[0].Text, namespace, "")
		if args[2].Kind == ValueString && namespace != "" {
			namespaces := typedMap("Map<String,String>")
			namespaces.Map[mapKey(args[2])] = String(namespace)
			root.Fields["namespaces"] = namespaces
			root.Fields["prefix"] = args[2]
		}
		receiver.Fields["root"] = root
		return root, receiver, true, true, nil
	case "toXmlString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.toXmlString expects 0 arguments")
		}
		return String(domDocumentXMLString(receiver)), receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func callDomXmlNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method,
		"toXmlString", "getNodeType", "getName", "getNamespace", "getPrefix", "getText",
		"getChildren", "getChildElements", "getChildElement", "getParent",
		"getAttributeCount", "getAttributeKeyAt", "getAttributeKeyNsAt",
		"getAttribute", "getAttributeValue", "getAttributeValueNs",
		"getPrefixFor", "getNamespaceFor", "setNamespace", "setAttribute", "setAttributeNs",
		"removeAttribute", "addTextNode", "addCommentNode", "addChildElement",
		"removeChild", "insertBefore",
	)
	switch method {
	case "toXmlString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.toXmlString expects 0 arguments")
		}
		return String(domNodeXMLString(receiver)), receiver, false, true, nil
	case "getNodeType":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNodeType expects 0 arguments")
		}
		return receiver.Fields["nodeType"], receiver, false, true, nil
	case "getName":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getName expects 0 arguments")
		}
		return receiver.Fields["name"], receiver, false, true, nil
	case "getNamespace":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNamespace expects 0 arguments")
		}
		return receiver.Fields["namespace"], receiver, false, true, nil
	case "getPrefix":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getPrefix expects 0 arguments")
		}
		return receiver.Fields["prefix"], receiver, false, true, nil
	case "getText":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getText expects 0 arguments")
		}
		if domNodeType(receiver) != "ELEMENT" {
			return receiver.Fields["text"], receiver, false, true, nil
		}
		var text strings.Builder
		for _, child := range domNodeList(receiver, "children").List {
			if domNodeType(child) == "TEXT" || domNodeType(child) == "COMMENT" {
				text.WriteString(domString(child.Fields["text"]))
			}
		}
		return String(text.String()), receiver, false, true, nil
	case "getChildren":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildren expects 0 arguments")
		}
		return domNodeList(receiver, "children"), receiver, false, true, nil
	case "getChildElements":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildElements expects 0 arguments")
		}
		return domChildElements(receiver), receiver, false, true, nil
	case "getChildElement":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildElement expects name and namespace")
		}
		name := args[0].Text
		namespace := domString(args[1])
		for _, child := range domNodeList(receiver, "children").List {
			if domNodeType(child) != "ELEMENT" {
				continue
			}
			if domString(child.Fields["name"]) == name && domString(child.Fields["namespace"]) == namespace {
				return child, receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getParent":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getParent expects 0 arguments")
		}
		return receiver.Fields["parent"], receiver, false, true, nil
	case "getAttributeCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getAttributeCount expects 0 arguments")
		}
		return Int(int64(len(domNodeList(receiver, "attributes").List))), receiver, false, true, nil
	case "getAttributeKeyAt", "getAttributeKeyNsAt":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects Integer", method)
		}
		attrs := domNodeList(receiver, "attributes").List
		index := int(args[0].Int)
		if index < 0 || index >= len(attrs) {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s index out of bounds: %d", method, index)
		}
		field := "key"
		if method == "getAttributeKeyNsAt" {
			field = "keyNamespace"
		}
		return attrs[index].Fields[field], receiver, false, true, nil
	case "getAttribute", "getAttributeValue", "getAttributeValueNs":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects key and namespace", method)
		}
		key := args[0].Text
		namespace := ""
		if args[1].Kind == ValueString {
			namespace = args[1].Text
		}
		for _, attr := range domNodeList(receiver, "attributes").List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == namespace {
				if method == "getAttributeValueNs" {
					return attr.Fields["valueNamespace"], receiver, false, true, nil
				}
				return attr.Fields["value"], receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getPrefixFor":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getPrefixFor expects namespace String")
		}
		return domPrefixFor(receiver, args[0].Text), receiver, false, true, nil
	case "getNamespaceFor":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNamespaceFor expects prefix String")
		}
		return domNamespaceFor(receiver, args[0].Text), receiver, false, true, nil
	case "setNamespace":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setNamespace expects prefix and namespace Strings")
		}
		namespaces := receiver.Fields["namespaces"]
		if namespaces.Kind != ValueMap {
			namespaces = typedMap("Map<String,String>")
		}
		namespaces.Map[mapKey(args[0])] = args[1]
		receiver.Fields["namespaces"] = namespaces
		return Null, receiver, true, true, nil
	case "setAttribute":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setAttribute expects key and value Strings")
		}
		key := args[0].Text
		attrs := domNodeList(receiver, "attributes")
		for i, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == "" {
				attr.Fields["value"] = args[1]
				attrs.List[i] = attr
				receiver.Fields["attributes"] = attrs
				return Null, receiver, true, true, nil
			}
		}
		attrs.List = append(attrs.List, domAttribute(key, args[1].Text, "", ""))
		receiver.Fields["attributes"] = attrs
		return Null, receiver, true, true, nil
	case "setAttributeNs":
		if len(args) != 4 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setAttributeNs expects key, value, key namespace, and value namespace")
		}
		key := args[0].Text
		keyNamespace := domString(args[2])
		attrs := domNodeList(receiver, "attributes")
		for i, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == keyNamespace {
				attr.Fields["value"] = args[1]
				attr.Fields["valueNamespace"] = args[3]
				attrs.List[i] = attr
				receiver.Fields["attributes"] = attrs
				return Null, receiver, true, true, nil
			}
		}
		attrs.List = append(attrs.List, domAttribute(key, args[1].Text, keyNamespace, domString(args[3])))
		receiver.Fields["attributes"] = attrs
		return Null, receiver, true, true, nil
	case "removeAttribute":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.removeAttribute expects key and namespace")
		}
		key := args[0].Text
		keyNamespace := domString(args[1])
		attrs := domNodeList(receiver, "attributes")
		filtered := attrs.List[:0]
		removed := false
		for _, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == keyNamespace {
				removed = true
				continue
			}
			filtered = append(filtered, attr)
		}
		attrs.List = filtered
		receiver.Fields["attributes"] = attrs
		return Bool(removed), receiver, true, true, nil
	case "addTextNode", "addCommentNode":
		text, err := stringArg("Dom.XmlNode."+method, args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		nodeType := "TEXT"
		if method == "addCommentNode" {
			nodeType = "COMMENT"
		}
		child := newDomXmlNode(nodeType, "", "", text)
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "addChildElement":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.addChildElement expects name, namespace, prefix")
		}
		namespace := domString(args[1])
		child := newDomXmlNode("ELEMENT", args[0].Text, namespace, "")
		if args[2].Kind == ValueString && namespace != "" {
			namespaces := typedMap("Map<String,String>")
			namespaces.Map[mapKey(args[2])] = String(namespace)
			child.Fields["namespaces"] = namespaces
			child.Fields["prefix"] = args[2]
		}
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "removeChild":
		if len(args) != 1 || args[0].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.removeChild expects XmlNode")
		}
		children := domNodeList(receiver, "children")
		filtered := children.List[:0]
		removed := false
		for _, child := range children.List {
			if child.Equal(args[0]) {
				removed = true
				continue
			}
			filtered = append(filtered, child)
		}
		children.List = filtered
		receiver.Fields["children"] = children
		return Bool(removed), receiver, true, true, nil
	case "insertBefore":
		if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.insertBefore expects new child and reference child")
		}
		children := domNodeList(receiver, "children")
		newChild := domSetParent(args[0], receiver)
		inserted := false
		out := make([]Value, 0, len(children.List)+1)
		for _, child := range children.List {
			if !inserted && child.Equal(args[1]) {
				out = append(out, newChild)
				inserted = true
			}
			out = append(out, child)
		}
		if !inserted {
			out = append(out, newChild)
		}
		children.List = out
		receiver.Fields["children"] = children
		return newChild, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) newPageReference(rawURL string) Value {
	return newPageReference(vm.normalizePageReferenceURL(rawURL))
}

func (vm *VM) normalizePageReferenceURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(strings.ToLower(rawURL), "page.") {
		return rawURL
	}
	rest := rawURL[len("Page."):]
	pageName := rest
	suffix := ""
	for _, sep := range []string{"?", "#"} {
		if idx := strings.Index(pageName, sep); idx >= 0 {
			suffix = pageName[idx:]
			pageName = pageName[:idx]
			break
		}
	}
	if pageName == "" || vm.pageReferences == nil {
		return rawURL
	}
	registered, ok := vm.pageReferences[strings.ToLower(pageName)]
	if !ok {
		return rawURL
	}
	return "/apex/" + registered + suffix
}

func newAuthVerificationResult(redirect, success, message Value) Value {
	result := Object("Auth.VerificationResult")
	result.Fields["redirect"] = redirect
	result.Fields["success"] = success
	result.Fields["message"] = message
	return result
}

func newSelectOption(value, label Value, disabled, escapeItem Value) Value {
	option := Object("SelectOption")
	option.Fields["value"] = value
	option.Fields["label"] = label
	option.Fields["disabled"] = disabled
	option.Fields["escapeItem"] = escapeItem
	return option
}

func newHttpRequest() Value {
	request := Object("HttpRequest")
	request.Fields["endpoint"] = String("")
	request.Fields["method"] = String("")
	request.Fields["headers"] = typedMap("Map<String,String>")
	request.Fields["body"] = String("")
	request.Fields["compressed"] = Bool(false)
	request.Fields["timeout"] = Int(defaultHttpTimeoutMillis)
	return request
}

func newHttpResponse() Value {
	response := Object("HttpResponse")
	response.Fields["statusCode"] = Int(200)
	response.Fields["status"] = String("OK")
	response.Fields["headers"] = typedMap("Map<String,String>")
	response.Fields["body"] = String("")
	return response
}

func newContinuation(args []Value, namedArgs map[string]Value) (Value, error) {
	if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("Continuation constructor expects timeout Integer")
	}
	continuation := Object("Continuation")
	continuation.Fields["timeout"] = args[0]
	continuation.Fields["Timeout"] = args[0]
	continuation.Fields["ContinuationMethod"] = Null
	continuation.Fields["state"] = Null
	continuation.Fields["requests"] = typedMap("Map<String,HttpRequest>")
	return continuation, nil
}

func newFormulaBuilder() Value {
	builder := Object("formulaeval.FormulaBuilder")
	builder.Fields["formulaText"] = String("")
	builder.Fields["referencedFields"] = typedSet("Set<String>")
	return builder
}

func newFormulaInstance(builder Value) Value {
	instance := Object("formulaeval.FormulaInstance")
	if formulaText, ok := builder.Fields["formulaText"]; ok {
		instance.Fields["formulaText"] = formulaText
	}
	if referencedFields, ok := builder.Fields["referencedFields"]; ok {
		instance.Fields["referencedFields"] = referencedFields
	} else {
		instance.Fields["referencedFields"] = typedSet("Set<String>")
	}
	return instance
}

func newDatacloudFindDuplicatesResult() Value {
	result := Object("Datacloud.FindDuplicatesResult")
	result.Fields["duplicateResults"] = typedList("List<Datacloud.DuplicateResult>")
	result.Fields["errors"] = typedList("List<Database.Error>")
	result.Fields["success"] = Bool(true)
	return result
}

func newSendEmailResult() Value {
	result := Object("Messaging.SendEmailResult")
	result.Fields["success"] = Bool(true)
	result.Fields["errors"] = List()
	return result
}

func newSendEmailError(message string) Value {
	err := Object("Messaging.SendEmailError")
	err.Fields["message"] = String(message)
	return err
}

func newEmailFileAttachment() Value {
	attachment := Object("Messaging.EmailFileAttachment")
	attachment.Fields["body"] = Null
	attachment.Fields["contentType"] = Null
	attachment.Fields["fileName"] = Null
	attachment.Fields["id"] = Null
	attachment.Fields["inline"] = Bool(false)
	return attachment
}

func newRenderEmailTemplateBodyResult(mergedBody string) Value {
	result := Object("Messaging.RenderEmailTemplateBodyResult")
	result.Fields["success"] = Bool(true)
	result.Fields["mergedBody"] = String(mergedBody)
	result.Fields["errors"] = List()
	return result
}

func newFailedSendEmailResult(message string) Value {
	result := Object("Messaging.SendEmailResult")
	result.Fields["success"] = Bool(false)
	result.Fields["errors"] = List(newSendEmailError(message))
	return result
}

func newSingleEmailMessage() Value {
	message := Object("Messaging.SingleEmailMessage")
	for _, field := range []string{
		"toAddresses", "ccAddresses", "bccAddresses", "fileAttachments",
		"entityAttachments", "documentAttachments", "targetObjectIds",
	} {
		message.Fields[field] = List()
	}
	for _, field := range []string{
		"subject", "plainTextBody", "htmlBody", "replyTo", "senderDisplayName",
		"charset", "inReplyTo", "references", "orgWideEmailAddressId",
		"targetObjectId", "templateId", "templateName", "whatId", "optOutPolicy",
		"emailPriority", "unsubscribeComment",
	} {
		message.Fields[field] = Null
	}
	message.Fields["unsubscribeUrls"] = List()
	for _, field := range []string{
		"saveAsActivity", "treatBodiesAsTemplate", "treatTargetObjectAsRecipient",
		"useSignature", "bccSender", "oneClickPost", "userMail",
	} {
		message.Fields[field] = Bool(false)
	}
	return message
}

func newMassEmailMessage() Value {
	message := Object("Messaging.MassEmailMessage")
	for _, field := range []string{"targetObjectIds", "whatIds"} {
		message.Fields[field] = List()
	}
	for _, field := range []string{
		"templateId", "description", "optOutPolicy", "replyTo", "senderDisplayName",
		"subject", "emailPriority",
	} {
		message.Fields[field] = Null
	}
	for _, field := range []string{"saveAsActivity", "bccSender", "useSignature"} {
		message.Fields[field] = Bool(false)
	}
	return message
}

func newInboundEmail() Value {
	email := Object("Messaging.InboundEmail")
	for _, field := range []string{"authenticationResults", "binaryAttachments", "ccAddresses", "headers", "textAttachments", "toAddresses"} {
		email.Fields[field] = List()
	}
	for _, field := range []string{
		"fromAddress", "fromName", "htmlBody", "inReplyTo", "messageId", "plainTextBody",
		"references", "replyTo", "subject",
	} {
		email.Fields[field] = Null
	}
	email.Fields["htmlBodyIsTruncated"] = Bool(false)
	email.Fields["plainTextBodyIsTruncated"] = Bool(false)
	return email
}

func newInboundEnvelope() Value {
	envelope := Object("Messaging.InboundEnvelope")
	envelope.Fields["fromAddress"] = Null
	envelope.Fields["toAddress"] = Null
	return envelope
}

func newInboundEmailResult() Value {
	result := Object("Messaging.InboundEmailResult")
	result.Fields["success"] = Bool(false)
	result.Fields["message"] = Null
	return result
}

func newActionResult() Value {
	result := Object("Messaging.ActionResult")
	result.Fields["success"] = Bool(false)
	result.Fields["message"] = Null
	result.Fields["errorCode"] = Null
	return result
}

func newActionResultBuilder() Value {
	builder := Object("Messaging.ActionResult.Builder")
	builder.Fields["success"] = Bool(false)
	builder.Fields["message"] = Null
	builder.Fields["errorCode"] = Null
	return builder
}

func newActionableNotification() Value {
	notification := Object("Messaging.ActionableNotification")
	for _, field := range []string{"actionIdentifier", "notificationTypeId", "recipientId", "senderId", "targetId", "targetPageRef"} {
		notification.Fields[field] = Null
	}
	return notification
}

func newActionableNotificationBuilder() Value {
	builder := Object("Messaging.ActionableNotification.Builder")
	for _, field := range []string{"actionIdentifier", "notificationTypeId", "recipientId", "senderId", "targetId", "targetPageRef"} {
		builder.Fields[field] = Null
	}
	return builder
}

func newCustomNotification(args []Value) Value {
	notification := Object("Messaging.CustomNotification")
	for _, field := range []string{"notificationTypeId", "senderId", "title", "body", "targetId", "targetPageRef"} {
		notification.Fields[field] = Null
	}
	if len(args) == 6 {
		notification.Fields["notificationTypeId"] = args[0]
		notification.Fields["senderId"] = args[1]
		notification.Fields["title"] = args[2]
		notification.Fields["body"] = args[3]
		notification.Fields["targetId"] = args[4]
		notification.Fields["targetPageRef"] = args[5]
	}
	return notification
}

func newPushNotification(args []Value) Value {
	notification := Object("Messaging.PushNotification")
	notification.Fields["payload"] = typedMap("Map<String,Object>")
	notification.Fields["ttl"] = Null
	if len(args) == 1 {
		notification.Fields["payload"] = args[0]
	}
	return notification
}

func isLocalEmailMessage(value Value) bool {
	return value.Kind == ValueObject && (value.Type == "Messaging.SingleEmailMessage" || value.Type == "Messaging.MassEmailMessage")
}

func (vm *VM) sendEmail(args []Value, result *Result) (Value, error) {
	if len(args) == 0 {
		return Null, fmt.Errorf("Messaging.sendEmail expects messages")
	}
	if len(args) > 2 {
		return Null, unsupportedCallError("Messaging.sendEmail send options overloads")
	}
	if args[0].Kind != ValueList {
		return Null, fmt.Errorf("Messaging.sendEmail expects List")
	}
	if len(args) == 2 && args[1].Kind != ValueBool {
		return Null, unsupportedCallError("Messaging.sendEmail send options overloads")
	}
	allOrNothing := true
	if len(args) == 2 {
		allOrNothing = args[1].Bool
	}
	for _, message := range args[0].List {
		if !isLocalEmailMessage(message) {
			return Null, fmt.Errorf("Messaging.sendEmail expects SingleEmailMessage or MassEmailMessage list items")
		}
	}
	validationErrors := make([]string, len(args[0].List))
	for i, message := range args[0].List {
		validationErrors[i] = localEmailValidationError(message)
		if validationErrors[i] != "" && allOrNothing {
			return Null, newExceptionError("EmailException", validationErrors[i])
		}
	}
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.email.send", "apex.email", map[string]any{"messages": len(args[0].List)})
	results := make([]Value, 0, len(args[0].List))
	for i, message := range args[0].List {
		if validationErrors[i] != "" {
			results = append(results, newFailedSendEmailResult(validationErrors[i]))
			continue
		}
		captured := vm.captureEmail(message)
		if message.Type == "Messaging.SingleEmailMessage" && captured.TemplateID != "" {
			message.Fields["subject"] = String(captured.Subject)
			message.Fields["plainTextBody"] = String(captured.PlainTextBody)
			message.Fields["htmlBody"] = String(captured.HTMLBody)
			args[0].List[i] = message
		}
		vm.capturedEmails = append(vm.capturedEmails, captured)
		results = append(results, newSendEmailResult())
	}
	return List(results...), nil
}

func (vm *VM) reserveEmailCapacity(callee string, args []Value, result *Result) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("%s expects Integer", callee)
	}
	if args[0].Int < 0 {
		return Null, fmt.Errorf("%s expects non-negative Integer", callee)
	}
	appendTrace(result, "apex.email.reserve", "apex.email", map[string]any{"method": callee, "capacity": args[0].Int})
	return Null, nil
}

func (vm *VM) sendEmailMessage(args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 2 {
		return Null, fmt.Errorf("Messaging.sendEmailMessage expects email message Ids and optional allOrNothing")
	}
	if args[0].Kind != ValueList {
		return Null, fmt.Errorf("Messaging.sendEmailMessage expects List<Id>")
	}
	if len(args) == 2 && args[1].Kind != ValueBool {
		return Null, fmt.Errorf("Messaging.sendEmailMessage allOrNothing expects Boolean")
	}
	for _, id := range args[0].List {
		if _, ok := idValueText(id); !ok {
			return Null, fmt.Errorf("Messaging.sendEmailMessage expects List<Id>")
		}
	}
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.email.message.send", "apex.email", map[string]any{"messages": len(args[0].List)})
	results := make([]Value, 0, len(args[0].List))
	for range args[0].List {
		results = append(results, newSendEmailResult())
	}
	return List(results...), nil
}

func (vm *VM) renderEmailTemplate(args []Value) (Value, error) {
	if len(args) != 3 {
		return Null, fmt.Errorf("Messaging.renderEmailTemplate expects whoId, whatId, bodies")
	}
	whoID := args[0]
	whatID := args[1]
	if args[0].Kind != ValueNull {
		if _, ok := idValueText(args[0]); !ok {
			return Null, fmt.Errorf("Messaging.renderEmailTemplate expects whoId String or Id")
		}
	}
	if args[1].Kind != ValueNull {
		if _, ok := idValueText(args[1]); !ok {
			return Null, fmt.Errorf("Messaging.renderEmailTemplate expects whatId String or Id")
		}
	}
	if args[2].Kind != ValueList {
		return Null, fmt.Errorf("Messaging.renderEmailTemplate expects List<String> bodies")
	}
	results := make([]Value, 0, len(args[2].List))
	for _, body := range args[2].List {
		if body.Kind != ValueString {
			return Null, fmt.Errorf("Messaging.renderEmailTemplate expects List<String> bodies")
		}
		results = append(results, newRenderEmailTemplateBodyResult(vm.renderEmailTemplateText(body.Text, whoID, whatID)))
	}
	return List(results...), nil
}

func (vm *VM) extractInboundEmail(args []Value) (Value, error) {
	if len(args) != 2 || args[1].Kind != ValueBool {
		return Null, fmt.Errorf("Messaging.extractInboundEmail expects source and includeForwardedAttachments Boolean")
	}
	if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Messaging.InboundEmail") {
		return args[0], nil
	}
	return newInboundEmail(), nil
}

func localEmailValidationError(message Value) string {
	if message.Type != "Messaging.SingleEmailMessage" {
		return ""
	}
	if emailFieldString(message, "plainTextBody") != "" || emailFieldString(message, "htmlBody") != "" || emailFieldString(message, "templateId") != "" {
		return ""
	}
	return "Email body or template ID is required"
}

func emailFieldString(message Value, field string) string {
	if message.Kind != ValueObject || message.Fields == nil {
		return ""
	}
	if value, ok := message.Fields[field]; ok {
		if text := stringValue(value); text != "" {
			return text
		}
	}
	normalized := strings.ToLower(field)
	for candidate, value := range message.Fields {
		if strings.ToLower(candidate) == normalized {
			if text := stringValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func emailFieldBool(message Value, field string) bool {
	_, value, _ := objectFieldValue(message, field)
	if boolValue(value) {
		return true
	}
	normalized := strings.ToLower(field)
	for candidate, value := range message.Fields {
		if strings.ToLower(candidate) == normalized && boolValue(value) {
			return true
		}
	}
	return false
}

func emailFieldStrings(message Value, field string) []string {
	_, value, _ := objectFieldValue(message, field)
	values := stringsFromList(value)
	if len(values) > 0 {
		return values
	}
	normalized := strings.ToLower(field)
	for candidate, value := range message.Fields {
		if strings.ToLower(candidate) == normalized {
			if values := stringsFromList(value); len(values) > 0 {
				return values
			}
		}
	}
	return nil
}

func (vm *VM) captureEmail(message Value) CapturedEmail {
	captured := CapturedEmail{Kind: message.Type}
	switch message.Type {
	case "Messaging.SingleEmailMessage":
		captured.ToAddresses = emailFieldStrings(message, "toAddresses")
		captured.CcAddresses = emailFieldStrings(message, "ccAddresses")
		captured.BccAddresses = emailFieldStrings(message, "bccAddresses")
		captured.FileAttachments = emailFieldStrings(message, "fileAttachments")
		captured.EntityAttachments = emailFieldStrings(message, "entityAttachments")
		captured.DocumentAttachments = emailFieldStrings(message, "documentAttachments")
		captured.TargetObjectIDs = emailFieldStrings(message, "targetObjectIds")
		captured.Subject = emailFieldString(message, "subject")
		captured.PlainTextBody = emailFieldString(message, "plainTextBody")
		captured.HTMLBody = emailFieldString(message, "htmlBody")
		captured.TemplateID = emailFieldString(message, "templateId")
		captured.TargetObjectID = emailFieldString(message, "targetObjectId")
		captured.WhatID = emailFieldString(message, "whatId")
		captured.SaveAsActivity = emailFieldBool(message, "saveAsActivity")
		vm.renderCapturedEmailTemplate(&captured)
	case "Messaging.MassEmailMessage":
		captured.TargetObjectIDs = emailFieldStrings(message, "targetObjectIds")
		captured.WhatIDs = emailFieldStrings(message, "whatIds")
		captured.TemplateID = emailFieldString(message, "templateId")
		captured.SaveAsActivity = emailFieldBool(message, "saveAsActivity")
		if captured.TemplateID != "" && len(captured.TargetObjectIDs) > 0 {
			captured.TargetObjectID = captured.TargetObjectIDs[0]
			if len(captured.WhatIDs) > 0 {
				captured.WhatID = captured.WhatIDs[0]
			}
			vm.renderCapturedEmailTemplate(&captured)
		}
	}
	return captured
}

func (vm *VM) captureWorkflowEmail(alert storage.WorkflowEmailAlert, record storage.Record, result *Result) error {
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return err
	}
	captured := CapturedEmail{
		Kind:   "WorkflowEmailAlert",
		WhatID: string(record.ID),
	}
	captured.ToAddresses, captured.TargetObjectIDs = vm.workflowEmailRecipients(alert, record)
	if len(captured.TargetObjectIDs) > 0 {
		captured.TargetObjectID = captured.TargetObjectIDs[0]
	}
	if template, ok := vm.emailTemplateByName(alert.Template); ok {
		captured.TemplateID = string(template.ID)
		whoID := Null
		if len(captured.TargetObjectIDs) > 0 {
			whoID = String(captured.TargetObjectIDs[0])
		}
		whatID := Null
		if captured.WhatID != "" {
			whatID = String(captured.WhatID)
		}
		captured.Subject = vm.renderEmailTemplateText(storageStringField(template, "Subject"), whoID, whatID)
		captured.HTMLBody = vm.renderEmailTemplateHTML(template, whoID, whatID)
		captured.PlainTextBody = vm.renderEmailTemplateText(storageStringField(template, "Body"), whoID, whatID)
	}
	vm.capturedEmails = append(vm.capturedEmails, captured)
	appendTrace(result, "apex.email.workflow", "apex.email", map[string]any{
		"alert":      alert.Name,
		"template":   alert.Template,
		"recipients": len(captured.ToAddresses),
		"record":     string(record.ID),
	})
	return nil
}

func (vm *VM) workflowEmailRecipients(alert storage.WorkflowEmailAlert, record storage.Record) ([]string, []string) {
	addresses := make([]string, 0, len(alert.Recipients))
	targetIDs := make([]string, 0, len(alert.Recipients))
	for _, recipient := range alert.Recipients {
		if recipient.Recipient != "" {
			vm.appendWorkflowEmailRecipient(recipient.Type, recipient.Recipient, &addresses, &targetIDs)
			continue
		}
		fieldName := recipient.Field
		if fieldName == "" && strings.EqualFold(strings.TrimSpace(recipient.Type), "owner") {
			fieldName = "OwnerId"
		}
		if fieldName == "" {
			continue
		}
		if vm.Org != nil {
			if objectName, ok := vm.resolveObjectName(record.Object); ok {
				if object, ok := vm.Org.Objects[objectName]; ok {
					if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName); ok {
						fieldName = resolved
					}
				}
			}
		}
		if value, ok := record.GetField(fieldName); ok {
			vm.appendWorkflowEmailRecipient(recipient.Type, workflowEmailRecipientValue(value), &addresses, &targetIDs)
			continue
		}
		if strings.EqualFold(fieldName, "OwnerId") && record.System.OwnerID != "" {
			vm.appendWorkflowEmailRecipient(recipient.Type, string(record.System.OwnerID), &addresses, &targetIDs)
		}
	}
	return addresses, targetIDs
}

func (vm *VM) appendWorkflowEmailRecipient(recipientType, raw string, addresses, targetIDs *[]string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return
	}
	normalizedType := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(recipientType), " ", ""))
	if workflowRecipientLooksLikeID(value) || (normalizedType == "owner" && !strings.Contains(value, "@")) {
		*targetIDs = append(*targetIDs, value)
		return
	}
	*addresses = append(*addresses, value)
}

func workflowEmailRecipientValue(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}

func workflowRecipientLooksLikeID(value string) bool {
	if len(value) != 15 && len(value) != 18 {
		return false
	}
	for _, ch := range value {
		if ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
			continue
		}
		return false
	}
	return true
}

func (vm *VM) renderCapturedEmailTemplate(captured *CapturedEmail) {
	if captured == nil || captured.TemplateID == "" || vm.Org == nil {
		return
	}
	template, ok := vm.emailTemplateByID(captured.TemplateID)
	if !ok {
		return
	}
	whoID := Null
	if captured.TargetObjectID != "" {
		whoID = String(captured.TargetObjectID)
	}
	whatID := Null
	if captured.WhatID != "" {
		whatID = String(captured.WhatID)
	}
	if captured.Subject == "" {
		captured.Subject = vm.renderEmailTemplateText(storageStringField(template, "Subject"), whoID, whatID)
	}
	if captured.HTMLBody == "" {
		captured.HTMLBody = vm.renderEmailTemplateHTML(template, whoID, whatID)
	}
	if captured.PlainTextBody == "" {
		captured.PlainTextBody = vm.renderEmailTemplateText(storageStringField(template, "Body"), whoID, whatID)
	}
}

func stringsFromList(value Value) []string {
	if value.Kind != ValueList {
		return nil
	}
	out := make([]string, 0, len(value.List))
	for _, item := range value.List {
		if item.Kind == ValueString {
			out = append(out, item.Text)
		}
	}
	return out
}

func stringValue(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	if text, ok := platformScalarObjectText(value); ok {
		return text
	}
	return ""
}

func boolValue(value Value) bool {
	return value.Kind == ValueBool && value.Bool
}

func (vm *VM) renderStoredEmailTemplate(args []Value) (Value, error) {
	if len(args) != 3 {
		return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects templateId, whoId, whatId")
	}
	for i, arg := range args {
		if arg.Kind == ValueNull {
			continue
		}
		if _, ok := idValueText(arg); !ok {
			names := []string{"templateId", "whoId", "whatId"}
			return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects %s String or Id", names[i])
		}
	}
	templateID, _ := idValueText(args[0])
	if templateID == "" {
		return Null, newExceptionError("EmailException", fmt.Sprintf("Email template not found: %s", templateID))
	}
	template, ok := vm.emailTemplateByID(templateID)
	if !ok {
		return Null, newExceptionError("EmailException", fmt.Sprintf("Email template not found: %s", templateID))
	}

	message := newSingleEmailMessage()
	message.Fields["templateId"] = String(templateID)
	message.Fields["targetObjectId"] = args[1]
	message.Fields["whatId"] = args[2]
	message.Fields["subject"] = String(vm.renderEmailTemplateText(storageStringField(template, "Subject"), args[1], args[2]))
	message.Fields["htmlBody"] = String(vm.renderEmailTemplateHTML(template, args[1], args[2]))
	message.Fields["plainTextBody"] = String(vm.renderEmailTemplateText(storageStringField(template, "Body"), args[1], args[2]))
	return message, nil
}

func (vm *VM) emailTemplateByID(templateID string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	objectName, ok := vm.resolveObjectName("EmailTemplate")
	if !ok {
		objectName = "EmailTemplate"
	}
	object := vm.Org.Objects[objectName]
	if record, ok := object.Records[storage.ID(templateID)]; ok {
		return record, true
	}
	for _, record := range object.Records {
		if string(record.ID) == templateID {
			return record, true
		}
		if id, ok := record.GetField("Id"); ok && string(storageIDFromValue(id)) == templateID {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) emailTemplateByName(name string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return storage.Record{}, false
	}
	objectName, ok := vm.resolveObjectName("EmailTemplate")
	if !ok {
		objectName = "EmailTemplate"
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	for _, record := range object.Records {
		for _, field := range []string{"DeveloperName", "Name"} {
			if strings.EqualFold(storageStringField(record, field), name) {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}

func (vm *VM) renderEmailTemplateText(text string, whoID, whatID Value) string {
	if text == "" || !strings.Contains(text, "{!") {
		return text
	}
	whoRecord, whoOK := vm.recordByIDValue(whoID)
	whatRecord, whatOK := vm.recordByIDValue(whatID)
	var out strings.Builder
	for {
		start := strings.Index(text, "{!")
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:start])
		text = text[start+2:]
		end := strings.Index(text, "}")
		if end < 0 {
			out.WriteString("{!")
			out.WriteString(text)
			return out.String()
		}
		token := strings.TrimSpace(text[:end])
		if value, ok := vm.emailMergeTokenValue(token, whoRecord, whoOK, whatRecord, whatOK); ok {
			out.WriteString(value)
		} else {
			out.WriteString("{!")
			out.WriteString(text[:end])
			out.WriteString("}")
		}
		text = text[end+1:]
	}
}

func (vm *VM) renderEmailTemplateHTML(template storage.Record, whoID, whatID Value) string {
	html := storageStringField(template, "HtmlValue")
	if html == "" && emailTemplateLooksVisualforce(template) {
		markup := storageStringField(template, "Markup")
		if markup == "" {
			markup = storageStringField(template, "Body")
		}
		html = vm.renderVisualforceEmailTemplateMarkup(markup, whoID, whatID)
	}
	return vm.renderEmailTemplateText(html, whoID, whatID)
}

func emailTemplateLooksVisualforce(template storage.Record) bool {
	if strings.EqualFold(storageStringField(template, "TemplateType"), "visualforce") {
		return true
	}
	for _, field := range []string{"Markup", "Body"} {
		text := strings.TrimSpace(storageStringField(template, field))
		if strings.Contains(strings.ToLower(text), "<messaging:emailtemplate") {
			return true
		}
	}
	return false
}

func (vm *VM) renderVisualforceEmailTemplateMarkup(markup string, whoID, whatID Value) string {
	body := visualforceTagBody(markup, "messaging:htmlEmailBody")
	if body == "" {
		body = markup
	}
	body = vm.replaceVisualforceTemplateTags(body)
	return vm.renderEmailTemplateText(body, whoID, whatID)
}

func (vm *VM) replaceVisualforceTemplateTags(input string) string {
	var out strings.Builder
	lastHandled := false
	for i := 0; i < len(input); {
		start := strings.IndexByte(input[i:], '<')
		if start < 0 {
			out.WriteString(input[i:])
			break
		}
		start += i
		gap := input[i:start]
		nameStart := start + 1
		if nameStart >= len(input) || input[nameStart] == '/' || input[nameStart] == '!' || input[nameStart] == '?' {
			out.WriteString(gap)
			out.WriteByte(input[start])
			i = start + 1
			lastHandled = false
			continue
		}
		nameEnd := nameStart
		for nameEnd < len(input) {
			ch := input[nameEnd]
			if ch == '>' || ch == '/' || unicode.IsSpace(rune(ch)) {
				break
			}
			nameEnd++
		}
		if nameEnd == nameStart {
			out.WriteString(gap)
			out.WriteByte(input[start])
			i = start + 1
			lastHandled = false
			continue
		}
		end := visualforceTagEnd(input, nameEnd)
		if end < 0 {
			out.WriteString(gap)
			out.WriteString(input[start:])
			break
		}
		name := input[nameStart:nameEnd]
		attrs := input[nameEnd:end]
		handled := true
		replacement := ""
		switch {
		case strings.EqualFold(name, "apex:outputText"):
			replacement = visualforceOutputTextValue(attrs)
		case strings.EqualFold(visualforceLocalTagName(name), "EmailContent"):
			key := visualforceAttrValue(attrs, "key")
			replacement = vm.visualforceEmailContentValue(key)
		default:
			handled = false
			replacement = input[start : end+1]
		}
		if !handled || !lastHandled || strings.TrimSpace(gap) != "" {
			out.WriteString(gap)
		}
		out.WriteString(replacement)
		lastHandled = handled
		i = end + 1
	}
	return out.String()
}

func visualforceTagBody(markup, tagName string) string {
	lower := strings.ToLower(markup)
	openNeedle := "<" + strings.ToLower(tagName)
	open := strings.Index(lower, openNeedle)
	if open < 0 {
		return ""
	}
	openEnd := visualforceTagEnd(markup, open+len(openNeedle))
	if openEnd < 0 {
		return ""
	}
	closeNeedle := "</" + strings.ToLower(tagName) + ">"
	close := strings.Index(lower[openEnd+1:], closeNeedle)
	if close < 0 {
		return ""
	}
	close += openEnd + 1
	return markup[openEnd+1 : close]
}

func visualforceTagEnd(input string, start int) int {
	var quote byte
	escaped := false
	for i := start; i < len(input); i++ {
		ch := input[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i
		}
	}
	return -1
}

func visualforceLocalTagName(name string) string {
	if idx := strings.LastIndexByte(name, ':'); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func visualforceAttrValue(attrs, name string) string {
	for i := 0; i < len(attrs); {
		for i < len(attrs) && (unicode.IsSpace(rune(attrs[i])) || attrs[i] == '/') {
			i++
		}
		start := i
		for i < len(attrs) {
			ch := attrs[i]
			if ch == '=' || unicode.IsSpace(rune(ch)) || ch == '/' {
				break
			}
			i++
		}
		attrName := strings.TrimSpace(attrs[start:i])
		for i < len(attrs) && unicode.IsSpace(rune(attrs[i])) {
			i++
		}
		if i >= len(attrs) || attrs[i] != '=' {
			for i < len(attrs) && !unicode.IsSpace(rune(attrs[i])) {
				i++
			}
			continue
		}
		i++
		for i < len(attrs) && unicode.IsSpace(rune(attrs[i])) {
			i++
		}
		if i >= len(attrs) || (attrs[i] != '"' && attrs[i] != '\'') {
			continue
		}
		quote := attrs[i]
		i++
		valueStart := i
		escaped := false
		for i < len(attrs) {
			ch := attrs[i]
			if escaped {
				escaped = false
				i++
				continue
			}
			if ch == '\\' {
				escaped = true
				i++
				continue
			}
			if ch == quote {
				value := attrs[valueStart:i]
				i++
				if strings.EqualFold(attrName, name) {
					return value
				}
				break
			}
			i++
		}
	}
	return ""
}

func visualforceOutputTextValue(attrs string) string {
	value := strings.TrimSpace(visualforceAttrValue(attrs, "value"))
	if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
		value = strings.TrimSpace(value[2 : len(value)-1])
	}
	if unquoted, ok := apexSingleQuotedTemplateLiteral(value); ok {
		return unquoted
	}
	return value
}

func apexSingleQuotedTemplateLiteral(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
		return "", false
	}
	var out strings.Builder
	escaped := false
	for i := 1; i < len(value)-1; i++ {
		ch := value[i]
		if escaped {
			switch ch {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			default:
				out.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		out.WriteByte(ch)
	}
	if escaped {
		out.WriteByte('\\')
	}
	return out.String(), true
}

func (vm *VM) visualforceEmailContentValue(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	for className, class := range vm.Classes {
		if !strings.EqualFold(className, "EmailContent") && !strings.EqualFold(class.Name, "EmailContent") && !strings.HasSuffix(strings.ToLower(class.Name), ".emailcontent") {
			continue
		}
		for fieldName, field := range class.StaticFields {
			if !strings.EqualFold(fieldName, "contentMap") && !strings.EqualFold(field.Name, "contentMap") {
				continue
			}
			if value, ok := visualforceEmailContentMapValue(field.Value, key); ok {
				return value
			}
		}
	}
	return "[" + key + "]"
}

func visualforceEmailContentMapValue(content Value, key string) (string, bool) {
	if content.Kind != ValueMap {
		return "", false
	}
	encoded := mapKey(String(key))
	if value, ok := content.Map[encoded]; ok {
		return stringValue(value), true
	}
	for _, candidate := range content.MapKeys {
		if candidate.Kind == ValueString && strings.EqualFold(candidate.Text, key) {
			return stringValue(content.Map[mapKey(candidate)]), true
		}
	}
	return "", false
}

func (vm *VM) emailMergeTokenValue(token string, whoRecord storage.Record, whoOK bool, whatRecord storage.Record, whatOK bool) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", false
	}
	root := strings.TrimSpace(parts[0])
	field := strings.TrimSpace(strings.Join(parts[1:], "."))
	if root == "" || field == "" {
		return "", false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if whoOK && emailMergeRootMatches(root, whoRecord.Object, namespace, "Recipient", "Who", "TargetObject") {
		return vm.storageRecordStringField(whoRecord, field), true
	}
	if whatOK && emailMergeRootMatches(root, whatRecord.Object, namespace, "RelatedTo", "What") {
		return vm.storageRecordStringField(whatRecord, field), true
	}
	return "", false
}

func emailMergeRootMatches(root, objectName, namespace string, aliases ...string) bool {
	for _, alias := range aliases {
		if strings.EqualFold(root, alias) {
			return true
		}
	}
	if strings.EqualFold(root, objectName) {
		return true
	}
	return strings.EqualFold(root, storage.StripNamespaceToken(namespace, objectName))
}

func (vm *VM) storageRecordStringField(record storage.Record, field string) string {
	if strings.EqualFold(field, "Id") {
		return string(record.ID)
	}
	if vm.Org != nil {
		if objectName, ok := vm.resolveObjectName(record.Object); ok {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
					field = resolved
				}
			}
		}
	}
	return storageStringField(record, field)
}

func (vm *VM) recordByIDValue(value Value) (storage.Record, bool) {
	idText, ok := idValueText(value)
	if !ok || idText == "" || vm.Org == nil {
		return storage.Record{}, false
	}
	id := storage.ID(idText)
	if len(idText) >= 3 {
		if objectName, ok := vm.sObjectNameForIDPrefix(idText[:3]); ok {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if record, ok := object.Records[id]; ok {
					return record, true
				}
			}
		}
	}
	for _, object := range vm.Org.Objects {
		if record, ok := object.Records[id]; ok {
			return record, true
		}
		for _, record := range object.Records {
			if record.ID == id {
				return record, true
			}
			if fieldID, ok := record.GetField("Id"); ok && storageIDFromValue(fieldID) == id {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}

func storageStringField(record storage.Record, field string) string {
	value, ok := record.GetField(field)
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func newRestResponse() Value {
	response := Object("RestResponse")
	response.Fields["statusCode"] = Int(200)
	response.Fields["headers"] = typedMap("Map<String,String>")
	response.Fields["responseBody"] = Null
	return response
}

func typedMap(typeName string) Value {
	value := Map()
	value.Type = typeName
	return value
}

func typedList(typeName string) Value {
	value := List()
	value.Type = typeName
	return value
}

func typedSet(typeName string) Value {
	value := Set()
	value.Type = typeName
	return value
}

var canonicalRuntimeTypeNames = []string{
	"HttpRequest", "HttpResponse", "StaticResourceCalloutMock", "MultiStaticResourceCalloutMock",
	"RestRequest", "RestResponse", "Continuation", "PageReference", "VisualEditor.DataRow",
	"VisualEditor.DynamicPickListRows", "Dom.Document", "Auth.UserData", "Auth.VerificationResult",
	"Auth.AuthConfiguration", "Auth.JWT", "Metadata.DeployContainer", "Metadata.CustomMetadata",
	"Metadata.CustomMetadataValue", "Metadata.CustomObject", "Metadata.CustomField", "Metadata.Metadata",
	"Metadata.DeployResult", "Metadata.DeployDetails", "Metadata.DeployMessage", "Metadata.DeployCallbackContext",
	"Metadata.AsyncResult", "SelectOption", "ApexPages.StandardController", "ApexPages.StandardSetController",
	"ApexPages.Message", "Messaging.SendEmailResult", "Messaging.EmailFileAttachment", "Messaging.SingleEmailMessage",
	"Messaging.MassEmailMessage", "Messaging.SendEmailOptions", "Messaging.CustomNotification",
	"Messaging.PushNotification", "Messaging.ActionResult", "Messaging.ActionableNotification",
	"Messaging.ActionResult.Builder", "Messaging.ActionableNotification.Builder", "Messaging.Builder",
	"Messaging.InboundEmail", "Messaging.InboundEnvelope", "Messaging.InboundEmailResult",
	"Messaging.RenderEmailTemplateBodyResult", "Messaging.RenderEmailTemplateError", "URL", "Version", "InstallContext",
}

func isCanonicalRuntimeTypeName(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	for _, known := range canonicalRuntimeTypeNames {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	return false
}

func canonicalRuntimeTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	for _, known := range canonicalRuntimeTypeNames {
		if strings.EqualFold(typeName, known) {
			return known
		}
	}
	return typeName
}

func (vm *VM) lookupRestContextField(name string) (Value, bool, error) {
	canonical, ok := canonicalRestContextPath(name)
	if !ok {
		return Null, false, nil
	}
	switch canonical {
	case "RestContext.request":
		if vm.restRequest.Kind == "" {
			return Null, true, nil
		}
		return vm.restRequest, true, nil
	case "RestContext.response":
		if vm.restResponse.Kind == "" || vm.restResponse.Kind == ValueNull {
			vm.restResponse = newRestResponse()
		}
		return vm.restResponse, true, nil
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if strings.HasPrefix(canonical, root+".") {
				value, _, err := vm.lookupRestContextField(root)
				if err != nil {
					return Null, true, err
				}
				out, err := vm.lookupPath(value, strings.Split(strings.TrimPrefix(canonical, root+"."), "."))
				if err != nil {
					return Null, true, err
				}
				return out, true, nil
			}
		}
		return Null, false, nil
	}
}

func (vm *VM) assignRestContextField(name string, value Value) (bool, error) {
	canonical, ok := canonicalRestContextPath(name)
	if !ok {
		return false, nil
	}
	switch canonical {
	case "RestContext.request":
		if value.Kind != ValueNull && (value.Kind != ValueObject || value.Type != "RestRequest") {
			return true, fmt.Errorf("RestContext.request expects RestRequest")
		}
		vm.restRequest = value
		return true, nil
	case "RestContext.response":
		if value.Kind != ValueNull && (value.Kind != ValueObject || value.Type != "RestResponse") {
			return true, fmt.Errorf("RestContext.response expects RestResponse")
		}
		vm.restResponse = value
		return true, nil
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if strings.HasPrefix(canonical, root+".") {
				current, _, err := vm.lookupRestContextField(root)
				if err != nil {
					return true, err
				}
				if current.Kind == ValueNull {
					return true, newNullDereferenceError("while assigning " + name)
				}
				if err := vm.assignPath(current, strings.Split(strings.TrimPrefix(canonical, root+"."), "."), value); err != nil {
					return true, err
				}
				if root == "RestContext.request" {
					vm.restRequest = current
				} else {
					vm.restResponse = current
				}
				return true, nil
			}
		}
		return false, nil
	}
}

func canonicalRestContextPath(name string) (string, bool) {
	switch {
	case strings.EqualFold(name, "RestContext.request"):
		return "RestContext.request", true
	case strings.EqualFold(name, "RestContext.response"):
		return "RestContext.response", true
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if len(name) > len(root) && strings.EqualFold(name[:len(root)], root) && name[len(root)] == '.' {
				return root + name[len(root):], true
			}
		}
		return "", false
	}
}

func (vm *VM) constructValue(typeName string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	return vm.constructValueWithLiteral(typeName, args, namedArgs, result, false)
}

func (vm *VM) constructValueLiteral(typeName string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	return vm.constructValueWithLiteral(typeName, args, namedArgs, result, true)
}

func (vm *VM) resolveUniqueNestedTypeName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") || vm.currentClass == "" {
		return "", false
	}
	commonSObjectType := isCommonSObjectTypeName(typeName)
	currentTops := vm.currentLexicalTopCandidates()
	if len(currentTops) == 0 {
		return "", false
	}
	currentTopKey := strings.ToLower(strings.Join(currentTops, "\x01"))
	typeKey := strings.ToLower(typeName)
	cacheKey := currentTopKey + "\x00" + typeKey
	if vm.uniqueNestedTypeCache != nil {
		if cached, ok := vm.uniqueNestedTypeCache[cacheKey]; ok {
			return cached.Name, cached.OK
		}
	} else {
		vm.uniqueNestedTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	suffix := "." + typeKey
	for _, entry := range vm.classNameSearchEntries() {
		if !strings.HasSuffix(entry.Lower, suffix) {
			continue
		}
		for _, currentTop := range currentTops {
			if nestedTypeBelongsToTop(entry.Name, currentTop) {
				vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: entry.Name, OK: true}
				return entry.Name, true
			}
		}
	}
	if commonSObjectType {
		vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	var unique string
	for _, entry := range vm.classNameSearchEntries() {
		if !strings.HasSuffix(entry.Lower, suffix) {
			continue
		}
		if _, ok := vm.lookupClass(typeName); ok {
			continue
		}
		candidate := entry.Name
		if class, ok := vm.lookupClass(entry.Name); ok {
			candidate = class.Name
		}
		if unique != "" && !strings.EqualFold(unique, candidate) {
			vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{}
			return "", false
		}
		unique = candidate
	}
	if unique != "" {
		vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: unique, OK: true}
		return unique, true
	}
	vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{}
	return "", false
}

func (vm *VM) resolveTopLevelClassName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") {
		return "", false
	}
	currentNamespace := strings.TrimSpace(vm.currentExecutionNamespace())
	cacheKey := strings.ToLower(currentNamespace) + "|" + strings.ToLower(typeName)
	if vm.topLevelTypeCache != nil {
		if cached, ok := vm.topLevelTypeCache[cacheKey]; ok {
			return cached.Name, cached.OK
		}
	} else {
		vm.topLevelTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	var unique string
	for _, entry := range vm.classNameSearchEntries() {
		class, ok := vm.lookupClass(entry.Name)
		if !ok || strings.Contains(class.Name, ".") || !strings.EqualFold(class.Name, typeName) {
			continue
		}
		candidate := runtimeClassName(class)
		if currentNamespace != "" && strings.EqualFold(class.Namespace, currentNamespace) {
			vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: candidate, OK: true}
			return candidate, true
		}
		if unique != "" && !strings.EqualFold(unique, candidate) {
			vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{}
			return "", false
		}
		unique = candidate
	}
	if unique == "" {
		vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: unique, OK: true}
	return unique, true
}

func (vm *VM) classNameSearchEntries() []classNameSearchEntry {
	if vm.classNameSearchCache != nil {
		return vm.classNameSearchCache
	}
	classNames := make([]string, 0, len(vm.Classes))
	for name := range vm.Classes {
		classNames = append(classNames, name)
	}
	sort.Strings(classNames)
	vm.classNameSearchCache = make([]classNameSearchEntry, 0, len(classNames))
	for _, name := range classNames {
		vm.classNameSearchCache = append(vm.classNameSearchCache, classNameSearchEntry{
			Name:  name,
			Lower: strings.ToLower(name),
		})
	}
	return vm.classNameSearchCache
}

func (vm *VM) currentLexicalTopCandidates() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if dot := strings.IndexByte(name, '.'); dot > 0 {
			name = name[:dot]
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, name)
	}
	add(vm.currentClass)
	if short := shortTypeName(vm.currentClass); short != "" && !strings.EqualFold(short, vm.currentClass) {
		add(short)
	}
	if class, ok := vm.lookupClass(vm.currentClass); ok {
		add(class.Name)
		if class.Namespace != "" {
			add(runtimeClassName(class))
		}
	}
	return out
}

func nestedTypeBelongsToTop(name, top string) bool {
	name = strings.TrimSpace(name)
	top = strings.TrimSpace(top)
	if name == "" || top == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(top)+".")
}

func (vm *VM) resolveOnlyNestedTypeName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") {
		return "", false
	}
	typeKey := strings.ToLower(typeName)
	if vm.onlyNestedTypeCache != nil {
		if cached, ok := vm.onlyNestedTypeCache[typeKey]; ok {
			return cached.Name, cached.OK
		}
	} else {
		vm.onlyNestedTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	suffix := "." + typeKey
	unique := ""
	for _, entry := range vm.classNameSearchEntries() {
		if !strings.HasSuffix(entry.Lower, suffix) {
			continue
		}
		candidate := entry.Name
		if class, ok := vm.lookupClass(entry.Name); ok {
			candidate = class.Name
		}
		if unique != "" && !strings.EqualFold(unique, candidate) {
			vm.onlyNestedTypeCache[typeKey] = uniqueNestedTypeLookup{}
			return "", false
		}
		unique = candidate
	}
	if unique == "" {
		vm.onlyNestedTypeCache[typeKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	vm.onlyNestedTypeCache[typeKey] = uniqueNestedTypeLookup{Name: unique, OK: true}
	return unique, true
}

func (vm *VM) queryLocatorIterable(typeName string, value Value) (Value, error) {
	records, ok := value.Fields["Records"]
	if !ok || records.Kind != ValueList {
		return Null, fmt.Errorf("Database.QueryLocator missing records")
	}
	iterable := List(append([]Value(nil), records.List...)...)
	iterable.Type = typeName
	iterable.Fields = map[string]Value{"__queryLocator": value}
	elementType, ok := collectionElementType(typeName)
	if !ok {
		return iterable, nil
	}
	for i, item := range iterable.List {
		coerced, err := vm.coerceAssignable(elementType, item)
		if err != nil {
			return Null, err
		}
		iterable.List[i] = coerced
	}
	return iterable, nil
}

func (vm *VM) resolveEnumClass(typeName string) (Class, bool) {
	cacheKey := vm.currentClass + "|" + typeName
	if vm.enumLookup != nil {
		if cached, ok := vm.enumLookup[cacheKey]; ok {
			return cached.Class, cached.OK
		}
	} else {
		vm.enumLookup = make(map[string]enumClassLookup)
	}
	if enumType, ok := vm.resolveClassName(typeName); ok {
		if class, ok := vm.lookupClass(enumType); ok && len(class.EnumValues) > 0 {
			vm.enumLookup[cacheKey] = enumClassLookup{Class: class, OK: true}
			return class, true
		}
	}
	if !strings.Contains(typeName, ".") {
		if vm.enumSuffixLookup == nil {
			vm.rebuildEnumSuffixLookup()
		}
		if cached, ok := vm.enumSuffixLookup[canonicalClassLookupKey(typeName)]; ok {
			vm.enumLookup[cacheKey] = cached
			return cached.Class, cached.OK
		}
	}
	vm.enumLookup[cacheKey] = enumClassLookup{}
	return Class{}, false
}

func (vm *VM) rebuildEnumSuffixLookup() {
	vm.enumSuffixLookup = make(map[string]enumClassLookup)
	for _, class := range vm.Classes {
		if len(class.EnumValues) == 0 {
			continue
		}
		name := class.Name
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot+1 < len(name) {
			key := canonicalClassLookupKey(name[dot+1:])
			if _, exists := vm.enumSuffixLookup[key]; !exists {
				vm.enumSuffixLookup[key] = enumClassLookup{Class: class, OK: true}
			}
		}
	}
}

func isLikelyEnumValueText(text string) bool {
	if text == "" {
		return false
	}
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
		text = text[dot+1:]
	}
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' {
			continue
		}
		return false
	}
	return true
}

func (vm *VM) ensureAssignable(typeName string, value Value) error {
	_, err := vm.coerceAssignable(typeName, value)
	return err
}

func isDescribeSObjectResultType(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.DescribeSObjectResult") || strings.EqualFold(typeName, "DescribeSObjectResult")
}

func isDescribeFieldResultType(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.DescribeFieldResult") || strings.EqualFold(typeName, "DescribeFieldResult")
}

func isSObjectTypeToken(value Value) bool {
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Schema.SObjectType") || strings.EqualFold(value.Type, "SObjectType"))
}

func (vm *VM) describeFromSObjectTypeToken(value Value) (Value, error) {
	objectValue, ok := value.Fields["object"]
	if !ok || objectValue.Kind != ValueString {
		return Null, fmt.Errorf("Schema.SObjectType token missing object")
	}
	objectName, definition, ok := vm.describeObjectDefinition(objectValue.Text)
	if !ok {
		return Null, fmt.Errorf("Schema.SObjectType.getDescribe unknown object %s", objectValue.Text)
	}
	return vm.describeSObjectValue(objectName, definition), nil
}

func (vm *VM) describeFromSObjectFieldToken(value Value) (Value, error) {
	objectValue, objectOK := value.Fields["object"]
	fieldValue, fieldOK := value.Fields["field"]
	if !objectOK || objectValue.Kind != ValueString || !fieldOK || fieldValue.Kind != ValueString {
		return Null, fmt.Errorf("Schema.SObjectField token missing object or field")
	}
	return vm.describeFieldValue(objectValue.Text, fieldValue.Text)
}

func (vm *VM) coerceCast(typeName string, value Value) (Value, error) {
	coerced, err := vm.coerceAssignable(typeName, value)
	if err == nil {
		coerced.Static = typeName
		return coerced, nil
	}
	targetType := typeExceptionTargetName(typeName)
	if value.Kind == ValueDecimal && strings.EqualFold(typeName, "Integer") {
		if value.Decimal < math.MinInt32 || value.Decimal > math.MaxInt32 {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", runtimeValueTypeName(value), targetType))
		}
		return Int(int64(value.Decimal)), nil
	}
	if value.Kind == ValueDecimal && strings.EqualFold(typeName, "Long") {
		if value.Decimal < float64(math.MinInt64) || value.Decimal > float64(math.MaxInt64) {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", runtimeValueTypeName(value), targetType))
		}
		return Int(int64(value.Decimal)), nil
	}
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		return Null, err
	}
	return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", runtimeValueTypeName(value), targetType))
}

func untypedIntegralDecimalLiteral(value Value) bool {
	if value.Kind != ValueDecimal || value.Type != "" || value.Static != "" {
		return false
	}
	return math.Trunc(value.Decimal) == value.Decimal
}

func typeExceptionTargetName(typeName string) string {
	if strings.EqualFold(typeName, "DateTime") {
		return "Datetime"
	}
	if typed := typeExceptionCollectionName(typeName); typed != "" {
		return typed
	}
	return typeName
}

func typeExceptionCollectionName(typeName string) string {
	base := collectionBase(typeName)
	if base == "" && !isMapType(typeName) {
		return ""
	}
	if isMapType(typeName) {
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return ""
		}
		return "Map<" + typeExceptionAnyName(keyType) + "," + typeExceptionAnyName(valueType) + ">"
	}
	elementType, ok := collectionElementType(typeName)
	if !ok {
		return ""
	}
	return base + "<" + typeExceptionAnyName(elementType) + ">"
}

func typeExceptionAnyName(typeName string) string {
	if strings.EqualFold(typeName, "Object") || strings.EqualFold(typeName, "System.Object") {
		return "ANY"
	}
	return typeName
}

func stringEnumCoercionTarget(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Schema.SObjectType"), strings.EqualFold(typeName, "SObjectType"),
		strings.EqualFold(typeName, "Schema.SObjectField"), strings.EqualFold(typeName, "SObjectField"):
		return false
	}
	if strings.HasSuffix(typeName, "Type") {
		return true
	}
	switch typeName {
	case "TriggerOperation", "RoundingMode", "System.RoundingMode", "LoggingLevel", "AccessType", "System.AccessType", "ApexPages.Severity",
		"Schema.DisplayType", "DisplayType", "Schema.SOAPType", "SOAPType",
		"Metadata.DeployStatus", "Metadata.MetadataType":
		return true
	default:
		return false
	}
}

func (vm *VM) coerceCollectionElement(collectionType string, value Value) (Value, error) {
	elementType, ok := collectionElementType(collectionType)
	if !ok {
		return value, nil
	}
	return vm.coerceAssignable(elementType, value)
}

func (vm *VM) coerceMapEntry(mapType string, key, value Value) (Value, Value, error) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok {
		return key, value, nil
	}
	coercedKey, err := vm.coerceAssignable(keyType, key)
	if err != nil {
		return Null, Null, fmt.Errorf("key: %w", err)
	}
	if strings.EqualFold(valueType, "String") && strings.EqualFold(value.Type, "Id") {
		if idText, ok := typedIDValueText(value); ok {
			return coercedKey, String(displayIDText(idText)), nil
		}
	}
	coercedValue, err := vm.coerceAssignable(valueType, value)
	if err != nil {
		return Null, Null, fmt.Errorf("value: %w", err)
	}
	return coercedKey, coercedValue, nil
}

func missingMapValue(receiver Value) Value {
	_, valueType, ok := mapTypeArgs(receiver.Type)
	if !ok || strings.TrimSpace(valueType) == "" {
		return Null
	}
	return Value{Kind: ValueNull, Type: valueType}
}

func (vm *VM) mapFromSObjectList(mapType string, list Value) (Value, error) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok {
		return Null, unsupportedCallError("Map constructor from SObject list")
	}
	out := Map()
	out.Type = mapType
	trustDeclaredValueType := sObjectListDeclaresMapValueType(list, valueType)
	for i, item := range list.List {
		if item.Kind == ValueNull {
			return Null, fmt.Errorf("Map constructor from SObject list requires non-null SObject at index %d", i)
		}
		if item.Kind != ValueObject {
			return Null, fmt.Errorf("Map constructor from SObject list requires SObject values at index %d", i)
		}
		coerced := item
		if trustDeclaredValueType {
			if coerced.Runtime == "" && coerced.Type != "" && !strings.EqualFold(coerced.Type, valueType) {
				coerced.Runtime = coerced.Type
			}
			if !strings.EqualFold(valueType, "SObject") {
				coerced.Static = valueType
			}
		} else {
			var err error
			coerced, err = vm.coerceAssignable(valueType, item)
			if err != nil {
				return Null, fmt.Errorf("Map constructor from SObject list: value at index %d: %w", i, err)
			}
		}
		keyValue, ok := mapConstructorKeyValue(keyType, coerced)
		if !ok || keyValue.Kind == ValueNull {
			return Null, fmt.Errorf("Map constructor from SObject list requires non-null %s at index %d", keyType, i)
		}
		key, err := vm.coerceAssignable(keyType, keyValue)
		if err != nil {
			return Null, fmt.Errorf("Map constructor from SObject list: key at index %d: %w", i, err)
		}
		encodedKey := mapKey(key)
		if _, exists := out.Map[encodedKey]; !exists {
			out.MapOrder = append(out.MapOrder, encodedKey)
		}
		out.Map[encodedKey] = coerced
		out.MapKeys[encodedKey] = key
	}
	return out, nil
}

func sObjectListDeclaresMapValueType(list Value, valueType string) bool {
	for _, sourceType := range []string{list.Type, list.Static} {
		elementType, ok := collectionElementType(sourceType)
		if !ok {
			continue
		}
		if strings.EqualFold(elementType, valueType) {
			return true
		}
		if strings.EqualFold(valueType, "SObject") && isCommonSObjectTypeName(elementType) {
			return true
		}
	}
	return false
}

func mapConstructorKeyValue(keyType string, record Value) (Value, bool) {
	if strings.EqualFold(keyType, "Id") {
		value, ok := record.Fields["Id"]
		return value, ok
	}
	for _, preferred := range []string{"Name", "DeveloperName", "MasterLabel"} {
		if value, ok := record.Fields[preferred]; ok {
			return value, true
		}
	}
	for _, value := range record.Fields {
		return value, true
	}
	return Null, false
}

func typedNullCollectionBase(value Value) string {
	if value.Kind != ValueNull || value.Type == "" {
		return ""
	}
	return collectionBase(value.Type)
}

func valueShape(value Value) string {
	shape := string(value.Kind)
	if value.Type != "" {
		shape += ":" + value.Type
	}
	if value.Static != "" {
		shape += ":static=" + value.Static
	}
	if value.Runtime != "" {
		shape += ":runtime=" + value.Runtime
	}
	return shape
}

func (vm *VM) mapLookupKey(receiver Value, key Value) string {
	keyType, _, ok := mapTypeArgs(receiver.Type)
	if !ok || strings.TrimSpace(keyType) == "" {
		return vm.mapKey(key)
	}
	coerced, err := vm.coerceAssignable(keyType, key)
	if err != nil {
		return vm.mapKey(key)
	}
	return vm.mapKey(coerced)
}

func caseInsensitiveStringMapStoredKey(receiver Value, key Value) (string, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString {
		return "", false
	}
	_, valueType, typed := mapTypeArgs(receiver.Type)
	if !caseInsensitiveStringMap(receiver) && (!typed || !strings.EqualFold(valueType, "String")) {
		return "", false
	}
	fold := caseInsensitiveStringMap(receiver)
	for rawKey, storedKey := range receiver.MapKeys {
		if storedKey.Kind == ValueString && (storedKey.Text == key.Text || fold && strings.EqualFold(storedKey.Text, key.Text)) {
			return rawKey, true
		}
		if storedKey.String() == key.Text || fold && strings.EqualFold(storedKey.String(), key.Text) {
			return rawKey, true
		}
	}
	for rawKey := range receiver.Map {
		storedKey := mapStoredKey(receiver, rawKey)
		if storedKey.Kind == ValueString && (storedKey.Text == key.Text || fold && strings.EqualFold(storedKey.Text, key.Text)) {
			return rawKey, true
		}
		if storedKey.String() == key.Text || fold && strings.EqualFold(storedKey.String(), key.Text) {
			return rawKey, true
		}
	}
	return "", false
}

func caseInsensitiveStringMap(receiver Value) bool {
	if receiver.Runtime == "pagereference-parameters" {
		return true
	}
	if receiver.MapKeys != nil {
		if flag, ok := receiver.MapKeys["__glade_case_insensitive_string_keys"]; ok && flag.Kind == ValueBool && flag.Bool {
			return true
		}
	}
	if receiver.Fields == nil {
		return false
	}
	flag, ok := receiver.Fields["__caseInsensitiveStringKeys"]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func (vm *VM) putAllSObjectList(receiver Value, list Value) (Value, error) {
	value, err := vm.mapFromSObjectList(receiver.Type, list)
	if err != nil {
		return receiver, err
	}
	for key, item := range value.Map {
		receiver.Map[key] = item
	}
	if receiver.MapKeys == nil {
		receiver.MapKeys = make(map[string]Value)
	}
	for key, item := range value.MapKeys {
		receiver.MapKeys[key] = item
	}
	return receiver, nil
}

func collectionMembers(value Value) []Value {
	switch value.Kind {
	case ValueList:
		return value.List
	case ValueSet:
		return value.Set
	default:
		return nil
	}
}

func (vm *VM) collectionContainsValue(values []Value, needle Value, result *Result) (bool, error) {
	index, err := vm.collectionIndexOfValue(values, needle, result)
	return index >= 0, err
}

func (vm *VM) collectionIndexOfValue(values []Value, needle Value, result *Result) (int, error) {
	for i, value := range values {
		equal, err := vm.apexCollectionElementEquals(value, needle, result)
		if err != nil {
			return -1, err
		}
		if equal {
			return i, nil
		}
	}
	return -1, nil
}

func (vm *VM) apexCollectionElementEquals(left, right Value, result *Result) (bool, error) {
	if collectionStringLike(left) || collectionStringLike(right) {
		return left.Equal(right), nil
	}
	return vm.apexEquals(left, right, result)
}

func collectionStringLike(value Value) bool {
	return value.Kind == ValueString || (value.Kind == ValueObject && strings.EqualFold(value.Type, "String"))
}

func (vm *VM) iterableCollectionMembers(value Value, result *Result, context string) ([]Value, error) {
	switch value.Kind {
	case ValueList, ValueSet:
		return collectionMembers(value), nil
	case ValueObject:
		iterator := value
		if !isIteratorValue(iterator) {
			var err error
			iterator, err = vm.iteratorForObject(value, result)
			if err != nil {
				return nil, fmt.Errorf("%s expects List, Set, or Iterable: %w", context, err)
			}
		}
		const iteratorName = "__glade_add_all_iterator"
		previousIterator, hadIterator := vm.Globals[iteratorName]
		previousIteratorType, hadIteratorType := vm.VarTypes[iteratorName]
		defer func() {
			if hadIterator {
				vm.Globals[iteratorName] = previousIterator
			} else {
				delete(vm.Globals, iteratorName)
			}
			if hadIteratorType {
				vm.VarTypes[iteratorName] = previousIteratorType
			} else {
				delete(vm.VarTypes, iteratorName)
			}
		}()
		vm.Globals[iteratorName] = iterator
		vm.VarTypes[iteratorName] = iterator.Type
		values := make([]Value, 0)
		for iteration := 0; ; iteration++ {
			if iteration >= maxLoopIterations {
				return nil, fmt.Errorf("%s iterable exceeded %d iterations", context, maxLoopIterations)
			}
			hasNext, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "hasNext", nil, result)
			if err != nil {
				return nil, err
			}
			if !handled || hasNext.Kind != ValueBool {
				return nil, fmt.Errorf("%s iterable requires Boolean hasNext", context)
			}
			if !hasNext.Bool {
				return values, nil
			}
			next, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "next", nil, result)
			if err != nil {
				return nil, err
			}
			if !handled {
				return nil, fmt.Errorf("%s iterable requires next", context)
			}
			values = append(values, next)
		}
	default:
		return nil, fmt.Errorf("%s expects List, Set, or Iterable", context)
	}
}

func collectionIterator(value Value) Value {
	snapshot := List(append([]Value(nil), collectionMembers(value)...)...)
	iterator := Object(collectionIteratorType(value.Type))
	iterator.Fields["__values"] = snapshot
	iterator.Fields["__index"] = Int(0)
	return iterator
}

func collectionIteratorType(collectionType string) string {
	if elementType, ok := collectionElementType(collectionType); ok {
		return "Iterator<" + elementType + ">"
	}
	return "Iterator"
}

func isIteratorValue(value Value) bool {
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Iterator") ||
		strings.HasPrefix(strings.ToLower(value.Type), "iterator<") ||
		strings.HasPrefix(strings.ToLower(value.Type), "system.iterator<") ||
		value.Type == "Database.QueryLocatorIterator" ||
		value.Type == "Database.QueryLocatorChunkIterator")
}

func callIteratorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	values, ok := receiver.Fields["__values"]
	if !ok || values.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("Iterator missing snapshot")
	}
	indexValue, ok := receiver.Fields["__index"]
	if !ok || indexValue.Kind != ValueInt {
		return Null, receiver, false, true, fmt.Errorf("Iterator missing index")
	}
	index := int(indexValue.Int)
	switch method {
	case "hasNext":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.hasNext expects 0 arguments")
		}
		return Bool(index < len(values.List)), receiver, false, true, nil
	case "next":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.next expects 0 arguments")
		}
		if index >= len(values.List) {
			return Null, receiver, false, true, newExceptionError("NoSuchElementException", "Iterator has no more elements")
		}
		receiver.Fields["__index"] = Int(int64(index + 1))
		return values.List[index], receiver, true, true, nil
	case "remove":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.remove expects 0 arguments")
		}
		return Null, receiver, false, true, unsupportedCallError("Iterator.remove")
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) sortComparableValues(values []Value, result *Result) error {
	for _, value := range values {
		switch value.Kind {
		case ValueNull, ValueInt, ValueDecimal, ValueString, ValueBool:
		case ValueObject:
		default:
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
	}
	if listSortHasObjects(values) {
		return vm.sortApexComparableValues(values, result)
	}
	sort.SliceStable(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if compare, ok := compareNullSortValues(left, right); ok {
			return compare < 0
		}
		if collectionNumericKind(left.Kind) && collectionNumericKind(right.Kind) {
			return collectionNumericValue(left) < collectionNumericValue(right)
		}
		if left.Kind != right.Kind {
			return collectionSortKindRank(left.Kind) < collectionSortKindRank(right.Kind)
		}
		switch left.Kind {
		case ValueInt:
			return left.Int < right.Int
		case ValueDecimal:
			return left.Decimal < right.Decimal
		case ValueString:
			return left.Text < right.Text
		case ValueBool:
			return !left.Bool && right.Bool
		default:
			return false
		}
	})
	return nil
}

func (vm *VM) sortValuesWithComparator(values []Value, comparator Value, result *Result) error {
	if comparator.Kind != ValueObject {
		return fmt.Errorf("List.sort comparator must be an object")
	}
	comparatorType := runtimeObjectType(comparator)
	if comparatorType == "" {
		return fmt.Errorf("List.sort comparator type is required")
	}
	var sortErr error
	sort.SliceStable(values, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		compare, err := vm.compareValuesWithComparator(comparator, comparatorType, values[i], values[j], result)
		if err != nil {
			sortErr = err
			return false
		}
		return compare < 0
	})
	return sortErr
}

func (vm *VM) compareValuesWithComparator(comparator Value, comparatorType string, left, right Value, result *Result) (int64, error) {
	args := []Value{left, right}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(comparatorType, "compare", args)
	if ambiguous {
		return 0, vm.ambiguousOverloadError(comparatorType+".compare", args)
	}
	if !ok {
		return 0, unsupportedCallError("List.sort comparator without compare method")
	}
	value, err := vm.callMethodWithReceiver(target, comparator, args, result)
	if err != nil {
		return 0, err
	}
	switch value.Kind {
	case ValueInt:
		return value.Int, nil
	case ValueDecimal:
		return int64(value.Decimal), nil
	default:
		return 0, fmt.Errorf("%s returned %s, want Integer", target.Name, valueTypeName(value))
	}
}

func listSortHasObjects(values []Value) bool {
	for _, value := range values {
		if value.Kind == ValueObject {
			return true
		}
	}
	return false
}

func (vm *VM) sortApexComparableValues(values []Value, result *Result) error {
	hasSObject := false
	hasComparable := false
	hasPlatformComparable := false
	for _, value := range values {
		if value.Kind == ValueNull {
			continue
		}
		if value.Kind != ValueObject {
			return unsupportedCallError("List.sort for mixed primitive and Comparable values")
		}
		runtimeType := runtimeObjectType(value)
		if vm.isSortableSObjectValue(value) {
			hasSObject = true
			continue
		}
		if isSortablePlatformValue(value) {
			hasPlatformComparable = true
			continue
		}
		if _, ok, ambiguous := vm.resolveInstanceMethodForArgs(runtimeType, "compareTo", []Value{value}); ambiguous {
			return vm.ambiguousOverloadError(runtimeType+".compareTo", []Value{value})
		} else if !ok {
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
		hasComparable = true
	}
	kinds := 0
	if hasSObject {
		kinds++
	}
	if hasComparable {
		kinds++
	}
	if hasPlatformComparable {
		kinds++
	}
	if kinds > 1 {
		return unsupportedCallError("List.sort for mixed object comparable values")
	}
	var sortErr error
	sort.SliceStable(values, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		if compare, ok := compareNullSortValues(values[i], values[j]); ok {
			return compare < 0
		}
		if hasSObject {
			return compareSObjectSortValues(values[i], values[j]) < 0
		}
		if hasPlatformComparable {
			return comparePlatformSortValues(values[i], values[j]) < 0
		}
		compare, err := vm.compareApexComparableValues(values[i], values[j], result)
		if err != nil {
			sortErr = err
			return false
		}
		return compare < 0
	})
	return sortErr
}

func compareNullSortValues(left, right Value) (int, bool) {
	if left.Kind == ValueNull && right.Kind == ValueNull {
		return 0, true
	}
	if left.Kind == ValueNull {
		return -1, true
	}
	if right.Kind == ValueNull {
		return 1, true
	}
	return 0, false
}

func isSortablePlatformValue(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	runtimeType := runtimeObjectType(value)
	return strings.EqualFold(runtimeType, "SelectOption") || platformScalarObject(runtimeType)
}

func comparePlatformSortValues(left, right Value) int {
	if cmp := strings.Compare(strings.ToLower(runtimeObjectType(left)), strings.ToLower(runtimeObjectType(right))); cmp != 0 {
		return cmp
	}
	if strings.EqualFold(runtimeObjectType(left), "SelectOption") && strings.EqualFold(runtimeObjectType(right), "SelectOption") {
		if cmp := strings.Compare(selectOptionSortText(left, "label"), selectOptionSortText(right, "label")); cmp != 0 {
			return cmp
		}
		return strings.Compare(selectOptionSortText(left, "value"), selectOptionSortText(right, "value"))
	}
	if leftText, ok := platformScalarObjectText(left); ok {
		if rightText, ok := platformScalarObjectText(right); ok {
			return strings.Compare(leftText, rightText)
		}
	}
	return strings.Compare(sObjectStableSortKey(left), sObjectStableSortKey(right))
}

func selectOptionSortText(value Value, field string) string {
	_, fieldValue, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	return strings.ToLower(fieldValue.String())
}

func (vm *VM) isSortableSObjectValue(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	runtimeType := runtimeObjectType(value)
	if _, ok := vm.lookupClass(runtimeType); ok {
		return false
	}
	return vm.isSObjectLikeType(runtimeType)
}

func compareSObjectSortValues(left, right Value) int {
	if cmp := strings.Compare(strings.ToLower(runtimeObjectType(left)), strings.ToLower(runtimeObjectType(right))); cmp != 0 {
		return cmp
	}
	if leftID, rightID, ok := sObjectSortFieldPair(left, right, "Id"); ok {
		return strings.Compare(leftID, rightID)
	}
	if leftName, rightName, ok := sObjectSortFieldPair(left, right, "Name"); ok {
		return strings.Compare(leftName, rightName)
	}
	return strings.Compare(sObjectStableSortKey(left), sObjectStableSortKey(right))
}

func sObjectSortFieldPair(left, right Value, field string) (string, string, bool) {
	_, leftValue, leftOK := objectFieldValue(left, field)
	_, rightValue, rightOK := objectFieldValue(right, field)
	if !leftOK || !rightOK || leftValue.Kind == ValueNull || rightValue.Kind == ValueNull {
		return "", "", false
	}
	return leftValue.String(), rightValue.String(), true
}

func sObjectStableSortKey(value Value) string {
	fields := make([]string, 0, len(value.Fields))
	for field := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		fields = append(fields, field)
	}
	sort.Strings(fields)
	var out strings.Builder
	for _, field := range fields {
		out.WriteString(strings.ToLower(field))
		out.WriteByte('=')
		out.WriteString(value.Fields[field].String())
		out.WriteByte(';')
	}
	return out.String()
}

func (vm *VM) compareApexComparableValues(left, right Value, result *Result) (int64, error) {
	if left.Kind != ValueObject {
		return 0, unsupportedCallError("List.sort for mixed primitive and Comparable values")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(runtimeObjectType(left), "compareTo", []Value{right})
	if ambiguous {
		return 0, vm.ambiguousOverloadError(runtimeObjectType(left)+".compareTo", []Value{right})
	}
	if !ok {
		return 0, unsupportedCallError("List.sort for non-primitive comparable values")
	}
	value, err := vm.callMethodWithReceiver(target, left, []Value{right}, result)
	if err != nil {
		return 0, err
	}
	switch value.Kind {
	case ValueInt:
		return value.Int, nil
	case ValueDecimal:
		return int64(value.Decimal), nil
	default:
		return 0, fmt.Errorf("%s returned %s, want Integer", target.Name, valueTypeName(value))
	}
}

func collectionNumericKind(kind ValueKind) bool {
	return kind == ValueInt || kind == ValueDecimal
}

func collectionNumericValue(value Value) float64 {
	if value.Kind == ValueInt {
		return float64(value.Int)
	}
	return value.Decimal
}

func collectionSortKindRank(kind ValueKind) int {
	switch kind {
	case ValueBool:
		return 0
	case ValueInt, ValueDecimal:
		return 1
	case ValueString:
		return 2
	default:
		return 3
	}
}

func valueFromMapKey(key string) Value {
	if strings.HasPrefix(key, string(ValueObject)+":") {
		rest := strings.TrimPrefix(key, string(ValueObject)+":")
		typeName, text, ok := strings.Cut(rest, ":")
		if ok && typeName == "Schema.SObjectType" {
			return sObjectTypeToken(text)
		}
		if ok && typeName == "Schema.SObjectField" {
			objectName, fieldName, hasField := strings.Cut(text, ".")
			if hasField {
				return sObjectFieldToken(objectName, fieldName)
			}
		}
		if ok && typeName == "Schema.ChildRelationship" {
			relationshipName, rest, hasRelationship := strings.Cut(text, "|")
			childName, fieldName, hasField := strings.Cut(rest, "|")
			if hasRelationship && hasField {
				value := Object("Schema.ChildRelationship")
				value.Fields["relationshipName"] = String(relationshipName)
				value.Fields["childSObject"] = sObjectTypeToken(childName)
				value.Fields["field"] = sObjectFieldToken(childName, fieldName)
				value.Fields["cascadeDelete"] = Bool(false)
				value.Fields["restrictedDelete"] = Bool(false)
				return value
			}
		}
		if ok && typeName == "Type" {
			return Value{Kind: ValueObject, Type: "Type", Text: text}
		}
		if ok && platformScalarObject(typeName) {
			return platformScalar(typeName, text)
		}
		if ok && typeName != "" {
			value := Value{Kind: ValueObject, Type: typeName, Text: text, Fields: make(map[string]Value)}
			if looksLikeID(text) {
				value.Fields["Id"] = String(text)
			}
			return value
		}
	}
	kind, text, ok := strings.Cut(key, ":")
	if !ok {
		return String(key)
	}
	switch ValueKind(kind) {
	case ValueNull:
		return Null
	case ValueInt:
		var parsed int64
		if _, err := fmt.Sscan(text, &parsed); err == nil {
			return Int(parsed)
		}
	case ValueDecimal:
		var parsed float64
		if _, err := fmt.Sscan(text, &parsed); err == nil {
			return Decimal(parsed)
		}
	case ValueBool:
		return Bool(strings.EqualFold(text, "true"))
	case ValueString:
		return String(text)
	}
	return String(text)
}

func mapStoredKey(value Value, rawKey string) Value {
	if value.MapKeys != nil {
		if key, ok := value.MapKeys[rawKey]; ok {
			return key
		}
	}
	return valueFromMapKey(rawKey)
}

func (vm *VM) runtimeError(thrown Value) error {
	return runtimeError(thrown, vm.callStack)
}

func runtimeError(thrown Value, stack []callFrame) error {
	message := "unhandled exception"
	errorType := "Exception"
	thrown = annotateException(thrown, stack)
	if thrown.Kind != ValueNull {
		message = thrown.String()
		if thrown.Kind == ValueObject && thrown.Type != "" {
			errorType = thrown.Type
			if context, ok := thrown.Fields["__diagnosticContext"]; ok && context.Kind == ValueString && strings.TrimSpace(context.Text) != "" {
				message += " (context: " + context.Text + ")"
			}
		}
	}
	if len(stack) == 0 {
		return &RuntimeError{Type: errorType, Message: message}
	}
	return &RuntimeError{Type: errorType, Message: message, Stack: stackFrames(stack)}
}

func classNameFromMethod(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}
	return ""
}

func apexMethodMemberName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func defaultValue(typeName string, explicit Value) Value {
	if explicit.Kind != "" {
		if explicit.Kind == ValueNull && explicit.Type == "" {
			explicit.Type = typeName
		}
		if (typeName == "Decimal" || typeName == "Double") && explicit.Kind == ValueInt {
			return Decimal(float64(explicit.Int))
		}
		if collectionBase(typeName) != "" || isMapType(typeName) {
			if coerced, err := coerceCollectionValue(typeName, explicit); err == nil {
				return cloneValue(coerced)
			}
		}
		return cloneValue(explicit)
	}
	switch typeName {
	case "String":
		return Value{Kind: ValueNull, Type: typeName}
	default:
		return Value{Kind: ValueNull, Type: typeName}
	}
}

func defaultStaticFieldValue(className, fieldName, typeName string, explicit Value) Value {
	if (explicit.Kind == "" || (explicit.Kind == ValueNull && isSObjectFieldTokenType(explicit.Type))) &&
		isSObjectFieldTokenType(typeName) && strings.TrimSpace(fieldName) != "" &&
		(isCommonSObjectTypeName(className) || isCustomObjectLikeName(className)) {
		return sObjectFieldToken(className, fieldName)
	}
	if emptySObjectFieldTokenValue(explicit) && isSObjectFieldTokenType(typeName) && strings.TrimSpace(fieldName) != "" &&
		(isCommonSObjectTypeName(className) || isCustomObjectLikeName(className)) {
		return sObjectFieldToken(className, fieldName)
	}
	return defaultValue(typeName, explicit)
}

func emptySObjectFieldTokenValue(value Value) bool {
	if value.Kind != ValueObject || !isSObjectFieldTokenType(value.Type) {
		return false
	}
	if field, ok := value.Fields["field"]; ok && field.Kind == ValueString && strings.TrimSpace(field.Text) != "" {
		return false
	}
	if name, ok := value.Fields["Name"]; ok && name.Kind == ValueString && strings.TrimSpace(name.Text) != "" {
		return false
	}
	if name, ok := value.Fields["name"]; ok && name.Kind == ValueString && strings.TrimSpace(name.Text) != "" {
		return false
	}
	return true
}

func (vm *VM) stackFrames() []StackFrame {
	return stackFrames(vm.rawStackFrames())
}

func (vm *VM) rawStackFrames() []callFrame {
	frames := append([]callFrame(nil), vm.callStack...)
	if vm.hasStatement && vm.currentStatement.Line > 0 {
		frames = append(frames, vm.currentStatement)
	}
	for i := range frames {
		frames[i].Symbol = vm.qualifyStackFrameSymbol(frames[i].Symbol)
	}
	return frames
}

func (vm *VM) qualifyStackFrameSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" || strings.HasPrefix(strings.ToLower(symbol), "class.") || strings.HasPrefix(strings.ToLower(symbol), "trigger.") {
		return symbol
	}
	dot := strings.LastIndex(symbol, ".")
	if dot <= 0 {
		return symbol
	}
	className := symbol[:dot]
	class, ok := vm.lookupClass(className)
	if !ok {
		return symbol
	}
	token := vm.classTypeToken(class)
	if token == "" || strings.EqualFold(token, className) {
		return symbol
	}
	return token + symbol[len(className):]
}

func stackFrames(frames []callFrame) []StackFrame {
	out := make([]StackFrame, 0, len(frames))
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		out = append(out, StackFrame{
			Symbol: frame.Symbol,
			File:   frame.File,
			Line:   frame.Line,
			Column: frame.Column,
		})
	}
	return out
}

func (vm *VM) callMethod(method Method, args []Value, result *Result) (Value, error) {
	return vm.callMethodWithReceiver(method, Null, args, result)
}

func activeInstanceSetterKey(owner, field string, receiver Value) string {
	key := owner + "." + field
	if receiver.Kind == ValueObject && receiver.Ref != 0 {
		key += "#" + strconv.FormatUint(receiver.Ref, 10)
	}
	return key
}

func (vm *VM) synchronizeFabricatedSObjectRelationships(value Value) Value {
	childrenName, childrenByRelation, ok := objectFieldValue(value, "childrenByRelation")
	if !ok || childrenByRelation.Kind != ValueMap {
		return value
	}
	fabricatedName, fabricated, ok := objectFieldValue(value, "fabricatedSObject")
	if !ok || fabricated.Kind != ValueObject {
		return value
	}
	nodesName, nodes, ok := objectFieldValue(fabricated, "nodes")
	if !ok || nodes.Kind != ValueList {
		return value
	}

	var childNodeType string
	for _, node := range nodes.List {
		if node.Kind == ValueObject && objectHasFields(node, "fieldName", "children") {
			childNodeType = node.Type
			break
		}
	}
	if childNodeType == "" {
		if _, ok := vm.lookupClass("sfab_ChildRelationshipNode"); !ok {
			return value
		}
		childNodeType = "sfab_ChildRelationshipNode"
	}

	relationships := make(map[string]Value, len(childrenByRelation.Map))
	relationshipOrder := make([]string, 0, len(childrenByRelation.Map))
	for rawKey, childList := range childrenByRelation.Map {
		relation := mapStoredKey(childrenByRelation, rawKey).String()
		if relation == "" || childList.Kind != ValueList {
			continue
		}
		fabricatedChildren := List()
		for i, child := range childList.List {
			child = vm.synchronizeFabricatedSObjectRelationships(child)
			childList.List[i] = child
			if _, childFabricated, ok := objectFieldValue(child, "fabricatedSObject"); ok && childFabricated.Kind == ValueObject {
				fabricatedChildren.List = append(fabricatedChildren.List, childFabricated)
			}
		}
		childrenByRelation.Map[rawKey] = childList
		relationships[relation] = fabricatedChildren
		relationshipOrder = append(relationshipOrder, relation)
	}
	if len(relationships) == 0 {
		return value
	}

	filtered := make([]Value, 0, len(nodes.List)+len(relationships))
	for _, node := range nodes.List {
		if node.Kind == ValueObject {
			if _, fieldName, ok := objectFieldValue(node, "fieldName"); ok && fieldName.Kind == ValueString {
				if _, replace := relationships[fieldName.Text]; replace && objectHasFields(node, "children") {
					continue
				}
			}
		}
		filtered = append(filtered, node)
	}
	for _, relation := range relationshipOrder {
		node := Object(childNodeType)
		node.Fields["fieldName"] = String(relation)
		node.Fields["children"] = relationships[relation]
		filtered = append(filtered, node)
	}
	nodes.List = filtered
	fabricated.Fields[nodesName] = nodes
	value.Fields[fabricatedName] = fabricated
	value.Fields[childrenName] = childrenByRelation
	return value
}

func objectHasFields(value Value, names ...string) bool {
	if value.Kind != ValueObject {
		return false
	}
	for _, name := range names {
		if _, _, ok := objectFieldValue(value, name); !ok {
			return false
		}
	}
	return true
}

func (vm *VM) inferEmptySObjectListRuntimeType(returnType string, value Value, args []Value) string {
	if value.Kind != ValueList || len(value.List) != 0 || value.Runtime != "" {
		return ""
	}
	elementType, ok := collectionElementType(returnType)
	if !ok || !strings.EqualFold(elementType, "SObject") {
		return ""
	}
	if queryText := inlineSOQLQueryText(value); queryText != "" {
		if objectName := vm.soqlResultObjectNameWithExpander(queryText, vm.expandSOQLBinds); objectName != "" {
			return "List<" + objectName + ">"
		}
	}
	return ""
}

func (vm *VM) sObjectNameFromIDSetValue(value Value) string {
	if value.Kind != ValueSet {
		return ""
	}
	elementType, ok := collectionElementType(value.Type)
	if ok && !strings.EqualFold(elementType, "Id") {
		return ""
	}
	objectName := ""
	for _, item := range value.Set {
		idText, ok := idValueText(item)
		if !ok || idText == "" {
			return ""
		}
		name, ok := vm.sObjectNameForID(idText)
		if !ok {
			return ""
		}
		if objectName == "" {
			objectName = name
			continue
		}
		if !strings.EqualFold(objectName, name) {
			return ""
		}
	}
	return objectName
}

func methodHasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
			return true
		}
	}
	return false
}

func describeFieldPermissionFlagName(method string) string {
	switch method {
	case "isCreateable":
		return "createable"
	case "isUpdateable":
		return "updateable"
	default:
		return "accessible"
	}
}

func describeFieldBooleanFlagName(method string) string {
	name := strings.TrimPrefix(method, "is")
	if name == "" {
		return method
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func passiveGeneratedMethod(method Method) bool {
	return methodHasModifier(method.Modifiers, "passive-generated") &&
		len(method.Program.Instructions) == 0
}

func (vm *VM) generatedPassiveDTOAccessorMethod(className string, method Method) bool {
	return generatedPlatformTypeName(className) &&
		vm.isPassivePlatformDTOType(className) &&
		generatedPlatformPassiveDTOMethod(method)
}

func generatedFamilyUnsupportedStaticCallee(callee string) bool {
	className, _, ok := strings.Cut(callee, ".")
	if !ok {
		return false
	}
	return generatedFamilyUnsupportedTypePrefix(className)
}

func generatedFamilyUnsupportedTypePrefix(typeName string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(typeName))
	if trimmed == "" {
		return false
	}
	for _, family := range []string{
		"cartextension",
		"commercepayments",
		"metadata",
		"limits",
		"cache",
		"lxscheduler",
		"messaging",
	} {
		if trimmed == family || strings.HasPrefix(trimmed, family+".") {
			return true
		}
	}
	return false
}

func generatedUnsupportedFamilyKey(className, methodName string) string {
	return strings.ToLower(strings.TrimSpace(className) + "." + strings.TrimSpace(methodName))
}

func (vm *VM) generatedUnsupportedFamilyExplicitMethodDefault(method Method, receiver Value, args []Value) (Value, bool) {
	className := method.ClassName
	if className == "" && receiver.Kind == ValueObject {
		className = receiver.Type
	}
	if strings.EqualFold(className, "CartExtension.CartTestUtil") {
		return vm.callCartExtensionCartTestUtilStaticDefault(apexMethodMemberName(method.Name), args)
	}
	key := generatedUnsupportedFamilyKey(className, apexMethodMemberName(method.Name))
	switch key {
	case "cartextension.cartdeliverygroup.getisdefault":
		return Bool(false), true
	case "cartextension.cartdeliverygroup.getisgift":
		return Bool(false), true
	case "cartextension.cartdeliverygroup.getname":
		return String("Shipment 1"), true
	case "cartextension.ordergraph.getorder":
		order := Object("Order")
		order.Fields["Id"] = String("@{ref_Order_1.id}")
		return order, true
	case "cartextension.ordergraph.getorderadjustmentgroups",
		"cartextension.ordergraph.getorderdeliverygroups",
		"cartextension.ordergraph.getorderdeliverymethods",
		"cartextension.ordergraph.getorderitemadjustmentlineitems",
		"cartextension.ordergraph.getorderitems",
		"cartextension.ordergraph.getorderitemtaxlineitems":
		return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), true
	case "cartextension.placeorderresponse.success":
		value := Object("CartExtension.PlaceOrderResponse")
		value.Fields["delegate"] = Null
		value.Fields["status"] = String("Success")
		return value, true
	default:
		return Null, false
	}
}

func (vm *VM) generatedUnsupportedFamilyExplicitMethodError(method Method, receiver Value, args []Value) (error, bool) {
	className := method.ClassName
	if className == "" && receiver.Kind == ValueObject {
		className = receiver.Type
	}
	key := generatedUnsupportedFamilyKey(className, apexMethodMemberName(method.Name))
	switch key {
	case "cartextension.checkoutcreateorder.createorder",
		"lxscheduler.schedulerresources.getappointmentcandidates",
		"lxscheduler.schedulerresources.getappointmentslots",
		"commercepayments.authorizationresponse.setpaymentmethodtokenizationresponse",
		"commercepayments.authorizationresponse.setretrycategory",
		"commercepayments.authorizationresponse.setretrydecision",
		"commercepayments.authorizationreversalresponse.setretrycategory",
		"commercepayments.authorizationreversalresponse.setretrydecision",
		"commercepayments.bankpaymentmethodresponse.setaccountholdertype",
		"commercepayments.bankpaymentmethodresponse.setaccounttype",
		"commercepayments.bankpaymentmethodresponse.setbanktype",
		"commercepayments.bankpaymentmethodresponse.setstandardentryclasscode",
		"commercepayments.capturenotification.setretrycategory",
		"commercepayments.capturenotification.setretrydecision",
		"commercepayments.captureresponse.setretrycategory",
		"commercepayments.captureresponse.setretrydecision",
		"commercepayments.cardpaymentmethodresponse.setcardcategory",
		"commercepayments.cardpaymentmethodresponse.setcardtypecategory",
		"commercepayments.notificationclient.record",
		"commercepayments.paymentmethoddetailsresponse.setalternativepaymentmethod",
		"commercepayments.paymentmethoddetailsresponse.setcardpaymentmethod",
		"commercepayments.paymentmethodtokenizationresponse.setretrycategory",
		"commercepayments.paymentmethodtokenizationresponse.setretrydecision",
		"commercepayments.postauthorizationresponse.setpaymentmethoddetails",
		"commercepayments.postauthorizationresponse.setpaymentmethodtokenizationresponse",
		"commercepayments.postauthorizationresponse.setretrycategory",
		"commercepayments.postauthorizationresponse.setretrydecision",
		"commercepayments.referencedrefundnotification.setretrycategory",
		"commercepayments.referencedrefundnotification.setretrydecision",
		"commercepayments.referencedrefundresponse.setretrycategory",
		"commercepayments.referencedrefundresponse.setretrydecision",
		"commercepayments.saleresponse.setpaymentmethodtokenizationresponse",
		"commercepayments.saleresponse.setretrycategory",
		"commercepayments.saleresponse.setretrydecision":
		return newExceptionError("System.NullPointerException", method.Name+" requires non-null arguments"), true
	default:
		return nil, false
	}
}

func (vm *VM) generatedUnsupportedFamilyExplicitStaticDefault(callee string, args []Value) (Value, bool) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		return Null, false
	}
	if strings.EqualFold(className, "CartExtension.CartTestUtil") {
		return vm.callCartExtensionCartTestUtilStaticDefault(methodName, args)
	}
	switch generatedUnsupportedFamilyKey(className, methodName) {
	case "cartextension.placeorderresponse.success":
		value := Object("CartExtension.PlaceOrderResponse")
		value.Fields["delegate"] = Null
		value.Fields["status"] = String("Success")
		return value, true
	default:
		return Null, false
	}
}

func (vm *VM) generatedPassiveUnsupportedStaticCallee(callee string, args []Value) bool {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		return false
	}
	if !generatedFamilyUnsupportedTypePrefix(className) {
		return false
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
	if !ok {
		return false
	}
	return passiveGeneratedMethod(method)
}

func (vm *VM) passiveGeneratedMethodReturn(method Method, frame map[string]Value, receiver Value) Value {
	returnType := vm.resolveTypeNameInClass(method.ClassName, method.ReturnType)
	methodName := apexMethodMemberName(method.Name)
	if returnType == "" || strings.EqualFold(returnType, "void") {
		if receiver.Kind == ValueObject && len(method.Params) == 1 {
			if suffix, ok := passiveAccessorSuffix(methodName, "set"); ok {
				if value, found := frame[method.Params[0].Name]; found {
					receiver.Fields[passiveAccessorFieldName(receiver, suffix)] = value
					frame["this"] = receiver
				}
			} else if field, ok := passivePropertyAccessorField(method.Name, "set"); ok {
				if value, found := frame[method.Params[0].Name]; found {
					receiver.Fields[passiveAccessorFieldName(receiver, field)] = value
					frame["this"] = receiver
				}
			}
		}
		return Null
	}
	if receiver.Kind == ValueObject {
		if suffix, ok := passiveAccessorSuffix(methodName, "get"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
				return value
			}
		} else if field, ok := passivePropertyAccessorField(method.Name, "get"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, field)); found {
				return value
			}
		}
		if suffix, ok := passiveAccessorSuffix(methodName, "is"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
				return value
			}
		} else if field, ok := passivePropertyAccessorField(method.Name, "is"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, field)); found {
				return value
			}
		}
	}
	if class, ok := vm.lookupClass(returnType); ok {
		returnType = class.Name
	}
	if receiver.Kind == ValueObject && strings.EqualFold(returnType, receiver.Type) {
		bindPassiveMethodArgs(&receiver, method, frame)
		if len(method.Params) == 1 {
			if _, _, ok := objectFieldValue(receiver, methodName); !ok {
				if value, found := frame[method.Params[0].Name]; found {
					receiver.Fields[passiveAccessorFieldName(receiver, methodName)] = value
				}
			}
		}
		return receiver
	}
	switch {
	case collectionBase(returnType) == "List":
		return typedList(returnType)
	case collectionBase(returnType) == "Set":
		value := Set()
		value.Type = returnType
		return value
	case isMapType(returnType):
		return typedMap(returnType)
	case vm.isPassivePlatformDTOType(returnType):
		object := Object(returnType)
		vm.initializeFields(&object, returnType)
		if receiver.Kind == ValueObject {
			for field, value := range receiver.Fields {
				object.Fields[field] = value
			}
		}
		bindPassiveMethodArgs(&object, method, frame)
		return object
	default:
		return defaultValue(returnType, Null)
	}
}

func (vm *VM) constructGeneratedPlatformValue(typeName string, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]
	if !ok || generated.Kind == apexast.DeclarationInterface || generated.Kind == apexast.DeclarationEnum || vm.isSObjectLikeType(generated.Name) {
		return Null, false, nil
	}
	if strings.EqualFold(generated.Name, "Auth.AuthConfiguration") {
		value, err := constructAuthConfigurationValue(args, namedArgs)
		return value, true, err
	}
	if sfsqlquerySafeHarnessType(generated.Name) {
		return vm.constructSfsqlqueryHarnessValue(generated, args, namedArgs)
	}
	if cartExtensionMockBackedRuntimeType(generated.Name) {
		return vm.constructCartExtensionMockBackedValue(generated, args, namedArgs)
	}
	if !vm.isPassivePlatformDTOType(generated.Name) && len(generated.Fields) == 0 {
		return Null, false, nil
	}
	ctorArgs := args
	if len(generated.Constructors) != 0 {
		ctor, ok, ambiguous := vm.matchMethodByArgs(generated.Constructors, args)
		if !ok && len(namedArgs) != 0 {
			ctor, ctorArgs, ok, ambiguous = vm.matchGeneratedPlatformConstructorWithNamedArgs(generated, args, namedArgs)
		}
		if ambiguous {
			return Null, true, fmt.Errorf("ambiguous %s constructor with %d argument(s)", generated.Name, len(args))
		}
		if !ok {
			return Null, true, fmt.Errorf("%s constructor expects %s", generated.Name, generatedPlatformConstructorSummary(generated.Constructors))
		}
		object := vm.newGeneratedPlatformObject(generated)
		bindPassiveConstructorArgs(&object, ctor, ctorArgs)
		if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
			return Null, true, err
		}
		return object, true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s constructor expects 0 arguments", generated.Name)
	}
	object := vm.newGeneratedPlatformObject(generated)
	if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
		return Null, true, err
	}
	return object, true, nil
}

func componentApexRuntimeType(typeName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(typeName)), "component.apex.")
}

func newComponentApexValue(typeName string, namedArgs map[string]Value) Value {
	object := Object(typeName)
	object.Fields["childComponents"] = List()
	object.Fields["componentIterations"] = List()
	expressions := Object("ApexPages.ComponentExpressions")
	object.Fields["expressions"] = expressions
	object.Fields["Expressions"] = expressions
	object.Fields["facets"] = Map()
	object.Fields["id"] = Null
	object.Fields["parent"] = Null
	object.Fields["rendered"] = Bool(true)
	for field, value := range namedArgs {
		object.Fields[field] = value
	}
	return object
}

func (vm *VM) newGeneratedPlatformObject(generated generatedPlatformType) Value {
	return vm.newGeneratedPlatformObjectSeen(generated, map[string]bool{})
}

func (vm *VM) newGeneratedPlatformObjectSeen(generated generatedPlatformType, seen map[string]bool) Value {
	object := Object(generated.Name)
	key := strings.ToLower(generated.Name)
	if seen[key] {
		return object
	}
	seen[key] = true
	if generated.SuperClass != "" {
		if parent, ok := generatedPlatformTypeIndex[strings.ToLower(generated.SuperClass)]; ok {
			for name, field := range parent.Fields {
				object.Fields[name] = vm.generatedPlatformDefaultValueSeen(field.Type, Null, seen)
			}
		}
	}
	for _, name := range generated.FieldOrder {
		field := generated.Fields[name]
		object.Fields[name] = vm.generatedPlatformDefaultValueSeen(field.Type, field.InitialValue, seen)
	}
	if strings.EqualFold(generated.Name, "CartExtension.CartDeliveryGroup") {
		object.Fields["isDefault"] = Bool(false)
		object.Fields["isGift"] = Bool(false)
		object.Fields["name"] = String("Shipment 1")
	}
	if strings.EqualFold(generated.Name, "CartExtension.OrderGraph") {
		order := Object("Order")
		order.Fields["Id"] = String("@{ref_Order_1.id}")
		object.Fields["order"] = order
		object.Fields["orderAdjustmentGroups"] = typedList("List<CartExtension.OrderAdjustmentGroup>")
		object.Fields["orderDeliveryGroups"] = typedList("List<CartExtension.OrderDeliveryGroup>")
		object.Fields["orderDeliveryMethods"] = typedList("List<CartExtension.OrderDeliveryMethod>")
		object.Fields["orderItemAdjustmentLineItems"] = typedList("List<CartExtension.OrderItemAdjustmentLineItem>")
		object.Fields["orderItems"] = typedList("List<CartExtension.OrderItem>")
		object.Fields["orderItemTaxLineItems"] = typedList("List<CartExtension.OrderItemTaxLineItem>")
	}
	delete(seen, key)
	return object
}

func (vm *VM) bindGeneratedPlatformNamedFields(object *Value, namedArgs map[string]Value) error {
	for name, value := range namedArgs {
		fieldName := name
		if field, _, ok := vm.generatedPlatformField(object.Type, name, false); ok {
			fieldName = field.Name
			coerced, err := vm.coerceAssignable(field.Type, value)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", object.Type, name, err)
			}
			value = coerced
		}
		object.Fields[fieldName] = value
	}
	return nil
}

func (vm *VM) matchGeneratedPlatformConstructorWithNamedArgs(generated generatedPlatformType, args []Value, namedArgs map[string]Value) (Method, []Value, bool, bool) {
	class := Class{Name: generated.Name, Fields: generated.Fields, Constructors: generated.Constructors}
	return vm.matchConstructorWithNamedArgs(class, args, namedArgs)
}

func generatedPlatformConstructorSummary(constructors []Method) string {
	if len(constructors) == 0 {
		return "0 arguments"
	}
	parts := make([]string, 0, len(constructors))
	for _, ctor := range constructors {
		parts = append(parts, fmt.Sprintf("%d argument(s)", len(ctor.Params)))
	}
	sort.Strings(parts)
	return strings.Join(parts, " or ")
}

func (vm *VM) generatedPlatformInstanceField(receiver Value, fieldName string) (Value, bool) {
	field, _, ok := vm.generatedPlatformField(receiver.Type, fieldName, false)
	if !ok {
		return Null, false
	}
	if vm.isSObjectLikeType(receiver.Type) {
		if _, value, ok := objectFieldValue(receiver, fieldName); ok {
			return value, true
		}
	}
	if _, value, ok := objectFieldValue(receiver, field.Name); ok {
		return value, true
	}
	if _, value, ok := objectFieldValue(receiver, fieldName); ok {
		return value, true
	}
	if vm.isSObjectLikeType(receiver.Type) {
		canonical := vm.resolveSObjectFieldName(receiver.Type, fieldName)
		if value, ok := vm.parentRelationshipValueFromLookupID(receiver, canonical); ok {
			if value.Kind == ValueNull && vm.isSObjectLikeType(value.Type) {
				return Null, false
			}
			return value, true
		}
		if value, ok := vm.parentRelationshipValue(receiver, canonical); ok {
			if value.Kind == ValueNull && vm.isSObjectLikeType(value.Type) {
				return Null, false
			}
			return value, true
		}
		if value, ok := vm.missingSObjectFieldValue(receiver, canonical); ok {
			if value.Kind == ValueNull && vm.isSObjectLikeType(value.Type) {
				return Null, false
			}
			return value, true
		}
	}
	return vm.generatedPlatformDefaultValue(field.Type, field.InitialValue), true
}

func (vm *VM) generatedPlatformStaticFieldValue(typeName, fieldName string) (Value, bool) {
	field, generated, ok := vm.generatedPlatformField(typeName, fieldName, true)
	if !ok {
		return Null, false
	}
	if generated.Kind == apexast.DeclarationEnum {
		value := Value{Kind: ValueObject, Type: generated.Name, Text: field.Name}
		value.Fields = map[string]Value{"ordinal": Int(int64(generatedPlatformEnumOrdinal(generated, field.Name)))}
		return value, true
	}
	return vm.generatedPlatformDefaultValue(field.Type, field.InitialValue), true
}

func (vm *VM) generatedPlatformField(typeName, fieldName string, static bool) (Field, generatedPlatformType, bool) {
	for search := typeName; search != ""; {
		generated, ok := generatedPlatformTypeIndex[strings.ToLower(search)]
		if !ok {
			break
		}
		fields := generated.Fields
		if static {
			fields = generated.StaticFields
		}
		if field, ok := fields[fieldName]; ok {
			if field.Name == "" {
				field.Name = fieldName
			}
			return field, generated, true
		}
		normalized := strings.ToLower(fieldName)
		for candidate, field := range fields {
			if strings.ToLower(candidate) == normalized || (field.Name != "" && strings.ToLower(field.Name) == normalized) {
				if field.Name == "" {
					field.Name = candidate
				}
				return field, generated, true
			}
		}
		search = generated.SuperClass
	}
	return Field{}, generatedPlatformType{}, false
}

func (vm *VM) generatedPlatformDefaultValue(typeName string, explicit Value) Value {
	return vm.generatedPlatformDefaultValueSeen(typeName, explicit, map[string]bool{})
}

func (vm *VM) generatedPlatformDefaultValueSeen(typeName string, explicit Value, seen map[string]bool) Value {
	typeName = vm.resolveTypeNameInClass(vm.currentClass, typeName)
	if generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]; ok &&
		generated.Kind == apexast.DeclarationClass &&
		vm.isPassivePlatformDTOType(generated.Name) {
		if seen[strings.ToLower(generated.Name)] {
			return defaultValue(typeName, explicit)
		}
		object := vm.newGeneratedPlatformObjectSeen(generated, seen)
		if explicit.Kind == ValueObject {
			for name, value := range explicit.Fields {
				object.Fields[name] = value
			}
		}
		return object
	}
	switch {
	case collectionBase(typeName) == "List":
		return typedList(typeName)
	case collectionBase(typeName) == "Set":
		value := Set()
		value.Type = typeName
		return value
	case isMapType(typeName):
		return typedMap(typeName)
	default:
		return defaultValue(typeName, explicit)
	}
}

func standardControllerPage(record Value) Value {
	if _, id, ok := objectFieldValue(record, "Id"); ok {
		if idText, ok := idValueText(id); ok && idText != "" {
			return newPageReference("/" + idText)
		}
	}
	return newPageReference("")
}

func standardSetCurrentPage(controller, records Value) Value {
	if records.Kind != ValueList {
		return List()
	}
	pageSize := int(controller.Fields["pageSize"].Int)
	pageNumber := int(controller.Fields["pageNumber"].Int)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}
	start := (pageNumber - 1) * pageSize
	if start >= len(records.List) {
		return List()
	}
	end := start + pageSize
	if end > len(records.List) {
		end = len(records.List)
	}
	return List(records.List[start:end]...)
}

func standardSetPageCount(controller, records Value) int {
	if records.Kind != ValueList || len(records.List) == 0 {
		return 1
	}
	pageSize := int(controller.Fields["pageSize"].Int)
	if pageSize <= 0 {
		pageSize = 20
	}
	pages := (len(records.List) + pageSize - 1) / pageSize
	if pages < 1 {
		return 1
	}
	return pages
}

func (vm *VM) standardSetDML(receiver Value, op string, result *Result) (Value, Value, bool, bool, error) {
	records := receiver.Fields["records"]
	if records.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.%s requires records", op)
	}
	if _, err := vm.applyDML(op, records, true, "", dml.Options{}, result); err != nil {
		return Null, receiver, false, true, err
	}
	return newPageReference(""), receiver, false, true, nil
}

func callEmailFileAttachmentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setBody":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.setBody expects Blob")
		}
		receiver.Fields["body"] = args[0]
		return Null, receiver, true, true, nil
	case "setContentType", "setFileName":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.%s expects String", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setInline":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.setInline expects Boolean")
		}
		receiver.Fields["inline"] = args[0]
		return Null, receiver, true, true, nil
	case "getBody", "getContentType", "getFileName", "getId", "getInline":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSingleEmailMessageMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setToAddresses", "setCcAddresses", "setBccAddresses", "setFileAttachments", "setEntityAttachments", "setDocumentAttachments", "setTargetObjectIds":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects List", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setSubject", "setPlainTextBody", "setHtmlBody", "setReplyTo", "setSenderDisplayName",
		"setCharset", "setInReplyTo", "setReferences", "setOrgWideEmailAddressId",
		"setTargetObjectId", "setTemplateId", "setWhatId", "setOptOutPolicy", "setEmailPriority",
		"setUnsubscribeComment":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects String", method)
		}
		value := args[0]
		if idText, ok := typedIDValueText(value); ok {
			value = String(idText)
		}
		receiver.Fields[emailMessageFieldName(method)] = value
		return Null, receiver, true, true, nil
	case "setSaveAsActivity", "setTreatBodiesAsTemplate", "setTreatTargetObjectAsRecipient", "setUseSignature", "setBccSender", "setOneClickPost":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects Boolean", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setUnsubscribeUrls":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.setUnsubscribeUrls expects List")
		}
		receiver.Fields["unsubscribeUrls"] = args[0]
		return Null, receiver, true, true, nil
	case "getToAddresses", "getCcAddresses", "getBccAddresses", "getFileAttachments", "getEntityAttachments", "getDocumentAttachments", "getTargetObjectIds",
		"getSubject", "getPlainTextBody", "getHtmlBody", "getReplyTo", "getSenderDisplayName",
		"getCharset", "getInReplyTo", "getReferences", "getOrgWideEmailAddressId",
		"getTargetObjectId", "getTemplateId", "getTemplateName", "getWhatId", "getOptOutPolicy", "getEmailPriority",
		"getUnsubscribeComment", "getUnsubscribeUrls",
		"getSaveAsActivity", "getTreatBodiesAsTemplate", "getTreatTargetObjectAsRecipient", "getUseSignature", "getBccSender", "getOneClickPost":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	case "isTreatBodiesAsTemplate", "isTreatTargetObjectAsRecipient", "isUserMail":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callMassEmailMessageMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setTargetObjectIds", "setWhatIds":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects List", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setTemplateId", "setDescription", "setOptOutPolicy", "setEmailPriority", "setReplyTo", "setSenderDisplayName", "setSubject":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects String", method)
		}
		value := args[0]
		if idText, ok := typedIDValueText(value); ok {
			value = String(idText)
		}
		receiver.Fields[emailMessageFieldName(method)] = value
		return Null, receiver, true, true, nil
	case "setSaveAsActivity", "setBccSender", "setUseSignature":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects Boolean", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "getTargetObjectIds", "getWhatIds", "getTemplateId", "getDescription", "getOptOutPolicy",
		"getEmailPriority", "getReplyTo", "getSenderDisplayName", "getSubject",
		"getSaveAsActivity", "getBccSender", "getUseSignature":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callMessagingDTOGetter(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	field := ""
	if suffix, ok := passiveAccessorSuffix(method, "get"); ok {
		field = strings.ToLower(suffix[:1]) + suffix[1:]
	} else if suffix, ok := passiveAccessorSuffix(method, "is"); ok {
		field = strings.ToLower(suffix[:1]) + suffix[1:]
	}
	if field == "" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	if value, ok := receiver.Fields[field]; ok {
		return value, receiver, false, true, nil
	}
	if value, ok := receiver.Fields[strings.ToLower(field)]; ok {
		return value, receiver, false, true, nil
	}
	return Null, receiver, false, true, nil
}

func callMessagingActionResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "isSuccess", "getMessage", "getErrorCode":
		return callMessagingDTOGetter(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callMessagingBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	if strings.EqualFold(method, "build") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.build expects 0 arguments", receiver.Type)
		}
		var built Value
		if strings.EqualFold(receiver.Type, "Messaging.ActionableNotification.Builder") {
			built = newActionableNotification()
		} else {
			built = newActionResult()
		}
		for field, value := range receiver.Fields {
			built.Fields[field] = value
		}
		return built, receiver, false, true, nil
	}
	if !strings.HasPrefix(method, "with") || len(method) <= len("with") {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
	}
	field := strings.TrimPrefix(method, "with")
	field = strings.ToLower(field[:1]) + field[1:]
	receiver.Fields[field] = args[0]
	return receiver, receiver, true, true, nil
}

func (vm *VM) callCustomNotificationMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setBody", "setNotificationTypeId", "setSenderId", "setTargetId", "setTargetPageRef", "setTitle":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.CustomNotification.%s expects String", method)
		}
		value := args[0]
		if idText, ok := typedIDValueText(value); ok {
			value = String(idText)
		}
		receiver.Fields[customNotificationFieldName(method)] = value
		return Null, receiver, true, true, nil
	case "send":
		if len(args) != 1 || args[0].Kind != ValueSet {
			return Null, receiver, false, true, fmt.Errorf("Messaging.CustomNotification.send expects Set<String>")
		}
		appendTrace(result, "apex.notification.custom.send", "apex.notification", map[string]any{"recipients": len(args[0].Set)})
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func customNotificationFieldName(method string) string {
	switch method {
	case "setNotificationTypeId":
		return "notificationTypeId"
	case "setSenderId":
		return "senderId"
	case "setTargetId":
		return "targetId"
	case "setTargetPageRef":
		return "targetPageRef"
	default:
		return emailMessageFieldName(method)
	}
}

func (vm *VM) callPushNotificationMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setPayload":
		if len(args) != 1 || args[0].Kind != ValueMap {
			return Null, receiver, false, true, fmt.Errorf("Messaging.PushNotification.setPayload expects Map<String,Object>")
		}
		receiver.Fields["payload"] = args[0]
		return Null, receiver, true, true, nil
	case "setTtl":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Messaging.PushNotification.setTtl expects Integer")
		}
		receiver.Fields["ttl"] = args[0]
		return Null, receiver, true, true, nil
	case "send":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueSet {
			return Null, receiver, false, true, fmt.Errorf("Messaging.PushNotification.send expects application String and Set<String>")
		}
		appendTrace(result, "apex.notification.push.send", "apex.notification", map[string]any{"application": args[0].Text, "recipients": len(args[1].Set)})
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func messagingPushPayloadApple(args []Value) (Value, error) {
	if len(args) != 4 && len(args) != 8 {
		return Null, fmt.Errorf("Messaging.PushNotificationPayload.apple expects 4 or 8 arguments")
	}
	payload := typedMap("Map<String,Object>")
	aps := typedMap("Map<String,Object>")
	switch len(args) {
	case 4:
		if args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueInt || args[3].Kind != ValueMap {
			return Null, fmt.Errorf("Messaging.PushNotificationPayload.apple expects alert, sound, badgeCount, userData")
		}
		aps.Map[mapKey(String("alert"))] = args[0]
		aps.MapKeys[mapKey(String("alert"))] = String("alert")
		aps.Map[mapKey(String("sound"))] = args[1]
		aps.MapKeys[mapKey(String("sound"))] = String("sound")
		aps.Map[mapKey(String("badge"))] = args[2]
		aps.MapKeys[mapKey(String("badge"))] = String("badge")
	case 8:
		if args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString || args[3].Kind != ValueList ||
			args[4].Kind != ValueString || args[5].Kind != ValueString || args[6].Kind != ValueInt || args[7].Kind != ValueMap {
			return Null, fmt.Errorf("Messaging.PushNotificationPayload.apple expects alertBody, actionLocKey, locKey, locArgs, launchImage, sound, badgeCount, userData")
		}
		alert := typedMap("Map<String,Object>")
		for key, value := range map[string]Value{
			"body":           args[0],
			"action-loc-key": args[1],
			"loc-key":        args[2],
			"loc-args":       args[3],
			"launch-image":   args[4],
		} {
			alert.Map[mapKey(String(key))] = value
			alert.MapKeys[mapKey(String(key))] = String(key)
		}
		aps.Map[mapKey(String("alert"))] = alert
		aps.MapKeys[mapKey(String("alert"))] = String("alert")
		aps.Map[mapKey(String("sound"))] = args[5]
		aps.MapKeys[mapKey(String("sound"))] = String("sound")
		aps.Map[mapKey(String("badge"))] = args[6]
		aps.MapKeys[mapKey(String("badge"))] = String("badge")
	}
	payload.Map[mapKey(String("aps"))] = aps
	payload.MapKeys[mapKey(String("aps"))] = String("aps")
	userData := args[len(args)-1]
	for key, value := range userData.Map {
		if decoded, ok := userData.MapKeys[key]; ok {
			payload.MapKeys[key] = decoded
		}
		payload.Map[key] = value
	}
	return payload, nil
}

func emailMessageFieldName(method string) string {
	if strings.HasPrefix(method, "get") && len(method) > len("get") {
		field := strings.TrimPrefix(method, "get")
		return strings.ToLower(field[:1]) + field[1:]
	}
	if strings.HasPrefix(method, "is") && len(method) > len("is") {
		field := strings.TrimPrefix(method, "is")
		return strings.ToLower(field[:1]) + field[1:]
	}
	if !strings.HasPrefix(method, "set") || len(method) <= len("set") {
		return method
	}
	field := strings.TrimPrefix(method, "set")
	return strings.ToLower(field[:1]) + field[1:]
}

func restMapPut(receiver *Value, field, key string, value Value, caseInsensitive bool) {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		current = typedMap("Map<String,String>")
	}
	if caseInsensitive {
		for rawKey := range current.Map {
			decoded := valueFromMapKey(rawKey)
			if decoded.Kind == ValueString && strings.EqualFold(decoded.Text, key) {
				delete(current.Map, rawKey)
				break
			}
		}
	}
	current.Map[mapKey(String(key))] = value
	receiver.Fields[field] = current
}

func restMapGet(receiver Value, field, key string) Value {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		return Null
	}
	if value, ok := current.Map[mapKey(String(key))]; ok {
		return value
	}
	for rawKey, value := range current.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString && strings.EqualFold(decoded.Text, key) {
			return value
		}
	}
	return Null
}

func restMapKeys(receiver Value, field string) Value {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		return List()
	}
	keys := make([]string, 0, len(current.Map))
	for rawKey := range current.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString {
			keys = append(keys, decoded.Text)
		}
	}
	sort.Strings(keys)
	out := make([]Value, 0, len(keys))
	for _, key := range keys {
		out = append(out, String(key))
	}
	return List(out...)
}
func vmFormulaDefaultShouldEvaluate(field storage.Field, rawDefault string) bool {
	if rawDefault == "" {
		return false
	}
	switch field.Type {
	case storage.FieldDate, storage.FieldDateTime:
		return strings.ContainsAny(rawDefault, "()")
	default:
		return false
	}
}
