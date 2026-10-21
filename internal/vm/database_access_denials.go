package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// databaseAccessLevelDenial preserves Database's all-or-none USER_MODE
// exceptions without changing the statement and trigger security checks.
func (vm *VM) databaseAccessLevelDenial(op string, value Value, accessLevel Value) error {
	if !isUserModeAccessLevel(accessLevel) || (op != "upsert" && op != "undelete") {
		return nil
	}
	scope := accessLevelPermissionSetID(accessLevel)
	for index, record := range dmlAccessRecords(value) {
		if record.Kind != ValueObject {
			continue
		}
		objectName := vm.canonicalSObjectValueType(record)
		_, idValue, _ := objectFieldValue(record, "Id")
		id, _ := idValueText(idValue)
		if op == "undelete" {
			stored, found := vm.findOrgRecord(objectName, storage.ID(id))
			if !found || !stored.System.IsDeleted {
				continue
			}
			if vm.currentUserObjectPermissionWithScope(objectName, "isDeletable", scope) &&
				vm.currentUserCanWriteRecord(objectName, stored, vm.currentUserID(), op) {
				continue
			}
			results := make([]dml.Result, index+1)
			results[index] = dml.Result{ID: stored.ID, Error: "invalid record id", StatusCode: "INVALID_ID_FIELD"}
			return statementDMLException(op, results, vm.dmlExceptionObjectTypes(value))
		}
		permission := "isCreateable"
		if id != "" {
			permission = "isUpdateable"
		}
		denied := !vm.currentUserObjectPermissionWithScope(objectName, permission, scope)
		for field := range record.Fields {
			if denied {
				break
			}
			if isInternalSObjectField(field) || isSObjectSystemField(field) || isDefaultedSObjectField(record, field) {
				continue
			}
			if record.Fields[field].Kind == ValueList && vm.childRelationshipLookup(objectName, field).ChildType != "" {
				continue
			}
			canonicalField := vm.resolveSObjectFieldName(objectName, field)
			if strings.EqualFold(canonicalField, "Id") {
				continue
			}
			denied = !vm.currentUserFieldPermissionWithScope(objectName, canonicalField, permission, scope)
		}
		if !denied {
			continue
		}
		message := fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result! ", objectName)
		return newExceptionError("System.DmlException", message)
	}
	return nil
}
