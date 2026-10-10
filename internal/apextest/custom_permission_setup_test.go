package apextest

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"path/filepath"
	"testing"
)

func TestProjectCustomPermissionSeedingPreservesExistingRecords(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.customPermission-meta.xml")
	writeFile(t, path, `<CustomPermission><label>Owned Label</label><description>Owned description</description><isLicensed>false</isLicensed></CustomPermission>`)
	malformed := filepath.Join(root, "Malformed.customPermission-meta.xml")
	writeFile(t, malformed, `<CustomPermission`)
	p := project.Project{CustomPermissionFiles: []string{path, malformed}}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	applyProjectCustomPermissionRecords(&org, p)
	state := org.Objects["CustomPermission"]
	if len(state.Records) != 1 {
		t.Fatalf("records=%+v", state.Records)
	}
	for _, record := range state.Records {
		for field, want := range map[string]string{"DeveloperName": "Owned", "MasterLabel": "Owned Label", "Description": "Owned description"} {
			got, _ := record.GetField(field)
			if got.String != want {
				t.Fatalf("%s=%+v", field, got)
			}
		}
	}
	sequence := org.IDSequences["CustomPermission"]
	writeFile(t, path, `<CustomPermission><label>Changed</label></CustomPermission>`)
	applyProjectCustomPermissionRecords(&org, p)
	if len(org.Objects["CustomPermission"].Records) != 1 || org.IDSequences["CustomPermission"] != sequence {
		t.Fatal("seeding duplicated existing metadata")
	}
	for _, record := range org.Objects["CustomPermission"].Records {
		got, _ := record.GetField("MasterLabel")
		if got.String != "Owned Label" {
			t.Fatal("existing record overwritten")
		}
	}
}
