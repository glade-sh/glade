package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/dml"
)

const databaseAsyncLocatorField = "__glade_database_async_locator"

// Named Account-list callback observations cover API59/60 and every source API61–68.
// Modern validation also precedes startTest. An Account-backed list held as
// List<SObject> has a mode-specific failure inside the modern test window.
func (vm *VM) capturedAccountAsyncInsert(op string, args []Value, callback Value) (Value, bool, error) {
	if vm == nil || vm.Org == nil || vm.testContext == nil || vm.testContext.Stopped || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || vm.currentTrigger || vm.testContext.RunAsDepth != 0 || op != "insert" || len(args) != 3 || callback.Kind == ValueNull || !isDatabaseAccessLevelValue(args[2]) {
		return Null, false, nil
	}
	version, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok || version < 59 || version > 68 {
		return Null, false, nil
	}
	if !vm.testContext.Started && version < 61 {
		return Null, false, nil
	}
	mode := databaseAccessLevelSecurityMode(args[2])
	if mode != "USER_MODE" && mode != "SYSTEM_MODE" {
		return Null, false, nil
	}
	input := args[0]
	element, closed := collectionElementType(runtimeObjectType(input))
	concreteAccount := version >= 61 && vm.testContext.Started && strings.EqualFold(element, "Account")
	if input.Kind != ValueList || len(input.List) != 1 || !closed || (!strings.EqualFold(element, "SObject") && !concreteAccount) {
		return Null, false, nil
	}
	record := input.List[0]
	if record.Kind != ValueObject || record.classInstance || record.projectClass || sObjectIDFromFields(record.Fields) != "" {
		return Null, false, nil
	}
	name, known := vm.resolveObjectName(runtimeObjectType(record))
	if !known || name != "Account" {
		return Null, false, nil
	}
	if version >= 60 {
		if !concreteAccount {
			return Null, true, newExceptionError("System.TypeException", "Invalid entity type in input for this operation")
		}
		if mode == "SYSTEM_MODE" {
			return Null, true, newExceptionError("System.TypeException", "Asynchronous DML not allowed on Account")
		}
	} else {
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return Null, true, err
		}
		if err := vm.incrementLimit("dmlRows", 1); err != nil {
			return Null, true, err
		}
	}
	failure := dmlFailure("", "Asynchronous DML not allowed on Account", "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil)
	value := fieldKeyDMLResultsValue("insert", input, []dml.Result{failure})
	for _, row := range value.List {
		// Native serialization omits absent identity and internal error summaries.
		delete(row.Fields, "id")
		delete(row.Fields, "error")
		errors := row.Fields["errors"]
		for _, detail := range errors.List {
			delete(detail.Fields, "extendedErrorDetails")
		}
		row.Fields[databaseAsyncLocatorField] = Null
	}
	return value, true, nil
}
