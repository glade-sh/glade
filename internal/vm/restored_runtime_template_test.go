package vm

import (
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRestoredRuntimeTemplateZeroValueIsInvalid(t *testing.T) {
	var template RestoredRuntimeTemplate
	if template.Valid() {
		t.Fatal("zero RestoredRuntimeTemplate is valid")
	}
	if machine := template.CloneMachine(nil); machine != nil {
		t.Fatalf("zero RestoredRuntimeTemplate CloneMachine() = %p, want nil", machine)
	}
}

func TestRestoredRuntimeTemplateClonesOwnedRuntime(t *testing.T) {
	org := restoredRuntimeTestOrg()
	machine := restoredRuntimeTestMachine(t)
	template := NewRestoredRuntimeTemplate(org, machine)
	if !template.Valid() {
		t.Fatal("constructed RestoredRuntimeTemplate is invalid")
	}

	firstOrg := template.CloneOrg()
	firstObject := firstOrg.Objects["CacheBoundary__c"]
	firstRecord := firstObject.Records["a00000000000001"]
	firstRecord.Fields["Name"] = storage.StringValue("changed")
	firstRecord.Fields["Tags"].List[0] = storage.StringValue("changed")
	firstObject.Records[firstRecord.ID] = firstRecord
	firstObject.Indexes["Name"].Entries["base"][0] = "a00000000000002"
	firstObject.Indexes["Name"].Entries["changed"] = []storage.ID{firstRecord.ID}
	firstOrg.Objects["CacheBoundary__c"] = firstObject
	firstOrg.IDSequences["CacheBoundary__c"] = 99
	if _, ok := storage.EnsureMutableObjectDefinition(&firstOrg, "CacheBoundary__c"); !ok {
		t.Fatal("EnsureMutableObjectDefinition() did not find CacheBoundary__c")
	}
	firstObject = firstOrg.Objects["CacheBoundary__c"]
	firstObject.Definition.Fields["RuntimeOnly__c"] = storage.Field{APIName: "RuntimeOnly__c", Type: storage.FieldString}
	firstOrg.Objects["CacheBoundary__c"] = firstObject

	secondOrg := template.CloneOrg()
	if got := secondOrg.Objects["CacheBoundary__c"].Records[firstRecord.ID].Fields["Name"].String; got != "base" {
		t.Fatalf("second CloneOrg() Name = %q, want base", got)
	}
	if got := secondOrg.Objects["CacheBoundary__c"].Records[firstRecord.ID].Fields["Tags"].List[0].String; got != "base" {
		t.Fatalf("second CloneOrg() nested list value = %q, want base", got)
	}
	if got := secondOrg.Objects["CacheBoundary__c"].Indexes["Name"].Entries["base"][0]; got != firstRecord.ID {
		t.Fatalf("second CloneOrg() existing index entry = %q, want %q", got, firstRecord.ID)
	}
	if _, ok := secondOrg.Objects["CacheBoundary__c"].Indexes["Name"].Entries["changed"]; ok {
		t.Fatal("second CloneOrg() retained first clone index entry")
	}
	if got := secondOrg.IDSequences["CacheBoundary__c"]; got != 1 {
		t.Fatalf("second CloneOrg() sequence = %d, want 1", got)
	}
	if _, ok := secondOrg.Objects["CacheBoundary__c"].Definition.Fields["RuntimeOnly__c"]; ok {
		t.Fatal("second CloneOrg() retained first clone definition mutation")
	}

	firstMachine := template.CloneMachine(nil)
	class := firstMachine.Classes["BoundaryCounter"]
	field := class.StaticFields["value"]
	field.Value = Int(41)
	class.StaticFields["value"] = field
	values := class.StaticFields["values"]
	values.Value.List[0] = String("changed")
	class.StaticFields["values"] = values
	firstMachine.Classes["BoundaryCounter"] = class
	secondMachine := template.CloneMachine(nil)
	if got := secondMachine.Classes["BoundaryCounter"].StaticFields["value"].Value; got.Kind != ValueNull {
		t.Fatalf("second CloneMachine() static value = %#v, want null", got)
	}
	if got := secondMachine.Classes["BoundaryCounter"].StaticFields["values"].Value.List[0].Text; got != "base" {
		t.Fatalf("second CloneMachine() nested static value = %q, want base", got)
	}
}

func TestRestoredRuntimeTemplateConcurrentClonesAreIndependent(t *testing.T) {
	template := NewRestoredRuntimeTemplate(restoredRuntimeTestOrg(), restoredRuntimeTestMachine(t))
	const cloneCount = 16
	orgs := make([]storage.OrgState, cloneCount)
	machines := make([]*VM, cloneCount)

	var wg sync.WaitGroup
	for i := 0; i < cloneCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			org := template.CloneOrg()
			record := org.Objects["CacheBoundary__c"].Records["a00000000000001"]
			record.Fields["Name"] = storage.StringValue(string(rune('a' + i)))
			org.Objects["CacheBoundary__c"].Records[record.ID] = record
			orgs[i] = org

			machine := template.CloneMachine(nil)
			class := machine.Classes["BoundaryCounter"]
			field := class.StaticFields["value"]
			field.Value = Int(int64(i))
			class.StaticFields["value"] = field
			machine.Classes["BoundaryCounter"] = class
			machines[i] = machine
		}(i)
	}
	wg.Wait()

	for i := 0; i < cloneCount; i++ {
		wantName := string(rune('a' + i))
		if got := orgs[i].Objects["CacheBoundary__c"].Records["a00000000000001"].Fields["Name"].String; got != wantName {
			t.Fatalf("org clone %d Name = %q, want %q", i, got, wantName)
		}
		if got := machines[i].Classes["BoundaryCounter"].StaticFields["value"].Value; got.Kind != ValueInt || got.Int != int64(i) {
			t.Fatalf("machine clone %d static value = %#v, want %d", i, got, i)
		}
	}
	freshOrg := template.CloneOrg()
	if got := freshOrg.Objects["CacheBoundary__c"].Records["a00000000000001"].Fields["Name"].String; got != "base" {
		t.Fatalf("fresh org after concurrent clones Name = %q, want base", got)
	}
	freshMachine := template.CloneMachine(nil)
	if got := freshMachine.Classes["BoundaryCounter"].StaticFields["value"].Value; got.Kind != ValueNull {
		t.Fatalf("fresh machine after concurrent clones static value = %#v, want null", got)
	}
}

func restoredRuntimeTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	id := storage.ID("a00000000000001")
	org.Objects["CacheBoundary__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CacheBoundary__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			id: {
				ID:     id,
				Object: "CacheBoundary__c",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("base"),
					"Tags": storage.ListValue(storage.StringValue("base")),
				},
			},
		},
		Indexes: map[string]storage.IndexSet{
			"Name": {
				Definition: storage.IndexDefinition{Name: "Name", Object: "CacheBoundary__c", Fields: []string{"Name"}},
				Entries:    map[string][]storage.ID{"base": {id}},
			},
		},
	}
	org.IDSequences["CacheBoundary__c"] = 1
	return org
}

func restoredRuntimeTestMachine(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "BoundaryCounter",
		StaticFields: map[string]Field{
			"value":  {Name: "value", Type: "Integer", Static: true},
			"values": {Name: "values", Type: "List<String>", Static: true, Value: List(String("base"))},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}
