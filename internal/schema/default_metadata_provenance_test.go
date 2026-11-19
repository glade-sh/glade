package schema

import (
	"encoding/json"
	"testing"
)

func TestDefaultMetadataProvenance(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{{"'seed'", true}, {"7", true}, {"", false}} {
		field := fieldFromXML(customFieldXML{DefaultValue: tc.input}, "Probe__c")
		data, err := json.Marshal(field)
		if err != nil {
			t.Fatal(err)
		}
		var restored Field
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if restored.DefaultValueFromMetadata != tc.want {
			t.Errorf("default %q metadata provenance = %v, want %v", tc.input, restored.DefaultValueFromMetadata, tc.want)
		}
	}
	var legacy Field
	if err := json.Unmarshal([]byte(`{"name":"Probe__c","defaultValue":"7"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.DefaultValueFromMetadata {
		t.Fatal("legacy JSON must retain unknown provenance")
	}
}
