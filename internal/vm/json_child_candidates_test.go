package vm

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestJSONChildRelationshipCandidateIsolation(t *testing.T) {
	org := storage.NewOrgState()
	org.RuntimeSchemaStamp = "candidate-generation"
	for i := 0; i < 16; i++ {
		parent, child := fmt.Sprintf("Parent%d__c", i), fmt.Sprintf("Child%d__c", i)
		org.Objects[parent] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: parent}}
		org.Objects[child] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: child, Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{parent}, ChildRelationshipName: "Children__r"},
		}}}
	}
	// Unrelated scalar fields must not be rescanned for every parent lookup.
	for i := 0; i < 16; i++ {
		name := fmt.Sprintf("Unrelated%d__c", i)
		fields := map[string]storage.Field{}
		for j := 0; j < 512; j++ {
			key := fmt.Sprintf("Value%d__c", j)
			fields[key] = storage.Field{APIName: key, Type: storage.FieldString}
		}
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: fields}}
	}
	machine := New(nil)
	machine.SetOrg(&org)
	start := time.Now()
	for i := 0; i < 16; i++ {
		parent := fmt.Sprintf("Parent%d__c", i)
		want := fmt.Sprintf("List<Child%d__c>", i)
		if got, ok := machine.jsonSObjectChildRelationshipType(parent, "Children__r"); !ok || got != want {
			t.Fatalf("%s: %s, %v; want %s", parent, got, ok, want)
		}
		if got, ok := machine.jsonSObjectChildRelationshipType(parent, "Missing__r"); ok || got != "" {
			t.Fatalf("negative lookup: %s, %v", got, ok)
		}
	}
	t.Logf("16 parents, 32 lookups: %.6fs", time.Since(start).Seconds())
	// Replacing the schema at the host boundary must detach positive and negative caches.
	next := org.Clone()
	child := next.Objects["Child0__c"]
	field := child.Definition.Fields["Parent__c"]
	field.ChildRelationshipName = "Missing__r"
	child.Definition.Fields["Parent__c"] = field
	next.Objects["Child0__c"] = child
	machine.SetOrg(&next)
	if got, ok := machine.jsonSObjectChildRelationshipType("Parent0__c", "Missing__r"); !ok || got != "List<Child0__c>" {
		t.Fatalf("replacement: %s, %v", got, ok)
	}
	if got, ok := machine.jsonSObjectChildRelationshipType("Parent0__c", "Children__r"); ok || got != "" {
		t.Fatalf("stale relationship: %s, %v", got, ok)
	}
}

func TestJSONChildRelationshipCandidatesKeepNamespaceAliases(t *testing.T) {
	org := storage.NewOrgState()
	org.RuntimeSchemaStamp = "candidate-generation"
	org.Namespace = "pkg"
	org.Objects["pkg__Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "pkg__Parent__c"}}
	org.Objects["pkg__Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "pkg__Child__c", Relations: []storage.Relationship{{ParentObjects: []string{"Parent__c", "Account"}, ChildRelationship: "pkg__Children__r"}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, name := range []string{"Children__r", "pkg__Children__r", "Children"} {
		if got, ok := machine.jsonSObjectChildRelationshipType("pkg__Parent__c", name); !ok || got != "List<pkg__Child__c>" {
			t.Fatalf("%s: %s, %v", name, got, ok)
		}
	}
}
