package storage

import "testing"

// Imported records keep their source-org IDs. When the local schema gave a
// custom object's real prefix to another object, the import must move the
// prefixes so each ID resolves to the object that holds it.
func TestApplyFixtureAdoptsImportedRecordKeyPrefixes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Gadget__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Gadget__c", KeyPrefix: "a0B", Fields: map[string]Field{}}, Records: map[ID]Record{}}
	org.Objects["Widget__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Widget__c", KeyPrefix: "a0C", Fields: map[string]Field{}}, Records: map[ID]Record{}}
	org.Objects["Spare__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Spare__c", KeyPrefix: "a0D", Fields: map[string]Field{}}, Records: map[ID]Record{}}
	fixture := Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Widget__c", Records: []FixtureRecord{
				{ID: "a0B00000001AAAAAAA", Fields: map[string]Value{"Name": StringValue("W1")}},
				{ID: "a0B00000001BBBBAAA", Fields: map[string]Value{"Name": StringValue("W2")}},
			}},
			{Name: "Gadget__c", Records: []FixtureRecord{
				{ID: "a0D00000001AAAAAAA", Fields: map[string]Value{"Name": StringValue("G1")}},
			}},
		},
	}
	if err := ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["Widget__c"].Definition.KeyPrefix; got != "a0B" {
		t.Fatalf("Widget__c prefix = %q, want a0B from its record IDs", got)
	}
	if got := org.Objects["Gadget__c"].Definition.KeyPrefix; got != "a0D" {
		t.Fatalf("Gadget__c prefix = %q, want a0D from its record IDs", got)
	}
	spare := org.Objects["Spare__c"].Definition.KeyPrefix
	if spare == "a0B" || spare == "a0D" || len(spare) != 3 {
		t.Fatalf("Spare__c prefix = %q, want a fresh prefix", spare)
	}
	seen := map[string]string{}
	for name, object := range org.Objects {
		prefix := object.Definition.KeyPrefix
		if prefix == "" {
			continue
		}
		if other, ok := seen[prefix]; ok {
			t.Fatalf("prefix %s shared by %s and %s", prefix, other, name)
		}
		seen[prefix] = name
	}
}

// Records under one prefix in two objects are ambiguous: no object adopts it,
// and an object whose records match its own prefix keeps it.
func TestApplyFixtureLeavesAmbiguousRecordKeyPrefixAlone(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a0E", Fields: map[string]Field{}}, Records: map[ID]Record{}}
	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a0F", Fields: map[string]Field{}}, Records: map[ID]Record{}}
	org.Objects["Gamma__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Gamma__c", KeyPrefix: "a0G", Fields: map[string]Field{}}, Records: map[ID]Record{}}
	fixture := Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Alpha__c", Records: []FixtureRecord{{ID: "a0H00000001AAAAAAA", Fields: map[string]Value{"Name": StringValue("A1")}}}},
			{Name: "Beta__c", Records: []FixtureRecord{{ID: "a0H00000001BBBBAAA", Fields: map[string]Value{"Name": StringValue("B1")}}}},
			{Name: "Gamma__c", Records: []FixtureRecord{{ID: "a0G00000001AAAAAAA", Fields: map[string]Value{"Name": StringValue("C1")}}}},
		},
	}
	if err := ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"Alpha__c": "a0E", "Beta__c": "a0F", "Gamma__c": "a0G"} {
		if got := org.Objects[name].Definition.KeyPrefix; got != want {
			t.Fatalf("%s prefix = %q, want %q", name, got, want)
		}
	}
}
