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
