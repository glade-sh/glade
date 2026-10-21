package vm

import "github.com/glade-sh/glade/internal/dml"

// A single-record statement insert exposes the deferred Delete Records failure
// through the same catchable row exception as immediate DML validation.
func (vm *VM) flowCoreStatementInsertError(op string, value Value, err error) error {
	if op != "insert" || value.Kind != ValueObject {
		return err
	}
	if failure, ok := dml.FlowCustomErrorFailureResult(err); ok {
		return statementDMLException(op, []dml.Result{failure}, vm.dmlExceptionObjectTypes(value))
	}
	if failure, ok := dml.FlowSubflowOutputFailureResult(err); ok {
		return statementDMLException(op, []dml.Result{failure}, vm.dmlExceptionObjectTypes(value))
	}
	failure, ok := dml.FlowRecordDeleteFailureResult(err)
	if !ok {
		return err
	}
	return statementDMLException(op, []dml.Result{failure}, vm.dmlExceptionObjectTypes(value))
}
