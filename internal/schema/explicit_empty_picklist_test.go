package schema

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestExplicitPicklistDefinitionProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, valueSet string
		configured     bool
	}{
		{"empty", `<valueSet><valueSetDefinition><sorted>false</sorted></valueSetDefinition></valueSet>`, true},
		{"populated", `<valueSet><valueSetDefinition><value><fullName>Sample</fullName><label>Sample</label></value></valueSetDefinition></valueSet>`, true},
		{"absent", ``, false},
		{"named-only", `<valueSet><valueSetName>SomeSet</valueSetName></valueSet>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Example.field-meta.xml")
			writeFile(t, path, `<CustomField><fullName>Example</fullName><type>Picklist</type>`+tc.valueSet+`</CustomField>`)
			field, err := loadField(path)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(field)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err = json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			got, _ := raw["picklistValuesConfigured"].(bool)
			if got != tc.configured {
				t.Fatalf("configured=%v; want %v: %s", got, tc.configured, data)
			}
		})
	}
}

func TestExplicitPicklistSnapshotPresence(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		want       bool
	}{
		{"empty", `{"name":"GenderIdentity","picklistValues":[]}`, true},
		{"populated", `{"name":"GenderIdentity","picklistValues":[{"fullName":"Sample"}]}`, true},
		{"omitted", `{"name":"GenderIdentity"}`, false},
		{"null", `{"name":"GenderIdentity","picklistValues":null}`, false},
		{"explicit-false", `{"name":"GenderIdentity","picklistValues":[],"picklistValuesConfigured":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f Field
			if err := json.Unmarshal([]byte(tc.data), &f); err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(f)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err = json.Unmarshal(b, &raw); err != nil {
				t.Fatal(err)
			}
			got, _ := raw["picklistValuesConfigured"].(bool)
			if got != tc.want {
				t.Fatalf("configured=%v want %v: %s", got, tc.want, b)
			}
		})
	}
}

func TestExplicitEmptySliceSchemaSnapshotRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name       string
		values     []PicklistValue
		configured bool
	}{
		{"explicit empty", []PicklistValue{}, true}, {"omitted", nil, false}, {"populated", []PicklistValue{{FullName: "Sample"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := Field{Name: "GenderIdentity", PicklistValues: tc.values}
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var restored Field
			if err = json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.PicklistValuesConfigured != tc.configured {
				t.Fatalf("snapshot %s lost configured=%v", data, tc.configured)
			}
		})
	}
}
