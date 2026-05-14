package storage

import "testing"

func TestIDGeneratorUsesDeterministicObjectSequences(t *testing.T) {
	g := NewIDGenerator(map[string]string{
		"Account":  "001",
		"Thing__c": "a00",
	})

	firstAccount, err := g.Next("Account")
	if err != nil {
		t.Fatal(err)
	}
	secondAccount, err := g.Next("Account")
	if err != nil {
		t.Fatal(err)
	}
	firstThing, err := g.Next("Thing__c")
	if err != nil {
		t.Fatal(err)
	}

	if firstAccount != "001000000000001" {
		t.Fatalf("first account id = %s", firstAccount)
	}
	if secondAccount != "001000000000002" {
		t.Fatalf("second account id = %s", secondAccount)
	}
	if firstThing != "a00000000000001" {
		t.Fatalf("first thing id = %s", firstThing)
	}
}

func TestRuntimeIDGeneratorKeepsLogicalSequencesButOffsetsIDBody(t *testing.T) {
	g := NewRuntimeIDGenerator(map[string]string{"Account": "001"})

	id, err := g.Next("Account")
	if err != nil {
		t.Fatal(err)
	}

	if id == "001000000000001" {
		t.Fatalf("runtime id collided with low fake-id sequence: %s", id)
	}
	if g.Sequences["Account"] != 1 {
		t.Fatalf("logical sequence = %d, want 1", g.Sequences["Account"])
	}
}

func TestAssignDeterministicPrefixesKeepsStandardAndExplicitPrefixes(t *testing.T) {
	prefixes := AssignDeterministicPrefixes(
		[]string{"Widget__c", "Account", "Alpha__c"},
		map[string]string{"Widget__c": "a99"},
	)

	if prefixes["Account"] != "001" {
		t.Fatalf("Account prefix = %q", prefixes["Account"])
	}
	if prefixes["Widget__c"] != "a99" {
		t.Fatalf("Widget__c prefix = %q", prefixes["Widget__c"])
	}
	if prefixes["Alpha__c"] != "a00" {
		t.Fatalf("Alpha__c prefix = %q", prefixes["Alpha__c"])
	}
}

func TestValidateIDAccepts15And18CharacterBase62IDs(t *testing.T) {
	for _, id := range []ID{"001000000000001", "001000000000001AAA"} {
		if err := ValidateID(id); err != nil {
			t.Fatalf("ValidateID(%q): %v", id, err)
		}
	}
	if err := ValidateID("00100000000000!"); err == nil {
		t.Fatal("ValidateID accepted non-base62 id")
	}
}
