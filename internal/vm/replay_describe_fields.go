package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured rows cover this public standard event, independently of custom
// publishBehavior metadata. A source class with the same name is not a record.
func (vm *VM) replayDescribeStandardEvent(objectName string) bool {
	_, definition, ok := vm.describeObjectDefinition(objectName)
	return ok && strings.EqualFold(definition.APIName, "BatchApexErrorEvent") && storage.IsKnownStandardObject(definition.APIName)
}

// The captured rows: overlay only the captured field kinds.
// Permission checks remain in the ordinary describe path; metadata still owns
// accessible/createable flags for custom fields.
func (vm *VM) replayDescribeFieldFlags(desc *Value, objectName string, field storage.Field) {
	event := vm.hasPlatformEventMetadata(objectName) || vm.replayDescribeStandardEvent(objectName)
	if event {
		switch strings.ToLower(field.APIName) {
		case "replayid", "eventuuid":
			desc.Fields["createable"] = Bool(false)
			desc.Fields["updateable"] = Bool(false)
			desc.Fields["filterable"] = Bool(false)
			desc.Fields["sortable"] = Bool(false)
			desc.Fields["nillable"] = Bool(true)
			desc.Fields["defaultedOnCreate"] = Bool(false)
			desc.Fields["permissionable"] = Bool(false)
		case "createddate", "createdbyid":
			desc.Fields["createable"] = Bool(false)
			desc.Fields["updateable"] = Bool(false)
			desc.Fields["filterable"] = Bool(false)
			desc.Fields["sortable"] = Bool(false)
			desc.Fields["nillable"] = Bool(false)
			desc.Fields["defaultedOnCreate"] = Bool(true)
			desc.Fields["permissionable"] = Bool(false)
		default:
			if isCustomSchemaName(field.APIName) && field.Type == storage.FieldString && !field.Required && !field.AutoNumber && strings.TrimSpace(field.Formula) == "" && (field.DisplayType == "" || strings.EqualFold(field.DisplayType, "STRING")) {
				desc.Fields["updateable"] = Bool(false)
				desc.Fields["filterable"] = Bool(false)
				desc.Fields["sortable"] = Bool(false)
				desc.Fields["permissionable"] = Bool(false)
			}
		}
		return
	}
	// The captured rows are ordinary custom-record audit-field boundaries.
	_, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok || !isCustomObjectLikeName(definition.APIName) || storage.IsCustomSettingDefinition(definition) || storage.IsCustomMetadataDefinition(definition) {
		return
	}
	if strings.EqualFold(field.APIName, "CreatedDate") || strings.EqualFold(field.APIName, "CreatedById") {
		desc.Fields["nillable"] = Bool(false)
		desc.Fields["defaultedOnCreate"] = Bool(true)
		desc.Fields["permissionable"] = Bool(false)
	}
}
