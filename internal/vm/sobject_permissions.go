package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// standardPlatformUserObjectPermission follows the owned standard_profile_object_crud
// rows (API 62.0 and 67.0, plain Developer org): the Standard Platform User
// profile has no access to the sales and service objects below and full access to
// Account, Contact, Task and Event. Objects without an owned row stay permitted.
func standardPlatformUserObjectPermission(objectName, method string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) {
	case "case", "lead", "campaign", "opportunity", "product2", "contract", "order", "authprovider":
		return false
	}
	return true
}

// standardUserObjectPermission follows the same owned rows for the Standard User
// profile: Campaign and Product2 are read-only, Case cannot be deleted and
// the AuthProvider setup object is not accessible.
func standardUserObjectPermission(objectName, method string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) {
	case "authprovider":
		return false
	case "campaign", "product2":
		return method == "isAccessible"
	case "case":
		return method != "isDeletable"
	}
	return true
}

// standardProfileHiddenOptOutField reports the communication opt-out fields that
// the owned rows show as inaccessible (not readable, creatable or updateable) in
// a plain Developer org. The administrator rows are not modeled.
func standardProfileHiddenOptOutField(objectName, fieldName string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) + "." + strings.ToLower(strings.TrimSpace(fieldName)) {
	case "lead.donotcall", "lead.hasoptedoutofemail", "contact.donotcall":
		return true
	}
	return false
}

// standardUserManagedUserField lists the User fields a Standard User can read
// but neither create nor update: native user_self_update describeAsStandard
// rows (API 62.0 and 67.0) report createable and updateable false for these
// and true for the other probed User fields (Username, Alias, names, Email,
// Phone, Title, locale and time zone keys, CommunityNickname, Department,
// UserRoleId, IsActive).
func standardUserManagedUserField(objectName, fieldName string) bool {
	if !strings.EqualFold(objectName, "User") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fieldName)) {
	case "profileid", "managerid", "userpermissionsmarketinguser":
		return true
	}
	return false
}

func isBaselineReadableObject(objectName string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) {
	case "task", "event":
		return true
	default:
		return false
	}
}

func (vm *VM) standardPlatformUserFieldPermission(objectName, fieldName, method string) bool {
	if strings.EqualFold(objectName, "Case") {
		if strings.EqualFold(fieldName, "CaseNumber") {
			return false
		}
		return vm.isBaselineReadableField(objectName, fieldName)
	}
	return true
}

func stripInaccessibleKeepsSObjectBaselineWritableUpdateField(objectName, fieldName string) bool {
	return strings.EqualFold(objectName, "Lead") && leadCommunicationOptOutField(fieldName)
}

func leadCommunicationOptOutField(fieldName string) bool {
	switch strings.ToLower(strings.TrimSpace(fieldName)) {
	case "donotcall", "hasoptedoutofemail", "hasoptedoutoffax":
		return true
	default:
		return false
	}
}

func (vm *VM) ensureUserProfilePermissionSetAssignment(record storage.Record) {
	if vm == nil || vm.Org == nil || record.ID == "" {
		return
	}
	profileID := storageValueIDText(record.Fields["ProfileId"])
	if profileID == "" {
		return
	}
	permissionSetID := vm.profileOwnedPermissionSetID(profileID)
	if permissionSetID == "" {
		return
	}
	storage.EnsureStandardObject(vm.Org, "PermissionSetAssignment")
	state := vm.Org.Objects["PermissionSetAssignment"]
	for _, existing := range state.Records {
		if storage.IDsEqual(storage.ID(storageValueIDText(existing.Fields["AssigneeId"])), record.ID) &&
			storage.IDsEqual(storage.ID(storageValueIDText(existing.Fields["PermissionSetId"])), storage.ID(permissionSetID)) {
			return
		}
	}
	prefix := state.Definition.KeyPrefix
	if prefix == "" {
		prefix = storage.StandardKeyPrefix("PermissionSetAssignment")
	}
	generator := storage.NewRuntimeIDGenerator(map[string]string{"PermissionSetAssignment": prefix})
	generator.Sequences = copyOrgIDSequences(vm.Org.IDSequences)
	vm.recordIsolationJournalSequence("PermissionSetAssignment")
	id, err := generator.Next("PermissionSetAssignment")
	if err != nil {
		return
	}
	if _, cloned := storage.EnsureMutableObjectRecords(vm.Org, "PermissionSetAssignment"); cloned {
		state = vm.Org.Objects["PermissionSetAssignment"]
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.recordIsolationJournalMutation("PermissionSetAssignment", id, storage.Record{}, false)
	state.Records[id] = storage.Record{
		ID:     id,
		Object: "PermissionSetAssignment",
		Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue(record.ID),
			"PermissionSetId": storage.IDValue(storage.ID(permissionSetID)),
		},
	}
	vm.Org.IDSequences = copyOrgIDSequences(generator.Sequences)
	vm.Org.Objects["PermissionSetAssignment"] = state
}

// Owned RG01-RG03 observations expose both group types immediately after role
// insertion (API 62/67/68, anonymous and named, default Developer org).
func (vm *VM) ensureUserRoleGroup(record storage.Record) {
	vm.ensureUserRoleGroupType(record, "Role")
	vm.ensureUserRoleGroupType(record, "RoleAndSubordinatesInternal")
}

func (vm *VM) ensureUserRoleGroupType(record storage.Record, groupType string) {
	if vm == nil || vm.Org == nil || record.ID == "" || !strings.EqualFold(record.Object, "UserRole") {
		return
	}
	storage.EnsureStandardObject(vm.Org, "Group")
	state := vm.Org.Objects["Group"]
	for _, existing := range state.Records {
		if strings.EqualFold(storageValueIDText(existing.Fields["RelatedId"]), string(record.ID)) && strings.EqualFold(existing.Fields["Type"].String, groupType) {
			return
		}
	}
	prefix := state.Definition.KeyPrefix
	if prefix == "" {
		prefix = storage.StandardKeyPrefix("Group")
	}
	generator := storage.NewRuntimeIDGenerator(map[string]string{"Group": prefix})
	generator.Sequences = copyOrgIDSequences(vm.Org.IDSequences)
	vm.recordIsolationJournalSequence("Group")
	id, err := generator.Next("Group")
	if err != nil {
		return
	}
	if _, cloned := storage.EnsureMutableObjectRecords(vm.Org, "Group"); cloned {
		state = vm.Org.Objects["Group"]
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	fields := map[string]storage.Value{
		"Type": storage.StringValue(groupType), "RelatedId": storage.IDValue(record.ID),
	}
	if groupType == "Role" {
		fields["Name"] = storage.StringValue(record.Fields["Name"].String)
	}
	vm.recordIsolationJournalMutation("Group", id, storage.Record{}, false)
	state.Records[id] = storage.Record{ID: id, Object: "Group", Fields: fields}
	vm.Org.IDSequences = copyOrgIDSequences(generator.Sequences)
	vm.Org.Objects["Group"] = state
}

func (vm *VM) profileOwnedPermissionSetID(profileID string) string {
	if vm == nil || vm.Org == nil || strings.TrimSpace(profileID) == "" {
		return ""
	}
	state, ok := vm.Org.Objects["PermissionSet"]
	if !ok {
		return ""
	}
	for id, record := range state.Records {
		ownedByProfile, ok := record.Fields["IsOwnedByProfile"]
		if !ok || ownedByProfile.Kind != storage.ValueBoolean || !ownedByProfile.Boolean {
			continue
		}
		if storage.IDsEqual(storage.ID(storageValueIDText(record.Fields["ProfileId"])), storage.ID(profileID)) {
			if record.ID != "" {
				return string(record.ID)
			}
			return string(id)
		}
	}
	return ""
}

// undeletableFollowsDeletable reports whether describe isUndeletable tracks the
// profile's delete permission. The owned standard_profile_object_crud rows show
// isUndeletable equal to isDeletable for Standard User and Standard Platform User.
func (vm *VM) undeletableFollowsDeletable(user Value) bool {
	switch vm.currentUserProfileName(user) {
	case "Standard User", "Standard Platform User":
		return true
	}
	return false
}
