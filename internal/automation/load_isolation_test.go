package automation

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

const isolationValidFlow = `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start>
    <object>Widget__c</object>
    <triggerType>RecordBeforeSave</triggerType>
    <connector><targetReference>Set_Status</targetReference></connector>
  </start>
  <assignments>
    <name>Set_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>flowed</stringValue></value>
    </assignmentItems>
  </assignments>
</Flow>`

const isolationBrokenFlow = `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Widget__c</object>`

const isolationValidWorkflow = `<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
  <fieldUpdates><fullName>Set_Stage</fullName><field>Stage__c</field><literalValue>Ready</literalValue></fieldUpdates>
  <rules>
    <fullName>Stage_Rule</fullName>
    <active>true</active>
    <criteriaItems><field>Widget__c.Name</field><operation>equals</operation><value>Acme</value></criteriaItems>
    <actions><name>Set_Stage</name><type>FieldUpdate</type></actions>
  </rules>
</Workflow>`

func TestLoadProjectIsolatesUnparsableFlowFile(t *testing.T) {
	root := t.TempDir()
	flows := filepath.Join(root, "force-app/main/default/flows")
	broken := filepath.Join(flows, "Broken_Flow.flow-meta.xml")
	valid := filepath.Join(flows, "Widget_Status.flow-meta.xml")
	workflow := filepath.Join(root, "force-app/main/default/workflows/Widget__c.workflow-meta.xml")
	writeWorkflowTestFile(t, broken, isolationBrokenFlow)
	writeWorkflowTestFile(t, valid, isolationValidFlow)
	writeWorkflowTestFile(t, workflow, isolationValidWorkflow)

	idx, err := LoadProject(project.Project{FlowFiles: []string{broken, valid}, WorkflowFiles: []string{workflow}})
	if err != nil {
		t.Fatalf("LoadProject() error = %v; one bad flow must not fail the project", err)
	}
	if len(idx.Flows) != 1 || idx.Flows[0].File != valid || len(idx.Flows[0].Rules) != 1 || idx.Flows[0].Rules[0].Name != "Widget_Status" {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	if len(idx.Workflows) != 1 || len(idx.Workflows[0].Rules) != 1 {
		t.Fatalf("workflows = %#v", idx.Workflows)
	}
	if len(idx.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	diag := idx.Diagnostics[0]
	if diag.Severity != "warning" || diag.Code != "GLADEAUTO003" || diag.File != broken ||
		!strings.Contains(diag.Message, "Broken_Flow") || !strings.Contains(diag.Message, "XML syntax error") {
		t.Fatalf("diagnostic = %#v", diag)
	}

	control, err := LoadProject(project.Project{FlowFiles: []string{valid}, WorkflowFiles: []string{workflow}})
	if err != nil {
		t.Fatal(err)
	}
	if len(control.Diagnostics) != 0 || len(control.Flows) != 1 || len(control.Workflows) != 1 {
		t.Fatalf("control index = %#v", control)
	}
}

func TestLoadProjectIsolatesUnparsableWorkflowFile(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "force-app/main/default/flows/Widget_Status.flow-meta.xml")
	broken := filepath.Join(root, "force-app/main/default/workflows/Gadget__c.workflow-meta.xml")
	workflow := filepath.Join(root, "force-app/main/default/workflows/Widget__c.workflow-meta.xml")
	writeWorkflowTestFile(t, valid, isolationValidFlow)
	writeWorkflowTestFile(t, broken, `<Workflow><rules><fullName>Open`)
	writeWorkflowTestFile(t, workflow, isolationValidWorkflow)

	idx, err := LoadProject(project.Project{FlowFiles: []string{valid}, WorkflowFiles: []string{broken, workflow}})
	if err != nil {
		t.Fatalf("LoadProject() error = %v; one bad workflow must not fail the project", err)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	if len(idx.Workflows) != 1 || idx.Workflows[0].ObjectName != "Widget__c" || len(idx.Workflows[0].Rules) != 1 {
		t.Fatalf("workflows = %#v", idx.Workflows)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO003" || idx.Diagnostics[0].File != broken ||
		!strings.Contains(idx.Diagnostics[0].Message, "Gadget__c") {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectWarnsOnUnsupportedStartFilterLogic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Either.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start>
    <object>Widget__c</object>
    <triggerType>RecordBeforeSave</triggerType>
    <filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Acme</stringValue></value></filters>
    <filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Beta</stringValue></value></filters>
    <filterLogic>1 XOR 2</filterLogic>
    <connector><targetReference>Set_Status</targetReference></connector>
  </start>
  <assignments>
    <name>Set_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>flowed</stringValue></value>
    </assignmentItems>
  </assignments>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	// The warning does not change what loads: the rule and its criteria stay
	// as they were before the warning existed.
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 || len(idx.Flows[0].Rules[0].Criteria) != 2 || idx.Flows[0].Rules[0].Formula != "" {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO002" || idx.Diagnostics[0].File != path ||
		!strings.Contains(idx.Diagnostics[0].Message, "Widget_Either") || !strings.Contains(idx.Diagnostics[0].Message, `start filter logic "1 XOR 2"`) {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectKeepsStartFilterOrAndCustomLogicAsAlternatives(t *testing.T) {
	for _, tc := range []struct {
		logic string
		want  [][]int
	}{
		{"or", [][]int{{0}, {1}, {2}}},
		{"1 OR (2 AND 3)", [][]int{{0}, {1, 2}}},
		{"(1 OR 2) AND 3", [][]int{{0, 2}, {1, 2}}},
	} {
		t.Run(tc.logic, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "force-app/main/default/flows/Widget_Either.flow-meta.xml")
			writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start>
    <object>Widget__c</object>
    <triggerType>RecordBeforeSave</triggerType>
    <recordTriggerType>Create</recordTriggerType>
    <filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Acme</stringValue></value></filters>
    <filters><field>Size__c</field><operator>LessThan</operator><value><numberValue>5</numberValue></value></filters>
    <filters><field>Kind__c</field><operator>NotEqualTo</operator><value><stringValue>x</stringValue></value></filters>
    <filterLogic>`+tc.logic+`</filterLogic>
    <connector><targetReference>Set_Status</targetReference></connector>
  </start>
  <assignments>
    <name>Set_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>flowed</stringValue></value>
    </assignmentItems>
  </assignments>
</Flow>`)
			idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) != 0 || len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
				t.Fatalf("diagnostics = %#v flows = %#v", idx.Diagnostics, idx.Flows)
			}
			rule := idx.Flows[0].Rules[0]
			if len(rule.Criteria) != 3 || rule.Formula != "" || !reflect.DeepEqual(rule.CriteriaGroups, tc.want) {
				t.Fatalf("criteria=%d formula=%q groups=%v, want %v", len(rule.Criteria), rule.Formula, rule.CriteriaGroups, tc.want)
			}
		})
	}
}
