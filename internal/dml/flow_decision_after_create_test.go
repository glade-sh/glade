package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func afterCreateChangeTestOrg(want string) storage.OrgState {
	org := testOrg()
	state := org.Objects["Account"]
	state.Definition.Fields["Marker__c"] = storage.Field{APIName: "Marker__c", Type: storage.FieldString}
	mark := func(value string) []storage.FlowStep {
		return []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Marker__c", LiteralValue: value}}}}
	}
	state.Definition.FlowRules = []storage.FlowRule{{
		Name: "AfterCreateChange", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "CreateAndUpdate",
		Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
			{Name: "Changed", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "isChanged", Value: want}}, Steps: mark("matched")},
			{Name: "Default", Default: true, Steps: mark("default")},
		}}},
	}}
	org.Objects["Account"] = state
	return org
}

// Is Changed is false for a newly created record even when an after-save Flow
// sees the assigned Id. The explicit insertion event must distinguish this
// from an existing record with no prior snapshot supplied.
func TestAfterSaveCreateDecisionIsChanged(t *testing.T) {
	for _, tc := range []struct {
		name, want, marker string
		deferred           bool
	}{
		{name: "immediateChangedFalse", want: "true", marker: "default"},
		{name: "immediateUnchangedTrue", want: "false", marker: "matched"},
		{name: "deferredChangedFalse", deferred: true, want: "true", marker: "default"},
		{name: "deferredUnchangedTrue", deferred: true, want: "false", marker: "matched"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := afterCreateChangeTestOrg(tc.want)
			engine := NewEngine(&org)
			engine.DeferAutomation = tc.deferred
			results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned after-save creation")}}})
			if len(results) != 1 || !results[0].Success {
				t.Fatalf("insert=%+v", results)
			}
			if tc.deferred {
				if _, err := engine.ApplyAutomation("Account", results[0].ID); err != nil {
					t.Fatal(err)
				}
			}
			stored := org.Objects["Account"].Records[results[0].ID]
			if stored.ID == "" {
				t.Fatal("after-save source has no assigned Id")
			}
			if got := stored.Fields["Marker__c"].String; got != tc.marker {
				t.Fatalf("marker=%q want %q", got, tc.marker)
			}
			// Reapplying without another explicit create event must not retain
			// the initial creation classification on the Engine.
			if _, err := engine.ApplyAutomation("Account", results[0].ID); err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
				t.Fatalf("reapply boundary: %v", err)
			}
		})
	}
}

func TestAfterSaveCreateDecisionRetainsUpdateBoundary(t *testing.T) {
	org := afterCreateChangeTestOrg("true")
	state := org.Objects["Account"]
	id := storage.ID("001000000000001")
	state.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Before")}}
	org.Objects["Account"] = state
	engine := NewEngine(&org)
	if _, err := engine.ApplyAutomation("Account", id); err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("existing record without creation event: %v", err)
	}
	results := engine.Update([]storage.Record{{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("After")}}})
	// A real update supplies a committed prior record, so its direct-field
	// comparison is supported even though bare automation above has no prior.
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("update with prior=%+v", results)
	}
	if got := org.Objects["Account"].Records[id].Fields["Marker__c"].String; got != "matched" {
		t.Fatalf("changed update marker=%q, want matched", got)
	}
	if _, err := engine.ApplyAutomation("Account", id); err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("missing-prior boundary after update: %v", err)
	}
}

func TestAfterSaveCreateDecisionErrorConsumesEvent(t *testing.T) {
	org := afterCreateChangeTestOrg("unknown")
	engine := NewEngine(&org)
	engine.DeferAutomation = true
	results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned error boundary")}}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("insert=%+v", results)
	}
	if _, err := engine.ApplyAutomation("Account", results[0].ID); err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("invalid Boolean boundary: %v", err)
	}
	state := org.Objects["Account"]
	state.Definition.FlowRules[0].Steps[0].Branches[0].Criteria[0].Value = "true"
	org.Objects["Account"] = state
	if _, err := engine.ApplyAutomation("Account", results[0].ID); err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("creation event leaked through failed automation: %v", err)
	}
}
