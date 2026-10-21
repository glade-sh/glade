package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Custom share objects associate ParentId with a direct UserOrGroupId grant.
// Only the documented Read/Edit levels and direct-user recipients are handled
// here; group expansion and deletion authority remain separate.
func (vm *VM) currentUserHasDirectCustomShare(objectName string, recordID storage.ID, userID string, edit bool) bool {
	if vm == nil || vm.Org == nil || recordID == "" || userID == "" {
		return false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok || !hasSuffixFold(canonical, "__c") || storage.IsCustomSettingDefinition(vm.Org.Objects[canonical].Definition) {
		return false
	}
	shareName, ok := storage.ResolveObjectName(*vm.Org, canonical[:len(canonical)-3]+"__Share")
	if !ok {
		return false
	}
	for _, share := range vm.Org.Objects[shareName].Records {
		if share.System.IsDeleted {
			continue
		}
		parent, parentOK := share.GetField("ParentId")
		recipient, recipientOK := share.GetField("UserOrGroupId")
		access, accessOK := share.GetField("AccessLevel")
		if !parentOK || !recipientOK || !accessOK ||
			!storage.IDsEqual(storage.ID(storageValueIDText(parent)), recordID) ||
			!storage.IDsEqual(storage.ID(storageValueIDText(recipient)), storage.ID(userID)) {
			continue
		}
		if strings.EqualFold(access.String, "Edit") || !edit && strings.EqualFold(access.String, "Read") {
			return true
		}
	}
	return false
}
