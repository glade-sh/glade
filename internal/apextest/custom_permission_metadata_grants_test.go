package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce's access-troubleshooting documentation includes custom permissions
// in SetupEntityAccess and queries assignments through that junction.
func TestCustomPermissionMetadataGrantConformance(t *testing.T) {
	var fixture struct {
		APIs, Sources, Boundaries []string
		TestClass, Source         string
		MethodCount               int
		Files                     map[string]string
	}
	raw, err := os.ReadFile("testdata/conformance/custom_permission_metadata_grants.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fixture.APIs, []string{"62.0", "67.0", "68.0"}) || fixture.MethodCount != 4 || len(fixture.Files) != 3 || len(fixture.Sources) != 2 || fixture.Source == "" {
		t.Fatal("incomplete documented metadata fixture")
	}
	for _, api := range fixture.APIs {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for path, contents := range fixture.Files {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), contents)
			}
			path := filepath.Join(root, "force-app/main/default/classes", fixture.TestClass+".cls")
			writeFile(t, path, fixture.Source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{fixture.TestClass}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != fixture.MethodCount || summary.Passed != fixture.MethodCount {
				data, _ := json.Marshal(run)
				t.Fatalf("documented metadata grant: %s", data)
			}
		})
	}
	for _, boundary := range fixture.Boundaries {
		t.Log("UNCHANGED", boundary)
	}
}

func TestProjectCustomPermissionGrantImportControls(t *testing.T) {
	for _, tc := range []struct {
		name, reference, existing string
		enabled                   bool
		want                      int
	}{
		{"enabled declared", "OwnedView", "", true, 1},
		{"disabled", "OwnedView", "", false, 0},
		{"unresolved", "UndeclaredPermission", "", true, 0},
		{"preserved existing", "OwnedView", "same", true, 1},
		{"preserved unrelated with stale sequence", "OwnedView", "other", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			permission := filepath.Join(root, "OwnedView.customPermission-meta.xml")
			set := filepath.Join(root, "OwnedReader.permissionset-meta.xml")
			writeFile(t, permission, `<CustomPermission><label>Owned View</label></CustomPermission>`)
			writeFile(t, set, fmt.Sprintf(`<PermissionSet><label>Owned Reader</label><customPermissions><enabled>%t</enabled><name>%s</name></customPermissions></PermissionSet>`, tc.enabled, tc.reference))
			p := project.Project{CustomPermissionFiles: []string{permission}, PermissionSetFiles: []string{set}}
			org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
			preservedID := storage.ID("0J0000000000901")
			if tc.existing != "" {
				applyProjectCustomPermissionRecords(&org, p)
				permissionID, _ := recordFieldID(org.Objects["CustomPermission"], "DeveloperName", "OwnedView")
				storage.EnsureStandardObject(&org, "PermissionSet")
				parentID := storage.ID("0PS000000000901")
				org.Objects["PermissionSet"].Records[parentID] = storage.Record{ID: parentID, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("OwnedReader")}}
				storage.EnsureStandardObject(&org, "SetupEntityAccess")
				if tc.existing == "other" {
					parentID = storage.ID("0PS000000000902")
					org.Objects["PermissionSet"].Records[parentID] = storage.Record{ID: parentID, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("OwnedOther")}}
					generator := storage.NewStandardIDGenerator()
					var err error
					preservedID, err = generator.Next("SetupEntityAccess")
					if err != nil {
						t.Fatal(err)
					}
				}
				org.Objects["SetupEntityAccess"].Records[preservedID] = storage.Record{ID: preservedID, Object: "SetupEntityAccess", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID), "SetupEntityId": storage.IDValue(permissionID), "SetupEntityType": storage.StringValue("CustomPermission"), "Preserved": storage.StringValue("owned")}}
			}
			ApplyProjectPermissionSets(&org, p)
			if got := len(org.Objects["SetupEntityAccess"].Records); got != tc.want {
				t.Fatalf("junction count=%d, want %d", got, tc.want)
			}
			before := org.Clone()
			ApplyProjectPermissionSets(&org, p)
			if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
				t.Fatal("repeated metadata import changed records or sequences")
			}
			if got := len(org.Objects["CustomPermission"].Records); got != 1 {
				t.Fatalf("undeclared custom permission fabricated: %d", got)
			}
			if len(org.Objects["PermissionSetAssignment"].Records) != 0 {
				t.Fatal("metadata import assigned a permission set to a user")
			}
			if tc.existing != "" {
				value, _ := org.Objects["SetupEntityAccess"].Records[preservedID].GetField("Preserved")
				if value.String != "owned" {
					t.Fatal("existing access row overwritten")
				}
			}
		})
	}
}
