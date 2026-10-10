package storage

import (
	"reflect"
	"testing"
)

func TestStandardObjectFieldsWritePredicateIsIdempotent(t *testing.T) {
	cases := []struct {
		name     string
		features []string
	}{
		{name: "Account"},
		{name: "Contact"},
		{name: "Lead"},
		{name: "Case"},
		{name: "User"},
		{name: "Opportunity"},
		{name: "Probe__c"},
		{name: "Probe__mdt"},
		{name: "Account", features: []string{"PersonAccounts", "StateAndCountryPicklist"}},
	}
	for _, tc := range cases {
		signature := canonicalFeatureSignature(tc.features)
		t.Run(tc.name+"/"+signature, func(t *testing.T) {
			definition := ObjectDefinition{APIName: tc.name, Fields: map[string]Field{
				"Name": {APIName: "Name", Type: FieldString},
			}}
			EnsureStandardObjectFieldsForFeatures(&definition, tc.features)
			if standardObjectFieldsNeedWrite(definition, signature) {
				t.Fatal("fully enriched definition still requests a write")
			}
			before := definition.Clone()
			fields := reflect.ValueOf(definition.Fields).Pointer()
			EnsureStandardObjectFieldsForFeatures(&definition, tc.features)
			if !reflect.DeepEqual(definition, before) {
				t.Fatal("repeated enrichment changed a fully enriched definition")
			}
			if reflect.ValueOf(definition.Fields).Pointer() != fields {
				t.Fatal("repeated enrichment replaced its field map")
			}
		})
	}
}
