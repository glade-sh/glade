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

func TestFormulaMetadataPolicyAndScaleRoundTrip(t *testing.T) {
	root := t.TempDir()
	objectDir := filepath.Join(root, "objects", "Policy__c")
	if err := os.MkdirAll(filepath.Join(objectDir, "fields"), 0700); err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(objectDir, "Policy__c.object-meta.xml")
	if err := os.WriteFile(objectPath, []byte(`<CustomObject><label>Policy</label><pluralLabel>Policies</pluralLabel></CustomObject>`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, scale, policy string
		specified           bool
		wantScale           int
	}{
		{"zero", "<scale>0</scale>", "BlankAsZero", true, 0},
		{"two", "<scale>2</scale>", "BlankAsBlank", true, 2},
		{"absent", "", "", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(objectDir, "fields", "Total__c.field-meta.xml")
			xml := `<CustomField><fullName>Total__c</fullName><label>Total</label><type>Currency</type><formula>Amount__c</formula>` + tc.scale + `<formulaTreatBlanksAs>` + tc.policy + `</formulaTreatBlanksAs></CustomField>`
			if err := os.WriteFile(path, []byte(xml), 0600); err != nil {
				t.Fatal(err)
			}
			parsed, err := schema.LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(parsed)
			if err != nil {
				t.Fatal(err)
			}
			var restored schema.Schema
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			desc, err := BuildDescribeRegistry(restored).Describe("Policy__c")
			if err != nil {
				t.Fatal(err)
			}
			def := ToObjectDefinition(desc)
			data, err = json.Marshal(def)
			if err != nil {
				t.Fatal(err)
			}
			var cached storage.ObjectDefinition
			if err := json.Unmarshal(data, &cached); err != nil {
				t.Fatal(err)
			}
			again := ToObjectDefinition(FromObjectDefinition(cached))
			field, exists := again.Fields["Total__c"]
			if !exists {
				t.Fatal("formula field missing from metadata round trip")
			}
			if field.FormulaTreatBlanksAs != tc.policy || field.ScaleSpecified != tc.specified || field.Scale != tc.wantScale {
				t.Fatalf("round trip field=%#v", field)
			}
		})
	}
}
