package dml

// flowActionBatchFailure retains each interview's failure at its input index.
// It is used only when a shared action fault reaches a Custom Error node.
type flowActionBatchFailure struct {
	cause   error
	results []Result
}

func (failure *flowActionBatchFailure) Error() string { return failure.cause.Error() }
func (failure *flowActionBatchFailure) Unwrap() error { return failure.cause }

// FlowActionBatchFailureResults returns the indexed outcomes of a shared
// invocable-action fault. Successful entries preserve gaps in the input list.
func FlowActionBatchFailureResults(err error) ([]Result, bool) {
	failure, ok := err.(*flowActionBatchFailure)
	if !ok {
		return nil, false
	}
	return failure.results, true
}
