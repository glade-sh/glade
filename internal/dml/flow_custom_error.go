package dml

import "errors"

type flowCustomErrorFailure struct{ cause error }

func (failure *flowCustomErrorFailure) Error() string { return failure.cause.Error() }
func (failure *flowCustomErrorFailure) Unwrap() error { return failure.cause }

// FlowCustomErrorFailureResult identifies explicit Custom Error nodes without
// converting unsupported or unrelated interview failures into row errors.
func FlowCustomErrorFailureResult(err error) (Result, bool) {
	var failure *flowCustomErrorFailure
	if !errors.As(err, &failure) {
		return Result{}, false
	}
	return resultFromError("", failure.cause), true
}
