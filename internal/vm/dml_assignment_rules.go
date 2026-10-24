package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func assignmentRuleRequested(object string, selection *storage.AssignmentRuleSelection) bool {
	return (strings.EqualFold(object, "Lead") || strings.EqualFold(object, "Case")) &&
		selection != nil && (selection.UseDefault || selection.ID != "")
}

func (vm *VM) hasAssignmentRuleUpdateTriggers(records []storage.Record, targets []*Value, options dml.Options) bool {
	if !vm.hasTriggerForDML(triggerTimingBefore, "update", records) && !vm.hasAfterTriggerForDML("update", records) {
		return false
	}
	for i, record := range records {
		selection := options.AssignmentRule
		if selection == nil {
			selection = record.AssignmentRule
			if selection == nil && i < len(targets) && targets[i] != nil {
				// Per-record options have not yet been copied to storage inputs.
				// Their validation remains at the ordinary DML option boundary.
				selection, _ = databaseAssignmentRuleOptions(targets[i].Fields[sobjectDMLOptionsField])
			}
		}
		if assignmentRuleRequested(record.Object, selection) {
			return true
		}
	}
	return false
}

// Native assignment rules transfer ownership through an additional update
// trigger pass. It belongs to the original DML statement and adds no DML charge.
func (vm *VM) applyAssignmentRuleUpdates(engine *dml.Engine, inputs []storage.Record, results []dml.Result, result *Result) ([]dml.Result, error) {
	defer func() { engine.Options.AssignmentRule = nil }()
	var before, after []storage.Record
	var indices []int
	for i, input := range inputs {
		if i >= len(results) || !results[i].Success {
			continue
		}
		selection := engine.Options.AssignmentRule
		if selection == nil {
			selection = input.AssignmentRule
		}
		if !assignmentRuleRequested(input.Object, selection) {
			continue
		}
		input.ID = results[i].ID
		owner, err := engine.AssignmentRuleOwner(input)
		if err != nil {
			return nil, err
		}
		_, stored, exists := storage.LookupRecordByID(vm.Org.Objects[input.Object].Records, input.ID)
		if !exists || (owner != "" && storage.IDsEqual(owner, stored.System.OwnerID)) {
			continue
		}
		// An enabled unmatched or unknown rule still takes the update pass.
		// A matching rule whose assignee already owns the row does not.
		next := stored.Clone()
		next.AssignmentRule = nil
		if owner != "" {
			next.System.OwnerID = owner
			if _, exists := next.Fields["OwnerId"]; exists {
				next.Fields["OwnerId"] = storage.IDValue(owner)
			}
		}
		before = append(before, stored.Clone())
		after = append(after, next)
		indices = append(indices, i)
	}
	if len(after) == 0 {
		return nil, nil
	}
	engine.Options.AssignmentRule = nil
	failures := make([]dml.Result, len(inputs))
	for start := 0; start < len(after); start += 200 {
		end := min(start+200, len(after))
		saved, err := vm.applyAssignmentRuleUpdateChunk(engine, after[start:end], before[start:end], result, 0)
		if err != nil {
			return nil, err
		}
		for i, row := range saved {
			if !row.Success && row.Error != "" {
				failures[indices[start+i]] = row
			}
		}
	}
	return failures, nil
}

func (vm *VM) applyAssignmentRuleUpdateChunk(engine *dml.Engine, inputs, before []storage.Record, result *Result, attempt int) ([]dml.Result, error) {
	rollback := vm.beginDMLRollbackPoint(true, false)
	defer vm.finishDMLRollbackPoint(rollback)
	working := make([]storage.Record, len(inputs))
	for i, record := range inputs {
		working[i] = record.Clone()
	}
	failures, working, err := vm.runTriggersByObject(triggerTimingBefore, "update", working, before, result)
	if err != nil {
		return nil, dmlExceptionFromTriggerError("update", err)
	}
	updateBefore := before
	if hasDMLFailures(failures) {
		working, updateBefore, _ = filterDMLInputs(working, before, nil, failures)
	}
	vm.applyBeforeDMLDerivedFields(working)
	updated := engine.Update(working)
	if hasDMLFailures(updated) {
		return nil, databaseDMLException("update", updated, nil)
	}
	after, err := vm.afterRecords("update", working, updated)
	if err != nil {
		return nil, err
	}
	afterFailures, _, err := vm.runTriggersByObject(triggerTimingAfter, "update", after, updateBefore, result)
	if err != nil {
		return nil, dmlExceptionFromTriggerError("update", err)
	}
	updated = mergeAfterTriggerDMLResults(updated, afterFailures)
	if hasDMLFailures(failures) {
		updated = mergeDMLResults(failures, updated)
	}
	if !hasDMLFailures(updated) {
		return updated, nil
	}
	// This inner save retries its survivors even when the outer statement is
	// atomic. Its rollback retains the original save and Apex static state.
	if err := vm.restoreDMLRollbackPoint(rollback); err != nil {
		return nil, err
	}
	if !hasDMLSuccess(updated) {
		return updated, nil
	}
	if attempt >= 2 {
		message := "Too many retries of batch save in the presence of Apex triggers with failures: when triggers are present partial save requires that some subset of rows save without any errors in order to avoid inconsistent side effects from those triggers. Number of retries: 2"
		for i, saved := range updated {
			if saved.Success {
				updated[i] = dmlFailure(saved.ID, message, "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil)
			}
		}
		return updated, nil
	}
	next, nextBefore, _ := filterDMLInputs(inputs, before, nil, updated)
	// The failed pass may have populated engine indexes from rolled-back rows.
	retryEngine := vm.newDeferredAutomationDMLEngine(result)
	retryEngine.Options = engine.Options
	retryEngine.PriorRecords = dmlPriorRecordsByID(nextBefore)
	retried, err := vm.applyAssignmentRuleUpdateChunk(&retryEngine, next, nextBefore, result, attempt+1)
	if err != nil {
		return nil, err
	}
	return mergeDMLResults(updated, retried), nil
}
