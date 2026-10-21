package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) allowTestHistoryInsert() bool {
	if vm.testContext == nil || !apexversion.AtLeast(vm.currentMethod.APIVersion, 62) || apexversion.AtLeast(vm.currentMethod.APIVersion, 67) {
		return false
	}
	// A newer caller or intermediate helper retains the native rejection even
	// when the final insert executes in an older source method.
	for _, frame := range vm.callStack {
		if apexversion.AtLeast(frame.APIVersion, 67) {
			return false
		}
	}
	return true
}

func (vm *VM) generatedHistoryDMLBoundary(operation string, records []storage.Record) error {
	for _, record := range records {
		name, ok := storage.ResolveObjectName(*vm.Org, record.Object)
		if !ok || !storage.IsGeneratedHistoryObject(vm.Org.Objects[name].Definition) {
			continue
		}
		if operation != "insert" && operation != "update" && operation != "delete" {
			return unsupportedCallError("generated history " + operation)
		}
		if operation == "insert" && vm.allowTestHistoryInsert() {
			continue
		}
		return newExceptionError("System.TypeException", "DML operation "+strings.ToUpper(operation)+" not allowed on "+name)
	}
	return nil
}
