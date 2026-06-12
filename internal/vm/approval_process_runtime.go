package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
)

func (vm *VM) executeApprovalProcess(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Approval.process expects request and optional allOrNone")
	}
	allOrNone := true
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Approval.process allOrNone expects Boolean")
		}
		allOrNone = args[1].Bool
	}
	request := args[0]
	if request.Kind != ValueObject {
		return Null, unsupportedCallError("Approval.process request type " + runtimeValueTypeName(request))
	}
	switch {
	case strings.EqualFold(request.Type, "Approval.ProcessSubmitRequest") || strings.EqualFold(request.Type, "ProcessSubmitRequest"):
		return vm.approvalProcessSubmitRequest(request, allOrNone)
	case strings.EqualFold(request.Type, "Approval.ProcessWorkitemRequest") || strings.EqualFold(request.Type, "ProcessWorkitemRequest"):
		return vm.approvalProcessWorkitemRequest(request, allOrNone)
	default:
		return Null, unsupportedCallError("Approval.process request type " + request.Type)
	}
}

func (vm *VM) approvalProcessSubmitRequest(request Value, allOrNone bool) (Value, error) {
	objectID := approvalRequestID(request, "ObjectId")
	if objectID.Kind == ValueNull {
		return approvalProcessMissingIDResult(allOrNone, "ObjectId")
	}
	result := approvalProcessResult(true, objectID, nil)
	result.Fields["instanceId"] = platformScalar("Id", "04g000000000001AAA")
	result.Fields["newWorkitemIds"] = List(platformScalar("Id", "04i000000000001AAA"))
	return result, nil
}

func (vm *VM) approvalProcessWorkitemRequest(request Value, allOrNone bool) (Value, error) {
	workitemID := approvalRequestID(request, "WorkitemId")
	if workitemID.Kind == ValueNull {
		return approvalProcessMissingIDResult(allOrNone, "WorkitemId")
	}
	result := approvalProcessResult(true, workitemID, nil)
	result.Fields["instanceId"] = platformScalar("Id", "04g000000000001AAA")
	result.Fields["newWorkitemIds"] = List()
	return result, nil
}

func approvalRequestID(request Value, field string) Value {
	_, value, ok := objectFieldValue(request, field)
	if !ok || value.Kind == ValueNull {
		return Null
	}
	return value
}

func approvalProcessMissingIDResult(allOrNone bool, field string) (Value, error) {
	errValue := databaseErrorValue(dml.Error{StatusCode: "REQUIRED_FIELD_MISSING", Message: field + " is required", Fields: []string{field}})
	if allOrNone {
		return Null, newExceptionError("DmlException", field+" is required")
	}
	return approvalProcessResult(false, Null, []Value{errValue}), nil
}

func approvalProcessResult(success bool, entityID Value, errors []Value) Value {
	result := Object("Approval.ProcessResult")
	result.Fields["success"] = Bool(success)
	result.Fields["entityId"] = entityID
	result.Fields["instanceId"] = Null
	result.Fields["newWorkitemIds"] = List()
	result.Fields["errors"] = List(errors...)
	return result
}
