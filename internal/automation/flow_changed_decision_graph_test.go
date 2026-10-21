package automation

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowChangedDecisionEvaluatesCommittedPrior(t *testing.T) {
	// The owned picklist decision observations compare a direct field with its
	// saved value. The nested decision control already uses the frame evaluator.
	for _, nested := range []bool{false, true} {
		for _, tc := range []struct{ name, prior, current, want string }{
			{"changed_to_target", "Cold", "Hot", "Selected"},
			{"unchanged", "Hot", "Hot", "Keep"},
			{"changed_to_other", "Cold", "Warm", "Keep"},
		} {
			t.Run(fmt.Sprintf("nested=%t/%s", nested, tc.name), func(t *testing.T) {
				start, guard := "ChooseChange", ""
				if nested {
					start = "ChooseEntry"
					guard = `<decisions><name>ChooseEntry</name><defaultConnector><targetReference>ChooseChange</targetReference></defaultConnector><rules><name>OtherEntry</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>$Record.Name</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Other</stringValue></rightValue></conditions><connector><targetReference>MarkKeep</targetReference></connector></rules></decisions>`
				}
				path := filepath.Join(t.TempDir(), "MeadowChange.flow-meta.xml")
				writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>
<start><object>Account</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordBeforeSave</triggerType><connector><targetReference>`+start+`</targetReference></connector></start>`+guard+`
<decisions><name>ChooseChange</name><defaultConnector><targetReference>MarkKeep</targetReference></defaultConnector><rules><name>ChangedToTarget</name><conditionLogic>and</conditionLogic>
<conditions><leftValueReference>$Record.Rating</leftValueReference><operator>IsChanged</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions>
<conditions><leftValueReference>$Record.Rating</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Hot</stringValue></rightValue></conditions>
<connector><targetReference>MarkSelected</targetReference></connector></rules></decisions>
<assignments><name>MarkSelected</name><assignmentItems><assignToReference>$Record.Description</assignToReference><operator>Assign</operator><value><stringValue>Selected</stringValue></value></assignmentItems></assignments>
<assignments><name>MarkKeep</name><assignmentItems><assignToReference>$Record.Description</assignToReference><operator>Assign</operator><value><stringValue>Keep</stringValue></value></assignmentItems></assignments>
</Flow>`)
				idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
				if err != nil || len(idx.Diagnostics) != 0 || len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
					t.Fatalf("load: %v %#v", err, idx)
				}
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				account := org.Objects["Account"]
				id := storage.ID("001000000000001")
				account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Meadow"), "Rating": storage.StringValue(tc.prior), "Description": storage.StringValue("Initial")}}
				org.Objects["Account"] = account
				ApplyToOrg(&org, idx)
				before := org.Objects["Account"].Records[id].Clone()
				rows := []storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Meadow"), "Rating": storage.StringValue(tc.current), "Description": storage.StringValue("Initial")}}}
				engine := dml.NewEngine(&org)
				if err := engine.ApplyBeforeSaveFlows(rows); err != nil {
					t.Fatal(err)
				}
				if got := rows[0].Fields["Description"].String; got != tc.want {
					t.Fatalf("decision=%q, want %q", got, tc.want)
				}
				if rows[0].Fields["Name"].String != "Meadow" || rows[0].Fields["Rating"].String != tc.current {
					t.Fatalf("unassigned fields changed: %#v", rows[0].Fields)
				}
				if org.Objects["Account"].Records[id].Fields["Rating"].String != before.Fields["Rating"].String {
					t.Fatal("before-save evaluation mutated saved prior")
				}
			})
		}
	}
}
