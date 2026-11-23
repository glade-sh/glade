package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A runAs user reaches a custom metadata type only through a Custom Metadata
// Types grant of an assigned permission set or of the profile, or through the
// administrator's View All Data (native rows share_mdt_profile_grant for a
// custom profile). The grant covers every field of the type,
// MasterLabel, DeveloperName and custom fields alike; field permissions play
// no part. Without the grant the type is not accessible, the describe reports
// false for the type and every field, and a USER_MODE query raises
// "sObject type 'T__mdt' is not supported". Native rows:
// docs/native-evidence/share_mdt_native_fixtures/native-run.md (API 62.0, 67.0).
//
// The check applies only to an explicit runAs user in a project without a
// namespace: namespaced projects load no grant rows and no native row covers
// them, so they keep the unrestricted read they had before. The default test
// user and anonymous execution keep it too.
func (vm *VM) customMetadataTypeAccess(objectName string) (granted, applicable bool) {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.testContext == nil || vm.testContext.RunAsDepth == 0 {
		return false, false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok || !storage.IsCustomMetadataDefinition(vm.Org.Objects[canonical].Definition) {
		return false, false
	}
	if vm.currentUserBypassesRecordSharing() {
		return true, true
	}
	user := vm.executionUser
	if vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	profileSet := vm.profileOwnedPermissionSetID(stringField(user, "ProfileId"))
	sources := []string{}
	if profileSet != "" {
		sources = append(sources, profileSet)
	}
	for _, source := range vm.assignedPermissionSources(stringField(user, "Id"), false) {
		sources = append(sources, source.ID)
	}
	for _, row := range vm.Org.Objects["SetupEntityAccess"].Records {
		if row.System.IsDeleted || !storageStringValueEquals(row.Fields["SetupEntityType"], "CustomEntityDefinition") ||
			!strings.EqualFold(strings.TrimSpace(row.Fields["SetupEntityId"].String), canonical) {
			continue
		}
		parent, ok := row.GetField("ParentId")
		if !ok {
			continue
		}
		for _, source := range sources {
			if storage.IDsEqual(storage.ID(storageValueIDText(parent)), storage.ID(source)) {
				return true, true
			}
		}
	}
	return false, true
}
