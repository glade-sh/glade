package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowActionBatchLiteralFaultScope(t *testing.T) {
	rule := storage.FlowRule{Steps: []storage.FlowStep{{Kind: "action", FaultBranch: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Messages: []storage.FlowCustomErrorMessage{{Message: "synthetic action fault"}}}}}}}}
	if !flowActionBatchLiteralFault(rule) {
		t.Fatal("captured literal fault was excluded")
	}
	rule.Steps[0].FaultBranch[0].CustomError.Messages[0].Message = "fault {!$Record.Name}"
	if flowActionBatchLiteralFault(rule) {
		t.Fatal("record-dependent fault must retain its prior envelope")
	}
	rule.Steps[0].FaultBranch[0].CustomError.Messages[0].Message = "synthetic action fault"
	rule.Steps[0].FaultBranch[0].CustomError.Messages[0].Field = "Name"
	if flowActionBatchLiteralFault(rule) {
		t.Fatal("field-level fault must retain its prior envelope")
	}
}
