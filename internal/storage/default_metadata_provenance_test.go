package storage

import "testing"

func TestDefaultMetadataProvenanceOverlay(t *testing.T) {
	definition := ObjectDefinition{Fields: map[string]Field{
		"Explicit": {APIName: "Explicit", DefaultValue: "'metadata'", DefaultValueFromMetadata: true},
		"Missing":  {APIName: "Missing", DefaultValueFromMetadata: true},
	}}
	ensureFieldDefault(&definition, "Explicit", "fallback")
	if !definition.Fields["Explicit"].DefaultValueFromMetadata {
		t.Fatal("ensure lost retained metadata provenance")
	}
	ensureFieldDefault(&definition, "Missing", "fallback")
	if definition.Fields["Missing"].DefaultValueFromMetadata {
		t.Fatal("fallback retained stale metadata provenance")
	}
	setFieldDefault(&definition, "Explicit", "replacement")
	if definition.Fields["Explicit"].DefaultValueFromMetadata {
		t.Fatal("replacement retained metadata provenance")
	}
}
