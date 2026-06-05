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

func TestIDsEqualKeepsFifteenCharacterCaseSignificant(t *testing.T) {
	if IDsEqual("aDa000000000001", "aDA000000000001") {
		t.Fatal("15 character ids that differ by case must not compare equal")
	}
	if !IDsEqual("aDa000000000001", "aDa000000000001AAA") {
		t.Fatal("15 and 18 character forms with the same first 15 chars should compare equal")
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

func TestEnsureUniqueKeyPrefixesReassignsDuplicateCustomPrefixes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a00"}}
	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a00"}}
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001"}}

	EnsureUniqueKeyPrefixes(&org)

	alpha := org.Objects["Alpha__c"].Definition.KeyPrefix
	beta := org.Objects["Beta__c"].Definition.KeyPrefix
	if alpha == "" || beta == "" || alpha == beta {
		t.Fatalf("custom prefixes alpha=%q beta=%q, want unique", alpha, beta)
	}
	if got := org.Objects["Account"].Definition.KeyPrefix; got != "001" {
		t.Fatalf("Account prefix = %q", got)
	}
}

func TestCustomPrefixDoesNotCycleAfterLeadingARange(t *testing.T) {
	const fullFirstCycle = 62*62 + 61*62*62
	seen := make(map[string]struct{}, fullFirstCycle)
	for i := 0; i < fullFirstCycle; i++ {
		prefix := customPrefix(i)
		if len(prefix) != 3 {
			t.Fatalf("customPrefix(%d) length = %d", i, len(prefix))
		}
		if _, ok := seen[prefix]; ok {
			t.Fatalf("customPrefix(%d) repeated %q", i, prefix)
		}
		seen[prefix] = struct{}{}
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
