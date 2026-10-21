package vm

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) normalizeBeforeDMLEmailFields(records []storage.Record) {
	for i := range records {
		objectName, ok := vm.resolveObjectName(records[i].Object)
		if !ok {
			continue
		}
		definition := vm.describePreparedDefinition(objectName, vm.Org.Objects[objectName].Definition)
		dml.NormalizeEmailFields(definition, vm.Org.Namespace, &records[i])
	}
}
