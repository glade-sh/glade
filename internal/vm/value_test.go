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
	if apexIDTextEqual("aDa000000000001", "aDA000000000001") {
		t.Fatal("expected 15 character Id comparison to remain case-sensitive")
	}
	if apexIDTextEqual("012000000000001AAA", "012000000000002AAA") {
		t.Fatal("distinct generated RecordType Ids must not compare equal")
	}
	if apexIDTextEqual("012000000000001", "012000000000002") {
		t.Fatal("distinct 15 character Ids must not compare equal")
	}
}

func TestLooksLikeComparableIDTextRequiresKnownPrefix(t *testing.T) {
	if !looksLikeComparableIDText("001000000000001") {
		t.Fatal("expected standard Account prefix to be treated as comparable Id text")
	}
	if looksLikeComparableIDText("areaOfSpecialty") {
		t.Fatal("15 character field keys must not be treated as Id text")
	}
	if !shouldCompareTextAsID("aMM000000000002", "aMM000000000002GAA") {
		t.Fatal("custom 15 and 18 character Id forms should compare as Id text")
	}
	if shouldCompareTextAsID("areaOfSpecialty", "AreaOfSpecialty") {
		t.Fatal("case-different 15 character field keys must not compare as Id text")
	}
}

func TestCanonicalIDMapKeyKeepsCaseSignificantPrefix(t *testing.T) {
	left := platformScalar("Id", "aDa000000000001AAA")
	right := platformScalar("Id", "aDA000000000001AAA")

	if mapKey(left) == mapKey(right) {
		t.Fatal("Id map keys must not collapse case-distinct 15 character prefixes")
	}
}

func TestSObjectValueEqualityTreatsUnqualifiedAndNamespacedCustomObjectAsEquivalent(t *testing.T) {
	local := Object("Schedule__c")
	setExplicitSObjectField(&local, "Id", String("a010000000000001"))
	local.Fields["Id"] = String("a010000000000001")

	namespaced := Object("NU__Schedule__c")
	setExplicitSObjectField(&namespaced, "Id", String("a010000000000001"))
	namespaced.Fields["Id"] = String("a010000000000001")

	if !local.Equal(namespaced) {
		t.Fatal("expected unqualified and namespaced custom SObject values to compare equal")
	}
}

func TestSObjectValueEqualityKeepsDistinctNamespaceQualifiedObjectsSeparate(t *testing.T) {
	left := Object("pkgA__Schedule__c")
	right := Object("pkgB__Schedule__c")

	if left.Equal(right) {
		t.Fatal("expected two namespace-qualified custom SObject values to remain distinct")
	}
}

func TestSObjectFieldStringUsesFieldName(t *testing.T) {
	value := sObjectFieldToken("Account", "CreatedById")

	if got := value.String(); got != "CreatedById" {
		t.Fatalf("String() = %q", got)
	}
}
