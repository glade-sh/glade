package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native flow formula rows: supplemental/contexts/org.tsv, APIs 62 and 67.
func TestFlowFormulaCurrentPermission(t *testing.T) {
	for _, assigned := range []bool{false, true} {
		org := permissionRegexOrg(assigned, "Bypass")
		engine := NewEngine(&org)
		engine.UserID = permTestUser
		got, ok := engine.flowFrameValue(storage.FlowAssignment{Formula: "$Permission.Bypass"}, &flowFrame{})
		if !ok || got.Kind != storage.ValueBoolean || got.Boolean != assigned {
			t.Errorf("assigned=%v formula=%+v ok=%v", assigned, got, ok)
		}
	}
}
