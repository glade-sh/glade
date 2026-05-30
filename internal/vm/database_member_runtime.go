package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func callDatabaseResultObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if !databaseResultObjectLike(receiver) {
		return Null, false, nil
	}
	switch apexMemberKey(method) {
	case "issuccess":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.isSuccess expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "success", Bool(false)), true, nil
	case "getid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getId expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "id", Null), true, nil
	case "geterrors":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getErrors expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "errors", List()), true, nil
	case "getrelationshipsaveresults":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getRelationshipSaveResults expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "relationshipSaveResults", List()), true, nil
	case "getaccountid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getAccountId expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "accountId", Null), true, nil
	case "getcontactid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getContactId expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "contactId", Null), true, nil
	case "getleadid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getLeadId expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "leadId", Null), true, nil
	case "getopportunityid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getOpportunityId expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "opportunityId", Null), true, nil
	case "getrelatedpersonaccountid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getRelatedPersonAccountId expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "relatedPersonAccountId", Null), true, nil
	case "getrelationshipname":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getRelationshipName expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "relationshipName", String("")), true, nil
	case "getsaveresults":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getSaveResults expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "saveResults", List()), true, nil
	case "iscreated":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.isCreated expects 0 arguments", receiver.Type)
		}
		return databaseResultObjectField(receiver, "created", Bool(false)), true, nil
	}
	return Null, false, nil
}
func databaseResultObjectLike(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	switch {
	case strings.EqualFold(value.Type, "Database.SaveResult"),
		strings.EqualFold(value.Type, "Database.DeleteResult"),
		strings.EqualFold(value.Type, "Database.UndeleteResult"),
		strings.EqualFold(value.Type, "Database.EmptyRecycleBinResult"),
		strings.EqualFold(value.Type, "Database.LockResult"),
		strings.EqualFold(value.Type, "Database.LeadConvertResult"),
		strings.EqualFold(value.Type, "Database.NestedSaveResult"),
		strings.EqualFold(value.Type, "Database.RelationshipSaveResult"),
		strings.EqualFold(value.Type, "Database.UnlockResult"),
		strings.EqualFold(value.Type, "Database.UpsertResult"),
		strings.EqualFold(value.Type, "Approval.LockResult"),
		strings.EqualFold(value.Type, "Approval.UnlockResult"):
		return true
	}
	return false
}
func databaseResultObjectField(value Value, field string, fallback Value) Value {
	if _, found, ok := objectFieldValue(value, field); ok {
		return found
	}
	return fallback
}
func newDatabaseUnitOfWork() Value {
	uow := Object("Database.UnitOfWork")
	uow.Fields["__ops"] = List()
	return uow
}
func newPendingDatabaseResult(resultType string) Value {
	row := Object(resultType)
	row.Fields["success"] = Bool(false)
	row.Fields["id"] = Null
	row.Fields["error"] = String("")
	row.Fields["errors"] = List()
	if resultType == "Database.UpsertResult" {
		row.Fields["created"] = Bool(false)
	}
	return row
}
func databaseUnitOfWorkResultType(op string) string {
	switch op {
	case "delete":
		return "Database.DeleteResult"
	case "upsert":
		return "Database.UpsertResult"
	default:
		return "Database.SaveResult"
	}
}
func databaseUnitOfWorkQueuedOps(receiver Value) Value {
	if ops, ok := receiver.Fields["__ops"]; ok && ops.Kind == ValueList {
		return ops
	}
	return List()
}
func (vm *VM) callDatabaseUnitOfWorkMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	if receiver.Type != "Database.UnitOfWork" {
		return Null, receiver, false, false, nil
	}
	switch apexMemberKey(method) {
	case "insertrecord", "updaterecord", "upsertrecord", "deleterecord":
		if len(args) != 1 || args[0].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Database.UnitOfWork.%s expects SObject", method)
		}
		op := strings.TrimSuffix(apexMemberKey(method), "record")
		placeholder := newPendingDatabaseResult(databaseUnitOfWorkResultType(op))
		opValue := Object("Database.UnitOfWorkOperation")
		opValue.Fields["op"] = String(op)
		opValue.Fields["records"] = args[0]
		opValue.Fields["result"] = placeholder
		ops := databaseUnitOfWorkQueuedOps(receiver)
		ops.List = append(ops.List, opValue)
		receiver.Fields["__ops"] = ops
		return placeholder, receiver, true, true, nil
	case "insertrecords", "updaterecords", "upsertrecords", "deleterecords":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("Database.UnitOfWork.%s expects List<SObject>", method)
		}
		op := strings.TrimSuffix(apexMemberKey(method), "records")
		resultType := databaseUnitOfWorkResultType(op)
		placeholders := make([]Value, 0, len(args[0].List))
		for range args[0].List {
			placeholders = append(placeholders, newPendingDatabaseResult(resultType))
		}
		resultList := List(placeholders...)
		opValue := Object("Database.UnitOfWorkOperation")
		opValue.Fields["op"] = String(op)
		opValue.Fields["records"] = args[0]
		opValue.Fields["result"] = resultList
		ops := databaseUnitOfWorkQueuedOps(receiver)
		ops.List = append(ops.List, opValue)
		receiver.Fields["__ops"] = ops
		return resultList, receiver, true, true, nil
	case "discardwork":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Database.UnitOfWork.discardWork expects 0 arguments")
		}
		receiver.Fields["__ops"] = List()
		return Null, receiver, true, true, nil
	case "commitwork":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Database.UnitOfWork.commitWork expects 0 arguments")
		}
		ops := databaseUnitOfWorkQueuedOps(receiver)
		for _, queued := range ops.List {
			if queued.Kind != ValueObject {
				continue
			}
			opValue, ok := queued.Fields["op"]
			if !ok || opValue.Kind != ValueString {
				continue
			}
			records, ok := queued.Fields["records"]
			if !ok {
				continue
			}
			applied, err := vm.executeDatabaseDML(opValue.Text, []Value{records, Bool(false)}, result)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if placeholder, ok := queued.Fields["result"]; ok {
				copyDatabaseUnitOfWorkResult(placeholder, applied)
			}
		}
		receiver.Fields["__ops"] = List()
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}
func copyDatabaseUnitOfWorkResult(target Value, source Value) {
	if target.Kind == ValueObject && source.Kind == ValueObject {
		for field, value := range source.Fields {
			target.Fields[field] = value
		}
		return
	}
	if target.Kind != ValueList || source.Kind != ValueList {
		return
	}
	for i := range target.List {
		if i >= len(source.List) {
			return
		}
		copyDatabaseUnitOfWorkResult(target.List[i], source.List[i])
	}
}
func databaseResultIDValue(id storage.ID) Value {
	if id == "" {
		return Null
	}
	value := String(string(id))
	value.Type = "Id"
	return value
}
func (vm *VM) needsEarlyDMLRollbackSnapshot(op string, records []storage.Record, allOrNone bool) bool {
	if !allOrNone || len(records) == 0 {
		return false
	}
	if len(records) > 1 {
		return true
	}
	if !strings.EqualFold(op, "insert") && !strings.EqualFold(op, "update") {
		return true
	}
	if vm.hasTriggerForDML(triggerTimingBefore, op, records) || vm.hasTriggerForDML(triggerTimingAfter, op, records) {
		return true
	}
	if vm.hasAutomationForDML(records) || vm.hasSummarySideEffectsForDML(records) {
		return true
	}
	return false
}
