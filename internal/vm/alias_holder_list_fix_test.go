package vm

import "testing"

func TestAliasHolderListsPropagateNestedObjectMutations(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.SetTraceEnabled(false)
	for _, class := range []Class{
		{Name: "AliasHolderInterface", IsInterface: true},
		{
			Name: "AliasHolder", Interfaces: []string{"AliasHolderInterface"},
			Fields: map[string]Field{"record": {Name: "record", Type: "Object"}},
		},
		{
			Name:   "AliasHolderEnvelope",
			Fields: map[string]Field{"item": {Name: "item", Type: "AliasHolderInterface"}},
		},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	execute := func(source string) {
		t.Helper()
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatal(err)
		}
	}
	execute(`
Account original = new Account(Name = 'before');
Account second = original;
AliasHolder firstHolder = new AliasHolder();
firstHolder.record = original;
AliasHolder secondHolder = new AliasHolder();
secondHolder.record = original;
List<AliasHolder> holders = new List<AliasHolder>{firstHolder, secondHolder};
List<AliasHolderInterface> interfaces = new List<AliasHolderInterface>{firstHolder, secondHolder};
AliasHolderEnvelope firstEnvelope = new AliasHolderEnvelope();
firstEnvelope.item = firstHolder;
AliasHolderEnvelope secondEnvelope = new AliasHolderEnvelope();
secondEnvelope.item = secondHolder;
List<AliasHolderEnvelope> envelopes = new List<AliasHolderEnvelope>{firstEnvelope, secondEnvelope};
String expected;
`)
	for _, mutation := range []string{
		"expected = 'assigned'; second.Name = expected;",
		"expected = 'put'; second.put('Name', expected);",
	} {
		// Preserve Apex identities while detaching storage from the variable
		// being mutated, so shared Go maps cannot hide a missed replacement.
		for name, value := range machine.Globals {
			if name == "second" {
				continue
			}
			machine.Globals[name] = cloneValuePreserveRefs(value)
		}
		execute(mutation)
		globals := machine.Globals
		wantRef, wantName := globals["second"].Ref, globals["expected"]
		check := func(path string, record Value) {
			t.Helper()
			if record.Ref != wantRef || !record.Fields["Name"].Equal(wantName) {
				t.Errorf("%s: %s Ref=%d Name=%v, want Ref=%d Name=%v", mutation, path,
					record.Ref, record.Fields["Name"], wantRef, wantName)
			}
		}
		// Inspect every occurrence before any Apex read can refresh aliases.
		check("original", globals["original"])
		check("second", globals["second"])
		check("firstHolder.record", globals["firstHolder"].Fields["record"])
		check("secondHolder.record", globals["secondHolder"].Fields["record"])
		check("firstEnvelope.item.record", globals["firstEnvelope"].Fields["item"].Fields["record"])
		check("secondEnvelope.item.record", globals["secondEnvelope"].Fields["item"].Fields["record"])
		for _, name := range []string{"holders", "interfaces", "envelopes"} {
			items := globals[name].List
			if len(items) != 2 || items[0].Ref == 0 || items[0].Ref == items[1].Ref {
				t.Fatalf("%s must contain two distinct instances: %#v", name, items)
			}
			for i, item := range items {
				if name == "envelopes" {
					item = item.Fields["item"]
				}
				if record := item.Fields["record"]; record.Ref != wantRef || !record.Fields["Name"].Equal(wantName) {
					t.Errorf("%s: %s[%d].record Ref=%d Name=%v, want Ref=%d Name=%v", mutation, name, i,
						record.Ref, record.Fields["Name"], wantRef, wantName)
				}
			}
		}
		if t.Failed() {
			t.FailNow()
		}
		execute(`
System.assertEquals(expected, original.Name);
System.assertEquals(expected, second.Name);
System.assertEquals(expected, ((Account)firstHolder.record).Name);
System.assertEquals(expected, ((Account)secondHolder.record).Name);
System.assertEquals(expected, ((Account)((AliasHolder)firstEnvelope.item).record).Name);
System.assertEquals(expected, ((Account)((AliasHolder)secondEnvelope.item).record).Name);
for (AliasHolder holder : holders) {
    System.assertEquals(expected, ((Account)holder.record).Name);
}
for (AliasHolderInterface item : interfaces) {
    System.assertEquals(expected, ((Account)((AliasHolder)item).record).Name);
}
for (AliasHolderEnvelope envelope : envelopes) {
    System.assertEquals(expected, ((Account)((AliasHolder)envelope.item).record).Name);
}
`)
	}
}
