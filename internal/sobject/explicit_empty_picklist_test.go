package sobject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestExplicitPicklistProvenanceRoutes(t *testing.T) {
	for _, name := range []string{"GenderIdentity", "Pronouns"} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				configured bool
				values     []storage.PicklistValue
			}{
				{"empty", true, nil}, {"populated", true, []storage.PicklistValue{{Value: "Sample", Label: "Sample", Active: true}}}, {"unconfigured", false, nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					def, ok := storage.StandardObjectDefinition("Lead")
					if !ok {
						t.Fatal("missing Lead")
					}
					f := def.Fields[name]
					f.PicklistValues = tc.values
					f.PicklistValuesConfigured = tc.configured
					def.Fields[name] = f
					desc := FromObjectDefinition(def)
					data, err := json.Marshal(desc)
					if err != nil {
						t.Fatal(err)
					}
					var restored DescribeSObjectResult
					if err = json.Unmarshal(data, &restored); err != nil {
						t.Fatal(err)
					}
					round := ToObjectDefinition(restored)
					storage.EnsureStandardObjectFields(&round)
					got := round.Fields[name]
					if got.PicklistValuesConfigured != tc.configured {
						t.Fatalf("configured=%v; want %v", got.PicklistValuesConfigured, tc.configured)
					}
					if tc.configured && len(got.PicklistValues) != len(tc.values) {
						t.Fatalf("configured values=%v; want %v", got.PicklistValues, tc.values)
					}
					if !tc.configured && len(got.PicklistValues) == 0 {
						t.Fatal("unconfigured values were not repaired")
					}
				})
			}
		})
	}
}

func TestExplicitPicklistSchemaToStorage(t *testing.T) {
	for _, name := range []string{"GenderIdentity", "Pronouns"} {
		t.Run(name, func(t *testing.T) {
			var f schema.Field
			if err := json.Unmarshal([]byte(`{"name":"`+name+`","type":"Picklist","picklistValuesConfigured":true}`), &f); err != nil {
				t.Fatal(err)
			}
			reg := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{Name: "Lead", Fields: []schema.Field{f}}}})
			desc, err := reg.Describe("Lead")
			if err != nil {
				t.Fatal(err)
			}
			def := ToObjectDefinition(desc)
			storage.EnsureStandardObjectFields(&def)
			got := def.Fields[name]
			if !got.PicklistValuesConfigured || len(got.PicklistValues) != 0 {
				t.Fatalf("lost explicit empty values: configured=%v values=%v", got.PicklistValuesConfigured, got.PicklistValues)
			}
		})
	}
}

func TestExplicitEmptyPicklistProjectMetadataRoute(t *testing.T) {
	for _, name := range []string{"GenderIdentity", "Pronouns"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "objects", "Lead", "fields")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, name+".field-meta.xml")
			xml := `<CustomField><fullName>` + name + `</fullName><type>Picklist</type><valueSet><valueSetDefinition><sorted>false</sorted></valueSetDefinition></valueSet></CustomField>`
			if err := os.WriteFile(path, []byte(xml), 0644); err != nil {
				t.Fatal(err)
			}
			loaded, err := schema.LoadProject(project.Project{Root: root, FieldFiles: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			desc, err := BuildDescribeRegistry(loaded).Describe("Lead")
			if err != nil {
				t.Fatal(err)
			}
			def := ToObjectDefinition(desc)
			storage.EnsureStandardObjectFields(&def)
			got := def.Fields[name]
			if !got.PicklistValuesConfigured || len(got.PicklistValues) != 0 {
				t.Fatalf("project empty lost: configured=%v values=%v", got.PicklistValuesConfigured, got.PicklistValues)
			}
		})
	}
}

func TestExplicitPicklistFalseSnapshotAllowsRepair(t *testing.T) {
	for _, name := range []string{"GenderIdentity", "Pronouns"} {
		t.Run(name, func(t *testing.T) {
			var field schema.Field
			if err := json.Unmarshal([]byte(`{"name":"`+name+`","type":"Picklist","picklistValues":[],"picklistValuesConfigured":false}`), &field); err != nil {
				t.Fatal(err)
			}
			registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{Name: "Lead", Fields: []schema.Field{field}}}})
			desc, err := registry.Describe("Lead")
			if err != nil {
				t.Fatal(err)
			}
			def := ToObjectDefinition(desc)
			storage.EnsureStandardObjectFields(&def)
			got := def.Fields[name]
			if got.PicklistValuesConfigured || len(got.PicklistValues) == 0 {
				t.Fatalf("explicit false blocked native repair: %#v", got)
			}
		})
	}
}
