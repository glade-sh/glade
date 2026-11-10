package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The process_builder_wait and process_builder_shapes captures cover active
// processes of decisions, record updates, Apex, flow and email alert actions, and the
// wait-start stamp. Those run; TestProcessBuilderShapesNativeConformance replays
// their rows. Any other active process stays unrun and names its uncovered part
// in a GLADEAUTO002 warning, so it never half-runs; an inactive one stays silent.
// A flow action runs a flow made of record updates. The flow2 rows pin that shape;
// a flow with other steps, or one without source, leaves the process unrun.
func TestProcessBuilderFlowActionTargets(t *testing.T) {
	first := storage.FlowRecordUpdate{ObjectName: "Cedar__c", Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "recId"}}, InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Note__c", LiteralValue: "one"}}}
	second := first
	second.InputAssignments = []storage.WorkflowFieldUpdate{{Field: "Other__c", LiteralValue: "two"}}
	linear := storage.FlowRule{Name: "Sub", Active: true, Variables: []storage.FlowVariable{{Name: "recId", DataType: "String", IsInput: true}}, Steps: []storage.FlowStep{{Kind: "recordUpdate", RecordUpdate: first}, {Kind: "reference", Target: "next"}}, StepTargets: map[string][]storage.FlowStep{"next": {{Kind: "recordUpdate", RecordUpdate: second}}}}
	branched := storage.FlowRule{Name: "Sub", Active: true, Steps: []storage.FlowStep{{Kind: "decision"}, {Kind: "recordUpdate"}}}
	action := func(name string) Flow {
		return Flow{ObjectName: "Cedar__c", Rules: []storage.FlowRule{{Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{ActionType: "flow", ActionName: name, Inputs: []storage.WorkflowFieldUpdate{{Field: "recId", SourceField: "Id"}}}}}}}}
	}
	for _, tc := range []struct {
		name  string
		flows map[string]storage.FlowRule
		skip  string
	}{
		{"record_updates", map[string]storage.FlowRule{"sub": linear}, ""},
		{"other_steps", map[string]storage.FlowRule{"sub": branched}, "with other steps"},
		{"no_source", nil, "without its flow's source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defs := map[string]storage.ObjectDefinition{"Cedar__c": {APIName: "Cedar__c", Fields: map[string]storage.Field{"Note__c": {APIName: "Note__c", Type: storage.FieldString}, "Other__c": {APIName: "Other__c", Type: storage.FieldString}}}}
			got := processBuilderUncoveredFlowAction(action("Sub"), tc.flows, defs)
			if tc.skip == "" && got != "" || !strings.Contains(got, tc.skip) {
				t.Fatalf("skip = %q, want %q", got, tc.skip)
			}
		})
	}
}

func TestProcessBuilderNamespaceTokensStayUnrun(t *testing.T) {
	source := `<?xml version="1.0" encoding="UTF-8"?><Flow xmlns="http://soap.sforce.com/2006/04/metadata"><decisions><name>myDecision</name><label>d</label><rules><name>myRule_1</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>myVariable_current.%%%NAMESPACE%%%Stage__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>a</stringValue></rightValue></conditions><connector><targetReference>myRule_1_A1</targetReference></connector><label>r</label></rules></decisions><label>L</label>` +
		`<processMetadataValues><name>ObjectType</name><value><stringValue>%%%NAMESPACE%%%Cedar__c</stringValue></value></processMetadataValues><processMetadataValues><name>ObjectVariable</name><value><elementReference>myVariable_current</elementReference></value></processMetadataValues><processMetadataValues><name>OldObjectVariable</name><value><elementReference>myVariable_old</elementReference></value></processMetadataValues><processMetadataValues><name>TriggerType</name><value><stringValue>onAllChanges</stringValue></value></processMetadataValues>` +
		`<processType>Workflow</processType><recordUpdates><name>myRule_1_A1</name><label>u</label><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>%%%NAMESPACE%%%Note__c</field><value><stringValue>x</stringValue></value></inputAssignments><object>%%%NAMESPACE%%%Cedar__c</object></recordUpdates><startElementReference>myDecision</startElementReference><status>Active</status><variables><name>myVariable_current</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>true</isOutput><objectType>%%%NAMESPACE%%%Cedar__c</objectType></variables><variables><name>myVariable_old</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>false</isOutput><objectType>%%%NAMESPACE%%%Cedar__c</objectType></variables></Flow>`
	path := filepath.Join(t.TempDir(), "Cedar_Token.flow-meta.xml")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	flow, diagnostics, err := loadFlow(path)
	if err != nil {
		t.Fatal(err)
	}
	if flow.ObjectName != "" {
		t.Fatalf("object = %q, want an unrun process", flow.ObjectName)
	}
	warned := false
	for _, item := range diagnostics {
		warned = warned || strings.Contains(item.Message, "not run: namespace token")
	}
	if !warned {
		t.Fatalf("diagnostics = %+v, want the namespace token warning", diagnostics)
	}
}

func TestProcessBuilderOnlyCapturedShapeRuns(t *testing.T) {
	const action = `<actionCalls><name>myRule_1_A1</name><label>A</label><locationX>0</locationX><locationY>0</locationY><actionName>CedarDeleteMembers</actionName><actionType>apex</actionType><inputParameters><name>members</name><value><elementReference>myVariable_current</elementReference></value></inputParameters></actionCalls>`
	const decision = `<decisions><name>myDecision</name><label>myDecision</label><locationX>0</locationX><locationY>0</locationY><defaultConnectorLabel>default</defaultConnectorLabel><rules><name>myRule_1</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>formula_myRule_1</leftValueReference><operator>EqualTo</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions><connector><targetReference>myRule_1_A1</targetReference></connector><label>R</label></rules></decisions>`
	const formula = `<formulas><name>formula_myRule_1</name><dataType>Boolean</dataType><expression>AND({!myVariable_current.Active__c} = FALSE, ISCHANGED({!myVariable_current.Active__c}))</expression></formulas>`
	const variables = `<variables><name>myVariable_current</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>true</isOutput><objectType>Cedar_Member__c</objectType></variables><variables><name>myVariable_old</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>false</isOutput><objectType>Cedar_Member__c</objectType></variables>`
	metadata := func(trigger string) string {
		return `<processMetadataValues><name>ObjectType</name><value><stringValue>Cedar_Member__c</stringValue></value></processMetadataValues>` +
			`<processMetadataValues><name>ObjectVariable</name><value><elementReference>myVariable_current</elementReference></value></processMetadataValues>` +
			`<processMetadataValues><name>OldObjectVariable</name><value><elementReference>myVariable_old</elementReference></value></processMetadataValues>` +
			`<processMetadataValues><name>TriggerType</name><value><stringValue>` + trigger + `</stringValue></value></processMetadataValues>`
	}
	build := func(status, trigger, body string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>49.0</apiVersion>` + body + formula +
			`<interviewLabel>Cedar_Member_Update-1_InterviewLabel</interviewLabel><label>Cedar Member Update</label>` + metadata(trigger) +
			`<processType>Workflow</processType><startElementReference>myDecision</startElementReference><status>` + status + `</status>` + variables + `</Flow>`
	}
	const recordUpdate = `<recordUpdates><name>myRule_1_A2</name><label>U</label><locationX>0</locationX><locationY>0</locationY><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>Active__c</field><value><booleanValue>true</booleanValue></value></inputAssignments><object>Cedar_Member__c</object></recordUpdates>`
	emailAction := strings.Replace(strings.Replace(action, "<actionType>apex</actionType>", "<actionType>emailAlert</actionType>", 1), "CedarDeleteMembers", "Cedar_Member__c.Cedar_Alert", 1)
	chatterAction := strings.Replace(strings.Replace(action, "<actionType>apex</actionType>", "<actionType>chatterPost</actionType>", 1), "CedarDeleteMembers", "chatterPost", 1)
	const waitStart = `<assignments><name>myVariable_waitStartTimeAssignment</name><label>S</label><locationX>0</locationX><locationY>0</locationY><assignmentItems><assignToReference>myVariable_waitStartTimeVariable</assignToReference><operator>Assign</operator><value><elementReference>$Flow.CurrentDateTime</elementReference></value></assignmentItems><connector><targetReference>myDecision</targetReference></connector></assignments>`
	const waitStartVariable = `<variables><name>myVariable_waitStartTimeVariable</name><dataType>DateTime</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput></variables>`
	secondDecision := strings.NewReplacer("myDecision", "myDecision2", "myRule_1", "myRule_2").Replace(decision)
	extraVariable := `<variables><name>RecursiveCountVariable</name><dataType>Number</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><scale>2</scale></variables>`
	for _, tc := range []struct {
		name, status, trigger, body string
		runs                        bool
		skip                        string
	}{
		{"captured", "Active", "onAllChanges", action + decision, true, ""},
		{"inactive", "Obsolete", "onAllChanges", action + decision, false, ""},
		{"create_only", "Active", "onCreateOnly", action + decision, true, ""},
		{"record_update", "Active", "onAllChanges", action + decision + recordUpdate, true, ""},
		{"two_criteria", "Active", "onAllChanges", action + decision + secondDecision, true, ""},
		{"two_actions", "Active", "onAllChanges", action + strings.Replace(action, "myRule_1_A1", "myRule_1_A2", 1) + decision, true, ""},
		{"email_alert", "Active", "onAllChanges", emailAction + decision, true, ""},
		{"metrics_flag", "Active", "onAllChanges", action + decision + `<areMetricsLoggedToDataCloud>false</areMetricsLoggedToDataCloud>`, true, ""},
		{"wait_start", "Active", "onAllChanges", action + decision + waitStart + waitStartVariable, true, ""},
		{"wait_start_variable_alone", "Active", "onAllChanges", action + decision + waitStartVariable, false, `variable "myVariable_waitStartTimeVariable"`},
		{"other_assignment", "Active", "onAllChanges", action + decision + strings.ReplaceAll(waitStart, "myVariable_waitStartTimeAssignment", "myWaitAssignment_myWait_myRule_1"), false, `assignment "myWaitAssignment_myWait_myRule_1"`},
		{"waits", "Active", "onAllChanges", action + decision + `<waits><name>myWait_myRule_1</name></waits>`, false, `element "waits"`},
		{"chatter", "Active", "onAllChanges", chatterAction + decision, false, `actionType "chatterPost"`},
		{"record_create", "Active", "onAllChanges", action + decision + `<recordCreates><name>myRule_1_A2</name></recordCreates>`, false, `element "recordCreates"`},
		{"unknown_function", "Active", "onAllChanges", action + decision + `<formulas><name>formula_2</name><dataType>String</dataType><expression>GETSESSIONID()</expression></formulas>`, false, `calling GETSESSIONID()`},
		{"extra_variable", "Active", "onAllChanges", action + decision + extraVariable, false, `variable "RecursiveCountVariable"`},
		{"operator_without_row", "Active", "onAllChanges", action + strings.ReplaceAll(decision, "<operator>EqualTo</operator>", "<operator>StartsWith</operator>"), false, `operator "StartsWith"`},
		{"global_left", "Active", "onAllChanges", action + strings.ReplaceAll(decision, "<leftValueReference>formula_myRule_1</leftValueReference>", "<leftValueReference>$User.Id</leftValueReference>"), false, `reference "$User.Id"`},
		{"global_right", "Active", "onAllChanges", action + strings.ReplaceAll(decision, "<booleanValue>true</booleanValue>", "<elementReference>$Label.Cedar</elementReference>"), false, `right operand "$Label.Cedar"`},
		{"prior_right", "Active", "onAllChanges", action + strings.ReplaceAll(decision, "<booleanValue>true</booleanValue>", "<elementReference>myVariable_old.Active__c</elementReference>"), false, `right operand "myVariable_old.Active__c"`},
		{"field_right_ordering", "Active", "onAllChanges", action + strings.ReplaceAll(strings.ReplaceAll(decision, "<booleanValue>true</booleanValue>", "<elementReference>myVariable_current.Active__c</elementReference>"), "<operator>EqualTo</operator>", "<operator>GreaterThan</operator>"), false, `right operand "myVariable_current.Active__c"`},
		{"field_right_equality", "Active", "onAllChanges", action + strings.ReplaceAll(decision, "<booleanValue>true</booleanValue>", "<elementReference>myVariable_current.Active__c</elementReference>"), false, `uncaptured expression or usage`},
		{"formula_global", "Active", "onAllChanges", action + decision + `<formulas><name>formula_x</name><dataType>Boolean</dataType><expression>{!$User.Alias} = "x"</expression></formulas>`, false, `formula "formula_x" reading a global`},
		{"update_filter_operator", "Active", "onAllChanges", action + decision + strings.Replace(recordUpdate, "<operator>EqualTo</operator>", "<operator>NotEqualTo</operator>", 1), false, `record update filter operator "NotEqualTo"`},
		{"update_global_value", "Active", "onAllChanges", action + decision + strings.Replace(recordUpdate, "<booleanValue>true</booleanValue>", "<elementReference>$User.Id</elementReference>", 1), false, `reference "$User.Id"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Cedar_Member_Update.flow-meta.xml")
			if err := os.WriteFile(path, []byte(build(tc.status, tc.trigger, tc.body)), 0o644); err != nil {
				t.Fatal(err)
			}
			definitions := map[string]storage.ObjectDefinition{"Cedar_Member__c": {Fields: map[string]storage.Field{"Active__c": {APIName: "Active__c", Type: storage.FieldBoolean}}}}
			flow, diagnostics, err := loadFlow(path, definitions)
			if err != nil {
				t.Fatal(err)
			}
			if got := flow.ObjectName == "Cedar_Member__c"; got != tc.runs {
				t.Fatalf("runs as a record-triggered flow = %v (object %q), want %v", got, flow.ObjectName, tc.runs)
			}
			var warning string
			for _, diagnostic := range diagnostics {
				if strings.Contains(diagnostic.Message, "Process Builder process is not run") {
					warning = diagnostic.Message
				}
			}
			if tc.skip == "" {
				if warning != "" {
					t.Fatalf("unexpected warning %q", warning)
				}
				return
			}
			if !strings.Contains(warning, tc.skip) {
				t.Fatalf("warning = %q, want it to name %s; diagnostics %+v", warning, tc.skip, diagnostics)
			}
		})
	}
}

func TestProcessBuilderSeveralAfterSaveFlowsStayUnrun(t *testing.T) {
	process := `<?xml version="1.0" encoding="UTF-8"?><Flow xmlns="http://soap.sforce.com/2006/04/metadata"><decisions><name>myDecision</name><label>d</label><rules><name>myRule_1</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>myVariable_current.Stage__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>a</stringValue></rightValue></conditions><connector><targetReference>myRule_1_A1</targetReference></connector><label>r</label></rules></decisions><label>L</label>` +
		`<processMetadataValues><name>ObjectType</name><value><stringValue>Cedar__c</stringValue></value></processMetadataValues><processMetadataValues><name>ObjectVariable</name><value><elementReference>myVariable_current</elementReference></value></processMetadataValues><processMetadataValues><name>OldObjectVariable</name><value><elementReference>myVariable_old</elementReference></value></processMetadataValues><processMetadataValues><name>TriggerType</name><value><stringValue>onAllChanges</stringValue></value></processMetadataValues>` +
		`<processType>Workflow</processType><recordUpdates><name>myRule_1_A1</name><label>u</label><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>Note__c</field><value><stringValue>x</stringValue></value></inputAssignments><object>Cedar__c</object></recordUpdates><startElementReference>myDecision</startElementReference><status>Active</status><variables><name>myVariable_current</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>true</isOutput><objectType>Cedar__c</objectType></variables><variables><name>myVariable_old</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>false</isOutput><objectType>Cedar__c</objectType></variables></Flow>`
	path := filepath.Join(t.TempDir(), "Cedar_Process.flow-meta.xml")
	if err := os.WriteFile(path, []byte(process), 0o644); err != nil {
		t.Fatal(err)
	}
	after := func() Flow {
		return Flow{ObjectName: "Cedar__c", Rules: []storage.FlowRule{{Name: "after", Active: true, TriggerType: "RecordAfterSave"}}}
	}
	for _, tc := range []struct {
		afterFlows int
		runs       bool
	}{{1, true}, {2, false}} {
		flow, _, err := loadFlow(path)
		if err != nil {
			t.Fatal(err)
		}
		idx := &Index{Flows: []Flow{flow}}
		paths := []string{path}
		for i := 0; i < tc.afterFlows; i++ {
			idx.Flows = append(idx.Flows, after())
			paths = append(paths, "")
		}
		if err := leaveUncoveredProcessesUnrun(idx, paths, nil); err != nil {
			t.Fatal(err)
		}
		if got := idx.Flows[0].ObjectName == "Cedar__c"; got != tc.runs {
			t.Fatalf("%d after-save flows: runs = %v, want %v", tc.afterFlows, got, tc.runs)
		}
		warned := false
		for _, item := range idx.Diagnostics {
			warned = warned || strings.Contains(item.Message, "several after-save flows on its object")
		}
		if warned == tc.runs {
			t.Fatalf("%d after-save flows: warned = %v, diagnostics %+v", tc.afterFlows, warned, idx.Diagnostics)
		}
	}
}
