package vm

import "github.com/glade-sh/glade/internal/dml"

// Statement inserts expose captured deferred Flow failures through the same
// catchable row exception as immediate DML validation.
func (vm *VM) flowCoreStatementInsertError(op string, value Value, err error) error {
	if op != "insert" || (value.Kind != ValueObject && value.Kind != ValueList) {
		return err
	}
	// Native flow_followups AO11/AO12 cover an Apex action fault connector
	// ending in Custom Error for both singleton and multi-record insert lists.
	if failure, ok := dml.FlowCustomErrorFailureResult(err); ok {
		return statementDMLException(op, []dml.Result{failure}, vm.dmlExceptionObjectTypes(value))
	}
	if value.Kind != ValueObject {
		return err
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
