package automation

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Related-record updates are supported by record-triggered Flow:
// https://help.salesforce.com/s/articleView?id=release-notes.rn_automate_flow_builder_update_related_records.htm&type=5
// Resolve the relationship through schema metadata, including relationships
// whose name differs from their target object (Contact.ReportsTo -> Contact).
func TestFlowUpdateStandardRelationshipUsesSchemaTarget(t *testing.T) {
	for _, tc := range []struct {
		name, relationship, explicitObject, wantObject string
	}{
		{"account", "Account", "", "Account"},
		{"reports-to", "ReportsTo", "", "Contact"},
		{"explicit-object-control", "Account", "Account", "Account"},
		{"unknown-relationship-control", "MissingRelationship", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "RouteParent.flow-meta.xml")
			writeWorkflowTestFile(t, path, fmt.Sprintf(`<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType><status>Active</status>
  <start><object>Contact</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordAfterSave</triggerType><connector><targetReference>WriteParent</targetReference></connector></start>
  <recordUpdates><name>WriteParent</name><inputReference>$Record.%s</inputReference><object>%s</object><inputAssignments><field>Description</field><value><stringValue>Routed</stringValue></value></inputAssignments></recordUpdates>
</Flow>`, tc.relationship, tc.explicitObject))
			idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Flows) != 1 {
				t.Fatalf("flows = %#v", idx.Flows)
			}
			if tc.wantObject == "" {
				if len(idx.Flows[0].Rules) != 0 || len(idx.Diagnostics) == 0 {
					t.Fatalf("unknown relationship must stay unsupported: %#v", idx)
				}
				return
			}
			if len(idx.Diagnostics) != 0 || len(idx.Flows[0].Rules) != 1 {
				t.Fatalf("relationship update did not lower: %#v", idx)
			}
			steps := idx.Flows[0].Rules[0].Steps
			if len(steps) != 1 || steps[0].Kind != "recordUpdate" || steps[0].RecordUpdate.ObjectName != tc.wantObject {
				t.Fatalf("relationship target = %#v, want %s", steps, tc.wantObject)
			}

			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			storage.EnsureStandardObject(&org, "Contact")
			engine := dml.NewEngine(&org)
			parentFields := map[string]storage.Value{"Name": storage.StringValue("Parent")}
			lookupField := "AccountId"
			if tc.wantObject == "Contact" {
				parentFields = map[string]storage.Value{"LastName": storage.StringValue("Parent")}
				lookupField = "ReportsToId"
			}
			parents := engine.Insert([]storage.Record{{Object: tc.wantObject, Fields: parentFields}, {Object: tc.wantObject, Fields: parentFields}})
			for _, result := range parents {
				if !result.Success {
					t.Fatalf("seed parent: %#v", result)
				}
			}
			ApplyToOrg(&org, idx)
			inserted := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{
				"LastName": storage.StringValue("Child"), lookupField: storage.IDValue(parents[0].ID),
			}}})
			if !inserted[0].Success {
				t.Fatalf("insert child: %#v", inserted)
			}
			if got := org.Objects[tc.wantObject].Records[parents[0].ID].Fields["Description"].String; got != "Routed" {
				t.Fatalf("parent description = %q", got)
			}
			if got := org.Objects[tc.wantObject].Records[parents[1].ID].Fields["Description"].String; got != "" {
				t.Fatalf("unrelated parent description changed to %q", got)
			}
		})
	}
}
