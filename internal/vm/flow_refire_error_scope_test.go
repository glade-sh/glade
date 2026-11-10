package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowRefireCapturedTriggerFailureScope(t *testing.T) {
	records := []storage.Record{{ID: "a00000000000001"}, {ID: "a00000000000002"}}
	rule := storage.FlowRule{APIVersion: "62.0", Label: "Synthetic Flow"}
	failures := []dml.Result{{}, {Error: "synthetic rejection", StatusCode: "FIELD_CUSTOM_VALIDATION_EXCEPTION"}}
	if _, ok := flowRefireCapturedTriggerFailure(failures, records, 2, rule); !ok {
		t.Fatal("captured record-level rejection was excluded")
	}
	failures[1].Fields = []string{"Name"}
	if _, ok := flowRefireCapturedTriggerFailure(failures, records, 2, rule); ok {
		t.Fatal("unobserved field-level rejection must retain its previous path")
	}
	failures[1].Fields = nil
	rule.APIVersion = "66.0"
	if _, ok := flowRefireCapturedTriggerFailure(failures, records, 2, rule); ok {
		t.Fatal("unobserved Flow source version must retain its previous path")
	}
}
