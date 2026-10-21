package automation

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A scratch-org flow (API 62.0, 67.0, 68.0) assigned Get_Records.Id to a
// triggering-record lookup field. With no matching record the field ended
// null. The assignment must keep the lookup reference as its source so the
// runtime can resolve it, rather than lowering it to an empty literal.
func TestFlowTriggeringRecordUpdateKeepsLookupIdSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AssignLookupId.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>
<recordLookups><name>Get_Target</name><assignNullValuesIfNoRecordsFound>false</assignNullValuesIfNoRecordsFound><connector><targetReference>Set_Fields</targetReference></connector><filterLogic>and</filterLogic><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>No Such Target</stringValue></value></filters><getFirstRecordOnly>true</getFirstRecordOnly><object>Target__c</object><storeOutputAutomatically>true</storeOutputAutomatically></recordLookups>
<recordUpdates><name>Set_Fields</name><inputAssignments><field>Partner__c</field><value><elementReference>Get_Target.Id</elementReference></value></inputAssignments><inputAssignments><field>Description</field><value><stringValue>flow-set</stringValue></value></inputAssignments><inputReference>$Record</inputReference></recordUpdates>
<start><connector><targetReference>Get_Target</targetReference></connector><object>Account</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordBeforeSave</triggerType></start></Flow>`)
	flow, diags, err := loadFlow(path)
	if err != nil || len(diags) != 0 || len(flow.Rules) != 1 {
		t.Fatalf("flow: error=%v diagnostics=%v ruleCount=%d", err, diags, len(flow.Rules))
	}
	var updates []storage.WorkflowFieldUpdate
	updates = append(updates, flow.Rules[0].FieldUpdates...)
	var walk func([]storage.FlowStep)
	walk = func(steps []storage.FlowStep) {
		for _, step := range steps {
			updates = append(updates, step.FieldUpdates...)
			for _, branch := range step.Branches {
				walk(branch.Steps)
			}
		}
	}
	walk(flow.Rules[0].Steps)
	for _, target := range flow.Rules[0].StepTargets {
		walk(target)
	}
	found := false
	for _, update := range updates {
		if update.Field == "Partner__c" {
			found = true
			if update.SourceField != "Get_Target.Id" {
				t.Fatalf("Partner__c update = %#v, want SourceField Get_Target.Id", update)
			}
		}
	}
	if !found {
		t.Fatalf("no Partner__c update in %#v", flow.Rules[0])
	}
}
