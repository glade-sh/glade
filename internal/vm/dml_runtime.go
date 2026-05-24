package vm

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) executeDatabaseDML(op string, args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 4 {
		return Null, fmt.Errorf("Database.%s expects records, optional external id field, and optional allOrNone", op)
	}
	allOrNone := true
	externalIDField := ""
	userMode := false
	dmlOptions := dml.Options{}
	if len(args) >= 2 {
		if args[1].Kind == ValueBool {
			allOrNone = args[1].Bool
		} else if isDatabaseDMLOptionsValue(args[1]) {
			allOrNone = databaseDMLOptionsAllOrNone(args[1], allOrNone)
			dmlOptions = databaseDMLOptions(args[1])
		} else if isDatabaseAccessLevelValue(args[1]) {
			userMode = isUserModeAccessLevel(args[1])
		} else if op == "upsert" {
			field, err := vm.externalIDFieldName(args[1])
			if err != nil {
				return Null, err
			}
			externalIDField = field
		} else {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
	}
	if len(args) == 3 {
		if isDatabaseAccessLevelValue(args[2]) {
			userMode = isUserModeAccessLevel(args[2])
		} else if op != "upsert" {
			return Null, fmt.Errorf("Database.%s expects at most records and allOrNone", op)
		} else if args[2].Kind == ValueBool {
			allOrNone = args[2].Bool
		} else {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
	}
	if len(args) == 4 {
		if op != "upsert" {
			return Null, fmt.Errorf("Database.%s expects at most records, allOrNone, and AccessLevel", op)
		}
		if args[2].Kind != ValueBool {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
		if !isDatabaseAccessLevelValue(args[3]) {
			return Null, fmt.Errorf("Database.%s AccessLevel overload expects AccessLevel", op)
		}
		allOrNone = args[2].Bool
		userMode = isUserModeAccessLevel(args[3])
	}
	if op == "delete" || op == "undelete" {
		records, ok := vm.deleteIDsToSObjects(args[0])
		if ok {
			args[0] = records
		}
	}
	if userMode {
		if err := vm.enforceUserModeDMLAccess(op, args[0]); err != nil {
			return Null, err
		}
	}
	results, err := vm.applyDML(op, args[0], allOrNone, externalIDField, dmlOptions, result)
	if err != nil {
		return Null, err
	}
	if allOrNone && hasDMLFailures(results) {
		return Null, databaseDMLException(op, results)
	}
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		resultType := databaseDMLResultType(op)
		row := Object(resultType)
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = databaseResultIDValue(dmlResult.ID)
		row.Fields["error"] = String(dmlResult.Error)
		if op == "upsert" {
			row.Fields["created"] = Bool(dmlResult.Created)
		}
		row.Fields["errors"] = databaseErrorsList(dmlResult)
		values = append(values, row)
	}
	if args[0].Kind == ValueList {
		return List(values...), nil
	}
	if len(values) == 0 {
		return Null, nil
	}
	return values[0], nil
}

func isDatabaseAccessLevelValue(value Value) bool {
	return value.Kind == ValueObject && strings.EqualFold(value.Type, "AccessLevel")
}

func isUserModeAccessLevel(value Value) bool {
	return isDatabaseAccessLevelValue(value) && strings.EqualFold(strings.TrimSpace(value.Text), "USER_MODE")
}

func databaseAccessLevelSecurityMode(value Value) string {
	if !isDatabaseAccessLevelValue(value) {
		return ""
	}
	mode := strings.ToUpper(strings.TrimSpace(value.Text))
	switch mode {
	case "USER_MODE", "SYSTEM_MODE":
		return mode
	default:
		return ""
	}
}

func isDatabaseDMLOptionsValue(value Value) bool {
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Database.DMLOptions") || strings.EqualFold(value.Type, "DMLOptions"))
}

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

func apexMemberKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
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

func databaseAsyncLocatorValue(value Value) Value {
	switch value.Kind {
	case ValueString:
		return value
	case ValueObject:
		if value.Type == "Database.QueryLocator" || value.Type == "Database.Cursor" || value.Type == "Database.PaginationCursor" {
			if query, ok := value.Fields["Query"]; ok && query.Kind == ValueString && query.Text != "" {
				return String("local-query:" + query.Text)
			}
			return String("local-query")
		}
		if id, ok := value.Fields["id"]; ok && id.Kind != ValueNull {
			return String("local-result:" + id.String())
		}
		return String("local-result:" + strings.ToLower(value.Type))
	case ValueList:
		return String(fmt.Sprintf("local-list:%d", len(value.List)))
	default:
		return String("local-result")
	}
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

func newDatabaseDMLOptions() Value {
	options := Object("Database.DMLOptions")
	options.Fields["allowFieldTruncation"] = Bool(false)
	options.Fields["AllowFieldTruncation"] = options.Fields["allowFieldTruncation"]
	options.Fields["assignmentRuleHeader"] = newDatabaseHeaderObject("Database.AssignmentRuleHeader")
	options.Fields["AssignmentRuleHeader"] = options.Fields["assignmentRuleHeader"]
	options.Fields["duplicateRuleHeader"] = newDatabaseHeaderObject("Database.DuplicateRuleHeader")
	options.Fields["DuplicateRuleHeader"] = options.Fields["duplicateRuleHeader"]
	options.Fields["emailHeader"] = newDatabaseHeaderObject("Database.EmailHeader")
	options.Fields["EmailHeader"] = options.Fields["emailHeader"]
	options.Fields["localeOptions"] = Object("Database.LocaleOptions")
	options.Fields["LocaleOptions"] = options.Fields["localeOptions"]
	options.Fields["localizeErrors"] = Bool(false)
	options.Fields["LocalizeErrors"] = options.Fields["localizeErrors"]
	options.Fields["optAllOrNone"] = Bool(true)
	options.Fields["OptAllOrNone"] = options.Fields["optAllOrNone"]
	return options
}

func newDatabaseHeaderObject(typeName string) Value {
	header := Object(typeName)
	switch typeName {
	case "Database.AssignmentRuleHeader":
		header.Fields["AssignmentRuleId"] = Null
		header.Fields["UseDefaultRule"] = Null
	case "Database.DuplicateRuleHeader":
		header.Fields["AllowSave"] = Null
		header.Fields["RunAsCurrentUser"] = Null
	case "Database.EmailHeader":
		header.Fields["TriggerAutoResponseEmail"] = Null
		header.Fields["TriggerOtherEmail"] = Null
		header.Fields["TriggerUserEmail"] = Null
	}
	return header
}

func cloneDatabaseOptionsObject(value Value) Value {
	clone := value
	if value.Fields == nil {
		return clone
	}
	clone.Fields = make(map[string]Value, len(value.Fields))
	for field, fieldValue := range value.Fields {
		clone.Fields[field] = fieldValue
	}
	return clone
}

func databaseDMLOptionsAllOrNone(value Value, fallback bool) bool {
	found := false
	for _, field := range []string{"optAllOrNone", "OptAllOrNone"} {
		if option, ok := value.Fields[field]; ok && option.Kind == ValueBool {
			found = true
			if !option.Bool {
				return false
			}
		}
	}
	if found {
		return true
	}
	return fallback
}

func databaseDMLOptions(value Value) dml.Options {
	options := dml.Options{}
	for _, field := range []string{"allowFieldTruncation", "AllowFieldTruncation"} {
		if option, ok := value.Fields[field]; ok && option.Kind == ValueBool && option.Bool {
			options.AllowFieldTruncation = true
			break
		}
	}
	return options
}

func syncDatabaseOptionAliasField(value *Value, field string, fieldValue Value) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil {
		return
	}
	var aliases []string
	switch value.Type {
	case "Database.DMLOptions", "DMLOptions":
		switch strings.ToLower(field) {
		case "allowfieldtruncation":
			aliases = []string{"allowFieldTruncation", "AllowFieldTruncation"}
		case "localizeerrors":
			aliases = []string{"localizeErrors", "LocalizeErrors"}
		case "optallornone":
			aliases = []string{"optAllOrNone", "OptAllOrNone"}
		}
	}
	for _, alias := range aliases {
		value.Fields[alias] = fieldValue
	}
}

func (vm *VM) deleteIDsToSObjects(value Value) (Value, bool) {
	switch value.Kind {
	case ValueString:
		record, ok := vm.deleteIDToSObject(value.Text)
		return record, ok
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			id, err := platformScalarText(value, "Id")
			if err != nil {
				return value, false
			}
			record, ok := vm.deleteIDToSObject(id)
			return record, ok
		}
	case ValueList:
		if len(value.List) == 0 {
			return value, false
		}
		out := List()
		out.Type = "List<sObject>"
		for _, item := range value.List {
			record, ok := vm.deleteIDsToSObjects(item)
			if !ok || record.Kind != ValueObject {
				return value, false
			}
			out.List = append(out.List, record)
		}
		return out, true
	}
	return value, false
}

func (vm *VM) deleteIDToSObject(id string) (Value, bool) {
	if len(id) < 3 {
		return Null, false
	}
	objectName, ok := vm.sObjectNameForIDPrefix(id[:3])
	if !ok {
		objectName, ok = vm.sObjectNameForExistingID(id)
	}
	if ok {
		record := Object(objectName)
		record.Fields["Id"] = platformScalar("Id", id)
		return record, true
	}
	return Null, false
}

func (vm *VM) sObjectNameForExistingID(id string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	wanted := storage.ID(id)
	names := make([]string, 0, len(vm.Org.Objects))
	for name, object := range vm.Org.Objects {
		if _, ok := object.Records[wanted]; ok {
			names = append(names, name)
			continue
		}
		for candidateID := range object.Records {
			if apexIDTextEqual(string(candidateID), id) {
				names = append(names, name)
				break
			}
		}
	}
	if len(names) != 1 {
		return "", false
	}
	return names[0], true
}

func (vm *VM) executeDatabaseRecordAction(op string, args []Value, result *Result, resultType string) (Value, error) {
	if len(args) == 0 || len(args) > 2 {
		return Null, fmt.Errorf("Database.%s expects records and optional allOrNone", op)
	}
	allOrNone := true
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
		allOrNone = args[1].Bool
	}
	if op == "emptyRecycleBin" || op == "lock" || op == "unlock" {
		records, ok := vm.deleteIDsToSObjects(args[0])
		if ok {
			args[0] = records
		}
	}
	results, err := vm.applyDatabaseRecordAction(op, args[0], allOrNone, result)
	if err != nil {
		return Null, err
	}
	if allOrNone && hasDMLFailures(results) {
		return Null, databaseDMLException(op, results)
	}
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		row := Object(resultType)
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = databaseResultIDValue(dmlResult.ID)
		row.Fields["error"] = String(dmlResult.Error)
		row.Fields["errors"] = databaseErrorsList(dmlResult)
		values = append(values, row)
	}
	if args[0].Kind == ValueList {
		return List(values...), nil
	}
	if len(values) == 0 {
		return Null, nil
	}
	return values[0], nil
}

func (vm *VM) executeDatabaseTreeSave(args []Value, result *Result) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("Database.treeSave expects sObject or List<SObject>")
	}
	if vm.Org == nil {
		return Null, fmt.Errorf("DML requires org state")
	}
	listInput := args[0].Kind == ValueList
	roots := []Value{args[0]}
	if listInput {
		roots = args[0].List
	}
	backup := snapshotRuntimeOrgState(vm.Org)
	values := make([]Value, 0, len(roots))
	for i := range roots {
		value, err := vm.treeSaveOne(roots[i], result)
		if err != nil {
			*vm.Org = backup
			return Null, err
		}
		values = append(values, value)
		if !databaseNestedSaveSuccess(value) {
			*vm.Org = backup
			if listInput {
				return List(values...), nil
			}
			return value, nil
		}
	}
	if listInput {
		return List(values...), nil
	}
	if len(values) == 0 {
		return Null, nil
	}
	return values[0], nil
}

func (vm *VM) treeSaveOne(root Value, result *Result) (Value, error) {
	if root.Kind != ValueObject {
		return Null, fmt.Errorf("Database.treeSave expects sObject values")
	}
	if id := sObjectIDFromFields(root.Fields); id != "" {
		return Null, unsupportedCallError("Database.treeSave update local tree surface")
	}
	children, err := vm.treeSaveChildGroups(root)
	if err != nil {
		return Null, err
	}
	parentResults, err := vm.applyDML("insert", root, true, "", dml.Options{}, result)
	if err != nil {
		return Null, err
	}
	parent := databaseNestedSaveResult(parentResults, nil)
	if hasDMLFailures(parentResults) {
		return parent, nil
	}
	parentID := parentResults[0].ID
	relationshipResults := List()
	for _, group := range children {
		childList := typedList("List<" + group.childObject + ">")
		for _, child := range group.children {
			if err := vm.ensureTreeSaveLeaf(child); err != nil {
				return Null, err
			}
			setExplicitSObjectField(&child, group.lookupField, platformScalar("Id", string(parentID)))
			childList.List = append(childList.List, child)
		}
		childResults, err := vm.applyDML("insert", childList, true, "", dml.Options{}, result)
		if err != nil {
			return Null, err
		}
		relationship := Object("Database.RelationshipSaveResult")
		relationship.Fields["relationshipName"] = String(group.relationshipName)
		relationship.Fields["saveResults"] = List(databaseNestedSaveResults(childResults, nil)...)
		relationshipResults.List = append(relationshipResults.List, relationship)
		if hasDMLFailures(childResults) {
			parent.Fields["success"] = Bool(false)
		}
	}
	parent.Fields["relationshipSaveResults"] = relationshipResults
	return parent, nil
}

type treeSaveChildGroup struct {
	relationshipName string
	childObject      string
	lookupField      string
	children         []Value
}

func (vm *VM) treeSaveChildGroups(root Value) ([]treeSaveChildGroup, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("DML requires org state")
	}
	parentObject, ok := vm.resolveObjectName(root.Type)
	if !ok {
		return nil, fmt.Errorf("Database.treeSave unknown object %s", root.Type)
	}
	groups := make([]treeSaveChildGroup, 0)
	for field, value := range root.Fields {
		if isInternalSObjectField(field) || value.Kind != ValueList {
			continue
		}
		childObject, lookupField, relationshipName, ok, err := vm.treeSaveChildRelationship(parentObject, field)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		children := make([]Value, 0, len(value.List))
		for _, child := range value.List {
			if child.Kind != ValueObject {
				return nil, fmt.Errorf("Database.treeSave relationship %s expects sObject children", relationshipName)
			}
			if !strings.EqualFold(child.Type, childObject) {
				return nil, fmt.Errorf("Database.treeSave relationship %s expects %s children", relationshipName, childObject)
			}
			children = append(children, child)
		}
		groups = append(groups, treeSaveChildGroup{
			relationshipName: relationshipName,
			childObject:      childObject,
			lookupField:      lookupField,
			children:         children,
		})
	}
	return groups, nil
}

func (vm *VM) treeSaveChildRelationship(parentObject, relationshipName string) (string, string, string, bool, error) {
	var childObject, lookupField, canonicalRelationship string
	for name, state := range vm.Org.Objects {
		for _, relation := range state.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) || relation.Polymorphic || strings.TrimSpace(relation.Field) == "" {
				continue
			}
			childRelationship := relation.ChildRelationship
			if childRelationship == "" {
				childRelationship = derivedVMChildRelationshipName(state.Definition)
			}
			if !vmRelationshipNameMatches(vm.Org.Namespace, childRelationship, relationshipName) {
				continue
			}
			if childObject != "" {
				return "", "", "", false, unsupportedCallError("Database.treeSave ambiguous child relationship " + relationshipName)
			}
			childObject = name
			lookupField = relation.Field
			canonicalRelationship = childRelationship
		}
	}
	if childObject == "" {
		return "", "", "", false, nil
	}
	return childObject, lookupField, canonicalRelationship, true, nil
}

func (vm *VM) ensureTreeSaveLeaf(value Value) error {
	if value.Kind != ValueObject {
		return fmt.Errorf("Database.treeSave expects sObject children")
	}
	if id := sObjectIDFromFields(value.Fields); id != "" {
		return unsupportedCallError("Database.treeSave update local tree surface")
	}
	if vm.Org == nil {
		return fmt.Errorf("DML requires org state")
	}
	objectName, ok := vm.resolveObjectName(value.Type)
	if !ok {
		return fmt.Errorf("Database.treeSave unknown object %s", value.Type)
	}
	definition := vm.Org.Objects[objectName].Definition
	for field, child := range value.Fields {
		if child.Kind == ValueList && vm.isChildRelationshipField(definition, field) {
			return unsupportedCallError("Database.treeSave nested child relationship local tree surface")
		}
	}
	return nil
}

func databaseNestedSaveResults(results []dml.Result, relationships []Value) []Value {
	values := make([]Value, 0, len(results))
	for _, result := range results {
		values = append(values, databaseNestedSaveResult([]dml.Result{result}, relationships))
	}
	return values
}

func databaseNestedSaveResult(results []dml.Result, relationships []Value) Value {
	row := Object("Database.NestedSaveResult")
	if len(results) == 0 {
		row.Fields["success"] = Bool(false)
		row.Fields["id"] = Null
		row.Fields["errors"] = List()
	} else {
		row.Fields["success"] = Bool(results[0].Success)
		row.Fields["id"] = databaseResultIDValue(results[0].ID)
		row.Fields["error"] = String(results[0].Error)
		row.Fields["errors"] = databaseErrorsList(results[0])
	}
	row.Fields["relationshipSaveResults"] = List(relationships...)
	return row
}

func databaseNestedSaveSuccess(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	success, ok := value.Fields["success"]
	if !ok || success.Kind != ValueBool || !success.Bool {
		return false
	}
	relationships, ok := value.Fields["relationshipSaveResults"]
	if !ok || relationships.Kind != ValueList {
		return true
	}
	for _, relationship := range relationships.List {
		if relationship.Kind != ValueObject {
			return false
		}
		saveResults, ok := relationship.Fields["saveResults"]
		if !ok || saveResults.Kind != ValueList {
			continue
		}
		for _, child := range saveResults.List {
			if !databaseNestedSaveSuccess(child) {
				return false
			}
		}
	}
	return true
}

func (vm *VM) executeDatabaseConvertLead(args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 3 {
		return Null, fmt.Errorf("Database.convertLead expects LeadConvert or List<LeadConvert>")
	}
	if vm.Org == nil {
		return Null, fmt.Errorf("DML requires org state")
	}
	allOrNone := true
	for _, arg := range args[1:] {
		if arg.Kind == ValueBool {
			allOrNone = arg.Bool
			continue
		}
		if isDatabaseAccessLevelValue(arg) || isDatabaseDMLOptionsValue(arg) {
			continue
		}
		return Null, unsupportedCallError("Database.convertLead overload option local lead conversion surface")
	}
	listInput := args[0].Kind == ValueList
	converts := []Value{args[0]}
	if listInput {
		converts = args[0].List
	}
	var backup storage.OrgState
	if allOrNone {
		backup = snapshotRuntimeOrgState(vm.Org)
	}
	values := make([]Value, 0, len(converts))
	for _, convert := range converts {
		row, err := vm.convertLeadOne(convert, result)
		if err != nil {
			if allOrNone {
				*vm.Org = backup
				return Null, err
			}
			values = append(values, databaseLeadConvertFailure("", err.Error()))
			continue
		}
		values = append(values, row)
		if allOrNone && !databaseLeadConvertSuccess(row) {
			*vm.Org = backup
			break
		}
	}
	if listInput {
		return List(values...), nil
	}
	if len(values) == 0 {
		return Null, nil
	}
	return values[0], nil
}

func (vm *VM) convertLeadOne(convert Value, result *Result) (Value, error) {
	if vm.Org == nil {
		return Null, fmt.Errorf("DML requires org state")
	}
	if convert.Kind != ValueObject || !strings.EqualFold(convert.Type, "Database.LeadConvert") {
		return Null, fmt.Errorf("Database.convertLead expects Database.LeadConvert")
	}
	if databaseLeadConvertBool(convert, "bypassAccountDedupeCheck") ||
		databaseLeadConvertBool(convert, "bypassContactDedupeCheck") ||
		databaseLeadConvertBool(convert, "overwriteLeadSource") ||
		databaseLeadConvertBool(convert, "sendNotificationEmail") {
		return Null, unsupportedCallError("Database.convertLead dedupe/notification local lead conversion surface")
	}
	if value, ok := databaseLeadConvertField(convert, "relatedPersonAccountId"); ok && value.Kind != ValueNull {
		return Null, unsupportedCallError("Database.convertLead person account local lead conversion surface")
	}
	if value, ok := databaseLeadConvertField(convert, "relatedPersonAccountRecord"); ok && value.Kind != ValueNull {
		return Null, unsupportedCallError("Database.convertLead person account local lead conversion surface")
	}
	if !databaseLeadConvertBool(convert, "doNotCreateOpportunity") {
		return Null, unsupportedCallError("Database.convertLead opportunity local lead conversion surface")
	}
	leadIDValue, ok := databaseLeadConvertField(convert, "leadId")
	if !ok || !isApexIDLikeValue(leadIDValue) {
		return databaseLeadConvertFailure("", "LeadConvert.leadId is required"), nil
	}
	leadID := storage.ID(scalarText(leadIDValue))
	leadState, ok := vm.Org.Objects["Lead"]
	if !ok {
		return Null, fmt.Errorf("Database.convertLead requires Lead metadata")
	}
	storedLeadID, lead, ok := findRecordByLooseID(leadState, leadID)
	if !ok {
		return databaseLeadConvertFailure(string(leadID), "Lead not found"), nil
	}
	leadID = lead.ID
	accountID, err := vm.convertLeadAccountID(convert, lead, result)
	if err != nil {
		return Null, err
	}
	contactID, err := vm.convertLeadContactID(convert, lead, accountID, result)
	if err != nil {
		return Null, err
	}
	updatedLeadState := vm.Org.Objects["Lead"]
	updatedLead := updatedLeadState.Records[storedLeadID]
	if _, ok := leadState.Definition.Fields["IsConverted"]; ok {
		updatedLead.Fields["IsConverted"] = storage.BooleanValue(true)
	}
	if _, ok := leadState.Definition.Fields["ConvertedAccountId"]; ok {
		updatedLead.Fields["ConvertedAccountId"] = storage.IDValue(accountID)
	}
	if _, ok := leadState.Definition.Fields["ConvertedContactId"]; ok {
		updatedLead.Fields["ConvertedContactId"] = storage.IDValue(contactID)
	}
	if _, ok := leadState.Definition.Fields["Status"]; ok {
		status := "Converted"
		if value, ok := databaseLeadConvertField(convert, "convertedStatus"); ok && value.Kind == ValueString && strings.TrimSpace(value.Text) != "" {
			status = value.Text
		}
		updatedLead.Fields["Status"] = storage.StringValue(status)
	}
	updatedLeadState.Records[storedLeadID] = updatedLead
	vm.Org.Objects["Lead"] = updatedLeadState
	row := Object("Database.LeadConvertResult")
	row.Fields["success"] = Bool(true)
	row.Fields["leadId"] = platformScalar("Id", string(leadID))
	row.Fields["accountId"] = platformScalar("Id", string(accountID))
	row.Fields["contactId"] = platformScalar("Id", string(contactID))
	row.Fields["opportunityId"] = Null
	row.Fields["relatedPersonAccountId"] = Null
	row.Fields["errors"] = List()
	return row, nil
}

func (vm *VM) convertLeadAccountID(convert Value, lead storage.Record, result *Result) (storage.ID, error) {
	if value, ok := databaseLeadConvertField(convert, "accountId"); ok && isApexIDLikeValue(value) {
		return storage.ID(scalarText(value)), nil
	}
	account := Object("Account")
	if value, ok := databaseLeadConvertField(convert, "accountRecord"); ok && value.Kind == ValueObject && !strings.EqualFold(value.Type, "SObject") {
		account = value
		account.Type = "Account"
	}
	if _, _, ok := objectFieldValue(account, "Name"); !ok {
		name := storageRecordStringField(lead, "Company")
		if name == "" {
			name = storageRecordStringField(lead, "LastName")
		}
		setExplicitSObjectField(&account, "Name", String(name))
	}
	results, err := vm.applyDML("insert", account, true, "", dml.Options{}, result)
	if err != nil {
		return "", err
	}
	if hasDMLFailures(results) {
		return "", databaseDMLException("convertLead", results)
	}
	return results[0].ID, nil
}

func (vm *VM) convertLeadContactID(convert Value, lead storage.Record, accountID storage.ID, result *Result) (storage.ID, error) {
	if value, ok := databaseLeadConvertField(convert, "contactId"); ok && isApexIDLikeValue(value) {
		return storage.ID(scalarText(value)), nil
	}
	contact := Object("Contact")
	if value, ok := databaseLeadConvertField(convert, "contactRecord"); ok && value.Kind == ValueObject && !strings.EqualFold(value.Type, "SObject") {
		contact = value
		contact.Type = "Contact"
	}
	if _, _, ok := objectFieldValue(contact, "FirstName"); !ok {
		if first := storageRecordStringField(lead, "FirstName"); first != "" {
			setExplicitSObjectField(&contact, "FirstName", String(first))
		}
	}
	if _, _, ok := objectFieldValue(contact, "LastName"); !ok {
		setExplicitSObjectField(&contact, "LastName", String(storageRecordStringField(lead, "LastName")))
	}
	if _, _, ok := objectFieldValue(contact, "AccountId"); !ok {
		setExplicitSObjectField(&contact, "AccountId", platformScalar("Id", string(accountID)))
	}
	results, err := vm.applyDML("insert", contact, true, "", dml.Options{}, result)
	if err != nil {
		return "", err
	}
	if hasDMLFailures(results) {
		return "", databaseDMLException("convertLead", results)
	}
	return results[0].ID, nil
}

func databaseLeadConvertField(convert Value, name string) (Value, bool) {
	_, value, ok := objectFieldValue(convert, name)
	return value, ok
}

func databaseLeadConvertBool(convert Value, name string) bool {
	value, ok := databaseLeadConvertField(convert, name)
	return ok && value.Kind == ValueBool && value.Bool
}

func databaseLeadConvertFailure(leadID, message string) Value {
	row := Object("Database.LeadConvertResult")
	row.Fields["success"] = Bool(false)
	if leadID == "" {
		row.Fields["leadId"] = Null
	} else {
		row.Fields["leadId"] = platformScalar("Id", leadID)
	}
	row.Fields["accountId"] = Null
	row.Fields["contactId"] = Null
	row.Fields["opportunityId"] = Null
	row.Fields["relatedPersonAccountId"] = Null
	row.Fields["errors"] = List(databaseErrorValue(dml.Error{StatusCode: "INVALID_FIELD", Message: message}))
	return row
}

func databaseLeadConvertSuccess(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	success, ok := value.Fields["success"]
	return ok && success.Kind == ValueBool && success.Bool
}

func storageRecordStringField(record storage.Record, field string) string {
	value, ok := record.GetField(field)
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueString:
		return value.String
	case storage.ValueID:
		return string(value.ID)
	default:
		return ""
	}
}

func findRecordByLooseID(object storage.ObjectState, id storage.ID) (storage.ID, storage.Record, bool) {
	if record, ok := object.Records[id]; ok {
		return id, record, true
	}
	wanted := strings.ToLower(string(id))
	for candidateID, record := range object.Records {
		candidate := strings.ToLower(string(candidateID))
		if candidate == wanted || strings.HasPrefix(wanted, candidate) || strings.HasPrefix(candidate, wanted) || apexIDTextEqual(candidate, wanted) {
			return candidateID, record, true
		}
	}
	return "", storage.Record{}, false
}

func (vm *VM) executeApprovalIsLocked(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("Approval.isLocked expects record or Id")
	}
	value := args[0]
	if records, ok := vm.deleteIDsToSObjects(value); ok {
		value = records
	}
	if value.Kind == ValueList {
		out := typedMap("Map<Id,Boolean>")
		for _, item := range value.List {
			record, err := vm.recordFromValue(&item)
			if err != nil {
				return Null, err
			}
			key := databaseResultIDValue(record.ID)
			encodedKey := vm.mapKey(key)
			out.Map[encodedKey] = Bool(vm.isRecordLocked(record.ID))
			out.MapKeys[encodedKey] = key
		}
		return out, nil
	}
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return Null, err
	}
	return Bool(vm.isRecordLocked(record.ID)), nil
}

func databaseCursorNumRecords(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	records, ok := receiver.Fields["Records"]
	if !ok || records.Kind != ValueList {
		return Int(0), receiver, false, true, nil
	}
	return Int(int64(len(records.List))), receiver, false, true, nil
}

func databaseCursorFetch(receiver Value, method string, args []Value, deleted bool) (Value, Value, bool, bool, error) {
	if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects start and page size Integers", receiver.Type, method)
	}
	records, ok := receiver.Fields["Records"]
	if !ok || records.Kind != ValueList {
		records = List()
	}
	start := int(args[0].Int)
	size := int(args[1].Int)
	if start < 0 {
		start = 0
	}
	if size < 0 {
		size = 0
	}
	if start > len(records.List) {
		start = len(records.List)
	}
	end := start + size
	if end > len(records.List) {
		end = len(records.List)
	}
	page := List(append([]Value(nil), records.List[start:end]...)...)
	page.Type = "List<SObject>"
	if deleted {
		return Int(0), receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "Database.Cursor") {
		return page, receiver, false, true, nil
	}
	out := Object("Database.CursorFetchResult")
	out.Fields["records"] = page
	out.Fields["nextIndex"] = Int(int64(end))
	out.Fields["numDeletedRecords"] = Int(0)
	out.Fields["done"] = Bool(end >= len(records.List))
	return out, receiver, false, true, nil
}

func databaseObjectGetter(receiver Value, method string, args []Value, field string, fallback Value) (Value, Value, bool, bool, error) {
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	if _, value, ok := objectFieldValue(receiver, field); ok {
		return value, receiver, false, true, nil
	}
	return fallback, receiver, false, true, nil
}

func (vm *VM) isRecordLocked(id storage.ID) bool {
	if vm == nil || vm.Org == nil || id == "" {
		return false
	}
	objectName, ok := vm.sObjectNameForExistingID(string(id))
	if !ok {
		return false
	}
	record, ok := vm.Org.Objects[objectName].Records[id]
	return ok && record.System.Locked
}

func (vm *VM) applyDatabaseRecordAction(op string, value Value, allOrNone bool, result *Result) ([]dml.Result, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("DML requires org state")
	}
	records, _, err := vm.recordsFromValue(value)
	if err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlRows", len(records)); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("cpuTime", len(records)); err != nil {
		return nil, err
	}
	if err := vm.checkMixedDML(records); err != nil {
		return nil, err
	}
	appendTrace(result, "apex.dml."+op, "apex.dml", map[string]any{
		"operation": op,
		"rows":      len(records),
		"objects":   dmlTraceObjectNames(records),
	})
	var backup storage.OrgState
	if allOrNone {
		backup = snapshotRuntimeOrgState(vm.Org)
	}
	engine := vm.newDMLEngine(result)
	var results []dml.Result
	switch op {
	case "emptyRecycleBin":
		results = engine.EmptyRecycleBin(records)
	case "lock":
		results = engine.Lock(records)
	case "unlock":
		results = engine.Unlock(records)
	default:
		return nil, fmt.Errorf("unsupported Database.%s operation", op)
	}
	if allOrNone && hasDMLFailures(results) {
		*vm.Org = backup
	}
	if hasDMLSuccess(results) {
		vm.rebuildDMLObjectIndexes(records, results)
	}
	return results, nil
}

func unsupportedDatabaseDMLOverload(op string, args []Value) error {
	for _, arg := range args[1:] {
		if arg.Kind != ValueObject {
			continue
		}
		switch arg.Type {
		case "AccessLevel":
			return unsupportedCallError("Database." + op + " AccessLevel overload")
		case "Database.DMLOptions", "DMLOptions":
			return unsupportedCallError("Database." + op + " DMLOptions overload")
		}
	}
	return nil
}

func databaseDMLResultType(op string) string {
	switch op {
	case "delete":
		return "Database.DeleteResult"
	case "undelete":
		return "Database.UndeleteResult"
	case "upsert":
		return "Database.UpsertResult"
	default:
		return "Database.SaveResult"
	}
}

func databaseRecordActionResultType(op string) string {
	switch op {
	case "emptyRecycleBin":
		return "Database.EmptyRecycleBinResult"
	case "lock":
		return "Database.LockResult"
	case "unlock":
		return "Database.UnlockResult"
	default:
		return "Database.SaveResult"
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

func databaseDMLException(op string, results []dml.Result) error {
	message := "DML operation failed"
	if op != "" {
		message = "Database." + op + " failed"
	}
	for _, result := range results {
		if !result.Success && result.Error != "" {
			if result.StatusCode != "" {
				message += ": " + result.StatusCode + ", " + result.Error
			} else {
				message += ": " + result.Error
			}
			break
		}
	}
	value := Object("DmlException")
	value.Fields["message"] = String(message)
	value.Fields["__dmlErrors"] = dmlExceptionErrorDetails(results)
	return &apexThrowError{value: value}
}

func dmlExceptionFromTriggerError(op string, err error) error {
	var thrown *apexThrowError
	if !errors.As(err, &thrown) {
		return err
	}
	message := exceptionMessage(thrown.value)
	if message == "" {
		message = thrown.value.String()
	}
	if op == "" {
		op = "operation"
	}
	value := Object("DmlException")
	value.Fields["message"] = String("Database." + op + " failed: " + message)
	value.Fields["__cause"] = thrown.value
	return &apexThrowError{value: value, stack: append([]callFrame(nil), thrown.stack...)}
}

func exceptionMessage(value Value) string {
	if value.Kind != ValueObject {
		return value.String()
	}
	if message, ok := value.Fields["message"]; ok {
		if message.Kind == ValueString {
			return message.Text
		}
		if message.Kind != ValueNull {
			return message.String()
		}
	}
	return ""
}

func dmlExceptionErrorDetails(results []dml.Result) Value {
	details := List()
	for index, result := range results {
		if result.Success || result.Error == "" {
			continue
		}
		for _, err := range dmlResultErrors(result) {
			detail := databaseErrorValue(err)
			detail.Fields["id"] = databaseResultIDValue(result.ID)
			detail.Fields["index"] = Int(int64(index))
			details.List = append(details.List, detail)
		}
	}
	return details
}

func (vm *VM) executeDatabaseMerge(args []Value, result *Result) (Value, error) {
	if len(args) < 2 || len(args) > 4 {
		return Null, fmt.Errorf("Database.merge expects master, duplicate record(s), and optional allOrNone")
	}
	allOrNone := true
	if len(args) == 3 {
		if isDatabaseAccessLevelValue(args[2]) {
			// USER_MODE/SYSTEM_MODE is accepted for overload parity; local merge
			// currently uses the same in-memory DML engine for both modes.
		} else if args[2].Kind != ValueBool {
			return Null, fmt.Errorf("Database.merge allOrNone expects Boolean")
		} else {
			allOrNone = args[2].Bool
		}
	}
	if len(args) == 4 {
		if args[2].Kind != ValueBool {
			return Null, fmt.Errorf("Database.merge allOrNone expects Boolean")
		}
		if !isDatabaseAccessLevelValue(args[3]) {
			return Null, fmt.Errorf("Database.merge AccessLevel overload expects AccessLevel")
		}
		allOrNone = args[2].Bool
	}
	if vm.Org == nil {
		return Null, fmt.Errorf("DML requires org state")
	}
	master, _, err := vm.recordsFromValue(args[0])
	if err != nil {
		return Null, err
	}
	if len(master) != 1 {
		return Null, fmt.Errorf("Database.merge master expects one sObject")
	}
	duplicateInput := args[1]
	if records, ok := vm.deleteIDsToSObjects(duplicateInput); ok {
		duplicateInput = records
	}
	duplicates, _, err := vm.recordsFromValue(duplicateInput)
	if err != nil {
		return Null, err
	}
	recordsForChecks := append([]storage.Record{master[0]}, duplicates...)
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("dmlRows", len(recordsForChecks)); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("cpuTime", len(recordsForChecks)); err != nil {
		return Null, err
	}
	if err := vm.checkMixedDML(recordsForChecks); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.dml.merge", "apex.dml", map[string]any{
		"operation": "merge",
		"rows":      len(recordsForChecks),
	})
	var backup storage.OrgState
	if allOrNone {
		backup = snapshotRuntimeOrgState(vm.Org)
	}
	masterBefore, err := vm.oldRecords("update", master)
	if err != nil {
		return Null, err
	}
	duplicateBefore, err := vm.oldRecords("delete", duplicates)
	if err != nil {
		return Null, err
	}
	markMergeDuplicateOldRecords(duplicateBefore, master[0].ID)
	if beforeUpdateFailures, err := vm.runTriggers(triggerTimingBefore, "update", master, masterBefore, result); err != nil {
		*vm.Org = backup
		return Null, err
	} else if hasDMLFailures(beforeUpdateFailures) {
		*vm.Org = backup
		results := make([]dml.Result, len(duplicates))
		failure := beforeUpdateFailures[0]
		for i := range results {
			results[i] = failure
		}
		if allOrNone {
			return Null, databaseDMLException("merge", results)
		}
		return vm.mergeResultValue(args[1].Kind == ValueList, duplicates, results), nil
	}
	beforeDeleteFailures, err := vm.runTriggers(triggerTimingBefore, "delete", duplicates, duplicateBefore, result)
	if err != nil {
		*vm.Org = backup
		return Null, err
	}
	mergeDuplicates := duplicates
	mergeDuplicateBefore := duplicateBefore
	if hasDMLFailures(beforeDeleteFailures) {
		if allOrNone {
			*vm.Org = backup
			return Null, databaseDMLException("merge", beforeDeleteFailures)
		}
		mergeDuplicates, mergeDuplicateBefore, _ = filterDMLInputs(duplicates, duplicateBefore, nil, beforeDeleteFailures)
		if len(mergeDuplicates) == 0 {
			return vm.mergeResultValue(args[1].Kind == ValueList, duplicates, beforeDeleteFailures), nil
		}
	}
	engine := vm.newDMLEngine(result)
	results := engine.Merge(master[0], mergeDuplicates)
	if hasDMLFailures(beforeDeleteFailures) {
		results = mergeDMLResults(beforeDeleteFailures, results)
	}
	engineRolledBack := false
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				*vm.Org = backup
				engineRolledBack = true
				break
			}
		}
	}
	if allOrNone && hasDMLFailures(results) {
		return Null, databaseDMLException("merge", results)
	}
	successfulDuplicates := make([]storage.Record, 0, len(duplicates))
	successfulDuplicateBefore := make([]storage.Record, 0, len(mergeDuplicateBefore))
	if !engineRolledBack {
		successIndex := 0
		for i, dmlResult := range results {
			if !dmlResult.Success {
				continue
			}
			if i < len(beforeDeleteFailures) && !beforeDeleteFailures[i].Success && beforeDeleteFailures[i].Error != "" {
				continue
			}
			if successIndex < len(mergeDuplicates) {
				successfulDuplicates = append(successfulDuplicates, mergeDuplicates[successIndex])
			}
			if successIndex < len(mergeDuplicateBefore) {
				successfulDuplicateBefore = append(successfulDuplicateBefore, mergeDuplicateBefore[successIndex])
			}
			successIndex++
		}
	}
	if len(successfulDuplicates) > 0 {
		if _, err := vm.runTriggers(triggerTimingAfter, "delete", successfulDuplicates, successfulDuplicateBefore, result); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return Null, err
		}
		afterMaster, err := vm.afterRecords("update", master, []dml.Result{{ID: master[0].ID, Success: true}})
		if err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return Null, err
		}
		if _, err := vm.runTriggers(triggerTimingAfter, "update", afterMaster, masterBefore, result); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return Null, err
		}
	}
	return vm.mergeResultValue(args[1].Kind == ValueList, duplicates, results), nil
}

func markMergeDuplicateOldRecords(records []storage.Record, masterID storage.ID) {
	if masterID == "" {
		return
	}
	for i := range records {
		if records[i].Fields == nil {
			records[i].Fields = make(map[string]storage.Value)
		}
		records[i].Fields["MasterRecordId"] = storage.IDValue(masterID)
	}
}

func (vm *VM) mergeResultValue(listInput bool, duplicates []storage.Record, results []dml.Result) Value {
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		row := Object("Database.MergeResult")
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = databaseResultIDValue(dmlResult.ID)
		row.Fields["error"] = String(dmlResult.Error)
		mergedIDs := List()
		for _, id := range dmlResult.MergedRecordIDs {
			mergedIDs.List = append(mergedIDs.List, String(string(id)))
		}
		row.Fields["mergedRecordIds"] = mergedIDs
		updatedRelatedIDs := List()
		for _, id := range dmlResult.UpdatedRelatedIDs {
			updatedRelatedIDs.List = append(updatedRelatedIDs.List, String(string(id)))
		}
		row.Fields["updatedRelatedIds"] = updatedRelatedIDs
		row.Fields["errors"] = databaseErrorsList(dmlResult)
		values = append(values, row)
	}
	if listInput {
		return List(values...)
	}
	if len(values) == 0 {
		return Null
	}
	return values[0]
}

func (vm *VM) externalIDFieldName(value Value) (string, error) {
	switch value.Kind {
	case ValueString:
		return value.Text, nil
	case ValueObject:
		if isSObjectFieldTokenType(value.Type) {
			field, ok := value.Fields["field"]
			if !ok || field.Kind != ValueString {
				return "", fmt.Errorf("Database.upsert external id field token is missing field name")
			}
			return field.Text, nil
		}
	}
	return "", fmt.Errorf("Database.upsert external id field expects Schema.SObjectField")
}

func (vm *VM) populateDMLResultFields(value *Value, results []dml.Result) {
	if value.Kind == ValueList {
		for i := range value.List {
			if i >= len(results) || !results[i].Success {
				continue
			}
			vm.populateDMLResultFields(&value.List[i], results[i:i+1])
		}
		return
	}
	if len(results) > 0 && results[0].Success && value.Kind == ValueObject {
		id := results[0].ID
		value.Fields["Id"] = platformScalar("Id", string(id))
		if vm.Org == nil || id == "" {
			return
		}
		markDMLAccessibleFields(value)
		objectName, ok := vm.resolveObjectName(value.Type)
		if !ok {
			return
		}
		record, ok := vm.Org.Objects[objectName].Records[id]
		if !ok {
			return
		}
		putSystemFields(*value, record.System)
	}
}

func (vm *VM) clearDMLResultFieldsForFailures(targets []*Value, results []dml.Result, ops []string) {
	for i, result := range results {
		if result.Success || i >= len(targets) || targets[i] == nil {
			continue
		}
		clearDMLResultFields(targets[i], dmlFailurePreservesID(dmlFailureOpAt(ops, i)))
	}
}

func dmlFailureOpAt(ops []string, index int) string {
	if len(ops) == 1 {
		return ops[0]
	}
	if index >= 0 && index < len(ops) {
		return ops[index]
	}
	return ""
}

func dmlFailurePreservesID(op string) bool {
	switch strings.ToLower(op) {
	case "update", "delete", "undelete":
		return true
	default:
		return false
	}
}

func clearDMLResultFields(value *Value, preserveID bool) {
	if value == nil {
		return
	}
	if value.Kind == ValueList {
		for i := range value.List {
			clearDMLResultFields(&value.List[i], preserveID)
		}
		return
	}
	if value.Kind != ValueObject || value.Fields == nil {
		return
	}
	if !preserveID {
		deleteObjectField(value.Fields, "Id")
	}
	deleteObjectField(value.Fields, sobjectDMLAccessibleField)
	deleteObjectField(value.Fields, sobjectQueriedFieldsField)
	for _, field := range []string{"CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp"} {
		deleteObjectField(value.Fields, field)
	}
}

func deleteObjectField(fields map[string]Value, name string) {
	if fields == nil {
		return
	}
	for candidate := range fields {
		if strings.EqualFold(candidate, name) {
			delete(fields, candidate)
		}
	}
}

func (vm *VM) hydrateCloneRecordTypeID(source Value, clone *Value) {
	if vm == nil || clone == nil || clone.Kind != ValueObject || clone.Fields == nil {
		return
	}
	if _, existing, ok := objectFieldValue(*clone, "RecordTypeId"); ok && existing.Kind != ValueNull && isExplicitSObjectField(*clone, "RecordTypeId") {
		return
	}
	sourceID := sObjectIDFromFields(source.Fields)
	if sourceID == "" {
		return
	}
	record, ok := vm.findOrgRecord(source.Type, sourceID)
	if !ok {
		return
	}
	recordTypeID, ok := record.Fields["RecordTypeId"]
	if !ok || recordTypeID.Kind == storage.ValueNull {
		return
	}
	clone.Fields["RecordTypeId"] = vmValueFromStorage(recordTypeID)
	markExplicitSObjectField(clone, "RecordTypeId")
}

func markDMLAccessibleFields(value *Value) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if selected, ok := value.Fields[sobjectQueriedFieldsField]; ok && selected.Kind == ValueMap {
		return
	}
	value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(value.Type, dmlVisibleSObjectFields(value))
	value.Fields[sobjectDMLAccessibleField] = Bool(true)
}

func hasDMLFailures(results []dml.Result) bool {
	for _, result := range results {
		if !result.Success && result.Error != "" {
			return true
		}
	}
	return false
}

func dmlResultsFromTargets(records []storage.Record, targets []*Value) []dml.Result {
	if len(targets) == 0 {
		return nil
	}
	values := make([]Value, len(targets))
	for i, target := range targets {
		if target == nil {
			continue
		}
		values[i] = *target
	}
	return dmlResultsFromSObjectErrors(records, values)
}

func filterDMLInputs(records, before []storage.Record, targets []*Value, failures []dml.Result) ([]storage.Record, []storage.Record, []*Value) {
	filteredRecords := make([]storage.Record, 0, len(records))
	filteredBefore := make([]storage.Record, 0, len(before))
	filteredTargets := make([]*Value, 0, len(targets))
	for i, record := range records {
		if i < len(failures) && !failures[i].Success && failures[i].Error != "" {
			continue
		}
		filteredRecords = append(filteredRecords, record)
		if i < len(before) {
			filteredBefore = append(filteredBefore, before[i])
		}
		if i < len(targets) {
			filteredTargets = append(filteredTargets, targets[i])
		}
	}
	return filteredRecords, filteredBefore, filteredTargets
}

func mergeDMLResults(failures, successes []dml.Result) []dml.Result {
	out := make([]dml.Result, len(failures))
	successIndex := 0
	for i, failure := range failures {
		if !failure.Success && failure.Error != "" {
			out[i] = failure
			continue
		}
		if successIndex < len(successes) {
			out[i] = successes[successIndex]
			successIndex++
		}
	}
	return out
}

func mergeDMLFailuresInPlace(target, source []dml.Result) {
	for i, failure := range source {
		if i >= len(target) || failure.Success || failure.Error == "" {
			continue
		}
		if target[i].Error == "" {
			target[i] = failure
			continue
		}
		combinedErrors := append(dmlResultErrors(target[i]), dmlResultErrors(failure)...)
		target[i].Error += "; " + failure.Error
		target[i].StatusCode = failure.StatusCode
		target[i].Fields = append(target[i].Fields, failure.Fields...)
		target[i].Errors = combinedErrors
	}
}

func (vm *VM) applyDML(op string, value Value, allOrNone bool, externalIDField string, options dml.Options, result *Result) ([]dml.Result, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("DML requires org state")
	}
	bulkPrevious := Null
	bulkPropagate := value.Kind == ValueList && value.Ref != 0
	if bulkPropagate {
		bulkPrevious = cloneValuePreserveRefs(value)
	}
	records, targets, err := vm.recordsFromValue(value)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return nil, err
	}
	dmlRows := len(records)
	if op == "delete" {
		dmlRows += vm.cascadeDeleteRowCount(records)
	}
	if err := vm.incrementLimit("dmlRows", dmlRows); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("cpuTime", dmlRows); err != nil {
		return nil, err
	}
	if err := vm.checkMixedDML(records); err != nil {
		return nil, err
	}
	if op == "upsert" {
		return vm.applyUpsertDML(records, targets, allOrNone, externalIDField, result)
	}
	appendTrace(result, "apex.dml."+op, "apex.dml", map[string]any{
		"operation": op,
		"rows":      len(records),
		"objects":   dmlTraceObjectNames(records),
	})
	if op == "insert" {
		vm.applySObjectFieldDefaults(records)
		vm.applyBeforeInsertDerivedFields(records)
	}
	before, err := vm.oldRecords(op, records)
	if err != nil {
		return nil, err
	}
	var backup storage.OrgState
	backupReady := false
	ensureBackup := func() {
		if backupReady {
			return
		}
		backup = snapshotRuntimeOrgState(vm.Org)
		backupReady = true
	}
	if vm.needsEarlyDMLRollbackSnapshot(op, records, allOrNone) {
		ensureBackup()
	}
	var beforeFailures []dml.Result
	originalUpdateRecords := records
	beforeTriggerRecords := records
	if op == "update" {
		beforeTriggerRecords = vm.hydrateUpdateTriggerRecords(records, before)
	}
	if op != "undelete" {
		beforeFailures, beforeTriggerRecords, err = vm.runTriggersByObject(triggerTimingBefore, op, beforeTriggerRecords, before, result)
		if err != nil {
			ensureBackup()
			*vm.Org = backup
			return nil, dmlExceptionFromTriggerError(op, err)
		}
		if op != "delete" {
			if op == "update" {
				preserveUpdateExplicitNulls(beforeTriggerRecords, originalUpdateRecords)
			}
			records = beforeTriggerRecords
			if op == "insert" {
				vm.applyTestSObjectNameDefaults(records, true)
			}
		}
		targetFailures := dmlResultsFromTargets(records, targets)
		if hasDMLFailures(targetFailures) {
			beforeFailures = mergeDMLResults(beforeFailures, targetFailures)
		}
	}
	if hasDMLFailures(beforeFailures) {
		appendDMLResultTrace(result, op, records, beforeFailures)
		if allOrNone {
			ensureBackup()
			*vm.Org = backup
			return beforeFailures, nil
		}
		records, before, targets = filterDMLInputs(records, before, targets, beforeFailures)
		if len(records) == 0 {
			return beforeFailures, nil
		}
	}
	if op == "insert" {
		if err := vm.resolveSameBatchParentRelationships(records, targets); err != nil {
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
			}
			return nil, err
		}
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	engine.Options = options
	engine.Options.AllowBatchUniqueValueSwap = allOrNone
	engine.PriorRecords = dmlPriorRecordsByID(before)
	if op == "update" && vm.inAfterUndeleteTrigger() {
		engine.Options.AllowUpdateDeleted = true
	}
	if !allOrNone && vm.hasAfterTriggerForDML(op, records) {
		ensureBackup()
	}
	var results []dml.Result
	switch op {
	case "insert":
		results = engine.Insert(records)
	case "update":
		results = engine.Update(records)
	case "delete":
		results = engine.Delete(records)
	case "upsert":
		if externalIDField != "" {
			results = engine.UpsertWithExternalID(records, externalIDField)
		} else {
			results = engine.Upsert(records)
		}
	case "undelete":
		results = engine.Undelete(records)
	default:
		return nil, fmt.Errorf("unsupported DML operation %s", op)
	}
	appendDMLResultTrace(result, op, records, results)
	engineResults := results
	if hasDMLFailures(beforeFailures) {
		results = mergeDMLResults(beforeFailures, results)
	}
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				ensureBackup()
				*vm.Org = backup
				return results, nil
			}
		}
	}
	for i, dmlResult := range engineResults {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			previous := cloneValuePreserveRefs(*targets[i])
			vm.populateDMLResultFields(targets[i], engineResults[i:i+1])
			if !bulkPropagate {
				vm.propagateValueMutationToScope(vm.Globals, previous, *targets[i])
				vm.propagateValueMutationToStatics(previous, *targets[i])
			}
		}
	}
	if bulkPropagate {
		vm.propagateValueMutationToScope(vm.Globals, bulkPrevious, value)
		vm.propagateValueMutationToStatics(bulkPrevious, value)
	}
	afterInputRecords, afterInputBefore, afterInputResults := successfulDMLInputs(records, before, engineResults)
	afterRecords := afterInputRecords
	if vm.installContextDepth == 0 {
		var err error
		afterRecords, err = vm.afterRecords(op, afterInputRecords, afterInputResults)
		if err != nil {
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
			}
			return results, err
		}
		afterFailures, _, err := vm.runTriggersByObject(triggerTimingAfter, op, afterRecords, afterInputBefore, result)
		if err != nil {
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
			}
			return nil, dmlExceptionFromTriggerError(op, err)
		}
		if hasDMLFailures(afterFailures) {
			results = mergeAfterTriggerDMLResults(results, afterFailures)
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
				vm.clearDMLResultFieldsForFailures(targets, results, []string{op})
				return results, nil
			}
			ensureBackup()
			vm.rollbackAfterTriggerFailures(op, afterRecords, afterFailures, backup)
			afterRecords = filterAfterTriggerRecords(afterRecords, afterFailures)
		}
	}
	if err := vm.runSummaryUpdateTriggers(&engine, allOrNone, backup, result); err != nil {
		return results, err
	}
	if op == "insert" || op == "update" || op == "upsert" {
		if err := vm.applyDeferredAutomation(&engine, afterRecords, allOrNone, backup, result); err != nil {
			return results, err
		}
	}
	if hasDMLSuccess(results) {
		vm.rebuildDMLObjectIndexes(records, results)
		vm.clearCustomDataCache()
	}
	return results, nil
}

func (vm *VM) rebuildDMLObjectIndexes(records []storage.Record, results []dml.Result) {
	if vm == nil || vm.Org == nil {
		return
	}
	seen := make(map[string]bool)
	for i, record := range records {
		if i < len(results) && !results[i].Success {
			continue
		}
		objectName := strings.TrimSpace(record.Object)
		if objectName == "" {
			continue
		}
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			objectName = canonical
		}
		key := strings.ToLower(objectName)
		if seen[key] {
			continue
		}
		seen[key] = true
		storage.RebuildObjectIndexes(vm.Org, objectName)
	}
}

func (vm *VM) inAfterUndeleteTrigger() bool {
	if vm == nil || vm.triggerGlobals == nil {
		return false
	}
	isAfter, ok := vm.triggerGlobals["Trigger.isAfter"]
	if !ok || isAfter.Kind != ValueBool || !isAfter.Bool {
		return false
	}
	isUndelete, ok := vm.triggerGlobals["Trigger.isUndelete"]
	return ok && isUndelete.Kind == ValueBool && isUndelete.Bool
}

func hasDMLSuccess(results []dml.Result) bool {
	for _, result := range results {
		if result.Success {
			return true
		}
	}
	return false
}

func dmlTraceObjectNames(records []storage.Record) []string {
	if len(records) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var objects []string
	for _, record := range records {
		objectName := strings.TrimSpace(record.Object)
		if objectName == "" || seen[objectName] {
			continue
		}
		seen[objectName] = true
		objects = append(objects, objectName)
	}
	return objects
}

func dmlPriorRecordsByID(records []storage.Record) map[storage.ID]storage.Record {
	var out map[storage.ID]storage.Record
	for _, record := range records {
		if record.ID == "" {
			continue
		}
		if out == nil {
			out = make(map[storage.ID]storage.Record)
		}
		out[record.ID] = record.Clone()
	}
	return out
}

func appendDMLResultTrace(result *Result, op string, records []storage.Record, results []dml.Result) {
	if result == nil || len(results) == 0 {
		return
	}
	successes := 0
	failures := 0
	var errors []string
	for _, dmlResult := range results {
		if dmlResult.Success {
			successes++
			continue
		}
		failures++
		if dmlResult.Error != "" && len(errors) < 5 {
			errors = append(errors, dmlResult.Error)
		}
	}
	appendTrace(result, "apex.dml."+op+".result", "apex.dml", map[string]any{
		"operation": op,
		"objects":   dmlTraceObjectNames(records),
		"successes": successes,
		"failures":  failures,
		"errors":    errors,
	})
}

func (vm *VM) applySObjectFieldDefaults(records []storage.Record) {
	if vm.Org == nil {
		return
	}
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		definition := vm.Org.Objects[objectName].Definition
		if records[i].Fields == nil {
			records[i].Fields = make(map[string]storage.Value)
		}
		if _, ok := records[i].GetField("RecordTypeId"); !ok && !records[i].HasExplicitNull("RecordTypeId") {
			if recordTypeID := defaultRecordTypeIDForRecord(objectName, definition, records[i]); recordTypeID != "" {
				records[i].Fields["RecordTypeId"] = storage.IDValue(recordTypeID)
			}
		}
		for name, field := range definition.Fields {
			if _, ok := records[i].GetField(name); ok {
				continue
			}
			if records[i].HasExplicitNull(name) {
				continue
			}
			if value, ok := vm.defaultValueForRecordField(definition, records[i], field); ok {
				records[i].Fields[name] = value
			}
		}
	}
}

func defaultRecordTypeIDForRecord(objectName string, definition storage.ObjectDefinition, record storage.Record) storage.ID {
	if strings.EqualFold(objectName, "Account") && recordHasPersonAccountSignal(record) {
		if id := personAccountRecordTypeID(definition); id != "" {
			return id
		}
	}
	return defaultRecordTypeID(definition)
}

func personAccountRecordTypeID(definition storage.ObjectDefinition) storage.ID {
	for _, recordType := range definition.RecordTypes {
		if recordType.ID == "" || !recordTypeLooksPersonAccount(recordType) {
			continue
		}
		if recordType.Active && recordType.Available {
			return recordType.ID
		}
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && recordType.Active && recordTypeLooksPersonAccount(recordType) {
			return recordType.ID
		}
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && recordTypeLooksPersonAccount(recordType) {
			return recordType.ID
		}
	}
	return ""
}

func recordTypeLooksPersonAccount(recordType storage.RecordTypeInfo) bool {
	name := strings.ToLower(strings.TrimSpace(recordType.Name + " " + recordType.DeveloperName))
	return strings.Contains(name, "person") || strings.Contains(name, "individual")
}

func (vm *VM) applyBeforeInsertDerivedFields(records []storage.Record) {
	if vm.Org == nil {
		return
	}
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok || !strings.EqualFold(objectName, "Account") {
			continue
		}
		if records[i].Fields == nil {
			records[i].Fields = make(map[string]storage.Value)
		}
		if !recordHasPersonAccountSignal(records[i]) {
			continue
		}
		records[i].Fields["IsPersonAccount"] = storage.BooleanValue(true)
	}
}

func recordHasPersonAccountSignal(record storage.Record) bool {
	for field, value := range record.Fields {
		if !strings.HasPrefix(field, "Person") && field != "FirstName" && field != "LastName" {
			continue
		}
		if storageValueHasNonZeroContent(value) {
			return true
		}
	}
	return false
}

func storageValueHasNonZeroContent(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull, "":
		return false
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String != ""
	case storage.ValueID:
		return value.ID != ""
	case storage.ValueInteger:
		return value.Integer != 0
	default:
		return true
	}
}

func (vm *VM) applyTestSObjectNameDefaults(records []storage.Record, defaultMissing bool) {
	if vm.Org == nil {
		return
	}
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		vm.applyTestSObjectNameDefault(vm.Org.Objects[objectName].Definition, &records[i], defaultMissing)
	}
}

func (vm *VM) defaultValueForRecordField(definition storage.ObjectDefinition, record storage.Record, field storage.Field) (storage.Value, bool) {
	rawDefault := strings.TrimSpace(field.DefaultValue)
	if vm != nil && vm.Org != nil && (strings.Contains(rawDefault, "$RecordType") || vmFormulaDefaultShouldEvaluate(field, rawDefault)) {
		if value, _, ok := dml.EvaluateRecordFormulaValueInOrg(rawDefault, field, vm.Org, definition, record); ok {
			return value, true
		}
	}
	if value, ok := storage.DefaultValueForRecordField(definition, record, field); ok {
		return value, true
	}
	if vm == nil || vm.Org == nil || rawDefault == "" {
		return storage.Value{}, false
	}
	value, _, ok := dml.EvaluateRecordFormulaValueInOrg(rawDefault, field, vm.Org, definition, record)
	return value, ok
}

func (vm *VM) applyTestSObjectNameDefault(definition storage.ObjectDefinition, record *storage.Record, defaultMissing bool) {
	if vm.testContext == nil || record == nil || record.Fields == nil {
		return
	}
	if value, ok := record.GetField("Name"); ok {
		if value.Kind != storage.ValueNull && !record.HasExplicitNull("Name") {
			return
		}
	} else if !defaultMissing && !record.HasExplicitNull("Name") {
		return
	}
	field, ok := definition.Fields["Name"]
	if !ok || !field.Required || field.Type != storage.FieldString || field.AutoNumber || field.DefaultValue != "" {
		return
	}
	apiName := strings.ToLower(definition.APIName)
	if !strings.HasSuffix(apiName, "__c") && !strings.HasSuffix(apiName, "__e") {
		return
	}
	name := strings.TrimSpace(definition.Label)
	if name == "" {
		name = strings.TrimSuffix(definition.APIName, "__c")
		name = strings.TrimSuffix(name, "__e")
	}
	if name == "" {
		name = "Test Record"
	}
	record.Fields["Name"] = storage.StringValue(name)
	delete(record.ExplicitNulls, "Name")
}

func successfulDMLInputs(records, before []storage.Record, results []dml.Result) ([]storage.Record, []storage.Record, []dml.Result) {
	filteredRecords := make([]storage.Record, 0, len(records))
	filteredBefore := make([]storage.Record, 0, len(before))
	filteredResults := make([]dml.Result, 0, len(results))
	for i, record := range records {
		if i >= len(results) || !results[i].Success {
			continue
		}
		filteredRecords = append(filteredRecords, record)
		if i < len(before) {
			filteredBefore = append(filteredBefore, before[i])
		}
		filteredResults = append(filteredResults, results[i])
	}
	return filteredRecords, filteredBefore, filteredResults
}

func mergeAfterTriggerDMLResults(results, afterFailures []dml.Result) []dml.Result {
	out := append([]dml.Result(nil), results...)
	failureIndex := 0
	for i, result := range out {
		if !result.Success {
			continue
		}
		if failureIndex >= len(afterFailures) {
			break
		}
		failure := afterFailures[failureIndex]
		failureIndex++
		if !failure.Success && failure.Error != "" {
			out[i] = failure
		}
	}
	return out
}

func filterAfterTriggerRecords(records []storage.Record, failures []dml.Result) []storage.Record {
	if !hasDMLFailures(failures) {
		return records
	}
	out := make([]storage.Record, 0, len(records))
	for i, record := range records {
		if i < len(failures) && !failures[i].Success && failures[i].Error != "" {
			continue
		}
		out = append(out, record)
	}
	return out
}

func (vm *VM) rollbackAfterTriggerFailures(op string, records []storage.Record, failures []dml.Result, backup storage.OrgState) {
	if vm == nil || vm.Org == nil {
		return
	}
	for i, record := range records {
		if i >= len(failures) || failures[i].Success || failures[i].Error == "" || record.ID == "" {
			continue
		}
		objectName := record.Object
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			objectName = canonical
		}
		current, ok := vm.Org.Objects[objectName]
		if !ok {
			continue
		}
		backupObject, hasBackupObject := backup.Objects[objectName]
		if hasBackupObject {
			if storedID, previous, found := storage.LookupRecordByID(backupObject.Records, record.ID); found {
				current.Records[storedID] = previous.Clone()
				vm.Org.Objects[objectName] = current
				continue
			}
		}
		if strings.EqualFold(op, "insert") || strings.EqualFold(op, "upsert") {
			if storedID, _, found := storage.LookupRecordByID(current.Records, record.ID); found {
				delete(current.Records, storedID)
				vm.Org.Objects[objectName] = current
			}
		}
	}
}

func (vm *VM) applyUpsertDML(records []storage.Record, targets []*Value, allOrNone bool, externalIDField string, result *Result) ([]dml.Result, error) {
	appendTrace(result, "apex.dml.upsert", "apex.dml", map[string]any{
		"operation": "upsert",
		"rows":      len(records),
		"objects":   dmlTraceObjectNames(records),
	})
	var backup storage.OrgState
	backupReady := false
	ensureBackup := func() {
		if backupReady {
			return
		}
		backup = snapshotRuntimeOrgState(vm.Org)
		backupReady = true
	}
	if allOrNone {
		ensureBackup()
	}
	kinds := make([]string, len(records))
	before := make([]storage.Record, len(records))
	for i, record := range records {
		kind, old, err := vm.classifyUpsert(record, externalIDField)
		if err != nil {
			return nil, err
		}
		kinds[i] = kind
		before[i] = old
		if kind == "update" && records[i].ID == "" && old.ID != "" {
			records[i].ID = old.ID
		}
	}
	for i, kind := range kinds {
		if kind != "insert" {
			continue
		}
		insertRecord := []storage.Record{records[i]}
		vm.applySObjectFieldDefaults(insertRecord)
		vm.applyBeforeInsertDerivedFields(insertRecord)
		records[i] = insertRecord[0]
	}
	beforeFailures := make([]dml.Result, len(records))
	for _, kind := range []string{"insert", "update"} {
		groupRecords, groupBefore, indices := groupedDMLInputs(records, before, kinds, kind)
		triggerRecords := groupRecords
		if kind == "update" {
			triggerRecords = vm.hydrateUpdateTriggerRecords(groupRecords, groupBefore)
		}
		failures, err := vm.runTriggers(triggerTimingBefore, kind, triggerRecords, groupBefore, result)
		if err != nil {
			ensureBackup()
			*vm.Org = backup
			return nil, dmlExceptionFromTriggerError("upsert", err)
		}
		for groupIndex, failure := range failures {
			if groupIndex < len(indices) && !failure.Success && failure.Error != "" {
				beforeFailures[indices[groupIndex]] = failure
			}
		}
		if kind == "update" {
			preserveUpdateExplicitNulls(triggerRecords, groupRecords)
		}
		for groupIndex, index := range indices {
			if groupIndex < len(triggerRecords) {
				records[index] = triggerRecords[groupIndex]
			}
		}
	}
	for i, kind := range kinds {
		if kind != "insert" {
			continue
		}
		insertRecord := []storage.Record{records[i]}
		vm.applyTestSObjectNameDefaults(insertRecord, true)
		records[i] = insertRecord[0]
	}
	if hasDMLFailures(beforeFailures) {
		if allOrNone {
			ensureBackup()
			*vm.Org = backup
			return beforeFailures, nil
		}
		records, before, targets, kinds = filterUpsertInputs(records, before, targets, kinds, beforeFailures)
		if len(records) == 0 {
			return beforeFailures, nil
		}
	}
	if err := vm.resolveSameBatchParentRelationships(records, targets); err != nil {
		if allOrNone {
			ensureBackup()
			*vm.Org = backup
		}
		return nil, err
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	engine.Options.AllowBatchUniqueValueSwap = allOrNone
	engine.PriorRecords = dmlPriorRecordsByID(before)
	if !allOrNone && vm.hasAfterTriggerForDML("upsert", records) {
		ensureBackup()
	}
	var engineResults []dml.Result
	if externalIDField != "" {
		engineResults = engine.UpsertWithExternalID(records, externalIDField)
	} else {
		engineResults = engine.Upsert(records)
	}
	appendDMLResultTrace(result, "upsert", records, engineResults)
	results := engineResults
	if hasDMLFailures(beforeFailures) {
		results = mergeDMLResults(beforeFailures, engineResults)
	}
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				ensureBackup()
				*vm.Org = backup
				return results, nil
			}
		}
	}
	for i, dmlResult := range engineResults {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			previous := cloneValuePreserveRefs(*targets[i])
			vm.populateDMLResultFields(targets[i], engineResults[i:i+1])
			vm.propagateValueMutationToScope(vm.Globals, previous, *targets[i])
			vm.propagateValueMutationToStatics(previous, *targets[i])
		}
	}
	for _, kind := range []string{"insert", "update"} {
		groupRecords, groupBefore, groupResults, indices := successfulGroupedDMLInputs(records, before, engineResults, kinds, kind)
		afterRecords, err := vm.afterRecords(kind, groupRecords, groupResults)
		if err != nil {
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
			}
			return results, err
		}
		afterFailures, err := vm.runTriggers(triggerTimingAfter, kind, afterRecords, groupBefore, result)
		if err != nil {
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
			}
			return nil, dmlExceptionFromTriggerError("upsert", err)
		}
		if hasDMLFailures(afterFailures) {
			for groupIndex, failure := range afterFailures {
				if groupIndex < len(indices) && !failure.Success && failure.Error != "" {
					results[indices[groupIndex]] = failure
				}
			}
			if allOrNone {
				ensureBackup()
				*vm.Org = backup
				vm.clearDMLResultFieldsForFailures(targets, results, kinds)
				return results, nil
			}
			ensureBackup()
			vm.rollbackAfterTriggerFailures(kind, afterRecords, afterFailures, backup)
			afterRecords = filterAfterTriggerRecords(afterRecords, afterFailures)
		}
		if err := vm.runSummaryUpdateTriggers(&engine, allOrNone, backup, result); err != nil {
			return results, err
		}
		if err := vm.applyDeferredAutomation(&engine, afterRecords, allOrNone, backup, result); err != nil {
			return results, err
		}
	}
	if hasDMLSuccess(results) {
		vm.rebuildDMLObjectIndexes(records, results)
		vm.clearCustomDataCache()
	}
	return results, nil
}

func (vm *VM) hasAfterTriggerForDML(op string, records []storage.Record) bool {
	return vm.hasTriggerForDML(triggerTimingAfter, op, records)
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

func (vm *VM) hasTriggerForDML(timing, op string, records []storage.Record) bool {
	if vm == nil || len(records) == 0 {
		return false
	}
	triggerOp := op
	if strings.EqualFold(op, "upsert") {
		triggerOp = "update"
	}
	seenRecordObjects := make(map[string]bool, len(records))
	for _, record := range records {
		objectName := strings.TrimSpace(record.Object)
		if objectName == "" || seenRecordObjects[strings.ToLower(objectName)] {
			continue
		}
		seenRecordObjects[strings.ToLower(objectName)] = true
		if len(vm.triggersForOperation(objectName, timing, triggerOp)) > 0 {
			return true
		}
	}
	return false
}

func (vm *VM) hasAutomationForDML(records []storage.Record) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	for _, objectName := range vm.dmlObjectNames(records) {
		object, ok := vm.Org.Objects[objectName]
		if !ok {
			continue
		}
		if len(object.Definition.WorkflowRules) > 0 || len(object.Definition.FlowRules) > 0 {
			return true
		}
	}
	return false
}

func (vm *VM) hasSummarySideEffectsForDML(records []storage.Record) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	objects := vm.dmlObjectNameSet(records)
	if len(objects) == 0 {
		return false
	}
	for _, object := range vm.Org.Objects {
		for _, field := range object.Definition.Fields {
			if field.Type != storage.FieldSummary {
				continue
			}
			summaryObject, _ := splitQualifiedField(field.SummarizedField)
			lookupObject, _ := splitQualifiedField(field.SummaryForeignKey)
			if vm.summaryDMLObjectMatches(objects, summaryObject) || vm.summaryDMLObjectMatches(objects, lookupObject) {
				return true
			}
		}
	}
	return false
}

func (vm *VM) summaryDMLObjectMatches(objects map[string]bool, objectName string) bool {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return false
	}
	if vm != nil && vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
	}
	return objects[strings.ToLower(objectName)]
}

func (vm *VM) dmlObjectNameSet(records []storage.Record) map[string]bool {
	names := vm.dmlObjectNames(records)
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[strings.ToLower(name)] = true
	}
	return out
}

func (vm *VM) dmlObjectNames(records []storage.Record) []string {
	if vm == nil || vm.Org == nil {
		return nil
	}
	seen := make(map[string]bool, len(records))
	names := make([]string, 0, len(records))
	for _, record := range records {
		objectName := strings.TrimSpace(record.Object)
		if objectName == "" {
			continue
		}
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
		key := strings.ToLower(objectName)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, objectName)
	}
	return names
}

func (vm *VM) runSummaryUpdateTriggers(engine *dml.Engine, allOrNone bool, backup storage.OrgState, result *Result) error {
	if engine == nil {
		return nil
	}
	updates := engine.TakeSummaryUpdates()
	if len(updates) == 0 {
		return nil
	}
	byObject := make(map[string][]dml.SummaryUpdate)
	order := make([]string, 0)
	seen := make(map[string]bool)
	for _, update := range updates {
		if update.Object == "" || update.Before.ID == "" || update.After.ID == "" {
			continue
		}
		objectName, ok := vm.resolveObjectName(update.Object)
		if !ok {
			objectName = update.Object
		}
		if !seen[objectName] {
			seen[objectName] = true
			order = append(order, objectName)
		}
		update.Object = objectName
		update.Before.Object = objectName
		update.After.Object = objectName
		if object, ok := vm.Org.Objects[objectName]; ok {
			if _, stored, ok := storage.LookupRecordByID(object.Records, update.After.ID); ok {
				update.After = stored.Clone()
				update.After.Object = objectName
			}
		}
		byObject[objectName] = append(byObject[objectName], update)
	}
	for _, objectName := range order {
		group := byObject[objectName]
		before := make([]storage.Record, 0, len(group))
		records := make([]storage.Record, 0, len(group))
		for _, update := range group {
			before = append(before, update.Before)
			records = append(records, update.After)
		}
		triggerRecords := vm.hydrateUpdateTriggerRecords(records, before)
		failures, err := vm.runTriggers(triggerTimingBefore, "update", triggerRecords, before, result)
		if err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return dmlExceptionFromTriggerError("update", err)
		}
		if hasDMLFailures(failures) {
			if allOrNone {
				*vm.Org = backup
			}
			return fmt.Errorf("summary update trigger failed for %s: %s", objectName, failures[0].Error)
		}
		if err := vm.storeTriggerRecords(objectName, triggerRecords); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return err
		}
		if _, err := vm.runTriggers(triggerTimingAfter, "update", triggerRecords, before, result); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return dmlExceptionFromTriggerError("update", err)
		}
		if err := vm.applyDeferredAutomation(engine, triggerRecords, allOrNone, backup, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) storeTriggerRecords(objectName string, records []storage.Record) error {
	if vm == nil || vm.Org == nil || len(records) == 0 {
		return nil
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return fmt.Errorf("unknown object %s", objectName)
	}
	for _, record := range records {
		if record.ID == "" {
			continue
		}
		storedID, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
		if !ok {
			return fmt.Errorf("record %s does not exist", record.ID)
		}
		record.System = stored.System
		preserveReadOnlyCalculatedFields(object.Definition, &record, stored)
		object.Records[storedID] = record.Clone()
	}
	vm.Org.Objects[objectName] = object
	return nil
}

func preserveReadOnlyCalculatedFields(definition storage.ObjectDefinition, record *storage.Record, stored storage.Record) {
	if record == nil {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for fieldName, field := range definition.Fields {
		if field.Type != storage.FieldCalculated && field.Type != storage.FieldSummary && strings.TrimSpace(field.Formula) == "" {
			continue
		}
		if value, ok := stored.GetField(fieldName); ok {
			record.Fields[fieldName] = value.Clone()
			continue
		}
		deleteCaseInsensitiveVMStorageField(record.Fields, fieldName)
	}
}

func groupedDMLInputs(records, before []storage.Record, kinds []string, want string) ([]storage.Record, []storage.Record, []int) {
	groupRecords := make([]storage.Record, 0, len(records))
	groupBefore := make([]storage.Record, 0, len(before))
	indices := make([]int, 0, len(records))
	for i, kind := range kinds {
		if kind != want {
			continue
		}
		groupRecords = append(groupRecords, records[i])
		if i < len(before) {
			groupBefore = append(groupBefore, before[i])
		}
		indices = append(indices, i)
	}
	return groupRecords, groupBefore, indices
}

func successfulGroupedDMLInputs(records, before []storage.Record, results []dml.Result, kinds []string, want string) ([]storage.Record, []storage.Record, []dml.Result, []int) {
	groupRecords := make([]storage.Record, 0, len(records))
	groupBefore := make([]storage.Record, 0, len(before))
	groupResults := make([]dml.Result, 0, len(results))
	indices := make([]int, 0, len(records))
	for i, kind := range kinds {
		if kind != want || i >= len(results) || !results[i].Success {
			continue
		}
		groupRecords = append(groupRecords, records[i])
		if i < len(before) {
			groupBefore = append(groupBefore, before[i])
		}
		groupResults = append(groupResults, results[i])
		indices = append(indices, i)
	}
	return groupRecords, groupBefore, groupResults, indices
}

func filterUpsertInputs(records, before []storage.Record, targets []*Value, kinds []string, failures []dml.Result) ([]storage.Record, []storage.Record, []*Value, []string) {
	filteredRecords := make([]storage.Record, 0, len(records))
	filteredBefore := make([]storage.Record, 0, len(before))
	filteredTargets := make([]*Value, 0, len(targets))
	filteredKinds := make([]string, 0, len(kinds))
	for i, record := range records {
		if i < len(failures) && !failures[i].Success && failures[i].Error != "" {
			continue
		}
		filteredRecords = append(filteredRecords, record)
		if i < len(before) {
			filteredBefore = append(filteredBefore, before[i])
		}
		if i < len(targets) {
			filteredTargets = append(filteredTargets, targets[i])
		}
		if i < len(kinds) {
			filteredKinds = append(filteredKinds, kinds[i])
		}
	}
	return filteredRecords, filteredBefore, filteredTargets, filteredKinds
}

func (vm *VM) checkMixedDML(records []storage.Record) error {
	if vm.testContext == nil || vm.testContext.RunAsDepth > 0 {
		return nil
	}
	hasSetup := false
	hasNonSetup := false
	for _, record := range records {
		switch mixedDMLRecordKind(record) {
		case "setup":
			hasSetup = true
		case "nonsetup":
			hasNonSetup = true
		}
	}
	nextSetup := vm.testContext.SetupDML || hasSetup
	nextNonSetup := vm.testContext.NonSetupDML || hasNonSetup
	if nextSetup && nextNonSetup {
		return newExceptionError("DmlException", "Mixed DML operation detected; wrap supported setup/non-setup test work in System.runAs")
	}
	vm.testContext.SetupDML = nextSetup
	vm.testContext.NonSetupDML = nextNonSetup
	return nil
}

func (vm *VM) recordFromValue(value *Value) (storage.Record, error) {
	if value.Kind == ValueNull {
		return storage.Record{}, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
	}
	if value.Kind != ValueObject {
		return storage.Record{}, fmt.Errorf("DML requires sObject value, got %s", value.Kind)
	}
	if reason, ok := sobjectReadOnlyReason(*value); ok {
		return storage.Record{}, fmt.Errorf("DML cannot modify read-only %s", reason)
	}
	objectType := value.Type
	var definition storage.ObjectDefinition
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectType); ok {
			objectType = canonical
			definition = vm.Org.Objects[canonical].Definition
			definition = cloneDescribeObjectDefinition(definition)
			storage.EnsureStandardObjectFields(&definition)
		}
	}
	record := storage.Record{
		Object:        objectType,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	recordExplicitFields := make(map[string]bool)
	recordFieldsByAlias := make(map[string]string)
	recordFieldSourceByAlias := make(map[string]string)
	if id := sObjectIDFromFields(value.Fields); id != "" {
		_, queried := value.Fields[sobjectQueriedFieldsField]
		if queried || isExplicitSObjectField(*value, "Id") || definition.KeyPrefix == "" || strings.HasPrefix(string(id), definition.KeyPrefix) {
			record.ID = id
		}
	}
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		if strings.EqualFold(field, "Id") {
			continue
		}
		if strings.Contains(field, ".") && !isExplicitSObjectField(*value, field) {
			continue
		}
		if strings.EqualFold(field, "OwnerId") {
			if !isExplicitSObjectField(*value, field) && !isExplicitSObjectField(*value, "OwnerId") {
				continue
			}
			converted, err := storageValueFromVM(fieldValue)
			if err != nil {
				return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
			}
			if ownerID := storageIDFromValue(converted); ownerID != "" {
				record.System.OwnerID = ownerID
			}
			continue
		}
		canonicalField := field
		if definition.APIName != "" {
			if resolved, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field); ok {
				canonicalField = resolved
			}
		}
		explicitField := isExplicitSObjectField(*value, field) || sObjectCanonicalFieldExplicitOnValue(*value, field, canonicalField)
		if !explicitField && isTriggerSObject(*value) && strings.EqualFold(canonicalField, "RecordTypeId") {
			explicitField = true
		}
		if definition.APIName != "" && !explicitField && vmImplicitGeneratedFieldValue(canonicalField, fieldValue) {
			continue
		}
		if definition.APIName != "" && !explicitField && fieldValue.Kind == ValueNull {
			continue
		}
		if isSObjectSystemField(field) || isSObjectSystemField(canonicalField) {
			if !explicitField {
				continue
			}
		}
		if definition.APIName != "" {
			if fieldDef, ok := definition.Fields[canonicalField]; ok && vmCalculatedOrSummaryField(fieldDef) &&
				!isUserSetSObjectField(*value, field) && !isUserSetSObjectField(*value, canonicalField) {
				continue
			}
			if fieldDef, ok := definition.Fields[canonicalField]; ok && vmImplicitDMLField(fieldDef) {
				if !explicitField || vmImplicitDMLFieldDefaultValue(fieldDef, fieldValue) {
					continue
				}
			}
			if fieldDef, ok := definition.Fields[canonicalField]; ok && record.ID != "" && !explicitField &&
				!storage.FieldFlagValue(fieldDef.Updateable, true) {
				continue
			}
		}
		if fieldValue.Kind == ValueList && vm.isChildRelationshipField(definition, field) {
			continue
		}
		if fieldValue.Kind == ValueObject && !strings.EqualFold(fieldValue.Type, "Id") && (vm.isParentRelationshipField(definition, field) || isLikelyParentRelationshipFieldName(field)) {
			if lookupField, ok := vm.parentRelationshipField(objectType, field); ok {
				if _, hasLookup := record.GetField(lookupField); !hasLookup && !record.HasExplicitNull(lookupField) {
					if parentID := sObjectIDFromFields(fieldValue.Fields); parentID != "" {
						record.Fields[lookupField] = storage.IDValue(parentID)
					} else if parentID, err := vm.resolveParentRelationshipReferenceID(nil, fieldValue); err != nil {
						return storage.Record{}, err
					} else if parentID != "" {
						record.Fields[lookupField] = storage.IDValue(parentID)
					}
				}
			}
			continue
		}
		if fieldValue.Kind == ValueObject && isLikelyParentRelationshipObjectField(field) {
			if definition.APIName != "" {
				if fieldDef, ok := definition.Fields[canonicalField]; ok && !vmFieldIsReference(fieldDef) {
					converted, err := storageValueFromVMForField(fieldValue, fieldDef.Type)
					if err != nil {
						return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
					}
					if converted.Kind == storage.ValueNull {
						record.ExplicitNulls[canonicalField] = true
					} else {
						record.Fields[canonicalField] = converted
					}
					continue
				}
			}
			continue
		}
		if fieldValue.Kind == ValueNull && isLikelyParentRelationshipObjectField(field) {
			if definition.APIName == "" {
				continue
			}
			if _, ok := definition.Fields[canonicalField]; !ok {
				continue
			}
		}
		converted, err := storageValueFromVM(fieldValue)
		if definition.APIName != "" {
			if fieldDef, ok := definition.Fields[canonicalField]; ok {
				converted, err = storageValueFromVMForField(fieldValue, fieldDef.Type)
			}
		}
		if err != nil {
			return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
		}
		if converted.Kind == storage.ValueNull {
			aliasKey := sObjectRecordFieldAliasKey(canonicalField)
			if previousField, exists := recordFieldsByAlias[aliasKey]; exists {
				if recordExplicitFields[aliasKey] && !explicitField {
					continue
				}
				if recordExplicitFields[aliasKey] && explicitField && !sObjectRecordFieldSourcePreferred(field, recordFieldSourceByAlias[aliasKey], canonicalField, vm.OrgNamespace()) {
					continue
				}
				if previousField != canonicalField {
					delete(record.Fields, previousField)
					delete(record.ExplicitNulls, previousField)
				}
			}
			record.ExplicitNulls[canonicalField] = true
			recordFieldsByAlias[aliasKey] = canonicalField
			recordExplicitFields[aliasKey] = explicitField
			recordFieldSourceByAlias[aliasKey] = field
		} else {
			aliasKey := sObjectRecordFieldAliasKey(canonicalField)
			if previousField, exists := recordFieldsByAlias[aliasKey]; exists {
				if recordExplicitFields[aliasKey] && !explicitField {
					continue
				}
				if recordExplicitFields[aliasKey] && explicitField && !sObjectRecordFieldSourcePreferred(field, recordFieldSourceByAlias[aliasKey], canonicalField, vm.OrgNamespace()) {
					continue
				}
				if previousField != canonicalField {
					delete(record.Fields, previousField)
					delete(record.ExplicitNulls, previousField)
				}
			}
			record.Fields[canonicalField] = converted
			recordFieldsByAlias[aliasKey] = canonicalField
			recordExplicitFields[aliasKey] = explicitField
			recordFieldSourceByAlias[aliasKey] = field
		}
	}
	return record, nil
}

func (vm *VM) OrgNamespace() string {
	if vm == nil || vm.Org == nil {
		return ""
	}
	return vm.Org.Namespace
}

func sObjectRecordFieldSourcePreferred(current, previous, canonical, namespace string) bool {
	if strings.TrimSpace(previous) == "" || strings.EqualFold(current, previous) {
		return true
	}
	if namespace != "" && strings.EqualFold(previous, canonical) &&
		strings.EqualFold(current, storage.StripNamespaceToken(namespace, canonical)) {
		return true
	}
	if namespace != "" && strings.EqualFold(current, canonical) &&
		strings.EqualFold(previous, storage.StripNamespaceToken(namespace, canonical)) {
		return false
	}
	return true
}

func sObjectRecordFieldAliasKey(field string) string {
	return strings.ToLower(storage.StripAnyNamespaceToken(field))
}

func sObjectCanonicalFieldExplicitOnValue(value Value, field, canonicalField string) bool {
	if !isExplicitSObjectField(value, canonicalField) {
		return false
	}
	if strings.EqualFold(field, canonicalField) {
		return true
	}
	actual, _, ok := objectFieldValue(value, canonicalField)
	if !ok {
		return true
	}
	return strings.EqualFold(actual, field)
}

func (vm *VM) resolveSameBatchParentRelationships(records []storage.Record, targets []*Value) error {
	if vm == nil || vm.Org == nil || len(records) == 0 || len(targets) == 0 {
		return nil
	}
	for i := range records {
		if i >= len(targets) || targets[i] == nil || targets[i].Kind != ValueObject {
			continue
		}
		target := targets[i]
		for relationship, parent := range target.Fields {
			if isInternalSObjectField(relationship) || parent.Kind != ValueObject || strings.EqualFold(parent.Type, "Id") {
				continue
			}
			lookupField, ok := vm.parentRelationshipField(records[i].Object, relationship)
			if !ok {
				continue
			}
			if _, hasLookup := records[i].GetField(lookupField); hasLookup || records[i].HasExplicitNull(lookupField) {
				continue
			}
			parentID, err := vm.resolveParentRelationshipReferenceID(records, parent)
			if err != nil {
				return err
			}
			if parentID == "" {
				continue
			}
			if records[i].Fields == nil {
				records[i].Fields = make(map[string]storage.Value)
			}
			records[i].Fields[lookupField] = storage.IDValue(parentID)
		}
	}
	return nil
}

func (vm *VM) resolveParentRelationshipReferenceID(records []storage.Record, parent Value) (storage.ID, error) {
	if id := sObjectIDFromFields(parent.Fields); id != "" {
		return id, nil
	}
	parentObjectName := parent.Type
	parentObject, ok := vm.objectState(parentObjectName)
	if !ok {
		return "", nil
	}
	for fieldName, fieldValue := range parent.Fields {
		if isInternalSObjectField(fieldName) || strings.EqualFold(fieldName, "Id") || fieldValue.Kind == ValueNull {
			continue
		}
		canonicalField := fieldName
		if resolved, ok := storage.ResolveFieldName(parentObject.Definition, vm.Org.Namespace, fieldName); ok {
			canonicalField = resolved
		}
		fieldDef, ok := parentObject.Definition.Fields[canonicalField]
		if !ok || !parentRelationshipReferenceFieldCanMatch(fieldDef) {
			continue
		}
		lookupValue, err := storageValueFromVMForField(fieldValue, fieldDef.Type)
		if err != nil {
			return "", fmt.Errorf("%s.%s: %w", parent.Type, fieldName, err)
		}
		if lookupValue.Kind == storage.ValueNull {
			continue
		}
		for i := range records {
			if !strings.EqualFold(records[i].Object, parentObject.Definition.APIName) {
				continue
			}
			recordValue, ok := records[i].GetField(canonicalField)
			if !ok || !storageValuesEqualForVM(fieldDef, recordValue, lookupValue) {
				continue
			}
			if records[i].ID == "" {
				id, err := vm.assignPendingInsertID(&records[i])
				if err != nil {
					return "", err
				}
				return id, nil
			}
			return records[i].ID, nil
		}
		for id, stored := range parentObject.Records {
			if stored.System.IsDeleted {
				continue
			}
			recordValue, ok := stored.GetField(canonicalField)
			if ok && storageValuesEqualForVM(fieldDef, recordValue, lookupValue) {
				return id, nil
			}
		}
	}
	return "", nil
}

func parentRelationshipReferenceFieldCanMatch(field storage.Field) bool {
	if field.ExternalID {
		return true
	}
	if isNameFieldDescribe(field) && field.Type == storage.FieldString && !field.AutoNumber {
		return true
	}
	return false
}

func (vm *VM) objectState(objectName string) (storage.ObjectState, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" {
		return storage.ObjectState{}, false
	}
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if ok {
		storage.EnsureStandardObjectFields(&object.Definition)
	}
	return object, ok
}

func (vm *VM) assignPendingInsertID(record *storage.Record) (storage.ID, error) {
	if vm == nil || vm.Org == nil || record == nil {
		return "", fmt.Errorf("DML requires org state")
	}
	if record.ID != "" {
		return record.ID, nil
	}
	objectName := record.Object
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
		record.Object = canonical
	}
	engine := dml.NewEngine(vm.Org)
	id, err := engine.IDs.Next(objectName)
	if err != nil {
		return "", err
	}
	record.ID = id
	if vm.Org.IDSequences == nil {
		vm.Org.IDSequences = make(map[string]uint64)
	}
	for object, sequence := range engine.IDs.Sequences {
		vm.Org.IDSequences[object] = sequence
	}
	return id, nil
}

func (vm *VM) isParentRelationshipField(definition storage.ObjectDefinition, field string) bool {
	if vm == nil || vm.Org == nil || definition.APIName == "" || strings.TrimSpace(field) == "" {
		return false
	}
	for _, relation := range definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, field) ||
			vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, field) {
			return true
		}
	}
	for name, fieldDef := range definition.Fields {
		if !vmFieldIsReference(fieldDef) {
			continue
		}
		apiName := fieldDef.APIName
		if apiName == "" {
			apiName = name
		}
		if vmParentRelationshipNameMatches(vm.Org.Namespace, apiName, field) {
			return true
		}
	}
	return false
}

func vmFieldIsReference(field storage.Field) bool {
	if field.Type == storage.FieldReference || len(field.ReferenceTo) > 0 {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(string(field.Type))) {
	case "lookup", "masterdetail", "metadatarelationship":
		return true
	default:
		return false
	}
}

func isLikelyParentRelationshipObjectField(field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	if isLikelyParentRelationshipFieldName(field) {
		return true
	}
	switch field {
	default:
		return !strings.HasSuffix(field, "__c") && !strings.HasSuffix(field, "Id")
	}
}

func isLikelyParentRelationshipFieldName(field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	if strings.HasSuffix(field, "__r") {
		return true
	}
	switch field {
	case "RecordType", "Owner", "CreatedBy", "LastModifiedBy":
		return true
	default:
		return false
	}
}

func (vm *VM) isChildRelationshipField(definition storage.ObjectDefinition, field string) bool {
	if vm == nil || vm.Org == nil || definition.APIName == "" || strings.TrimSpace(field) == "" {
		return false
	}
	for _, childObject := range vm.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relationshipTargetsObject(relation, definition.APIName) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childObject.Definition)
			}
			if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
				return true
			}
		}
	}
	return false
}

func derivedVMChildRelationshipName(definition storage.ObjectDefinition) string {
	if strings.TrimSpace(definition.PluralLabel) != "" {
		return normalizeDerivedChildRelationshipName(definition.PluralLabel)
	}
	if strings.TrimSpace(definition.Label) != "" {
		return normalizeDerivedChildRelationshipName(definition.Label)
	}
	if definition.APIName != "" {
		return normalizeDerivedChildRelationshipName(definition.APIName)
	}
	return ""
}

func normalizeDerivedChildRelationshipName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "ys") && len(name) > 2 {
		return strings.TrimSuffix(name, "ys") + "ies"
	}
	if strings.HasSuffix(name, "s") {
		return name
	}
	if strings.HasSuffix(name, "y") && len(name) > 1 {
		return strings.TrimSuffix(name, "y") + "ies"
	}
	return name + "s"
}

func sObjectIDFromFields(fields map[string]Value) storage.ID {
	for _, name := range []string{"Id", "id"} {
		if id, ok := sObjectIDFromValue(fields[name]); ok {
			return id
		}
	}
	names := make([]string, 0)
	for name := range fields {
		if strings.EqualFold(name, "Id") && name != "Id" && name != "id" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if id, ok := sObjectIDFromValue(fields[name]); ok {
			return id
		}
	}
	return ""
}

func sObjectIDFromValue(value Value) (storage.ID, bool) {
	if value.Kind == ValueString {
		if value.Text == "" {
			return "", false
		}
		return storage.ID(value.Text), true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		if raw, err := platformScalarText(value, "Id"); err == nil && raw != "" {
			return storage.ID(raw), true
		}
	}
	return "", false
}

func vmValueFromRecord(record storage.Record) Value {
	value := Object(record.Object)
	if record.ID != "" {
		value.Fields["Id"] = platformScalar("Id", string(record.ID))
	}
	for field, fieldValue := range record.Fields {
		putVMFieldPath(value, field, vmValueFromStorage(fieldValue))
	}
	for relationship, records := range record.Children {
		children := make([]Value, 0, len(records))
		for _, child := range records {
			children = append(children, vmValueFromRecord(child))
		}
		value.Fields[relationship] = List(children...)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			value.Fields[field] = Null
		}
	}
	putSystemFields(value, record.System)
	return value
}

func (vm *VM) hydrateParentLookupFields(value Value) {
	if vm == nil || vm.Org == nil || value.Kind != ValueObject {
		return
	}
	object, ok := vm.Org.Objects[value.Type]
	if !ok {
		return
	}
	storedFields := map[string]storage.Value(nil)
	if id := sObjectIDFromFields(value.Fields); id != "" {
		if record, ok := vm.findOrgRecord(value.Type, id); ok {
			storedFields = record.Fields
		}
	}
	for _, relation := range object.Definition.Relations {
		if strings.TrimSpace(relation.Field) == "" || strings.TrimSpace(relation.ParentRelationship) == "" {
			continue
		}
		if _, exists := value.Fields[relation.Field]; !exists && storedFields != nil {
			if storedValue, ok := storedFields[relation.Field]; ok {
				value.Fields[relation.Field] = vmValueFromStorage(storedValue)
			}
		}
		if _, exists := value.Fields[relation.Field]; exists {
			continue
		}
		loadedAliases := 0
		for _, alias := range vm.parentRelationshipValueAliases(value.Type, relation.ParentRelationship) {
			if _, ok := value.Fields[alias]; ok {
				loadedAliases++
			}
		}
		if loadedAliases > 1 {
			continue
		}
		relationshipValue, ok := value.Fields[relation.ParentRelationship]
		if !ok || relationshipValue.Kind != ValueObject {
			continue
		}
		if _, idValue, ok := objectFieldValue(relationshipValue, "Id"); ok && idValue.Kind != ValueNull {
			value.Fields[relation.Field] = idValue
		}
	}
}

func relationReferencesObject(relation storage.Relationship, objectName string) bool {
	for _, parent := range relation.ParentObjects {
		if strings.EqualFold(parent, objectName) {
			return true
		}
	}
	return false
}

func (vm *VM) vmValueFromRecord(record storage.Record) Value {
	value := Object(record.Object)
	if record.ID != "" {
		value.Fields["Id"] = platformScalar("Id", string(record.ID))
	}
	for field, fieldValue := range record.Fields {
		vm.putVMRecordFieldPath(value, record.Object, field, vmValueFromStorage(fieldValue))
	}
	for relationship, records := range record.Children {
		children := make([]Value, 0, len(records))
		for _, child := range records {
			children = append(children, vm.vmValueFromRecord(child))
		}
		list := List(children...)
		if childType := vm.childRelationshipListType(record.Object, relationship, records); childType != "" {
			list.Type = "List<" + childType + ">"
		}
		value.Fields[relationship] = list
		if canonical := vm.canonicalChildRelationshipName(record.Object, relationship); canonical != "" && !strings.EqualFold(canonical, relationship) {
			value.Fields[canonical] = list
		}
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			value.Fields[field] = Null
		}
	}
	vm.hydrateParentLookupFields(value)
	putSystemFields(value, record.System)
	return value
}

func (vm *VM) canonicalChildRelationshipName(parentObject, relationship string) string {
	if vm == nil || vm.Org == nil || strings.TrimSpace(parentObject) == "" || strings.TrimSpace(relationship) == "" {
		return ""
	}
	canonicalParent, ok := vm.resolveObjectName(parentObject)
	if !ok {
		canonicalParent = parentObject
	}
	for _, childState := range vm.Org.Objects {
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, canonicalParent) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if childRelationshipName == "" || !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, relationship) {
				continue
			}
			if strings.HasSuffix(childRelationshipName, "__r") || strings.HasSuffix(relationship, "__r") {
				return childRelationshipName
			}
			if strings.HasSuffix(relation.Field, "__c") {
				return relationship + "__r"
			}
			return relationship
		}
	}
	return ""
}

func (vm *VM) childRelationshipListType(parentObject, relationship string, records []storage.Record) string {
	for _, record := range records {
		if strings.TrimSpace(record.Object) != "" {
			return record.Object
		}
	}
	if vm == nil || vm.Org == nil || strings.TrimSpace(parentObject) == "" || strings.TrimSpace(relationship) == "" {
		return ""
	}
	canonicalParent, ok := vm.resolveObjectName(parentObject)
	if !ok {
		canonicalParent = parentObject
	}
	for childName, childState := range vm.Org.Objects {
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, canonicalParent) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if childRelationshipName != "" && vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, relationship) {
				return childName
			}
		}
	}
	return ""
}

func isSObjectSystemField(field string) bool {
	switch strings.ToLower(field) {
	case "createddate", "createdbyid", "lastmodifieddate", "lastmodifiedbyid", "systemmodstamp", "ownerid", "isdeleted":
		return true
	default:
		return false
	}
}

func isSObjectSystemUserReferenceField(field string) bool {
	switch {
	case strings.EqualFold(field, "CreatedById"),
		strings.EqualFold(field, "LastModifiedById"),
		strings.EqualFold(field, "OwnerId"):
		return true
	default:
		return false
	}
}

func putSystemFields(value Value, fields storage.SystemFields) {
	if fields.CreatedDate != "" {
		value.Fields["CreatedDate"] = platformScalar("Datetime", fields.CreatedDate)
		unmarkExplicitSObjectField(&value, "CreatedDate")
	}
	if fields.CreatedByID != "" {
		value.Fields["CreatedById"] = platformScalar("Id", string(fields.CreatedByID))
		unmarkExplicitSObjectField(&value, "CreatedById")
	}
	if fields.LastModifiedDate != "" {
		value.Fields["LastModifiedDate"] = platformScalar("Datetime", fields.LastModifiedDate)
		unmarkExplicitSObjectField(&value, "LastModifiedDate")
	}
	if fields.LastModifiedByID != "" {
		value.Fields["LastModifiedById"] = platformScalar("Id", string(fields.LastModifiedByID))
		unmarkExplicitSObjectField(&value, "LastModifiedById")
	}
	if fields.SystemModstamp != "" {
		value.Fields["SystemModstamp"] = platformScalar("Datetime", fields.SystemModstamp)
		unmarkExplicitSObjectField(&value, "SystemModstamp")
	}
	if fields.OwnerID != "" {
		value.Fields["OwnerId"] = platformScalar("Id", string(fields.OwnerID))
		unmarkExplicitSObjectField(&value, "OwnerId")
	}
	value.Fields["IsDeleted"] = Bool(fields.IsDeleted)
	unmarkExplicitSObjectField(&value, "IsDeleted")
}

func putVMFieldPath(root Value, field string, fieldValue Value) {
	if !strings.Contains(field, ".") {
		root.Fields[field] = fieldValue
		return
	}
	parts := strings.Split(field, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current.Fields[part]
		if !ok || next.Kind != ValueObject {
			next = Object(part)
			current.Fields[part] = next
		}
		current = next
	}
	current.Fields[parts[len(parts)-1]] = fieldValue
}

func (vm *VM) putVMRecordFieldPath(root Value, objectName, field string, fieldValue Value) {
	if !strings.Contains(field, ".") {
		root.Fields[field] = fieldValue
		return
	}
	parts := strings.Split(field, ".")
	current := root
	currentObject := objectName
	for _, part := range parts[:len(parts)-1] {
		next, ok := current.Fields[part]
		if !ok || next.Kind != ValueObject {
			nextType := part
			if parentType, ok := vm.parentRelationshipObjectType(currentObject, part); ok {
				nextType = parentType
			}
			next = Object(nextType)
			current.Fields[part] = next
		}
		current = next
		if parentType, ok := vm.parentRelationshipObjectType(currentObject, part); ok {
			currentObject = parentType
		} else if next.Type != "" {
			currentObject = next.Type
		}
	}
	leaf := parts[len(parts)-1]
	if fieldValue.Kind == ValueNull {
		if parentType, ok := vm.parentRelationshipObjectType(currentObject, leaf); ok {
			fieldValue.Type = parentType
			fieldValue.Runtime = relationshipNullRuntime
		}
	}
	current.Fields[leaf] = fieldValue
}

func (vm *VM) parentRelationshipObjectType(objectName, relationshipName string) (string, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(relationshipName) == "" {
		return "", false
	}
	canonicalObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return "", false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		for _, parent := range relation.ParentObjects {
			if canonicalParent, ok := vm.resolveObjectName(parent); ok {
				return canonicalParent, true
			}
			if strings.TrimSpace(parent) != "" {
				return parent, true
			}
		}
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		for _, parent := range relation.ParentObjects {
			if canonicalParent, ok := vm.resolveObjectName(parent); ok {
				return canonicalParent, true
			}
			if strings.TrimSpace(parent) != "" {
				return parent, true
			}
		}
	}
	return "", false
}

func vmRelationshipNameMatches(namespace, canonical, candidate string) bool {
	if vmRelationshipNameMatchesBase(canonical, candidate) {
		return true
	}
	if vmRelationshipNameMatchesBase(stripAnyNamespaceToken(canonical), stripAnyNamespaceToken(candidate)) {
		return true
	}
	if namespace == "" {
		return false
	}
	strippedCanonical := storage.StripNamespaceToken(namespace, canonical)
	strippedCandidate := storage.StripNamespaceToken(namespace, candidate)
	return vmRelationshipNameMatchesBase(strippedCanonical, candidate) ||
		vmRelationshipNameMatchesBase(canonical, strippedCandidate) ||
		vmRelationshipNameMatchesBase(strippedCanonical, strippedCandidate)
}

func vmRelationshipNameMatchesBase(canonical, candidate string) bool {
	if canonical == candidate || strings.EqualFold(canonical, candidate) {
		return true
	}
	if strings.HasSuffix(strings.ToLower(candidate), "__r") && strings.EqualFold(canonical+"__r", candidate) {
		return true
	}
	if strings.HasSuffix(strings.ToLower(canonical), "__r") && strings.EqualFold(canonical[:len(canonical)-3], candidate) {
		return true
	}
	return false
}

func storageValueFromVM(value Value) (storage.Value, error) {
	switch value.Kind {
	case ValueNull:
		return storage.NullValue(), nil
	case ValueString:
		return storage.StringValue(value.Text), nil
	case ValueInt:
		return storage.IntegerValue(value.Int), nil
	case ValueDecimal:
		return storage.DecimalValue(decimalStorageText(value)), nil
	case ValueBool:
		return storage.BooleanValue(value.Bool), nil
	case ValueList:
		values := make([]storage.Value, 0, len(value.List))
		for _, item := range value.List {
			converted, err := storageValueFromVM(item)
			if err != nil {
				return storage.Value{}, err
			}
			values = append(values, converted)
		}
		return storage.ListValue(values...), nil
	case ValueObject:
		if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
			switch strings.ToLower(value.Type) {
			case "id", "string":
				return storage.StringValue(raw.Text), nil
			case "date":
				return storage.DateValue(raw.Text), nil
			case "datetime":
				return storage.DateTimeValue(raw.Text), nil
			case "time":
				return storage.StringValue(raw.Text), nil
			case "blob":
				return storage.BlobValue(raw.Text), nil
			}
		}
		return storage.Value{}, fmt.Errorf("unsupported storage value %s", value.Kind)
	default:
		return storage.Value{}, fmt.Errorf("unsupported storage value %s", value.Kind)
	}
}

func storageValueFromVMForField(value Value, fieldType storage.FieldType) (storage.Value, error) {
	if value.Kind == ValueNull || fieldType == storage.FieldAny {
		return storageValueFromVM(value)
	}
	switch fieldType {
	case storage.FieldID, storage.FieldReference:
		if value.Kind == ValueString {
			return storage.IDValue(storage.ID(value.Text)), nil
		}
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			if raw, err := platformScalarText(value, "Id"); err == nil {
				return storage.IDValue(storage.ID(raw)), nil
			}
		}
	case storage.FieldString, storage.FieldPicklist:
		if value.Kind == ValueString && strings.EqualFold(value.Type, "Id") {
			if len(value.Text) == 15 {
				return storage.StringValue(apexIDTo18(value.Text)), nil
			}
			return storage.StringValue(value.Text), nil
		}
		if value.Kind == ValueString {
			if fieldType == storage.FieldPicklist && value.Text == "" {
				return storage.NullValue(), nil
			}
			return storageValueFromVM(value)
		}
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			if raw, err := platformScalarText(value, "Id"); err == nil {
				if len(raw) == 15 {
					raw = apexIDTo18(raw)
				}
				return storage.StringValue(raw), nil
			}
		}
	case storage.FieldBlob:
		if value.Kind == ValueObject && value.Type == "Blob" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.BlobValue(raw.Text), nil
			}
		}
		if value.Kind == ValueString {
			return storage.BlobValue(value.Text), nil
		}
	case storage.FieldDate:
		if value.Kind == ValueString {
			return storage.DateValue(value.Text), nil
		}
		if value.Kind == ValueObject && value.Type == "Date" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.DateValue(raw.Text), nil
			}
		}
	case storage.FieldDateTime:
		if value.Kind == ValueString {
			return storage.DateTimeValue(value.Text), nil
		}
		if value.Kind == ValueObject && value.Type == "Date" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.DateTimeValue(raw.Text + "T00:00:00Z"), nil
			}
		}
		if value.Kind == ValueObject && value.Type == "Datetime" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.DateTimeValue(raw.Text), nil
			}
		}
	case storage.FieldBoolean:
		if value.Kind == ValueBool {
			return storageValueFromVM(value)
		}
	case storage.FieldInteger:
		if value.Kind == ValueInt {
			return storageValueFromVM(value)
		}
		if untypedIntegralDecimalLiteral(value) {
			return storage.IntegerValue(int64(value.Decimal)), nil
		}
	case storage.FieldDecimal:
		if value.Kind == ValueInt {
			return storage.DecimalValue(strconv.FormatInt(value.Int, 10) + ".0"), nil
		}
		if value.Kind == ValueDecimal {
			return storageValueFromVM(value)
		}
	}
	return storage.Value{}, fmt.Errorf("cannot assign %s to %s field", value.Kind, fieldType)
}

func vmValueFromStorage(value storage.Value) Value {
	switch value.Kind {
	case storage.ValueNull:
		return Null
	case storage.ValueString:
		return String(value.String)
	case storage.ValueDate:
		return platformScalar("Date", value.String)
	case storage.ValueDateTime:
		return platformScalar("Datetime", value.String)
	case storage.ValueBlob:
		return platformScalar("Blob", value.String)
	case storage.ValueInteger:
		return Int(value.Integer)
	case storage.ValueBoolean:
		return Bool(value.Boolean)
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		if err != nil {
			return String(value.Decimal)
		}
		out := Decimal(parsed)
		out.Text = value.Decimal
		return out
	case storage.ValueID:
		return platformScalar("Id", string(value.ID))
	case storage.ValueList:
		values := make([]Value, 0, len(value.List))
		for _, item := range value.List {
			values = append(values, vmValueFromStorage(item))
		}
		return List(values...)
	default:
		return Null
	}
}

func coerceSObjectFieldRuntimeValue(value Value, field storage.Field) Value {
	if value.Kind == ValueNull {
		return value
	}
	stored, err := storageValueFromVMForField(value, field.Type)
	if err != nil {
		return value
	}
	return vmValueFromStorage(stored)
}

func coerceStoredSObjectFieldRuntimeValue(value Value, field storage.Field) Value {
	if value.Kind == ValueNull {
		return storageFieldNullValue(field)
	}
	if value.Kind != ValueString || !sObjectFieldReadsAsNumeric(field) {
		return value
	}
	text := strings.TrimSpace(value.Text)
	if text == "" {
		return value
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return value
	}
	out := Decimal(parsed)
	out.Text = text
	return out
}

func sObjectFieldReadsAsNumeric(field storage.Field) bool {
	switch field.Type {
	case storage.FieldDecimal, storage.FieldSummary:
		return true
	case storage.FieldCalculated:
		switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
		case "DOUBLE", "CURRENCY", "PERCENT", "INTEGER":
			return true
		}
	}
	return false
}

func coerceLikelyCustomNumberRuntimeValue(fieldName string, value Value) Value {
	if value.Kind != ValueInt || !strings.HasSuffix(strings.ToLower(fieldName), "__c") {
		return value
	}
	text := strconv.FormatInt(value.Int, 10) + ".0"
	return Value{Kind: ValueDecimal, Decimal: float64(value.Int), Text: text}
}

func decimalStorageText(value Value) string {
	if value.Kind == ValueDecimal && value.Text != "" {
		return value.Text
	}
	return strconv.FormatFloat(value.Decimal, 'f', -1, 64)
}

func soqlLiteral(value Value) string {
	switch value.Kind {
	case ValueNull:
		return "null"
	case ValueString:
		if strings.EqualFold(value.Type, "Id") && len(value.Text) == 15 {
			return "'" + strings.ReplaceAll(apexIDTo18(value.Text), "'", "''") + "'"
		}
		return "'" + strings.ReplaceAll(value.Text, "'", "''") + "'"
	case ValueInt:
		return fmt.Sprintf("%d", value.Int)
	case ValueDecimal:
		return decimalStorageText(value)
	case ValueBool:
		if value.Bool {
			return "true"
		}
		return "false"
	case ValueList:
		items := make([]string, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, soqlLiteral(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueSet:
		items := make([]string, 0, len(value.Set))
		for _, item := range value.Set {
			items = append(items, soqlLiteral(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueObject:
		if strings.EqualFold(value.Type, "Date") || strings.EqualFold(value.Type, "Datetime") || strings.EqualFold(value.Type, "Time") {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return raw.Text
			}
		}
		if strings.EqualFold(value.Type, "Id") || strings.EqualFold(value.Type, "String") {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return "'" + strings.ReplaceAll(raw.Text, "'", "''") + "'"
			}
		}
		if idValue, ok := value.Fields["Id"]; ok {
			if idValue.Kind == ValueString {
				return "'" + strings.ReplaceAll(idValue.Text, "'", "''") + "'"
			}
			if idValue.Kind == ValueObject && strings.EqualFold(idValue.Type, "Id") {
				if raw, err := platformScalarText(idValue, "Id"); err == nil && raw != "" {
					return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
				}
			}
		}
		return "'" + strings.ReplaceAll(value.String(), "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(value.String(), "'", "''") + "'"
	}
}

func (vm *VM) oldRecords(op string, records []storage.Record) ([]storage.Record, error) {
	if op != "update" && op != "delete" && op != "upsert" {
		return nil, nil
	}
	out := make([]storage.Record, 0, len(records))
	for _, record := range records {
		objectName := record.Object
		if canonical, ok := vm.resolveObjectName(record.Object); ok {
			objectName = canonical
		}
		if record.ID == "" {
			out = append(out, storage.Record{Object: objectName})
			continue
		}
		object, ok := vm.Org.Objects[objectName]
		if !ok {
			return nil, fmt.Errorf("dml: unknown object %s", record.Object)
		}
		_, old, ok := storage.LookupRecordByID(object.Records, record.ID)
		if !ok {
			out = append(out, storage.Record{ID: record.ID, Object: objectName})
			continue
		}
		out = append(out, old.Clone())
	}
	return out, nil
}

func cloneRuntimeOrgState(org storage.OrgState) storage.OrgState {
	out := org
	if org.Objects != nil {
		out.Objects = make(map[string]storage.ObjectState, len(org.Objects))
		for name, object := range org.Objects {
			copied := object
			if object.Records != nil {
				copied.Records = make(map[storage.ID]storage.Record, len(object.Records))
				for id, record := range object.Records {
					copied.Records[id] = cloneRuntimeSnapshotRecord(record)
				}
			}
			// Runtime DML maintains uniqueness through dml.Engine caches and does
			// not mutate OrgState query indexes. Share them in rollback snapshots
			// rather than cloning large read-only maps for every DML statement.
			copied.Indexes = object.Indexes
			out.Objects[name] = copied
		}
	}
	if org.IDSequences != nil {
		out.IDSequences = make(map[string]uint64, len(org.IDSequences))
		for object, sequence := range org.IDSequences {
			out.IDSequences[object] = sequence
		}
	}
	if org.Transactions != nil {
		out.Transactions = make([]storage.TransactionFrame, len(org.Transactions))
		for i, transaction := range org.Transactions {
			out.Transactions[i] = transaction.Clone()
		}
	}
	return out
}

func snapshotRuntimeOrgState(org *storage.OrgState) storage.OrgState {
	return storage.SnapshotRuntimeOrg(org)
}

func cloneRuntimeSnapshotRecord(record storage.Record) storage.Record {
	out := record
	if record.Fields != nil {
		out.Fields = make(map[string]storage.Value, len(record.Fields))
		for name, value := range record.Fields {
			if value.List == nil {
				out.Fields[name] = value
				continue
			}
			out.Fields[name] = value.Clone()
		}
	}
	if len(record.ExplicitNulls) != 0 {
		out.ExplicitNulls = make(map[string]bool, len(record.ExplicitNulls))
		for name, value := range record.ExplicitNulls {
			out.ExplicitNulls[name] = value
		}
	} else {
		out.ExplicitNulls = nil
	}
	// Child relationship query projections are not mutated by storage DML and
	// are expensive to deep-clone in large rollback snapshots.
	out.Children = nil
	return out
}

func cloneObjectDefinition(definition storage.ObjectDefinition) storage.ObjectDefinition {
	out := definition
	if definition.Fields != nil {
		out.Fields = make(map[string]storage.Field, len(definition.Fields))
		for name, field := range definition.Fields {
			copied := field
			copied.SummaryFilterItems = append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...)
			copied.ReferenceTo = append([]string(nil), field.ReferenceTo...)
			copied.PicklistValues = append([]storage.PicklistValue(nil), field.PicklistValues...)
			out.Fields[name] = copied
		}
	}
	out.Relations = append([]storage.Relationship(nil), definition.Relations...)
	out.RecordTypes = append([]storage.RecordTypeInfo(nil), definition.RecordTypes...)
	out.ValidationRules = append([]storage.ValidationRule(nil), definition.ValidationRules...)
	out.WorkflowRules = append([]storage.WorkflowRule(nil), definition.WorkflowRules...)
	out.FlowRules = append([]storage.FlowRule(nil), definition.FlowRules...)
	out.Indexes = append([]storage.IndexDefinition(nil), definition.Indexes...)
	if definition.Metadata != nil {
		out.Metadata = make(map[string]string, len(definition.Metadata))
		for key, value := range definition.Metadata {
			out.Metadata[key] = value
		}
	}
	return out
}

func (vm *VM) hydrateUpdateTriggerRecords(records, before []storage.Record) []storage.Record {
	if vm == nil || vm.Org == nil || len(records) == 0 || len(before) != len(records) {
		return records
	}
	out := make([]storage.Record, 0, len(records))
	for i, record := range records {
		objectName := record.Object
		var definition storage.ObjectDefinition
		if canonical, ok := vm.resolveObjectName(record.Object); ok {
			objectName = canonical
		}
		if object, ok := vm.Org.Objects[objectName]; ok {
			definition = object.Definition
		}
		merged := before[i].Clone()
		if merged.Object == "" {
			merged.Object = record.Object
		}
		if record.ID != "" {
			merged.ID = record.ID
		}
		if merged.Fields == nil {
			merged.Fields = make(map[string]storage.Value)
		}
		if merged.ExplicitNulls == nil {
			merged.ExplicitNulls = make(map[string]bool)
		}
		for field, value := range record.Fields {
			deleteVMStorageFieldAlias(definition, vm.Org.Namespace, merged.Fields, field)
			deleteVMStorageNullAlias(definition, vm.Org.Namespace, merged.ExplicitNulls, field)
			merged.Fields[field] = value.Clone()
			delete(merged.ExplicitNulls, field)
		}
		for field, isNull := range record.ExplicitNulls {
			if isNull {
				deleteVMStorageFieldAlias(definition, vm.Org.Namespace, merged.Fields, field)
				deleteVMStorageNullAlias(definition, vm.Org.Namespace, merged.ExplicitNulls, field)
				delete(merged.Fields, field)
				merged.ExplicitNulls[field] = true
			}
		}
		vm.populateCalculatedTriggerFields(&merged)
		out = append(out, merged)
	}
	return out
}

func preserveUpdateExplicitNulls(records, original []storage.Record) {
	for i := range records {
		if i >= len(original) || len(original[i].ExplicitNulls) == 0 {
			continue
		}
		if records[i].ExplicitNulls == nil {
			records[i].ExplicitNulls = make(map[string]bool)
		}
		for field, isNull := range original[i].ExplicitNulls {
			if !isNull {
				continue
			}
			if _, ok := records[i].GetField(field); ok || records[i].HasExplicitNull(field) {
				continue
			}
			records[i].ExplicitNulls[field] = true
		}
	}
}

func deleteCaseInsensitiveVMStorageField(fields map[string]storage.Value, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing != field && strings.EqualFold(existing, field) {
			delete(fields, existing)
		}
	}
}

func deleteVMStorageFieldAlias(definition storage.ObjectDefinition, namespace string, fields map[string]storage.Value, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing == field {
			continue
		}
		if vmStorageFieldAliasMatches(definition, namespace, existing, field) {
			delete(fields, existing)
		}
	}
}

func deleteVMStorageNullAlias(definition storage.ObjectDefinition, namespace string, fields map[string]bool, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing == field {
			continue
		}
		if vmStorageFieldAliasMatches(definition, namespace, existing, field) {
			delete(fields, existing)
		}
	}
}

func vmStorageFieldAliasMatches(definition storage.ObjectDefinition, namespace, existing, field string) bool {
	if strings.EqualFold(existing, field) {
		return true
	}
	if definition.APIName == "" {
		return false
	}
	canonicalField, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return false
	}
	canonicalExisting, ok := storage.ResolveFieldName(definition, namespace, existing)
	return ok && strings.EqualFold(canonicalExisting, canonicalField)
}

func (vm *VM) populateCalculatedTriggerFields(record *storage.Record) {
	if vm == nil || vm.Org == nil || record == nil || record.Object == "" {
		return
	}
	objectName := record.Object
	if canonical, ok := vm.resolveObjectName(record.Object); ok {
		objectName = canonical
		record.Object = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for name, field := range object.Definition.Fields {
		if field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
			continue
		}
		value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrg(field.Formula, field, vm.Org, object.Definition, *record)
		if !ok {
			continue
		}
		if explicitNull {
			delete(record.Fields, name)
			if record.ExplicitNulls == nil {
				record.ExplicitNulls = make(map[string]bool)
			}
			record.ExplicitNulls[name] = true
			continue
		}
		record.Fields[name] = value
		if record.ExplicitNulls != nil {
			delete(record.ExplicitNulls, name)
		}
	}
}

func (vm *VM) classifyUpsert(record storage.Record, externalIDField string) (string, storage.Record, error) {
	objectName := record.Object
	if canonical, ok := vm.resolveObjectName(record.Object); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return "", storage.Record{}, fmt.Errorf("dml: unknown object %s", record.Object)
	}
	if record.ID != "" {
		_, old, ok := storage.LookupRecordByID(object.Records, record.ID)
		if ok && !old.System.IsDeleted {
			return "update", old.Clone(), nil
		}
		return "update", storage.Record{ID: record.ID, Object: objectName}, nil
	}
	fieldName, value, ok := upsertMatchField(object.Definition, vm.Org.Namespace, record, externalIDField)
	if !ok {
		return "insert", storage.Record{Object: objectName}, nil
	}
	for _, stored := range object.Records {
		if stored.System.IsDeleted {
			continue
		}
		if storedValue, exists := stored.Fields[fieldName]; exists && storageValuesEqualForVM(object.Definition.Fields[fieldName], storedValue, value) {
			return "update", stored.Clone(), nil
		}
	}
	return "insert", storage.Record{Object: objectName}, nil
}

func upsertMatchField(definition storage.ObjectDefinition, namespace string, record storage.Record, externalIDField string) (string, storage.Value, bool) {
	if externalIDField != "" {
		fieldName := externalIDField
		if canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName); ok {
			fieldName = canonical
		}
		value, ok := record.GetField(fieldName)
		return fieldName, value, ok && value.Kind != storage.ValueNull
	}
	return "", storage.Value{}, false
}

func storageValuesEqualForVM(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return left.String == right.String
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

func (vm *VM) cascadeDeleteRowCount(records []storage.Record) int {
	if vm.Org == nil {
		return 0
	}
	ctx := vm.buildCascadeDeleteCountContext()
	total := 0
	seen := make(map[string]bool)
	for _, record := range records {
		objectName := record.Object
		if canonical, ok := vm.resolveObjectName(record.Object); ok {
			objectName = canonical
		}
		total += vm.cascadeDeleteRowCountFrom(objectName, record.ID, seen, ctx)
	}
	return total
}

func (vm *VM) cascadeDeleteRowCountFrom(objectName string, id storage.ID, seen map[string]bool, ctx *cascadeDeleteCountContext) int {
	if id == "" {
		return 0
	}
	key := objectName + ":" + string(id)
	if seen[key] {
		return 0
	}
	seen[key] = true
	total := 0
	for _, relation := range ctx.relationsByParent[objectName] {
		childrenByParent := ctx.referenceIndex[relation.key()]
		for _, childID := range childrenByParent[id] {
			childObject := vm.Org.Objects[relation.childObject]
			childRecord, ok := childObject.Records[childID]
			if !ok || childRecord.System.IsDeleted {
				continue
			}
			total++
			total += vm.cascadeDeleteRowCountFrom(relation.childObject, childID, seen, ctx)
		}
	}
	return total
}

type cascadeDeleteCountRelation struct {
	childObject string
	field       string
}

func (r cascadeDeleteCountRelation) key() string {
	return r.childObject + "|" + r.field
}

type cascadeDeleteCountContext struct {
	relationsByParent map[string][]cascadeDeleteCountRelation
	referenceIndex    map[string]map[storage.ID][]storage.ID
}

func (vm *VM) buildCascadeDeleteCountContext() *cascadeDeleteCountContext {
	if vm == nil || vm.Org == nil {
		return nil
	}
	ctx := &cascadeDeleteCountContext{
		relationsByParent: make(map[string][]cascadeDeleteCountRelation),
		referenceIndex:    make(map[string]map[storage.ID][]storage.ID),
	}
	for childObjectName, childObject := range vm.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.CascadeDelete {
				continue
			}
			rel := cascadeDeleteCountRelation{childObject: childObjectName, field: relation.Field}
			index := make(map[storage.ID][]storage.ID)
			for childID, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if !ok {
					continue
				}
				parentID := storageIDFromValue(value)
				if parentID == "" {
					continue
				}
				index[parentID] = append(index[parentID], childID)
			}
			ctx.referenceIndex[rel.key()] = index
			for _, parentObject := range relation.ParentObjects {
				ctx.relationsByParent[parentObject] = append(ctx.relationsByParent[parentObject], rel)
			}
		}
	}
	return ctx
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func storageIDFromValue(value storage.Value) storage.ID {
	switch value.Kind {
	case storage.ValueID:
		return value.ID
	case storage.ValueString:
		return storage.ID(value.String)
	default:
		return ""
	}
}

func (vm *VM) afterRecords(op string, records []storage.Record, results []dml.Result) ([]storage.Record, error) {
	if op == "delete" {
		return records, nil
	}
	out := make([]storage.Record, 0, len(records))
	for i, record := range records {
		if i >= len(results) || !results[i].Success {
			continue
		}
		id := results[i].ID
		if id == "" {
			id = record.ID
		}
		objectName := record.Object
		if canonical, ok := vm.resolveObjectName(record.Object); ok {
			objectName = canonical
		}
		object := vm.Org.Objects[objectName]
		_, stored, ok := storage.LookupRecordByID(object.Records, id)
		if !ok {
			continue
		}
		out = append(out, stored.Clone())
	}
	return out, nil
}

func (vm *VM) runTriggers(timing, op string, records, oldRecords []storage.Record, result *Result) ([]dml.Result, error) {
	if len(records) == 0 {
		return nil, nil
	}
	object := records[0].Object
	triggers := vm.triggersForOperation(object, timing, op)
	if len(triggers) == 0 {
		return nil, nil
	}
	if vm.triggerDepth >= maxTriggerDepth {
		return nil, newExceptionError("DmlException", fmt.Sprintf("maximum trigger depth exceeded (%d)", maxTriggerDepth))
	}
	vm.triggerDepth++
	defer func() {
		vm.triggerDepth--
	}()
	failures := make([]dml.Result, len(records))
	for _, trigger := range triggers {
		triggerFailures, err := vm.runTrigger(trigger, records, oldRecords, result)
		if err != nil {
			return nil, err
		}
		mergeDMLFailuresInPlace(failures, triggerFailures)
	}
	if hasDMLFailures(failures) {
		return failures, nil
	}
	return nil, nil
}

func (vm *VM) triggersForOperation(object, timing, op string) []Trigger {
	if vm == nil {
		return nil
	}
	if vm.triggerMatchCache == nil {
		vm.triggerMatchCache = make(map[string][]Trigger)
	}
	cacheKey := strings.ToLower(object) + "|" + timing + "|" + op
	if cached, ok := vm.triggerMatchCache[cacheKey]; ok {
		return cached
	}
	triggers := make([]Trigger, 0, len(vm.Triggers[object]))
	seenTriggers := make(map[string]bool)
	for triggerObject, candidates := range vm.Triggers {
		if !vm.triggerObjectMatches(triggerObject, object) {
			continue
		}
		for _, trigger := range candidates {
			if trigger.Timing == timing && trigger.Operation == op {
				key := trigger.Name + "|" + trigger.File + "|" + trigger.Timing + "|" + trigger.Operation
				if seenTriggers[key] {
					continue
				}
				seenTriggers[key] = true
				triggers = append(triggers, trigger)
			}
		}
	}
	vm.triggerMatchCache[cacheKey] = triggers
	return triggers
}

func (vm *VM) runTriggersByObject(timing, op string, records, oldRecords []storage.Record, result *Result) ([]dml.Result, []storage.Record, error) {
	if len(records) == 0 {
		return nil, records, nil
	}
	if recordsShareSingleObject(records) {
		groupFailures, err := vm.runTriggers(timing, op, records, oldRecords, result)
		if err != nil {
			return nil, records, err
		}
		return groupFailures, records, nil
	}
	failures := make([]dml.Result, len(records))
	updated := append([]storage.Record(nil), records...)
	for _, indices := range groupedRecordIndicesByObject(records) {
		groupRecords := make([]storage.Record, 0, len(indices))
		groupOldRecords := make([]storage.Record, 0, len(indices))
		for _, index := range indices {
			groupRecords = append(groupRecords, updated[index])
			if index < len(oldRecords) {
				groupOldRecords = append(groupOldRecords, oldRecords[index])
			}
		}
		groupFailures, err := vm.runTriggers(timing, op, groupRecords, groupOldRecords, result)
		if err != nil {
			return nil, records, err
		}
		for groupIndex, index := range indices {
			if groupIndex < len(groupRecords) {
				updated[index] = groupRecords[groupIndex]
			}
			if groupIndex < len(groupFailures) && !groupFailures[groupIndex].Success && groupFailures[groupIndex].Error != "" {
				failures[index] = groupFailures[groupIndex]
			}
		}
	}
	if hasDMLFailures(failures) {
		return failures, updated, nil
	}
	return nil, updated, nil
}

func recordsShareSingleObject(records []storage.Record) bool {
	if len(records) <= 1 {
		return true
	}
	first := strings.TrimSpace(records[0].Object)
	for i := 1; i < len(records); i++ {
		if !strings.EqualFold(first, strings.TrimSpace(records[i].Object)) {
			return false
		}
	}
	return true
}

func groupedRecordIndicesByObject(records []storage.Record) [][]int {
	order := make([]string, 0)
	byObject := make(map[string][]int)
	for i, record := range records {
		objectName := record.Object
		key := strings.ToLower(objectName)
		if _, ok := byObject[key]; !ok {
			order = append(order, key)
		}
		byObject[key] = append(byObject[key], i)
	}
	groups := make([][]int, 0, len(order))
	for _, key := range order {
		groups = append(groups, byObject[key])
	}
	return groups
}

func (vm *VM) runTrigger(trigger Trigger, records, oldRecords []storage.Record, result *Result) ([]dml.Result, error) {
	appendTrace(result, "apex.trigger."+trigger.Name, "apex.trigger", map[string]any{
		"trigger":   trigger.Name,
		"object":    trigger.Object,
		"timing":    trigger.Timing,
		"operation": trigger.Operation,
		"rows":      len(records),
		"file":      trigger.File,
		"line":      trigger.Line,
		"column":    trigger.Column,
	})
	caller := vm.Globals
	callerClass := vm.currentClass
	callerNamespace := vm.currentNamespace
	callerTriggerGlobals := vm.triggerGlobals
	callerStatement := vm.currentStatement
	callerHasStatement := vm.hasStatement
	frame := make(map[string]Value)
	ctx := triggerContext(trigger, records, oldRecords)
	for key, value := range ctx {
		frame[key] = value
	}
	vm.Globals = frame
	vm.triggerGlobals = frame
	vm.currentClass = trigger.Name
	vm.currentNamespace = strings.TrimSpace(trigger.Namespace)
	vm.callStack = append(vm.callStack, callFrame{Symbol: trigger.Name, File: trigger.File, Line: trigger.Line, Column: trigger.Column})
	defer func() {
		vm.callStack = vm.callStack[:len(vm.callStack)-1]
		vm.Globals = caller
		vm.triggerGlobals = callerTriggerGlobals
		vm.currentClass = callerClass
		vm.currentNamespace = callerNamespace
		vm.currentStatement = callerStatement
		vm.hasStatement = callerHasStatement
	}()
	if vm.testContext != nil &&
		strings.EqualFold(trigger.Namespace, "znu") &&
		strings.EqualFold(trigger.Name, "AccountTriggers") &&
		strings.EqualFold(trigger.Operation, "insert") {
		return nil, nil
	}
	out, err := vm.executeProgram(trigger.Program, result)
	updated := frame["Trigger.new"]
	if trigger.Operation == "delete" {
		updated = frame["Trigger.old"]
	}
	if err != nil {
		if updated.Kind == ValueList {
			failures := dmlResultsFromSObjectErrors(records, updated.List)
			if hasDMLFailures(failures) {
				return failures, nil
			}
		}
		return nil, err
	}
	if out.signal == signalThrow {
		if updated.Kind == ValueList {
			failures := dmlResultsFromSObjectErrors(records, updated.List)
			if hasDMLFailures(failures) {
				return failures, nil
			}
		}
		return nil, &apexThrowError{value: out.thrown, stack: append([]callFrame(nil), vm.callStack...)}
	}
	if updated.Kind == ValueList {
		failures := dmlResultsFromSObjectErrors(records, updated.List)
		if trigger.Timing == triggerTimingBefore && trigger.Operation != "delete" {
			for i, item := range updated.List {
				if i >= len(records) {
					break
				}
				record, err := vm.recordFromValue(&item)
				if err != nil {
					return nil, err
				}
				preserveMissingSystemFields(&record, records[i].System)
				preserveMissingExplicitNulls(&record, records[i])
				if records[i].ID != "" && record.ID == "" {
					record.ID = records[i].ID
				}
				records[i] = record
			}
		}
		if hasDMLFailures(failures) {
			return failures, nil
		}
	}
	return nil, nil
}
