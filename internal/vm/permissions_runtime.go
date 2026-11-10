package vm

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/dml"
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
func (vm *VM) currentUserPermissionFields() map[string]storage.Value {
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	var fields map[string]storage.Value
	for _, field := range []string{"Permissions", "PermissionSets"} {
		if _, value, ok := objectFieldValue(user, field); ok {
			converted, ok := permissionFixtureStorageValue(value)
			if !ok {
				continue
			}
			if fields == nil {
				fields = make(map[string]storage.Value, 2)
			}
			fields[field] = converted
		}
	}
	return fields
}

// Convert supported fixture values independently so an unrelated list member
// cannot hide another grant. Matching remains in the shared storage lookup.
func permissionFixtureStorageValue(value Value) (storage.Value, bool) {
	if value.Kind == ValueList {
		items := make([]storage.Value, 0, len(value.List))
		for _, item := range value.List {
			if converted, ok := permissionFixtureStorageValue(item); ok {
				items = append(items, converted)
			}
		}
		return storage.ListValue(items...), true
	}
	converted, err := storageValueFromVM(value)
	return converted, err == nil && converted.Kind == storage.ValueString
}
func (vm *VM) formulaEvaluationOptions() dml.FormulaEvaluationOptions {
	return dml.FormulaEvaluationOptions{UserID: storage.ID(vm.currentUserID()), PermissionUserFields: vm.currentUserPermissionFields()}
}

// sObjectFormulaEvaluationOptions reads a formula on a record; a Trigger.new or
// Trigger.old record selects the trigger read surface (blank_text_formula).
func (vm *VM) sObjectFormulaEvaluationOptions(receiver Value) dml.FormulaEvaluationOptions {
	options := vm.formulaEvaluationOptions()
	options.TriggerRecord = isTriggerSObject(receiver)
	return options
}
func (vm *VM) currentUserHasDirectPermission(permission string) bool {
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	return userHasPermission(user, permission)
}

// currentUserHasPermission shares the formula lookup (permission set groups
// and SetupEntityAccess, q1formula). As assignedPermissionSetIDsFiltered does,
// a direct assignment captured as revoked, expired or inactive grants nothing
// (r6flow_expiry); group and uncaptured states keep the existing lookup.
func (vm *VM) currentUserHasPermission(permission string) bool {
	return storage.CustomPermissionGrantedWhere(vm.Org, storage.ID(vm.currentUserID()), vm.currentUserPermissionFields(), permission, func(assignment storage.Record) bool {
		if group, ok := assignment.GetField("PermissionSetGroupId"); ok && storageValueIDText(group) != "" {
			return true
		}
		active, known := dml.PermissionAssignmentState(assignment, func() time.Time { return vm.fakeNow })
		return !known || active
	})
}
func (vm *VM) currentUserObjectPermission(objectName, method string) bool {
	if method != "isQueryable" && method != "isSearchable" && vm.isNamedApexReferenceDescribe(objectName) {
		// Native describes of these reference objects deny create, update,
		// delete and read access to every user.
		return false
	}
	if method == "isCreateable" || method == "isUpdateable" || method == "isDeletable" {
		// These objects have no Apex object write access, including
		// for administrators. Profile privileges cannot enable this surface.
		if strings.EqualFold(objectName, "Organization") || strings.EqualFold(objectName, "Profile") || strings.EqualFold(objectName, "RecordType") || strings.EqualFold(objectName, "ApexLog") ||
			strings.EqualFold(objectName, "ApexClass") || strings.EqualFold(objectName, "ApexTrigger") || strings.EqualFold(objectName, "UserLicense") {
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
	if (method == "isQueryable" || method == "isCreateable" || method == "isUpdateable" || method == "isDeletable") && isChangeEventObjectName(objectName) {
		// A change event is neither queryable nor writable
		// (global_describe_changeevent_keys).
		return false
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
	for _, source := range vm.assignedPermissionSources(stringField(user, "Id"), false) {
		if storage.IDsEqual(storage.ID(source.ID), storage.ID(profilePermissionSetID)) {
			continue
		}
		if vm.sourceObjectPermission(source, objectName, method) {
			return true
		}
	}
	ownedAllowed, ownedKnown := vm.explicitObjectPermission(profilePermissionSetID, objectName, method)
	profileAllowed, profileKnown := vm.explicitObjectPermission(profileID, objectName, method)
	if ownedKnown && (method != "isAccessible" || !strings.EqualFold(objectName, "Account")) {
		return ownedAllowed
	}
	if ownedKnown || profileKnown {
		return ownedAllowed || profileAllowed
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
	// The captured standard profile inventory at API 62 and 67 grants Account
	// read exactly when its profile-owned set has a read row. Assigned and
	// profile grants have already been checked above. The same capture lists
	// every Account and Contact object permission row of every standard profile
	// (profile_object_permissions), so a standard profile without a row has no
	// access of any kind to either object (no_account_read_profile_user, API
	// 62.0 and 67.0).
	accountRead := method == "isAccessible" && strings.EqualFold(objectName, "Account")
	capturedStandardObject := (strings.EqualFold(objectName, "Account") || strings.EqualFold(objectName, "Contact")) &&
		(method == "isAccessible" || method == "isCreateable" || method == "isUpdateable" || method == "isDeletable")
	if accountRead || capturedStandardObject {
		owner, ok := vm.findOrgRecord("PermissionSet", storage.ID(profilePermissionSetID))
		custom, hasCustom := owner.GetField("IsCustom")
		standardOwner := ok && hasCustom && custom.Kind == storage.ValueBoolean && !custom.Boolean &&
			storageStringValueEquals(owner.Fields["Type"], "Profile")
		// A deployed custom profile on the full Salesforce license that lists no
		// Account or Contact row has none either
		// (custom_profile_unlisted_object_access; the Salesforce Platform license
		// carries default Account and Contact rows instead).
		fullLicenseOwner := ok && storageStringValueEquals(owner.Fields["Type"], "Profile") && vm.profileHasLicense(profileID, "Salesforce")
		if standardOwner || fullLicenseOwner {
			return false
		}
		if accountRead {
			// An unrelated profile retains the ordinary fallback, irrespective of
			// whether its display name matches a captured standard profile.
			return true
		}
	}
	switch profileName {
	case "Standard User":
		// Owned user_field_permission_fallback observations have no custom
		// object CRUD without an effective profile or permission-set grant.
		// Native event_publish_list_profiles rows: an ungranted platform event
		// is neither accessible nor creatable (API 62.0 to 68.0).
		if vm.ordinaryCustomPermissionObject(objectName) || vm.ordinaryCustomEvent(objectName) || strings.EqualFold(objectName, "AccountBrand") {
			return false
		}
		// An ungranted Standard User has no CRUD on a hierarchy or list custom
		// setting (list_setting_runas_standard_user).
		if (method == "isAccessible" || method == "isCreateable" || method == "isUpdateable" || method == "isDeletable") &&
			vm.hierarchyCustomSettingQueryDenied(objectName, "") {
			return false
		}
		if !standardUserObjectPermission(objectName, method) {
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
		// Native event_publish_list_profiles rows (62.0 to 68.0): an ordinary
		// custom object or platform event with no grant is not accessible or
		// creatable, as for the Standard User.
		if vm.ordinaryCustomPermissionObject(objectName) || vm.ordinaryCustomEvent(objectName) {
			return false
		}
		return standardPlatformUserObjectPermission(objectName, method)
	case "Read Only":
		return method == "isAccessible"
	}
	return true
}

// Owned User visibility observations permit reads for internal Standard/SFDC
// actors with User Read sharing. A fresh Developer org defaults User to Read
// (case159_sharing); explicit sharing models and CRUD checks still take precedence.
func (vm *VM) configuredUserDirectoryReadable(objectName, profileID string) bool {
	if !strings.EqualFold(objectName, "User") || !vm.currentProfileHasUserType(profileID, "Standard") {
		return false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	model := strings.TrimSpace(vm.Org.Objects[canonical].Definition.SharingModel)
	if model != "" && !strings.EqualFold(model, "Read") {
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

// ordinaryCustomEvent reports a custom platform event (an __e object).
func (vm *VM) ordinaryCustomEvent(objectName string) bool {
	return hasSuffixFold(objectName, "__e") && vm.hasPlatformEventMetadata(objectName)
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

// fieldPermissionPrincipal is the user whose field permissions apply.
func (vm *VM) fieldPermissionPrincipal() Value {
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if user.Kind == "" || user.Kind == ValueNull {
		// The captured anonymous route uses the default administrator. Keep
		// other implicit principals on their existing fallback until observed.
		if candidate := vm.defaultTestCurrentUser(); vm.currentUserProfileName(candidate) == "System Administrator" {
			user = candidate
		}
	}
	return user
}

func (vm *VM) currentUserFieldPermission(objectName, fieldName, method string) bool {
	if vm.Org == nil {
		return true
	}
	user := vm.fieldPermissionPrincipal()
	profileID := stringField(user, "ProfileId")
	profileName := vm.currentUserProfileName(user)
	if profileName == "System Administrator" {
		// A profile-owned field inventory defines the grants for permissionable
		// fields on that object. An assigned permission set can add a grant;
		// otherwise an absent row in the captured inventory remains denied.
		profileSet := vm.profileOwnedPermissionSetID(profileID)
		_, field, known := vm.sObjectFieldDefinition(objectName, fieldName)
		if known && storage.IsKnownStandardObject(objectName) && storage.FieldFlagValue(field.Permissionable, true) && vm.parentHasFieldPermissionsForObject(profileSet, objectName) {
			for _, source := range vm.assignedPermissionSources(stringField(user, "Id"), false) {
				if vm.sourceFieldPermission(source, objectName, fieldName, method) {
					return true
				}
			}
			profileMethod := method
			if method == "isCreateable" {
				// The native administrator can create an intrinsically writable
				// field whose profile row grants read but not edit.
				profileMethod = "isAccessible"
			}
			allowed, _ := vm.explicitFieldPermission(profileSet, objectName, fieldName, profileMethod)
			return allowed
		}
		// Optional custom fields on ordinary custom objects need an effective
		// field grant even for the administrator in a freshly deployed org.
		if adminOptionalFieldPolicyAPI(vm.currentSourceExecutionPolicy().APIVersion) && vm.ordinaryCustomPermissionObject(objectName) && isCustomSchemaName(fieldName) && !vm.customFieldHasIntrinsicAccess(objectName, fieldName) {
			parents := []string{vm.profileOwnedPermissionSetID(profileID), profileID}
			// glade exec cannot know assignment: a deployed set that grants the
			// field counts as assigned (native admin_field_grant_sources rows).
			if vm.assumeDeployedPermissionSets && vm.testContext == nil {
				parents = append(parents, vm.deployedPermissionSetIDs()...)
			}
			for _, parent := range parents {
				if allowed, ok := vm.explicitFieldPermission(parent, objectName, fieldName, method); ok && allowed {
					return true
				}
			}
			for _, source := range vm.assignedPermissionSources(stringField(user, "Id"), false) {
				if vm.sourceFieldPermission(source, objectName, fieldName, method) {
					return true
				}
			}
			return false
		}
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
	for _, source := range vm.assignedPermissionSources(stringField(user, "Id"), false) {
		permissionSetID := source.ID
		if storage.IDsEqual(storage.ID(permissionSetID), storage.ID(profilePermissionSetID)) {
			continue
		}
		if vm.sourceFieldPermission(source, objectName, fieldName, method) {
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
	if (profileName == "Standard User" || profileName == "Standard Platform User") &&
		standardProfileHiddenOptOutField(objectName, fieldName) {
		return false
	}
	if profileName == "Standard User" && (method == "isCreateable" || method == "isUpdateable") &&
		standardUserManagedUserField(objectName, fieldName) {
		return false
	}
	// Non-permissionable fields do not require a FieldPermissions grant. Their
	// schema read/write facets and object CRUD still apply. The native restricted
	// Case capture observes this (after explicit rows) for Status and OwnerId,
	// and standard_profile_object_crud (Account.Name TTT) and
	// native_formula_permission_regex (Contact.LastName insert) observe it for
	// the Standard User, whose profile-owned set lists every permissionable
	// Account and Contact field.
	nonPermissionable, nonPermissionableField := false, fieldName
	if _, field, ok := vm.sObjectFieldDefinition(objectName, fieldName); ok && field.Permissionable != nil && !*field.Permissionable {
		nonPermissionable, nonPermissionableField = true, field.APIName
	}
	if nonPermissionable && !strings.EqualFold(objectName, "User") {
		return vm.stripInaccessibleIntrinsicFieldPermission(objectName, nonPermissionableField, method)
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
	// User is the one object whose non-permissionable fields a profile field
	// listing still hides: invocable_record_followups userFields denies
	// User.ProfileId and User.UserRoleId (API 62.0, 67.0) under a profile that
	// lists other User fields, so for User the rule follows the two checks above.
	if nonPermissionable {
		return vm.stripInaccessibleIntrinsicFieldPermission(objectName, nonPermissionableField, method)
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
	defer vm.beginPermissionCheckMemo()()
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
	return vm.enforceUserModeDMLAccessDeferring(op, value, accessLevel, false)
}

// userModeFieldDMLException builds the statement-level exception for denied
// fields on the record at index. The outer message omits the field names; the
// row detail carries them and, for update and upsert by Id, the record Id
// (native partial_dml_multifield throw rows, APIs 62.0, 67.0 and 68.0).
func userModeFieldDMLException(op, objectName string, record Value, fields []string, index int) error {
	message := fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result! ", objectName)
	exception := Object("DmlException")
	exception.Fields["message"] = String(message)
	types := make([]string, index+1)
	results := make([]dml.Result, index+1)
	types[index] = objectName
	// Native rows join field names with "," and no space
	// (partial_dml_multifield/org.tsv); single-field dotfix_fls_followups rows agree.
	results[index] = userModeFieldDMLRowFailure(op, objectName, record, fields)
	exception.Fields["__dmlErrors"] = dmlExceptionErrorDetails(results, types)
	return &apexThrowError{value: exception}
}

// userModeFieldDMLRowFailure is the partial-DML counterpart: the row message
// appends the field names, as the native SaveResult errors do. A failed update
// keeps the record Id and a failed insert has none (native
// admin_optional_field_api_matrix mixedLists_noGrant rows, API 67.0 and 68.0).
func userModeFieldDMLRowFailure(op, objectName string, record Value, fields []string) dml.Result {
	message := fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result!  fieldNames: %s", objectName, strings.Join(fields, ","))
	failure := dml.Result{Error: message, StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", Fields: fields}
	if op == "update" || op == "upsert" {
		if _, id, ok := objectFieldValue(record, "Id"); ok {
			if text, ok := idValueText(id); ok {
				failure.ID = storage.ID(text)
			}
		}
	}
	return failure
}

// userModeDMLFieldFailures evaluates the captured field denial for each target
// of a partial-DML call. Rows without a denial keep their position and run.
func (vm *VM) userModeDMLFieldFailures(op string, targets []*Value) []dml.Result {
	objectPermission, fieldPermission := userModeDMLPermissions(op)
	if fieldPermission == "" {
		return nil
	}
	var failures []dml.Result
	for i, target := range targets {
		if target == nil || target.Kind != ValueObject {
			continue
		}
		// The statement check already ran CRUD and the SecurityException
		// fields; only policy-denied fields are left here.
		denied, _ := vm.userModeDMLRecordAccess(op, *target, objectPermission, fieldPermission, "")
		if len(denied) == 0 {
			continue
		}
		if failures == nil {
			failures = make([]dml.Result, len(targets))
		}
		failures[i] = userModeFieldDMLRowFailure(op, vm.canonicalSObjectValueType(*target), *target, denied)
	}
	return failures
}

// enforceUserModeDMLAccessDeferring checks CRUD and FLS for user-mode DML.
// With deferFieldRows, captured-shape field denials are left for the partial
// DML engine, which fails only the denied rows.
func (vm *VM) enforceUserModeDMLAccessDeferring(op string, value Value, accessLevel Value, deferFieldRows bool) error {
	objectPermission, fieldPermission := userModeDMLPermissions(op)
	if objectPermission == "" {
		return nil
	}
	scopedPermissionSetID := accessLevelPermissionSetID(accessLevel)
	records := dmlAccessRecords(value)
	defer vm.beginPermissionCheckMemo()()
	for index, record := range records {
		fields, err := vm.userModeDMLRecordAccess(op, record, objectPermission, fieldPermission, scopedPermissionSetID)
		if err != nil {
			return err
		}
		// Scoped permission sets keep the statement exception: the partial
		// engine checks rows without the scope.
		if len(fields) > 0 && (!deferFieldRows || scopedPermissionSetID != "") {
			return userModeFieldDMLException(op, vm.canonicalSObjectValueType(record), record, fields, index)
		}
	}
	return nil
}

// userModeDMLRecordAccess checks one record's user-mode object and field
// access. Fields denied only by the API 62+ ordinary custom field policy are
// returned in explicit assignment order for the caller to raise as a DmlException or failed row;
// other denials fail with a SecurityException. Upsert follows insert and
// update with or without an Id (native shapemisc_capture upsertInsertPath_text
// and api67_access_defaults dml_ups_secret rows).
func (vm *VM) userModeDMLRecordAccess(op string, record Value, objectPermission, fieldPermission, scopedPermissionSetID string) ([]string, error) {
	if record.Kind != ValueObject {
		return nil, nil
	}
	objectName := record.Type
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
	}
	if !vm.currentUserObjectPermissionWithScope(objectName, objectPermission, scopedPermissionSetID) {
		return nil, newExceptionError("SecurityException", fmt.Sprintf("Access to entity '%s' denied", objectName))
	}
	var policyFields []string
	for field := range record.Fields {
		if fieldPermission == "" || isInternalSObjectField(field) || isSObjectSystemField(field) || isDefaultedSObjectField(record, field) {
			continue
		}
		if record.Fields[field].Kind == ValueList && vm.childRelationshipLookup(objectName, field).ChildType != "" {
			continue
		}
		canonicalField := vm.resolveSObjectFieldName(objectName, field)
		if strings.EqualFold(canonicalField, "Id") || vm.sobjectFormulaField(objectName, canonicalField) {
			continue
		}
		// A queried null User field that no assignment or constructor argument
		// set is not written: native user_self_update queriedRoleActive (API
		// 67.0) saves a queried null ManagerId that an explicit null denies.
		if strings.EqualFold(objectName, "User") && record.Fields[field].Kind == ValueNull && !isExplicitSObjectField(record, field) {
			continue
		}
		if !vm.currentUserFieldPermissionWithScope(objectName, canonicalField, fieldPermission, scopedPermissionSetID) || vm.automatedProcessCustomFieldDenied(op, objectName, canonicalField, scopedPermissionSetID) {
			// Native dotfix_fls_followups case.insert.user rows (restricted user,
			// API 62.0, 67.0, 68.0): a denied standard field on a standard object
			// raises the same statement DmlException, with the field name.
			standardFieldDenial := !vm.ordinaryCustomPermissionObject(objectName) && !isCustomSchemaName(canonicalField)
			if (op == "insert" || op == "update" || op == "upsert") && adminOptionalFieldPolicyAPI(vm.currentSourceExecutionPolicy().APIVersion) && (vm.ordinaryCustomPermissionObject(objectName) && isCustomSchemaName(canonicalField) || standardFieldDenial) {
				policyFields = append(policyFields, canonicalField)
				continue
			}
			return nil, newExceptionError("SecurityException", fmt.Sprintf("Access to field '%s.%s' denied", objectName, canonicalField))
		}
	}
	order := make(map[string]int)
	for i, field := range explicitSObjectFieldNamesInInsertionOrder(record) {
		order[strings.ToLower(field)] = i
	}
	sort.SliceStable(policyFields, func(i, j int) bool {
		a, aok := order[strings.ToLower(policyFields[i])]
		b, bok := order[strings.ToLower(policyFields[j])]
		if aok && bok {
			return a < b
		}
		if aok != bok {
			return aok
		}
		return policyFields[i] < policyFields[j]
	})
	return policyFields, nil
}

// automatedProcessCustomFieldDenied reports the Automated Process user's
// USER_MODE write to an ungranted custom field below API 66.0. Native
// automated_process_user_mode rows (62.0, event trigger and the queueable and
// future jobs it enqueues): the insert and update fail with the ordinary
// custom field DmlException, while 66.0 to 68.0 succeed. Reads and deletes are
// unaffected at every API.
func (vm *VM) automatedProcessCustomFieldDenied(op, objectName, fieldName, scopedPermissionSetID string) bool {
	if scopedPermissionSetID != "" || (op != "insert" && op != "update" && op != "upsert") {
		return false
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if !strings.EqualFold(stringField(user, "UserType"), "AutomatedProcess") {
		return false
	}
	version := vm.currentSourceExecutionPolicy().APIVersion
	if !apexversion.AtLeast(version, 62) || !apexversion.Before(version, 66) {
		return false
	}
	return vm.ordinaryCustomPermissionObject(objectName) && isCustomSchemaName(fieldName) && !vm.customFieldHasIntrinsicAccess(objectName, fieldName)
}

// userModeUpsertKeyFieldDenial rejects a user-mode upsert whose external id
// field the ordinary custom field policy hides. Native rows (administrator,
// no grant on the key field, API 62.0 USER_MODE and 67.0/68.0 default and
// USER_MODE, which run in user mode): every upsert onto an existing or new
// record, Database.upsert with allOrNone true or false and the statement,
// throws SObjectException "Field Object.Field__c is inaccessible in this
// context" before any record is checked. With the key field granted and an
// optional field ungranted, the upsert follows the insert and update denials
// instead (userModeDMLRecordAccess).
func (vm *VM) userModeUpsertKeyFieldDenial(op string, value Value, externalIDField, scopedPermissionSetID string) error {
	if vm == nil || vm.Org == nil || op != "upsert" || externalIDField == "" || strings.EqualFold(externalIDField, "Id") {
		return nil
	}
	if !adminOptionalFieldPolicyAPI(vm.currentSourceExecutionPolicy().APIVersion) {
		return nil
	}
	records := dmlAccessRecords(value)
	if len(records) == 0 || records[0].Kind != ValueObject {
		return nil
	}
	objectName := records[0].Type
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		objectName = resolved
	}
	canonical := vm.resolveSObjectFieldName(objectName, externalIDField)
	if !vm.ordinaryCustomPermissionObject(objectName) || !isCustomSchemaName(canonical) {
		return nil
	}
	if vm.currentUserFieldPermissionWithScope(objectName, canonical, "isAccessible", scopedPermissionSetID) {
		return nil
	}
	return newExceptionError("SObjectException", "Field "+objectName+"."+canonical+" is inaccessible in this context")
}

// userModeFieldPolicyPartialFailures returns, for default-mode
// Database.insert/update(records, false), one failed SaveResult per record that
// writes an ordinary custom field the API 62+ field policy denies; records that
// write no denied field have a nil entry and are saved normally. Native
// admin_optional_field_api_matrix rows at 67.0 and 68.0 (administrator, no
// grant): no exception, isSuccess=false, CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY,
// fields (Optional__c), message "Operation failed due to fields being
// inaccessible on Sobject X, check errors on Exception or Result!  fieldNames:
// Optional__c"; a mixed list fails only the denied records (insert result Id
// null, update result Id the record Id). handled is false when no record is
// denied. Native multi-field rows use commas without spaces.
func (vm *VM) userModeFieldPolicyPartialFailures(op string, value Value) ([]*dml.Result, bool) {
	if op != "insert" && op != "update" {
		return nil, false
	}
	objectPermission, fieldPermission := userModeDMLPermissions(op)
	records := dmlAccessRecords(value)
	if len(records) == 0 {
		return nil, false
	}
	defer vm.beginPermissionCheckMemo()()
	results := make([]*dml.Result, len(records))
	denied := false
	for i, record := range records {
		if record.Kind != ValueObject {
			return nil, false
		}
		fields, err := vm.userModeDMLRecordAccess(op, record, objectPermission, fieldPermission, "")
		if err != nil {
			return nil, false
		}
		if len(fields) == 0 {
			continue
		}
		denied = true
		objectName := vm.canonicalSObjectValueType(record)
		failure := &dml.Result{
			StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY",
			Error:      fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result!  fieldNames: %s", objectName, strings.Join(fields, ",")),
			Fields:     fields,
		}
		if op == "update" {
			if _, id, ok := objectFieldValue(record, "Id"); ok {
				if text, ok := idValueText(id); ok {
					failure.ID = storage.ID(text)
				}
			}
		}
		results[i] = failure
	}
	return results, denied
}

// adminOptionalFieldSecureDenial reports a WITH SECURITY_ENFORCED projection
// of an ordinary custom field the API 62+ field policy hides. Native
// admin_optional_field_api_matrix rows at 62.0 and 66.0 (administrator, no
// grant): QueryException "Insufficient permissions: secure query included
// inaccessible field". API 67+ rejects SECURITY_ENFORCED before this check.
func (vm *VM) adminOptionalFieldSecureDenial(objectName, fieldName, mode, permissionSetID string) bool {
	return mode == "SECURITY_ENFORCED" && permissionSetID == "" &&
		adminOptionalFieldPolicyAPI(vm.currentSourceExecutionPolicy().APIVersion) &&
		vm.ordinaryCustomPermissionObject(objectName) && isCustomSchemaName(fieldName) &&
		!vm.customFieldHasIntrinsicAccess(objectName, fieldName)
}

// adminOptionalFieldPolicyAPI bounds the ordinary custom field policy to the
// captured source APIs 62.0-68.0: 62.0 and 67.0 (default_permissions_v035,
// ordinary_controls), 66.0 and 68.0 (admin_optional_field_api_matrix,
// ordinary_controls). 68.0 behaves exactly as 67.0 in every captured row.
func adminOptionalFieldPolicyAPI(version string) bool {
	return apexversion.AtLeast(version, 62) && !apexversion.AtLeast(version, 69)
}

// sobjectFormulaField reports a formula or auto-number field of objectName.
// DML never writes one, so a queried value on the record is not a write the
// user-mode field check governs. Native rows at API 67.0 with the capture's
// read-only grant: flow_refs record rows update records queried with a formula
// field; field_key_upsert the captured row upserts a clone carrying an auto-number
// key and fails on the key, not on field access.
func (vm *VM) sobjectFormulaField(objectName, field string) bool {
	if vm.Org == nil {
		return false
	}
	definition, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	metadata, ok := definition.Definition.Fields[field]
	return ok && (describeFieldCalculated(metadata) || metadata.AutoNumber)
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
		if found && userMode && op == "update" {
			if _, applicable := vm.junctionParentsReadable(record.Object, stored, vm.currentUserID()); applicable {
				// Captured junction updates report row-level DML failures.
				continue
			}
		}
		if found && !vm.currentUserCanWriteRecord(record.Object, stored, vm.currentUserID(), op) {
			// A denied User update is a DmlException row failure at API 67.0
			// too, not the SecurityException (native user_self_update
			// otherUserTitle and otherStandardUserTitle rows); the DML record
			// access check reports it.
			if strings.EqualFold(record.Object, "User") && (op == "update" || op == "upsert") {
				continue
			}
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

type permissionCheckKey struct {
	object, field, method, scope string
}

type permissionCheckMemo struct {
	objects map[permissionCheckKey]bool
	fields  map[permissionCheckKey]bool
}

// beginPermissionCheckMemo memoizes object and field permission answers until
// the returned function runs. Callers hold it only across a loop over the
// records of one statement that runs no Apex and writes no org state, so the
// user, source API and permission rows are fixed for its lifetime and every
// answer is the one the uncached check would give. Nested calls share the
// outer memo.
func (vm *VM) beginPermissionCheckMemo() func() {
	if vm == nil || vm.permissionChecks != nil {
		return func() {}
	}
	vm.permissionChecks = &permissionCheckMemo{}
	return func() { vm.permissionChecks = nil }
}

// canonicalPermissionSetKey maps an Id spelling (an Apex Id expands to
// eighteen characters) onto the key the permission rows were stored under.
func (vm *VM) canonicalPermissionSetKey(id string) string {
	if id == "" || vm == nil || vm.Org == nil {
		return id
	}
	if stored, _, found := storage.LookupRecordByID(vm.Org.Objects["PermissionSet"].Records, storage.ID(id)); found {
		return string(stored)
	}
	return id
}

func (vm *VM) currentUserObjectPermissionWithScope(objectName, method, permissionSetID string) bool {
	permissionSetID = vm.canonicalPermissionSetKey(permissionSetID)
	memo := vm.permissionChecks
	key := permissionCheckKey{object: objectName, method: method, scope: permissionSetID}
	if memo != nil {
		if allowed, ok := memo.objects[key]; ok {
			return allowed
		}
	}
	// A scoped permission set adds to the running user's own grants (native
	// rows, API 62 and 67): the user's access stays effective and the set
	// can only widen it.
	allowed := vm.currentUserObjectPermission(objectName, method)
	if !allowed && permissionSetID != "" {
		explicit, ok := vm.explicitObjectPermission(permissionSetID, objectName, method)
		allowed = ok && explicit
	}
	if memo != nil {
		if memo.objects == nil {
			memo.objects = make(map[permissionCheckKey]bool)
		}
		memo.objects[key] = allowed
	}
	return allowed
}

func (vm *VM) currentUserFieldPermissionWithScope(objectName, fieldName, method, permissionSetID string) bool {
	permissionSetID = vm.canonicalPermissionSetKey(permissionSetID)
	memo := vm.permissionChecks
	key := permissionCheckKey{object: objectName, field: fieldName, method: method, scope: permissionSetID}
	if memo != nil {
		if allowed, ok := memo.fields[key]; ok {
			return allowed
		}
	}
	allowed := vm.currentUserFieldPermission(objectName, fieldName, method)
	if !allowed && permissionSetID != "" {
		explicit, ok := vm.explicitFieldPermission(permissionSetID, objectName, fieldName, method)
		allowed = ok && explicit
		// A custom object's Text Name field has no field permission; the
		// scoped set's object grant carries it (native rows, API 62 and 67).
		if !allowed && vm.nameFieldHasNoFieldLevelPermissions(objectName, fieldName) {
			if objectAllowed, ok := vm.explicitObjectPermission(permissionSetID, objectName, fieldPermissionObjectMethod(method)); ok && objectAllowed {
				allowed = vm.stripInaccessibleIntrinsicFieldPermission(objectName, fieldName, method)
			}
		}
	}
	if memo != nil {
		if memo.fields == nil {
			memo.fields = make(map[permissionCheckKey]bool)
		}
		memo.fields[key] = allowed
	}
	return allowed
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
		if isInternalSObjectField(field) || isSObjectSystemField(field) && !isExplicitSObjectField(*record, field) && !isLoadedSObjectDefault(*record, field) && !readableOwner {
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
			// strip_omitted_defaults (API 55.0, 62.0, 67.0): an omitted checkbox stays
			// out of the removed set for custom and standard fields alike.
			if declared && definition.Type == storage.FieldBoolean &&
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
		if scopedPermissionSetID == "" && !isLoadedSObjectDefault(*record, field) {
			fieldAllowed = fieldAllowed || vm.stripInaccessibleKeepsBaselineWritableField(accessType, objectName, canonicalField, true)
		}
		if isExplicitSObjectField(*record, field) || isLoadedSObjectDefault(*record, field) || vm.queriedSObjectFieldsIncludes(*record, field) {
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
		// JSON subquery projections can carry an implicit join value without
		// making it a queried Apex field. Inspect it privately for stripping.
		selectedFields := selected.MapKeys
		if supplemental := record.Fields[jsonSObjectQueryProjectionField].Fields; len(supplemental) > 0 {
			selectedFields = maps.Clone(selectedFields)
			if selectedFields == nil {
				selectedFields = make(map[string]Value, len(supplemental))
			}
			for field := range supplemental {
				key := String(strings.ToLower(field))
				selectedFields[mapKey(key)] = key
			}
		}
		for _, selectedFieldValue := range selectedFields {
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
				fieldAllowed = fieldAllowed || vm.stripInaccessibleKeepsBaselineWritableField(accessType, objectName, canonicalField, false)
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
func (vm *VM) stripInaccessibleKeepsBaselineWritableField(accessType, objectName, fieldName string, fieldPresent bool) bool {
	switch strings.ToUpper(strings.TrimSpace(accessType)) {
	case "CREATABLE":
		if !vm.isBaselineWritableField(objectName, fieldName) {
			return false
		}
		// Native rows cover present custom Name fields with root CRUD
		// enforcement disabled. Other required and absent fields retain
		// the existing baseline until those routes have their own oracle.
		if fieldPresent && vm.ordinaryCustomPermissionObject(objectName) {
			_, field, known := vm.sObjectFieldDefinition(objectName, fieldName)
			if known && isNameFieldDescribe(field) {
				return vm.currentUserObjectPermission(objectName, "isCreateable")
			}
		}
		return true
	case "UPSERTABLE":
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
		if method == "isAccessible" && record.System.IsDeleted && vm.testContext != nil && vm.testContext.RunAsDepth > 0 {
			continue
		}
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
	return vm.assignedPermissionSetIDsFiltered(userID, false)
}

// permissionGrantSource is one assigned permission set and the muting sets of
// the group that supplied it. Muting affects that group's grant only; the same
// set assigned directly or through another group stays independent.
type permissionGrantSource struct {
	ID    string
	Mutes []string
}

func (vm *VM) assignedPermissionSetIDsFiltered(userID string, activeOnly bool) []string {
	sources := vm.assignedPermissionSources(userID, activeOnly)
	if len(sources) == 0 {
		return nil
	}
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		out = append(out, source.ID)
	}
	return out
}

func (vm *VM) assignedPermissionSources(userID string, activeOnly bool) []permissionGrantSource {
	if userID == "" || vm.Org == nil {
		return nil
	}
	state, ok := vm.Org.Objects["PermissionSetAssignment"]
	if !ok {
		return nil
	}
	var out []permissionGrantSource
	for _, record := range state.Records {
		if record.System.IsDeleted {
			continue
		}
		group, hasGroup := record.GetField("PermissionSetGroupId")
		if activeOnly && (!hasGroup || storageValueIDText(group) == "") {
			active, known := dml.PermissionAssignmentState(record, func() time.Time { return vm.fakeNow })
			if known && !active {
				continue
			}
		}
		if assigneeValue, ok := record.GetField("AssigneeId"); ok && storage.IDsEqual(storage.ID(storageValueIDText(assigneeValue)), storage.ID(userID)) {
			if permissionSetValue, ok := record.GetField("PermissionSetId"); ok {
				if id := storageValueIDText(permissionSetValue); id != "" {
					// Text conversion can expand an Id to eighteen characters.
					// Existing permission rows still refer to the stored set key.
					if storedID, _, found := storage.LookupRecordByID(vm.Org.Objects["PermissionSet"].Records, storage.ID(id)); found {
						id = string(storedID)
					}
					out = append(out, permissionGrantSource{ID: id})
				}
			}
			if permissionSetGroupValue, ok := record.GetField("PermissionSetGroupId"); ok {
				members, mutes := vm.permissionSetGroupGrantSets(storageValueIDText(permissionSetGroupValue))
				for _, member := range members {
					out = append(out, permissionGrantSource{ID: member, Mutes: mutes})
				}
			}
		}
	}
	return out
}

// sourceObjectPermission reports an explicit object grant on the source that
// none of its group's muting sets removes (same permission flag).
func (vm *VM) sourceObjectPermission(source permissionGrantSource, objectName, method string) bool {
	if allowed, ok := vm.explicitObjectPermission(source.ID, objectName, method); !ok || !allowed {
		return false
	}
	for _, mute := range source.Mutes {
		if muted, ok := vm.explicitObjectPermission(mute, objectName, method); ok && muted {
			return false
		}
	}
	return true
}

func (vm *VM) sourceFieldPermission(source permissionGrantSource, objectName, fieldName, method string) bool {
	if allowed, ok := vm.explicitFieldPermission(source.ID, objectName, fieldName, method); !ok || !allowed {
		return false
	}
	for _, mute := range source.Mutes {
		if muted, ok := vm.explicitFieldPermission(mute, objectName, fieldName, method); ok && muted {
			return false
		}
	}
	return true
}

func (vm *VM) permissionSetGroupComponentIDs(groupID string) []string {
	members, _ := vm.permissionSetGroupGrantSets(groupID)
	return members
}

// permissionSetGroupGrantSets splits a group's components into granting sets
// and the muting sets recorded in MutingPermissionSet.
func (vm *VM) permissionSetGroupGrantSets(groupID string) (members, mutes []string) {
	if groupID == "" || vm.Org == nil {
		return nil, nil
	}
	state, ok := vm.Org.Objects["PermissionSetGroupComponent"]
	if !ok {
		return nil, nil
	}
	muting := vm.Org.Objects["MutingPermissionSet"].Records
	for _, record := range state.Records {
		if groupValue, ok := record.GetField("PermissionSetGroupId"); !ok || !storageIDValueEquals(groupValue, groupID) {
			continue
		}
		if permissionSetValue, ok := record.GetField("PermissionSetId"); ok {
			id := storageValueIDText(permissionSetValue)
			if id == "" {
				continue
			}
			if _, mute, isMute := storage.LookupRecordByID(muting, storage.ID(id)); isMute {
				if !mute.System.IsDeleted {
					mutes = append(mutes, id)
				}
				continue
			}
			members = append(members, id)
		}
	}
	return members, mutes
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
	_, profile, ok := storage.LookupRecordByID(profiles.Records, storage.ID(profileID))
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
	_, profile, ok := storage.LookupRecordByID(profiles.Records, storage.ID(profileID))
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
	_, profile, ok := storage.LookupRecordByID(profiles.Records, storage.ID(profileID))
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

// deployedPermissionSetIDs lists the stored keys of every permission set that
// is not owned by a profile.
func (vm *VM) deployedPermissionSetIDs() []string {
	if vm == nil || vm.Org == nil {
		return nil
	}
	state, ok := vm.Org.Objects["PermissionSet"]
	if !ok {
		return nil
	}
	var out []string
	for id, record := range state.Records {
		if owned, ok := record.Fields["IsOwnedByProfile"]; ok && owned.Kind == storage.ValueBoolean && owned.Boolean {
			continue
		}
		if record.ID != "" {
			out = append(out, string(record.ID))
		} else {
			out = append(out, string(id))
		}
	}
	return out
}
