package vm

import "testing"

func TestFrameworkFieldsMapDoesNotDropExplicitNamespace(t *testing.T) {
	field := sObjectFieldToken("Account", "Name")
	foreign := sObjectFieldToken("Contact", "Name")
	values := Value{Kind: ValueMap, Type: "Map<String,Schema.SObjectField>", Map: map[string]Value{
		mapKey(String("name__c")):             foreign,
		mapKey(String("fflib_test__name__c")): field,
	}}
	machine := New(nil)
	qualified := machine.frameworkNamespacedAttributeMapGet(values, "fflib_test__name__c", false, "someOtherNamespace")
	if !qualified.Equal(field) {
		t.Fatalf("qualified lookup = %v, want %v", qualified, field)
	}
	foreignQualified := machine.frameworkNamespacedAttributeMapGet(values, "other_test__name__c", false, "someOtherNamespace")
	if foreignQualified.Kind != ValueNull {
		t.Fatalf("foreign qualified lookup = %v, want null", foreignQualified)
	}
	implicit := machine.frameworkNamespacedAttributeMapGet(values, "name__c", true, "fflib_test")
	if !implicit.Equal(field) {
		t.Fatalf("unqualified lookup = %v, want %v", implicit, field)
	}
}
