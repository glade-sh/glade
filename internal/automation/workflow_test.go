package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/storage"
)

func TestLoadProjectWorkflowFieldUpdatesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/workflows/Account.workflow-meta.xml")
	writeWorkflowTestFile(t, path, `<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
  <rules>
    <fullName>MarkActive</fullName>
    <active>true</active>
    <criteriaItems><field>Account.Name</field><operation>equals</operation><value>Acme</value></criteriaItems>
    <actions><name>SetStatus</name><type>FieldUpdate</type></actions>
    <actions><name>SendMail</name><type>Alert</type></actions>
  </rules>
  <fieldUpdates>
    <fullName>SetStatus</fullName>
    <field>Account.Status__c</field>
    <literalValue>Active</literalValue>
  </fieldUpdates>
  <alerts>
    <fullName>SendMail</fullName>
    <template>welcome</template>
    <recipients><type>user</type><recipient>workflow@example.test</recipient></recipients>
  </alerts>
</Workflow>`)
	idx, err := LoadProject(project.Project{WorkflowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Workflows) != 1 || idx.Workflows[0].ObjectName != "Account" {
		t.Fatalf("workflows = %#v", idx.Workflows)
	}
	rules := idx.Workflows[0].Rules
	if len(rules) != 1 || rules[0].Name != "MarkActive" || len(rules[0].Criteria) != 1 || len(rules[0].FieldUpdates) != 1 || len(rules[0].EmailAlerts) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
	if rules[0].EmailAlerts[0].Template != "welcome" || rules[0].EmailAlerts[0].Recipients[0].Recipient != "workflow@example.test" {
		t.Fatalf("email alerts = %#v", rules[0].EmailAlerts)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestApplyToOrgInstallsWorkflowRules(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	ApplyToOrg(&org, Index{Workflows: []Workflow{{ObjectName: "Account", Rules: []storage.WorkflowRule{{Name: "Rule", Active: true}}}}})
	if got := org.Objects["Account"].Definition.WorkflowRules; len(got) != 1 || got[0].Name != "Rule" {
		t.Fatalf("workflow rules = %#v", got)
	}
}

func TestLoadProjectFlowFieldUpdatesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Status.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start>
    <object>Widget__c</object>
    <triggerType>RecordAfterSave</triggerType>
    <filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Acme</stringValue></value></filters>
  </start>
  <recordUpdates>
    <name>Update_Status</name>
    <inputReference>$Record</inputReference>
    <inputAssignments><field>Status__c</field><value><stringValue>FlowActive</stringValue></value></inputAssignments>
  </recordUpdates>
  <screens><name>UnsupportedScreen</name></screens>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Flows) != 1 || idx.Flows[0].ObjectName != "Widget__c" {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	rules := idx.Flows[0].Rules
	if len(rules) != 1 || rules[0].Name != "Widget_Status" || len(rules[0].Criteria) != 1 || len(rules[0].FieldUpdates) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "OAERAUTO002" {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowDecisionAssignments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Decision.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
  <decisions>
    <name>Check_Name</name>
    <rules>
      <name>Name_Is_Acme</name>
      <conditions><leftValueReference>$Record.Name</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Acme</stringValue></rightValue></conditions>
    </rules>
  </decisions>
  <assignments>
    <name>Assign_Status</name>
    <assignmentItems><assignToReference>$Record.Status__c</assignToReference><operator>Assign</operator><value><stringValue>FlowActive</stringValue></value></assignmentItems>
  </assignments>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	rule := idx.Flows[0].Rules[0]
	if len(rule.Criteria) != 1 || rule.Criteria[0].Field != "Name" || rule.Criteria[0].Value != "Acme" {
		t.Fatalf("criteria = %#v", rule.Criteria)
	}
	if len(rule.FieldUpdates) != 1 || rule.FieldUpdates[0].Field != "Status__c" || rule.FieldUpdates[0].LiteralValue != "FlowActive" {
		t.Fatalf("field updates = %#v", rule.FieldUpdates)
	}
}

func TestLoadProjectFlowApexActionCalls(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Action.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
  <actionCalls>
    <name>Invoke_Widget_Action</name>
    <actionType>apex</actionType>
    <actionName>WidgetFlowAction.run</actionName>
  </actionCalls>
  <actionCalls>
    <name>Unsupported_Email</name>
    <actionType>emailAlert</actionType>
    <actionName>Send_Widget_Email</actionName>
  </actionCalls>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	actions := idx.Flows[0].Rules[0].Actions
	if len(actions) != 1 || actions[0].ClassName != "WidgetFlowAction" || actions[0].MethodName != "run" {
		t.Fatalf("actions = %#v", actions)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "OAERAUTO002" {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowProcessBuilderFormulaAndTypedValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Process.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>Workflow</processType>
  <status>Active</status>
  <formulas>
    <name>Should_Run</name>
    <dataType>Boolean</dataType>
    <expression>{!$Record.Name} = "Acme" &amp;&amp; {!$Record.Score__c} &gt;= 10</expression>
  </formulas>
  <start>
    <object>Widget__c</object>
    <triggerType>RecordAfterSave</triggerType>
    <filterLogic>Should_Run</filterLogic>
  </start>
  <assignments>
    <name>Assign_Process_Status</name>
    <assignmentItems><assignToReference>$Record.Status__c</assignToReference><operator>Assign</operator><value><formula>"Process-" &amp; {!$Record.Name}</formula></value></assignmentItems>
    <assignmentItems><assignToReference>$Record.Active__c</assignToReference><operator>Assign</operator><value><booleanValue>true</booleanValue></value></assignmentItems>
    <assignmentItems><assignToReference>$Record.Score_Copy__c</assignToReference><operator>Assign</operator><value><elementReference>$Record.Score__c</elementReference></value></assignmentItems>
  </assignments>
  <recordLookups><name>Unsupported_Lookup</name></recordLookups>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	rule := idx.Flows[0].Rules[0]
	if rule.ProcessType != "Workflow" || rule.Formula != `Name = "Acme" && Score__c >= 10` {
		t.Fatalf("rule = %#v", rule)
	}
	if len(rule.Criteria) != 0 {
		t.Fatalf("criteria should be replaced by formula: %#v", rule.Criteria)
	}
	if len(rule.FieldUpdates) != 3 {
		t.Fatalf("field updates = %#v", rule.FieldUpdates)
	}
	if rule.FieldUpdates[0].Formula != `"Process-" & Name` {
		t.Fatalf("formula update = %#v", rule.FieldUpdates[0])
	}
	if rule.FieldUpdates[1].LiteralValue != "true" {
		t.Fatalf("boolean update = %#v", rule.FieldUpdates[1])
	}
	if rule.FieldUpdates[2].SourceField != "Score__c" {
		t.Fatalf("source update = %#v", rule.FieldUpdates[2])
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "OAERAUTO002" || !strings.Contains(idx.Diagnostics[0].Message, "record lookup") {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestApplyToOrgInstallsFlowRules(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Widget__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	ApplyToOrg(&org, Index{Flows: []Flow{{ObjectName: "Widget__c", Rules: []storage.FlowRule{{Name: "Rule", Active: true}}}}})
	if got := org.Objects["Widget__c"].Definition.FlowRules; len(got) != 1 || got[0].Name != "Rule" {
		t.Fatalf("flow rules = %#v", got)
	}
}

func writeWorkflowTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
