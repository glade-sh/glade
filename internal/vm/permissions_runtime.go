package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

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
	return vm.permissionSetGrantsCustomPermission(permissionSetID, permission)
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
	if method == "isCreateable" || method == "isUpdateable" || method == "isDeletable" {
		// These objects have no Apex object write access, including
		// for administrators. Profile privileges cannot enable this surface.
		if strings.EqualFold(objectName, "Organization") || strings.EqualFold(objectName, "Profile") || strings.EqualFold(objectName, "RecordType") || strings.EqualFold(objectName, "ApexLog") {
			return false
		}
	}
	if strings.EqualFold(objectName, "User") && method == "isDeletable" {
		return false
	}
	if method == "isSearchable" {
		if vm != nil {
			if _, definition, ok := vm.describeObjectDefinition(objectName); ok {
				return vm.describePreparedDefinition(objectName, definition).EnableSearch
			}
		}
		return true
	}
	if method == "isQueryable" {
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
	profileName := vm.currentUserProfileName(user)
	if profileName == "System Administrator" {
		return true
	}
	profilePermissionSetID := vm.profileOwnedPermissionSetID(profileID)
	if method == "isAccessible" && vm.apexMetadataObjectReadable(objectName) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if storage.IDsEqual(storage.ID(permissionSetID), storage.ID(profilePermissionSetID)) {
			continue
		}
		if allowed, ok := vm.explicitObjectPermission(permissionSetID, objectName, method); ok && allowed {
			return true
		}
	}
	if allowed, ok := vm.explicitObjectPermission(profilePermissionSetID, objectName, method); ok {
		return allowed
	}
	if allowed, ok := vm.explicitObjectPermission(profileID, objectName, method); ok {
		return allowed
	}
	if vm.profileHasCapturedAccountLicenseRestriction(profileID, objectName) {
		return false
	}
	if vm.profileHasLicense(profileID, "Chatter External") && !isSetupObject(objectName) {
		return false
	}
	if vm.currentProfileHasUserType(profileID, "Guest") {
		return false
	}
	if method == "isAccessible" && vm.configuredUserDirectoryReadable(objectName, profileID) {
		return true
	}
	switch profileName {
	case "Standard User":
		// Owned user_field_permission_fallback observations have no custom
		// object CRUD without an effective profile or permission-set grant.
		if vm.ordinaryCustomPermissionObject(objectName) || strings.EqualFold(objectName, "AccountBrand") {
			return false
		}
		if method == "isUpdateable" {
			_, definition, known := vm.describeObjectDefinition(objectName)
			if known && storage.IsCustomSettingDefinition(definition) &&
				strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") &&
				!vm.customSettingManagementPermission(user, profilePermissionSetID) {
				// Management requires Customize Application. The exact describe
				// facet is pending native hierarchy-setting permission rows.
				return false
			}
		}
	case "Minimum Access - Salesforce":
		return method == "isAccessible" && isBaselineReadableObject(objectName)
	case "Standard Platform User":
		return standardPlatformUserObjectPermission(objectName, method)
	case "Read Only":
		return method == "isAccessible"
	}
	return true
}

// Owned user_visibility_current_configuration observations permit User reads
// for internal Standard/SFDC actors with explicit User Read sharing. Keep this
// after explicit CRUD checks; other models, licenses and write facets are not
// implied. Existing field permissions and record sharing still apply.
func (vm *VM) configuredUserDirectoryReadable(objectName, profileID string) bool {
	if !strings.EqualFold(objectName, "User") || !vm.currentProfileHasUserType(profileID, "Standard") {
		return false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok || !strings.EqualFold(strings.TrimSpace(vm.Org.Objects[canonical].Definition.SharingModel), "Read") {
		return false
	}
	profile, ok := vm.findOrgRecord("Profile", storage.ID(profileID))
	if !ok {
		return false
	}
	licenseID := storageValueIDText(profile.Fields["UserLicenseId"])
	license, ok := vm.findOrgRecord("UserLicense", storage.ID(licenseID))
	return ok && storageStringValueEquals(license.Fields["LicenseDefinitionKey"], "SFDC")
}

func (vm *VM) apexMetadataObjectReadable(objectName string) bool {
	if strings.EqualFold(objectName, "RecordType") {
		return true
	}
	if vm == nil || vm.Org == nil {
		return false
	}
	resolved, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	return storage.IsCustomMetadataDefinition(vm.Org.Objects[resolved].Definition)
}

func (vm *VM) ordinaryCustomPermissionObject(objectName string) bool {
	name, definition, ok := vm.describeObjectDefinition(objectName)
	return ok && hasSuffixFold(name, "__c") && !storage.IsCustomSettingDefinition(definition)
}

// Universally required custom fields override FLS (Salesforce Help, What
// Determines Field Access?). Owned user_field_permission_fallback observations
// also establish RecordTypeId on an ordinary custom object as non-permissionable.
// Both still require object CRUD; record-type availability is a separate fact.
func (vm *VM) customFieldHasIntrinsicAccess(objectName, fieldName string) bool {
	name, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok || storage.IsCustomSettingDefinition(definition) {
		return false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName)
	if !ok {
		return false
	}
	field := definition.Fields[canonical]
	// A universally required custom field is always accessible, on standard
	// objects too (scratch org row: User.ReqU__c under a profile that lists
	// another User field).
	if field.Required && isCustomSchemaName(canonical) {
		return true
	}
	if !hasSuffixFold(name, "__c") {
		return false
	}
	// Name (Text) and OwnerId on an ordinary custom object are not
	// permissionable: a profile that lists other fields of the object does not
	// hide them. Object CRUD still gates them (native rows, API 62 and 67).
	if strings.EqualFold(canonical, "Name") && strings.EqualFold(definition.Metadata["nameFieldType"], "Text") {
		return true
	}
	if strings.EqualFold(canonical, "OwnerId") && !storage.IsMasterDetailCustomObject(definition) {
		return true
	}
	if strings.EqualFold(canonical, "RecordTypeId") && field.Type == storage.FieldReference {
		for _, target := range field.ReferenceTo {
			if strings.EqualFold(target, "RecordType") {
				return true
			}
		}
	}
	return false
}

func (vm *VM) customSettingManagementPermission(user Value, profilePermissionSetID string) bool {
	if userHasPermission(user, "CustomizeApplication") {
		return true
	}
	parents := append([]string{profilePermissionSetID}, vm.assignedPermissionSetIDs(stringField(user, "Id"))...)
	for _, id := range parents {
		record, ok := vm.findOrgRecord("PermissionSet", storage.ID(id))
		if !ok {
			continue
		}
		if flag, ok := record.GetField("PermissionsCustomizeApplication"); ok && flag.Kind == storage.ValueBoolean && flag.Boolean {
			return true
		}
	}
	return false
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
	profileName := vm.currentUserProfileName(user)
	if profileName == "System Administrator" {
		return true
	}
	intrinsicAccess := vm.customFieldHasIntrinsicAccess(objectName, fieldName)
	if method == "isAccessible" && isBaselineSystemFieldReadableWithoutObjectAccess(fieldName) && !intrinsicAccess &&
		(!vm.profileHasCapturedAccountLicenseRestriction(profileID, objectName) || vm.currentUserObjectPermission(objectName, "isAccessible")) {
		return true
	}
	if objectMethod := fieldPermissionObjectMethod(method); objectMethod != "" && !vm.currentUserObjectPermission(objectName, objectMethod) {
		return false
	}
	profilePermissionSetID := vm.profileOwnedPermissionSetID(profileID)
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if storage.IDsEqual(storage.ID(permissionSetID), storage.ID(profilePermissionSetID)) {
			continue
		}
		if allowed, ok := vm.explicitFieldPermission(permissionSetID, objectName, fieldName, method); ok && allowed {
			return true
		}
		// CR-039: assigned object CRUD grants apply to schema-designated,
		// non-permissionable Name fields without a FieldPermissions row.
		// Keep this observed runAs path subject to intrinsic write limits.
		if vm.testContext != nil && vm.testContext.RunAsDepth > vm.testContext.PackageRunAsDepth &&
			vm.nameFieldHasNoFieldLevelPermissions(objectName, fieldName) {
			if allowed, ok := vm.explicitObjectPermission(permissionSetID, objectName, fieldPermissionObjectMethod(method)); ok && allowed {
				return vm.stripInaccessibleIntrinsicFieldPermission(objectName, fieldName, method)
			}
		}
	}
	if allowed, ok := vm.explicitFieldPermission(profilePermissionSetID, objectName, fieldName, method); ok {
		return allowed
	}
	if allowed, ok := vm.explicitFieldPermission(profileID, objectName, fieldName, method); ok {
		return allowed
	}
	if intrinsicAccess {
		return true
	}
	if method == "isAccessible" && strings.EqualFold(objectName, "User") {
		// These standard User fields remain readable under a custom profile
		// that lists other User field permissions (owned API 62/67 rows).
		switch strings.ToLower(fieldName) {
		case "name", "email", "phone", "alias", "isactive", "lastname", "managerid":
			return true
		}
	}
	if profilePermissionSetID != "" && vm.parentHasFieldPermissionsForObject(profilePermissionSetID, objectName) {
		return false
	}
	if profileID != "" && vm.parentHasFieldPermissionsForObject(profileID, objectName) {
		return false
	}
	if vm.currentProfileHasUserType(profileID, "Guest") {
		return method == "isAccessible"
	}
	if method == "isAccessible" && vm.isBaselineReadableField(objectName, fieldName) {
		return true
	}
	switch profileName {
	case "Standard User":
		if vm.ordinaryCustomPermissionObject(objectName) && (method != "isUpdateable" || isCustomSchemaName(fieldName)) {
			return false
		}
		// Object CRUD does not grant access to optional, permissionable custom
		// fields on standard objects. Explicit field grants above still combine.
		if canonical, definition, ok := vm.describeObjectDefinition(objectName); ok && storage.IsKnownStandardObject(canonical) {
			if field, found := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName); found && isCustomSchemaName(field) {
				metadata := definition.Fields[field]
				if !metadata.Required && storage.FieldFlagValue(metadata.Permissionable, true) {
					return false
				}
			}
		}
	case "Minimum Access - Salesforce":
		return false
	case "Standard Platform User":
		return vm.standardPlatformUserFieldPermission(objectName, fieldName, method)
	case "Read Only":
		return method == "isAccessible"
	}
	if vm.profileHasLicense(profileID, "Chatter External") && !isSetupObject(objectName) {
		return false
	}
	return true
}

func isBaselineSystemFieldReadableWithoutObjectAccess(fieldName string) bool {
	return strings.EqualFold(fieldName, "Id") || isBaselineSystemReadableField(fieldName)
}

func (vm *VM) nameFieldHasNoFieldLevelPermissions(objectName, fieldName string) bool {
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[canonical].Definition
	fieldName, ok = storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return false
	}
	field := definition.Fields[fieldName]
	if !storage.IsKnownStandardObject(canonical) {
		nameType := definition.Metadata["nameFieldType"]
		if !strings.EqualFold(nameType, "Text") && !strings.EqualFold(nameType, "AutoNumber") {
			return false
		}
	}
	// Activity COMBOBOX name fields are outside the captured shapes.
	if !isNameFieldDescribe(field) || strings.EqualFold(field.DisplayType, "COMBOBOX") {
		return false
	}
	return field.Permissionable != nil && !*field.Permissionable
}

func fieldPermissionObjectMethod(method string) string {
	switch method {
	case "isCreateable":
		return "isCreateable"
	case "isUpdateable":
		return "isUpdateable"
	case "isAccessible":
		return "isAccessible"
	default:
		return ""
	}
}
func (vm *VM) stripInaccessibleRecords(accessType string, records Value, enforceRootObjectCRUD bool, scopedPermissionSetID string) (Value, Value, Value, error) {
	for _, record := range records.List {
		if record.Kind == ValueNull {
			return Null, Null, Null, newExceptionError("NullPointerException", "Argument cannot be null.")
		}
	}
	if scopedPermissionSetID != "" {
		resolved := false
		if vm != nil && vm.Org != nil {
			if state, ok := vm.Org.Objects["PermissionSet"]; ok {
				_, _, resolved = storage.LookupRecordByID(state.Records, storage.ID(scopedPermissionSetID))
			}
		}
		if !resolved {
			return Null, Null, Null, newExceptionError("NoDataFoundException", fmt.Sprintf("Invalid permission set id: %s", scopedPermissionSetID))
		}
	}
	out := cloneValue(records)
	out.Type = records.Type
	removedFields := Map()
	removedFields.Type = "Map<String,Set<String>>"
	modifiedIndexes := Set()
	modifiedIndexes.Type = "Set<Integer>"
	for i := range out.List {
		modified, err := vm.stripInaccessibleRecord(accessType, &out.List[i], enforceRootObjectCRUD, removedFields, scopedPermissionSetID)
		if err != nil {
			return Null, Null, Null, err
		}
		if modified {
			modifiedIndexes.Set = append(modifiedIndexes.Set, Int(int64(i)))
		}
	}
	return out, removedFields, modifiedIndexes, nil
}
func (vm *VM) enforceUserModeDMLAccess(op string, value Value, accessLevel Value) error {
	objectPermission, fieldPermission := userModeDMLPermissions(op)
	if objectPermission == "" {
		return nil
	}
	scopedPermissionSetID := accessLevelPermissionSetID(accessLevel)
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
		if !vm.currentUserObjectPermissionWithScope(objectName, objectPermission, scopedPermissionSetID) {
			return newExceptionError("SecurityException", fmt.Sprintf("Access to entity '%s' denied", objectName))
		}
		for field := range record.Fields {
			if fieldPermission == "" || isInternalSObjectField(field) || isSObjectSystemField(field) || isDefaultedSObjectField(record, field) {
				continue
			}
			if record.Fields[field].Kind == ValueList && vm.childRelationshipLookup(objectName, field).ChildType != "" {
				continue
			}
			canonicalField := vm.resolveSObjectFieldName(objectName, field)
			if strings.EqualFold(canonicalField, "Id") {
				continue
			}
			if !vm.currentUserFieldPermissionWithScope(objectName, canonicalField, fieldPermission, scopedPermissionSetID) {
				return newExceptionError("SecurityException", fmt.Sprintf("Access to field '%s.%s' denied", objectName, canonicalField))
			}
		}
	}
	return nil
}

func (vm *VM) enforceDMLRecordAccess(op string, value Value, externalIDField string, userMode bool) error {
	if vm == nil || vm.Org == nil || (op != "update" && op != "delete" && op != "undelete" && op != "upsert") {
		return nil
	}
	if !userMode && (vm.currentTrigger || !strings.EqualFold(vm.currentSharingMode(), "with sharing")) {
		return nil
	}
	for _, item := range dmlAccessRecords(value) {
		record, err := vm.recordFromValue(&item)
		if err != nil {
			return err
		}
		var stored storage.Record
		var found bool
		if strings.EqualFold(op, "upsert") {
			kind, existing, classifyErr := vm.classifyUpsert(record, externalIDField)
			if classifyErr != nil {
				return classifyErr
			}
			if kind == "update" && existing.ID != "" {
				stored, found = vm.findOrgRecord(record.Object, existing.ID)
			}
		} else if record.ID != "" {
			stored, found = vm.findOrgRecord(record.Object, record.ID)
		}
		if found && !vm.currentUserCanWriteRecord(record.Object, stored, vm.currentUserID(), op) {
			return newExceptionError("SecurityException", fmt.Sprintf("Access to record '%s' denied", stored.ID))
		}
	}
	return nil
}

func accessLevelPermissionSetID(accessLevel Value) string {
	if !isDatabaseAccessLevelValue(accessLevel) || accessLevel.Fields == nil {
		return ""
	}
	value, ok := accessLevel.Fields["permissionSetId"]
	if !ok {
		return ""
	}
	if value.Kind == ValueString {
		return strings.TrimSpace(value.Text)
	}
	if text, ok := platformScalarObjectText(value); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func (vm *VM) currentUserObjectPermissionWithScope(objectName, method, permissionSetID string) bool {
	if permissionSetID != "" {
		allowed, ok := vm.explicitObjectPermission(permissionSetID, objectName, method)
		return ok && allowed
	}
	return vm.currentUserObjectPermission(objectName, method)
}

func (vm *VM) currentUserFieldPermissionWithScope(objectName, fieldName, method, permissionSetID string) bool {
	if permissionSetID != "" {
		allowed, ok := vm.explicitFieldPermission(permissionSetID, objectName, fieldName, method)
		return ok && allowed
	}
	return vm.currentUserFieldPermission(objectName, fieldName, method)
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
func (vm *VM) stripInaccessibleRecord(accessType string, record *Value, enforceRootObjectCRUD bool, removedFields Value, scopedPermissionSetID string) (bool, error) {
	if record == nil || record.Kind != ValueObject {
		return false, nil
	}
	if strings.EqualFold(record.Type, "AggregateResult") {
		return false, newExceptionError("SObjectException", "AggregateResult SObject types are not supported")
	}
	modified := false
	objectName := record.Type
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
	}
	objectPermission := stripInaccessibleObjectPermission(accessType)
	if enforceRootObjectCRUD && objectPermission != "" && !vm.currentUserObjectPermissionWithScope(objectName, objectPermission, scopedPermissionSetID) {
		return false, newExceptionError("NoAccessException", "No access to entity: "+objectName)
	}
	fieldPermission := stripInaccessibleFieldPermission(accessType)
	// Disabling root CRUD enforcement suppresses the exception; it does not
	// retain readable fields of an inaccessible object. Id is handled separately.
	readableObject := accessType != "READABLE" || vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", scopedPermissionSetID)
	for _, field := range orderedSObjectFieldIteration(*record) {
		value, ok := record.Fields[field]
		if !ok {
			continue
		}
		// Schema defaults expose OwnerId to callers just like an explicit
		// assignment. Its read permission still applies when root CRUD is disabled.
		readableOwner := accessType == "READABLE" && strings.EqualFold(field, "OwnerId")
		if isInternalSObjectField(field) || isSObjectSystemField(field) && !isExplicitSObjectField(*record, field) && !readableOwner {
			continue
		}
		// Pending native rows cover only a fresh ordinary custom record's
		// implicit false checkbox default, not explicit or queried values.
		_, queriedRecord := record.Fields[sobjectQueriedFieldsField]
		_, clonedRecord := record.Fields[sobjectCloneMarkerField]
		// Omitted checkbox defaults stay unpopulated during stripping on
		// standard objects and ordinary custom-object updates.
		// Explicit and queried values still require field permissions.
		if (accessType == "CREATABLE" || accessType == "UPDATABLE") && scopedPermissionSetID == "" &&
			value.Kind == ValueBool && !value.Bool && (!queriedRecord || dmlAccessibleSObject(*record)) && !isTriggerSObject(*record) &&
			isDefaultedSObjectField(*record, field) && !isExplicitSObjectField(*record, field) &&
			!isUserSetSObjectField(*record, field) &&
			(storage.IsKnownStandardObject(objectName) || accessType == "UPDATABLE" && vm.ordinaryCustomPermissionObject(objectName)) {
			canonicalField := vm.resolveSObjectFieldName(objectName, field)
			_, definition, declared := vm.sObjectFieldDefinition(objectName, canonicalField)
			if declared && isCustomSchemaName(canonicalField) && definition.Type == storage.FieldBoolean &&
				strings.EqualFold(strings.TrimSpace(definition.DefaultValue), "false") {
				continue
			}
		}
		if accessType == "CREATABLE" && scopedPermissionSetID == "" && sObjectIDFromFields(record.Fields) == "" &&
			value.Kind == ValueBool && !value.Bool && !queriedRecord && !clonedRecord && !isTriggerSObject(*record) &&
			isDefaultedSObjectField(*record, field) && !isExplicitSObjectField(*record, field) &&
			!isUserSetSObjectField(*record, field) && !vm.queriedSObjectFieldsIncludes(*record, field) &&
			vm.ordinaryCustomPermissionObject(objectName) &&
			vm.currentUserFieldPermissionWithScope(objectName, vm.resolveSObjectFieldName(objectName, field), "isAccessible", scopedPermissionSetID) {
			_, definition, declared := vm.sObjectFieldDefinition(objectName, field)
			if declared && storage.FieldFlagValue(definition.Accessible, true) && definition.Type == storage.FieldBoolean &&
				strings.EqualFold(strings.TrimSpace(definition.DefaultValue), "false") &&
				vm.stripInaccessibleIntrinsicFieldPermission(objectName, vm.resolveSObjectFieldName(objectName, field), "isCreateable") {
				continue
			}
		}
		if value.Kind == ValueList {
			if childObjectName := sObjectListObjectName(value); childObjectName != "" {
				if vm.Org != nil {
					if resolved, ok := vm.resolveObjectName(childObjectName); ok {
						childObjectName = resolved
					}
				}
				if objectPermission != "" && !vm.currentUserObjectPermissionWithScope(childObjectName, objectPermission, scopedPermissionSetID) {
					delete(record.Fields, field)
					unmarkQueriedSObjectField(record, field)
					markStrippedChildRelationship(record, field)
					addStripInaccessibleRemovedField(removedFields, objectName, field)
					modified = true
					continue
				}
			}
			childModified := false
			for i := range value.List {
				modifiedChild, err := vm.stripInaccessibleRecord(accessType, &value.List[i], enforceRootObjectCRUD, removedFields, scopedPermissionSetID)
				if err != nil {
					return false, err
				}
				if modifiedChild {
					childModified = true
					modified = true
				}
			}
			if childModified && !stripInaccessibleChildRelationshipHasPayload(value) {
				delete(record.Fields, field)
				unmarkQueriedSObjectField(record, field)
				markStrippedChildRelationship(record, field)
				addStripInaccessibleRemovedField(removedFields, objectName, field)
				modified = true
				continue
			}
			record.Fields[field] = value
			continue
		}
		canonicalField := vm.resolveSObjectFieldName(objectName, field)
		if strings.EqualFold(canonicalField, "Id") {
			continue
		}
		fieldAllowed := readableObject && vm.currentUserFieldPermissionWithScope(objectName, canonicalField, fieldPermission, scopedPermissionSetID)
		if scopedPermissionSetID == "" {
			fieldAllowed = fieldAllowed || vm.stripInaccessibleKeepsBaselineWritableField(accessType, objectName, canonicalField)
		}
		if isExplicitSObjectField(*record, field) || vm.queriedSObjectFieldsIncludes(*record, field) {
			fieldAllowed = fieldAllowed && vm.stripInaccessibleIntrinsicFieldPermission(objectName, canonicalField, fieldPermission)
		}
		if fieldPermission == "" || fieldAllowed {
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
			fieldAllowed := readableObject && vm.currentUserFieldPermissionWithScope(objectName, canonicalField, fieldPermission, scopedPermissionSetID)
			if scopedPermissionSetID == "" {
				fieldAllowed = fieldAllowed || vm.stripInaccessibleKeepsBaselineWritableField(accessType, objectName, canonicalField)
			}
			fieldAllowed = fieldAllowed && vm.stripInaccessibleIntrinsicFieldPermission(objectName, canonicalField, fieldPermission)
			if fieldPermission == "" || fieldAllowed {
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

// Even an administrator cannot write calculated, autonumber or audit fields.
// Keep that capability separate from user/permission-set grants.
func (vm *VM) stripInaccessibleIntrinsicFieldPermission(objectName, fieldName, permission string) bool {
	_, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok {
		return true
	}
	field := definition.Fields[fieldName]
	if field.APIName == "" {
		if synthetic, ok := syntheticSObjectSystemField(fieldName); ok {
			field = synthetic
		} else {
			return true
		}
	}
	field = describeFieldWithSystemOverlay(field)
	writable := !describeFieldCalculated(field) && !field.AutoNumber
	switch permission {
	case "isCreateable":
		return storage.FieldFlagValue(field.Createable, writable)
	case "isUpdateable":
		return storage.FieldFlagValue(field.Updateable, writable)
	default:
		return storage.FieldFlagValue(field.Accessible, true)
	}
}

func stripInaccessibleChildRelationshipHasPayload(value Value) bool {
	if value.Kind != ValueList {
		return false
	}
	for _, child := range value.List {
		if child.Kind != ValueObject {
			continue
		}
		for field, fieldValue := range child.Fields {
			if isInternalSObjectField(field) || isSObjectSystemField(field) || strings.EqualFold(field, "Id") {
				continue
			}
			if fieldValue.Kind != ValueNull {
				return true
			}
		}
	}
	return false
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
		return stripInaccessibleKeepsSObjectBaselineWritableUpdateField(objectName, fieldName)
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
		if sObjectTypeValue, ok := record.GetField("SObjectType"); !ok || (sObjectTypeValue.Kind != storage.ValueString || !vm.permissionMetadataNameMatches(objectName, sObjectTypeValue.String)) {
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
		if sObjectTypeValue, ok := record.GetField("SObjectType"); !ok || (sObjectTypeValue.Kind != storage.ValueString || !vm.permissionMetadataNameMatches(objectName, sObjectTypeValue.String)) {
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
		if sObjectTypeValue, ok := record.GetField("SObjectType"); ok && sObjectTypeValue.Kind == storage.ValueString && vm.permissionMetadataNameMatches(objectName, sObjectTypeValue.String) {
			return true
		}
	}
	return false
}
func (vm *VM) permissionMetadataNameMatches(canonical, candidate string) bool {
	if strings.EqualFold(canonical, candidate) {
		return true
	}
	if !strings.HasSuffix(strings.ToLower(canonical), "__c") || !strings.HasSuffix(strings.ToLower(candidate), "__c") {
		return false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	return metadataObjectNameMatches(namespace, canonical, candidate)
}
func (vm *VM) fieldPermissionFieldMatches(value storage.Value, objectName, fieldName string) bool {
	if value.Kind != storage.ValueString {
		return false
	}
	text := value.String
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
		return vm.permissionMetadataNameMatches(objectName, text[:dot]) && vm.fieldPermissionNameMatches(objectName, text[dot+1:], fieldName)
	}
	return vm.fieldPermissionNameMatches(objectName, text, fieldName)
}
func (vm *VM) fieldPermissionNameMatches(objectName, permissionField, fieldName string) bool {
	if vm.permissionMetadataNameMatches(fieldName, permissionField) {
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
		if assigneeValue, ok := record.GetField("AssigneeId"); ok && storage.IDsEqual(storage.ID(storageValueIDText(assigneeValue)), storage.ID(userID)) {
			if permissionSetValue, ok := record.GetField("PermissionSetId"); ok {
				if id := storageValueIDText(permissionSetValue); id != "" {
					// Text conversion can expand an Id to eighteen characters.
					// Existing permission rows still refer to the stored set key.
					if storedID, _, found := storage.LookupRecordByID(vm.Org.Objects["PermissionSet"].Records, storage.ID(id)); found {
						id = string(storedID)
					}
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

// currentUserProfileName resolves both persisted users with ProfileId and the
// unsaved nested User.Profile form accepted by System.runAs in Apex tests.
func (vm *VM) currentUserProfileName(user Value) string {
	profileID := stringField(user, "ProfileId")
	if name := vm.currentProfileName(profileID); name != "" {
		return name
	}
	_, profile, ok := objectFieldValue(user, "Profile")
	if !ok || profile.Kind != ValueObject {
		return ""
	}
	return stringField(profile, "Name")
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

func (vm *VM) currentProfileHasUserType(profileID, userType string) bool {
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
	return storageStringValueEquals(profile.Fields["UserType"], userType)
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
	case "createdbyid", "createddate", "isdeleted", "lastmodifiedbyid", "lastmodifieddate", "lastreferenceddate", "lastvieweddate", "ownerid", "recordtypeid", "systemmodstamp":
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

// SetupEntityAccess links assigned permission sets to standalone metadata rows.
func (vm *VM) permissionSetGrantsCustomPermission(permissionSetID, permission string) bool {
	permissions := vm.Org.Objects["CustomPermission"]
	for _, access := range vm.Org.Objects["SetupEntityAccess"].Records {
		parent, ok := access.GetField("ParentId")
		if !ok || !storageIDValueEquals(parent, permissionSetID) {
			continue
		}
		entity, ok := access.GetField("SetupEntityId")
		if !ok {
			continue
		}
		for id, record := range permissions.Records {
			if !storageIDValueEquals(entity, string(id)) {
				continue
			}
			name, ok := record.GetField("DeveloperName")
			if !ok {
				continue
			}
			if storagePermissionValueMatches(name, permission) {
				return true
			}
		}
	}
	return false
}
