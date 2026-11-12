package vm

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// withoutRunningProcessRecords drops records of a running process's cohort. A
// save of such a record from inside the run fires its triggers only: no
// workflow rule, process or after-save flow (process_builder_shapes seq, bmp
// and rec rows, API 62.0 and 67.0). Other records of the object run in full
// (sib rows).
func (vm *VM) withoutRunningProcessRecords(records, oldRecords []storage.Record) ([]storage.Record, []storage.Record) {
	if len(vm.runningProcessRecords) == 0 {
		return records, oldRecords
	}
	keptRecords := make([]storage.Record, 0, len(records))
	keptOld := make([]storage.Record, 0, len(oldRecords))
	for i, record := range records {
		if vm.runningProcessRecords[record.ID] > 0 {
			continue
		}
		keptRecords = append(keptRecords, record)
		if i < len(oldRecords) {
			keptOld = append(keptOld, oldRecords[i])
		}
	}
	return keptRecords, keptOld
}

// applyProcessObjectAutomation owns a save whose object has an active process.
// Native order (ord, ordbulk rows): workflow rules, their update trigger refire
// for the whole cohort, processes, then after-save flows. Other saves return
// false and keep the ordinary path.
func (vm *VM) applyProcessObjectAutomation(engine *dml.Engine, records, oldRecords []storage.Record, allOrNone bool, rollback vmDMLRollbackPoint, result *Result) (bool, error) {
	found, previous := false, ""
	for _, record := range records {
		if record.Object == previous || record.ID == "" {
			continue
		}
		previous = record.Object
		if engine.HasActiveProcess(record.Object) {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	sideEffects := vm.snapshotSideEffects()
	fail := func(err error) error {
		if allOrNone {
			if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
				return rollbackErr
			}
			vm.restoreSideEffects(sideEffects)
		}
		return err
	}
	// Keep the ordinary path for other objects of a mixed list, in list order.
	var order []string
	byObject := make(map[string][]int)
	for i, record := range records {
		if record.Object == "" || record.ID == "" {
			continue
		}
		objectName := record.Object
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			objectName = canonical
		}
		if _, seen := byObject[objectName]; !seen {
			order = append(order, objectName)
		}
		byObject[objectName] = append(byObject[objectName], i)
	}
	for _, objectName := range order {
		cohort := make([]storage.Record, 0, len(byObject[objectName]))
		old := make([]storage.Record, 0, len(byObject[objectName]))
		for _, i := range byObject[objectName] {
			cohort = append(cohort, records[i])
			if i < len(oldRecords) {
				old = append(old, oldRecords[i])
			} else {
				old = append(old, records[i])
			}
		}
		if !engine.HasActiveProcess(objectName) {
			if err := vm.applyDeferredAutomation(engine, cohort, old, allOrNone, rollback, result); err != nil {
				return true, err
			}
			continue
		}
		if err := vm.runProcessCohort(engine, objectName, cohort, old, allOrNone, rollback, result); err != nil {
			return true, fail(err)
		}
	}
	return true, nil
}

func (vm *VM) runProcessCohort(engine *dml.Engine, objectName string, cohort, old []storage.Record, allOrNone bool, rollback vmDMLRollbackPoint, result *Result) error {
	if vm.runningProcessRecords == nil {
		vm.runningProcessRecords = make(map[storage.ID]int)
	}
	ids := make([]storage.ID, 0, len(cohort))
	for _, record := range cohort {
		ids = append(ids, record.ID)
		vm.runningProcessRecords[record.ID]++
	}
	defer func() {
		for _, id := range ids {
			if vm.runningProcessRecords[id]--; vm.runningProcessRecords[id] <= 0 {
				delete(vm.runningProcessRecords, id)
			}
		}
	}()
	previousUpdater, previousCreator, previousInvocable, previousFailure := engine.ProcessRecordUpdater, engine.ProcessRecordCreator, engine.InvocableFlowRecordUpdater, engine.ProcessActionFailure
	previousMixedSuffix := engine.CapturedMixedAfterSaveErrorSuffix
	engine.CapturedMixedAfterSaveErrorSuffix = func() string {
		if vm.testContext == nil {
			return "  Error ID: " + strconv.Itoa(vm.nextProcessErrorID())
		}
		return " "
	}
	engine.ProcessActionFailure = vm.processApexActionFailure
	engine.ProcessRecordUpdater = func(records []storage.Record) ([]dml.Result, error) {
		return vm.updateProcessRecords(records, result)
	}
	engine.ProcessRecordCreator = func(records []storage.Record) ([]dml.Result, error) {
		list := typedList("List<SObject>")
		for _, record := range records {
			value := vmValueFromRecord(record)
			for field := range record.Fields {
				markExplicitSObjectField(&value, field)
			}
			list.List = append(list.List, value)
		}
		return vm.applyDML("insert", list, true, "", dml.Options{}, result)
	}
	engine.InvocableFlowRecordUpdater = func(record storage.Record) (dml.Result, error) {
		return vm.mutateInvocableFlowRecord("update", record, result)
	}
	defer func() {
		engine.ProcessRecordUpdater, engine.ProcessRecordCreator, engine.InvocableFlowRecordUpdater, engine.ProcessActionFailure = previousUpdater, previousCreator, previousInvocable, previousFailure
		engine.CapturedMixedAfterSaveErrorSuffix = previousMixedSuffix
	}()

	created := engine.CreatedAutomationIDs(ids)
	var updatedIDs []storage.ID
	var updatedOld []storage.Record
	// Trigger.old of the workflow refire is the record before the save, or on
	// create the record as workflow found it (bmp, ordsaves rows).
	for i, record := range cohort {
		appendTrace(result, "apex.automation.apply", "apex.automation", map[string]any{"object": objectName, "id": string(record.ID)})
		found := old[i]
		if object, ok := vm.Org.Objects[objectName]; ok {
			if _, stored, exists := storage.LookupRecordByID(object.Records, record.ID); exists && created[record.ID] {
				found = stored.Clone()
			}
		}
		updated, err := engine.ApplyWorkflowPhase(objectName, record.ID)
		if err != nil {
			return err
		}
		if updated {
			updatedIDs = append(updatedIDs, record.ID)
			updatedOld = append(updatedOld, found)
		}
	}
	if err := vm.refireAutomationUpdateTriggersFor(objectName, updatedIDs, updatedOld, allOrNone, rollback, result); err != nil {
		return err
	}
	// Descendant automation writes do not inherit the caller's DML header.
	assignment := engine.Options.AssignmentRule
	engine.Options.AssignmentRule = nil
	defer func() { engine.Options.AssignmentRule = assignment }()
	engine.RestoreCreatedAutomation(created)
	processErr := engine.RunProcesses(objectName, cohort)
	var failure *dml.ProcessFailure
	if processErr != nil && !errors.As(processErr, &failure) {
		return processErr
	}
	// A failed element stops the process. After-save flows still run for the
	// other records before the failure surfaces (fanmulti rows).
	flowCohort := cohort
	if failure != nil {
		flowCohort = make([]storage.Record, 0, len(cohort))
		for _, record := range cohort {
			if !failure.Failed[record.ID] {
				flowCohort = append(flowCohort, record)
			}
		}
	}
	// Their update triggers refire once for the cohort. Trigger.old is the record
	// as the flow found it (asu, asubulk rows).
	before := make([]storage.Record, len(flowCohort))
	if object, ok := vm.Org.Objects[objectName]; ok {
		for i, record := range flowCohort {
			if _, stored, found := storage.LookupRecordByID(object.Records, record.ID); found {
				before[i] = stored.Clone()
			}
		}
	}
	var refireIDs []storage.ID
	var refireOld []storage.Record
	if err := engine.ApplyAfterSaveFlowPhase(objectName, flowCohort, func(i int, outcome dml.AutomationResult) error {
		if outcome.WorkflowUpdated || outcome.FlowUpdated {
			refireIDs = append(refireIDs, flowCohort[i].ID)
			refireOld = append(refireOld, before[i])
		}
		return nil
	}); err != nil {
		return err
	}
	if err := vm.refireAutomationUpdateTriggersFor(objectName, refireIDs, refireOld, allOrNone, rollback, result); err != nil {
		return err
	}
	return processErr
}

// processApexActionFailure names an exception an Apex action left unhandled
// (thr rows: "An Apex error occurred: FernThrow.BoomException: boom action "; the
// anonymous route adds an Error ID that a test does not).
func (vm *VM) processApexActionFailure(err error) string {
	var failure *RuntimeError
	if !errors.As(err, &failure) {
		return ""
	}
	detail := apexFailureText(failure)
	if vm.testContext == nil {
		detail += "  Error ID: " + strconv.Itoa(vm.nextProcessErrorID())
	}
	return detail
}

// apexFailureText renders an unhandled Apex exception as "Type: message".
func apexFailureText(failure *RuntimeError) string {
	typeName := failure.Type
	if _, standard := typesys.StandardExceptionParent(typeName); standard && !strings.HasPrefix(typeName, "System.") {
		typeName = "System." + typeName
	}
	message := failure.Message
	if failure.exceptionMessage != nil {
		message = *failure.exceptionMessage
	}
	return typeName + ": " + message
}

func (vm *VM) nextProcessErrorID() int {
	vm.processErrorSequence++
	return vm.processErrorSequence
}

// updateProcessRecords saves one DML of a process element as a partial save.
// Native rolls the attempt back and saves the surviving rows again, so their
// triggers fire again (fan, mergep, flow2fail rows). Results keep input positions.
func (vm *VM) updateProcessRecords(records []storage.Record, result *Result) ([]dml.Result, error) {
	out := make([]dml.Result, len(records))
	positions := make([]int, len(records))
	for i := range records {
		positions[i] = i
	}
	for {
		list := typedList("List<SObject>")
		for _, position := range positions {
			record := records[position]
			value := vmValueFromRecord(record)
			for field := range record.Fields {
				markExplicitSObjectField(&value, field)
			}
			for field, isNull := range record.ExplicitNulls {
				if isNull {
					markExplicitSObjectField(&value, field)
				}
			}
			list.List = append(list.List, value)
		}
		point := vm.beginDMLRollbackPoint(true, true)
		saved, err := vm.applyDML("update", list, false, "", dml.Options{}, result)
		if err != nil {
			vm.finishDMLRollbackPoint(point)
			return nil, err
		}
		survivors := make([]int, 0, len(positions))
		for i, position := range positions {
			if i < len(saved) {
				out[position] = saved[i]
				if saved[i].Success {
					survivors = append(survivors, position)
				}
			}
		}
		if len(survivors) == len(positions) || len(survivors) == 0 {
			vm.finishDMLRollbackPoint(point)
			return out, nil
		}
		if err := vm.restoreDMLRollbackPoint(point); err != nil {
			vm.finishDMLRollbackPoint(point)
			return nil, err
		}
		vm.finishDMLRollbackPoint(point)
		positions = survivors
	}
}

// processFailureDMLResults turns a process failure into row results for the
// records whose interviews failed. Native err rows: the row carries
// CANNOT_EXECUTE_FLOW_TRIGGER and partial save retries the other rows.
func processFailureDMLResults(op string, results []dml.Result, records []storage.Record, err error) ([]dml.Result, bool) {
	var failure *dml.ProcessFailure
	if !errors.As(err, &failure) {
		return nil, false
	}
	failures := append([]dml.Result(nil), results...)
	index := 0
	for i, saved := range failures {
		if !saved.Success {
			continue
		}
		id := saved.ID
		if index < len(records) && id == "" {
			id = records[index].ID
		}
		index++
		if !failure.Failed[id] {
			continue
		}
		row := failure.Result
		if specific, ok := failure.RowResults[id]; ok {
			row = specific
		}
		row.Created, row.UpsertCreated = saved.Created, saved.UpsertCreated
		if op == "update" || op == "upsert" && !saved.Created {
			row.ID = saved.ID
		}
		failures[i] = row
	}
	return failures, true
}

// processUpsertRetry asks applyDMLWithAccessPolicy to rerun a partial upsert on
// its surviving rows, as insert and update do (errups rows).
type processUpsertRetry struct{ results []dml.Result }

func (*processUpsertRetry) Error() string { return "process failure: retry partial upsert" }

func (vm *VM) retryProcessUpsert(input Value, results []dml.Result, externalIDField string, options dml.Options, result *Result, checkRecordAccess, checkApexIdentity bool, resolvedMode string, attempt int) ([]dml.Result, error) {
	if !hasDMLSuccess(results) {
		return results, nil
	}
	next := input
	next.Ref = 0
	next.List = make([]Value, 0, len(input.List))
	for i, row := range input.List {
		if i < len(results) && results[i].Success {
			next.List = append(next.List, row)
		}
	}
	retried, err := vm.applyDMLWithAccessPolicy("upsert", next, false, externalIDField, options, result, checkRecordAccess, checkApexIdentity, resolvedMode, attempt+1)
	if err != nil {
		return nil, err
	}
	return mergeDMLResults(results, retried), nil
}
