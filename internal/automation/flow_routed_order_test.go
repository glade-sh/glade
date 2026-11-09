package automation

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Org capture flow_assign_routes (API 62.0 and 67.0): an Assignment ahead of a
// routed Decision runs together with the chosen outcome.
func TestFlowAssignmentBeforeRoutedDecisionIsOrdered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Pre.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<apiVersion>62.0</apiVersion>
<assignments><name>A_Pre</name><label>A_Pre</label><locationX>0</locationX><locationY>0</locationY><assignmentItems><assignToReference>$Record.Description</assignToReference><operator>Assign</operator><value><stringValue>pre</stringValue></value></assignmentItems><connector><targetReference>D</targetReference></connector></assignments>
<assignments><name>A_R1</name><label>A_R1</label><locationX>0</locationX><locationY>0</locationY><assignmentItems><assignToReference>$Record.Site</assignToReference><operator>Assign</operator><value><stringValue>r1</stringValue></value></assignmentItems></assignments>
<assignments><name>A_Def</name><label>A_Def</label><locationX>0</locationX><locationY>0</locationY><assignmentItems><assignToReference>$Record.Site</assignToReference><operator>Assign</operator><value><stringValue>null</stringValue></value></assignmentItems></assignments>
<decisions><name>D</name><label>D</label><locationX>0</locationX><locationY>0</locationY><defaultConnector><targetReference>A_Def</targetReference></defaultConnector><defaultConnectorLabel>Default</defaultConnectorLabel><rules><name>R1</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>$Record.Name</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>r1</stringValue></rightValue></conditions><connector><targetReference>A_R1</targetReference></connector><label>R1</label></rules></decisions>
<label>Pre</label>
<processType>AutoLaunchedFlow</processType>
<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>A_Pre</targetReference></connector><object>Account</object><recordTriggerType>Create</recordTriggerType><triggerType>RecordBeforeSave</triggerType></start>
<status>Active</status>
</Flow>`)
	flow, diagnostics, err := loadFlow(path)
	if err != nil || len(diagnostics) != 0 || len(flow.Rules) != 1 {
		t.Fatalf("err=%v diagnostics=%v rules=%d", err, diagnostics, len(flow.Rules))
	}
	rule := flow.Rules[0]
	if len(rule.Branches) != 0 || len(rule.Steps) < 2 {
		t.Fatalf("branches=%d steps=%d", len(rule.Branches), len(rule.Steps))
	}
	data, err := json.Marshal(struct {
		Steps   any
		Targets any
	}{rule.Steps, rule.StepTargets})
	if err != nil {
		t.Fatal(err)
	}
	encoded := strings.ToLower(string(data))
	if !strings.Contains(encoded, "a_pre") || !strings.Contains(encoded, "a_r1") || !strings.Contains(encoded, `\"null\"`) {
		t.Fatalf("steps=%s", encoded)
	}
}

func TestFlowRoutedDecisionNeedsGraph(t *testing.T) {
	routed := func(name string) flowDecisionXML {
		return flowDecisionXML{Name: name, Rules: []flowDecisionRuleXML{{Name: "R", Connector: flowConnectorXML{TargetReference: "X"}}}}
	}
	for _, tc := range []struct {
		name      string
		start     string
		decisions []flowDecisionXML
		want      bool
	}{
		{"decision_first", "D", []flowDecisionXML{routed("D")}, false},
		{"assignment_first", "A", []flowDecisionXML{routed("D")}, true},
		{"two_decisions", "D", []flowDecisionXML{routed("D"), routed("E")}, true},
		{"no_start", "", []flowDecisionXML{routed("D")}, false},
		{"orchestration_stages", "A", []flowDecisionXML{routed("D")}, false},
		{"orchestrator_process_type", "A", []flowDecisionXML{routed("D"), routed("E")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := flowXML{Decisions: tc.decisions}
			switch tc.name {
			case "orchestration_stages":
				raw.OrchestratedStages = []flowNamedNodeXML{{Name: "Stage_0"}}
			case "orchestrator_process_type":
				raw.ProcessType = "RecordTriggeredOrchestration"
			}
			raw.Start.Connector.TargetReference = tc.start
			if got := flowRoutedDecisionNeedsGraph(raw); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlowCustomErrorKeepsResourcesAndField(t *testing.T) {
	node := flowCustomErrorXML{Name: "E", Messages: []flowCustomErrorMessageXML{{ErrorMessage: "E {!FErrBr}", IsFieldError: true, FieldSelection: "Fw1__c"}}}
	got := modeledFlowCustomError(node, nil, nil, map[string]string{"ferrbr": "'x' & BR() & 'y'", "unused": "1"}).Messages[0]
	if got.Field != "Fw1__c" || len(got.ResourceFormulas) != 1 || got.ResourceFormulas["ferrbr"] == "" {
		t.Fatalf("message=%+v", got)
	}
}

// An unsupported orchestration flow keeps the flat projection's diagnostics:
// the routed Decision reports its own unmodeled stage target, not the start's.
func TestFlowOrchestrationKeepsFlatRoutedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Orch.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<apiVersion>62.0</apiVersion>
<decisions><name>D</name><label>D</label><defaultConnector><targetReference>Stage_1</targetReference></defaultConnector><rules><name>R1</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>$Record.Name</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>r1</stringValue></rightValue></conditions><connector><targetReference>Stage_3</targetReference></connector><label>R1</label></rules></decisions>
<label>Orch</label>
<orchestratedStages><name>Stage_0</name><label>Stage_0</label><connector><targetReference>D</targetReference></connector></orchestratedStages>
<orchestratedStages><name>Stage_1</name><label>Stage_1</label></orchestratedStages>
<orchestratedStages><name>Stage_3</name><label>Stage_3</label></orchestratedStages>
<processType>AutoLaunchedFlow</processType>
<start><connector><targetReference>Stage_0</targetReference></connector><object>Account</object><recordTriggerType>Create</recordTriggerType><triggerType>RecordAfterSave</triggerType></start>
<status>Active</status>
</Flow>`)
	_, diagnostics, err := loadFlow(path)
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, d := range diagnostics {
		messages = append(messages, d.Message)
	}
	all := strings.Join(messages, "\n")
	if strings.Contains(all, `routed decision target "Stage_0"`) || !strings.Contains(all, `routed decision target "Stage_3"`) {
		t.Fatalf("diagnostics=%s", all)
	}
}
