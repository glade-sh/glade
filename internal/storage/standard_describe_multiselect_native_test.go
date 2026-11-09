package storage

import "testing"

// The Roles policy is bound to standard_industries_schema/org.tsv:29,41.
// Other describe fields retain their established conversion until captured.
func TestNativeDescribeMultiselectStorageType(t *testing.T) {
	gate := standardFieldPolicyGate{OverrideStubType: true, Field: standardDescribeField{Name: "Roles", Type: "multipicklist"}}
	if got := standardPolicyGateDescribeField(gate).Type; got != FieldMultiPicklist {
		t.Fatalf("captured policy type=%s, want %s", got, FieldMultiPicklist)
	}
	ordinary := standardFieldPolicyGate{Field: standardDescribeField{Name: "SyntheticChoice", Type: "multipicklist"}}
	if got := standardPolicyGateDescribeField(ordinary).Type; got != FieldPicklist {
		t.Fatalf("ordinary catalog conversion changed: %s", got)
	}
}

func TestIndustriesSchemaRepairsPersistedStubType(t *testing.T) {
	features := []string{"ContactsToMultipleAccounts"}
	def := ObjectDefinition{APIName: "AccountContactRelation", Fields: map[string]Field{"Roles": {APIName: "Roles", Type: FieldString, DisplayType: "STRING"}}, Metadata: map[string]string{standardFieldsOverlayMarker: canonicalFeatureSignature(features), standardConfiguredPolicyFields: "", standardConfiguredPolicyFieldsLabelProvenance: "true"}}
	EnsureStandardObjectFieldsForFeatures(&def, features)
	if field := def.Fields["Roles"]; field.Type != FieldMultiPicklist || field.DisplayType != "MULTIPICKLIST" {
		t.Fatalf("persisted generated Roles not repaired: %#v", field)
	}
	configured := ObjectDefinition{APIName: "AccountContactRelation", Fields: map[string]Field{"Roles": {APIName: "Roles", Type: FieldString, DisplayType: "STRING"}}, Metadata: map[string]string{standardFieldsOverlayMarker: canonicalFeatureSignature(features), standardConfiguredPolicyFields: "Roles", standardConfiguredPolicyFieldsLabelProvenance: "true"}}
	EnsureStandardObjectFieldsForFeatures(&configured, features)
	if field := configured.Fields["Roles"]; field.Type != FieldString || field.DisplayType != "STRING" {
		t.Fatalf("configured Roles changed: %#v", field)
	}
}

func TestIndustriesSchemaConfiguredFieldRetainsOwnership(t *testing.T) {
	features := []string{"ContactsToMultipleAccounts"}
	configured := ObjectDefinition{APIName: "AccountContactRelation", Fields: map[string]Field{"Roles": {APIName: "Roles", Type: FieldString, DisplayType: "STRING"}}}
	EnsureStandardObjectFieldsForFeatures(&configured, features)
	EnsureStandardObjectFieldsForFeatures(&configured, features)
	if field := configured.Fields["Roles"]; field.Type != FieldString || !standardFieldPolicyPreservesConfiguredField(configured, "Roles") {
		t.Fatalf("configured field or its snapshot changed: %#v", field)
	}
}
