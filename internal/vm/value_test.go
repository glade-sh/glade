package vm

import "testing"

func TestValueStringGenericObjectIncludesApexTypeDelimiter(t *testing.T) {
	value := Object("TriggerHandler")

	if got := value.String(); got != "TriggerHandler:{}" {
		t.Fatalf("String() = %q", got)
	}
}

func TestApexIDTextEqualUsesFifteenCharacterCanonicalID(t *testing.T) {
	if !apexIDTextEqual("012000000000001", "012000000000001AAA") {
		t.Fatal("expected 15 and 18 character forms of the same Id to compare equal")
	}
	if apexIDTextEqual("012000000000001AAA", "012000000000002AAA") {
		t.Fatal("distinct generated RecordType Ids must not compare equal")
	}
	if apexIDTextEqual("012000000000001", "012000000000002") {
		t.Fatal("distinct 15 character Ids must not compare equal")
	}
}

func TestSObjectFieldStringUsesFieldName(t *testing.T) {
	value := sObjectFieldToken("Account", "CreatedById")

	if got := value.String(); got != "CreatedById" {
		t.Fatalf("String() = %q", got)
	}
}
