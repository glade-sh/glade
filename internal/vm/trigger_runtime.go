package vm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

type eventBusTriggerContext struct {
	value         Value
	replayIDs     map[string]struct{}
	checkpoint    string
	hasCheckpoint bool
}

type eventBusTriggerRetryError struct {
	cause     error
	records   []storage.Record
	retries   int
	handled   bool
	retryable bool
}

func (e *eventBusTriggerRetryError) Error() string {
	return e.cause.Error()
}

func (e *eventBusTriggerRetryError) Unwrap() error {
	return e.cause
}

func (vm *VM) wrapEventBusTriggerFailure(trigger Trigger, records []storage.Record, err error) error {
	if !hasSuffixFold(trigger.Object, "__e") || vm.eventBusTriggerContext == nil {
		return err
	}
	retryRecords := append([]storage.Record(nil), records...)
	if vm.eventBusTriggerContext.hasCheckpoint {
		for index, record := range records {
			if recordFieldString(record, "ReplayId") == vm.eventBusTriggerContext.checkpoint {
				retryRecords = append([]storage.Record(nil), records[index+1:]...)
				break
			}
		}
	}
	retryable := false
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		retryable = strings.EqualFold(thrown.value.Type, "eventbus.RetryableException")
	}
	var runtime *RuntimeError
	if errors.As(err, &runtime) {
		retryable = retryable || strings.EqualFold(runtime.Type, "eventbus.RetryableException")
	}
	return &eventBusTriggerRetryError{
		cause: err, records: retryRecords,
		retries:   int(vm.eventBusTriggerContext.value.Fields["retries"].Int),
		handled:   retryable || vm.eventBusTriggerContext.hasCheckpoint,
		retryable: retryable,
	}
}

func dmlExceptionFromTriggerError(op string, err error) error {
	var thrown *apexThrowError
	message := err.Error()
	if errors.As(err, &thrown) {
		if detail := exceptionMessage(thrown.value); detail != "" {
			message = detail
		}
	} else {
		// Preserve fatal/uncatchable raw errors, including LimitException.
		// Method calls return the captured readonly failures as RuntimeErrors.
		readonlyMessage := message
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) {
			if runtimeErr.Type != "RuntimeError" {
				return err
			}
			readonlyMessage = runtimeErr.Message
		}
		switch readonlyMessage {
		case "System.FinalException: Record is read-only", "System.FinalException: Collection is read-only":
		default:
			return err
		}
	}
	// A trigger failure is a row-bearing DmlException,
	// including uncatchable readonly failures from a trigger context.
	wrapped := databaseDMLException(op, []dml.Result{{Error: message, StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}}, nil).(*apexThrowError)
	if thrown != nil {
		wrapped.value.Fields["__cause"] = thrown.value
		wrapped.stack = append([]callFrame(nil), thrown.stack...)
	}
	return wrapped
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
		storage.EnsureMutableObjectRecords(vm.Org, objectName)
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
func (vm *VM) hasAfterTriggerForDML(op string, records []storage.Record) bool {
	return vm.hasTriggerForDML(triggerTimingAfter, op, records)
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
func (vm *VM) runSummaryUpdateTriggers(engine *dml.Engine, allOrNone bool, rollback vmDMLRollbackPoint, result *Result) error {
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
				if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
					return rollbackErr
				}
			}
			return dmlExceptionFromTriggerError("update", err)
		}
		if hasDMLFailures(failures) {
			if allOrNone {
				if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
					return rollbackErr
				}
			}
			return fmt.Errorf("summary update trigger failed for %s: %s", objectName, failures[0].Error)
		}
		if err := vm.storeTriggerRecords(objectName, triggerRecords); err != nil {
			if allOrNone {
				if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
					return rollbackErr
				}
			}
			return err
		}
		if _, err := vm.runTriggers(triggerTimingAfter, "update", triggerRecords, before, result); err != nil {
			if allOrNone {
				if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
					return rollbackErr
				}
			}
			return dmlExceptionFromTriggerError("update", err)
		}
		if err := vm.applyDeferredAutomation(engine, triggerRecords, before, allOrNone, rollback, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) storeTriggerRecords(objectName string, records []storage.Record) error {
	if vm == nil || vm.Org == nil || len(records) == 0 {
		return nil
	}
	storage.EnsureMutableObjectRecords(vm.Org, objectName)
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
		merged := stored.Clone()
		if merged.Fields == nil {
			merged.Fields = make(map[string]storage.Value)
		}
		if merged.ExplicitNulls == nil {
			merged.ExplicitNulls = make(map[string]bool)
		}
		for field, value := range record.Fields {
			merged.Fields[field] = value.Clone()
			delete(merged.ExplicitNulls, field)
		}
		for field, isNull := range record.ExplicitNulls {
			if !isNull {
				continue
			}
			delete(merged.Fields, field)
			merged.ExplicitNulls[field] = true
		}
		merged.System = stored.System
		preserveReadOnlyCalculatedFields(object.Definition, &merged, stored)
		object.Records[storedID] = merged
	}
	vm.Org.Objects[objectName] = object
	return nil
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
		if record.System.OwnerID != "" {
			merged.System.OwnerID = record.System.OwnerID
		}
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
		merged.LoadedReferences = nil
		if len(record.LoadedReferences) > 0 {
			merged.LoadedReferences = make(map[string]storage.ID, len(record.LoadedReferences))
			for field, id := range record.LoadedReferences {
				merged.LoadedReferences[field] = id
			}
		}
		vm.populateCalculatedTriggerFields(&merged)
		out = append(out, merged)
	}
	return out
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
		vm.triggerMatchCache = newTriggerMatchCache()
	}
	cacheKey := strings.ToLower(object) + "|" + timing + "|" + op
	if cached, ok := vm.triggerMatchCache.load(cacheKey); ok {
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
	vm.triggerMatchCache.store(cacheKey, triggers)
	return triggers
}
func newTriggerMatchCache() *triggerMatchCache {
	return &triggerMatchCache{entries: make(map[string][]Trigger)}
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
func (vm *VM) runTrigger(trigger Trigger, records, oldRecords []storage.Record, result *Result) ([]dml.Result, error) {
	appendTraceLazy(result, "apex.trigger."+trigger.Name, "apex.trigger", func() map[string]any {
		return vm.traceTriggerArgs(trigger, len(records))
	})
	caller := vm.Globals
	callerClass := vm.currentClass
	callerNamespace := vm.currentNamespace
	callerMethod := vm.currentMethod
	callerTrigger := vm.currentTrigger
	callerEventBusContext := vm.eventBusTriggerContext
	callerTriggerGlobals := vm.triggerGlobals
	callerTriggerNamespaces := vm.activeTriggerNamespaces
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
	vm.currentMethod = Method{
		Name:       trigger.Name,
		ClassName:  trigger.Name,
		File:       trigger.File,
		APIVersion: trigger.APIVersion,
	}
	vm.currentTrigger = true
	vm.eventBusTriggerContext = nil
	if hasSuffixFold(trigger.Object, "__e") {
		replayIDs := make(map[string]struct{}, len(records))
		for _, record := range records {
			replayIDs[recordFieldString(record, "ReplayId")] = struct{}{}
		}
		vm.eventBusTriggerContext = &eventBusTriggerContext{
			value:     Object("eventbus.TriggerContext"),
			replayIDs: replayIDs,
		}
		retries := 0
		if vm.testContext != nil && len(records) > 0 {
			retries = vm.testContext.PlatformEventRetries[recordFieldString(records[0], "ReplayId")]
		}
		vm.eventBusTriggerContext.value.Fields["retries"] = Int(int64(retries))
	}
	if vm.currentNamespace != "" {
		vm.activeTriggerNamespaces = append(vm.activeTriggerNamespaces, vm.currentNamespace)
	}
	vm.callStack = append(vm.callStack, callFrame{Symbol: trigger.Name, File: trigger.File, Line: trigger.Line, Column: trigger.Column, APIVersion: trigger.APIVersion, SharingMode: "without sharing"})
	defer func() {
		vm.callStack = vm.callStack[:len(vm.callStack)-1]
		vm.Globals = caller
		vm.triggerGlobals = callerTriggerGlobals
		vm.activeTriggerNamespaces = callerTriggerNamespaces
		vm.currentClass = callerClass
		vm.currentNamespace = callerNamespace
		vm.currentMethod = callerMethod
		vm.currentTrigger = callerTrigger
		vm.eventBusTriggerContext = callerEventBusContext
		vm.currentStatement = callerStatement
		vm.hasStatement = callerHasStatement
	}()
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
		return nil, vm.wrapEventBusTriggerFailure(trigger, records, err)
	}
	if out.signal == signalThrow {
		if updated.Kind == ValueList {
			failures := dmlResultsFromSObjectErrors(records, updated.List)
			if hasDMLFailures(failures) {
				return failures, nil
			}
		}
		return nil, vm.wrapEventBusTriggerFailure(trigger, records, &apexThrowError{value: out.thrown, stack: append([]callFrame(nil), vm.callStack...)})
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
				preserveMissingRecordFields(&record, records[i])
				preserveMissingExplicitNulls(&record, records[i])
				preserveLoadedReferenceInput(&record, records[i], item)
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

func preserveMissingRecordFields(record *storage.Record, original storage.Record) {
	if record == nil || len(original.Fields) == 0 {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for field, value := range original.Fields {
		if _, ok := record.Fields[field]; ok {
			continue
		}
		if record.ExplicitNulls[field] {
			continue
		}
		if _, ok := record.GetField(field); ok {
			continue
		}
		if record.HasExplicitNull(field) {
			continue
		}
		record.Fields[field] = value.Clone()
	}
}
func (vm *VM) triggerObjectMatches(triggerObject, recordObject string) bool {
	if strings.EqualFold(triggerObject, recordObject) {
		return true
	}
	if vm != nil && vm.Org != nil {
		if resolvedTrigger, ok := vm.resolveObjectName(triggerObject); ok {
			if strings.EqualFold(resolvedTrigger, recordObject) {
				return true
			}
		}
		if resolvedRecord, ok := vm.resolveObjectName(recordObject); ok {
			if strings.EqualFold(resolvedRecord, triggerObject) {
				return true
			}
			if resolvedTrigger, ok := vm.resolveObjectName(triggerObject); ok && strings.EqualFold(resolvedTrigger, resolvedRecord) {
				return true
			}
		}
	}
	return strings.EqualFold(storage.StripAnyNamespaceToken(triggerObject), storage.StripAnyNamespaceToken(recordObject))
}
func triggerContext(trigger Trigger, records, oldRecords []storage.Record) map[string]Value {
	newValues := make([]Value, 0, len(records))
	newMap := Map()
	for _, record := range records {
		value := vmValueFromRecord(record)
		markTriggerSObject(&value)
		if trigger.Timing == triggerTimingAfter {
			value.Fields[sobjectReadOnlyField] = String("trigger record")
		}
		newValues = append(newValues, value)
		if record.ID != "" {
			key := platformScalar("Id", string(record.ID))
			encodedKey := mapKey(key)
			newMap.Map[encodedKey] = value
			newMap.MapKeys[encodedKey] = key
		}
	}
	oldValues := make([]Value, 0, len(oldRecords))
	oldMap := Map()
	for _, record := range oldRecords {
		value := vmValueFromRecord(record)
		markTriggerSObject(&value)
		value.Fields[sobjectReadOnlyField] = String("trigger record")
		oldValues = append(oldValues, value)
		if record.ID != "" {
			key := platformScalar("Id", string(record.ID))
			encodedKey := mapKey(key)
			oldMap.Map[encodedKey] = value
			oldMap.MapKeys[encodedKey] = key
		}
	}
	newListValue := Null
	newMapValue := Null
	if trigger.Operation == "insert" || trigger.Operation == "update" || trigger.Operation == "undelete" {
		newListValue = List(newValues...)
		newListValue.Type = "List<" + trigger.Object + ">"
		newListValue.Runtime = "List<SObject>"
		if trigger.Operation != "insert" || trigger.Timing == triggerTimingAfter {
			newMap.Type = "Map<Id," + trigger.Object + ">"
			newMap.Runtime = "Map<Id,SObject>"
			newMapValue = newMap
		}
	}
	oldListValue := Null
	oldMapValue := Null
	if trigger.Operation == "update" || trigger.Operation == "delete" {
		oldListValue = List(oldValues...)
		oldListValue.Type = "List<" + trigger.Object + ">"
		oldListValue.Runtime = "List<SObject>"
		oldMap.Type = "Map<Id," + trigger.Object + ">"
		oldMap.Runtime = "Map<Id,SObject>"
		oldMapValue = oldMap
	}
	ctx := map[string]Value{
		"Trigger.new":           newListValue,
		"Trigger.old":           oldListValue,
		"Trigger.newMap":        newMapValue,
		"Trigger.oldMap":        oldMapValue,
		"Trigger.isExecuting":   Bool(true),
		"Trigger.isBefore":      Bool(trigger.Timing == triggerTimingBefore),
		"Trigger.isAfter":       Bool(trigger.Timing == triggerTimingAfter),
		"Trigger.isInsert":      Bool(trigger.Operation == "insert"),
		"Trigger.isUpdate":      Bool(trigger.Operation == "update"),
		"Trigger.isDelete":      Bool(trigger.Operation == "delete"),
		"Trigger.isUndelete":    Bool(trigger.Operation == "undelete"),
		"Trigger.isUnDelete":    Bool(trigger.Operation == "undelete"),
		"Trigger.operationType": Value{Kind: ValueObject, Type: "TriggerOperation", Text: strings.ToUpper(trigger.Timing + "_" + trigger.Operation)},
		"Trigger.size":          Int(int64(len(records))),
	}
	return ctx
}

func (vm *VM) isTriggerContextList(value Value) bool {
	if vm == nil || value.Kind != ValueList || value.Ref == 0 {
		return false
	}
	for _, name := range []string{"Trigger.new", "Trigger.old"} {
		context := vm.triggerGlobals[name]
		if context.Kind == ValueList && context.Ref == value.Ref {
			return true
		}
	}
	return false
}

func (vm *VM) storeTriggerContextMap(name string, value Value) bool {
	if vm == nil || value.Kind != ValueMap || value.Ref == 0 {
		return false
	}
	// A called class may
	// mutate the active context map. Write it back to the trigger frame,
	// without treating the platform property as a class-local variable.
	for _, field := range []string{"Trigger.newMap", "Trigger.oldMap"} {
		current := vm.triggerGlobals[field]
		if strings.EqualFold(name, field) && current.Kind == ValueMap && current.Ref == value.Ref {
			vm.triggerGlobals[field] = value
			return true
		}
	}
	return false
}

func (vm *VM) recordsFromTriggerRetryInput(value Value) ([]storage.Record, []*Value, error) {
	vm.recordDMLRecordConversion(false)
	records := make([]storage.Record, 0, len(value.List))
	targets := make([]*Value, 0, len(value.List))
	for i := range value.List {
		// This private snapshot predates
		// the rolled-back insert. Merging live aliases would resurrect its
		// transient Id and reject a survivor as an existing insert input.
		record, err := vm.recordFromValue(&value.List[i])
		if err != nil {
			return nil, nil, err
		}
		records = append(records, record)
		targets = append(targets, &value.List[i])
	}
	return records, targets, nil
}

func (vm *VM) retryPartialTriggerDML(op string, input Value, externalIDField string, options dml.Options, result *Result, checkRecordAccess, checkApexIdentity bool, results []dml.Result, rollback vmDMLRollbackPoint, attempt int) ([]dml.Result, error) {
	// Complete the first trigger cycle,
	// roll back its writes, then rerun both events on the surviving inputs.
	// Apex static observations remain visible across these storage rollbacks.
	if err := vm.restoreDMLRollbackPoint(rollback); err != nil {
		return nil, err
	}
	if !hasDMLSuccess(results) {
		return results, nil
	}
	if attempt >= 2 {
		message := "Too many retries of batch save in the presence of Apex triggers with failures: when triggers are present partial save requires that some subset of rows save without any errors in order to avoid inconsistent side effects from those triggers. Number of retries: 2"
		code := "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"
		if op == "delete" {
			code = "DELETE_FAILED"
		} else if op == "undelete" {
			code = "UNDELETE_FAILED"
		}
		for i, saved := range results {
			if saved.Success {
				results[i] = dmlFailure(saved.ID, message, code, nil)
			}
		}
		return results, nil
	}
	// A partial trigger failure with survivors necessarily has a list input.
	// Detach its shape while retaining row references, so retry Id writes update
	// caller aliases without replacing their full list with the survivor list.
	next := input
	next.Ref = 0
	next.List = make([]Value, 0, len(input.List))
	for i, row := range input.List {
		if i < len(results) && results[i].Success {
			next.List = append(next.List, row)
		}
	}
	retried, err := vm.applyDMLWithAccessPolicy(op, next, false, externalIDField, options, result, checkRecordAccess, checkApexIdentity, attempt+1)
	if err != nil {
		return nil, err
	}
	return mergeDMLResults(results, retried), nil
}

func snapshotTriggerDMLInput(value Value, targets []*Value) Value {
	if value.Kind != ValueList {
		return cloneValuePreserveRefs(value)
	}
	out := value
	out.List = make([]Value, 0, len(targets))
	for _, target := range targets {
		if target != nil {
			out.List = append(out.List, cloneValuePreserveRefs(*target))
		}
	}
	return out
}

func (vm *VM) applyTriggerDMLChunks(op string, value Value, targets []*Value, allOrNone bool, externalIDField string, options dml.Options, result *Result, checkRecordAccess, checkApexIdentity bool) ([]dml.Result, error) {
	// Only insert/update split here, and each
	// chunk completes its before/after cycle before the next chunk begins.
	input := snapshotTriggerDMLInput(value, targets)
	rollback := vm.beginDMLRollbackPoint(true, false)
	defer vm.finishDMLRollbackPoint(rollback)
	results := make([]dml.Result, 0, len(input.List))
	for start := 0; start < len(input.List); start += 200 {
		chunk := input
		chunk.Ref = 0
		chunk.List = input.List[start:min(start+200, len(input.List))]
		// The outer call already counted the full DML operation. Attempt zero
		// gives this chunk its own native two-retry budget without recounting it.
		saved, err := vm.applyDMLWithAccessPolicy(op, chunk, allOrNone, externalIDField, options, result, checkRecordAccess, checkApexIdentity, 0)
		if err != nil {
			if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
				return nil, rollbackErr
			}
			return nil, err
		}
		results = append(results, saved...)
		if allOrNone && hasDMLFailures(saved) {
			if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
				return nil, rollbackErr
			}
			return results, nil
		}
	}
	return results, nil
}
