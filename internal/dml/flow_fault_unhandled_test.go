package dml

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Only the unhandled faults with native rows get the new wording: Update,
// Delete and Get Records, and Create Records rejected with STRING_TOO_LONG.
// Other Create codes and the reference-type rejections keep origin behavior.
func TestFlowUnhandledFaultCoverage(t *testing.T) {
	rule := storage.FlowRule{Name: "Probe", TriggerType: "RecordAfterSave"}
	cases := []struct {
		name  string
		fault *flowElementFault
		want  bool
	}{
		{"update", &flowElementFault{verb: "update", code: "STRING_TOO_LONG", text: "t"}, true},
		{"delete", &flowElementFault{verb: "delete", code: "DELETE_FAILED", text: "t"}, true},
		{"lookup", &flowElementFault{verb: "look up", text: "t"}, true},
		{"create string too long", &flowElementFault{verb: "create", code: "STRING_TOO_LONG", text: "t"}, true},
		{"create required field", &flowElementFault{verb: "create", code: "REQUIRED_FIELD_MISSING", text: "t"}, false},
		{"create validation", &flowElementFault{verb: "create", code: "FIELD_CUSTOM_VALIDATION_EXCEPTION", text: "t"}, false},
		{"create duplicate", &flowElementFault{verb: "create", code: "DUPLICATE_VALUE", text: "t"}, false},
		{"create reference type", &flowElementFault{verb: "create", code: "FIELD_INTEGRITY_EXCEPTION", text: "t"}, false},
		{"create malformed id", &flowElementFault{verb: "create", code: "MALFORMED_ID", text: "t"}, false},
		{"no captured text", &flowElementFault{err: errors.New("x")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := flowUnhandledTriggerError(rule, c.fault); ok != c.want {
				t.Fatalf("record-triggered: got %v, want %v", ok, c.want)
			}
			if _, ok := FlowUnhandledDetail(c.fault); ok != c.want {
				t.Fatalf("autolaunched: got %v, want %v", ok, c.want)
			}
		})
	}
}
