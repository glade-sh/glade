package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) applyAccountDescribeDefaults(definition *storage.ObjectDefinition) {
	if definition == nil || !strings.EqualFold(definition.APIName, "Account") {
		return
	}
	if len(definition.RecordTypes) == 0 {
		storage.EnsureStandardObjectFieldsForFeatures(definition, []string{"PersonAccounts"})
	}
	if vm == nil || vm.Org == nil {
		return
	}
	if personAccountName, ok := vm.resolveObjectName("PersonAccount"); ok {
		personAccount := vm.Org.Objects[personAccountName]
		definition.RecordTypes = appendMissingRecordTypes(definition.RecordTypes, personAccount.Definition.RecordTypes)
	}
}

func fieldSetMemberDBRequired(objectName, fieldName string) bool {
	return strings.EqualFold(objectName, "Account") && strings.EqualFold(fieldName, "LastName")
}
