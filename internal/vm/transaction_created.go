package vm

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

type recordCreationCheckpoint struct {
	records *dml.RecordCreations
	mark    int
}

func (vm *VM) recordCreationState() *dml.RecordCreations {
	if vm.recordCreations == nil {
		vm.recordCreations = &dml.RecordCreations{}
	}
	return vm.recordCreations
}

func (vm *VM) lookupHistoryState() *storage.LookupHistoryScope {
	if vm.lookupHistoryScope == nil {
		vm.lookupHistoryScope = storage.NewLookupHistoryScope()
	}
	return vm.lookupHistoryScope
}

func (vm *VM) creationCheckpoint() recordCreationCheckpoint {
	records := vm.recordCreationState()
	return recordCreationCheckpoint{records: records, mark: records.Mark()}
}

func (checkpoint recordCreationCheckpoint) restore() {
	checkpoint.records.Rollback(checkpoint.mark)
}

func (vm *VM) beginRecordCreationTransaction() func() {
	previousCallouts := vm.calloutPendingWork
	vm.calloutPendingWork = calloutPendingWork{}
	previous := vm.recordCreations
	previousLookupScope := vm.lookupHistoryScope
	vm.recordCreations = &dml.RecordCreations{}
	vm.lookupHistoryScope = storage.NewLookupHistoryScope()
	return func() {
		vm.calloutPendingWork = previousCallouts
		vm.recordCreations = previous
		vm.lookupHistoryScope = previousLookupScope
	}
}
