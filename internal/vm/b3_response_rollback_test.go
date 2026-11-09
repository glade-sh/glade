package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Unsupported local rule processing must not leave a partially applied DML.
func TestB3ResponseProcessingErrorRollsBackInsert(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry storage.AutoResponseRuleEntry
	}{
		{"template", storage.AutoResponseRuleEntry{Criteria: []storage.WorkflowCriteriaItem{{Field: "Lead.Company", Operation: "equals", Value: "Synthetic"}}}},
		{"criteria", storage.AutoResponseRuleEntry{Formula: "unsupported_expression()", Template: "Synthetic", Subject: "Synthetic"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Lead")
			lead := org.Objects["Lead"]
			lead.Definition.AutoResponseRules = []storage.AutoResponseRule{{Name: "Synthetic", Active: true, Entries: []storage.AutoResponseRuleEntry{tc.entry}}}
			org.Objects["Lead"] = lead
			before := len(lead.Records)
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous(`Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.insert(new Lead(LastName='Synthetic',Company='Synthetic',Email='synthetic@example.invalid'),o);`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err == nil {
				t.Fatal("expected unsupported local rule processing to fail")
			}
			if got := len(org.Objects["Lead"].Records); got != before {
				t.Errorf("failed response processing left %d inserted Leads", got-before)
			}
		})
	}
}

func TestB3ResponseProcessingErrorRollsBackEarlierActivity(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Lead")
	storage.EnsureStandardObject(&org, "Task")
	lead := org.Objects["Lead"]
	lead.Definition.AutoResponseRules = []storage.AutoResponseRule{{Name: "Synthetic", Active: true, Entries: []storage.AutoResponseRuleEntry{
		{Template: "Synthetic", Subject: "Synthetic", Criteria: []storage.WorkflowCriteriaItem{{Field: "Lead.Company", Operation: "equals", Value: "Synthetic"}}},
		{Criteria: []storage.WorkflowCriteriaItem{{Field: "Lead.Company", Operation: "equals", Value: "Other"}}},
	}}}
	org.Objects["Lead"] = lead
	beforeLeads, beforeTasks := len(lead.Records), len(org.Objects["Task"].Records)
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.insert(new List<Lead>{new Lead(LastName='First',Company='Synthetic',Email='first@example.invalid'),new Lead(LastName='Second',Company='Other',Email='second@example.invalid')},o);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err == nil {
		t.Fatal("expected the second response template to be unsupported")
	}
	if got := len(org.Objects["Lead"].Records); got != beforeLeads {
		t.Errorf("response failure left %d inserted Leads", got-beforeLeads)
	}
	if got := len(org.Objects["Task"].Records); got != beforeTasks {
		t.Errorf("response failure left %d earlier activities", got-beforeTasks)
	}
}
