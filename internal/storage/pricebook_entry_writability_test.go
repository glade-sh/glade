package storage

import "testing"

func TestPricebookEntryPricebookCreateOnlyWritability(t *testing.T) {
	definition, ok := StandardObjectDefinition("PricebookEntry")
	if !ok {
		t.Fatal("missing standard PricebookEntry")
	}
	for _, name := range []string{"Pricebook2Id", "Product2Id"} {
		field := definition.Fields[name]
		if !FieldFlagValue(field.Createable, false) || FieldFlagValue(field.Updateable, true) {
			t.Fatalf("%s should permit creation only: %#v", name, field)
		}
		if StandardFieldAssignmentReadOnly("PricebookEntry", name) {
			t.Fatalf("create-only %s must allow transient Apex assignment", name)
		}
	}
	if !StandardFieldAssignmentReadOnly("PricebookEntry", "CreatedDate") {
		t.Fatal("creation support must preserve readonly system fields")
	}
}

func TestStandardFieldAssignmentEmptyIDScope(t *testing.T) {
	for _, tc := range []struct {
		object, field string
		want          bool
	}{
		{"PricebookEntry", "Pricebook2Id", true}, {"pricebookentry", "pricebook2id", true},
		{"PricebookEntry", "Product2Id", false}, {"PermissionSetAssignment", "AssigneeId", false},
		{"ContentVersion", "PathOnClient", false}, {"Owned__c", "Pricebook2Id", false},
	} {
		if got := StandardFieldAssignmentRequiresEmptyID(tc.object, tc.field); got != tc.want {
			t.Errorf("%s.%s requires empty ID=%v want=%v", tc.object, tc.field, got, tc.want)
		}
	}
}
