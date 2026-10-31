package schema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Salesforce collision fixture binds Industry standardValueSet metadata to
// Account.Industry (sobject_json3_collision/org.tsv:2-4, API 62 and 67 twins).
func TestIndustryStandardValueSetNativeBinding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "standardValueSets", "Industry.standardValueSet-meta.xml")
	writeFile(t, path, `<StandardValueSet xmlns="http://soap.sforce.com/2006/04/metadata"><standardValue><fullName>Agriculture</fullName><label>Banking</label><default>false</default></standardValue></StandardValueSet>`)
	got, err := LoadProject(project.Project{Root: root, StandardValueSetFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Objects) != 1 || got.Objects[0].Name != "Account" {
		t.Fatalf("objects: %#v", got.Objects)
	}
	field, ok := fieldsByName(got.Objects[0].Fields)["Industry"]
	if !ok || len(field.PicklistValues) != 1 || field.PicklistValues[0].FullName != "Agriculture" || field.PicklistValues[0].Label != "Banking" {
		t.Fatalf("Industry binding: %#v", field)
	}
}
