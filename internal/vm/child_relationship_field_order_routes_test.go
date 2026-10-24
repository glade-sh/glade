package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestChildRelationshipFieldOrderColdWarmAndRestoredRoutes(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c"}}
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Child__c", Relations: []storage.Relationship{
		{Field: "Z__c", ParentObjects: []string{"Parent__c"}, ChildRelationship: "Alpha__r"},
		{Field: "A__c", ParentObjects: []string{"Parent__c"}, ChildRelationship: "Zulu__r"},
	}}}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, route := range []string{"cold", "warm", "restored"} {
		t.Run(route, func(t *testing.T) {
			target := machine
			if route == "restored" {
				target = machine.CloneRuntime(nil)
				target.SetRuntimeTemplateOrg(&org)
			}
			values := target.describeChildRelationships("Parent__c")
			if len(values) != 2 {
				t.Fatalf("relationships=%v", values)
			}
			if got := values[0].Fields["relationshipName"].Text; got != "Zulu__r" {
				t.Fatalf("first relationship=%s; want field A before Z", got)
			}
		})
	}
}
