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
		if !ok || !storage.IDsEqual(storage.ID(storageValueIDText(sharedUser)), storage.ID(userID)) {
			continue
		}
		if access, ok := share.GetField("AccountAccessLevel"); ok && strings.EqualFold(access.String, "None") {
			continue
		}
		return true
	}
	return false
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
