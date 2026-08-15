package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) currentUserCanSeeSharedRecord(objectName string, record storage.Record, userID string) bool {
	if strings.EqualFold(objectName, "User") || strings.EqualFold(record.Object, "User") {
		return storage.IDsEqual(record.ID, storage.ID(userID))
	}
	if strings.EqualFold(objectName, "PermissionSetAssignment") || strings.EqualFold(record.Object, "PermissionSetAssignment") {
		return vm.permissionSetAssignmentVisibleToUser(record, userID)
	}
	if strings.EqualFold(objectName, "Account") || strings.EqualFold(record.Object, "Account") {
		return storage.IDsEqual(record.ID, storage.ID(vm.currentUserContactAccountID())) ||
			vm.currentUserHasDirectAccountShare(record.ID, userID)
	}
	return false
}

func (vm *VM) userModeRecordVisible(objectName string, record storage.Record, userID string) bool {
	if vm == nil || vm.soqlObjectHasPublicReadSharing(objectName) || vm.currentUserBypassesRecordSharing() || userID == "" {
		return true
	}
	return record.System.OwnerID == "" || storage.IDsEqual(record.System.OwnerID, storage.ID(userID)) || vm.currentUserCanSeeSharedRecord(objectName, record, userID)
}

func (vm *VM) currentUserHasDirectAccountShare(accountID storage.ID, userID string) bool {
	if vm == nil || vm.Org == nil || accountID == "" || userID == "" {
		return false
	}
	state, ok := vm.Org.Objects["AccountShare"]
	if !ok {
		return false
	}
	for _, share := range state.Records {
		sharedAccount, ok := share.GetField("AccountId")
		if !ok || !storage.IDsEqual(storage.ID(storageValueIDText(sharedAccount)), accountID) {
			continue
		}
		sharedUser, ok := share.GetField("UserOrGroupId")
		if !ok {
			continue
		}
		if access, ok := share.GetField("AccountAccessLevel"); ok && strings.EqualFold(access.String, "None") {
			continue
		}
		sharedTarget := storage.ID(storageValueIDText(sharedUser))
		if storage.IDsEqual(sharedTarget, storage.ID(userID)) || vm.currentUserCanSeeAccountShareTarget(sharedTarget, userID) {
			return true
		}
	}
	return false
}

func (vm *VM) currentUserCanSeeAccountShareTarget(targetID storage.ID, userID string) bool {
	if targetID == "" || userID == "" {
		return false
	}
	return vm.currentUserInPublicGroup(targetID, userID, make(map[storage.ID]bool)) ||
		vm.currentUserInRoleGroup(targetID, userID)
}

func (vm *VM) currentUserInPublicGroup(groupID storage.ID, userID string, visited map[storage.ID]bool) bool {
	if vm == nil || vm.Org == nil || groupID == "" || visited[groupID] {
		return false
	}
	groups, ok := vm.Org.Objects["Group"]
	if !ok {
		return false
	}
	group, ok := groups.Records[groupID]
	if !ok {
		return false
	}
	typeValue, ok := group.GetField("Type")
	if !ok || !strings.EqualFold(typeValue.String, "Regular") {
		return false
	}
	visited[groupID] = true
	members, ok := vm.Org.Objects["GroupMember"]
	if !ok {
		return false
	}
	for _, member := range members.Records {
		memberGroup, ok := member.GetField("GroupId")
		if !ok || !storage.IDsEqual(storage.ID(storageValueIDText(memberGroup)), groupID) {
			continue
		}
		memberTarget, ok := member.GetField("UserOrGroupId")
		if !ok {
			continue
		}
		targetID := storage.ID(storageValueIDText(memberTarget))
		if storage.IDsEqual(targetID, storage.ID(userID)) || vm.currentUserInPublicGroup(targetID, userID, visited) {
			return true
		}
	}
	return false
}

func (vm *VM) currentUserInRoleGroup(groupID storage.ID, userID string) bool {
	if vm == nil || vm.Org == nil || groupID == "" || userID == "" {
		return false
	}
	groups, ok := vm.Org.Objects["Group"]
	if !ok {
		return false
	}
	group, ok := groups.Records[groupID]
	if !ok {
		return false
	}
	typeValue, ok := group.GetField("Type")
	if !ok || !strings.EqualFold(typeValue.String, "Role") {
		return false
	}
	roleValue, ok := group.GetField("RelatedId")
	if !ok {
		return false
	}
	targetRoleID := storage.ID(storageValueIDText(roleValue))
	currentRoleID := vm.currentUserRoleID(userID)
	if targetRoleID == "" || currentRoleID == "" {
		return false
	}
	roles, ok := vm.Org.Objects["UserRole"]
	if !ok {
		return false
	}
	visited := make(map[storage.ID]bool)
	for currentRoleID != "" && !visited[currentRoleID] {
		if storage.IDsEqual(currentRoleID, targetRoleID) {
			return true
		}
		visited[currentRoleID] = true
		role, ok := roles.Records[currentRoleID]
		if !ok {
			return false
		}
		parent, ok := role.GetField("ParentRoleId")
		if !ok {
			return false
		}
		currentRoleID = storage.ID(storageValueIDText(parent))
	}
	return false
}

func (vm *VM) currentUserRoleID(userID string) storage.ID {
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if roleID := stringField(user, "UserRoleId"); roleID != "" {
		return storage.ID(roleID)
	}
	if vm == nil || vm.Org == nil {
		return ""
	}
	users, ok := vm.Org.Objects["User"]
	if !ok {
		return ""
	}
	stored, ok := users.Records[storage.ID(userID)]
	if !ok {
		return ""
	}
	role, ok := stored.GetField("UserRoleId")
	if !ok {
		return ""
	}
	return storage.ID(storageValueIDText(role))
}

func (vm *VM) permissionSetAssignmentVisibleToUser(record storage.Record, userID string) bool {
	if permissionSetAssignmentRecordVisibleToUser(record, userID) {
		return true
	}
	if vm == nil || vm.Org == nil || record.ID == "" {
		return false
	}
	state, ok := vm.Org.Objects["PermissionSetAssignment"]
	if !ok {
		return false
	}
	_, stored, ok := storage.LookupRecordByID(state.Records, record.ID)
	return ok && permissionSetAssignmentRecordVisibleToUser(stored, userID)
}

func permissionSetAssignmentRecordVisibleToUser(record storage.Record, userID string) bool {
	if userID == "" {
		return false
	}
	assignee, ok := record.GetField("AssigneeId")
	return ok && storage.IDsEqual(storage.ID(storageValueIDText(assignee)), storage.ID(userID))
}

func (vm *VM) currentUserContactAccountID() string {
	if vm == nil || vm.Org == nil {
		return ""
	}
	contactID := strings.TrimSpace(vm.currentUserInfoField("ContactId", ""))
	if contactID == "" {
		return ""
	}
	contacts, ok := vm.Org.Objects["Contact"]
	if !ok {
		return ""
	}
	contact, ok := contacts.Records[storage.ID(contactID)]
	if !ok {
		return ""
	}
	value, ok := contact.GetField("AccountId")
	if !ok || value.Kind != storage.ValueID {
		return ""
	}
	return string(value.ID)
}

func standardObjectDefaultsToPublicRead(objectName string) bool {
	name := strings.TrimSpace(objectName)
	switch {
	case strings.EqualFold(name, "Campaign"):
		return true
	default:
		return false
	}
}
