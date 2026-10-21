package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

const sharingHiddenParentNullRuntime = "__glade_sharing_hidden_parent_null"

// A private custom self-lookup's projected ancestors use their own record
// access, independently of the leaf's share, as observed in native queries.
// Only the returned projection is changed; lookup IDs and query inputs survive.
func (vm *VM) applyPrivateSelfLookupSharing(value *Value, objectName, userID string) {
	if vm == nil || vm.Org == nil || value == nil || value.Kind != ValueObject || userID == "" {
		return
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok || !hasSuffixFold(canonical, "__c") {
		return
	}
	definition := vm.Org.Objects[canonical].Definition
	if !strings.EqualFold(strings.TrimSpace(definition.SharingModel), "Private") || storage.IsCustomSettingDefinition(definition) {
		return
	}
	for name, parent := range value.Fields {
		if parent.Kind != ValueObject || !objectBoolField(parent, sobjectParentProjectionField) {
			continue
		}
		relation, ok := vm.parentRelationshipMetadata(canonical, name)
		if !ok || relation.Polymorphic || len(relation.ParentObjects) != 1 || !strings.EqualFold(relation.ParentObjects[0], canonical) {
			continue
		}
		id := sObjectIDFromFields(parent.Fields)
		record, ok := vm.findOrgRecord(canonical, id)
		if !ok {
			continue
		}
		if !vm.userModeRecordVisible(canonical, record, userID) {
			hidden := vm.parentRelationshipTypedNull(relation)
			hidden.Runtime = sharingHiddenParentNullRuntime
			value.Fields[name] = hidden
			continue
		}
		vm.applyPrivateSelfLookupSharing(&parent, canonical, userID)
	}
}
