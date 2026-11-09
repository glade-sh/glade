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

// Records under one prefix in two objects are ambiguous; leave both alone.
func TestAdoptRecordKeyPrefixesSkipsAmbiguousPrefix(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a0E"}, Records: map[ID]Record{"a0F00000001AAAAAAA": {ID: "a0F00000001AAAAAAA"}}}
	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a0F"}, Records: map[ID]Record{"a0F00000001BBBBAAA": {ID: "a0F00000001BBBBAAA"}}}
	if AdoptRecordKeyPrefixes(&org) {
		t.Fatal("ambiguous prefix adopted")
	}
	if org.Objects["Alpha__c"].Definition.KeyPrefix != "a0E" || org.Objects["Beta__c"].Definition.KeyPrefix != "a0F" {
		t.Fatalf("prefixes changed: %q %q", org.Objects["Alpha__c"].Definition.KeyPrefix, org.Objects["Beta__c"].Definition.KeyPrefix)
	}
}
