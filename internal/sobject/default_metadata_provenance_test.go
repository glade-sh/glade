package sobject

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestDefaultMetadataProvenanceRoundTrip(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{Name: "Probe__c", Fields: []schema.Field{{Name: "Amount__c", Type: "Number", DefaultValue: "7", DefaultValueFromMetadata: true}}}}})
	described, err := registry.Describe("Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		data, err := json.Marshal(described)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &described); err != nil {
			t.Fatal(err)
		}
		definition := ToObjectDefinition(described)
		if !definition.Fields["Amount__c"].DefaultValueFromMetadata {
			t.Fatalf("pass %d: lost metadata provenance", pass)
		}
		data, err = json.Marshal(definition)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &definition); err != nil {
			t.Fatal(err)
		}
		described = FromObjectDefinition(definition)
	}
}
