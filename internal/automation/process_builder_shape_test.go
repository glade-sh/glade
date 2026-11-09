package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The process_builder_wait capture covers one Process Builder shape: an active
// onAllChanges process with one criteria node whose single immediate action is
// an Apex action. That shape runs as a record-triggered after-save flow. Any
// other active process stays unrun and names its uncovered part in a
// GLADEAUTO002 warning, so it never half-runs; an inactive one stays silent.
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
	secondDecision := strings.NewReplacer("myDecision", "myDecision2", "myRule_1", "myRule_2").Replace(decision)
	extraVariable := `<variables><name>RecursiveCountVariable</name><dataType>Number</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><scale>2</scale></variables>`
	for _, tc := range []struct {
		name, status, trigger, body string
		runs                        bool
		skip                        string
	}{
		{"captured", "Active", "onAllChanges", action + decision, true, ""},
		{"inactive", "Obsolete", "onAllChanges", action + decision, false, ""},
		{"create_only", "Active", "onCreateOnly", action + decision, false, `TriggerType "onCreateOnly"`},
		{"record_update", "Active", "onAllChanges", action + decision + recordUpdate, false, `element "recordUpdates"`},
		{"email_alert", "Active", "onAllChanges", emailAction + decision, false, `actionType "emailAlert"`},
		{"two_criteria", "Active", "onAllChanges", action + decision + secondDecision, false, "2 criteria nodes"},
		{"two_actions", "Active", "onAllChanges", action + strings.Replace(action, "myRule_1_A1", "myRule_1_A2", 1) + decision, false, "2 immediate actions"},
		{"extra_variable", "Active", "onAllChanges", action + decision + extraVariable, false, `variable "RecursiveCountVariable"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Cedar_Member_Update.flow-meta.xml")
			if err := os.WriteFile(path, []byte(build(tc.status, tc.trigger, tc.body)), 0o644); err != nil {
				t.Fatal(err)
			}
			flow, diagnostics, err := loadFlow(path)
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
