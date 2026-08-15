package vm

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) databaseGetUpdated(args []Value) (Value, error) {
	start, end, err := databaseSyncWindow(args[1], args[2], "Database.getUpdated")
	if err != nil {
		return Null, err
	}
	ids := make([]string, 0)
	if object, ok := vm.databaseSyncObject(args[0].Text); ok {
		for id, record := range object.Records {
			if record.System.IsDeleted {
				continue
			}
			stamp, ok := databaseSyncRecordStamp(record)
			if !ok || !databaseSyncStampInWindow(stamp, start, end) {
				continue
			}
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	values := make([]Value, 0, len(ids))
	for _, id := range ids {
		values = append(values, platformScalar("Id", id))
	}
	updated := Object("Database.GetUpdatedResult")
	updated.Fields["ids"] = List(values...)
	updated.Fields["latestDateCovered"] = platformScalar("Date", formatPlatformDate(end))
	return updated, nil
}

func (vm *VM) databaseGetDeleted(args []Value) (Value, error) {
	start, end, err := databaseSyncWindow(args[1], args[2], "Database.getDeleted")
	if err != nil {
		return Null, err
	}
	records := make([]Value, 0)
	if object, ok := vm.databaseSyncObject(args[0].Text); ok {
		ids := make([]string, 0)
		byID := make(map[string]storage.Record)
		for id, record := range object.Records {
			if !record.System.IsDeleted {
				continue
			}
			stamp, ok := databaseSyncRecordStamp(record)
			if !ok || !databaseSyncStampInWindow(stamp, start, end) {
				continue
			}
			rawID := string(id)
			ids = append(ids, rawID)
			byID[rawID] = record
		}
		sort.Strings(ids)
		for _, id := range ids {
			record := byID[id]
			stamp, _ := databaseSyncRecordStamp(record)
			deleted := Object("Database.DeletedRecord")
			deleted.Fields["id"] = platformScalar("Id", id)
			deleted.Fields["deletedDate"] = platformScalar("Date", formatPlatformDate(stamp))
			records = append(records, deleted)
		}
	}
	deleted := Object("Database.GetDeletedResult")
	deleted.Fields["deletedRecords"] = List(records...)
	deleted.Fields["earliestDateAvailable"] = platformScalar("Date", formatPlatformDate(start))
	deleted.Fields["latestDateCovered"] = platformScalar("Date", formatPlatformDate(end))
	return deleted, nil
}

func (vm *VM) databaseSyncObject(name string) (storage.ObjectState, bool) {
	if vm.Org == nil {
		return storage.ObjectState{}, false
	}
	objectName := strings.TrimSpace(name)
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	return object, ok
}

func databaseSyncWindow(startValue, endValue Value, label string) (time.Time, time.Time, error) {
	start, err := parsePlatformDatetime(startValue)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s expects start Datetime: %w", label, err)
	}
	end, err := parsePlatformDatetime(endValue)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s expects end Datetime: %w", label, err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("%s end Datetime must not be before start Datetime", label)
	}
	return start, end, nil
}

func databaseSyncRecordStamp(record storage.Record) (time.Time, bool) {
	stamp := strings.TrimSpace(record.System.SystemModstamp)
	if stamp == "" {
		stamp = strings.TrimSpace(record.System.LastModifiedDate)
	}
	if stamp == "" {
		return time.Time{}, false
	}
	parsed, err := parsePlatformDatetimeText(stamp)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func databaseSyncStampInWindow(stamp, start, end time.Time) bool {
	return !stamp.Before(start) && !stamp.After(end)
}

func (vm *VM) executeDatabaseDML(op string, args []Value, result *Result) (Value, error) {
	traceStart, traceStartedAt := traceSpanStart(result)
	if len(args) == 0 || len(args) > 4 {
		return Null, fmt.Errorf("Database.%s expects records, optional external id field, and optional allOrNone", op)
	}
	allOrNone := true
	externalIDField := ""
	dmlMode := vm.defaultAccessLevelMode()
	accessLevel := Value{}
	dmlOptions := dml.Options{}
	if len(args) >= 2 {
		if args[1].Kind == ValueBool {
			allOrNone = args[1].Bool
		} else if isDatabaseDMLOptionsValue(args[1]) {
			allOrNone = databaseDMLOptionsAllOrNone(args[1], allOrNone)
			var optionsErr error
			dmlOptions, optionsErr = databaseDMLOptions(args[1])
			if optionsErr != nil {
				return Null, optionsErr
			}
		} else if isDatabaseAccessLevelValue(args[1]) {
			dmlMode = databaseAccessLevelSecurityMode(args[1])
			accessLevel = args[1]
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
			dmlMode = databaseAccessLevelSecurityMode(args[2])
			accessLevel = args[2]
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
		dmlMode = databaseAccessLevelSecurityMode(args[3])
		accessLevel = args[3]
	}
	if op == "delete" || op == "undelete" {
		records, ok := vm.deleteIDsToSObjects(args[0])
		if ok {
			args[0] = records
		}
	}
	if dmlMode == "USER_MODE" {
		if err := vm.enforceUserModeDMLAccess(op, args[0], accessLevel); err != nil {
			return Null, err
		}
	}
	if err := vm.enforceDMLRecordAccess(op, args[0], externalIDField, dmlMode == "USER_MODE"); err != nil {
		return Null, err
	}
	var traceRecords []storage.Record
	if traceIsEnabled(result) {
		var recordsErr error
		traceRecords, _, recordsErr = vm.recordsFromValueForTrace(args[0])
		if recordsErr != nil {
			return Null, recordsErr
		}
	} else if vm.Org == nil {
		if _, _, recordsErr := vm.recordsFromValue(args[0]); recordsErr != nil {
			return Null, recordsErr
		}
	}
	results, err := vm.applyDML(op, args[0], allOrNone, externalIDField, dmlOptions, result)
	appendDurationTraceLazy(result, "apex.dml."+op, "apex.dml", traceStart, traceDurationSince(traceStartedAt), func() map[string]any {
		return vm.traceDMLArgs(op, traceRecords, len(traceRecords))
	})
	if err != nil {
		return Null, err
	}
	if allOrNone && hasDMLFailures(results) {
		vm.addVisualforceDMLPageMessages(results)
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
		row.Fields["errors"] = databaseErrorsList(dmlResult, resultType)
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

func (vm *VM) executeDatabaseAsyncDML(op string, args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 3 {
		return Null, fmt.Errorf("Database.%sAsync expects records, optional callback or AccessLevel", op)
	}
	callback := Null
	dmlArgs := []Value{args[0]}
	if len(args) >= 2 {
		if isDatabaseAllowCalloutsValue(args[1]) {
			return Null, unsupportedCallError("Database." + op + "Async AllowCallouts overload local async callout surface")
		}
		if isAsyncDMLCallbackCandidate(args[1]) {
			if err := vm.validateAsyncDMLCallbackValue(op, args[1]); err != nil {
				return Null, err
			}
			callback = args[1]
			if len(args) == 3 {
				if !isDatabaseAccessLevelValue(args[2]) {
					return Null, fmt.Errorf("Database.%sAsync callback overload expects AccessLevel", op)
				}
				dmlArgs = append(dmlArgs, args[2])
			}
		} else {
			dmlArgs = append(dmlArgs, args[1:]...)
		}
	}
	value, err := vm.executeDatabaseDML(op, dmlArgs, result)
	if err != nil {
		return Null, err
	}
	if callback.Kind != ValueNull {
		if err := vm.invokeAsyncDMLCallback(op, callback, value, result); err != nil {
			return Null, err
		}
	}
	return value, nil
}

func isDatabaseAllowCalloutsValue(value Value) bool {
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Database.AllowCallouts") || strings.EqualFold(value.Type, "AllowCallouts"))
}

func isAsyncDMLCallbackCandidate(value Value) bool {
	return value.Kind == ValueObject && !isDatabaseAccessLevelValue(value) && !isDatabaseDMLOptionsValue(value)
}

func (vm *VM) validateAsyncDMLCallbackValue(op string, value Value) error {
	callbackType := asyncDMLCallbackType(op)
	if strings.EqualFold(value.Type, callbackType) || vm.typeAssignableTo(value.Type, callbackType) {
		_, err := vm.asyncDMLCallbackMethod(op, value)
		return err
	}
	return fmt.Errorf("Database.%sAsync callback overload expects %s", op, callbackType)
}

func (vm *VM) asyncDMLCallbackMethod(op string, callback Value) (Method, error) {
	methodName := asyncDMLCallbackMethodName(op)
	arg := Object(asyncDMLCallbackResultType(op))
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(callback.Type, methodName, []Value{arg})
	if ambiguous {
		return Method{}, vm.ambiguousOverloadError(callback.Type+"."+methodName, []Value{arg})
	}
	if !ok {
		return Method{}, fmt.Errorf("async DML callback %s has no %s method", callback.Type, methodName)
	}
	return method, nil
}

func asyncDMLCallbackMethodName(op string) string {
	if op == "delete" {
		return "processDelete"
	}
	return "processSave"
}

func asyncDMLCallbackResultType(op string) string {
	if op == "delete" {
		return "Database.DeleteResult"
	}
	return "Database.SaveResult"
}

func asyncDMLCallbackType(op string) string {
	if op == "delete" {
		return "DataSource.AsyncDeleteCallback"
	}
	return "DataSource.AsyncSaveCallback"
}

func (vm *VM) invokeAsyncDMLCallback(op string, callback Value, value Value, result *Result) error {
	method, err := vm.asyncDMLCallbackMethod(op, callback)
	if err != nil {
		return err
	}
	values := []Value{value}
	if value.Kind == ValueList {
		values = value.List
	}
	for _, item := range values {
		if _, err := vm.callMethodWithReceiver(method, callback, []Value{item}, result); err != nil {
			return err
		}
	}
	return nil
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

func apexMemberKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
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

func newDatabaseDMLOptions() Value {
	options := Object("Database.DMLOptions")
	options.Fields["allowFieldTruncation"] = Null
	options.Fields["AllowFieldTruncation"] = options.Fields["allowFieldTruncation"]
	options.Fields["assignmentRuleHeader"] = newDatabaseHeaderObject("Database.AssignmentRuleHeader")
	options.Fields["AssignmentRuleHeader"] = options.Fields["assignmentRuleHeader"]
	options.Fields["duplicateRuleHeader"] = newDatabaseHeaderObject("Database.DuplicateRuleHeader")
	options.Fields["DuplicateRuleHeader"] = options.Fields["duplicateRuleHeader"]
	options.Fields["emailHeader"] = newDatabaseHeaderObject("Database.EmailHeader")
	options.Fields["EmailHeader"] = options.Fields["emailHeader"]
	options.Fields["localeOptions"] = Null
	options.Fields["LocaleOptions"] = Null
	options.Fields["localizeErrors"] = Null
	options.Fields["LocalizeErrors"] = options.Fields["localizeErrors"]
	options.Fields["optAllOrNone"] = Null
	options.Fields["OptAllOrNone"] = Null
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

func databaseDMLOptions(value Value) (dml.Options, error) {
	for _, field := range []struct {
		name       string
		inspectMap bool
	}{
		{name: "AssignmentRuleHeader", inspectMap: true},
		{name: "DuplicateRuleHeader", inspectMap: true},
		{name: "EmailHeader", inspectMap: true},
		{name: "LocaleOptions"},
		{name: "LocalizeErrors"},
	} {
		if databaseDMLOptionConfigured(value, field.name, field.inspectMap) {
			return dml.Options{}, unsupportedCallError("Database.DMLOptions." + field.name + " local DML option behavior")
		}
	}
	options := dml.Options{}
	for _, field := range []string{"allowFieldTruncation", "AllowFieldTruncation"} {
		if option, ok := value.Fields[field]; ok && option.Kind == ValueBool && option.Bool {
			options.AllowFieldTruncation = true
			break
		}
	}
	return options, nil
}

func databaseDMLOptionConfigured(value Value, fieldName string, inspectMap bool) bool {
	for field, option := range value.Fields {
		if !strings.EqualFold(field, fieldName) || option.Kind == ValueNull {
			continue
		}
		if !inspectMap || option.Kind != ValueObject {
			return true
		}
		for _, nested := range option.Fields {
			if nested.Kind != ValueNull {
				return true
			}
		}
	}
	return false
}

func (vm *VM) applyPerRecordDMLTargetOptions(records []storage.Record, targets []*Value) error {
	if vm == nil || vm.Org == nil || len(records) == 0 || len(targets) == 0 {
		return nil
	}
	for i := range records {
		if i >= len(targets) || targets[i] == nil || targets[i].Kind != ValueObject {
			continue
		}
		value, ok := targets[i].Fields[sobjectDMLOptionsField]
		if !ok || !isDatabaseDMLOptionsValue(value) {
			continue
		}
		options, err := databaseDMLOptions(value)
		if err != nil {
			return err
		}
		if !options.AllowFieldTruncation {
			continue
		}
		vm.applyRecordFieldTruncation(&records[i])
	}
	return nil
}

func (vm *VM) applyRecordFieldTruncation(record *storage.Record) {
	if vm == nil || vm.Org == nil || record == nil {
		return
	}
	state, ok := vm.objectState(record.Object)
	if !ok {
		return
	}
	definition := state.Definition
	for fieldName, value := range record.Fields {
		if value.Kind != storage.ValueString {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
		if !ok {
			continue
		}
		field := definition.Fields[canonical]
		if field.Length <= 0 || !vmSingleLineTextField(field) {
			continue
		}
		runes := []rune(value.String)
		if len(runes) <= field.Length {
			continue
		}
		value.String = string(runes[:field.Length])
		record.Fields[fieldName] = value
	}
}

func vmSingleLineTextField(field storage.Field) bool {
	if field.Type != storage.FieldString && field.Type != storage.FieldPicklist {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "TEXTAREA", "LONGTEXTAREA", "RICHTEXTAREA":
		return false
	default:
		return true
	}
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
		row.Fields["errors"] = databaseErrorsList(dmlResult, resultType)
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
	children, err := vm.treeSaveChildGroups(root)
	if err != nil {
		return Null, err
	}
	parentOperation := "insert"
	if id := sObjectIDFromFields(root.Fields); id != "" {
		parentOperation = "update"
	}
	parentResults, err := vm.applyDML(parentOperation, root, true, "", dml.Options{}, result)
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
		childResults := make([]dml.Result, 0, len(group.children))
		for _, child := range group.children {
			if err := vm.ensureTreeSaveLeaf(child); err != nil {
				return Null, err
			}
			if _, _, ok := objectFieldValue(child, group.lookupField); !ok {
				vm.setExplicitSObjectFieldValue(&child, group.lookupField, platformScalar("Id", string(parentID)))
			}
			childOperation := "insert"
			if id := sObjectIDFromFields(child.Fields); id != "" {
				childOperation = "update"
			}
			results, err := vm.applyDML(childOperation, child, true, "", dml.Options{}, result)
			if err != nil {
				return Null, err
			}
			childResults = append(childResults, results...)
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
		row.Fields["errors"] = databaseErrorsList(results[0], "Database.NestedSaveResult")
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
	if (strings.EqualFold(receiver.Type, "Database.Cursor") || strings.EqualFold(receiver.Type, "Database.PaginationCursor")) && size > 0 && start+size > len(records.List) {
		return Null, receiver, false, true, newExceptionError("System.InvalidParameterValueException", fmt.Sprintf("Fetch beyond bound detected: %d", start+size))
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
	nextIndex := end
	if end >= len(records.List) {
		nextIndex = 0
	}
	out.Fields["nextIndex"] = Int(int64(nextIndex))
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
	appendTraceLazy(result, "apex.dml."+op, "apex.dml", func() map[string]any {
		return vm.traceDMLArgs(op, records, len(records))
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
	return vm.executeDatabaseMergeWithMode(args, ir.DMLModeDefault, result)
}

func (vm *VM) executeDatabaseMergeWithMode(args []Value, mode ir.DMLMode, result *Result) (Value, error) {
	if len(args) < 2 || len(args) > 4 {
		return Null, fmt.Errorf("Database.merge expects master, duplicate record(s), and optional allOrNone")
	}
	allOrNone := true
	dmlMode := vm.resolveDMLMode(mode)
	accessLevel := Value{}
	if len(args) == 3 {
		if isDatabaseAccessLevelValue(args[2]) {
			dmlMode = databaseAccessLevelSecurityMode(args[2])
			accessLevel = args[2]
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
		dmlMode = databaseAccessLevelSecurityMode(args[3])
		accessLevel = args[3]
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
	if dmlMode == "USER_MODE" {
		if err := vm.enforceUserModeDMLAccess("update", args[0], accessLevel); err != nil {
			return Null, err
		}
		if err := vm.enforceUserModeDMLAccess("delete", duplicateInput, accessLevel); err != nil {
			return Null, err
		}
	}
	if err := vm.enforceDMLRecordAccess("update", args[0], "", dmlMode == "USER_MODE"); err != nil {
		return Null, err
	}
	if err := vm.enforceDMLRecordAccess("delete", duplicateInput, "", dmlMode == "USER_MODE"); err != nil {
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
	appendTraceLazy(result, "apex.dml.merge", "apex.dml", func() map[string]any {
		return vm.traceDMLArgs("merge", recordsForChecks, len(recordsForChecks))
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
		row.Fields["errors"] = databaseErrorsList(dmlResult, "Database.MergeResult")
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

func (vm *VM) preflightUpdateIdentityFailures(records []storage.Record) []dml.Result {
	var out []dml.Result
	for i, record := range records {
		failure, failed := vm.preflightUpdateIdentityFailure(record)
		if !failed {
			if out != nil {
				out[i] = dml.Result{ID: record.ID, Success: true}
			}
			continue
		}
		if out == nil {
			out = make([]dml.Result, len(records))
			for j := 0; j < i; j++ {
				out[j] = dml.Result{ID: records[j].ID, Success: true}
			}
		}
		out[i] = failure
	}
	return out
}

func (vm *VM) preflightUpdateIdentityFailure(record storage.Record) (dml.Result, bool) {
	if record.ID == "" {
		return dmlFailure(record.ID, "Id not specified in an update call:", "MISSING_ARGUMENT", []string{"Id"}), true
	}
	objectName := record.Object
	if canonical, ok := vm.resolveObjectName(record.Object); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return dmlFailure(record.ID, "dml: unknown object "+record.Object, "INVALID_TYPE", nil), true
	}
	if storedID, existing, found := storage.LookupRecordByID(object.Records, record.ID); found {
		if existing.System.IsDeleted {
			if vm.inAfterUndeleteTrigger() {
				return dml.Result{}, false
			}
			return dmlFailure(storedID, "dml: record "+string(record.ID)+" is deleted", "ENTITY_IS_DELETED", nil), true
		}
		return dml.Result{}, false
	}
	if prefix := object.Definition.KeyPrefix; prefix != "" && !strings.HasPrefix(string(record.ID), prefix) {
		return dmlFailure(record.ID, "dml: id "+string(record.ID)+" does not belong to "+record.Object, "INVALID_FIELD", []string{"Id"}), true
	}
	return dmlFailure(record.ID, "invalid cross reference id", "INVALID_CROSS_REFERENCE_KEY", nil), true
}

func dmlFailure(id storage.ID, message, statusCode string, fields []string) dml.Result {
	copiedFields := append([]string(nil), fields...)
	return dml.Result{
		ID:         id,
		Success:    false,
		Error:      message,
		StatusCode: statusCode,
		Fields:     copiedFields,
		Errors: []dml.Error{{
			Message:    message,
			StatusCode: statusCode,
			Fields:     append([]string(nil), copiedFields...),
		}},
	}
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

type vmDMLRollbackPoint struct {
	enabled          bool
	journal          bool
	temporaryJournal bool
	mark             storage.IsolationMark
	org              storage.OrgState
	previousJournal  *storage.IsolationJournal
}

func (vm *VM) beginDMLRollbackPoint(enabled bool, forceSnapshot bool) vmDMLRollbackPoint {
	if !enabled {
		return vmDMLRollbackPoint{}
	}
	if !forceSnapshot && vm != nil && vm.Org != nil {
		if vm.isolationJournal != nil {
			vm.recordJournalDMLRollbackPoint()
			return vmDMLRollbackPoint{enabled: true, journal: true, mark: vm.isolationJournal.Mark()}
		}
		journal := storage.NewIsolationJournal(vm.Org)
		previous := vm.isolationJournal
		vm.isolationJournal = journal
		vm.recordTemporaryDMLJournalPoint()
		return vmDMLRollbackPoint{
			enabled:          true,
			journal:          true,
			temporaryJournal: true,
			mark:             journal.Mark(),
			previousJournal:  previous,
		}
	}
	if vm == nil || vm.Org == nil {
		return vmDMLRollbackPoint{enabled: true}
	}
	vm.recordSnapshotDMLRollbackPoint()
	return vmDMLRollbackPoint{enabled: true, org: snapshotRuntimeOrgState(vm.Org)}
}

func (vm *VM) finishDMLRollbackPoint(point vmDMLRollbackPoint) {
	if vm == nil || !point.enabled || !point.temporaryJournal {
		return
	}
	vm.isolationJournal = point.previousJournal
}

func (vm *VM) restoreDMLRollbackPoint(point vmDMLRollbackPoint) error {
	if vm == nil || vm.Org == nil || !point.enabled {
		return nil
	}
	defer vm.finishDMLRollbackPoint(point)
	if point.journal && vm.isolationJournal != nil {
		return vm.isolationJournal.Rollback(point.mark)
	}
	*vm.Org = point.org
	return nil
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
	bulkPrevious := aliasSnapshot{}
	bulkPropagate := value.Kind == ValueList && value.Ref != 0
	if bulkPropagate {
		bulkPrevious = snapshotAlias(value)
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
		return vm.applyUpsertDML(records, targets, allOrNone, externalIDField, options, result)
	}
	appendTraceLazy(result, "apex.dml."+op, "apex.dml", func() map[string]any {
		return vm.traceDMLArgs(op, records, len(records))
	})
	if op == "insert" {
		vm.applySObjectFieldDefaults(records)
		vm.applyBeforeDMLDerivedFields(records)
	}
	before, err := vm.oldRecords(op, records)
	if err != nil {
		return nil, err
	}
	var beforeFailures []dml.Result
	var currentFailures []dml.Result
	if op == "update" {
		identityFailures := vm.preflightUpdateIdentityFailures(records)
		if hasDMLFailures(identityFailures) {
			appendDMLResultTrace(result, op, records, identityFailures)
			if allOrNone {
				return identityFailures, nil
			}
			records, before, targets = filterDMLInputs(records, before, targets, identityFailures)
			if len(records) == 0 {
				return identityFailures, nil
			}
			beforeFailures = identityFailures
		}
	}
	var rollback vmDMLRollbackPoint
	rollbackReady := false
	ensureRollback := func(forceSnapshot bool) {
		if rollbackReady {
			return
		}
		rollback = vm.beginDMLRollbackPoint(true, forceSnapshot)
		rollbackReady = true
	}
	restoreRollback := func() error {
		return vm.restoreDMLRollbackPoint(rollback)
	}
	defer func() {
		if rollbackReady {
			vm.finishDMLRollbackPoint(rollback)
		}
	}()
	var partialAfterTriggerBackup storage.OrgState
	partialAfterTriggerBackupReady := false
	ensurePartialAfterTriggerBackup := func() {
		if partialAfterTriggerBackupReady {
			return
		}
		partialAfterTriggerBackup = snapshotRuntimeOrgState(vm.Org)
		partialAfterTriggerBackupReady = true
	}
	if vm.needsEarlyDMLRollbackSnapshot(op, records, allOrNone) {
		ensureRollback(false)
	}
	originalUpdateRecords := records
	beforeTriggerRecords := records
	if op == "update" {
		beforeTriggerRecords = vm.hydrateUpdateTriggerRecords(records, before)
	}
	if op == "insert" || op == "update" {
		vm.applyBeforeDMLDerivedFields(beforeTriggerRecords)
		if err := vm.applyBeforeSaveFlows(beforeTriggerRecords, result); err != nil {
			ensureRollback(false)
			if rollbackErr := restoreRollback(); rollbackErr != nil {
				return nil, rollbackErr
			}
			return nil, err
		}
		vm.applyBeforeDMLDerivedFields(beforeTriggerRecords)
	}
	if op != "undelete" {
		triggerFailures, triggerRecords, triggerErr := vm.runTriggersByObject(triggerTimingBefore, op, beforeTriggerRecords, before, result)
		beforeTriggerRecords = triggerRecords
		err = triggerErr
		if err != nil {
			ensureRollback(false)
			if rollbackErr := restoreRollback(); rollbackErr != nil {
				return nil, rollbackErr
			}
			return nil, dmlExceptionFromTriggerError(op, err)
		}
		currentFailures = triggerFailures
		if len(beforeFailures) > 0 {
			beforeFailures = mergeDMLResults(beforeFailures, triggerFailures)
		} else {
			beforeFailures = triggerFailures
		}
		if op != "delete" {
			if op == "insert" || op == "update" {
				vm.applyBeforeDMLDerivedFields(beforeTriggerRecords)
			}
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
			if len(beforeFailures) > 0 {
				beforeFailures = mergeDMLResults(beforeFailures, targetFailures)
			} else {
				beforeFailures = targetFailures
			}
			if len(currentFailures) > 0 {
				currentFailures = mergeDMLResults(currentFailures, targetFailures)
			} else {
				currentFailures = targetFailures
			}
		}
	}
	if hasDMLFailures(beforeFailures) {
		appendDMLResultTrace(result, op, records, beforeFailures)
		if allOrNone {
			ensureRollback(false)
			if rollbackErr := restoreRollback(); rollbackErr != nil {
				return nil, rollbackErr
			}
			return beforeFailures, nil
		}
		filterFailures := beforeFailures
		if len(currentFailures) > 0 {
			filterFailures = currentFailures
		}
		records, before, targets = filterDMLInputs(records, before, targets, filterFailures)
		if len(records) == 0 {
			return beforeFailures, nil
		}
	}
	if op == "insert" {
		if err := vm.resolveSameBatchParentRelationships(records, targets); err != nil {
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
			}
			return nil, err
		}
	}
	vm.stripTransientDMLDerivedFields(records)
	if err := vm.applyPerRecordDMLTargetOptions(records, targets); err != nil {
		return nil, err
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	engine.Options = options
	engine.Options.AllowBatchUniqueValueSwap = allOrNone
	engine.PriorRecords = dmlPriorRecordsByID(before)
	if op == "update" && vm.inAfterUndeleteTrigger() {
		engine.Options.AllowUpdateDeleted = true
	}
	if !allOrNone && vm.hasAfterTriggerForDML(op, records) {
		ensurePartialAfterTriggerBackup()
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
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
				return results, nil
			}
		}
	}
	vm.applyStoredDMLDerivedFields(records, engineResults)
	for i, dmlResult := range engineResults {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			previous := snapshotAlias(*targets[i])
			vm.populateDMLResultFields(targets[i], engineResults[i:i+1])
			if !bulkPropagate {
				vm.propagateAliasSnapshotToScope(vm.Globals, previous, *targets[i])
				vm.propagateAliasSnapshotToStatics(previous, *targets[i])
			}
		}
	}
	if bulkPropagate {
		vm.propagateAliasSnapshotToScope(vm.Globals, bulkPrevious, value)
		vm.propagateAliasSnapshotToStatics(bulkPrevious, value)
	}
	afterInputRecords, afterInputBefore, afterInputResults := successfulDMLInputs(records, before, engineResults)
	afterRecords := afterInputRecords
	if vm.installContextDepth == 0 {
		var err error
		afterRecords, err = vm.afterRecords(op, afterInputRecords, afterInputResults)
		if err != nil {
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
			}
			return nil, err
		}
		afterFailures, _, err := vm.runTriggersByObject(triggerTimingAfter, op, afterRecords, afterInputBefore, result)
		if err != nil {
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
			}
			return nil, dmlExceptionFromTriggerError(op, err)
		}
		if hasDMLFailures(afterFailures) {
			results = mergeAfterTriggerDMLResults(results, afterFailures)
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
				vm.clearDMLResultFieldsForFailures(targets, results, []string{op})
				return results, nil
			}
			ensurePartialAfterTriggerBackup()
			vm.rollbackAfterTriggerFailures(op, afterRecords, afterFailures, partialAfterTriggerBackup)
			afterRecords = filterAfterTriggerRecords(afterRecords, afterFailures)
		}
	}
	if err := vm.runSummaryUpdateTriggers(&engine, allOrNone, rollback, result); err != nil {
		return results, err
	}
	if op == "insert" || op == "update" || op == "upsert" {
		if err := vm.applyDeferredAutomation(&engine, afterRecords, afterInputBefore, allOrNone, rollback, result); err != nil {
			return nil, err
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
	appendTraceLazy(result, "apex.dml."+op+".result", "apex.dml", func() map[string]any {
		return map[string]any{
			"operation": op,
			"objects":   dmlTraceObjectNames(records),
			"successes": successes,
			"failures":  failures,
			"errors":    errors,
		}
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
	if policy := dmlPolicyForObject(objectName); policy != nil && policy.defaultRecordTypeID != nil {
		if id := policy.defaultRecordTypeID(definition, record); id != "" {
			return id
		}
	}
	return defaultRecordTypeID(definition)
}

func (vm *VM) applyBeforeDMLDerivedFields(records []storage.Record) {
	if vm.Org == nil {
		return
	}
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		policy := dmlPolicyForObject(objectName)
		if policy == nil || policy.beforeDMLDerivedFields == nil {
			continue
		}
		policy.beforeDMLDerivedFields(vm, &records[i])
	}
}

func (vm *VM) stripTransientDMLDerivedFields(records []storage.Record) {
	if vm == nil || vm.Org == nil {
		return
	}
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		policy := dmlPolicyForObject(objectName)
		if policy == nil {
			continue
		}
		for _, field := range policy.transientDMLDerivedFields {
			delete(records[i].Fields, field)
			delete(records[i].ExplicitNulls, field)
		}
	}
}

func (vm *VM) applyStoredDMLDerivedFields(records []storage.Record, results []dml.Result) {
	if vm == nil || vm.Org == nil {
		return
	}
	for i, result := range results {
		if !result.Success || i >= len(records) {
			continue
		}
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		policy := dmlPolicyForObject(objectName)
		if policy == nil || policy.storedDMLDerivedFields == nil {
			continue
		}
		id := result.ID
		if id == "" {
			id = records[i].ID
		}
		if id == "" {
			continue
		}
		object, ok := vm.Org.Objects[objectName]
		if !ok {
			continue
		}
		recordID, record, ok := storage.LookupRecordByID(object.Records, id)
		if !ok {
			continue
		}
		vm.recordIsolationJournalMutation(objectName, recordID, record, true)
		policy.storedDMLDerivedFields(vm, &record)
		object.Records[recordID] = record
		vm.Org.Objects[objectName] = object
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
	if storage.IsCustomSettingDefinition(definition) && strings.EqualFold(definition.Metadata["customSettingsType"], "List") {
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

func (vm *VM) applyUpsertDML(records []storage.Record, targets []*Value, allOrNone bool, externalIDField string, options dml.Options, result *Result) ([]dml.Result, error) {
	appendTraceLazy(result, "apex.dml.upsert", "apex.dml", func() map[string]any {
		return vm.traceDMLArgs("upsert", records, len(records))
	})
	var rollback vmDMLRollbackPoint
	rollbackReady := false
	ensureRollback := func(forceSnapshot bool) {
		if rollbackReady {
			return
		}
		rollback = vm.beginDMLRollbackPoint(true, forceSnapshot)
		rollbackReady = true
	}
	restoreRollback := func() error {
		return vm.restoreDMLRollbackPoint(rollback)
	}
	defer func() {
		if rollbackReady {
			vm.finishDMLRollbackPoint(rollback)
		}
	}()
	var partialAfterTriggerBackup storage.OrgState
	partialAfterTriggerBackupReady := false
	ensurePartialAfterTriggerBackup := func() {
		if partialAfterTriggerBackupReady {
			return
		}
		partialAfterTriggerBackup = snapshotRuntimeOrgState(vm.Org)
		partialAfterTriggerBackupReady = true
	}
	if allOrNone {
		ensureRollback(false)
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
		vm.applyBeforeDMLDerivedFields(insertRecord)
		records[i] = insertRecord[0]
	}
	beforeFailures := make([]dml.Result, len(records))
	for _, kind := range []string{"insert", "update"} {
		groupRecords, groupBefore, indices := groupedDMLInputs(records, before, kinds, kind)
		triggerRecords := groupRecords
		if kind == "update" {
			triggerRecords = vm.hydrateUpdateTriggerRecords(groupRecords, groupBefore)
		}
		vm.applyBeforeDMLDerivedFields(triggerRecords)
		if err := vm.applyBeforeSaveFlows(triggerRecords, result); err != nil {
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
			}
			return nil, err
		}
		vm.applyBeforeDMLDerivedFields(triggerRecords)
		failures, err := vm.runTriggers(triggerTimingBefore, kind, triggerRecords, groupBefore, result)
		if err != nil {
			ensureRollback(false)
			if rollbackErr := restoreRollback(); rollbackErr != nil {
				return nil, rollbackErr
			}
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
		vm.applyBeforeDMLDerivedFields(triggerRecords)
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
			ensureRollback(false)
			if rollbackErr := restoreRollback(); rollbackErr != nil {
				return nil, rollbackErr
			}
			return beforeFailures, nil
		}
		records, before, targets, kinds = filterUpsertInputs(records, before, targets, kinds, beforeFailures)
		if len(records) == 0 {
			return beforeFailures, nil
		}
	}
	if err := vm.resolveSameBatchParentRelationships(records, targets); err != nil {
		if allOrNone {
			ensureRollback(false)
			if rollbackErr := restoreRollback(); rollbackErr != nil {
				return nil, rollbackErr
			}
		}
		return nil, err
	}
	vm.stripTransientDMLDerivedFields(records)
	if err := vm.applyPerRecordDMLTargetOptions(records, targets); err != nil {
		return nil, err
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	engine.Options = options
	engine.Options.AllowBatchUniqueValueSwap = allOrNone
	engine.PriorRecords = dmlPriorRecordsByID(before)
	if !allOrNone && vm.hasAfterTriggerForDML("upsert", records) {
		ensurePartialAfterTriggerBackup()
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
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
				return results, nil
			}
		}
	}
	vm.applyStoredDMLDerivedFields(records, engineResults)
	for i, dmlResult := range engineResults {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			previous := snapshotAlias(*targets[i])
			vm.populateDMLResultFields(targets[i], engineResults[i:i+1])
			vm.propagateAliasSnapshotToScope(vm.Globals, previous, *targets[i])
			vm.propagateAliasSnapshotToStatics(previous, *targets[i])
		}
	}
	for _, kind := range []string{"insert", "update"} {
		groupRecords, groupBefore, groupResults, indices := successfulGroupedDMLInputs(records, before, engineResults, kinds, kind)
		afterRecords, err := vm.afterRecords(kind, groupRecords, groupResults)
		if err != nil {
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
			}
			return results, err
		}
		afterFailures, err := vm.runTriggers(triggerTimingAfter, kind, afterRecords, groupBefore, result)
		if err != nil {
			if allOrNone {
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
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
				ensureRollback(false)
				if rollbackErr := restoreRollback(); rollbackErr != nil {
					return nil, rollbackErr
				}
				vm.clearDMLResultFieldsForFailures(targets, results, kinds)
				return results, nil
			}
			ensurePartialAfterTriggerBackup()
			vm.rollbackAfterTriggerFailures(kind, afterRecords, afterFailures, partialAfterTriggerBackup)
			afterRecords = filterAfterTriggerRecords(afterRecords, afterFailures)
		}
		if err := vm.runSummaryUpdateTriggers(&engine, allOrNone, rollback, result); err != nil {
			return results, err
		}
		if err := vm.applyDeferredAutomation(&engine, afterRecords, groupBefore, allOrNone, rollback, result); err != nil {
			return results, err
		}
	}
	if hasDMLSuccess(results) {
		vm.rebuildDMLObjectIndexes(records, results)
		vm.clearCustomDataCache()
	}
	return results, nil
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
	index := vm.summarySideEffectObjectIndex()
	for objectName := range objects {
		if index[objectName] {
			return true
		}
	}
	return false
}

func (vm *VM) summarySideEffectObjectIndex() map[string]bool {
	if vm == nil || vm.Org == nil {
		return nil
	}
	if vm.summarySideEffectObjects != nil {
		return vm.summarySideEffectObjects
	}
	index := make(map[string]bool)
	for _, object := range vm.Org.Objects {
		for _, field := range object.Definition.Fields {
			if field.Type != storage.FieldSummary {
				continue
			}
			summaryObject, _ := splitQualifiedField(field.SummarizedField)
			lookupObject, _ := splitQualifiedField(field.SummaryForeignKey)
			vm.addSummarySideEffectObject(index, summaryObject)
			vm.addSummarySideEffectObject(index, lookupObject)
		}
	}
	vm.summarySideEffectObjects = index
	return index
}

func (vm *VM) addSummarySideEffectObject(index map[string]bool, objectName string) {
	objectName = vm.resolveSummarySideEffectObjectName(objectName)
	if objectName != "" {
		index[strings.ToLower(objectName)] = true
	}
}

func (vm *VM) resolveSummarySideEffectObjectName(objectName string) string {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return ""
	}
	if vm != nil && vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
	}
	return objectName
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
	sourceKeyPrefix := ""
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectType); ok {
			objectType = canonical
			sourceDefinition := vm.Org.Objects[canonical].Definition
			sourceKeyPrefix = sourceDefinition.KeyPrefix
			definition = vm.describePreparedDefinition(canonical, sourceDefinition)
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
		if queried || isExplicitSObjectField(*value, "Id") || sourceKeyPrefix == "" || strings.HasPrefix(string(id), sourceKeyPrefix) {
			record.ID = id
		}
	}
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) || isDefaultedSObjectField(*value, field) {
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
			if resolved, ok := vm.resolveFieldName(definition, field); ok {
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
			if fieldDef, ok := definition.Fields[canonicalField]; ok && vmImplicitDMLFieldForOperation(fieldDef, record.ID == "") {
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
		if fieldValue.Kind == ValueObject && !strings.EqualFold(fieldValue.Type, "Id") && vm.isParentRelationshipField(definition, field) {
			if lookupField, ok := vm.parentRelationshipField(objectType, field); ok {
				if parentRecord, ok, err := vm.parentRelationshipRecordFromValue(fieldValue); err != nil {
					return storage.Record{}, err
				} else if ok {
					vm.storeParentRelationshipRecord(&record, definition, lookupField, field, parentRecord)
				}
				if !recordHasFieldAlias(record, lookupField) && !recordHasExplicitNullAlias(record, lookupField) {
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
		if fieldValue.Kind == ValueNull && vm.isParentRelationshipField(definition, field) {
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
				converted, err = storageValueFromVMForField(fieldValue, fieldDef)
			}
		}
		if err != nil {
			return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
		}
		if converted.Kind == storage.ValueNull {
			aliasKey := sObjectRecordFieldAliasKey(canonicalField)
			if previousField, exists := recordFieldsByAlias[aliasKey]; exists {
				userSetField := isUserSetSObjectFieldAlias(*value, field) || isUserSetSObjectFieldAlias(*value, canonicalField)
				if !explicitField || !userSetField {
					if _, hasNonNull := record.Fields[previousField]; hasNonNull {
						continue
					}
				}
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
			if isUserSetSObjectFieldAlias(*value, field) || isUserSetSObjectFieldAlias(*value, canonicalField) {
				deleteRecordFieldAliases(&record, canonicalField)
			}
			record.ExplicitNulls[canonicalField] = true
			recordFieldsByAlias[aliasKey] = canonicalField
			recordExplicitFields[aliasKey] = explicitField
			recordFieldSourceByAlias[aliasKey] = field
		} else {
			aliasKey := sObjectRecordFieldAliasKey(canonicalField)
			if previousField, exists := recordFieldsByAlias[aliasKey]; exists {
				previousSource := recordFieldSourceByAlias[aliasKey]
				if record.ExplicitNulls[previousField] {
					previousUserSet := isUserSetSObjectFieldAlias(*value, previousSource) || isUserSetSObjectFieldAlias(*value, previousField)
					if previousUserSet {
						continue
					}
				} else {
					if recordExplicitFields[aliasKey] && !explicitField {
						continue
					}
					if recordExplicitFields[aliasKey] && explicitField && !sObjectRecordFieldSourcePreferred(field, previousSource, canonicalField, vm.OrgNamespace()) {
						continue
					}
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

func (vm *VM) parentRelationshipRecordFromValue(value Value) (storage.Record, bool, error) {
	if value.Kind != ValueObject || strings.EqualFold(value.Type, "Id") {
		return storage.Record{}, false, nil
	}
	objectType := value.Type
	var definition storage.ObjectDefinition
	if vm != nil && vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectType); ok {
			objectType = canonical
			definition = vm.describePreparedDefinition(canonical, vm.Org.Objects[canonical].Definition)
		}
	}
	record := storage.Record{
		Object:        objectType,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	if id := sObjectIDFromFields(value.Fields); id != "" {
		record.ID = id
	}
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) || strings.EqualFold(field, "Id") {
			continue
		}
		if strings.Contains(field, ".") && !isExplicitSObjectField(value, field) {
			continue
		}
		canonicalField := field
		if definition.APIName != "" {
			if resolved, ok := vm.resolveFieldName(definition, field); ok {
				canonicalField = resolved
			}
		}
		if fieldValue.Kind == ValueList && definition.APIName != "" && vm.isChildRelationshipField(definition, field) {
			continue
		}
		if fieldValue.Kind == ValueObject && !strings.EqualFold(fieldValue.Type, "Id") && vm.isParentRelationshipField(definition, field) {
			if lookupField, ok := vm.parentRelationshipField(objectType, field); ok {
				if parentRecord, ok, err := vm.parentRelationshipRecordFromValue(fieldValue); err != nil {
					return storage.Record{}, false, err
				} else if ok {
					vm.storeParentRelationshipRecord(&record, definition, lookupField, field, parentRecord)
				}
				if !recordHasFieldAlias(record, lookupField) && !recordHasExplicitNullAlias(record, lookupField) {
					if parentID := sObjectIDFromFields(fieldValue.Fields); parentID != "" {
						record.Fields[lookupField] = storage.IDValue(parentID)
					} else if parentID, err := vm.resolveParentRelationshipReferenceID(nil, fieldValue); err != nil {
						return storage.Record{}, false, err
					} else if parentID != "" {
						record.Fields[lookupField] = storage.IDValue(parentID)
					}
				}
			}
			continue
		}
		if fieldValue.Kind == ValueObject || fieldValue.Kind == ValueList {
			continue
		}
		converted, err := storageValueFromVM(fieldValue)
		if definition.APIName != "" {
			if fieldDef, ok := definition.Fields[canonicalField]; ok {
				converted, err = storageValueFromVMForField(fieldValue, fieldDef)
			}
		}
		if err != nil {
			return storage.Record{}, false, fmt.Errorf("%s.%s: %w", value.Type, field, err)
		}
		if converted.Kind == storage.ValueNull {
			record.ExplicitNulls[canonicalField] = true
			delete(record.Fields, canonicalField)
		} else {
			record.Fields[canonicalField] = converted
			delete(record.ExplicitNulls, canonicalField)
		}
	}
	return record, true, nil
}

func (vm *VM) storeParentRelationshipRecord(record *storage.Record, definition storage.ObjectDefinition, lookupField, relationship string, parent storage.Record) {
	if record == nil {
		return
	}
	if record.ParentRelationships == nil {
		record.ParentRelationships = make(map[string]storage.Record)
	}
	for _, name := range vm.parentRelationshipStorageNames(definition, lookupField, relationship) {
		if strings.TrimSpace(name) == "" {
			continue
		}
		record.ParentRelationships[name] = parent.Clone()
	}
}

func (vm *VM) parentRelationshipStorageNames(definition storage.ObjectDefinition, lookupField, relationship string) []string {
	names := []string{relationship}
	if parentRelationship, ok := vm.parentRelationshipNameForField(definition, lookupField); ok {
		names = append(names, parentRelationship)
	}
	if derived := lookupFieldRelationshipName(lookupField); derived != "" {
		names = append(names, derived)
	}
	return names
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

func recordHasFieldAlias(record storage.Record, field string) bool {
	if _, ok := record.GetField(field); ok {
		return true
	}
	if record.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	wanted := sObjectRecordFieldAliasKey(field)
	for key := range record.Fields {
		if sObjectRecordFieldAliasKey(key) == wanted {
			return true
		}
	}
	return false
}

func recordHasExplicitNullAlias(record storage.Record, field string) bool {
	if record.HasExplicitNull(field) {
		return true
	}
	if record.ExplicitNulls == nil || strings.TrimSpace(field) == "" {
		return false
	}
	wanted := sObjectRecordFieldAliasKey(field)
	for key, value := range record.ExplicitNulls {
		if value && sObjectRecordFieldAliasKey(key) == wanted {
			return true
		}
	}
	return false
}

func deleteRecordFieldAliases(record *storage.Record, field string) {
	if record == nil || record.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	wanted := sObjectRecordFieldAliasKey(field)
	for key := range record.Fields {
		if sObjectRecordFieldAliasKey(key) == wanted {
			delete(record.Fields, key)
		}
	}
}

func isUserSetSObjectFieldAlias(value Value, field string) bool {
	if isUserSetSObjectField(value, field) {
		return true
	}
	if value.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	selected, ok := value.Fields[sobjectUserSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	wanted := sObjectRecordFieldAliasKey(field)
	for _, key := range selected.MapKeys {
		if key.Kind == ValueString && sObjectRecordFieldAliasKey(key.Text) == wanted {
			return true
		}
	}
	return false
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
			if recordHasFieldAlias(records[i], lookupField) || recordHasExplicitNullAlias(records[i], lookupField) {
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
		if resolved, ok := vm.resolveFieldName(parentObject.Definition, fieldName); ok {
			canonicalField = resolved
		}
		fieldDef, ok := parentObject.Definition.Fields[canonicalField]
		if !ok || !parentRelationshipReferenceFieldCanMatch(fieldDef) {
			continue
		}
		lookupValue, err := storageValueFromVMForField(fieldValue, fieldDef)
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
	if ok && storage.StandardObjectFieldsNeedWrite(object.Definition) {
		object.Definition = object.Definition.Clone()
		storage.EnsureStandardObjectFields(&object.Definition)
		vm.Org.Objects[objectName] = object
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
	vm.collapseNullParentRelationshipObjects(&value, record.Object)
	vm.hydrateParentLookupFields(value)
	putSystemFields(value, record.System)
	return value
}

func (vm *VM) collapseNullParentRelationshipObjects(value *Value, objectName string) {
	if vm == nil || vm.Org == nil || value == nil || value.Kind != ValueObject || value.Fields == nil {
		return
	}
	canonicalObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return
	}
	for _, relation := range object.Definition.Relations {
		relationshipName := relation.ParentRelationship
		if strings.TrimSpace(relationshipName) == "" {
			continue
		}
		actual, relationship, ok := objectFieldValue(*value, relationshipName)
		if !ok || relationship.Kind != ValueObject {
			continue
		}
		parentType := ""
		for _, parent := range relation.ParentObjects {
			if resolved, ok := vm.resolveObjectName(parent); ok {
				parentType = resolved
				break
			}
			if strings.TrimSpace(parent) != "" {
				parentType = parent
				break
			}
		}
		if parentType != "" {
			vm.collapseNullParentRelationshipObjects(&relationship, parentType)
		}
		if parentRelationshipObjectIsMissing(relationship) {
			typedNull := Null
			if parentType != "" {
				typedNull.Type = parentType
				typedNull.Runtime = relationshipNullRuntime
			}
			value.Fields[actual] = typedNull
			continue
		}
		value.Fields[actual] = relationship
	}
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) || !customRelationshipLikeSOQLNameForVM(field) || fieldValue.Kind != ValueObject {
			continue
		}
		parentType := fieldValue.Type
		if parentType == "" || !vm.isSObjectLikeType(parentType) {
			if inferred, ok := vm.parentRelationshipObjectType(canonicalObject, field); ok {
				parentType = inferred
			}
		}
		if parentType != "" {
			vm.collapseNullParentRelationshipObjects(&fieldValue, parentType)
		}
		if parentRelationshipObjectIsMissing(fieldValue) {
			typedNull := Null
			if parentType != "" {
				typedNull.Type = parentType
				typedNull.Runtime = relationshipNullRuntime
			}
			value.Fields[field] = typedNull
			continue
		}
		value.Fields[field] = fieldValue
	}
}

func customRelationshipLikeSOQLNameForVM(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__r") && strings.Count(lower, "__") >= 1
}

func parentRelationshipObjectIsMissing(value Value) bool {
	if value.Kind != ValueObject || value.Fields == nil {
		return false
	}
	if _, id, ok := objectFieldValue(value, "Id"); ok && id.Kind != ValueNull {
		return false
	}
	seenField := false
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		seenField = true
		if isExplicitSObjectField(value, field) {
			return false
		}
		if fieldValue.Kind != ValueNull {
			return false
		}
	}
	return seenField
}

func parentRelationshipObjectIsMissingDeep(value Value) bool {
	if value.Kind != ValueObject || value.Fields == nil {
		return false
	}
	if _, id, ok := objectFieldValue(value, "Id"); ok && id.Kind != ValueNull {
		return false
	}
	seenField := false
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		seenField = true
		if isExplicitSObjectField(value, field) {
			return false
		}
		if fieldValue.Kind == ValueObject && parentRelationshipObjectIsMissingDeep(fieldValue) {
			continue
		}
		if fieldValue.Kind != ValueNull {
			return false
		}
	}
	return seenField
}

func (vm *VM) canonicalChildRelationshipName(parentObject, relationship string) string {
	return vm.childRelationshipLookup(parentObject, relationship).CanonicalRelationship
}

func (vm *VM) childRelationshipListType(parentObject, relationship string, records []storage.Record) string {
	if lookup := vm.childRelationshipLookup(parentObject, relationship); lookup.ChildType != "" {
		return lookup.ChildType
	}
	if vm == nil || vm.Org == nil || strings.TrimSpace(parentObject) == "" || strings.TrimSpace(relationship) == "" {
		for _, record := range records {
			if strings.TrimSpace(record.Object) != "" {
				return record.Object
			}
		}
		return ""
	}
	for _, record := range records {
		if strings.TrimSpace(record.Object) != "" {
			return record.Object
		}
	}
	return ""
}

func (vm *VM) childRelationshipLookup(parentObject, relationship string) childRelationshipLookup {
	if vm == nil || vm.Org == nil {
		return childRelationshipLookup{}
	}
	parentObject = strings.TrimSpace(parentObject)
	relationship = strings.TrimSpace(relationship)
	if parentObject == "" || relationship == "" {
		return childRelationshipLookup{}
	}
	canonicalParent, ok := vm.resolveObjectName(parentObject)
	if !ok {
		canonicalParent = parentObject
	}
	if vm.childRelationshipLookupCache == nil {
		vm.childRelationshipLookupCache = newChildRelationshipLookupCache()
	}
	key := childRelationshipLookupKey{ParentObject: canonicalParent, Relationship: relationship}
	if cached, ok := vm.childRelationshipLookupCache.load(key); ok {
		return cached
	}
	lookup := childRelationshipLookup{}
	matches := make([]string, 0, 1)
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
				matches = appendUniqueStringFold(matches, childName)
				if lookup.CanonicalRelationship == "" {
					switch {
					case strings.HasSuffix(childRelationshipName, "__r") || strings.HasSuffix(relationship, "__r"):
						lookup.CanonicalRelationship = childRelationshipName
					case strings.HasSuffix(relation.Field, "__c"):
						lookup.CanonicalRelationship = relationship + "__r"
					default:
						lookup.CanonicalRelationship = relationship
					}
				}
			}
		}
	}
	if childName := vm.bestChildRelationshipObject(matches); childName != "" {
		lookup.ChildType = childName
	}
	return vm.childRelationshipLookupCache.store(key, lookup)
}

func isSObjectSystemField(field string) bool {
	switch strings.ToLower(field) {
	case "id", "createddate", "createdbyid", "lastmodifieddate", "lastmodifiedbyid", "systemmodstamp", "ownerid", "isdeleted":
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
		if fieldValue.Kind == ValueNull {
			if parentType, ok := vm.parentRelationshipObjectType(objectName, field); ok {
				fieldValue.Type = parentType
				fieldValue.Runtime = relationshipNullRuntime
			}
		}
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
	if hasSuffixFold(candidate, "__r") && strings.EqualFold(canonical+"__r", candidate) {
		return true
	}
	if hasSuffixFold(canonical, "__r") && strings.EqualFold(canonical[:len(canonical)-3], candidate) {
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

func storageValueFromVMForField(value Value, field storage.Field) (storage.Value, error) {
	fieldType := field.Type
	if value.Kind == ValueNull || fieldType == storage.FieldAny {
		return storageValueFromVM(value)
	}
	switch fieldType {
	case storage.FieldCalculated, storage.FieldSummary:
		return storageValueFromVM(value)
	case storage.FieldID, storage.FieldReference:
		if fieldReferencesNamedMetadata(field) {
			if value.Kind == ValueString {
				return storage.StringValue(value.Text), nil
			}
			if text, ok := platformScalarObjectText(value); ok {
				return storage.StringValue(text), nil
			}
		}
		if value.Kind == ValueString {
			return storage.IDValue(storage.ID(value.Text)), nil
		}
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			if raw, err := platformScalarText(value, "Id"); err == nil {
				return storage.IDValue(storage.ID(raw)), nil
			}
		}
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		if value.Kind == ValueString && strings.EqualFold(value.Type, "Id") {
			if len(value.Text) == 15 {
				return storage.StringValue(textFieldIDStorageText(value.Text, field)), nil
			}
			return storage.StringValue(value.Text), nil
		}
		if value.Kind == ValueString {
			if (fieldType == storage.FieldPicklist || fieldType == storage.FieldMultiPicklist) && value.Text == "" {
				return storage.NullValue(), nil
			}
			return storageValueFromVM(value)
		}
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			if raw, err := platformScalarText(value, "Id"); err == nil {
				if len(raw) == 15 {
					raw = textFieldIDStorageText(raw, field)
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

func fieldReferencesNamedMetadata(field storage.Field) bool {
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, "EntityDefinition") || strings.EqualFold(target, "FieldDefinition") {
			return true
		}
	}
	return false
}

func textFieldIDStorageText(raw string, field storage.Field) string {
	if len(raw) == 15 && (field.Length <= 0 || field.Length >= 18) {
		return apexIDTo18(raw)
	}
	return raw
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
		out, err := decimalFromText(value.Decimal)
		if err != nil {
			return String(value.Decimal)
		}
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
	stored, err := storageValueFromVMForField(value, field)
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
		out, _ := decimalFromText("0")
		return out
	}
	out, err := decimalFromText(text)
	if err != nil {
		return value
	}
	return out
}

func coerceReadSObjectFieldRuntimeValue(value Value, field storage.Field) Value {
	if rawRecordTypeDefaultRuntimeValue(value, field) {
		return storageFieldNullValue(field)
	}
	return coerceStoredSObjectFieldRuntimeValue(value, field)
}

func coerceRawRecordTypeDefaultTokenRuntimeValue(fieldName string, value Value) Value {
	if value.Kind != ValueString {
		return value
	}
	base := strings.ToLower(storage.StripAnyNamespaceToken(fieldName))
	if !strings.Contains(base, "recordtype") {
		return value
	}
	text := strings.TrimSpace(value.Text)
	if strings.EqualFold(text, "$RecordType.Name") || strings.EqualFold(text, "$RecordType.DeveloperName") {
		return typedNull("String")
	}
	return value
}

func rawRecordTypeDefaultStorageValue(value storage.Value, field storage.Field) bool {
	if value.Kind != storage.ValueString {
		return false
	}
	raw := strings.TrimSpace(field.DefaultValue)
	switch {
	case strings.EqualFold(raw, "$RecordType.Name"), strings.EqualFold(raw, "$RecordType.DeveloperName"):
		return strings.EqualFold(strings.TrimSpace(value.String), raw)
	default:
		return false
	}
}

func rawRecordTypeDefaultRuntimeValue(value Value, field storage.Field) bool {
	if value.Kind != ValueString {
		return false
	}
	raw := strings.TrimSpace(field.DefaultValue)
	switch {
	case strings.EqualFold(raw, "$RecordType.Name"), strings.EqualFold(raw, "$RecordType.DeveloperName"):
		return strings.EqualFold(strings.TrimSpace(value.Text), raw)
	default:
		return false
	}
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
			if idValue.Kind == ValueNull {
				return "null"
			}
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
			copied.FilteredLookupInfo = cloneFilteredLookupInfo(field.FilteredLookupInfo)
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

// triggerMatchCache is shared across clones produced by CloneRuntime so that
// repeated DML in test methods does not rebuild the same trigger lookup
// table for every test. Reads dominate; an RWMutex keeps the hot path cheap
// without locking out concurrent --parallel-methods workers.
type triggerMatchCache struct {
	mu      sync.RWMutex
	entries map[string][]Trigger
}

func (c *triggerMatchCache) load(key string) ([]Trigger, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *triggerMatchCache) store(key string, value []Trigger) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
}

func (c *triggerMatchCache) reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries = make(map[string][]Trigger)
	c.mu.Unlock()
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

func (vm *VM) executeDML(op string, expr ir.Expr, externalIDField string, mode ir.DMLMode, result *Result) error {
	traceStart, traceStartedAt := traceSpanStart(result)
	if op == "merge" {
		if expr.Kind != ir.ExprCall || len(expr.Args) < 2 {
			return fmt.Errorf("merge statement requires master and duplicate record(s)")
		}
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return err
			}
			args = append(args, value)
		}
		value, err := vm.executeDatabaseMergeWithMode(args, mode, result)
		appendDurationTraceLazy(result, "apex.dml."+op, "apex.dml", traceStart, traceDurationSince(traceStartedAt), func() map[string]any {
			return vm.traceDMLArgs(op, nil, len(args))
		})
		if err != nil {
			return err
		}
		results := []Value{value}
		if value.Kind == ValueList {
			results = value.List
		}
		for _, mergeResult := range results {
			if mergeResult.Kind != ValueObject {
				continue
			}
			success, ok := mergeResult.Fields["success"]
			if ok && success.Kind == ValueBool && success.Bool {
				continue
			}
			if errValue, ok := mergeResult.Fields["error"]; ok && errValue.Kind == ValueString && errValue.Text != "" {
				return errors.New(errValue.Text)
			}
			return errors.New("merge failed")
		}
		return nil
	}
	value, err := vm.eval(expr, result)
	if err != nil {
		return err
	}
	var traceRecords []storage.Record
	if traceIsEnabled(result) {
		var recordsErr error
		traceRecords, _, recordsErr = vm.recordsFromValueForTrace(value)
		if recordsErr != nil {
			return recordsErr
		}
	} else if vm.Org == nil {
		if _, _, recordsErr := vm.recordsFromValue(value); recordsErr != nil {
			return recordsErr
		}
	}
	dmlMode := vm.resolveDMLMode(mode)
	if dmlMode == "USER_MODE" {
		if err := vm.enforceUserModeDMLAccess(op, value, Value{}); err != nil {
			return err
		}
	}
	if err := vm.enforceDMLRecordAccess(op, value, externalIDField, dmlMode == "USER_MODE"); err != nil {
		return err
	}
	traceStart, traceStartedAt = traceSpanStart(result)
	results, err := vm.applyDML(op, value, true, externalIDField, dml.Options{}, result)
	if err != nil {
		return err
	}
	appendDurationTraceLazy(result, "apex.dml."+op, "apex.dml", traceStart, traceDurationSince(traceStartedAt), func() map[string]any {
		return vm.traceDMLArgs(op, traceRecords, len(traceRecords))
	})
	for _, dmlResult := range results {
		if !dmlResult.Success {
			vm.addVisualforceDMLPageMessages(results)
			return databaseDMLException(op, results)
		}
	}
	if expr.Kind == ir.ExprVariable {
		vm.populateDMLResultFields(&value, results)
		if err := vm.assign(expr.Name, value); err != nil {
			return err
		}
	}
	return nil
}
func copyOrgIDSequences(in map[string]uint64) map[string]uint64 {
	if in == nil {
		return nil
	}
	out := make(map[string]uint64, len(in))
	for objectName, sequence := range in {
		out[objectName] = sequence
	}
	return out
}
func maxOrgIDSequences(left, right map[string]uint64) map[string]uint64 {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := copyOrgIDSequences(left)
	if out == nil {
		out = map[string]uint64{}
	}
	for objectName, sequence := range right {
		if sequence > out[objectName] {
			out[objectName] = sequence
		}
	}
	return out
}
func (vm *VM) recordsFromValue(value Value) ([]storage.Record, []*Value, error) {
	vm.recordDMLRecordConversion(false)
	return vm.recordsFromValueUnchecked(value)
}

func (vm *VM) recordsFromValueForTrace(value Value) ([]storage.Record, []*Value, error) {
	vm.recordDMLRecordConversion(true)
	return vm.recordsFromValueUnchecked(value)
}

func (vm *VM) recordsFromValueUnchecked(value Value) ([]storage.Record, []*Value, error) {
	if value.Kind == ValueList {
		records := make([]storage.Record, 0, len(value.List))
		targets := make([]*Value, 0, len(value.List))
		var aliases map[uint64]Value
		if len(value.List) > 1 {
			aliases = vm.sObjectAliasMergeIndexForRefs(vm.sObjectAliasRefsForDMLList(value.List))
		}
		for i := range value.List {
			merged := value.List[i]
			if aliases != nil && merged.Kind == ValueObject && merged.Ref != 0 && vm.isSObjectLikeType(merged.Type) {
				if alias, ok := aliases[merged.Ref]; ok {
					mergeSObjectFieldsInto(&merged, alias)
				}
			} else {
				merged = vm.mergeSObjectAliasFields(merged)
			}
			value.List[i] = merged
			record, err := vm.recordFromValue(&value.List[i])
			if err != nil {
				return nil, nil, err
			}
			records = append(records, record)
			targets = append(targets, &value.List[i])
		}
		return records, targets, nil
	}
	value = vm.mergeSObjectAliasFields(value)
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return nil, nil, err
	}
	return []storage.Record{record}, []*Value{&value}, nil
}
func (vm *VM) mergeSObjectAliasFields(value Value) Value {
	if value.Kind != ValueObject || value.Ref == 0 || !vm.isSObjectLikeType(value.Type) {
		return value
	}
	merged := value
	for _, root := range vm.Globals {
		mergeSObjectAliasFieldsFromValue(root, value.Ref, &merged, make(map[uint64]bool))
	}
	for _, scope := range vm.scopeStack {
		for _, root := range scope {
			mergeSObjectAliasFieldsFromValue(root, value.Ref, &merged, make(map[uint64]bool))
		}
	}
	return merged
}
func (vm *VM) sObjectAliasMergeIndex() map[uint64]Value {
	return vm.sObjectAliasMergeIndexForRefs(nil)
}
func (vm *VM) sObjectAliasRefsForDMLList(values []Value) map[uint64]struct{} {
	var refs map[uint64]struct{}
	for _, value := range values {
		if value.Kind != ValueObject || value.Ref == 0 || !vm.isSObjectLikeType(value.Type) {
			continue
		}
		if refs == nil {
			refs = make(map[uint64]struct{}, len(values))
		}
		refs[value.Ref] = struct{}{}
	}
	return refs
}
func (vm *VM) sObjectAliasMergeIndexForRefs(refs map[uint64]struct{}) map[uint64]Value {
	if vm == nil || (len(vm.Globals) == 0 && len(vm.scopeStack) == 0) {
		return nil
	}
	if refs != nil && len(refs) == 0 {
		return nil
	}
	index := make(map[uint64]Value)
	seen := make(map[uint64]bool)
	sawSObjectAlias := false
	for _, root := range vm.Globals {
		if vm.collectSObjectAliasMergeIndex(root, index, seen, refs) {
			sawSObjectAlias = true
		}
	}
	for _, scope := range vm.scopeStack {
		for _, root := range scope {
			if vm.collectSObjectAliasMergeIndex(root, index, seen, refs) {
				sawSObjectAlias = true
			}
		}
	}
	if len(index) == 0 {
		if refs != nil && sawSObjectAlias {
			return index
		}
		return nil
	}
	return index
}
func (vm *VM) collectSObjectAliasMergeIndex(value Value, index map[uint64]Value, seen map[uint64]bool, refs map[uint64]struct{}) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	sawSObjectAlias := false
	if value.Kind == ValueObject && value.Ref != 0 && vm.isSObjectLikeType(value.Type) {
		sawSObjectAlias = true
		shouldIndex := refs == nil
		if refs != nil {
			_, shouldIndex = refs[value.Ref]
		}
		if shouldIndex {
			merged := index[value.Ref]
			if merged.Kind == "" {
				merged = value
			} else {
				mergeSObjectFieldsInto(&merged, value)
			}
			index[value.Ref] = merged
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if vm.collectSObjectAliasMergeIndex(child, index, seen, refs) {
				sawSObjectAlias = true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if vm.collectSObjectAliasMergeIndex(child, index, seen, refs) {
				sawSObjectAlias = true
			}
		}
		for _, child := range value.MapKeys {
			if vm.collectSObjectAliasMergeIndex(child, index, seen, refs) {
				sawSObjectAlias = true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if vm.collectSObjectAliasMergeIndex(child, index, seen, refs) {
				sawSObjectAlias = true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if vm.collectSObjectAliasMergeIndex(child, index, seen, refs) {
				sawSObjectAlias = true
			}
		}
	}
	return sawSObjectAlias
}
func mergeSObjectFieldsInto(merged *Value, source Value) {
	if merged == nil || source.Kind != ValueObject {
		return
	}
	if merged.Fields == nil {
		merged.Fields = make(map[string]Value)
	}
	for field, fieldValue := range source.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		sourceExplicit := isExplicitSObjectField(source, field)
		if _, exists := merged.Fields[field]; !exists {
			if sourceExplicit {
				setExplicitSObjectField(merged, field, fieldValue)
			} else {
				merged.Fields[field] = fieldValue
			}
			continue
		}
		if current := merged.Fields[field]; current.Kind == ValueNull && fieldValue.Kind != ValueNull {
			if sourceExplicit {
				setExplicitSObjectField(merged, field, fieldValue)
			} else {
				merged.Fields[field] = fieldValue
			}
		}
	}
}
func mergeSObjectAliasFieldsFromValue(value Value, ref uint64, merged *Value, seen map[uint64]bool) {
	if value.Ref == ref && value.Kind == ValueObject {
		mergeSObjectFieldsInto(merged, value)
		return
	}
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	case ValueList:
		for _, child := range value.List {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
		for _, child := range value.MapKeys {
			mergeSObjectAliasFieldsFromValue(child, ref, merged, seen)
		}
	}
}
func preserveMissingExplicitNulls(record *storage.Record, previous storage.Record) {
	if record == nil || len(previous.ExplicitNulls) == 0 {
		return
	}
	for field, isNull := range previous.ExplicitNulls {
		if !isNull {
			continue
		}
		if _, ok := record.GetField(field); ok || record.HasExplicitNull(field) {
			continue
		}
		if record.ExplicitNulls == nil {
			record.ExplicitNulls = make(map[string]bool)
		}
		record.ExplicitNulls[field] = true
	}
}
func preserveMissingSystemFields(record *storage.Record, original storage.SystemFields) {
	if record == nil {
		return
	}
	if record.System.OwnerID == "" {
		record.System.OwnerID = original.OwnerID
	}
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = original.CreatedDate
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = original.CreatedByID
	}
	if record.System.LastModifiedDate == "" {
		record.System.LastModifiedDate = original.LastModifiedDate
	}
	if record.System.LastModifiedByID == "" {
		record.System.LastModifiedByID = original.LastModifiedByID
	}
	if record.System.SystemModstamp == "" {
		record.System.SystemModstamp = original.SystemModstamp
	}
}
