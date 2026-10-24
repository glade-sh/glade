package automation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlowTextNegationInterviewDecisionLowering(t *testing.T) {
	// Native EB and EC rows use NOT 1 over Contains, including a null input.
	for _, name := range []string{"TextEmailDirectEb", "TextEmailDirectEc"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("../dml/testdata/conformance/flow_text_email/project/force-app/main/default/flows", name+".flow-meta.xml")
			flow, diagnostics, err := loadFlow(path)
			if err != nil || len(diagnostics) != 0 || len(flow.Rules) != 1 {
				t.Fatalf("load: %v diagnostics=%+v rules=%d", err, diagnostics, len(flow.Rules))
			}
		})
	}
}

// Native direct_EB/direct_EC pin the match/null operands. This local route twin
// keeps the plain record trigger's start connected directly to the decision.
func TestFlowTextNegationPlainTriggerDecisionLowering(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, input := range []string{"match", "null"} {
			t.Run(api+"/trigger_plain/"+input, func(t *testing.T) {
				value := ""
				if input == "match" {
					value = `<value><stringValue>Hello World</stringValue></value>`
				}
				source := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>
<start><object>Account</object><triggerType>RecordAfterSave</triggerType><recordTriggerType>Create</recordTriggerType><connector><targetReference>Choose</targetReference></connector></start>
<variables><name>InputText</name><dataType>String</dataType><isCollection>false</isCollection>` + value + `</variables>
<variables><name>Matched</name><dataType>Boolean</dataType><isOutput>true</isOutput><isCollection>false</isCollection></variables>
<decisions><name>Choose</name><defaultConnector><targetReference>Skip</targetReference></defaultConnector><rules><name>Match</name><conditionLogic>NOT 1</conditionLogic><conditions><leftValueReference>InputText</leftValueReference><operator>Contains</operator><rightValue><stringValue>world</stringValue></rightValue></conditions><connector><targetReference>Mark</targetReference></connector></rules></decisions>
<recordCreates><name>Mark</name><object>Task</object><inputAssignments><field>Subject</field><value><stringValue>Negation route marker</stringValue></value></inputAssignments></recordCreates>
<assignments><name>Skip</name><assignmentItems><assignToReference>Matched</assignToReference><operator>Assign</operator><value><booleanValue>false</booleanValue></value></assignmentItems></assignments></Flow>`
				path := filepath.Join(t.TempDir(), "PlainNegation.flow-meta.xml")
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				flow, diagnostics, err := loadFlow(path)
				if err != nil || len(diagnostics) != 0 || len(flow.Rules) != 1 {
					t.Fatalf("load: %v diagnostics=%+v rules=%d", err, diagnostics, len(flow.Rules))
				}
				rule := flow.Rules[0]
				if len(rule.Steps) == 0 || len(rule.Branches) != 0 {
					t.Fatalf("plain-trigger scalar decision needs framed steps; steps=%d legacy branches=%d", len(rule.Steps), len(rule.Branches))
				}
			})
		}
	}
}
