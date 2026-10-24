package vm

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

type flowAsyncPathJob struct {
	objectName string
	rule       storage.FlowRule
	path       storage.FlowPath
	recordID   storage.ID
	sequence   uint64
}

func (vm *VM) enqueueFlowAsyncPath(objectName string, rule storage.FlowRule, path storage.FlowPath, id storage.ID) {
	// Async record paths do not execute in Apex tests, including stopTest and
	// paths submitted by an asynchronous test callback.
	if vm.testContext != nil {
		return
	}
	vm.flowAsyncSeq++
	vm.enqueueAsyncJob(AsyncJob{
		Kind: "FlowAsyncPath",
		Name: rule.Name + "/" + path.Name,
		FlowPath: &flowAsyncPathJob{
			objectName: objectName,
			rule:       rule,
			path:       path,
			recordID:   id,
			sequence:   vm.flowAsyncSeq,
		},
	})
}

func (vm *VM) runFlowAsyncPath(job AsyncJob, result *Result) error {
	payload := job.FlowPath
	_, err := vm.withAsyncLimitWindow(func() (Value, error) {
		return vm.withAsyncKind("FlowAsyncPath", func() (Value, error) {
			rollback := vm.beginDMLRollbackPoint(true, false)
			defer vm.finishDMLRollbackPoint(rollback)
			engine := vm.newDeferredAutomationDMLEngine(result)
			if err := engine.RunFlowPath(payload.objectName, payload.rule, payload.path, payload.recordID); err != nil {
				if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
					return Null, rollbackErr
				}
				return Null, err
			}
			return Null, nil
		})
	})
	return err
}

func (vm *VM) configureFlowAsyncPathEngine(engine *dml.Engine, result *Result) {
	engine.InvocableFlowRecordCreator = func(record storage.Record) (dml.Result, error) {
		return vm.insertInvocableFlowRecord(record, result)
	}
	engine.InvocableFlowRecordUpdater = func(record storage.Record) (dml.Result, error) {
		return vm.mutateInvocableFlowRecord("update", record, result)
	}
}

func (vm *VM) recordFlowPathSavepoint(id string) {
	if vm.flowPathSavepoints == nil {
		vm.flowPathSavepoints = make(map[string]uint64)
	}
	vm.flowPathSavepoints[id] = vm.flowAsyncSeq
}

func (vm *VM) rollbackFlowPathSavepoint(id string) {
	if mark, ok := vm.flowPathSavepoints[id]; ok {
		vm.rollbackFlowPathsAfter(mark)
	}
}

func (vm *VM) rollbackFlowPathsAfter(mark uint64) {
	kept := vm.localAsyncJobs[:0]
	for _, job := range vm.localAsyncJobs {
		if job.FlowPath == nil || job.FlowPath.sequence <= mark {
			kept = append(kept, job)
		}
	}
	clear(vm.localAsyncJobs[len(kept):])
	vm.localAsyncJobs = kept
}
