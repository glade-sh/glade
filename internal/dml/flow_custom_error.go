package dml

import "errors"

type flowCustomErrorFailure struct {
	cause           error
	capturedFormula bool
}

type flowActionBatchCustomError struct {
	cause error
	count int
}

func (failure *flowActionBatchCustomError) Error() string { return failure.cause.Error() }
func (failure *flowActionBatchCustomError) Unwrap() error { return failure.cause }

// FlowActionBatchCustomErrorResults identifies the captured all-admitted
// action-fault connector, retaining one result for each input index.
func FlowActionBatchCustomErrorResults(err error) ([]Result, bool) {
	var failure *flowActionBatchCustomError
	if !errors.As(err, &failure) {
		return nil, false
	}
	result, ok := FlowCustomErrorFailureResult(failure.cause)
	if !ok {
		return nil, false
	}
	results := make([]Result, failure.count)
	for i := range results {
		results[i] = result
	}
	return results, true
}

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

// FlowCapturedFormulaErrorResult identifies the record-triggered String-formula
// Custom Error shape observed on the singleton partial-insert route.
func FlowCapturedFormulaErrorResult(err error) (Result, bool) {
	var failure *flowCustomErrorFailure
	if !errors.As(err, &failure) || !failure.capturedFormula {
		return Result{}, false
	}
	return resultFromError("", failure.cause), true
}
