package automation

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// CurrentDateTime is evaluated when the referencing element runs:
// https://help.salesforce.com/s/articleView?id=platform.flow_ref_resources_system_variables.htm&type=5
func TestFlowCurrentDateTimeUsesExecutionClock(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		fixed       bool
	}{
		{"current-resource", "<elementReference>$Flow.CurrentDateTime</elementReference>", false},
		{"named-formula-control", "<elementReference>ClockValue</elementReference>", false},
		{"literal-control", "<dateTimeValue>2020-06-07T08:09:10Z</dateTimeValue>", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "StampClock.flow-meta.xml")
			writeWorkflowTestFile(t, path, fmt.Sprintf(`<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType><status>Active</status>
  <formulas><name>ClockValue</name><dataType>DateTime</dataType><expression>NOW()</expression></formulas>
  <start><object>Account</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordAfterSave</triggerType><connector><targetReference>WriteClock</targetReference></connector></start>
  <recordUpdates><name>WriteClock</name><inputReference>$Record</inputReference><inputAssignments><field>ObservedAt__c</field><value>%s</value></inputAssignments></recordUpdates>
</Flow>`, tc.value))
			idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) != 0 || len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
				t.Fatalf("load flow: %#v", idx)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			object := org.Objects["Account"]
			object.Definition.Fields["ObservedAt__c"] = storage.Field{APIName: "ObservedAt__c", Type: storage.FieldDateTime}
			org.Objects["Account"] = object
			ApplyToOrg(&org, idx)
			engine := dml.NewEngine(&org)
			clock := time.Date(2031, time.April, 9, 2, 3, 4, 0, time.UTC)
			engine.Now = func() time.Time { return clock }
			for i := 0; i < 2; i++ {
				inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Seed")}}})
				if !inserted[0].Success {
					t.Fatalf("insert: %#v", inserted)
				}
				want := clock.Format(time.RFC3339)
				if tc.fixed {
					want = "2020-06-07T08:09:10Z"
				}
				got := org.Objects["Account"].Records[inserted[0].ID].Fields["ObservedAt__c"]
				if got.Kind != storage.ValueDateTime || got.String != want {
					t.Fatalf("execution %d observed timestamp = %#v, want %s", i, got, want)
				}
				clock = clock.Add(3 * time.Hour)
			}
		})
	}
}
