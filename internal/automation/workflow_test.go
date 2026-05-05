package automation

import (
	"os"
	"path/filepath"
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
</Workflow>`)
	idx, err := LoadProject(project.Project{WorkflowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Workflows) != 1 || idx.Workflows[0].ObjectName != "Account" {
		t.Fatalf("workflows = %#v", idx.Workflows)
	}
	rules := idx.Workflows[0].Rules
	if len(rules) != 1 || rules[0].Name != "MarkActive" || len(rules[0].Criteria) != 1 || len(rules[0].FieldUpdates) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "OAERAUTO001" {
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

func writeWorkflowTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
