package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

var taskDMLPolicy = objectDMLPolicy{
	storedDMLDerivedFields:    applyTaskStatusFlagFields,
	transientDMLDerivedFields: []string{"IsClosed"},
}

// Task IsClosed changes after before-trigger execution, without populating callers.
func applyTaskStatusFlagFields(vm *VM, record *storage.Record) {
	if vm == nil || vm.Org == nil || record == nil {
		return
	}
	status := storageRecordStringField(*record, "Status")
	if status == "" {
		return
	}
	statuses, ok := vm.Org.Objects["TaskStatus"]
	if !ok {
		return
	}
	for _, row := range statuses.Records {
		if !strings.EqualFold(status, storageRecordStringField(row, "MasterLabel")) &&
			!strings.EqualFold(status, storageRecordStringField(row, "ApiName")) {
			continue
		}
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields["IsClosed"] = storage.BooleanValue(storageRecordBoolField(row, "IsClosed"))
		return
	}
}
