package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
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
    <actions><name>FollowUp</name><type>Task</type></actions>
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
  <tasks>
    <fullName>FollowUp</fullName>
    <subject>Follow up on Acme</subject>
    <status>Not Started</status>
    <priority>Normal</priority>
    <dueDateOffset>7</dueDateOffset>
  </tasks>
</Workflow>`)
	idx, err := LoadProject(project.Project{WorkflowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Workflows) != 1 || idx.Workflows[0].ObjectName != "Account" {
		t.Fatalf("workflows = %#v", idx.Workflows)
	}
	rules := idx.Workflows[0].Rules
	if len(rules) != 1 || rules[0].Name != "MarkActive" || len(rules[0].Criteria) != 1 || len(rules[0].FieldUpdates) != 1 || len(rules[0].EmailAlerts) != 1 || len(rules[0].Tasks) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
	if rules[0].EmailAlerts[0].Template != "welcome" || rules[0].EmailAlerts[0].Recipients[0].Recipient != "workflow@example.test" {
		t.Fatalf("email alerts = %#v", rules[0].EmailAlerts)
	}
	if rules[0].Tasks[0].Subject != "Follow up on Acme" || rules[0].Tasks[0].Status != "Not Started" || rules[0].Tasks[0].DueDateOffset != 7 || !rules[0].Tasks[0].HasDueDateOffset {
		t.Fatalf("tasks = %#v", rules[0].Tasks)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectWorkflowLegacyExtensionAndSimpleBooleanFilter(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src/workflows/Widget__c.workflow")
	writeWorkflowTestFile(t, path, `<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
  <fieldUpdates><fullName>SetStatus</fullName><field>Widget__c.Status__c</field><literalValue>Active</literalValue></fieldUpdates>
  <fieldUpdates><fullName>InactiveUpdate</fullName><field>Widget__c.Status__c</field><literalValue>Inactive</literalValue></fieldUpdates>
  <rules>
    <fullName>MarkActive</fullName>
    <active>true</active>
    <booleanFilter>1 AND 2</booleanFilter>
    <criteriaItems><field>Widget__c.Name</field><operation>notEqual</operation></criteriaItems>
    <criteriaItems><field>Widget__c.Status__c</field><operation>equals</operation></criteriaItems>
    <actions><name>SetStatus</name><type>FieldUpdate</type></actions>
  </rules>
  <rules>
    <fullName>InactiveComplexFilter</fullName>
    <active>false</active>
    <booleanFilter>1 OR 2</booleanFilter>
    <criteriaItems><field>Widget__c.Name</field><operation>equals</operation><value>Acme</value></criteriaItems>
    <criteriaItems><field>Widget__c.Status__c</field><operation>equals</operation></criteriaItems>
    <actions><name>InactiveUpdate</name><type>FieldUpdate</type></actions>
  </rules>
</Workflow>`)
	idx, err := LoadProject(project.Project{WorkflowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Workflows) != 1 || idx.Workflows[0].ObjectName != "Widget__c" {
		t.Fatalf("workflows = %#v", idx.Workflows)
	}
	rules := idx.Workflows[0].Rules
	if len(rules) != 2 || len(rules[0].Criteria) != 2 || len(rules[0].FieldUpdates) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestLoadProjectWorkflowIgnoresInactiveUnsupportedActionsAndActionlessRules(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src/workflows/Account.workflow")
	writeWorkflowTestFile(t, path, `<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
  <rules>
    <fullName>ActiveCriteriaOnly</fullName>
    <active>true</active>
    <booleanFilter>1 OR 2</booleanFilter>
    <criteriaItems><field>Account.AccountSource</field><operation>equals</operation><value>Data.com</value></criteriaItems>
    <criteriaItems><field>Account.Account_Source_Custom__c</field><operation>contains</operation><value>Data.com</value></criteriaItems>
  </rules>
  <rules>
    <fullName>InactiveTask</fullName>
    <actions><name>FollowUp</name><type>Task</type></actions>
    <active>false</active>
    <criteriaItems><field>Account.Name</field><operation>equals</operation><value>Acme</value></criteriaItems>
  </rules>
  <rules>
    <fullName>ActiveTask</fullName>
    <actions><name>FollowUp</name><type>Task</type></actions>
    <active>true</active>
    <criteriaItems><field>Account.Name</field><operation>equals</operation><value>Acme</value></criteriaItems>
  </rules>
  <tasks><fullName>FollowUp</fullName><subject>Follow up</subject><status>Not Started</status><priority>Normal</priority></tasks>
</Workflow>`)
	idx, err := LoadProject(project.Project{WorkflowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Workflows) != 1 || len(idx.Workflows[0].Rules) != 2 {
		t.Fatalf("workflow side-effect rules = %#v", idx.Workflows)
	}
	activeRule := idx.Workflows[0].Rules[0]
	if activeRule.Name != "ActiveTask" || len(activeRule.Tasks) != 1 || activeRule.Tasks[0].Subject != "Follow up" {
		t.Fatalf("active task rule = %#v", activeRule)
	}
	inactiveRule := idx.Workflows[0].Rules[1]
	if inactiveRule.Name != "InactiveTask" || inactiveRule.Active {
		t.Fatalf("inactive task rule = %#v", inactiveRule)
	}
}

func TestLoadProjectWorkflowResolvesSiblingFieldUpdates(t *testing.T) {
	root := t.TempDir()
	rulesPath := filepath.Join(root, "unpackaged/config/trial/workflows/Contact.workflow")
	updatesPath := filepath.Join(root, "unpackaged/config/trial_tso/workflows/Contact.workflow")
	writeWorkflowTestFile(t, rulesPath, `<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
  <rules>
    <fullName>CopyEmail</fullName>
    <active>true</active>
    <actions><name>ContactEmailUpdate</name><type>FieldUpdate</type></actions>
  </rules>
</Workflow>`)
	writeWorkflowTestFile(t, updatesPath, `<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
  <fieldUpdates>
    <fullName>ContactEmailUpdate</fullName>
    <field>Contact.OtherEmail__c</field>
    <formula>Email</formula>
  </fieldUpdates>
</Workflow>`)

	idx, err := LoadProject(project.Project{WorkflowFiles: []string{rulesPath, updatesPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	var ruleWorkflow Workflow
	for _, workflow := range idx.Workflows {
		if workflow.File == rulesPath {
			ruleWorkflow = workflow
			break
		}
	}
	if len(ruleWorkflow.Rules) != 1 || len(ruleWorkflow.Rules[0].FieldUpdates) != 1 {
		t.Fatalf("workflow = %#v", ruleWorkflow)
	}
	update := ruleWorkflow.Rules[0].FieldUpdates[0]
	if update.Field != "OtherEmail__c" || update.Formula != "Email" {
		t.Fatalf("field update = %#v", update)
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
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO002" {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowIgnoresNonRecordScreenFlowForSaveOrder(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/SetupWizard.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>Flow</processType>
  <status>Active</status>
  <screens><name>Wizard</name></screens>
  <recordLookups><name>Pick_Default</name></recordLookups>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Flows) != 1 || idx.Flows[0].ObjectName != "" || len(idx.Flows[0].Rules) != 0 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
}

func TestLoadProjectFlowIgnoresOrchestratorFlowsForSaveOrder(t *testing.T) {
	root := t.TempDir()
	var files []string
	for _, processType := range []string{"Orchestrator", "AppProcess"} {
		path := filepath.Join(root, "force-app/main/default/flows", processType+".flow-meta.xml")
		files = append(files, path)
		writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>`+processType+`</processType>
  <status>Active</status>
  <start><object>Case</object><triggerType>RecordAfterSave</triggerType></start>
  <screens><name>Approval</name></screens>
</Flow>`)
	}
	idx, err := LoadProject(project.Project{FlowFiles: files})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Flows) != 2 || idx.Flows[0].ObjectName != "Case" || len(idx.Flows[0].Rules) != 0 || len(idx.Flows[1].Rules) != 0 {
		t.Fatalf("flows = %#v", idx.Flows)
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

func TestLoadProjectFlowRoutedDecisionBranches(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Decision_Branches.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
  <decisions>
    <name>Route_Status</name>
    <rules>
      <name>Hot</name>
      <conditionLogic>and</conditionLogic>
      <conditions><leftValueReference>$Record.Score__c</leftValueReference><operator>GreaterThanOrEqualTo</operator><rightValue><numberValue>90</numberValue></rightValue></conditions>
      <connector><targetReference>Assign_Hot</targetReference></connector>
    </rules>
    <rules>
      <name>Warm</name>
      <conditionLogic>and</conditionLogic>
      <conditions><leftValueReference>$Record.Score__c</leftValueReference><operator>GreaterThanOrEqualTo</operator><rightValue><numberValue>50</numberValue></rightValue></conditions>
      <connector><targetReference>Assign_Warm</targetReference></connector>
    </rules>
    <defaultConnector><targetReference>Assign_Cold</targetReference></defaultConnector>
  </decisions>
  <assignments>
    <name>Assign_Hot</name>
    <assignmentItems><assignToReference>$Record.Status__c</assignToReference><operator>Assign</operator><value><stringValue>Hot</stringValue></value></assignmentItems>
  </assignments>
  <assignments>
    <name>Assign_Warm</name>
    <assignmentItems><assignToReference>$Record.Status__c</assignToReference><operator>Assign</operator><value><stringValue>Warm</stringValue></value></assignmentItems>
  </assignments>
  <assignments>
    <name>Assign_Cold</name>
    <assignmentItems><assignToReference>$Record.Status__c</assignToReference><operator>Assign</operator><value><stringValue>Cold</stringValue></value></assignmentItems>
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
	if len(rule.Branches) != 3 {
		t.Fatalf("branches = %#v", rule.Branches)
	}
	if len(rule.FieldUpdates) != 0 {
		t.Fatalf("routed decision should not keep global field updates: %#v", rule.FieldUpdates)
	}
	if rule.Branches[0].Name != "Hot" || rule.Branches[0].Criteria[0].Field != "Score__c" || rule.Branches[0].FieldUpdates[0].LiteralValue != "Hot" {
		t.Fatalf("hot branch = %#v", rule.Branches[0])
	}
	if !rule.Branches[2].Default || rule.Branches[2].FieldUpdates[0].LiteralValue != "Cold" {
		t.Fatalf("default branch = %#v", rule.Branches[2])
	}
}

func TestLoadProjectFlowRoutedDecisionBranchesSupportOrOfAndConditionLogic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Priority.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Widget__c</object><triggerType>RecordBeforeSave</triggerType></start>
  <decisions>
    <name>Route_Priority</name>
    <rules>
      <name>Moderate</name>
      <conditionLogic>(1 AND 2) OR (3 AND 4)</conditionLogic>
      <conditions><leftValueReference>$Record.Impact__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Medium</stringValue></rightValue></conditions>
      <conditions><leftValueReference>$Record.Urgency__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>High</stringValue></rightValue></conditions>
      <conditions><leftValueReference>$Record.Impact__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>High</stringValue></rightValue></conditions>
      <conditions><leftValueReference>$Record.Urgency__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Medium</stringValue></rightValue></conditions>
      <connector><targetReference>Set_Moderate</targetReference></connector>
    </rules>
  </decisions>
  <recordUpdates>
    <name>Set_Moderate</name>
    <inputReference>$Record</inputReference>
    <inputAssignments><field>Priority__c</field><value><stringValue>Moderate</stringValue></value></inputAssignments>
  </recordUpdates>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	branches := idx.Flows[0].Rules[0].Branches
	if len(branches) != 2 {
		t.Fatalf("branches = %#v", branches)
	}
	if len(branches[0].Criteria) != 2 || branches[0].Criteria[0].Value != "Medium" || branches[0].FieldUpdates[0].LiteralValue != "Moderate" {
		t.Fatalf("first branch = %#v", branches[0])
	}
	if len(branches[1].Criteria) != 2 || branches[1].Criteria[0].Value != "High" || branches[1].FieldUpdates[0].LiteralValue != "Moderate" {
		t.Fatalf("second branch = %#v", branches[1])
	}
}

func TestLoadProjectFlowRoutedDecisionBranchWithoutConditions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Default_Route.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Widget__c</object><triggerType>RecordBeforeSave</triggerType></start>
  <decisions>
    <name>Route_Default</name>
    <rules>
      <name>Always</name>
      <connector><targetReference>Set_Default</targetReference></connector>
    </rules>
  </decisions>
  <recordUpdates>
    <name>Set_Default</name>
    <inputReference>$Record</inputReference>
    <inputAssignments><field>Status__c</field><value><stringValue>Defaulted</stringValue></value></inputAssignments>
  </recordUpdates>
</Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	branches := idx.Flows[0].Rules[0].Branches
	if len(branches) != 1 || len(branches[0].Criteria) != 0 || branches[0].FieldUpdates[0].LiteralValue != "Defaulted" {
		t.Fatalf("branches = %#v", branches)
	}
}

func TestLoadProjectFlowOrderedDecisionBranchModelsRelatedRecordUpdate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Close_Task.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><connector><targetReference>Route_Task</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
  <decisions>
    <name>Route_Task</name>
    <rules>
      <name>Close_Task</name>
      <conditions><leftValueReference>$Record.Status__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Closed</stringValue></rightValue></conditions>
      <connector><targetReference>Update_Task_Associated</targetReference></connector>
    </rules>
  </decisions>
  <recordUpdates>
    <name>Update_Task_Associated</name>
    <object>Task</object>
    <filters><field>WhatId</field><operator>EqualTo</operator><value><elementReference>$Record.Id</elementReference></value></filters>
    <inputAssignments><field>Status</field><value><stringValue>Completed</stringValue></value></inputAssignments>
  </recordUpdates>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	rule := idx.Flows[0].Rules[0]
	if len(rule.FieldUpdates) != 0 {
		t.Fatalf("field updates = %#v", rule.FieldUpdates)
	}
	steps := rule.Steps
	if len(steps) != 1 || steps[0].Kind != "decision" {
		t.Fatalf("steps = %#v", steps)
	}
	branches := steps[0].Branches
	if len(branches) != 1 || branches[0].Name != "Close_Task" || len(branches[0].Steps) != 1 || branches[0].Steps[0].Kind != "recordUpdate" {
		t.Fatalf("branches = %#v", branches)
	}
	update := branches[0].Steps[0].RecordUpdate
	if update.ObjectName != "Task" || len(update.Criteria) != 1 || update.Criteria[0].Field != "WhatId" || update.Criteria[0].SourceField != "Id" {
		t.Fatalf("record update = %#v", update)
	}
	if len(update.InputAssignments) != 1 || update.InputAssignments[0].Field != "Status" || update.InputAssignments[0].LiteralValue != "Completed" {
		t.Fatalf("record update assignments = %#v", update.InputAssignments)
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
	if len(actions) != 2 || actions[0].ClassName != "WidgetFlowAction" || actions[0].MethodName != "run" {
		t.Fatalf("actions = %#v", actions)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowActionConnectorPreservesFollowingStep(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Action_Then_Update.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <actionCalls>
    <name>Notify</name>
    <actionType>apex</actionType>
    <actionName>NotifyWidget</actionName>
    <connector><targetReference>Set_Status</targetReference></connector>
  </actionCalls>
  <recordUpdates>
    <name>Set_Status</name>
    <inputReference>$Record</inputReference>
    <inputAssignments><field>Status__c</field><value><stringValue>Done</stringValue></value></inputAssignments>
  </recordUpdates>
  <start><connector><targetReference>Notify</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 2 || steps[0].Kind != "action" || steps[1].Kind != "fieldUpdate" {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestLoadProjectFlowFaultConnectorsAreNowSupported(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Faults.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups><name>Lookup</name><object>Account</object><faultConnector><targetReference>Fault</targetReference></faultConnector></recordLookups>
  <recordCreates><name>Create</name><object>Account</object><faultConnector><targetReference>Fault</targetReference></faultConnector></recordCreates>
  <loops><name>Loop</name><collectionReference>Lookup</collectionReference><faultConnector><targetReference>Fault</targetReference></faultConnector></loops>
  <actionCalls><name>Action</name><actionType>apex</actionType><actionName>NotifyWidget</actionName><faultConnector><targetReference>Fault</targetReference></faultConnector></actionCalls>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowChatterPostActionInputs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Chatter.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>MatchedUser</name>
    <object>User</object>
    <filters><field>Email</field><operator>EqualTo</operator><value><stringValue>owner@example.test</stringValue></value></filters>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <actionCalls>
    <name>Post_To_Chatter</name>
    <actionName>chatterPost</actionName>
    <actionType>chatterPost</actionType>
    <inputParameters><name>text</name><value><stringValue>Hello {!MatchedUser.Id}</stringValue></value></inputParameters>
    <inputParameters><name>subjectNameOrId</name><value><elementReference>$Record.Parent__r.Id</elementReference></value></inputParameters>
    <inputParameters><name>visibility</name><value><stringValue>allUsers</stringValue></value></inputParameters>
  </actionCalls>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	actions := idx.Flows[0].Rules[0].Actions
	if len(actions) != 1 || actions[0].ActionType != "chatterPost" || actions[0].ActionName != "chatterPost" {
		t.Fatalf("actions = %#v", actions)
	}
	if len(actions[0].Inputs) != 3 || actions[0].Inputs[0].LiteralValue != "Hello {!MatchedUser.Id}" || actions[0].Inputs[1].SourceField != "Parent__r.Id" {
		t.Fatalf("action inputs = %#v", actions[0].Inputs)
	}
}

func TestLoadProjectFlowStandardSideEffectActions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Standard_Actions.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <actionCalls>
    <name>Send_Quote_Email</name>
    <actionName>emailSimple</actionName>
    <actionType>emailSimple</actionType>
    <inputParameters><name>emailBody</name><value><stringValue>This is the quote.</stringValue></value></inputParameters>
    <inputParameters><name>emailAddresses</name><value><elementReference>$Record.Email__c</elementReference></value></inputParameters>
    <inputParameters><name>emailSubject</name><value><stringValue>AZ Insurance Quote</stringValue></value></inputParameters>
    <connector><targetReference>Notification_On_Adoption_Match</targetReference></connector>
  </actionCalls>
  <actionCalls>
    <name>Notification_On_Adoption_Match</name>
    <actionName>customNotificationAction</actionName>
    <actionType>customNotificationAction</actionType>
    <inputParameters><name>customNotifTypeId</name><value><elementReference>Get_Custom_Notification_ID.Id</elementReference></value></inputParameters>
    <inputParameters><name>recipientIds</name><value><elementReference>notificationRecipients</elementReference></value></inputParameters>
  </actionCalls>
  <recordLookups>
    <name>Get_Custom_Notification_ID</name>
    <object>CustomNotificationType</object>
    <filters><field>DeveloperName</field><operator>EqualTo</operator><value><stringValue>Animal_Adoption_Match</stringValue></value></filters>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <start><connector><targetReference>Send_Quote_Email</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 2 || steps[0].Action.ActionType != "emailSimple" || steps[1].Action.ActionType != "customNotificationAction" {
		t.Fatalf("steps = %#v", steps)
	}
	if got := steps[0].Action.Inputs[1].SourceField; got != "Email__c" {
		t.Fatalf("email source field = %q", got)
	}
}

func TestLoadProjectFlowActionFaultConnectorIsNowSupported(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Action_Fault.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <actionCalls>
    <name>Send_Quote_Email</name>
    <actionName>emailSimple</actionName>
    <actionType>emailSimple</actionType>
    <faultConnector><targetReference>Log_Email_Error</targetReference></faultConnector>
    <inputParameters><name>emailBody</name><value><stringValue>This is the quote.</stringValue></value></inputParameters>
    <inputParameters><name>emailAddresses</name><value><elementReference>$Record.Email__c</elementReference></value></inputParameters>
    <inputParameters><name>emailSubject</name><value><stringValue>AZ Insurance Quote</stringValue></value></inputParameters>
  </actionCalls>
  <recordCreates>
    <name>Log_Email_Error</name>
    <object>Error__c</object>
    <inputAssignments><field>Message__c</field><value><stringValue>email failed</stringValue></value></inputAssignments>
  </recordCreates>
  <start><connector><targetReference>Send_Quote_Email</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 1 || steps[0].Action.ActionType != "emailSimple" {
		t.Fatalf("steps = %#v", steps)
	}
	if steps[0].FaultTarget != "Log_Email_Error" {
		t.Fatalf("expected FaultTarget 'Log_Email_Error', got %q", steps[0].FaultTarget)
	}
}

func TestLoadProjectFlowFaultBranchBuiltForFaultOnlyRecoveryNode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Fault_Recovery.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <actionCalls>
    <name>Send_Email</name>
    <actionName>emailSimple</actionName>
    <actionType>emailSimple</actionType>
    <faultConnector><targetReference>Log_Error</targetReference></faultConnector>
    <connector><targetReference>Update_Status</targetReference></connector>
    <inputParameters><name>emailBody</name><value><stringValue>Body</stringValue></value></inputParameters>
    <inputParameters><name>emailAddresses</name><value><stringValue>test@example.test</stringValue></value></inputParameters>
    <inputParameters><name>emailSubject</name><value><stringValue>Subject</stringValue></value></inputParameters>
  </actionCalls>
  <recordCreates>
    <name>Log_Error</name>
    <object>Error__c</object>
    <inputAssignments><field>Message__c</field><value><stringValue>email failed</stringValue></value></inputAssignments>
  </recordCreates>
  <assignments>
    <name>Update_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Done</stringValue></value>
    </assignmentItems>
  </assignments>
  <start><connector><targetReference>Send_Email</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps (Send_Email, Update_Status), got %d: %#v", len(steps), steps)
	}
	emailStep := steps[0]
	if emailStep.Kind != "action" || emailStep.FaultTarget != "Log_Error" {
		t.Fatalf("email step = %#v", emailStep)
	}
	if len(emailStep.FaultBranch) != 1 {
		t.Fatalf("expected FaultBranch with 1 step (Log_Error), got %d: %#v", len(emailStep.FaultBranch), emailStep.FaultBranch)
	}
	faultStep := emailStep.FaultBranch[0]
	if faultStep.Kind != "recordCreate" || faultStep.RecordCreate.Name != "Log_Error" || faultStep.RecordCreate.ObjectName != "Error__c" {
		t.Fatalf("fault branch step = %#v", faultStep)
	}
	if faultStep.FaultTarget != "" {
		t.Fatalf("fault step should not have a secondary fault target, got %q", faultStep.FaultTarget)
	}
}

func TestLoadProjectFlowFaultBranchEmptyWhenTargetInMainBranch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Fault_In_Branch.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <actionCalls>
    <name>Send_Email</name>
    <actionName>emailSimple</actionName>
    <actionType>emailSimple</actionType>
    <faultConnector><targetReference>Update_Status</targetReference></faultConnector>
    <connector><targetReference>Update_Status</targetReference></connector>
    <inputParameters><name>emailBody</name><value><stringValue>Body</stringValue></value></inputParameters>
    <inputParameters><name>emailAddresses</name><value><stringValue>test@example.test</stringValue></value></inputParameters>
    <inputParameters><name>emailSubject</name><value><stringValue>Subject</stringValue></value></inputParameters>
  </actionCalls>
  <assignments>
    <name>Update_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Done</stringValue></value>
    </assignmentItems>
  </assignments>
  <start><connector><targetReference>Send_Email</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps (Send_Email, Update_Status), got %d: %#v", len(steps), steps)
	}
	emailStep := steps[0]
	if emailStep.FaultTarget != "Update_Status" {
		t.Fatalf("expected FaultTarget 'Update_Status', got %q", emailStep.FaultTarget)
	}
	if len(emailStep.FaultBranch) != 0 {
		t.Fatalf("FaultBranch should be empty when target is in main branch, got %#v", emailStep.FaultBranch)
	}
}

func TestLoadProjectFlowEnvironmentsDefaultIsAccepted(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Env_Default.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <environments>Default</environments>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("environments Default should not produce diagnostics: %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowORConditionLogicSingleCondition(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_OR_Single.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <decisions>
    <name>Check_Status</name>
    <defaultConnector><targetReference>Set_Ready</targetReference></defaultConnector>
    <rules>
      <name>Is_Done</name>
      <conditionLogic>or</conditionLogic>
      <conditions>
        <leftValueReference>$Record.Status__c</leftValueReference>
        <operator>EqualTo</operator>
        <rightValue><stringValue>Done</stringValue></rightValue>
      </conditions>
    </rules>
  </decisions>
  <assignments>
    <name>Set_Ready</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Ready</stringValue></value>
    </assignmentItems>
  </assignments>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("or condition logic with single condition should be supported: %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowORConditionLogicMultipleConditions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_OR_Multi.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <decisions>
    <name>Check_Status</name>
    <defaultConnector><targetReference>Set_Ready</targetReference></defaultConnector>
    <rules>
      <name>Is_Done</name>
      <conditionLogic>or</conditionLogic>
      <conditions>
        <leftValueReference>$Record.Status__c</leftValueReference>
        <operator>EqualTo</operator>
        <rightValue><stringValue>Done</stringValue></rightValue>
      </conditions>
      <conditions>
        <leftValueReference>$Record.Status__c</leftValueReference>
        <operator>EqualTo</operator>
        <rightValue><stringValue>Archived</stringValue></rightValue>
      </conditions>
      <connector><targetReference>Set_Done</targetReference></connector>
    </rules>
  </decisions>
  <assignments>
    <name>Set_Done</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Done</stringValue></value>
    </assignmentItems>
  </assignments>
  <assignments>
    <name>Set_Ready</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Ready</stringValue></value>
    </assignmentItems>
  </assignments>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("or condition logic with multiple conditions should be supported: %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowFormulaReferenceInDecisionCondition(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Formula_Cond.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <formulas>
    <name>NoMoreCapacity</name>
    <dataType>Boolean</dataType>
    <expression>{!$Record.Availability__c} = 0</expression>
  </formulas>
  <decisions>
    <name>Has_Capacity</name>
    <defaultConnector><targetReference>Set_Full</targetReference></defaultConnector>
    <rules>
      <name>At_Capacity</name>
      <conditionLogic>and</conditionLogic>
      <conditions>
        <leftValueReference>NoMoreCapacity</leftValueReference>
        <operator>EqualTo</operator>
        <rightValue><booleanValue>true</booleanValue></rightValue>
      </conditions>
      <connector><targetReference>Set_Full</targetReference></connector>
    </rules>
  </decisions>
  <assignments>
    <name>Set_Full</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Full</stringValue></value>
    </assignmentItems>
  </assignments>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("formula reference in decision condition should be supported: %#v", idx.Diagnostics)
	}
	branches := idx.Flows[0].Rules[0].Branches
	if len(branches) == 0 || branches[0].Formula != "(Availability__c = 0) = true" || len(branches[0].Criteria) != 0 {
		t.Fatalf("formula decision branch should be stored as executable formula, got %#v", branches)
	}
}

func TestLoadProjectFlowRecordReferenceObjectResolution(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Ref_Record_Update.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordUpdates>
    <name>Update_Parent</name>
    <inputAssignments><field>Name</field><value><stringValue>Updated</stringValue></value></inputAssignments>
    <inputReference>$Record.Parent__r</inputReference>
  </recordUpdates>
  <start><connector><targetReference>Update_Parent</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("$Record.Parent__r record update should resolve: %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 1 || steps[0].Kind != "recordUpdate" || steps[0].RecordUpdate.ObjectName != "Parent__c" {
		t.Fatalf("expected recordUpdate step with ObjectName=Parent__c, got %#v", steps)
	}
}

func TestLoadProjectFlowCustomErrorFollowsConnectorGraph(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_Custom_Error.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <assignments>
    <name>Set_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>Ready</stringValue></value>
    </assignmentItems>
  </assignments>
  <customErrors>
    <name>Block_Record</name>
    <customErrorMessages><message>Blocked</message></customErrorMessages>
  </customErrors>
  <start><connector><targetReference>Set_Status</targetReference></connector><object>Widget__c</object><triggerType>RecordBeforeSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 1 || steps[0].Kind != "fieldUpdate" {
		t.Fatalf("steps = %#v", steps)
	}

	path = filepath.Join(root, "force-app/main/default/flows/Widget_Custom_Error_Reached.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <customErrors>
    <name>Block_Record</name>
    <customErrorMessages><message>Blocked</message></customErrorMessages>
  </customErrors>
  <start><connector><targetReference>Block_Record</targetReference></connector><object>Widget__c</object><triggerType>RecordBeforeSave</triggerType></start>
</Flow>`)
	idx, err = LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps = idx.Flows[0].Rules[0].Steps
	if len(steps) != 1 || steps[0].Kind != "customError" || steps[0].CustomError.Messages[0].Message != "Blocked" {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestLoadProjectFlowNoOpSideEffectActionsAreModeled(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/flows/Widget_NoOp_Actions.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <actionCalls>
    <name>Run_Subflow_Action</name>
    <actionName>Widget_Helper_Flow</actionName>
    <actionType>flow</actionType>
  </actionCalls>
  <actionCalls>
    <name>Open_Quick_Action</name>
    <actionType>quickAction</actionType>
  </actionCalls>
  <start><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	actions := idx.Flows[0].Rules[0].Actions
	if len(actions) != 2 || actions[0].ActionType != "flow" || actions[1].ActionType != "quickAction" {
		t.Fatalf("actions = %#v", actions)
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
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO002" || !strings.Contains(idx.Diagnostics[0].Message, "record lookup") {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
}

func TestLoadProjectFlowBeforeDeleteLookupAndCreate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Propagate_Delete.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <decisions>
    <name>HasSourceValues</name>
    <rules>
      <name>HasValues</name>
      <conditionLogic>and</conditionLogic>
      <conditions><leftValueReference>SourceName</leftValueReference><operator>IsNull</operator><rightValue><booleanValue>false</booleanValue></rightValue></conditions>
      <connector><targetReference>PendingRequest</targetReference></connector>
    </rules>
  </decisions>
  <decisions>
    <name>ExistingRequest</name>
    <defaultConnector><targetReference>CreateRequest</targetReference></defaultConnector>
    <rules>
      <name>Exists</name>
      <conditionLogic>and</conditionLogic>
      <conditions><leftValueReference>PendingRequest</leftValueReference><operator>IsNull</operator><rightValue><booleanValue>false</booleanValue></rightValue></conditions>
    </rules>
  </decisions>
  <recordLookups>
    <name>PendingRequest</name>
    <object>ActionRequest__c</object>
    <filterLogic>and</filterLogic>
    <filters><field>SourceRecordId__c</field><operator>EqualTo</operator><value><elementReference>$Record.Id</elementReference></value></filters>
    <filters><field>ActionName__c</field><operator>EqualTo</operator><value><stringValue>Delete</stringValue></value></filters>
    <connector><targetReference>ExistingRequest</targetReference></connector>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <recordCreates>
    <name>CreateRequest</name>
    <object>ActionRequest__c</object>
    <inputAssignments><field>ActionName__c</field><value><stringValue>Delete</stringValue></value></inputAssignments>
    <inputAssignments><field>Payload__c</field><value><elementReference>Payload</elementReference></value></inputAssignments>
    <inputAssignments><field>SourceRecordId__c</field><value><elementReference>$Record.Id</elementReference></value></inputAssignments>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordCreates>
  <start><connector><targetReference>HasSourceValues</targetReference></connector><object>Widget__c</object><triggerType>RecordBeforeDelete</triggerType></start>
  <variables><name>Payload</name><dataType>String</dataType><value><stringValue>{"id":"{!$Record.Id}"}</stringValue></value></variables>
  <variables><name>SourceName</name><dataType>String</dataType><value><elementReference>$Record.Name</elementReference></value></variables>
  <variables><name>PendingRequest</name><dataType>SObject</dataType><objectType>ActionRequest__c</objectType></variables>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	rule := idx.Flows[0].Rules[0]
	if len(rule.RecordLookups) != 1 || rule.RecordLookups[0].ObjectName != "ActionRequest__c" {
		t.Fatalf("lookups = %#v", rule.RecordLookups)
	}
	if len(rule.RecordLookups[0].Criteria) != 2 || rule.RecordLookups[0].Criteria[0].SourceField != "Id" {
		t.Fatalf("lookup criteria = %#v", rule.RecordLookups[0].Criteria)
	}
	if len(rule.RecordCreates) != 1 || rule.RecordCreates[0].ObjectName != "ActionRequest__c" {
		t.Fatalf("creates = %#v", rule.RecordCreates)
	}
	if len(rule.Branches) != 2 || rule.Branches[0].Criteria[0].Field != "Name" || !rule.Branches[1].Default {
		t.Fatalf("branches = %#v", rule.Branches)
	}
	assignments := rule.RecordCreates[0].InputAssignments
	if len(assignments) != 3 || assignments[1].LiteralValue != `{"id":"{!$Record.Id}"}` || assignments[2].SourceField != "Id" {
		t.Fatalf("create assignments = %#v", assignments)
	}
}

func TestLoadProjectFlowRecordCreateCanReferenceLookupOutputField(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Create_From_Lookup.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>MatchedAccount</name>
    <object>Account</object>
    <filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Acme</stringValue></value></filters>
    <connector><targetReference>CreateRequest</targetReference></connector>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <recordCreates>
    <name>CreateRequest</name>
    <object>ActionRequest__c</object>
    <inputAssignments><field>SourceRecordId__c</field><value><elementReference>MatchedAccount.Id</elementReference></value></inputAssignments>
    <inputAssignments><field>Payload__c</field><value><elementReference>MatchedAccount.Name</elementReference></value></inputAssignments>
  </recordCreates>
  <start><connector><targetReference>MatchedAccount</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	assignments := idx.Flows[0].Rules[0].RecordCreates[0].InputAssignments
	if len(assignments) != 2 || assignments[0].SourceField != "MatchedAccount.Id" || assignments[1].SourceField != "MatchedAccount.Name" {
		t.Fatalf("create assignments = %#v", assignments)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 2 || steps[0].Kind != "recordLookup" || steps[1].Kind != "recordCreate" {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestLoadProjectFlowRejectsLookupConditionWithEffectBranch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Lookup_Decision.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>PendingRequest</name>
    <object>ActionRequest__c</object>
    <filters><field>SourceRecordId__c</field><operator>EqualTo</operator><value><elementReference>$Record.Id</elementReference></value></filters>
    <connector><targetReference>HasRequest</targetReference></connector>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <decisions>
    <name>HasRequest</name>
    <rules>
      <name>Exists</name>
      <conditions><leftValueReference>PendingRequest</leftValueReference><operator>IsNull</operator><rightValue><booleanValue>false</booleanValue></rightValue></conditions>
      <connector><targetReference>CreateRequest</targetReference></connector>
    </rules>
  </decisions>
  <recordCreates>
    <name>CreateRequest</name>
    <object>ActionRequest__c</object>
    <inputAssignments><field>SourceRecordId__c</field><value><elementReference>$Record.Id</elementReference></value></inputAssignments>
  </recordCreates>
  <start><connector><targetReference>PendingRequest</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) == 0 {
		t.Fatalf("expected lookup decision diagnostic")
	}
}

func TestLoadProjectFlowLoopCollectionRecordCreate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Create_Issues.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>Get_Widgets</name>
    <object>Widget__c</object>
    <filters><field>Status__c</field><operator>EqualTo</operator><value><stringValue>Open</stringValue></value></filters>
    <connector><targetReference>Count_Widgets</targetReference></connector>
    <getFirstRecordOnly>false</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <assignments>
    <name>Count_Widgets</name>
    <assignmentItems><assignToReference>widgetCount</assignToReference><operator>AssignCount</operator><value><elementReference>Get_Widgets</elementReference></value></assignmentItems>
    <connector><targetReference>Enough_Widgets</targetReference></connector>
  </assignments>
  <decisions>
    <name>Enough_Widgets</name>
    <rules>
      <name>At_Least_Two</name>
      <conditions><leftValueReference>widgetCount</leftValueReference><operator>GreaterThanOrEqualTo</operator><rightValue><numberValue>2.0</numberValue></rightValue></conditions>
      <connector><targetReference>Create_Request</targetReference></connector>
    </rules>
  </decisions>
  <recordCreates>
    <name>Create_Request</name>
    <object>ActionRequest__c</object>
    <storeOutputAutomatically>true</storeOutputAutomatically>
    <connector><targetReference>Loop_Widgets</targetReference></connector>
  </recordCreates>
  <loops>
    <name>Loop_Widgets</name>
    <collectionReference>Get_Widgets</collectionReference>
    <nextValueConnector><targetReference>Build_Link</targetReference></nextValueConnector>
    <noMoreValuesConnector><targetReference>Create_Links</targetReference></noMoreValuesConnector>
  </loops>
  <assignments>
    <name>Build_Link</name>
    <assignmentItems><assignToReference>newLink.Widget__c</assignToReference><operator>Assign</operator><value><elementReference>Loop_Widgets.Id</elementReference></value></assignmentItems>
    <assignmentItems><assignToReference>newLink.Request__c</assignToReference><operator>Assign</operator><value><elementReference>Create_Request</elementReference></value></assignmentItems>
    <assignmentItems><assignToReference>links</assignToReference><operator>Add</operator><value><elementReference>newLink</elementReference></value></assignmentItems>
    <connector><targetReference>Loop_Widgets</targetReference></connector>
  </assignments>
  <recordCreates>
    <name>Create_Links</name>
    <inputReference>links</inputReference>
  </recordCreates>
  <variables><name>widgetCount</name><dataType>Number</dataType></variables>
  <variables><name>newLink</name><dataType>SObject</dataType><objectType>WidgetLink__c</objectType></variables>
  <variables><name>links</name><dataType>SObject</dataType><objectType>WidgetLink__c</objectType><isCollection>true</isCollection></variables>
  <start><connector><targetReference>Get_Widgets</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 3 || steps[0].Kind != "recordLookup" || steps[1].Kind != "assignment" || steps[2].Kind != "decision" {
		t.Fatalf("steps = %#v", steps)
	}
	decisionBranches := steps[2].Branches
	if len(decisionBranches) != 1 || len(decisionBranches[0].Steps) != 3 || decisionBranches[0].Steps[1].Kind != "loop" {
		t.Fatalf("decision branches = %#v", decisionBranches)
	}
	loop := decisionBranches[0].Steps[1].Loop
	if loop.CollectionReference != "Get_Widgets" || len(loop.Steps) != 3 || loop.Steps[2].Assignment.Operator != "Add" {
		t.Fatalf("loop = %#v", loop)
	}
	if create := decisionBranches[0].Steps[2].RecordCreate; create.InputReference != "links" || create.ObjectName != "WidgetLink__c" {
		t.Fatalf("bulk create = %#v", create)
	}
}

func TestLoadProjectFlowLoopAssignNextValueReference(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Loop_Alias.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>Widgets</name>
    <object>Widget__c</object>
    <connector><targetReference>Loop_Widgets</targetReference></connector>
    <getFirstRecordOnly>false</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <loops>
    <name>Loop_Widgets</name>
    <collectionReference>Widgets</collectionReference>
    <assignNextValueToReference>currentWidget</assignNextValueToReference>
    <nextValueConnector><targetReference>Build_Link</targetReference></nextValueConnector>
  </loops>
  <assignments>
    <name>Build_Link</name>
    <assignmentItems><assignToReference>newLink.Widget__c</assignToReference><operator>Assign</operator><value><elementReference>currentWidget.Id</elementReference></value></assignmentItems>
  </assignments>
  <variables><name>newLink</name><dataType>SObject</dataType><objectType>WidgetLink__c</objectType></variables>
  <start><connector><targetReference>Widgets</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	loop := idx.Flows[0].Rules[0].Steps[1].Loop
	if loop.CurrentItemReference != "currentWidget" || loop.Steps[0].Assignment.SourceField != "currentWidget.Id" {
		t.Fatalf("loop = %#v", loop)
	}
}

func TestLoadProjectFlowObjectAndCollectionRecordUpdates(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Close_Related.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>Primary_Link</name>
    <object>WidgetLink__c</object>
    <filters><field>Change__c</field><operator>EqualTo</operator><value><elementReference>$Record.Id</elementReference></value></filters>
    <connector><targetReference>Close_Primary_Widget</targetReference></connector>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <recordUpdates>
    <name>Close_Primary_Widget</name>
    <object>Widget__c</object>
    <filters><field>Id</field><operator>EqualTo</operator><value><elementReference>Primary_Link.Widget__c</elementReference></value></filters>
    <inputAssignments><field>Status__c</field><value><stringValue>Closed</stringValue></value></inputAssignments>
    <connector><targetReference>Related_Links</targetReference></connector>
  </recordUpdates>
  <recordLookups>
    <name>Related_Links</name>
    <object>WidgetLink__c</object>
    <filters><field>Group__c</field><operator>EqualTo</operator><value><elementReference>Primary_Link.Group__c</elementReference></value></filters>
    <connector><targetReference>Loop_Links</targetReference></connector>
    <getFirstRecordOnly>false</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <loops>
    <name>Loop_Links</name>
    <collectionReference>Related_Links</collectionReference>
    <nextValueConnector><targetReference>Build_Update</targetReference></nextValueConnector>
    <noMoreValuesConnector><targetReference>Update_Widgets</targetReference></noMoreValuesConnector>
  </loops>
  <assignments>
    <name>Build_Update</name>
    <assignmentItems><assignToReference>widgetUpdate.Id</assignToReference><operator>Assign</operator><value><elementReference>Loop_Links.Widget__c</elementReference></value></assignmentItems>
    <assignmentItems><assignToReference>widgetUpdate.Status__c</assignToReference><operator>Assign</operator><value><stringValue>Closed</stringValue></value></assignmentItems>
    <assignmentItems><assignToReference>widgetUpdates</assignToReference><operator>Add</operator><value><elementReference>widgetUpdate</elementReference></value></assignmentItems>
    <connector><targetReference>Loop_Links</targetReference></connector>
  </assignments>
  <recordUpdates>
    <name>Update_Widgets</name>
    <inputReference>widgetUpdates</inputReference>
  </recordUpdates>
  <variables><name>widgetUpdate</name><dataType>SObject</dataType><objectType>Widget__c</objectType></variables>
  <variables><name>widgetUpdates</name><dataType>SObject</dataType><objectType>Widget__c</objectType><isCollection>true</isCollection></variables>
  <start><connector><targetReference>Primary_Link</targetReference></connector><object>Change__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v", idx.Flows)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 5 || steps[1].Kind != "recordUpdate" || steps[3].Kind != "loop" || steps[4].Kind != "recordUpdate" {
		t.Fatalf("steps = %#v", steps)
	}
	if update := steps[1].RecordUpdate; update.ObjectName != "Widget__c" || len(update.Criteria) != 1 || len(update.InputAssignments) != 1 {
		t.Fatalf("object update = %#v", update)
	}
	loop := steps[3].Loop
	if len(loop.Steps) != 3 {
		t.Fatalf("loop = %#v", loop)
	}
	if update := steps[4].RecordUpdate; update.InputReference != "widgetUpdates" || update.ObjectName != "Widget__c" {
		t.Fatalf("bulk update = %#v", update)
	}
}

func TestLoadProjectFlowRecordUpdateTargetsLookupOutput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flows", "Widget_Update_Lookup.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <recordLookups>
    <name>Get_Task</name>
    <object>Task</object>
    <filters><field>WhatId</field><operator>EqualTo</operator><value><elementReference>$Record.Id</elementReference></value></filters>
    <connector><targetReference>Done</targetReference></connector>
    <getFirstRecordOnly>true</getFirstRecordOnly>
    <storeOutputAutomatically>true</storeOutputAutomatically>
  </recordLookups>
  <recordUpdates>
    <name>Done</name>
    <inputReference>Get_Task</inputReference>
    <inputAssignments><field>Status</field><value><stringValue>Completed</stringValue></value></inputAssignments>
  </recordUpdates>
  <start><connector><targetReference>Get_Task</targetReference></connector><object>Widget__c</object><triggerType>RecordAfterSave</triggerType></start>
</Flow>`)

	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}
	steps := idx.Flows[0].Rules[0].Steps
	if len(steps) != 2 || steps[1].Kind != "recordUpdate" {
		t.Fatalf("steps = %#v", steps)
	}
	if update := steps[1].RecordUpdate; update.ObjectName != "Task" || update.InputReference != "Get_Task" {
		t.Fatalf("record update = %#v", update)
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
