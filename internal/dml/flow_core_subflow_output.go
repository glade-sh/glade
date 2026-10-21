package dml

import "errors"

type flowCoreUnknownSubflowOutputError struct {
	subflow string
	output  string
}

func (e *flowCoreUnknownSubflowOutputError) Error() string {
	return "dml: flow subflow " + e.subflow + " references unknown output " + e.output
}

type flowCoreSubflowOutputFailure struct{ cause error }

func (e *flowCoreSubflowOutputFailure) Error() string { return e.cause.Error() }
func (e *flowCoreSubflowOutputFailure) Unwrap() error { return e.cause }

// FlowSubflowOutputFailureResult exposes the native row error for a record
// flow whose subflow output binding faults during execution.
func FlowSubflowOutputFailureResult(err error) (Result, bool) {
	var failure *flowCoreSubflowOutputFailure
	if !errors.As(err, &failure) {
		return Result{}, false
	}
	return resultFromError("", failure.cause), true
}
