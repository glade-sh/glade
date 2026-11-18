package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured grant covers an owner below the reader in the role tree on an
// explicitly enabled private custom object. Unknown org settings fail closed.
func (vm *VM) currentUserHasHierarchyOwnerRead(objectName string, record storage.Record, userID string) bool {
	if vm == nil || vm.Org == nil || record.System.OwnerID == "" || userID == "" {
		return false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok || !hasSuffixFold(canonical, "__c") {
		return false
	}
	definition := vm.Org.Objects[canonical].Definition
	if !strings.EqualFold(definition.SharingModel, "Private") || definition.Metadata["gladeHierarchyAccess"] != "true" || storage.IsCustomSettingDefinition(definition) || storage.IsCustomMetadataDefinition(definition) {
		return false
	}
	return vm.currentUserCanSeeUserThroughRoleHierarchy(record.System.OwnerID, userID)
}

// The native owner-rule rows cover Read rules between direct members of
// public groups. Resolve the current memberships rather than trusting a share
// row, which can lag behind membership changes in the local DML model.
func (vm *VM) currentUserHasOwnerRuleRead(objectName string, record storage.Record, userID string) bool {
	if vm == nil || vm.Org == nil || record.System.OwnerID == "" || userID == "" {
		return false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok || !hasSuffixFold(canonical, "__c") {
		return false
	}
	definition := vm.Org.Objects[canonical].Definition
	if !strings.EqualFold(definition.SharingModel, "Private") || storage.IsCustomSettingDefinition(definition) || storage.IsCustomMetadataDefinition(definition) {
		return false
	}
	for _, rule := range definition.OwnerSharingRules {
		if !strings.EqualFold(rule.AccessLevel, "Read") {
			continue
		}
		var source, target storage.ID
		for _, group := range vm.Org.Objects["Group"].Records {
			if group.System.IsDeleted || !strings.EqualFold(group.Fields["Type"].String, "Regular") {
				continue
			}
			if strings.EqualFold(group.Fields["DeveloperName"].String, rule.SourceGroup) {
				source = group.ID
			}
			if strings.EqualFold(group.Fields["DeveloperName"].String, rule.TargetGroup) {
				target = group.ID
			}
		}
		if source == "" || target == "" {
			continue
		}
		ownerMember, readerMember := false, false
		for _, member := range vm.Org.Objects["GroupMember"].Records {
			if member.System.IsDeleted {
				continue
			}
			group, groupOK := member.GetField("GroupId")
			user, userOK := member.GetField("UserOrGroupId")
			if !groupOK || !userOK {
				continue
			}
			groupID, memberID := storage.ID(storageValueIDText(group)), storage.ID(storageValueIDText(user))
			ownerMember = ownerMember || storage.IDsEqual(groupID, source) && storage.IDsEqual(memberID, record.System.OwnerID)
			readerMember = readerMember || storage.IDsEqual(groupID, target) && storage.IDsEqual(memberID, storage.ID(userID))
		}
		if ownerMember && readerMember {
			return true
		}
	}
	return false
}

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

// A criteria rule grants Read to its target group through a Rule row on the
// object's share table, written when a transaction commits. Read the committed
// row, not the rule: a record that matches but has not committed has no row and
// no access, the same as natively inside its own transaction. Case is the one
// captured object. The reader holds the grant as a direct user or a direct
// member of the group.
func (vm *VM) currentUserHasCriteriaRuleRead(objectName string, record storage.Record, userID string) bool {
	if vm == nil || vm.Org == nil || record.ID == "" || userID == "" {
		return false
	}
	if canonical, ok := vm.resolveObjectName(objectName); !ok || canonical != "Case" {
		return false
	}
	shares := vm.Org.Objects["CaseShare"].Records
	if len(shares) == 0 {
		return false
	}
	for _, share := range shares {
		if share.System.IsDeleted {
			continue
		}
		cause, _ := share.GetField("RowCause")
		parent, _ := share.GetField("CaseId")
		recipient, _ := share.GetField("UserOrGroupId")
		access, _ := share.GetField("CaseAccessLevel")
		if cause.String != "Rule" || !storage.IDsEqual(storage.ID(storageValueIDText(parent)), record.ID) {
			continue
		}
		if !strings.EqualFold(access.String, "Read") && !strings.EqualFold(access.String, "Edit") && !strings.EqualFold(access.String, "All") {
			continue
		}
		target := storage.ID(storageValueIDText(recipient))
		if storage.IDsEqual(target, storage.ID(userID)) || vm.userIsDirectGroupMember(target, storage.ID(userID)) {
			return true
		}
	}
	return false
}

func (vm *VM) userIsDirectGroupMember(group, user storage.ID) bool {
	for _, member := range vm.Org.Objects["GroupMember"].Records {
		if member.System.IsDeleted {
			continue
		}
		groupID, groupOK := member.GetField("GroupId")
		userValue, userOK := member.GetField("UserOrGroupId")
		if groupOK && userOK && storage.IDsEqual(storage.ID(storageValueIDText(groupID)), group) && storage.IDsEqual(storage.ID(storageValueIDText(userValue)), user) {
			return true
		}
	}
	return false
}
