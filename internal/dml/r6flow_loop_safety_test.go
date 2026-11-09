package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestR6FlowBeforeSaveLoopBodySafety(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		want       bool
	}{
		{"custom error", "customError", true},
		{"assignment", "assignment", true},
		{"record creation", "recordCreate", false},
		{"external action", "action", false},
		{"record update", "recordUpdate", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := []storage.FlowStep{{Kind: "loop", Loop: storage.FlowLoop{CollectionReference: "Items", IterationOrder: "Asc", Steps: []storage.FlowStep{{Kind: tc.kind}}}}}
			if got := flowStepsBeforeSaveSafeWithTargets(steps, false, false, nil); got != tc.want {
				t.Fatalf("loop body %s: safe=%t want=%t", tc.kind, got, tc.want)
			}
		})
	}
}
