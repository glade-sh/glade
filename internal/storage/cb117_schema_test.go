package storage

import "testing"

func TestCB117CustomObjectStandardDescribeFields(t *testing.T) {
	definition := ObjectDefinition{APIName: "Probe__c"}
	EnsureStandardObjectFields(&definition)

	for _, fieldName := range []string{"IsDeleted", "LastViewedDate", "LastReferencedDate"} {
		if _, ok := definition.Fields[fieldName]; !ok {
			t.Fatalf("custom object standard field %s missing from %#v", fieldName, definition.Fields)
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
