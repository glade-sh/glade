package vm

import (
	"fmt"
	"sort"
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
	defer vm.beginPermissionCheckMemo()()
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
		objectDenied := !vm.currentUserObjectPermissionWithScope(objectName, permission, scope)
		var deniedFields []string
		fieldNames := make([]string, 0, len(record.Fields))
		for field := range record.Fields {
			fieldNames = append(fieldNames, field)
		}
		sort.Strings(fieldNames)
		// Explicit assignment order first (the marker holds lowercase names),
		// then the remaining fields alphabetically.
		order := map[string]int{}
		for i, field := range explicitSObjectFieldNamesInInsertionOrder(record) {
			order[strings.ToLower(field)] = i
		}
		sort.SliceStable(fieldNames, func(i, j int) bool {
			a, aok := order[strings.ToLower(fieldNames[i])]
			b, bok := order[strings.ToLower(fieldNames[j])]
			if aok && bok {
				return a < b
			}
			return aok && !bok
		})
		for _, field := range fieldNames {
			if objectDenied {
				break
			}
			if isInternalSObjectField(field) || isSObjectSystemField(field) || isDefaultedSObjectField(record, field) {
				continue
			}
			if record.Fields[field].Kind == ValueList && vm.childRelationshipLookup(objectName, field).ChildType != "" {
				continue
			}
			canonicalField := vm.resolveSObjectFieldName(objectName, field)
			// Native shapemisc_capture fm rows: Database.upsert(..., USER_MODE)
			// accepts formula and auto-number values the admin has no grant on.
			if strings.EqualFold(canonicalField, "Id") || vm.sobjectFormulaField(objectName, canonicalField) {
				continue
			}
			if !vm.currentUserFieldPermissionWithScope(objectName, canonicalField, permission, scope) {
				deniedFields = append(deniedFields, canonicalField)
			}
		}
		if !objectDenied && len(deniedFields) == 0 {
			continue
		}
		if !objectDenied {
			// Native partial_dml_multifield upsert throw rows: the exception
			// lists every denied field on the row, in assignment order.
			return userModeFieldDMLException(op, objectName, record, deniedFields, index)
		}
		message := fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result! ", objectName)
		return newExceptionError("System.DmlException", message)
	}
	return nil
}
