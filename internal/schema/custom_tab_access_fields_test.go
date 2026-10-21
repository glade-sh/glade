package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Malformed tab metadata that does not belong to a custom object must not
// fail the whole schema load; valid tabs still add recent-access fields.
func TestAddCustomTabAccessFieldsToleratesMalformedTab(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "Orbit__c.tab-meta.xml")
	bad := filepath.Join(dir, "Unrelated.tab-meta.xml")
	if err := os.WriteFile(good, []byte(`<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata"><customObject>true</customObject></CustomTab>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(`<CustomTab>`), 0o644); err != nil {
		t.Fatal(err)
	}
	objects := map[string]*Object{"Orbit__c": {Name: "Orbit__c"}}
	if err := addCustomTabAccessFields(project.Project{TabFiles: []string{bad, good}}, objects); err != nil {
		t.Fatalf("addCustomTabAccessFields error = %v", err)
	}
	if fieldIndex(objects["Orbit__c"].Fields, "LastViewedDate") < 0 {
		t.Fatalf("valid tab did not add LastViewedDate: %+v", objects["Orbit__c"].Fields)
	}
}
