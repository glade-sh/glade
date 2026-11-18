package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Only the captured failures map to a singleton row error; an invalid
// reference type in a Create Records element keeps its earlier error path.
func TestFlowSavedRecordFailureResultIsLimitedToCapturedFailures(t *testing.T) {
	rule := storage.FlowRule{Name: "Synthetic"}
	typed := flowCoreRecordCreateTriggerError(rule, &flowCreateReferenceTypeError{code: "FIELD_INTEGRITY_EXCEPTION", label: "Account", value: "x"})
	if _, ok := FlowSavedRecordFailureResult(typed); ok {
		t.Fatal("reference-type failure mapped to a row error")
	}
	if !(&flowElementFault{code: "DUPLICATES_DETECTED"}).savedRowFailure() || !(&flowElementFault{verb: flowApexActionVerb}).savedRowFailure() {
		t.Fatal("captured failure lost its row error")
	}
	if (&flowElementFault{verb: "create", code: "STRING_TOO_LONG"}).savedRowFailure() {
		t.Fatal("uncaptured failure marked as a row error")
	}
	other, ok := flowUnhandledTriggerError(rule, &flowElementFault{err: dmlErrorf("STRING_TOO_LONG", nil, "x"), verb: "create", code: "STRING_TOO_LONG", text: "long"})
	if !ok {
		t.Fatal("string fault not reported")
	}
	if _, ok := FlowSavedRecordFailureResult(other); ok {
		t.Fatal("other unhandled create fault mapped to a row error")
	}
}
