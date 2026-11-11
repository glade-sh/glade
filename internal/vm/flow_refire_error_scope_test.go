package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowRefireCapturedTriggerFailureScope(t *testing.T) {
	records := []storage.Record{{ID: "a00000000000001"}, {ID: "a00000000000002"}}
	positions := []int{0, 1}
	rule := storage.FlowRule{APIVersion: "62.0", Label: "Synthetic Flow"}
	failures := []dml.Result{{}, {Error: "synthetic rejection", StatusCode: "FIELD_CUSTOM_VALIDATION_EXCEPTION"}}
	if _, ok := flowRefireCapturedTriggerFailure(failures, records, positions, 2, rule); !ok {
		t.Fatal("captured record-level rejection was excluded")
	}
	positions = []int{0, 2}
	err, ok := flowRefireCapturedTriggerFailure(failures, records, positions, 3, rule)
	if !ok {
		t.Fatal("captured filtered rejection was excluded")
	}
	thrown := err.(*apexThrowError)
	details := thrown.value.Fields["__dmlErrors"]
	if len(details.List) != 1 || details.List[0].Fields["index"].Int != 2 {
		t.Fatalf("filtered failure lost original position: %+v", details)
	}
	positions = []int{0, 1}
	failures[1].Fields = []string{"Name"}
	if _, ok := flowRefireCapturedTriggerFailure(failures, records, positions, 2, rule); ok {
		t.Fatal("unobserved field-level rejection must retain its previous path")
	}
	failures[1].Fields = nil
	// Source 62-67 is now captured; keep the unobserved-version control.
	rule.APIVersion = "61.0"
	if _, ok := flowRefireCapturedTriggerFailure(failures, records, positions, 2, rule); ok {
		t.Fatal("unobserved Flow source version must retain its previous path")
	}
}
