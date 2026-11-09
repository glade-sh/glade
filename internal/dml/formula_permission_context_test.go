package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// These callers share the captured permission semantics; the snapshot values
// are synthetic fixture inputs rather than Salesforce metadata fields.
func TestFormulaPermissionSnapshotCallers(t *testing.T) {
	for _, tc := range []struct{ field, name string }{{"Permissions", "FirstGrant"}, {"PermissionSets", "SecondGrant"}} {
		t.Run(tc.field, func(t *testing.T) {
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			engine.UserID = permTestUser
			engine.PermissionUserFields = map[string]storage.Value{tc.field: storage.StringValue(tc.name)}
			def := storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{}}
			record := storage.Record{Object: def.APIName, Fields: map[string]storage.Value{}}
			formula := "$Permission." + tc.name
			checks := map[string]func() (bool, bool){
				"assignment": func() (bool, bool) {
					return engine.assignmentEntryMatches(storage.AssignmentRuleEntry{Formula: formula}, record, def)
				},
				"workflow": func() (bool, bool) {
					return evaluateWorkflowRuleWithUser(storage.WorkflowRule{Formula: formula}, record, def, &org, engine.UserID, engine.PermissionUserFields)
				},
				"flow-branch": func() (bool, bool) {
					return engine.evaluateFlowBranch(storage.FlowBranch{Formula: formula}, record, def, false)
				},
			}
			for name, check := range checks {
				if got, ok := check(); !ok || !got {
					t.Errorf("%s: got=%v ok=%v", name, got, ok)
				}
			}
			value, ok := engine.flowFrameValue(storage.FlowAssignment{Formula: formula}, &flowFrame{})
			if !ok || value.Kind != storage.ValueBoolean || !value.Boolean {
				t.Errorf("flow formula: value=%+v ok=%v", value, ok)
			}
		})
	}
}
