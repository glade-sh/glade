package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func standardPlatformUserObjectPermission(objectName, method string) bool {
	if strings.EqualFold(objectName, "Case") {
		return false
	}
	return true
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
		prefix = storage.StandardKeyPrefixes()["PermissionSetAssignment"]
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
