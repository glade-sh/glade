package vm

import (
	"strconv"
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
	class, ok := firstMachine.lookupClass("BoundaryCounter")
	if !ok {
		t.Fatal("first machine lost BoundaryCounter")
	}
	firstMachine.writeStaticFieldValue("BoundaryCounter", "value", class, class.StaticFields["value"], Int(41))
	firstMachine.writeStaticFieldValue("BoundaryCounter", "values", class, class.StaticFields["values"], List(String("changed")))
	secondMachine := template.CloneMachine(nil)
	secondClass, ok := secondMachine.classEntry("BoundaryCounter")
	if !ok {
		t.Fatal("second machine lost BoundaryCounter")
	}
	if got := secondClass.StaticFields["value"].Value; got.Kind != ValueNull {
		t.Fatalf("second CloneMachine() static value = %#v, want null", got)
	}
	if got := secondClass.StaticFields["values"].Value.List[0].Text; got != "base" {
		t.Fatalf("second CloneMachine() nested static value = %q, want base", got)
	}
}

func TestRestoredRuntimeTemplateConcurrentClonesAreIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}

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
			class, ok := machine.lookupClass("BoundaryCounter")
			if !ok {
				t.Errorf("machine %d lost BoundaryCounter", i)
				return
			}
			machine.writeStaticFieldValue("BoundaryCounter", "value", class, class.StaticFields["value"], Int(int64(i)))
			machines[i] = machine
		}(i)
	}
	wg.Wait()

	for i := 0; i < cloneCount; i++ {
		wantName := string(rune('a' + i))
		if got := orgs[i].Objects["CacheBoundary__c"].Records["a00000000000001"].Fields["Name"].String; got != wantName {
			t.Fatalf("org clone %d Name = %q, want %q", i, got, wantName)
		}
		class, ok := machines[i].classEntry("BoundaryCounter")
		if !ok {
			t.Fatalf("machine clone %d lost BoundaryCounter", i)
		}
		if got := class.StaticFields["value"].Value; got.Kind != ValueInt || got.Int != int64(i) {
			t.Fatalf("machine clone %d static value = %#v, want %d", i, got, i)
		}
	}
	freshOrg := template.CloneOrg()
	if got := freshOrg.Objects["CacheBoundary__c"].Records["a00000000000001"].Fields["Name"].String; got != "base" {
		t.Fatalf("fresh org after concurrent clones Name = %q, want base", got)
	}
	freshMachine := template.CloneMachine(nil)
	freshClass, ok := freshMachine.classEntry("BoundaryCounter")
	if !ok {
		t.Fatal("fresh machine lost BoundaryCounter")
	}
	if got := freshClass.StaticFields["value"].Value; got.Kind != ValueNull {
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

func TestRestoredRuntimeTemplateSharesFrozenMachineWithCopyOnWrite(t *testing.T) {
	base := restoredRuntimeTemplatePerfMachine(t)
	template := NewRestoredRuntimeTemplate(storage.NewOrgState(), base)
	_ = template.CloneMachine(nil)
	deepClones := testing.AllocsPerRun(3, func() { _ = base.CloneRuntime(nil) })
	rowClones := testing.AllocsPerRun(3, func() { _ = template.CloneMachine(nil) })
	t.Logf("restored VM clone allocations %.0f; deep runtime clone allocations %.0f", rowClones, deepClones)
	if rowClones >= deepClones {
		t.Fatalf("restored VM clone allocates %.0f, want fewer than deep runtime clone %.0f", rowClones, deepClones)
	}

	first, second := template.CloneMachine(nil), template.CloneMachine(nil)
	if err := first.RegisterClass(Class{Name: "RequestOnly"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := base.Classes["RequestOnly"]; ok {
		t.Fatal("request class registration changed the frozen template")
	}
	if _, ok := second.Classes["RequestOnly"]; ok {
		t.Fatal("request class registration leaked into a sibling VM")
	}
}

func BenchmarkRestoredRuntimeTemplateCloneMachine(b *testing.B) {
	base := restoredRuntimeTemplatePerfMachine(b)
	template := NewRestoredRuntimeTemplate(storage.NewOrgState(), base)
	_ = template.CloneMachine(nil)
	b.Run("deep", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = base.CloneRuntime(nil)
		}
	})
	b.Run("restored-shared", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = template.CloneMachine(nil)
		}
	})
}

func restoredRuntimeTemplatePerfMachine(tb testing.TB) *VM {
	tb.Helper()
	machine := New(nil)
	for i := 0; i < 64; i++ {
		name := "FrozenRuntime" + strconv.Itoa(i)
		fields := make(map[string]Field, 8)
		staticFields := make(map[string]Field, 8)
		for j := 0; j < 8; j++ {
			fieldName := "field" + strconv.Itoa(j)
			fields[fieldName] = Field{Name: fieldName, Type: "Integer", Value: Int(int64(j))}
			staticFields[fieldName] = Field{Name: fieldName, Type: "Integer", Static: true, Value: Int(int64(j))}
		}
		if err := machine.RegisterClass(Class{Name: name, Fields: fields, StaticFields: staticFields}); err != nil {
			tb.Fatal(err)
		}
	}
	return machine
}
