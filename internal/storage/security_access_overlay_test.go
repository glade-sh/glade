package storage

import (
	"reflect"
	"testing"
)

// R068/R069 require the same OwnerId facets when source-backed Name metadata
// is retained. The marker also preserves the canonical overlay's idempotence.
func TestSecurityAccessMetadataDoesNotSkipStandardOverlay(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "A28Access__c", SharingModel: "Private",
		Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString, Required: true}},
	}
	baseline := definition.Clone()
	EnsureStandardObjectFields(&baseline)
	for _, metadata := range []map[string]string{{}, {"nameFieldType": "Text"}, {"kind": "customObject"}} {
		candidate := definition.Clone()
		candidate.Metadata = metadata
		if standardFieldsOverlayApplied(candidate, "") {
			t.Fatal("absent marker treated as an applied overlay")
		}
		EnsureStandardObjectFields(&candidate)
		if !reflect.DeepEqual(candidate.Fields, baseline.Fields) || !reflect.DeepEqual(candidate.Relations, baseline.Relations) {
			t.Fatalf("unrelated metadata changed standard facets: %#v", candidate)
		}
		if !standardFieldsOverlayApplied(candidate, "") || standardFieldsOverlayApplied(candidate, "PersonAccounts") || StandardObjectFieldsNeedWrite(candidate) {
			t.Fatal("applied overlay lost its feature identity or idempotence")
		}
	}
}
