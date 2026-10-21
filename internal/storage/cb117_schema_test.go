package storage

import "testing"

func TestCB117CustomObjectStandardDescribeFields(t *testing.T) {
	definition := ObjectDefinition{APIName: "Probe__c"}
	EnsureStandardObjectFields(&definition)

	if _, ok := definition.Fields["IsDeleted"]; !ok {
		t.Fatalf("custom object standard field IsDeleted missing from %#v", definition.Fields)
	}
	for _, fieldName := range []string{"LastViewedDate", "LastReferencedDate"} {
		if _, ok := definition.Fields[fieldName]; ok {
			t.Fatalf("un-tabbed custom object unexpectedly includes %s", fieldName)
		}
	}
}

func TestCB117StandardObjectSearchabilityRemainsEnabled(t *testing.T) {
	definition, ok := StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("Account standard definition missing")
	}
	if !definition.EnableSearch {
		t.Fatal("standard Account searchability defaulted false")
	}
}
