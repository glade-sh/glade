package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native rows: testdata/flow_name_order/org.tsv, User equals and IsNull routes.
func TestFlowInsertDerivedUserNameKeepsWriteability(t *testing.T) {
	for _, tc := range []struct {
		name, operation, value          string
		explicit, wantSuccess, wantFlow bool
	}{
		{"derived-equals", "equals", "Local Provider", false, true, false},
		{"derived-null", "isNull", "true", false, true, true},
		{"derived-nonnull", "isNull", "false", false, true, false},
		{"explicit", "equals", "Local Provider", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "User")
			object := org.Objects["User"]
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Before", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: tc.operation, Value: tc.value}}, FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Alias", LiteralValue: "flow"}}}}
			org.Objects["User"] = object
			row := storage.Record{Object: "User", Fields: map[string]storage.Value{"FirstName": storage.StringValue("Local"), "LastName": storage.StringValue("Provider"), "TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}}
			if tc.explicit {
				row.Fields["Name"] = storage.StringValue("Explicit")
			}
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{row})[0]
			if result.Success != tc.wantSuccess {
				t.Fatalf("insert: %+v", result)
			}
			if result.Success {
				record := org.Objects["User"].Records[result.ID]
				if got := record.Fields["Alias"].String == "flow"; got != tc.wantFlow {
					t.Fatalf("flow applied=%v, want %v", got, tc.wantFlow)
				}
				if got := record.Fields["Name"].String; got != "Local Provider" {
					t.Fatalf("saved Name=%q", got)
				}
			}
		})
	}
}
