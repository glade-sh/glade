package dml

import "errors"

// FlowActionInputError marks a rejected numeric input without choosing the
// caller-specific interview exception, DML row failure, or action result.
type FlowActionInputError struct{ Cause error }

func (e *FlowActionInputError) Error() string { return e.Cause.Error() }
func (e *FlowActionInputError) Unwrap() error { return e.Cause }

func flowActionFailure(name string, err error) error {
	var input *FlowActionInputError
	if errors.As(err, &input) {
		return &FlowActionInputError{Cause: dmlErrorf("CANNOT_EXECUTE_FLOW_TRIGGER", nil, "dml: flow action %s failed: %v", name, err)}
	}
	return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s failed: %v", name, err)
}

// FlowActionInputFailureResult exposes only numeric input rejections. Other
// automation and unsupported-engine failures retain their existing boundary.
func FlowActionInputFailureResult(err error) (Result, bool) {
	var input *FlowActionInputError
	if !errors.As(err, &input) {
		return Result{}, false
	}
	return failedResult("", err.Error(), "CANNOT_EXECUTE_FLOW_TRIGGER", nil), true
}
