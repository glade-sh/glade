package vm

import "testing"

func TestValueStringGenericObjectIncludesApexTypeDelimiter(t *testing.T) {
	value := Object("TriggerHandler")

	if got := value.String(); got != "TriggerHandler:{}" {
		t.Fatalf("String() = %q", got)
	}
}

func TestValueStringGenericObjectIncludesFields(t *testing.T) {
	value := Object("Query")
	value.Fields["condition"] = String("Pro forma")
	value.Fields["limit"] = Int(10)

	if got := value.String(); got != "Query:{condition=Pro forma, limit=10}" {
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

func TestSObjectValueEqualityTreatsSameReferenceAsSameObject(t *testing.T) {
	left := Object("Schedule__c")
	left.Fields["Name"] = String("Before")

	right := Object("Schedule__c")
	right.Ref = left.Ref
	right.Fields["Name"] = String("After")

	if !left.Equal(right) {
		t.Fatal("expected two snapshots of the same SObject reference to compare equal")
	}
}

func TestSObjectValueEqualityTreatsStringAndPlatformIDFieldsAsEqual(t *testing.T) {
	left := Object("Account")
	left.Fields["Id"] = String("001000000000001")
	right := Object("Account")
	right.Fields["Id"] = platformScalar("Id", "001000000000001")

	if !left.Equal(right) {
		t.Fatal("expected string and platform Id fields to compare equal")
	}
}

func TestSObjectValueEqualityTreatsTypedRelationshipNullAndMissingProjectionAsEqual(t *testing.T) {
	left := Object("Product__c")
	relationshipNull := Null
	relationshipNull.Type = "DeferredRevenueMethod__c"
	relationshipNull.Runtime = relationshipNullRuntime
	left.Fields["DeferredRevenueMethod__r"] = relationshipNull

	right := Object("Product__c")
	relationship := Object("deferredrevenuemethod__r")
	relationship.Fields["name"] = Null
	relationship.Fields["recognition__c"] = Null
	right.Fields["deferredrevenuemethod__r"] = relationship

	if !left.Equal(right) {
		t.Fatal("expected typed relationship null and all-null relationship projection to compare equal")
	}
}

func TestValueEqualPageReferenceUsesURL(t *testing.T) {
	left := newPageReference("/Login?startUrl=/apex/MyLoginInformation")
	right := newPageReference("/Login?startUrl=/apex/MyLoginInformation")
	if !left.Equal(right) {
		t.Fatal("matching PageReference URLs should compare equal")
	}
	other := newPageReference("/")
	if left.Equal(other) {
		t.Fatal("different PageReference URLs should not compare equal")
	}
}

func TestSOQLLiteralSObjectWithExplicitNullIDUsesNull(t *testing.T) {
	value := Object("Order__c")
	value.Fields["Id"] = Null
	setExplicitSObjectField(&value, "Id", Null)

	if got := soqlLiteral(value); got != "null" {
		t.Fatalf("soqlLiteral() = %q, want null", got)
	}
}

func TestSetExplicitSObjectFieldClearsNamespaceEquivalentAlias(t *testing.T) {
	value := Object("pkg__Product__c")
	setExplicitSObjectField(&value, "pkg__Weight__c", Decimal(501))

	setExplicitSObjectField(&value, "Weight__c", Null)

	if _, got, ok := objectFieldValue(value, "pkg__Weight__c"); !ok || got.Kind != ValueNull {
		t.Fatalf("namespaced field = %#v ok=%v, want explicit null", got, ok)
	}
}

func TestSObjectFieldStringUsesFieldName(t *testing.T) {
	value := sObjectFieldToken("Account", "CreatedById")

	if got := value.String(); got != "CreatedById" {
		t.Fatalf("String() = %q", got)
	}
}
