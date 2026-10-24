package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRuntimeTemplateClonesReuseSummaryCandidates(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Total__c": {
				APIName: "Total__c", Type: storage.FieldSummary, SummaryOperation: "sum",
				SummarizedField: "Child__c.Amount__c", SummaryForeignKey: "Child__c.Parent__c",
			},
		},
	}}
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
		},
	}}
	storage.EnsureUniqueKeyPrefixes(&org)
	org.RuntimeSchemaStamp = "frozen-generation"
	template := storage.NewRuntimeTemplate(org)
	template.RuntimeSchemaStamp = org.RuntimeSchemaStamp
	before := summaryCandidateBuilds.Load()
	for i := 0; i < 2; i++ {
		clone := template.CloneRuntimeOrg()
		engine := NewEngine(&clone)
		if got := engine.summaryRelationsForChild("Child__c"); len(got) != 1 {
			t.Fatalf("summary relations for clone %d = %#v, want one", i, got)
		}
	}
	if got := summaryCandidateBuilds.Load() - before; got != 1 {
		t.Fatalf("frozen org clone summary candidate builds = %d, want 1", got)
	}
}
